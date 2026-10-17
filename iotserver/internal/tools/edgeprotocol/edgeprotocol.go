// Package edgeprotocol 解析参考系统（罗米物联网平台 / things，即 iotddc）的 /edge 上行协议。
//
// 与平台自有网关协议并存（自有协议见 tools/gatewaymqtt 的 iot/up/* 与
// docs/gateway-integration.md）。本包只做「把对方报文解析成平台内部结构」，
// 不含落库/注册逻辑，因此可独立单测。
//
// 主题（来自参考实现 mgmt/data 的 processor）：
//
//	/edge/connect/{pk}/{sn}                网关接入，报文含 tags[] 设备清单
//	/edge/property/{pk}/{sn}/post          实时数据
//	/edge/property/history/{pk}/{sn}/post  历史补传（比实时多一段 history）
//	/edge/lastwill/{pk}/{sn}               遗嘱 → 离线
//	/edge/db/{key}/{sn}/update             工程下发（下行）
//	/edge/upgrade/{pk}/{sn}/notify         OTA（下行）
//
// 数据报文实测形态（JSON 数组，每项一台设备）：
//
//	[{"dn":"THmeter","properties":{"AirRoomTemp1":"30.0000","DRoom Temp":"0.0000"},"time":1792122130}]
//
// 实测注意点：properties 的值**全是字符串**（"30.0000"/"0"/"-19108"）；
// time 是 **Unix 秒**且整包一个（无逐测点时间）；整包周期 60 秒；测点名**可含空格**。
package edgeprotocol

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
)

const (
	// TopicPrefix 参考系统所有主题的公共前缀
	TopicPrefix = "/edge/"
)

// 上行/下行主题种类
const (
	KindConnect         = "connect"          // 网关接入（含设备清单）
	KindProperty        = "property"         // 实时数据
	KindPropertyHistory = "property/history" // 历史补传
	KindLastwill        = "lastwill"         // 遗嘱
	KindDBUpdate        = "db/update"        // 工程下发（下行，回执为 /reply）
	KindUpgradeNotify   = "upgrade/notify"   // OTA（下行，回执为 /reply）
)

// accessMode 取值，与平台物模型的 accessMode 语义一致
const (
	AccessModeRead      = "r"
	AccessModeWrite     = "w"
	AccessModeReadWrite = "rw"
)

// ParseTopic 解析 /edge 主题，返回种类、pk、sn。
//
// history 主题比实时主题多一段，因此不能按固定下标取 pk/sn：
//
//	/edge/property/{pk}/{sn}/post          -> property,         pk, sn
//	/edge/property/history/{pk}/{sn}/post  -> property/history, pk, sn
func ParseTopic(topic string) (kind, pk, sn string, ok bool) {
	if !strings.HasPrefix(topic, TopicPrefix) {
		return "", "", "", false
	}
	segs := strings.Split(strings.TrimPrefix(topic, TopicPrefix), "/")
	for i := range segs {
		segs[i] = strings.TrimSpace(segs[i])
		// 空段与通配段都拒绝：订阅模式（/edge/property/+/+/post）只用于订阅，
		// 不该被当成上行报文解析——否则有人往该主题发布就会造出 pk="+" 的网关。
		if segs[i] == "" || strings.ContainsAny(segs[i], "+#") {
			return "", "", "", false
		}
	}

	switch {
	case len(segs) == 3 && segs[0] == "connect":
		// /edge/connect/{pk}/{sn}
		return KindConnect, segs[1], segs[2], true
	case len(segs) == 3 && segs[0] == "lastwill":
		// /edge/lastwill/{pk}/{sn}
		return KindLastwill, segs[1], segs[2], true
	case len(segs) == 4 && segs[0] == "property" && segs[3] == "post":
		return KindProperty, segs[1], segs[2], true
	case len(segs) == 5 && segs[0] == "property" && segs[1] == "history" && segs[4] == "post":
		return KindPropertyHistory, segs[2], segs[3], true
	case len(segs) == 4 && segs[0] == "db" && segs[3] == "update":
		return KindDBUpdate, segs[1], segs[2], true
	case len(segs) == 4 && segs[0] == "upgrade" && segs[3] == "notify":
		return KindUpgradeNotify, segs[1], segs[2], true
	}
	return "", "", "", false
}

// TopicsForGateway 按某台网关的 pk/sn 组装它需要订阅的上行主题。
//
// **逐个网关拼主题，而不是订阅 `/edge/property/+/+/post` 这类通配**，原因有二：
//
//  1. 数据边界：参考系统的 broker 上挂着**别的客户**的网关（实测该 broker 允许匿名
//     订阅），通配等于把别人的生产数据一并拉进自己的平台。
//  2. 开销：网关一多，通配订阅的流量与解析成本不可控，而这些数据平台根本用不上。
//
// pk 为空时用 sn 顶替。实测参考系统的 pk 与 sn 取值相同，但两者语义不同
// （pk 是产品/型号标识、sn 是设备序列号），若对方给的 pk 与 sn 不一致，
// 应在网关里把 pk 填上，否则主题拼不对、订阅不到数据。
func TopicsForGateway(pk, sn string) []string {
	sn = strings.TrimSpace(sn)
	if sn == "" {
		return nil
	}
	if pk = strings.TrimSpace(pk); pk == "" {
		pk = sn
	}
	return []string{
		TopicPrefix + "connect/" + pk + "/" + sn,
		TopicPrefix + "property/" + pk + "/" + sn + "/post",
		TopicPrefix + "property/history/" + pk + "/" + sn + "/post",
		TopicPrefix + "lastwill/" + pk + "/" + sn,
	}
}

// DeviceReport 数据报文解析后的一台设备一条记录。
type DeviceReport struct {
	// DeviceKey 源报文的 dn，作为平台侧的设备匹配键
	DeviceKey string
	// Properties 测点 code -> 原始字符串值（保持源报文形态，落库时再尝试转数值）
	Properties map[string]string
	// TimeMs 采集时间（毫秒）。源报文 time 为 Unix 秒且整包一个，
	// 逐测点时间即取此值（源数据没有逐点时间）。
	TimeMs int64
}

// propertyPostItem 数据报文数组的一项。
//
// 用 json.RawMessage 接 properties，是因为实测值全是字符串，但源系统并不保证
// （参考实现按 tagValue.toString() 兜底），这里按标量统一成字符串更稳。
type propertyPostItem struct {
	Dn         string                     `json:"dn"`
	Properties map[string]json.RawMessage `json:"properties"`
	Time       int64                      `json:"time"`
}

// ParsePropertyPost 解析 /edge/property/.../post 的报文体。
func ParsePropertyPost(payload []byte) ([]DeviceReport, error) {
	payload = bytes.TrimSpace(payload)
	if len(payload) == 0 {
		return nil, fmt.Errorf("空报文")
	}
	var items []propertyPostItem
	if err := json.Unmarshal(payload, &items); err != nil {
		return nil, fmt.Errorf("数据报文解析失败（应为 JSON 数组）: %w", err)
	}

	out := make([]DeviceReport, 0, len(items))
	for _, item := range items {
		dn := strings.TrimSpace(item.Dn)
		if dn == "" {
			// 单条记录缺 dn 无法定位设备，跳过该条而不是整包失败
			continue
		}
		props := make(map[string]string, len(item.Properties))
		for code, raw := range item.Properties {
			code = strings.TrimSpace(code)
			if code == "" {
				continue
			}
			props[code] = ScalarToString(raw)
		}
		out = append(out, DeviceReport{
			DeviceKey:  dn,
			Properties: props,
			TimeMs:     TimeToMillis(item.Time),
		})
	}
	return out, nil
}

// ScalarToString 把 JSON 标量统一成字符串。
//
// 字符串按原样返回（不做数值归一，避免 "30.0000" 变成 "30" 改变原始精度）；
// 数字/布尔取其字面量；null 与空对象返回空串。
func ScalarToString(raw json.RawMessage) string {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return ""
	}
	if trimmed[0] == '"' {
		var s string
		if err := json.Unmarshal(trimmed, &s); err == nil {
			return s
		}
		return string(trimmed)
	}
	if bytes.Equal(trimmed, []byte("null")) {
		return ""
	}
	return string(trimmed)
}

// NumericOrString 把源报文的字符串值转成落库用的类型。
//
// 必须做这一步：源报文的值**全是字符串**（"30.0000"/"0"/"-19108"），而时序层的
// splitValue 对 string 一律写进 text 列（internal/tools/datadb/duckdb/helpers.go:79
// 返回 (nil, v)），数值列会是空的——历史曲线、min/max/avg 这类数值查询就查不到数据。
// 因此能解析成数值的交给数值，其余保持字符串。
//
// 语义与参考实现一致（它用 new BigDecimal(tagValue.toString())，失败则存字符串），
// 但**不采纳**它把 inf/nan/空值改写成 0 的做法——那会伪造出并不存在的 0 值。
func NumericOrString(v string) interface{} {
	s := strings.TrimSpace(v)
	if s == "" {
		return v
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return v
	}
	// ParseFloat 会接受 "nan"/"inf" 且不报错，必须挡掉：NaN 落库后
	// 任何比较/聚合都不可靠，而且它会污染列的统计信息。
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return v
	}
	return f
}

// TimeToMillis 把源报文时间归一成毫秒。
//
// 源数据是 Unix 秒（实测 1792122130 = 2026-10-16 11:42:10），
// 但也容忍已经是毫秒的输入，避免上游换单位后历史数据错位到 1970 年。
func TimeToMillis(t int64) int64 {
	if t <= 0 {
		return 0
	}
	// 1e12 毫秒 = 2001-09-09；秒级时间戳在可预见未来都远小于它
	if t < 1_000_000_000_000 {
		return t * 1000
	}
	return t
}

// ConnectMessage /edge/connect/{pk}/{sn} 的报文体。
//
// 已知字段：ip / model / gwType / tags[]；其余键为各协议（驱动）的版本信息
// （参考实现按 Name/Version 读取），此处原样保留在 Protocols 里。
type ConnectMessage struct {
	Ip        string                  `json:"ip"`
	Model     string                  `json:"model"`
	GwType    string                  `json:"gwType"`
	Tags      []Tag                   `json:"tags"`
	Protocols map[string]ProtocolInfo `json:"-"`
}

// ProtocolInfo 单个协议（驱动）的版本信息。
type ProtocolInfo struct {
	Name    string `json:"Name"`
	Version string `json:"Version"`
}

// Tag 设备清单中的一条，同时承载「设备」与「测点」两层信息：
// 同一个 deviceCode 的多条 tag 属于同一台设备。
type Tag struct {
	// DeviceCode 设备标识，落平台侧 device_key
	DeviceCode string `json:"deviceCode"`
	// DeviceDescription 设备描述
	DeviceDescription string `json:"deviceDescription"`
	// TagCode 测点标识
	TagCode string `json:"tagCode"`
	// Description 测点描述
	Description string `json:"description"`
	// ObjectType 点类型，用于推导读写权限，缺省 AI
	ObjectType string `json:"objectType"`
}

// UnmarshalJSON 自定义解析：把已知字段摘出来，其余键视为协议版本信息。
func (m *ConnectMessage) UnmarshalJSON(data []byte) error {
	var all map[string]json.RawMessage
	if err := json.Unmarshal(data, &all); err != nil {
		return fmt.Errorf("接入报文解析失败: %w", err)
	}

	type alias ConnectMessage
	var a alias
	if err := json.Unmarshal(data, &a); err != nil {
		return fmt.Errorf("接入报文解析失败: %w", err)
	}
	*m = ConnectMessage(a)

	// 其余键：形如 {"modbusTcp":{"Name":"...","Version":"..."}}
	m.Protocols = make(map[string]ProtocolInfo)
	for key, raw := range all {
		switch key {
		case "ip", "model", "gwType", "tags":
			continue
		}
		var p ProtocolInfo
		if err := json.Unmarshal(raw, &p); err == nil && p.Name != "" {
			m.Protocols[key] = p
		}
	}
	return nil
}

// ParseConnect 解析接入报文，并对 tags 做规范化（丢弃无 deviceCode 的条目）。
func ParseConnect(payload []byte) (*ConnectMessage, error) {
	var msg ConnectMessage
	if err := json.Unmarshal(payload, &msg); err != nil {
		return nil, err
	}
	tags := make([]Tag, 0, len(msg.Tags))
	for _, t := range msg.Tags {
		t.DeviceCode = strings.TrimSpace(t.DeviceCode)
		if t.DeviceCode == "" {
			continue
		}
		t.TagCode = strings.TrimSpace(t.TagCode)
		t.ObjectType = strings.TrimSpace(t.ObjectType)
		tags = append(tags, t)
	}
	msg.Tags = tags
	return &msg, nil
}

// AccessMode 由 objectType 推导读写权限，与参考实现一致：
// 以 I 结尾（AI/DI）→ 只读；O（AO/DO）→ 只写；V（AV/DV）→ 读写；其他→只读。
func AccessMode(objectType string) string {
	ot := strings.ToUpper(strings.TrimSpace(objectType))
	if ot == "" {
		return AccessModeRead
	}
	last := ot[len(ot)-1]
	switch last {
	case 'O':
		return AccessModeWrite
	case 'V':
		return AccessModeReadWrite
	default:
		return AccessModeRead
	}
}

// DedupKey 数据报文的去重键。
//
// 实测报文**没有 seq 字段**（自有协议才有），因此兼容路径按
// 「设备 + 采集时间」去重：整包一个时间、周期 60 秒，粒度足够。
func DedupKey(deviceID string, tsMs int64) string {
	return fmt.Sprintf("%s@%d", deviceID, tsMs)
}
