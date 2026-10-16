package edgeingest

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/LMgateways/iotserver-driver-proto/drivercommon"
	"github.com/LMgateways/iotserver-driver-proto/thingmodel"

	"iotserver/internal/dtos"
	"iotserver/internal/iotserver/core/infrastructure/sqlite"
	interfaces "iotserver/internal/iotserver/core/interface"
	"iotserver/internal/models"
	"iotserver/internal/pkg/logger"
	"iotserver/internal/pkg/tenant"
	"iotserver/internal/tools/edgeprotocol"
)

// fakeMessageItf 捕获交给平台上行管道的物模型消息
type fakeMessageItf struct {
	interfaces.MessageItf
	msgs []dtos.ThingModelMessage
}

func (f *fakeMessageItf) ThingModelMsgReport(_ context.Context, msg dtos.ThingModelMessage) (*drivercommon.CommonResponse, error) {
	f.msgs = append(f.msgs, msg)
	return &drivercommon.CommonResponse{Success: true}, nil
}

// fakeGatewayItf 记录在线/离线调用
type fakeGatewayItf struct {
	interfaces.GatewayItf
	online  int
	offline int
	lastIp  string
}

func (f *fakeGatewayItf) GatewayOnline(_ context.Context, _, ip string, _ int64) error {
	f.online++
	f.lastIp = ip
	return nil
}

func (f *fakeGatewayItf) GatewayOffline(_ context.Context, _ string) error {
	f.offline++
	return nil
}

type edgeFixture struct {
	svc      *Service
	db       *sqlite.Client
	messages *fakeMessageItf
	gateways *fakeGatewayItf
	tenantID string
	gateway  models.Gateway
}

// newEdgeFixture 用真 sqlite（含迁移）搭一套接入环境：
// 这样 migration、部分唯一索引、仓储 upsert 都会被真实执行，而不是被假件掩盖。
func newEdgeFixture(t *testing.T) *edgeFixture {
	t.Helper()
	lc := logger.NewClient("edge-ingest-test", "ERROR", "")
	db, err := sqlite.NewClient(dtos.Configuration{DataSource: filepath.Join(t.TempDir(), "core.db")}, lc)
	if err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	t.Cleanup(db.CloseSession)
	if err := db.MigrateSystemTables(); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	pctx := tenant.Platform(context.Background())
	tc, err := db.WithCtx(pctx).AddTenant(models.Tenant{Code: "edge_tenant", Name: "参考系统接入租户"})
	if err != nil {
		t.Fatalf("create tenant: %v", err)
	}
	tctx := tenant.With(pctx, tenant.Context{TenantID: tc.Id})

	gw, err := db.WithCtx(tctx).AddGateway(models.Gateway{
		Name:              "参考系统网关",
		Sn:                "260112801",
		Pk:                "260112801",
		Protocol:          models.GatewayProtocolEdge,
		ExcludeDeviceKeys: "system,user",
	})
	if err != nil {
		t.Fatalf("create gateway: %v", err)
	}
	// 回读一次：确认 AutoProvision 的数据库默认值确实生效（默认应打开自动注册）
	gw, err = db.WithCtx(tctx).GatewayById(gw.Id)
	if err != nil {
		t.Fatalf("reload gateway: %v", err)
	}

	messages := &fakeMessageItf{}
	gateways := &fakeGatewayItf{}
	svc := &Service{lc: lc, dbClient: db, messageItf: messages, gatewayItf: gateways}
	return &edgeFixture{
		svc: svc, db: db, messages: messages, gateways: gateways,
		tenantID: tc.Id, gateway: gw,
	}
}

// tenantCtx 构造该网关所属租户的作用域（服务内部也是这么做的）
func (f *edgeFixture) tenantCtx() context.Context {
	return tenant.With(tenant.Platform(context.Background()), tenant.Context{TenantID: f.tenantID})
}

// connectPayload 构造接入报文：两台设备，一台带两个测点（AI/AO），一台带一个（AV）
const connectPayload = `{
  "ip":"10.0.0.9",
  "model":"GC201",
  "gwType":"EDGE-GW",
  "tags":[
    {"deviceCode":"THmeter","deviceDescription":"温湿度表","tagCode":"AirRoomTemp1","description":"室内温度1","objectType":"AI"},
    {"deviceCode":"THmeter","deviceDescription":"温湿度表","tagCode":"TempSet1","description":"温度设定1","objectType":"AO"},
    {"deviceCode":"pump","deviceDescription":"水泵","tagCode":"Speed","description":"转速","objectType":"AV"}
  ],
  "modbusTcp":{"Name":"modbusTcp","Version":"1.2.0"}
}`

func TestHandleConnect_AutoProvisionDefaultIsOn(t *testing.T) {
	f := newEdgeFixture(t)
	if f.gateway.AutoProvision != models.GatewayAutoProvisionOn {
		t.Fatalf("AutoProvision 默认值应为 %d（自动注册开启），实际 %d——"+
			"说明模型上的 default 标签没有生效，自动注册会被意外关闭",
			models.GatewayAutoProvisionOn, f.gateway.AutoProvision)
	}
}

func TestHandleConnect_RegistersDevicesAndTags(t *testing.T) {
	f := newEdgeFixture(t)
	topic := "/edge/connect/260112801/260112801"
	if err := f.svc.HandleMessage(topic, []byte(connectPayload)); err != nil {
		t.Fatalf("HandleMessage: %v", err)
	}

	devices, err := f.db.WithCtx(f.tenantCtx()).ListGatewayDevices(f.gateway.Id)
	if err != nil {
		t.Fatalf("list devices: %v", err)
	}
	if len(devices) != 2 {
		t.Fatalf("设备数 = %d, want 2: %+v", len(devices), devices)
	}

	th, ok := devices["THmeter"]
	if !ok {
		t.Fatalf("缺少 THmeter: %+v", devices)
	}
	// 匹配键是 dn → device_key；展示名用描述
	if th.DeviceKey != "THmeter" {
		t.Errorf("DeviceKey = %q, want THmeter", th.DeviceKey)
	}
	if th.Name != "温湿度表" {
		t.Errorf("Name = %q, want 温湿度表（优先用描述）", th.Name)
	}
	if th.GatewayId != f.gateway.Id {
		t.Errorf("GatewayId = %q, want %q", th.GatewayId, f.gateway.Id)
	}
	if th.ProvisionSource != models.DeviceSourceGateway {
		t.Errorf("ProvisionSource = %q, want %q", th.ProvisionSource, models.DeviceSourceGateway)
	}
	if th.ProvisionState != models.DeviceStateActive {
		t.Errorf("ProvisionState = %q, want %q", th.ProvisionState, models.DeviceStateActive)
	}
	if th.TenantId != f.tenantID {
		t.Errorf("TenantId = %q, want %q（租户隔离应自动写入）", th.TenantId, f.tenantID)
	}

	// 测点：objectType 推读写权限
	tags, err := f.db.WithCtx(f.tenantCtx()).ListDeviceTags(th.Id)
	if err != nil {
		t.Fatalf("list tags: %v", err)
	}
	byCode := map[string]models.DeviceTag{}
	for _, tg := range tags {
		byCode[tg.Code] = tg
	}
	if len(byCode) != 2 {
		t.Fatalf("测点数 = %d, want 2: %+v", len(byCode), byCode)
	}
	if got := byCode["AirRoomTemp1"]; got.AccessMode != edgeprotocol.AccessModeRead || got.Name != "室内温度1" {
		t.Errorf("AirRoomTemp1 = %+v, want accessMode=r name=室内温度1", got)
	}
	if got := byCode["TempSet1"]; got.AccessMode != edgeprotocol.AccessModeWrite {
		t.Errorf("TempSet1 accessMode = %q, want w（objectType=AO）", got.AccessMode)
	}
	if got := byCode["AirRoomTemp1"]; got.Source != models.DeviceTagSourceAuto {
		t.Errorf("测点 source = %q, want auto", got.Source)
	}

	// 接入即在线，且带上了报文里的 ip
	if f.gateways.online != 1 {
		t.Errorf("GatewayOnline 调用 %d 次, want 1", f.gateways.online)
	}
	if f.gateways.lastIp != "10.0.0.9" {
		t.Errorf("上报 ip = %q, want 10.0.0.9", f.gateways.lastIp)
	}
}

func TestHandleConnect_ExcludesConfiguredKeys(t *testing.T) {
	f := newEdgeFixture(t)
	// system/user 在网关的排除名单里（实测网关会把自己的时钟当设备上报）
	payload := `{"tags":[
	  {"deviceCode":"system","tagCode":"TIME_MINUTE"},
	  {"deviceCode":"THmeter","tagCode":"AirRoomTemp1","objectType":"AI"}
	]}`
	if err := f.svc.HandleMessage("/edge/connect/260112801/260112801", []byte(payload)); err != nil {
		t.Fatalf("HandleMessage: %v", err)
	}
	devices, err := f.db.WithCtx(f.tenantCtx()).ListGatewayDevices(f.gateway.Id)
	if err != nil {
		t.Fatalf("list devices: %v", err)
	}
	if _, exists := devices["system"]; exists {
		t.Errorf("system 应被排除名单过滤掉: %+v", devices)
	}
	if len(devices) != 1 {
		t.Errorf("设备数 = %d, want 1", len(devices))
	}
}

func TestHandleConnect_FullListMarksAbsentWithoutDeleting(t *testing.T) {
	f := newEdgeFixture(t)
	if err := f.svc.HandleMessage("/edge/connect/260112801/260112801", []byte(connectPayload)); err != nil {
		t.Fatalf("first connect: %v", err)
	}
	// 第二次接入只带一台：另一台应被标记摘除，但**不删行**
	second := `{"tags":[{"deviceCode":"THmeter","tagCode":"AirRoomTemp1","objectType":"AI"}]}`
	if err := f.svc.HandleMessage("/edge/connect/260112801/260112801", []byte(second)); err != nil {
		t.Fatalf("second connect: %v", err)
	}

	devices, err := f.db.WithCtx(f.tenantCtx()).ListGatewayDevices(f.gateway.Id)
	if err != nil {
		t.Fatalf("list devices: %v", err)
	}
	if len(devices) != 2 {
		t.Fatalf("设备数 = %d, want 2（摘除不删行）: %+v", len(devices), devices)
	}
	if got := devices["pump"].ProvisionState; got != models.DeviceStateAbsent {
		t.Errorf("pump.ProvisionState = %q, want %q", got, models.DeviceStateAbsent)
	}
	if got := devices["THmeter"].ProvisionState; got != models.DeviceStateActive {
		t.Errorf("THmeter.ProvisionState = %q, want %q", got, models.DeviceStateActive)
	}
}

func TestHandleConnect_ReappearingDeviceBecomesActive(t *testing.T) {
	f := newEdgeFixture(t)
	if err := f.svc.HandleMessage("/edge/connect/260112801/260112801", []byte(connectPayload)); err != nil {
		t.Fatalf("first connect: %v", err)
	}
	only := `{"tags":[{"deviceCode":"THmeter","tagCode":"AirRoomTemp1"}]}`
	if err := f.svc.HandleMessage("/edge/connect/260112801/260112801", []byte(only)); err != nil {
		t.Fatalf("second connect: %v", err)
	}
	// 第三次又把 pump 带回来 → 状态应恢复
	if err := f.svc.HandleMessage("/edge/connect/260112801/260112801", []byte(connectPayload)); err != nil {
		t.Fatalf("third connect: %v", err)
	}
	devices, err := f.db.WithCtx(f.tenantCtx()).ListGatewayDevices(f.gateway.Id)
	if err != nil {
		t.Fatalf("list devices: %v", err)
	}
	if got := devices["pump"].ProvisionState; got != models.DeviceStateActive {
		t.Errorf("重新出现后 pump.ProvisionState = %q, want %q", got, models.DeviceStateActive)
	}
}

func TestHandleConnect_AutoProvisionOffSkipsRegistration(t *testing.T) {
	f := newEdgeFixture(t)
	// 关掉自动注册
	if err := f.db.Pool.Exec(
		"UPDATE gateway SET auto_provision = 0 WHERE id = ?", f.gateway.Id).Error; err != nil {
		t.Fatalf("disable auto provision: %v", err)
	}
	gw, err := f.db.WithCtx(f.tenantCtx()).GatewayById(f.gateway.Id)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	f.svc.HandleMessage("/edge/connect/260112801/260112801", []byte(`{"ip":"1.2.3.4"}`))
	_ = gw

	devices, err := f.db.WithCtx(f.tenantCtx()).ListGatewayDevices(f.gateway.Id)
	if err != nil {
		t.Fatalf("list devices: %v", err)
	}
	if len(devices) != 0 {
		t.Errorf("关闭自动注册后不应建设备, got %+v", devices)
	}
}

// 实测抓到的真实报文（网关 260112801）：值全是字符串、整包一个 time（Unix 秒）
const realPropertyPost = `[{"dn":"THmeter","properties":{"AirRoomTemp1":"30.0000","DRoom Temp":"0.0000","Spare3":"-19108","Switch1":"0"},"time":1792122130},{"dn":"pump","properties":{"Speed":"1450"},"time":1792122130}]`

func TestHandleProperty_IngestsRealPayload(t *testing.T) {
	f := newEdgeFixture(t)
	if err := f.svc.HandleMessage("/edge/connect/260112801/260112801", []byte(connectPayload)); err != nil {
		t.Fatalf("connect: %v", err)
	}
	f.messages.msgs = nil

	if err := f.svc.HandleMessage("/edge/property/260112801/260112801/post", []byte(realPropertyPost)); err != nil {
		t.Fatalf("property: %v", err)
	}
	// 一包两台设备 → 两条上行消息
	if len(f.messages.msgs) != 2 {
		t.Fatalf("上行消息数 = %d, want 2", len(f.messages.msgs))
	}

	// 找到 THmeter 那条，校验测点值与时间
	var target *dtos.ThingModelMessage
	devices, _ := f.db.WithCtx(f.tenantCtx()).ListGatewayDevices(f.gateway.Id)
	for i := range f.messages.msgs {
		if f.messages.msgs[i].Cid == devices["THmeter"].Id {
			target = &f.messages.msgs[i]
		}
	}
	if target == nil {
		t.Fatal("没有找到 THmeter 的上行消息")
	}
	if target.OpType != int32(thingmodel.OperationType_DATA_BATCH_REPORT) {
		t.Errorf("OpType = %d, want DATA_BATCH_REPORT", target.OpType)
	}
	var report dtos.DeviceBatchReport
	if err := json.Unmarshal([]byte(target.Data), &report); err != nil {
		t.Fatalf("解析上报体: %v", err)
	}
	// 关键：字符串值必须转成数值，否则会落进时序库的 text 列、数值查询查不到
	v, ok := report.Data.Properties["AirRoomTemp1"].Value.(float64)
	if !ok {
		t.Fatalf("AirRoomTemp1 的值应为 float64, got %T (%v)",
			report.Data.Properties["AirRoomTemp1"].Value, report.Data.Properties["AirRoomTemp1"].Value)
	}
	if v != 30 {
		t.Errorf("AirRoomTemp1 = %v, want 30", v)
	}
	if got := report.Data.Properties["Spare3"].Value; got != float64(-19108) {
		t.Errorf("Spare3 = %v, want -19108", got)
	}
	if got := report.Data.Properties["Switch1"].Value; got != float64(0) {
		t.Errorf("Switch1 = %v, want 0", got)
	}
	// 含空格的测点名要原样保留
	if _, ok := report.Data.Properties["DRoom Temp"]; !ok {
		t.Errorf("含空格测点丢失: %+v", report.Data.Properties)
	}
	// 整包 Unix 秒 → 毫秒
	const wantMs int64 = 1792122130000
	if report.Time != wantMs {
		t.Errorf("上报时间 = %d, want %d", report.Time, wantMs)
	}
	for code, p := range report.Data.Properties {
		if p.Time != wantMs {
			t.Errorf("测点 %s 的时间 = %d, want %d（源数据整包一个时间）", code, p.Time, wantMs)
		}
	}
}

func TestHandleProperty_IsIdempotentByPacketTimestamp(t *testing.T) {
	f := newEdgeFixture(t)
	if err := f.svc.HandleMessage("/edge/connect/260112801/260112801", []byte(connectPayload)); err != nil {
		t.Fatalf("connect: %v", err)
	}
	f.messages.msgs = nil

	if err := f.svc.HandleMessage("/edge/property/260112801/260112801/post", []byte(realPropertyPost)); err != nil {
		t.Fatalf("first property: %v", err)
	}
	first := len(f.messages.msgs)
	if err := f.svc.HandleMessage("/edge/property/260112801/260112801/post", []byte(realPropertyPost)); err != nil {
		t.Fatalf("second property: %v", err)
	}
	if got := len(f.messages.msgs); got != first {
		t.Errorf("重发同一包(同时间戳)产生了重复数据: 第一次 %d 条，第二次后 %d 条", first, got)
	}

	// 时间戳推进后应正常处理（不是永久屏蔽）
	next := `[{"dn":"THmeter","properties":{"AirRoomTemp1":"31.0000"},"time":1792122190}]`
	if err := f.svc.HandleMessage("/edge/property/260112801/260112801/post", []byte(next)); err != nil {
		t.Fatalf("third property: %v", err)
	}
	if got := len(f.messages.msgs); got != first+1 {
		t.Errorf("新时间戳的报文未被处理: got %d, want %d", got, first+1)
	}
}

func TestHandleProperty_LazyProvisionsUnknownDevice(t *testing.T) {
	f := newEdgeFixture(t)
	// 不发接入报文，直接来数据：设备应被惰性注册，数据不丢
	if err := f.svc.HandleMessage("/edge/property/260112801/260112801/post", []byte(realPropertyPost)); err != nil {
		t.Fatalf("property: %v", err)
	}
	devices, err := f.db.WithCtx(f.tenantCtx()).ListGatewayDevices(f.gateway.Id)
	if err != nil {
		t.Fatalf("list devices: %v", err)
	}
	if len(devices) != 2 {
		t.Fatalf("惰性注册后设备数 = %d, want 2: %+v", len(devices), devices)
	}
	if len(f.messages.msgs) != 2 {
		t.Fatalf("数据不应被丢弃: 上行消息数 = %d, want 2", len(f.messages.msgs))
	}
	// 惰性注册时应顺手登记观测到的测点
	tags, err := f.db.WithCtx(f.tenantCtx()).ListDeviceTags(devices["THmeter"].Id)
	if err != nil {
		t.Fatalf("list tags: %v", err)
	}
	if len(tags) == 0 {
		t.Error("惰性注册应登记观测到的测点，否则读取侧拿不到任何测点定义")
	}
	byCode := map[string]models.DeviceTag{}
	for _, tg := range tags {
		byCode[tg.Code] = tg
	}
	if got := byCode["AirRoomTemp1"].Type; got != "float" {
		t.Errorf("AirRoomTemp1 类型 = %q, want float（按首值推断）", got)
	}
	if got := byCode["Spare3"].Type; got != "int" {
		t.Errorf("Spare3 类型 = %q, want int", got)
	}
}

func TestHandleProperty_RespectsExcludeList(t *testing.T) {
	f := newEdgeFixture(t)
	payload := `[{"dn":"system","properties":{"TIME_MINUTE":"42"},"time":1792122130},{"dn":"pump","properties":{"Speed":"1"},"time":1792122130}]`
	if err := f.svc.HandleMessage("/edge/property/260112801/260112801/post", []byte(payload)); err != nil {
		t.Fatalf("property: %v", err)
	}
	devices, err := f.db.WithCtx(f.tenantCtx()).ListGatewayDevices(f.gateway.Id)
	if err != nil {
		t.Fatalf("list devices: %v", err)
	}
	if _, exists := devices["system"]; exists {
		t.Errorf("排除名单里的设备不应被创建: %+v", devices)
	}
	if len(f.messages.msgs) != 1 {
		t.Errorf("被排除设备的数据不应入管道: 上行消息数 = %d, want 1", len(f.messages.msgs))
	}
}

func TestHandleMessage_RejectsUnknownGateway(t *testing.T) {
	f := newEdgeFixture(t)
	err := f.svc.HandleMessage("/edge/property/999999999/999999999/post", []byte(realPropertyPost))
	if err == nil {
		t.Fatal("未注册网关的报文应被拒绝（防止任意设备注入数据）")
	}
	if len(f.messages.msgs) != 0 {
		t.Errorf("未注册网关不应产生上行消息, got %d", len(f.messages.msgs))
	}
}

func TestHandleMessage_PkMismatchFallsBackToEmptyPk(t *testing.T) {
	f := newEdgeFixture(t)
	// 平台界面手工建的网关 pk 为空，也应能被 /edge 报文命中（便于逐步接入存量网关）
	if err := f.db.Pool.Exec(
		"UPDATE gateway SET pk = '' WHERE id = ?", f.gateway.Id).Error; err != nil {
		t.Fatalf("clear pk: %v", err)
	}
	if err := f.svc.HandleMessage("/edge/property/260112801/260112801/post", []byte(realPropertyPost)); err != nil {
		t.Fatalf("pk 为空的网关应能被命中: %v", err)
	}
}

func TestHandleMessage_LastwillMarksOffline(t *testing.T) {
	f := newEdgeFixture(t)
	if err := f.svc.HandleMessage("/edge/lastwill/260112801/260112801", []byte(`{"online":false}`)); err != nil {
		t.Fatalf("lastwill: %v", err)
	}
	if f.gateways.offline != 1 {
		t.Errorf("GatewayOffline 调用 %d 次, want 1", f.gateways.offline)
	}
}

func TestHandleMessage_RejectsBadTopics(t *testing.T) {
	f := newEdgeFixture(t)
	for _, topic := range []string{
		"/iot/up/260112801/data",        // 自有协议主题不该走这里
		"/edge/property/260112801/post", // 段数不对
		"/edge/unknown/260112801/260112801",
	} {
		if err := f.svc.HandleMessage(topic, []byte(`[]`)); err == nil {
			t.Errorf("主题 %q 应被拒绝", topic)
		}
	}
}
