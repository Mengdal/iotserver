package gatewayingest

import (
	"context"
	"encoding/json"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/LMgateways/iotserver-driver-proto/drivercommon"

	"iotserver/internal/dtos"
	"iotserver/internal/iotserver/core/infrastructure/sqlite"
	interfaces "iotserver/internal/iotserver/core/interface"
	"iotserver/internal/models"
	"iotserver/internal/pkg/constants"
	"iotserver/internal/pkg/logger"
	"iotserver/internal/pkg/tenant"
	"iotserver/internal/tools/gatewaymqtt"
)

// fakeMessageItf 捕获接入服务交给平台上行管道的物模型消息
type fakeMessageItf struct {
	interfaces.MessageItf
	msgs []dtos.ThingModelMessage
}

func (f *fakeMessageItf) ThingModelMsgReport(_ context.Context, msg dtos.ThingModelMessage) (*drivercommon.CommonResponse, error) {
	f.msgs = append(f.msgs, msg)
	return &drivercommon.CommonResponse{Success: true}, nil
}

// fakeGatewayItf 记录在线状态更新
type fakeGatewayItf struct {
	interfaces.GatewayItf
	online  int
	offline int
}

func (f *fakeGatewayItf) GatewayOnline(_ context.Context, _, _ string, _ int64) error {
	f.online++
	return nil
}

func (f *fakeGatewayItf) GatewayOffline(_ context.Context, _ string) error {
	f.offline++
	return nil
}

type ingestFixture struct {
	svc      *Service
	db       *sqlite.Client
	messages *fakeMessageItf
	gateways *fakeGatewayItf
	tenantID string
	gateway  models.Gateway
	device   models.Device
}

func newIngestFixture(t *testing.T) *ingestFixture {
	t.Helper()
	lc := logger.NewClient("gateway-ingest-test", "ERROR", "")
	db, err := sqlite.NewClient(dtos.Configuration{DataSource: filepath.Join(t.TempDir(), "core.db")}, lc)
	if err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	t.Cleanup(db.CloseSession)
	if err := db.MigrateSystemTables(); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	pctx := tenant.Platform(context.Background())
	tc, err := db.WithCtx(pctx).AddTenant(models.Tenant{Code: "gw_tenant", Name: "网关测试租户"})
	if err != nil {
		t.Fatalf("create tenant: %v", err)
	}
	tctx := tenant.With(pctx, tenant.Context{TenantID: tc.Id})

	gw, err := db.WithCtx(tctx).AddGateway(models.Gateway{Name: "网关1", Sn: "GW-0001"})
	if err != nil {
		t.Fatalf("create gateway: %v", err)
	}

	// 设备必须挂在产品下
	product, err := db.WithCtx(tctx).AddProduct(models.Product{Name: "注塑机", Key: "inj_machine"})
	if err != nil {
		t.Fatalf("create product: %v", err)
	}

	// device_key = 网关内设备标识；tenant_id 由租户上下文自动写入
	device := models.Device{
		Name:      "注塑机1",
		DeviceSn:  "M-0001",
		DeviceKey: "dev-1",
		GatewayId: gw.Id,
		ProductId: product.Id,
	}
	deviceID, err := db.WithCtx(tctx).AddDevice(device)
	if err != nil {
		t.Fatalf("create device: %v", err)
	}
	device.Id = deviceID

	messages := &fakeMessageItf{}
	gateways := &fakeGatewayItf{}
	svc := &Service{
		lc:         lc,
		dbClient:   db,
		messageItf: messages,
		gatewayItf: gateways,
	}
	return &ingestFixture{
		svc: svc, db: db, messages: messages, gateways: gateways,
		tenantID: tc.Id, gateway: gw, device: device,
	}
}

// TestParseTopic 主题解析
func TestParseTopic(t *testing.T) {
	cases := []struct {
		topic string
		sn    string
		kind  string
		ok    bool
	}{
		{"iot/up/GW-1/data", "GW-1", "data", true},
		{"iot/up/GW-1/batch", "GW-1", "batch", true},
		{"iot/up/GW-1/status", "GW-1", "status", true},
		{"iot/down/GW-1/config", "", "", false},
		{"iot/up/GW-1", "", "", false},
		{"other/topic", "", "", false},
	}
	for _, c := range cases {
		sn, kind, ok := ParseTopic(c.topic)
		if ok != c.ok || sn != c.sn || kind != c.kind {
			t.Fatalf("ParseTopic(%s) = (%s,%s,%v)，期望 (%s,%s,%v)", c.topic, sn, kind, ok, c.sn, c.kind, c.ok)
		}
	}
}

// TestIngestRealtimeData 实时上报：解析 → 映射设备 → 交给上行管道，且保留测点时间戳
func TestIngestRealtimeData(t *testing.T) {
	f := newIngestFixture(t)

	ts := time.Now().Add(-2 * time.Hour).UnixMilli() // 模拟网关缓存后补发
	payload, _ := json.Marshal(map[string]interface{}{
		"seq":       1001,
		"ts":        ts,
		"deviceKey": "dev-1",
		"properties": map[string]interface{}{
			"temp": map[string]interface{}{"value": 23.5, "time": ts},
			"rpm":  map[string]interface{}{"value": 1450},
		},
	})
	if err := f.svc.HandleMessage(gatewaymqtt.UpTopic("GW-0001", "data"), payload); err != nil {
		t.Fatalf("处理实时上报失败: %v", err)
	}
	if len(f.messages.msgs) != 1 {
		t.Fatalf("应产生 1 条物模型消息，实际 %d", len(f.messages.msgs))
	}
	msg := f.messages.msgs[0]
	if msg.Cid != f.device.Id {
		t.Fatalf("设备映射错误：期望 %s，实际 %s", f.device.Id, msg.Cid)
	}
	if msg.GetOpType().String() != "DATA_BATCH_REPORT" {
		t.Fatalf("操作类型应为 DATA_BATCH_REPORT，实际 %s", msg.GetOpType().String())
	}

	// 报文里的测点时间必须保留（补传要落到原始采集时刻）
	batch, err := msg.TransformMessageDataByBatchReport()
	if err != nil {
		t.Fatalf("解析批报失败: %v", err)
	}
	if got := batch.Data.Properties["temp"].Time; got != ts {
		t.Fatalf("temp 测点时间被改写：期望 %d，实际 %d", ts, got)
	}
	if got := batch.Data.Properties["rpm"].Time; got != ts {
		t.Fatalf("未带 time 的测点应回落到批次时间 %d，实际 %d", ts, got)
	}

	// 上报即视为在线
	if f.gateways.online == 0 {
		t.Fatal("收到数据后应把网关标记为在线")
	}
}

// TestIngestIdempotentBySeq 断点续传幂等：同一 seq 重发不会重复写入
func TestIngestIdempotentBySeq(t *testing.T) {
	f := newIngestFixture(t)
	ts := time.Now().Add(-time.Hour).UnixMilli()
	payload, _ := json.Marshal(map[string]interface{}{
		"seq":        2001,
		"ts":         ts,
		"deviceKey":  "dev-1",
		"properties": map[string]interface{}{"temp": map[string]interface{}{"value": 20}},
	})
	topic := gatewaymqtt.UpTopic("GW-0001", "data")
	if err := f.svc.HandleMessage(topic, payload); err != nil {
		t.Fatalf("首次上报失败: %v", err)
	}
	if err := f.svc.HandleMessage(topic, payload); err != nil {
		t.Fatalf("重复上报不应报错: %v", err)
	}
	if len(f.messages.msgs) != 1 {
		t.Fatalf("同一 seq 重复上报应只落 1 次，实际 %d", len(f.messages.msgs))
	}
}

// TestIngestBatchBackfill 批量补传：一段历史曲线，逐条带时间
func TestIngestBatchBackfill(t *testing.T) {
	f := newIngestFixture(t)
	base := time.Now().Add(-24 * time.Hour).UnixMilli()
	payload, _ := json.Marshal(map[string]interface{}{
		"seq": 3001,
		"records": []map[string]interface{}{
			{"ts": base, "deviceKey": "dev-1", "properties": map[string]interface{}{
				"temp": map[string]interface{}{"value": 10, "time": base}}},
			{"ts": base + 60000, "deviceKey": "dev-1", "properties": map[string]interface{}{
				"temp": map[string]interface{}{"value": 11, "time": base + 60000}}},
		},
	})
	if err := f.svc.HandleMessage(gatewaymqtt.UpTopic("GW-0001", "batch"), payload); err != nil {
		t.Fatalf("补传失败: %v", err)
	}
	if len(f.messages.msgs) != 2 {
		t.Fatalf("两条补传记录应产生 2 条消息，实际 %d", len(f.messages.msgs))
	}
	for i, msg := range f.messages.msgs {
		batch, err := msg.TransformMessageDataByBatchReport()
		if err != nil {
			t.Fatalf("解析第 %d 条失败: %v", i, err)
		}
		want := base + int64(i)*60000
		if got := batch.Data.Properties["temp"].Time; got != want {
			t.Fatalf("第 %d 条补传时间错误：期望 %d，实际 %d", i, want, got)
		}
	}
}

// TestIngestRejectsUnknownGateway 未注册网关必须被拒绝
func TestIngestRejectsUnknownGateway(t *testing.T) {
	f := newIngestFixture(t)
	payload, _ := json.Marshal(map[string]interface{}{
		"seq": 1, "deviceKey": "dev-1",
		"properties": map[string]interface{}{"temp": map[string]interface{}{"value": 1}},
	})
	err := f.svc.HandleMessage(gatewaymqtt.UpTopic("GW-NOT-EXIST", "data"), payload)
	if err == nil {
		t.Fatal("未注册的网关应被拒绝")
	}
	if len(f.messages.msgs) != 0 {
		t.Fatal("未注册网关不应产生数据")
	}
}

// TestIngestUnknownDeviceIsReported 网关上报未注册的 deviceKey 时给出明确错误
func TestIngestUnknownDeviceIsReported(t *testing.T) {
	f := newIngestFixture(t)
	payload, _ := json.Marshal(map[string]interface{}{
		"seq": 4001, "deviceKey": "not-registered",
		"properties": map[string]interface{}{"temp": map[string]interface{}{"value": 1}},
	})
	if err := f.svc.HandleMessage(gatewaymqtt.UpTopic("GW-0001", "data"), payload); err == nil {
		t.Fatal("未注册的设备应报错")
	}
	// 失败也要留痕，便于对账
	exists, err := f.db.WithCtx(tenant.Platform(context.Background())).GatewayIngestLogExists(f.gateway.Id, 4001)
	if err != nil {
		t.Fatalf("查询上报日志失败: %v", err)
	}
	if !exists {
		t.Fatal("失败的上报也应写入网关上报日志")
	}
}

// TestIngestStatusAndOffline 心跳与离线
func TestIngestStatusAndOffline(t *testing.T) {
	f := newIngestFixture(t)
	online, _ := json.Marshal(map[string]interface{}{"online": true, "ip": "10.0.0.9"})
	if err := f.svc.HandleMessage(gatewaymqtt.UpTopic("GW-0001", "status"), online); err != nil {
		t.Fatalf("心跳处理失败: %v", err)
	}
	if f.gateways.online == 0 {
		t.Fatal("心跳应把网关标记为在线")
	}
	if err := f.svc.HandleMessage(gatewaymqtt.UpTopic("GW-0001", "offline"), []byte("{}")); err != nil {
		t.Fatalf("离线处理失败: %v", err)
	}
	if f.gateways.offline == 0 {
		t.Fatal("LWT 离线应把网关标记为离线")
	}
}

var _ = http.StatusOK

// TestIngestMarksDeviceOnlineAndThrottles 网关上行落库成功后，设备应被置为「在线」。
//
// 背景：设备创建时初始状态就是「离线」（deviceapp.AddDevice），而网关上行原本只
// 更新了网关状态、完全没更新设备，导致经网关上报的设备永远显示离线。
func TestIngestMarksDeviceOnlineAndThrottles(t *testing.T) {
	f := newIngestFixture(t)
	pctx := tenant.Platform(context.Background())

	dev, err := f.db.WithCtx(pctx).DeviceById(f.device.Id)
	if err != nil {
		t.Fatalf("查询设备: %v", err)
	}
	if dev.Status == constants.DeviceStatusOnline {
		t.Fatalf("初始状态不应该是在线")
	}

	ts := time.Now().UnixMilli()
	payload, _ := json.Marshal(map[string]interface{}{
		"seq":       2001,
		"ts":        ts,
		"deviceKey": "dev-1",
		"properties": map[string]interface{}{
			"temp": map[string]interface{}{"value": 23.5, "time": ts},
		},
	})
	if err := f.svc.HandleMessage(gatewaymqtt.UpTopic("GW-0001", "data"), payload); err != nil {
		t.Fatalf("处理上报失败: %v", err)
	}

	online, err := f.db.WithCtx(pctx).DeviceById(f.device.Id)
	if err != nil {
		t.Fatalf("查询设备: %v", err)
	}
	if online.Status != constants.DeviceStatusOnline {
		t.Fatalf("收到数据后设备状态应为在线，实际 %q", online.Status)
	}
	if online.LastOnlineTime <= 0 {
		t.Fatalf("收到数据后应写入 last_online_time，实际 %d", online.LastOnlineTime)
	}

	// 节流：紧接着的第二次上报不应再次改写 last_online_time
	first := online.LastOnlineTime
	payload2, _ := json.Marshal(map[string]interface{}{
		"seq":       2002,
		"ts":        ts,
		"deviceKey": "dev-1",
		"properties": map[string]interface{}{
			"temp": map[string]interface{}{"value": 24.0, "time": ts},
		},
	})
	if err := f.svc.HandleMessage(gatewaymqtt.UpTopic("GW-0001", "data"), payload2); err != nil {
		t.Fatalf("处理第二次上报失败: %v", err)
	}
	again, err := f.db.WithCtx(pctx).DeviceById(f.device.Id)
	if err != nil {
		t.Fatalf("查询设备: %v", err)
	}
	if again.LastOnlineTime != first {
		t.Fatalf("节流窗口内不应重复写 last_online_time：%d -> %d", first, again.LastOnlineTime)
	}
	if again.Status != constants.DeviceStatusOnline {
		t.Fatalf("状态应保持在线，实际 %q", again.Status)
	}
}

// TestDeviceOnlineOfflineLifecycle 完整闭环：上报置在线 → 超时扫描置离线。
//
// 在线状态原本只有「置在线」一半，没有超时置离线，状态会永久停在在线；
// 这里把两半串起来锁定。
func TestDeviceOnlineOfflineLifecycle(t *testing.T) {
	f := newIngestFixture(t)
	pctx := tenant.Platform(context.Background())

	ts := time.Now().UnixMilli()
	payload, _ := json.Marshal(map[string]interface{}{
		"seq":       3001,
		"ts":        ts,
		"deviceKey": "dev-1",
		"properties": map[string]interface{}{
			"temp": map[string]interface{}{"value": 25.0, "time": ts},
		},
	})
	if err := f.svc.HandleMessage(gatewaymqtt.UpTopic("GW-0001", "data"), payload); err != nil {
		t.Fatalf("上报失败: %v", err)
	}

	online, err := f.db.WithCtx(pctx).DeviceById(f.device.Id)
	if err != nil {
		t.Fatalf("查询设备: %v", err)
	}
	if online.Status != constants.DeviceStatusOnline {
		t.Fatalf("上报后应为在线，实际 %q", online.Status)
	}

	// 把最后在线时间回拨 10 分钟，模拟「之后不再上报」
	backdated := time.Now().Add(-10 * time.Minute).UnixMilli()
	if err := f.db.Pool.Table("device").Where("id = ?", f.device.Id).
		Update("last_online_time", backdated).Error; err != nil {
		t.Fatalf("回拨时间: %v", err)
	}

	// 超时 5 分钟 → 应被置离线
	cutoff := time.Now().Add(-5 * time.Minute).UnixMilli()
	n, err := f.db.WithCtx(pctx).MarkStaleDevicesOffline(cutoff)
	if err != nil {
		t.Fatalf("扫描: %v", err)
	}
	if n != 1 {
		t.Fatalf("应置 1 台离线，实际 %d", n)
	}
	offline, err := f.db.WithCtx(pctx).DeviceById(f.device.Id)
	if err != nil {
		t.Fatalf("查询设备: %v", err)
	}
	if offline.Status != constants.DeviceStatusOffline {
		t.Fatalf("超时后应离线，实际 %q", offline.Status)
	}
}
