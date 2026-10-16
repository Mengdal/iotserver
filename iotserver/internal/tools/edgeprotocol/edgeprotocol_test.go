package edgeprotocol

import (
	"testing"
)

// 实测抓到的真实报文（网关 260112801，2026-10-16 11:42:10）。
// 用它当用例而不是自己造：里面包含了「值全是字符串」「测点名含空格」
// 「同包多设备」「整包一个 time」这些真实特征。
const realPropertyPost = `[{"dn":"THmeter","properties":{"AirRoomTemp1":"30.0000","AirRoomTemp2":"30.0000","AirRoomTemp3":"30.0000","CoilHumidity":"18.0000","CoilTemp":"18.0000","DRoom Temp":"0.0000","Energy":"139164.0000","ReAirVelocity":"18.0000","ShopHumidity":"18.0000","Spare1":"0.0000","Spare2":"0.0000","Spare3":"-19108","Spare4":"-19108","SupAirVelocity":"18.0000","Switch1":"0","Switch2":"0","TempSet1":"0.0000","TempSet2":"0.0000"},"time":1792122130},{"dn":"system","properties":{"TIME_MINUTE":"42","TIME_SECOND":"10"},"time":1792122130},{"dn":"user","properties":{"tag0001":"30","tag0002":"18"},"time":1792122130}]`

func TestParseTopic(t *testing.T) {
	cases := []struct {
		topic  string
		kind   string
		pk, sn string
		ok     bool
	}{
		{"/edge/connect/260112801/260112801", KindConnect, "260112801", "260112801", true},
		{"/edge/lastwill/260112801/260112801", KindLastwill, "260112801", "260112801", true},
		{"/edge/property/260112801/260112801/post", KindProperty, "260112801", "260112801", true},
		// history 比实时多一段，不能按下标误取成 pk=history
		{"/edge/property/history/260112801/260112801/post", KindPropertyHistory, "260112801", "260112801", true},
		{"/edge/db/abc/260112801/update", KindDBUpdate, "abc", "260112801", true},
		{"/edge/upgrade/260112801/260112801/notify", KindUpgradeNotify, "260112801", "260112801", true},
		// 非法：前缀不对 / 段数不对 / 空段
		{"/iot/up/260112801/data", "", "", "", false},
		{"/edge/property/260112801/post", "", "", "", false},
		{"/edge/connect//260112801", "", "", "", false},
		{"/edge/property/+/+/post", "", "", "", false},
	}
	for _, c := range cases {
		kind, pk, sn, ok := ParseTopic(c.topic)
		if ok != c.ok || kind != c.kind || pk != c.pk || sn != c.sn {
			t.Errorf("ParseTopic(%q) = (%q,%q,%q,%v), want (%q,%q,%q,%v)",
				c.topic, kind, pk, sn, ok, c.kind, c.pk, c.sn, c.ok)
		}
	}
}

func TestParsePropertyPost_RealPayload(t *testing.T) {
	reports, err := ParsePropertyPost([]byte(realPropertyPost))
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if len(reports) != 3 {
		t.Fatalf("设备数 = %d, want 3", len(reports))
	}

	th := reports[0]
	if th.DeviceKey != "THmeter" {
		t.Errorf("DeviceKey = %q, want THmeter", th.DeviceKey)
	}
	if len(th.Properties) != 18 {
		t.Errorf("测点数 = %d, want 18", len(th.Properties))
	}
	// 值保持源报文的字符串形态，不做数值归一（"30.0000" 不能变成 "30"）
	if got := th.Properties["AirRoomTemp1"]; got != "30.0000" {
		t.Errorf("AirRoomTemp1 = %q, want \"30.0000\"", got)
	}
	// 测点名含空格是实测存在的
	if got := th.Properties["DRoom Temp"]; got != "0.0000" {
		t.Errorf("含空格测点 \"DRoom Temp\" = %q, want \"0.0000\"", got)
	}
	// 整数形态也要原样保留
	if got := th.Properties["Spare3"]; got != "-19108" {
		t.Errorf("Spare3 = %q, want \"-19108\"", got)
	}

	// 整包一个 time，逐设备相同；Unix 秒 → 毫秒
	const wantMs int64 = 1792122130000
	for i, r := range reports {
		if r.TimeMs != wantMs {
			t.Errorf("reports[%d].TimeMs = %d, want %d", i, r.TimeMs, wantMs)
		}
	}
	if reports[1].DeviceKey != "system" || reports[2].DeviceKey != "user" {
		t.Errorf("设备顺序不符: %q, %q", reports[1].DeviceKey, reports[2].DeviceKey)
	}
}

func TestParsePropertyPost_Errors(t *testing.T) {
	if _, err := ParsePropertyPost(nil); err == nil {
		t.Error("空报文应报错")
	}
	// 对象而非数组：参考系统解析的就是数组，报错比静默吞掉好
	if _, err := ParsePropertyPost([]byte(`{"dn":"a","properties":{},"time":1}`)); err == nil {
		t.Error("非数组报文应报错")
	}
	// 缺 dn 的单条跳过，其余照常
	reports, err := ParsePropertyPost([]byte(`[{"properties":{"a":"1"},"time":1792122130},{"dn":"ok","properties":{"b":"2"},"time":1792122130}]`))
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if len(reports) != 1 || reports[0].DeviceKey != "ok" {
		t.Errorf("缺 dn 的条目应被跳过, got %+v", reports)
	}
}

func TestTimeToMillis(t *testing.T) {
	cases := []struct{ in, want int64 }{
		{1792122130, 1792122130000},    // Unix 秒
		{1792122130000, 1792122130000}, // 已经是毫秒，原样
		{0, 0},
		{-1, 0},
	}
	for _, c := range cases {
		if got := TimeToMillis(c.in); got != c.want {
			t.Errorf("TimeToMillis(%d) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestAccessMode(t *testing.T) {
	cases := []struct{ in, want string }{
		{"AI", AccessModeRead}, {"DI", AccessModeRead},
		{"AO", AccessModeWrite}, {"DO", AccessModeWrite},
		{"AV", AccessModeReadWrite}, {"DV", AccessModeReadWrite},
		{"", AccessModeRead},
		{"ai", AccessModeRead},  // 大小写不敏感
		{"XYZ", AccessModeRead}, // 未识别按只读
	}
	for _, c := range cases {
		if got := AccessMode(c.in); got != c.want {
			t.Errorf("AccessMode(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestParseConnect(t *testing.T) {
	// 结构取自参考实现 GatewayConnectProcessor.dealVersion 读取的字段；
	// 该报文只在网关启动时发一次，本次未能抓到线上样本（见 docs/device-access-design.md）
	payload := `{
	  "ip":"10.0.0.9",
	  "model":"GC201",
	  "gwType":"EDGE-GW",
	  "tags":[
	    {"deviceCode":"THmeter","deviceDescription":"温湿度表","tagCode":"AirRoomTemp1","description":"室内温度1","objectType":"AI"},
	    {"deviceCode":"THmeter","deviceDescription":"温湿度表","tagCode":"TempSet1","description":"温度设定1","objectType":"AO"},
	    {"deviceCode":"","tagCode":"broken"},
	    {"deviceCode":"pump","tagCode":"run","objectType":"V"}
	  ],
	  "modbusTcp":{"Name":"modbusTcp","Version":"1.2.0"},
	  "web":{"Name":"web","Version":"2.0.0"}
	}`
	msg, err := ParseConnect([]byte(payload))
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if msg.Ip != "10.0.0.9" || msg.Model != "GC201" || msg.GwType != "EDGE-GW" {
		t.Errorf("基础字段不符: %+v", msg)
	}
	// 无 deviceCode 的条目应被丢弃
	if len(msg.Tags) != 3 {
		t.Fatalf("tags 数 = %d, want 3（空 deviceCode 应丢弃）", len(msg.Tags))
	}
	if msg.Tags[0].DeviceCode != "THmeter" || msg.Tags[0].TagCode != "AirRoomTemp1" {
		t.Errorf("tag[0] 不符: %+v", msg.Tags[0])
	}
	// 其余键识别为协议版本
	if len(msg.Protocols) != 2 {
		t.Fatalf("协议数 = %d, want 2: %+v", len(msg.Protocols), msg.Protocols)
	}
	if p := msg.Protocols["modbusTcp"]; p.Version != "1.2.0" || p.Name != "modbusTcp" {
		t.Errorf("modbusTcp 协议不符: %+v", p)
	}
}

func TestScalarToString(t *testing.T) {
	cases := []struct{ in, want string }{
		{`"30.0000"`, "30.0000"},
		{`30.0000`, "30.0000"},
		{`-19108`, "-19108"},
		{`0`, "0"},
		{`true`, "true"},
		{`null`, ""},
		{``, ""},
	}
	for _, c := range cases {
		if got := ScalarToString([]byte(c.in)); got != c.want {
			t.Errorf("ScalarToString(%s) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestDedupKey(t *testing.T) {
	// 同一设备同一时刻 → 同键；不同设备或不同时刻 → 不同键
	if DedupKey("THmeter", 1000) != DedupKey("THmeter", 1000) {
		t.Error("同设备同时间应产生相同去重键")
	}
	if DedupKey("THmeter", 1000) == DedupKey("pump", 1000) {
		t.Error("不同设备不应产生相同去重键")
	}
	if DedupKey("THmeter", 1000) == DedupKey("THmeter", 2000) {
		t.Error("不同时间不应产生相同去重键")
	}
}

func TestNumericOrString(t *testing.T) {
	// 能解析成数值的必须转成数值：否则会落进时序库的 text 列，数值查询查不到
	numeric := []struct {
		in   string
		want float64
	}{
		{"30.0000", 30},
		{"-19108", -19108},
		{"0", 0},
		{"18", 18},
		{"139164.0000", 139164},
		{"1e3", 1000},
	}
	for _, c := range numeric {
		got, ok := NumericOrString(c.in).(float64)
		if !ok {
			t.Errorf("NumericOrString(%q) 应返回 float64, got %T", c.in, NumericOrString(c.in))
			continue
		}
		if got != c.want {
			t.Errorf("NumericOrString(%q) = %v, want %v", c.in, got, c.want)
		}
	}

	// 非数值与「看似数值但是特殊值」的必须原样保持字符串
	asString := []string{"auto", "manual", "true", "NONE", "", "  ", "nan", "NaN", "inf", "-inf", "+Inf", "12abc"}
	for _, in := range asString {
		got := NumericOrString(in)
		if _, isFloat := got.(float64); isFloat {
			t.Errorf("NumericOrString(%q) = %v, 不应转成数值", in, got)
		}
		if got != in {
			t.Errorf("NumericOrString(%q) = %v, 应原样保持", in, got)
		}
	}
}
