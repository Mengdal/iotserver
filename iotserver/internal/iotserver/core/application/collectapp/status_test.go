package collectapp

import (
	"testing"

	interfaces "iotserver/internal/iotserver/core/interface"
	"iotserver/internal/models"
	"iotserver/internal/tools/collectengine"
)

type fakeStatusDB struct {
	interfaces.DBClient
	updates []int
}

func (f *fakeStatusDB) UpdateGatewayStatus(_ string, status int, _ string, _ int64) error {
	f.updates = append(f.updates, status)
	return nil
}

func TestComputeSoftGatewayStatus(t *testing.T) {
	now := int64(1_700_000_000_000)
	configs := []models.CollectDeviceConfig{
		{DeviceId: "100", ChannelId: "1", GatewayId: "g1", Enabled: true},
		{DeviceId: "200", ChannelId: "1", GatewayId: "g1", Enabled: false},
	}
	points := map[string][]models.CollectPointConfig{
		"100": {{DeviceId: "100", TagId: "t1", Enabled: true}},
		"200": {{DeviceId: "200", TagId: "t2", Enabled: true}},
	}
	poll := map[int64]int64{1: 5000}

	records := map[string]collectengine.TagRealtimeRecord{
		"100.t1": {Id: "100.t1", Status: float64(192), Timestamp: now - 1000},
		"200.t2": {Id: "200.t2", Status: float64(192), Timestamp: now - 1000},
	}
	online, devices := computeSoftGatewayStatus(now, configs, points, poll, records)
	if !online || !devices["100"] {
		t.Fatalf("新鲜数据应判在线: gateway=%v devices=%v", online, devices)
	}
	if devices["200"] {
		t.Fatalf("禁用设备不应在线: %v", devices)
	}

	// 超过 3×轮询周期（且不少于 15 秒）
	records["100.t1"] = collectengine.TagRealtimeRecord{Id: "100.t1", Status: float64(192), Timestamp: now - 20_000}
	online, devices = computeSoftGatewayStatus(now, configs, points, poll, records)
	if online || devices["100"] {
		t.Fatalf("过期数据应判离线: gateway=%v devices=%v", online, devices)
	}

	// 引擎状态 Error
	records["100.t1"] = collectengine.TagRealtimeRecord{Id: "100.t1", Status: float64(0), Timestamp: now - 1000}
	online, devices = computeSoftGatewayStatus(now, configs, points, poll, records)
	if online || devices["100"] {
		t.Fatalf("Error 状态应判离线: gateway=%v devices=%v", online, devices)
	}
}

func TestSoftGatewayProbeFailureDebounce(t *testing.T) {
	db := &fakeStatusDB{}
	s := &service{statusFailures: make(map[string]int)}
	gateway := models.Gateway{Id: "g1", Sn: "1234567890", Status: models.GatewayStatusOnline}
	configs := []models.CollectDeviceConfig{{DeviceId: "100", GatewayId: "g1", Enabled: true}}
	points := map[string][]models.CollectPointConfig{
		"100": {{DeviceId: "100", TagId: "t1", Enabled: true}},
	}
	devices := map[int64]models.Device{}

	// 单次失败不应立即判离线。
	for i := 1; i < softStatusOfflineFailures; i++ {
		s.noteSoftGatewayProbeFailure(db, gateway, configs, points, devices, 0)
	}
	if len(db.updates) != 0 {
		t.Fatalf("失败 %d 次就判离线: %v", softStatusOfflineFailures-1, db.updates)
	}
	// 连续失败达到阈值才离线。
	s.noteSoftGatewayProbeFailure(db, gateway, configs, points, devices, 0)
	if len(db.updates) != 1 || db.updates[0] != models.GatewayStatusOffline {
		t.Fatalf("达到阈值后状态更新 = %v", db.updates)
	}
	// 探测成功后计数清零，下一次失败重新计数。
	delete(s.statusFailures, gateway.Id)
	s.noteSoftGatewayProbeFailure(db, gateway, configs, points, devices, 0)
	if len(db.updates) != 1 {
		t.Fatalf("成功后计数未清零: %v", db.updates)
	}
}

func TestSoftGatewayNeverStaysNever(t *testing.T) {
	db := &fakeStatusDB{}
	s := &service{statusFailures: make(map[string]int)}
	gateway := models.Gateway{Id: "g2", Sn: "1234567891", Status: models.GatewayStatusNever}
	configs := []models.CollectDeviceConfig{{DeviceId: "100", GatewayId: "g2", Enabled: true}}
	points := map[string][]models.CollectPointConfig{
		"100": {{DeviceId: "100", TagId: "t1", Enabled: true}},
	}
	for i := 0; i <= softStatusOfflineFailures; i++ {
		s.noteSoftGatewayProbeFailure(db, gateway, configs, points, map[int64]models.Device{}, 0)
	}
	if len(db.updates) != 0 {
		t.Fatalf("从未上线的网关不应被改成离线: %v", db.updates)
	}
}

func TestHasEvaluableCollect(t *testing.T) {
	points := map[string][]models.CollectPointConfig{
		"100": {{DeviceId: "100", Enabled: false}},
		"200": {{DeviceId: "200", Enabled: true}},
	}
	if hasEvaluableCollect([]models.CollectDeviceConfig{{DeviceId: "100", Enabled: true}}, points) {
		t.Fatal("点位全部禁用时不应判定为可评估")
	}
	if !hasEvaluableCollect([]models.CollectDeviceConfig{{DeviceId: "200", Enabled: true}}, points) {
		t.Fatal("启用点位应判定为可评估")
	}
	if hasEvaluableCollect([]models.CollectDeviceConfig{{DeviceId: "200", Enabled: false}}, points) {
		t.Fatal("设备禁用时不应判定为可评估")
	}
}
