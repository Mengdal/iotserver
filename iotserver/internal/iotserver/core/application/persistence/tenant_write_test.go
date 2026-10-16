package persistence

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/LMgateways/iotserver-driver-proto/thingmodel"

	"iotserver/internal/dtos"
	interfaces "iotserver/internal/iotserver/core/interface"
	"iotserver/internal/pkg/logger"
	"iotserver/internal/pkg/tenant"
	"iotserver/internal/tools/datadb/duckdb"
)

// TestWriteCarriesDeviceTenant 上行消息不带租户时，写入必须按设备归属补上 tenant_id
func TestWriteCarriesDeviceTenant(t *testing.T) {
	dir := t.TempDir()
	lc := logger.NewClient("tenant-write-test", "error", "")
	dl, err := duckdb.NewClient(dtos.Configuration{DataSource: dir + "/core-data.duckdb"}, lc)
	if err != nil {
		t.Fatalf("duckdb 不可用: %v", err)
	}
	defer dl.CloseSession()
	client := dl.(*duckdb.Client)

	app := &persistApp{
		dataDbClient: client,
		lc:           lc,
		tenantCache:  newDeviceTenantCache(time.Minute, 16),
	}

	const deviceID = "dev-tenant-write"
	const tenantID = "t-aaa"
	// 预置缓存：等价于「已从元数据库解析出设备归属」，避免测试依赖完整 DBClient
	app.tenantCache.set(deviceID, tenantID)

	now := time.Now().UnixMilli()
	payload, _ := json.Marshal(map[string]interface{}{
		"msgId": "tenant-write-1",
		"data": map[string]dtos.ReportData{
			"temp": {Value: 26.5, Time: now},
		},
	})
	msg := dtos.ThingModelMessage{
		Cid:    deviceID,
		OpType: int32(thingmodel.OperationType_PROPERTY_REPORT),
		Data:   string(payload),
	}
	// 上行链路本身没有租户上下文
	if err := app.SaveDeviceThingModelData(context.Background(), msg); err != nil {
		t.Fatalf("落库失败: %v", err)
	}

	hit, err := client.CountAllMsgByTimeRange(tenantID, now-60_000, now+60_000)
	if err != nil {
		t.Fatalf("按租户计数失败: %v", err)
	}
	if hit != 1 {
		t.Fatalf("租户 %s 应有 1 条，实际 %d（tenant_id 没落到行上）", tenantID, hit)
	}
	other, err := client.CountAllMsgByTimeRange("t-bbb", now-60_000, now+60_000)
	if err != nil {
		t.Fatalf("按租户计数失败: %v", err)
	}
	if other != 0 {
		t.Fatalf("租户 t-bbb 不应有数据，实际 %d", other)
	}
	platform, err := client.CountAllMsgByTimeRange("", now-60_000, now+60_000)
	if err != nil {
		t.Fatalf("平台计数失败: %v", err)
	}
	if platform != 1 {
		t.Fatalf("平台作用域应看到 1 条，实际 %d", platform)
	}
}

// TestWriteKeepsExplicitTenantContext 已带租户上下文时不应被改写
func TestWriteKeepsExplicitTenantContext(t *testing.T) {
	dir := t.TempDir()
	lc := logger.NewClient("tenant-write-test", "error", "")
	dl, err := duckdb.NewClient(dtos.Configuration{DataSource: dir + "/core-data.duckdb"}, lc)
	if err != nil {
		t.Fatalf("duckdb 不可用: %v", err)
	}
	defer dl.CloseSession()
	client := dl.(*duckdb.Client)

	app := &persistApp{
		dataDbClient: client,
		lc:           lc,
		tenantCache:  newDeviceTenantCache(time.Minute, 16),
	}

	const deviceID = "dev-ctx-tenant"
	// 缓存与上下文都给出租户，且一致
	app.tenantCache.set(deviceID, "t-ccc")
	ctx := tenant.With(context.Background(), tenant.Context{TenantID: "t-ccc"})

	now := time.Now().UnixMilli()
	payload, _ := json.Marshal(map[string]interface{}{
		"msgId": "tenant-write-2",
		"data": map[string]dtos.ReportData{
			"temp": {Value: 1.0, Time: now},
		},
	})
	msg := dtos.ThingModelMessage{
		Cid:    deviceID,
		OpType: int32(thingmodel.OperationType_PROPERTY_REPORT),
		Data:   string(payload),
	}
	if err := app.SaveDeviceThingModelData(ctx, msg); err != nil {
		t.Fatalf("落库失败: %v", err)
	}
	if n, _ := client.CountAllMsgByTimeRange("t-ccc", now-60_000, now+60_000); n != 1 {
		t.Fatalf("租户 t-ccc 应有 1 条，实际 %d", n)
	}
}

func TestDeviceTenantCache(t *testing.T) {
	c := newDeviceTenantCache(50*time.Millisecond, 4)

	if _, ok := c.get("dev-1"); ok {
		t.Fatalf("空缓存不应命中")
	}
	c.set("dev-1", "t-1")
	if tid, ok := c.get("dev-1"); !ok || tid != "t-1" {
		t.Fatalf("命中失败: %q %v", tid, ok)
	}

	// TTL 过期
	time.Sleep(80 * time.Millisecond)
	if _, ok := c.get("dev-1"); ok {
		t.Fatalf("过期后不应命中")
	}

	// 容量上限：超限后仍可写入且不会无界增长
	for i := 0; i < 20; i++ {
		c.set(string(rune('a'+i)), "t-x")
	}
	if c.size() > 4 {
		t.Fatalf("缓存超出上限: %d", c.size())
	}

	// invalidate
	c.set("dev-2", "t-2")
	c.invalidate("dev-2")
	if _, ok := c.get("dev-2"); ok {
		t.Fatalf("invalidate 后不应命中")
	}
}

// TestMsgCountUsesTenantScopeAndFastPath 消息计数：
// 平台作用域统计全平台、租户作用域只统计本租户，且走数据层的「一条 SQL」快路径
// （persistApp 未注入 dbClient，若走按设备汇总的 N+1 路径会 panic）。
func TestMsgCountUsesTenantScopeAndFastPath(t *testing.T) {
	dir := t.TempDir()
	lc := logger.NewClient("msgcount-test", "error", "")
	dl, err := duckdb.NewClient(dtos.Configuration{DataSource: dir + "/core-data.duckdb"}, lc)
	if err != nil {
		t.Fatalf("duckdb 不可用: %v", err)
	}
	defer dl.CloseSession()

	// 编译期/运行期双重确认：数据层实现了 MsgCounter
	counter, ok := dl.(interfaces.MsgCounter)
	if !ok {
		t.Fatalf("duckdb 未实现 interfaces.MsgCounter，计数会退化成 N+1")
	}
	_ = counter

	app := &persistApp{
		dataDbClient: dl,
		lc:           lc,
		tenantCache:  newDeviceTenantCache(time.Minute, 16),
	}

	now := time.Now().UnixMilli()
	seed := func(tenantID, deviceID string) {
		ctx := context.Background()
		if tenantID != "" {
			ctx = tenant.With(ctx, tenant.Context{TenantID: tenantID})
		}
		if err := dl.Insert(ctx, deviceID, map[string]interface{}{
			"temp": dtos.ReportData{Value: 1.0, Time: now},
		}); err != nil {
			t.Fatalf("insert %s: %v", deviceID, err)
		}
	}
	seed("t-aaa", "mc-dev-1")
	seed("t-aaa", "mc-dev-2")
	seed("t-bbb", "mc-dev-3")

	// 平台作用域：全平台 3 条
	all, err := app.searchDeviceMsgCount(tenant.Platform(context.Background()), now-60_000, now+60_000)
	if err != nil {
		t.Fatalf("平台计数失败: %v", err)
	}
	if all != 3 {
		t.Fatalf("平台计数 = %d, want 3", all)
	}

	// 租户作用域：只统计本租户
	a, err := app.searchDeviceMsgCount(
		tenant.With(context.Background(), tenant.Context{TenantID: "t-aaa"}), now-60_000, now+60_000)
	if err != nil {
		t.Fatalf("租户计数失败: %v", err)
	}
	if a != 2 {
		t.Fatalf("t-aaa 计数 = %d, want 2", a)
	}
	b, err := app.searchDeviceMsgCount(
		tenant.With(context.Background(), tenant.Context{TenantID: "t-bbb"}), now-60_000, now+60_000)
	if err != nil {
		t.Fatalf("租户计数失败: %v", err)
	}
	if b != 1 {
		t.Fatalf("t-bbb 计数 = %d, want 1", b)
	}
}
