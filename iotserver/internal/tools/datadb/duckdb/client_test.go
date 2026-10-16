package duckdb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"iotserver/internal/dtos"
	"iotserver/internal/models"
	"iotserver/internal/pkg/constants"
	"iotserver/internal/pkg/logger"
	"iotserver/internal/pkg/tenant"
)

// newTestClient 在临时目录建库
func newTestClient(t *testing.T) *Client {
	t.Helper()
	dir := t.TempDir()
	cfg := dtos.Configuration{DataSource: filepath.Join(dir, "core-data.duckdb")}
	lc := logger.NewClient("duckdb-test", "error", "")
	client, err := NewClient(cfg, lc)
	if err != nil {
		t.Fatalf("duckdb 不可用: %v", err)
	}
	t.Cleanup(func() { client.CloseSession() })
	return client.(*Client)
}

func itoa(v int64) string { return fmt.Sprintf("%d", v) }

func TestInsertAndQueryProperty(t *testing.T) {
	c := newTestClient(t)
	ctx := context.Background()
	device := models.Device{Id: "dev-1"}

	base := time.Now().Add(-2*time.Hour).UnixNano() / int64(time.Millisecond)
	for i := 0; i < 3; i++ {
		data := map[string]interface{}{"temp": dtos.ReportData{Value: 20.0 + float64(i), Time: base + int64(i)*60_000}}
		if err := c.Insert(ctx, device.Id, data); err != nil {
			t.Fatalf("insert temp: %v", err)
		}
	}
	for i := 0; i < 2; i++ {
		data := map[string]interface{}{"humidity": dtos.ReportData{Value: 55.0 + float64(i), Time: base + int64(i)*60_000}}
		if err := c.Insert(ctx, device.Id, data); err != nil {
			t.Fatalf("insert humidity: %v", err)
		}
	}

	req := dtos.ThingModelPropertyDataRequest{
		DeviceId: device.Id,
		Code:     "temp",
		ThingModelDataBaseRequest: dtos.ThingModelDataBaseRequest{
			Range: []int64{base - 1000, base + 10*60_000},
		},
		BaseSearchConditionQuery: dtos.BaseSearchConditionQuery{Page: 1, PageSize: 10},
	}

	rows, count, err := c.GetDeviceProperty(req, device)
	if err != nil {
		t.Fatalf("query temp: %v", err)
	}
	if count != 3 || len(rows) != 3 {
		t.Fatalf("temp: count=%d len=%d, want 3/3", count, len(rows))
	}
	// 返回按时间倒序
	if rows[0].Time <= rows[2].Time {
		t.Fatalf("期望按时间倒序, got %d..%d", rows[0].Time, rows[2].Time)
	}

	// 时间范围外应为空
	req.Range = []int64{base + 20*60_000, base + 30*60_000}
	empty, count, err := c.GetDeviceProperty(req, device)
	if err != nil {
		t.Fatalf("query out of range: %v", err)
	}
	if count != 0 || len(empty) != 0 {
		t.Fatalf("out of range: count=%d len=%d, want 0/0", count, len(empty))
	}

	// 不同属性互不干扰
	req.Code = "humidity"
	req.Range = []int64{base - 1000, base + 10*60_000}
	hum, count, err := c.GetDeviceProperty(req, device)
	if err != nil {
		t.Fatalf("query humidity: %v", err)
	}
	if count != 2 || len(hum) != 2 {
		t.Fatalf("humidity: count=%d len=%d, want 2/2", count, len(hum))
	}
}

func TestInsertTextValueAndCount(t *testing.T) {
	c := newTestClient(t)
	ctx := context.Background()
	device := models.Device{Id: "dev-text"}
	ts := time.Now().UnixNano() / int64(time.Millisecond)

	if err := c.Insert(ctx, device.Id, map[string]interface{}{"mode": dtos.ReportData{Value: "auto", Time: ts}}); err != nil {
		t.Fatalf("insert text: %v", err)
	}
	req := dtos.ThingModelPropertyDataRequest{
		DeviceId: device.Id,
		Code:     "mode",
		ThingModelDataBaseRequest: dtos.ThingModelDataBaseRequest{
			Range: []int64{ts - 1000, ts + 1000},
		},
		BaseSearchConditionQuery: dtos.BaseSearchConditionQuery{Page: 1, PageSize: 10},
	}
	rows, count, err := c.GetDeviceProperty(req, device)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if count != 1 || len(rows) != 1 {
		t.Fatalf("count=%d len=%d, want 1/1", count, len(rows))
	}
	if rows[0].Value != "auto" {
		t.Fatalf("value = %v, want auto", rows[0].Value)
	}

	n, err := c.GetDevicePropertyCount(req)
	if err != nil || n != 1 {
		t.Fatalf("GetDevicePropertyCount = %d, %v; want 1, nil", n, err)
	}
}

func TestEventAndService(t *testing.T) {
	c := newTestClient(t)
	ctx := context.Background()
	device := models.Device{Id: "dev-evt"}
	ts := time.Now().UnixNano() / int64(time.Millisecond)

	eventPayload := `{"eventCode":"alarm","eventTime":` + itoa(ts) + `,"outputParams":{"level":"high"}}`
	if err := c.Insert(ctx, eventPrefix+device.Id, map[string]interface{}{"alarm": eventPayload}); err != nil {
		t.Fatalf("insert event: %v", err)
	}
	evtReq := dtos.ThingModelEventDataRequest{
		DeviceId:  device.Id,
		EventCode: "alarm",
		ThingModelDataBaseRequest: dtos.ThingModelDataBaseRequest{
			Range: []int64{ts - 1000, ts + 1000},
		},
		BaseSearchConditionQuery: dtos.BaseSearchConditionQuery{Page: 1, PageSize: 10},
	}
	events, count, err := c.GetDeviceEvent(evtReq, device, models.Product{})
	if err != nil {
		t.Fatalf("query event: %v", err)
	}
	if count != 1 || len(events) != 1 {
		t.Fatalf("event count=%d len=%d, want 1/1", count, len(events))
	}
	if events[0].EventCode != "alarm" || events[0].EventTime != ts {
		t.Fatalf("event = %+v, want code=alarm time=%d", events[0], ts)
	}
	if events[0].OutputParams["level"] != "high" {
		t.Fatalf("event outputParams = %+v", events[0].OutputParams)
	}

	svcPayload := `{"code":"reboot","time":` + itoa(ts) + `,"inputParams":{"delay":3},"outputParams":{"ok":true}}`
	if err := c.Insert(ctx, servicePrefix+device.Id, map[string]interface{}{"reboot": svcPayload}); err != nil {
		t.Fatalf("insert service: %v", err)
	}
	svcReq := dtos.ThingModelServiceDataRequest{
		DeviceId: device.Id,
		Code:     "reboot",
		ThingModelDataBaseRequest: dtos.ThingModelDataBaseRequest{
			Range: []int64{ts - 1000, ts + 1000},
		},
		BaseSearchConditionQuery: dtos.BaseSearchConditionQuery{Page: 1, PageSize: 10},
	}
	svcs, count, err := c.GetDeviceService(svcReq, device, models.Product{})
	if err != nil {
		t.Fatalf("query service: %v", err)
	}
	if count != 1 || len(svcs) != 1 {
		t.Fatalf("service count=%d len=%d, want 1/1", count, len(svcs))
	}
	if svcs[0].Code != "reboot" || svcs[0].Time != ts {
		t.Fatalf("service = %+v, want code=reboot time=%d", svcs[0], ts)
	}

	total, err := c.GetDeviceMsgCountByGiveTime(device.Id, ts-1000, ts+1000)
	if err != nil {
		t.Fatalf("msg count: %v", err)
	}
	if total != 2 {
		t.Fatalf("msg count = %d, want 2", total)
	}
}

// TestEventWithoutTimeDoesNotInheritPrevTime 同批「无时间」事件不得继承上一个事件的时间。
// 旧实现把 ts 放在循环外，第二个事件会串成第一个事件的时间。
func TestEventWithoutTimeDoesNotInheritPrevTime(t *testing.T) {
	c := newTestClient(t)
	ctx := context.Background()
	device := models.Device{Id: "dev-bleed"}
	now := time.Now().UnixNano() / int64(time.Millisecond)
	oneHourAgo := now - 3600_000

	// 同一个 Insert 里混入「带时间」和「不带时间」两个事件
	payloadWithTime := `{"eventCode":"withTime","eventTime":` + itoa(oneHourAgo) + `}`
	payloadNoTime := `{"eventCode":"noTime"}`
	if err := c.Insert(ctx, eventPrefix+device.Id, map[string]interface{}{
		"withTime": payloadWithTime,
		"noTime":   payloadNoTime,
	}); err != nil {
		t.Fatalf("insert: %v", err)
	}

	req := dtos.ThingModelEventDataRequest{
		DeviceId:  device.Id,
		EventCode: "noTime",
		ThingModelDataBaseRequest: dtos.ThingModelDataBaseRequest{
			Range: []int64{now - 24*3600*1000, now + 60_000},
		},
		BaseSearchConditionQuery: dtos.BaseSearchConditionQuery{Page: 1, PageSize: 10},
	}
	events, count, err := c.GetDeviceEvent(req, device, models.Product{})
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if count != 1 || len(events) != 1 {
		t.Fatalf("count=%d len=%d, want 1/1", count, len(events))
	}
	if diff := events[0].EventTime - now; diff < -30_000 || diff > 30_000 {
		t.Fatalf("无时间事件的时间戳应为当前时刻，实际 %d（now=%d，差 %dms）——疑似继承了同批其它事件的时间",
			events[0].EventTime, now, diff)
	}
}

func TestDropTableByDevice(t *testing.T) {
	c := newTestClient(t)
	ctx := context.Background()
	device := models.Device{Id: "dev-drop"}
	ts := time.Now().UnixNano() / int64(time.Millisecond)
	if err := c.Insert(ctx, device.Id, map[string]interface{}{
		"temp": dtos.ReportData{Value: 9.0, Time: ts},
	}); err != nil {
		t.Fatalf("insert: %v", err)
	}
	if err := c.DropTable(ctx, constants.DB_PREFIX+device.Id); err != nil {
		t.Fatalf("drop: %v", err)
	}
	n, err := c.GetDevicePropertyCount(dtos.ThingModelPropertyDataRequest{
		DeviceId: device.Id,
		Code:     "temp",
		ThingModelDataBaseRequest: dtos.ThingModelDataBaseRequest{
			Range: []int64{ts - 1000, ts + 1000},
		},
	})
	if err != nil {
		t.Fatalf("count after drop: %v", err)
	}
	if n != 0 {
		t.Fatalf("count after drop = %d, want 0", n)
	}
}

// TestReadAfterWrite 写入后立刻查询应能读到（读前自动落盘）
func TestReadAfterWrite(t *testing.T) {
	c := newTestClient(t)
	ctx := context.Background()
	device := models.Device{Id: "dev-raw"}
	ts := time.Now().UnixNano() / int64(time.Millisecond)

	if err := c.Insert(ctx, device.Id, map[string]interface{}{
		"temp": dtos.ReportData{Value: 33.3, Time: ts},
	}); err != nil {
		t.Fatalf("insert: %v", err)
	}
	rows, count, err := c.GetDeviceProperty(dtos.ThingModelPropertyDataRequest{
		DeviceId: device.Id,
		Code:     "temp",
		ThingModelDataBaseRequest: dtos.ThingModelDataBaseRequest{
			Range: []int64{ts - 1000, ts + 1000},
		},
		BaseSearchConditionQuery: dtos.BaseSearchConditionQuery{Page: 1, PageSize: 10},
	}, device)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if count != 1 || len(rows) != 1 {
		t.Fatalf("写入后立刻查询 count=%d len=%d, want 1/1（读前落盘失效）", count, len(rows))
	}
}

// TestBatchWriteTriggersFlush 超过 flushRows 应自动落盘且数据完整
func TestBatchWriteTriggersFlush(t *testing.T) {
	c := newTestClient(t)
	ctx := context.Background()
	device := models.Device{Id: "dev-batch"}
	base := time.Now().Add(-time.Hour).UnixNano() / int64(time.Millisecond)

	const rows = defaultFlushRows + 137 // 触发一次行数阈值落盘
	for i := 0; i < rows; i++ {
		if err := c.Insert(ctx, device.Id, map[string]interface{}{
			"temp": dtos.ReportData{Value: float64(i), Time: base + int64(i)*10},
		}); err != nil {
			t.Fatalf("insert %d: %v", i, err)
		}
	}

	req := dtos.ThingModelPropertyDataRequest{
		DeviceId: device.Id,
		Code:     "temp",
		ThingModelDataBaseRequest: dtos.ThingModelDataBaseRequest{
			Range: []int64{base - 1000, base + int64(rows+10)*10},
		},
		BaseSearchConditionQuery: dtos.BaseSearchConditionQuery{Page: 1, PageSize: maxPageSize},
	}
	got, count, err := c.GetDeviceProperty(req, device)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	// count 反映真实总行数（证明超过 flushRows 后全部落盘、无丢失）
	if count != rows {
		t.Fatalf("批量写入后 count=%d, want %d（有数据未落盘）", count, rows)
	}
	// 单页被 maxPageSize 截断，这是预期行为
	if len(got) != maxPageSize {
		t.Fatalf("单页 len=%d, want %d", len(got), maxPageSize)
	}
	if v, ok := got[0].Value.(float64); !ok || v != float64(rows-1) {
		t.Fatalf("最新一条 = %v, want %d", got[0].Value, rows-1)
	}
}

// TestPurgeBeforeAndRetention 保留策略：删除超期数据，保留期内数据不受影响
func TestPurgeBeforeAndRetention(t *testing.T) {
	c := newTestClient(t)
	ctx := context.Background()
	device := models.Device{Id: "dev-purge"}
	now := time.Now().UnixNano() / int64(time.Millisecond)

	oldTs := now - 40*24*3600*1000
	newTs := now - 1*24*3600*1000
	if err := c.Insert(ctx, device.Id, map[string]interface{}{
		"temp": dtos.ReportData{Value: 1.0, Time: oldTs},
	}); err != nil {
		t.Fatalf("insert old: %v", err)
	}
	if err := c.Insert(ctx, device.Id, map[string]interface{}{
		"temp": dtos.ReportData{Value: 2.0, Time: newTs},
	}); err != nil {
		t.Fatalf("insert new: %v", err)
	}
	c.flushPending()

	cutoff := now - 30*24*3600*1000
	deleted, err := c.PurgeBefore(ctx, cutoff)
	if err != nil {
		t.Fatalf("purge: %v", err)
	}
	if deleted != 1 {
		t.Fatalf("purge 删除 %d 行, want 1", deleted)
	}

	rows, count, err := c.GetDeviceProperty(dtos.ThingModelPropertyDataRequest{
		DeviceId: device.Id,
		Code:     "temp",
		ThingModelDataBaseRequest: dtos.ThingModelDataBaseRequest{
			Range: []int64{now - 90*24*3600*1000, now + 1000},
		},
		BaseSearchConditionQuery: dtos.BaseSearchConditionQuery{Page: 1, PageSize: 10},
	}, device)
	if err != nil {
		t.Fatalf("query after purge: %v", err)
	}
	if count != 1 || len(rows) != 1 {
		t.Fatalf("purge 后 count=%d len=%d, want 1/1", count, len(rows))
	}
	if v, ok := rows[0].Value.(float64); !ok || v != 2.0 {
		t.Fatalf("保留的应是新数据, got %v", rows[0].Value)
	}
	if got := c.RetentionDays(); got != defaultRetentionDays {
		t.Fatalf("RetentionDays = %d, want %d（默认值）", got, defaultRetentionDays)
	}
}

// TestTenantCaptureOnWrite 写入应从 context 捕获租户，并把 tenant_id 落到行上
func TestTenantCaptureOnWrite(t *testing.T) {
	c := newTestClient(t)
	device := models.Device{Id: "dev-tenant"}
	now := time.Now().UnixNano() / int64(time.Millisecond)

	tenantA := tenant.With(context.Background(), tenant.Context{TenantID: "t-aaa"})
	tenantB := tenant.With(context.Background(), tenant.Context{TenantID: "t-bbb"})
	for _, tc := range []context.Context{tenantA, tenantB} {
		if err := c.Insert(tc, device.Id, map[string]interface{}{
			"temp": dtos.ReportData{Value: 1.0, Time: now},
		}); err != nil {
			t.Fatalf("insert: %v", err)
		}
	}
	c.flushPending()

	var a, b int
	if err := c.db.QueryRow("SELECT count(*) FROM " + tableProperty + " WHERE tenant_id = 't-aaa'").Scan(&a); err != nil {
		t.Fatalf("count t-aaa: %v", err)
	}
	if err := c.db.QueryRow("SELECT count(*) FROM " + tableProperty + " WHERE tenant_id = 't-bbb'").Scan(&b); err != nil {
		t.Fatalf("count t-bbb: %v", err)
	}
	if a != 1 || b != 1 {
		t.Fatalf("租户行数 t-aaa=%d t-bbb=%d, want 1/1", a, b)
	}
}

// TestPurgeTenantBefore 分级保留：只清理指定租户的超期数据，不影响其它租户
func TestPurgeTenantBefore(t *testing.T) {
	c := newTestClient(t)
	device := models.Device{Id: "dev-tenant-purge"}
	now := time.Now().UnixNano() / int64(time.Millisecond)
	oldTs := now - 40*24*3600*1000
	newTs := now - 1*24*3600*1000

	insert := func(tenantID string, ts int64, v float64) {
		ctx := tenant.With(context.Background(), tenant.Context{TenantID: tenantID})
		if err := c.Insert(ctx, device.Id, map[string]interface{}{
			"temp": dtos.ReportData{Value: v, Time: ts},
		}); err != nil {
			t.Fatalf("insert %s: %v", tenantID, err)
		}
	}
	insert("t-aaa", oldTs, 1)
	insert("t-aaa", newTs, 2)
	insert("t-bbb", oldTs, 3) // 另一个租户的老数据，不应被删

	c.flushPending()
	cutoff := now - 30*24*3600*1000
	deleted, err := c.PurgeTenantBefore(context.Background(), "t-aaa", cutoff)
	if err != nil {
		t.Fatalf("purge tenant: %v", err)
	}
	if deleted != 1 {
		t.Fatalf("t-aaa 删除 %d 行, want 1", deleted)
	}

	countTenant := func(tenantID string) int {
		var n int
		if err := c.db.QueryRow("SELECT count(*) FROM "+tableProperty+" WHERE tenant_id = ?", tenantID).Scan(&n); err != nil {
			t.Fatalf("count %s: %v", tenantID, err)
		}
		return n
	}
	if got := countTenant("t-aaa"); got != 1 {
		t.Fatalf("t-aaa 剩余 %d 行, want 1", got)
	}
	if got := countTenant("t-bbb"); got != 1 {
		t.Fatalf("t-bbb 应不受影响，剩余 %d 行, want 1", got)
	}
	if _, err := c.PurgeTenantBefore(context.Background(), "", cutoff); err == nil {
		t.Fatalf("空租户 ID 应报错，避免误删全表")
	}
}

// TestCountAllMsgByTimeRange 平台级计数应一条 SQL 完成，且可按租户过滤
func TestCountAllMsgByTimeRange(t *testing.T) {
	c := newTestClient(t)
	now := time.Now().UnixNano() / int64(time.Millisecond)
	base := now - 60_000

	insert := func(tenantID, deviceID string) {
		ctx := context.Background()
		if tenantID != "" {
			ctx = tenant.With(ctx, tenant.Context{TenantID: tenantID})
		}
		if err := c.Insert(ctx, deviceID, map[string]interface{}{
			"temp": dtos.ReportData{Value: 1.0, Time: base},
		}); err != nil {
			t.Fatalf("insert: %v", err)
		}
	}
	insert("t-aaa", "dev-1")
	insert("t-aaa", "dev-2")
	insert("t-bbb", "dev-3")
	c.flushPending()

	all, err := c.CountAllMsgByTimeRange("", base-1000, base+1000)
	if err != nil || all != 3 {
		t.Fatalf("全平台计数 = %d, %v; want 3", all, err)
	}
	onlyA, err := c.CountAllMsgByTimeRange("t-aaa", base-1000, base+1000)
	if err != nil || onlyA != 2 {
		t.Fatalf("租户 t-aaa 计数 = %d, %v; want 2", onlyA, err)
	}
}

// seedAggRows 写入指定时间点的数值行
func seedAggRows(t *testing.T, c *Client, deviceID string, rows []struct {
	ts int64
	v  float64
}) {
	t.Helper()
	ctx := context.Background()
	for _, r := range rows {
		if err := c.Insert(ctx, deviceID, map[string]interface{}{
			"temp": dtos.ReportData{Value: r.v, Time: r.ts},
		}); err != nil {
			t.Fatalf("insert agg row: %v", err)
		}
	}
}

// TestAggregateProperty 聚合查询：分桶、各聚合方法、桶内均值正确性
func TestAggregateProperty(t *testing.T) {
	c := newTestClient(t)
	device := models.Device{Id: "dev-agg"}
	base := int64(1_700_000_000_000)
	base = base - base%60_000 // 对齐到分钟

	rows := []struct {
		ts int64
		v  float64
	}{
		{base + 1000, 10}, {base + 2000, 20}, {base + 3000, 30},
		{base + 61000, 100}, {base + 62000, 200},
	}
	seedAggRows(t, c, device.Id, rows)

	baseReq := dtos.ThingModelPropertyDataRequest{
		DeviceId: device.Id,
		Code:     "temp",
		ThingModelDataBaseRequest: dtos.ThingModelDataBaseRequest{
			Range: []int64{base - 1000, base + 120_000},
		},
		BaseSearchConditionQuery: dtos.BaseSearchConditionQuery{Page: 1, PageSize: 10},
	}

	// 不聚合 → 5 条原始数据
	raw, rawTotal, err := c.GetDeviceProperty(baseReq, device)
	if err != nil {
		t.Fatalf("raw query: %v", err)
	}
	if rawTotal != 5 || len(raw) != 5 {
		t.Fatalf("raw count=%d len=%d, want 5/5", rawTotal, len(raw))
	}

	// 按 60 秒分桶 avg：桶 1=(10+20+30)/3=20，桶 2=(100+200)/2=150
	aggReq := baseReq
	aggReq.AggInterval = 60
	aggReq.AggMethod = "avg"
	agg, total, err := c.GetDeviceProperty(aggReq, device)
	if err != nil {
		t.Fatalf("aggregate avg: %v", err)
	}
	if total != 2 || len(agg) != 2 {
		t.Fatalf("avg 桶数 total=%d len=%d, want 2/2", total, len(agg))
	}
	// 结果按桶时间倒序：agg[0] 是桶 2
	if v, ok := agg[0].Value.(float64); !ok || v != 150 {
		t.Fatalf("桶 2 avg = %v, want 150", agg[0].Value)
	}
	if v, ok := agg[1].Value.(float64); !ok || v != 20 {
		t.Fatalf("桶 1 avg = %v, want 20", agg[1].Value)
	}

	// 各聚合方法
	for _, tc := range []struct {
		method string
		bucket int // 1 → 对应 agg[1]
		want   float64
	}{
		{"sum", 1, 60},
		{"min", 1, 10},
		{"max", 1, 30},
		{"count", 1, 3},
		{"sum", 0, 300},
		{"count", 0, 2},
	} {
		r := baseReq
		r.AggInterval = 60
		r.AggMethod = tc.method
		got, _, err := c.GetDeviceProperty(r, device)
		if err != nil {
			t.Fatalf("aggregate %s: %v", tc.method, err)
		}
		v, ok := got[tc.bucket].Value.(float64)
		if !ok || v != tc.want {
			t.Fatalf("%s 桶[%d] = %v, want %v", tc.method, tc.bucket, got[tc.bucket].Value, tc.want)
		}
	}
}

// TestAggregateTextProperty first/last 应保留文本类型，不被数值化
func TestAggregateTextProperty(t *testing.T) {
	c := newTestClient(t)
	ctx := context.Background()
	device := models.Device{Id: "dev-agg-text"}
	base := int64(1_700_000_000_000)
	base = base - base%60_000

	for _, item := range []struct {
		ts int64
		v  string
	}{
		{base + 1000, "auto"},
		{base + 2000, "manual"},
	} {
		if err := c.Insert(ctx, device.Id, map[string]interface{}{
			"mode": dtos.ReportData{Value: item.v, Time: item.ts},
		}); err != nil {
			t.Fatalf("insert: %v", err)
		}
	}

	req := dtos.ThingModelPropertyDataRequest{
		DeviceId: device.Id,
		Code:     "mode",
		ThingModelDataBaseRequest: dtos.ThingModelDataBaseRequest{
			Range: []int64{base - 1000, base + 120_000},
		},
		BaseSearchConditionQuery: dtos.BaseSearchConditionQuery{Page: 1, PageSize: 10},
		AggInterval:              60,
		AggMethod:                "first",
	}
	got, total, err := c.GetDeviceProperty(req, device)
	if err != nil {
		t.Fatalf("aggregate first: %v", err)
	}
	if total != 1 || len(got) != 1 {
		t.Fatalf("total=%d len=%d, want 1/1", total, len(got))
	}
	if got[0].Value != "auto" {
		t.Fatalf("first = %v, want auto（文本不应被数值化）", got[0].Value)
	}

	req.AggMethod = "last"
	got, _, err = c.GetDeviceProperty(req, device)
	if err != nil {
		t.Fatalf("aggregate last: %v", err)
	}
	if got[0].Value != "manual" {
		t.Fatalf("last = %v, want manual", got[0].Value)
	}
}

// TestLastAndPagination 最新值查询与分页
func TestLastAndPagination(t *testing.T) {
	c := newTestClient(t)
	device := models.Device{Id: "dev-last"}
	now := time.Now().UnixNano() / int64(time.Millisecond)
	base := now - 60_000

	for i := 0; i < 5; i++ {
		if err := c.Insert(context.Background(), device.Id, map[string]interface{}{
			"temp": dtos.ReportData{Value: float64(i), Time: base + int64(i)*1000},
		}); err != nil {
			t.Fatalf("insert: %v", err)
		}
	}

	// Last：取最近 30 分钟窗口内的最新一条
	lastReq := dtos.ThingModelPropertyDataRequest{
		DeviceId: device.Id,
		Code:     "temp",
		ThingModelDataBaseRequest: dtos.ThingModelDataBaseRequest{
			Last: true,
		},
		BaseSearchConditionQuery: dtos.BaseSearchConditionQuery{Page: 1, PageSize: 1},
	}
	got, _, err := c.GetDeviceProperty(lastReq, device)
	if err != nil {
		t.Fatalf("last query: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("last len=%d, want 1", len(got))
	}
	if v, ok := got[0].Value.(float64); !ok || v != 4 {
		t.Fatalf("last = %v, want 4", got[0].Value)
	}

	// 分页：第 2 页取 2 条
	pageReq := dtos.ThingModelPropertyDataRequest{
		DeviceId: device.Id,
		Code:     "temp",
		ThingModelDataBaseRequest: dtos.ThingModelDataBaseRequest{
			Range: []int64{base - 1000, now + 1000},
		},
		BaseSearchConditionQuery: dtos.BaseSearchConditionQuery{Page: 2, PageSize: 2},
	}
	page, total, err := c.GetDeviceProperty(pageReq, device)
	if err != nil {
		t.Fatalf("page query: %v", err)
	}
	if total != 5 || len(page) != 2 {
		t.Fatalf("page2 total=%d len=%d, want 5/2", total, len(page))
	}
	// 倒序第 2 页应为 value 2,1
	if v, _ := page[0].Value.(float64); v != 2 {
		t.Fatalf("page2[0] = %v, want 2", page[0].Value)
	}
}

// TestConcurrentWriteAndQuery 并发写入（多个 goroutine 同时触发 flushPending）
// 与并发查询下：不得报错、不得丢数据。
func TestConcurrentWriteAndQuery(t *testing.T) {
	c := newTestClient(t)
	// 调小阈值，让多个 goroutine 频繁同时进入 flushPending
	c.mu.Lock()
	c.flushRows = 50
	c.mu.Unlock()

	const writers = 16
	const perWriter = 200
	base := time.Now().Add(-time.Hour).UnixNano() / int64(time.Millisecond)

	var wg sync.WaitGroup
	errCh := make(chan error, writers*perWriter)
	for w := 0; w < writers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			deviceID := fmt.Sprintf("dev-conc-%02d", w)
			for i := 0; i < perWriter; i++ {
				err := c.Insert(context.Background(), deviceID, map[string]interface{}{
					"temp": dtos.ReportData{Value: float64(i), Time: base + int64(i)*10},
				})
				if err != nil {
					errCh <- fmt.Errorf("writer %d insert %d: %w", w, i, err)
					return
				}
			}
		}(w)
	}

	// 同时有读者在查
	stop := make(chan struct{})
	var readerWG sync.WaitGroup
	readerWG.Add(1)
	go func() {
		defer readerWG.Done()
		for {
			select {
			case <-stop:
				return
			default:
			}
			_, _, _ = c.GetDeviceProperty(dtos.ThingModelPropertyDataRequest{
				DeviceId: "dev-conc-00",
				Code:     "temp",
				ThingModelDataBaseRequest: dtos.ThingModelDataBaseRequest{
					Range: []int64{base - 1000, base + 100000},
				},
				BaseSearchConditionQuery: dtos.BaseSearchConditionQuery{Page: 1, PageSize: 10},
			}, models.Device{Id: "dev-conc-00"})
			time.Sleep(time.Millisecond)
		}
	}()

	wg.Wait()
	close(stop)
	readerWG.Wait()
	close(errCh)
	for err := range errCh {
		t.Fatalf("并发写入报错: %v", err)
	}

	// 校验：每个设备 200 行都要在
	total := 0
	for w := 0; w < writers; w++ {
		deviceID := fmt.Sprintf("dev-conc-%02d", w)
		n, err := c.GetDevicePropertyCount(dtos.ThingModelPropertyDataRequest{
			DeviceId: deviceID,
			Code:     "temp",
			ThingModelDataBaseRequest: dtos.ThingModelDataBaseRequest{
				Range: []int64{base - 1000, base + 100000},
			},
		})
		if err != nil {
			t.Fatalf("count %s: %v", deviceID, err)
		}
		if n != perWriter {
			t.Errorf("%s 写入 %d 行，实际 %d 行（并发下丢数据）", deviceID, perWriter, n)
		}
		total += n
	}
	t.Logf("并发写入合计 %d 行（期望 %d）", total, writers*perWriter)
}

// TestAggregateBucketPagination 聚合结果按「桶」分页：总数应为桶数，
// 逐页翻完不重不漏（改动前为了拿总数会把 GROUP BY 跑两遍，分页语义不能变）。
func TestAggregateBucketPagination(t *testing.T) {
	c := newTestClient(t)
	device := models.Device{Id: "dev-agg-page"}
	base := int64(1_700_000_000_000)
	base = base - base%60_000 // 对齐到分钟

	const buckets = 5
	const perBucket = 4
	ctx := context.Background()
	for b := 0; b < buckets; b++ {
		for i := 0; i < perBucket; i++ {
			if err := c.Insert(ctx, device.Id, map[string]interface{}{
				"temp": dtos.ReportData{
					Value: float64(b*10 + i),
					Time:  base + int64(b)*60_000 + int64(i)*1000,
				},
			}); err != nil {
				t.Fatalf("insert: %v", err)
			}
		}
	}

	newReq := func(page, pageSize int) dtos.ThingModelPropertyDataRequest {
		return dtos.ThingModelPropertyDataRequest{
			DeviceId: device.Id,
			Code:     "temp",
			ThingModelDataBaseRequest: dtos.ThingModelDataBaseRequest{
				Range: []int64{base - 1000, base + int64(buckets)*60_000},
			},
			BaseSearchConditionQuery: dtos.BaseSearchConditionQuery{Page: page, PageSize: pageSize},
			AggInterval:              60,
			AggMethod:                "count",
		}
	}

	seen := map[int64]bool{}
	totalSeen := 0
	for page := 1; page <= buckets; page++ {
		rows, total, err := c.GetDeviceProperty(newReq(page, 2), device)
		if err != nil {
			t.Fatalf("page %d: %v", page, err)
		}
		if total != buckets {
			t.Fatalf("第 %d 页 total=%d, want %d（总数应为桶数）", page, total, buckets)
		}
		for _, r := range rows {
			if seen[r.Time] {
				t.Fatalf("桶 %d 在分页中重复出现", r.Time)
			}
			seen[r.Time] = true
			totalSeen++
			if v, ok := r.Value.(float64); !ok || v != perBucket {
				t.Fatalf("桶 %d count=%v, want %d", r.Time, r.Value, perBucket)
			}
		}
	}
	if totalSeen != buckets {
		t.Fatalf("翻完所有页共 %d 个桶, want %d", totalSeen, buckets)
	}

	// IsAll：单页取回全部桶
	allReq := newReq(1, 0)
	allReq.IsAll = true
	rows, total, err := c.GetDeviceProperty(allReq, device)
	if err != nil {
		t.Fatalf("IsAll: %v", err)
	}
	if total != buckets || len(rows) != buckets {
		t.Fatalf("IsAll total=%d len=%d, want %d/%d", total, len(rows), buckets, buckets)
	}
	// 桶按时间倒序
	for i := 1; i < len(rows); i++ {
		if rows[i-1].Time <= rows[i].Time {
			t.Fatalf("桶未按时间倒序: %v", rows)
		}
	}
}

// TestBufferCapRejectsWrites 缓冲到顶后拒绝新写入（宁可明确报错，也不静默涨到 OOM）
func TestBufferCapRejectsWrites(t *testing.T) {
	c := newTestClient(t)
	c.mu.Lock()
	c.flushRows = 1_000_000 // 关掉行数触发的自动落盘，让数据留在缓冲里
	c.maxBufferRows = 3
	c.mu.Unlock()

	now := time.Now().UnixMilli()
	for i := 0; i < 3; i++ {
		if err := c.Insert(context.Background(), "dev-cap", map[string]interface{}{
			"temp": dtos.ReportData{Value: float64(i), Time: now},
		}); err != nil {
			t.Fatalf("第 %d 行不应被拒绝: %v", i+1, err)
		}
	}
	err := c.Insert(context.Background(), "dev-cap", map[string]interface{}{
		"temp": dtos.ReportData{Value: 9.0, Time: now},
	})
	if !errors.Is(err, errBufferFull) {
		t.Fatalf("缓冲已满应返回 errBufferFull, got %v", err)
	}
}

// TestFlushFailureBackoff 落盘失败后进入退避：定时落盘跳过、缓冲保留；
// 恢复后强制落盘能把缓冲写回，数据不丢。
func TestFlushFailureBackoff(t *testing.T) {
	c := newTestClient(t)
	ctx := context.Background()
	c.mu.Lock()
	c.flushRows = 2 // 攒 2 行就落盘，便于触发失败
	c.mu.Unlock()

	// 制造持续失败：删掉属性表，INSERT 必然报错
	if _, err := c.db.Exec("DROP TABLE " + tableProperty); err != nil {
		t.Fatalf("drop table: %v", err)
	}

	now := time.Now().UnixMilli()
	for i := 0; i < 4; i++ {
		if err := c.Insert(ctx, "dev-fail", map[string]interface{}{
			"temp": dtos.ReportData{Value: float64(i), Time: now + int64(i)},
		}); err != nil {
			t.Fatalf("insert %d: %v", i, err)
		}
	}

	c.mu.Lock()
	buffered := c.bufferedLocked()
	streak := c.failStreak
	backoffActive := !c.nextFlushAt.IsZero() && time.Now().Before(c.nextFlushAt)
	c.mu.Unlock()
	if streak == 0 || !backoffActive {
		t.Fatalf("落盘失败后应进入退避: streak=%d backoffActive=%v", streak, backoffActive)
	}
	if buffered == 0 {
		t.Fatalf("失败的行应保留在缓冲里，实际缓冲为 0（数据丢了）")
	}

	// 退避窗口内的定时落盘应被跳过（缓冲不变）
	c.flushIdle()
	c.mu.Lock()
	afterIdle := c.bufferedLocked()
	c.mu.Unlock()
	if afterIdle != buffered {
		t.Fatalf("退避窗口内 flushIdle 不应改变缓冲: %d -> %d", buffered, afterIdle)
	}

	// 恢复表结构，强制落盘应把缓冲写回
	if err := c.ensureTables(ctx); err != nil {
		t.Fatalf("recreate tables: %v", err)
	}
	c.flushPending()

	c.mu.Lock()
	left := c.bufferedLocked()
	c.mu.Unlock()
	if left != 0 {
		t.Fatalf("恢复后缓冲应清空，实际剩 %d 行", left)
	}
	n, err := c.GetDevicePropertyCount(dtos.ThingModelPropertyDataRequest{
		DeviceId: "dev-fail",
		Code:     "temp",
		ThingModelDataBaseRequest: dtos.ThingModelDataBaseRequest{
			Range: []int64{now - 60_000, now + 60_000},
		},
	})
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 4 {
		t.Fatalf("恢复后应有 4 行，实际 %d（失败期间的数据丢了）", n)
	}
}

// TestAggregateIsAllNotTruncated 聚合路径的 IsAll 不应被 maxPageSize 截断。
// 用超过 maxPageSize 的桶数验证：原始数据路径保留上限，聚合路径返回全部桶。
func TestAggregateIsAllNotTruncated(t *testing.T) {
	c := newTestClient(t)
	device := models.Device{Id: "dev-agg-isall"}
	ctx := context.Background()

	// 造 maxPageSize+200 个分钟桶，每桶 1 行
	const buckets = maxPageSize + 200
	base := int64(1_700_000_000_000)
	base = base - base%60_000

	for i := 0; i < buckets; i++ {
		if err := c.Insert(ctx, device.Id, map[string]interface{}{
			"temp": dtos.ReportData{Value: float64(i % 50), Time: base + int64(i)*60_000},
		}); err != nil {
			t.Fatalf("insert %d: %v", i, err)
		}
	}

	req := dtos.ThingModelPropertyDataRequest{
		DeviceId: device.Id,
		Code:     "temp",
		ThingModelDataBaseRequest: dtos.ThingModelDataBaseRequest{
			Range: []int64{base - 1000, base + int64(buckets+1)*60_000},
		},
		BaseSearchConditionQuery: dtos.BaseSearchConditionQuery{Page: 1, PageSize: 10, IsAll: true},
		AggInterval:              60,
		AggMethod:                "count",
	}
	rows, total, err := c.GetDeviceProperty(req, device)
	if err != nil {
		t.Fatalf("IsAll 聚合查询: %v", err)
	}
	if total != buckets {
		t.Fatalf("total=%d want %d", total, buckets)
	}
	if len(rows) != buckets {
		t.Fatalf("IsAll 聚合返回 %d 个桶, want %d（被 maxPageSize=%d 截断了）",
			len(rows), buckets, maxPageSize)
	}
}

// TestExportDatabaseOnline 在线导出：不停止数据层即可拿到一致快照，
// 导出的 Parquet + schema.sql 能被重新读回（备份可恢复）。
func TestExportDatabaseOnline(t *testing.T) {
	c := newTestClient(t)
	ctx := context.Background()
	device := models.Device{Id: "dev-backup"}
	now := time.Now().UnixMilli()

	for i := 0; i < 5; i++ {
		if err := c.Insert(ctx, device.Id, map[string]interface{}{
			"temp": dtos.ReportData{Value: float64(i), Time: now + int64(i)*1000},
		}); err != nil {
			t.Fatalf("insert: %v", err)
		}
	}

	dir := filepath.Join(t.TempDir(), "backup")
	if err := c.ExportDatabase(ctx, dir); err != nil {
		t.Fatalf("export: %v", err)
	}

	// 目录下应有 schema.sql 与属性表的 Parquet
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	var hasSchema, hasParquet bool
	for _, e := range entries {
		if e.Name() == "schema.sql" {
			hasSchema = true
		}
		if strings.HasPrefix(e.Name(), tableProperty) && strings.HasSuffix(e.Name(), ".parquet") {
			hasParquet = true
		}
	}
	if !hasSchema || !hasParquet {
		t.Fatalf("导出目录内容不完整: schema=%v parquet=%v, entries=%v", hasSchema, hasParquet, entries)
	}

	// 数据层仍可继续写入（在线导出不应影响服务）
	if err := c.Insert(ctx, device.Id, map[string]interface{}{
		"temp": dtos.ReportData{Value: 99.0, Time: now + 60_000},
	}); err != nil {
		t.Fatalf("导出后写入失败: %v", err)
	}

	// 导出的快照可被独立读回（恢复可行性）
	tmp, err := sql.Open("duckdb", "")
	if err != nil {
		t.Fatalf("open tmp: %v", err)
	}
	defer tmp.Close()
	if _, err := tmp.Exec(fmt.Sprintf("IMPORT DATABASE %s", quote(dir))); err != nil {
		t.Fatalf("import: %v", err)
	}
	var n int
	if err := tmp.QueryRow("SELECT count(*) FROM " + tableProperty).Scan(&n); err != nil {
		t.Fatalf("count imported: %v", err)
	}
	if n != 5 {
		t.Fatalf("导入快照应有 5 行, 实际 %d", n)
	}

	if err := c.ExportDatabase(ctx, "   "); err == nil {
		t.Fatalf("空目录应报错")
	}
}
