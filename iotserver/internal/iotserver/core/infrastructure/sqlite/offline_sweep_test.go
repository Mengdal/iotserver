package sqlite

import (
	"testing"
	"time"

	"iotserver/internal/models"
	"iotserver/internal/pkg/constants"
)

// TestMarkStaleDevicesOffline 超时扫描只应把「在线且已超时」的设备置为离线。
func TestMarkStaleDevicesOffline(t *testing.T) {
	c := newTestClient(t)
	if err := c.MigrateSystemTables(); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	now := time.Now().UnixMilli()
	stale := now - 10*60*1000 // 10 分钟前
	fresh := now - 30*1000    // 30 秒前

	insert := func(id, status string, lastOnline int64) {
		if err := c.Pool.Table("device").Create(map[string]interface{}{
			"id": id, "name": id, "device_sn": "SN-" + id, "tenant_id": "",
			"status": status, "last_online_time": lastOnline,
			"created": now, "modified": now,
		}).Error; err != nil {
			t.Fatalf("insert %s: %v", id, err)
		}
	}
	insert("dev-stale", string(constants.DeviceStatusOnline), stale)
	insert("dev-fresh", string(constants.DeviceStatusOnline), fresh)
	insert("dev-already-offline", string(constants.DeviceStatusOffline), stale)
	insert("dev-never", string(constants.DeviceStatusOnline), 0)
	insert("dev-unknown", string(constants.DeviceStatusUnKnow), stale)

	cutoff := now - 5*60*1000
	n, err := c.MarkStaleDevicesOffline(cutoff)
	if err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if n != 1 {
		t.Fatalf("应只置 1 台设备离线，实际 %d", n)
	}

	statusOf := func(id string) string {
		var d models.Device
		if err := c.Pool.Table("device").Where("id = ?", id).First(&d).Error; err != nil {
			t.Fatalf("query %s: %v", id, err)
		}
		return string(d.Status)
	}
	if got := statusOf("dev-stale"); got != string(constants.DeviceStatusOffline) {
		t.Fatalf("dev-stale 应离线，实际 %q", got)
	}
	if got := statusOf("dev-fresh"); got != string(constants.DeviceStatusOnline) {
		t.Fatalf("dev-fresh 应保持在线，实际 %q", got)
	}
	if got := statusOf("dev-already-offline"); got != string(constants.DeviceStatusOffline) {
		t.Fatalf("已离线的设备不应再被改动，实际 %q", got)
	}
	// 从未上过线（last_online_time=0）不应被当成超时离线
	if got := statusOf("dev-never"); got != string(constants.DeviceStatusOnline) {
		t.Fatalf("last_online_time=0 的设备不应被改动，实际 %q", got)
	}
	if got := statusOf("dev-unknown"); got != string(constants.DeviceStatusUnKnow) {
		t.Fatalf("非在线状态不应被改动，实际 %q", got)
	}

	// 幂等：再扫一次不应再有变更
	if n, err := c.MarkStaleDevicesOffline(cutoff); err != nil || n != 0 {
		t.Fatalf("重复扫描应无变更，n=%d err=%v", n, err)
	}
	// cutoff<=0 属参数异常，应直接返回 0 不做任何修改
	if n, err := c.MarkStaleDevicesOffline(0); err != nil || n != 0 {
		t.Fatalf("cutoff=0 应返回 0，n=%d err=%v", n, err)
	}
}

// TestMarkStaleGatewaysOffline 网关侧同规则，依据 last_online_at
func TestMarkStaleGatewaysOffline(t *testing.T) {
	c := newTestClient(t)
	if err := c.MigrateSystemTables(); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	now := time.Now().UnixMilli()
	stale := now - 10*60*1000
	fresh := now - 30*1000

	insert := func(id string, status int, lastOnline int64) {
		if err := c.Pool.Table(models.TableGateway).Create(map[string]interface{}{
			"id": id, "name": id, "sn": "SN-" + id, "tenant_id": "",
			"status": status, "last_online_at": lastOnline,
			"created": now, "modified": now,
		}).Error; err != nil {
			t.Fatalf("insert %s: %v", id, err)
		}
	}
	insert("gw-stale", models.GatewayStatusOnline, stale)
	insert("gw-fresh", models.GatewayStatusOnline, fresh)
	insert("gw-never", models.GatewayStatusNever, stale)

	cutoff := now - 5*60*1000
	n, err := c.MarkStaleGatewaysOffline(cutoff)
	if err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if n != 1 {
		t.Fatalf("应只置 1 个网关离线，实际 %d", n)
	}

	var g models.Gateway
	if err := c.Pool.Table(models.TableGateway).Where("id = ?", "gw-stale").First(&g).Error; err != nil {
		t.Fatalf("query: %v", err)
	}
	if g.Status != models.GatewayStatusOffline {
		t.Fatalf("gw-stale 应离线，实际 %d", g.Status)
	}
	var fresh2 models.Gateway
	if err := c.Pool.Table(models.TableGateway).Where("id = ?", "gw-fresh").First(&fresh2).Error; err != nil {
		t.Fatalf("query: %v", err)
	}
	if fresh2.Status != models.GatewayStatusOnline {
		t.Fatalf("gw-fresh 应保持在线，实际 %d", fresh2.Status)
	}
}
