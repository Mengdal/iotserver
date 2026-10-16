// Package gatewayingest 负责第三方工业网关的 MQTT 上行接入。
//
// 数据链路：网关（MQTT 发布）→ 本服务（订阅/解析/鉴权/幂等）→
// 复用平台既有上行管道 MessageItf.ThingModelMsgReport（落库 + 推 eKuiper 规则）。
//
// 主题约定（详见 docs/网关接入规范）：
//
//	iot/up/{sn}/data     实时数据（QoS1）
//	iot/up/{sn}/batch    断点续传批量补传（QoS1，逐条带采集时间）
//	iot/up/{sn}/status   心跳/状态（含 LWT 上线）
//	iot/up/{sn}/offline  MQTT 遗嘱：网关异常断开
//	iot/down/{sn}/config 平台下发配置
//	iot/down/{sn}/cmd    平台下发命令
//
// 幂等：报文带 seq，平台按 (gateway_id, seq) 去重，网关补传重复发送不会产生重复数据。
package gatewayingest

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/LMgateways/iotserver-driver-proto/thingmodel"

	"iotserver/internal/dtos"
	coreConfig "iotserver/internal/iotserver/core/config"
	coreContainer "iotserver/internal/iotserver/core/container"
	interfaces "iotserver/internal/iotserver/core/interface"
	"iotserver/internal/models"
	pkgcontainer "iotserver/internal/pkg/container"
	"iotserver/internal/pkg/di"
	"iotserver/internal/pkg/logger"
	"iotserver/internal/pkg/tenant"
	"iotserver/internal/tools/gatewaymqtt"
	pkgMQTT "iotserver/internal/tools/mqttclient"
)

// GatewayUplinkMessage 网关上行报文的统一结构。
//
// 实时上报（data）：单条设备数据；
// 批量补传（batch）：records 内含多条，每条带自己的 ts。
type GatewayUplinkMessage struct {
	// Seq 网关侧单调递增序号，用于幂等去重（同一 seq 只处理一次）
	Seq int64 `json:"seq"`
	// Ts 报文时间（毫秒），作为 records/测点未带时间时的兜底
	Ts int64 `json:"ts"`
	// 实时上报字段
	DeviceKey  string                        `json:"deviceKey"`
	ProductKey string                        `json:"productKey"`
	Properties map[string]dtos.BatchProperty `json:"properties"`
	Events     map[string]dtos.BatchEvent    `json:"events"`
	// 批量补传
	Records []GatewayUplinkRecord `json:"records"`
}

// GatewayUplinkRecord 批量补传中的一条设备数据
type GatewayUplinkRecord struct {
	Ts         int64                         `json:"ts"`
	DeviceKey  string                        `json:"deviceKey"`
	ProductKey string                        `json:"productKey"`
	Properties map[string]dtos.BatchProperty `json:"properties"`
	Events     map[string]dtos.BatchEvent    `json:"events"`
}

// GatewayStatusMessage 网关心跳/状态
type GatewayStatusMessage struct {
	Ts            int64  `json:"ts"`
	Online        bool   `json:"online"`
	Firmware      string `json:"fw"`
	Ip            string `json:"ip"`
	BufferPending int    `json:"bufferPending"`
}

// Service 网关接入服务
type Service struct {
	dic        *di.Container
	lc         logger.LoggingClient
	dbClient   interfaces.DBClient
	messageItf interfaces.MessageItf
	gatewayItf interfaces.GatewayItf
	mqttClient pkgMQTT.MQTTClient

	// 设备置在线的节流状态：网关是按消息批量上报的，同一设备不必每批都写一次元数据库
	onlineMu   sync.Mutex
	onlineSeen map[string]time.Time
}

// deviceOnlineThrottle 同一设备最多多久写一次「置在线」。
// 只影响 last_online_time 的精度（最多滞后这么久），不影响状态正确性。
const deviceOnlineThrottle = 30 * time.Second

// markDeviceOnline 收到数据即视为设备在线。
//
// 网关上行原本只更新了**网关**状态（handleData 里的 GatewayOnline），
// 完全没有更新设备状态，而设备创建时初始状态写死为「离线」，
// 于是经网关上报的设备会永远显示离线。这里补上设备侧。
//
// 带节流：同一设备最多每 deviceOnlineThrottle 更新一次，避免高频上报把元数据库写爆。
func (s *Service) markDeviceOnline(ctx context.Context, deviceID string) {
	if deviceID == "" {
		return
	}
	now := time.Now()
	s.onlineMu.Lock()
	if s.onlineSeen == nil {
		s.onlineSeen = make(map[string]time.Time)
	}
	if last, ok := s.onlineSeen[deviceID]; ok && now.Sub(last) < deviceOnlineThrottle {
		s.onlineMu.Unlock()
		return
	}
	s.onlineSeen[deviceID] = now
	// 防止 map 无界增长：条目过多时清理掉已过节的项
	if len(s.onlineSeen) > 10000 {
		for id, t := range s.onlineSeen {
			if now.Sub(t) >= deviceOnlineThrottle {
				delete(s.onlineSeen, id)
			}
		}
	}
	s.onlineMu.Unlock()

	if err := s.dbClient.WithCtx(ctx).DeviceOnlineById(deviceID); err != nil && s.lc != nil {
		s.lc.Warnf("标记设备在线失败 device=%s: %v", deviceID, err)
	}
}

// New 构造接入服务
func New(dic *di.Container) *Service {
	return &Service{
		dic:        dic,
		lc:         pkgcontainer.LoggingClientFrom(dic.Get),
		dbClient:   coreContainer.DBClientFrom(dic.Get),
		messageItf: coreContainer.MessageItfFrom(dic.Get),
		gatewayItf: coreContainer.GatewayItfFrom(dic.Get),
	}
}

// Start 连接 MQTT 并订阅网关上行主题，同时把自身注册为下行发布器
func (s *Service) Start() error {
	req := dtos.NewMQTTClient{
		Broker:   s.config().MessageQueue.URL(),
		ClientId: "iotserver-core-gateway",
		Username: s.config().MessageQueue.Optional["Username"],
		Password: s.config().MessageQueue.Optional["Password"],
		SubTopics: []string{
			gatewaymqtt.TopicPrefixUp + "+/data",
			gatewaymqtt.TopicPrefixUp + "+/batch",
			gatewaymqtt.TopicPrefixUp + "+/status",
			gatewaymqtt.TopicPrefixUp + "+/offline",
		},
	}
	handler := func(_ mqtt.Client, message mqtt.Message) {
		if err := s.HandleMessage(message.Topic(), message.Payload()); err != nil && s.lc != nil {
			s.lc.Errorf("网关上报处理失败 topic=%s err=%v", message.Topic(), err)
		}
	}
	client, err := pkgMQTT.NewMQTTClient(req, s.lc, handler,
		func(ctx context.Context) { s.lc.Info("gateway mqtt connected") },
		func(ctx context.Context, msg dtos.CallbackMessage) { s.lc.Info("gateway mqtt disconnected") })
	if err != nil {
		return fmt.Errorf("网关接入 MQTT 连接失败: %w", err)
	}
	s.mqttClient = client
	gatewaymqtt.SetPublisher(s)
	return nil
}

// Publish 实现 gatewaymqtt.Publisher（平台 -> 网关下行）
func (s *Service) Publish(topic string, payload []byte) error {
	if s.mqttClient == nil {
		return gatewaymqtt.ErrNotReady
	}
	s.mqttClient.AsyncPublish(nil, topic, payload, false)
	return nil
}

func (s *Service) config() *coreConfig.ConfigurationStruct {
	return coreContainer.ConfigurationFrom(s.dic.Get)
}

// HandleMessage 处理一条网关上行报文（与传输解耦，便于单测直接调用）
func (s *Service) HandleMessage(topic string, payload []byte) error {
	sn, kind, ok := ParseTopic(topic)
	if !ok {
		return fmt.Errorf("非法的网关主题: %s", topic)
	}
	// 平台作用域：此刻还没有租户上下文，先靠 SN 定位网关（SN 全局唯一）
	platformCtx := tenant.Platform(context.Background())
	gateway, err := s.dbClient.WithCtx(platformCtx).GatewayBySn(sn)
	if err != nil {
		// 未注册网关：记录后拒绝，避免任意设备注入数据
		return fmt.Errorf("未注册的网关 sn=%s", sn)
	}
	// 之后的读写都在该网关所属租户的作用域内进行
	tenantCtx := tenant.With(platformCtx, tenant.Context{TenantID: gateway.TenantId, Platform: false})

	switch kind {
	case "status":
		return s.handleStatus(tenantCtx, gateway, payload)
	case "offline":
		return s.gatewayItf.GatewayOffline(tenantCtx, sn)
	case "data":
		return s.handleData(tenantCtx, gateway, payload)
	case "batch":
		return s.handleData(tenantCtx, gateway, payload)
	default:
		return fmt.Errorf("未知的上行类型: %s", kind)
	}
}

func (s *Service) handleStatus(ctx context.Context, g models.Gateway, payload []byte) error {
	var msg GatewayStatusMessage
	if err := json.Unmarshal(payload, &msg); err != nil {
		return fmt.Errorf("状态报文解析失败: %w", err)
	}
	if msg.Ts <= 0 {
		msg.Ts = time.Now().UnixMilli()
	}
	if !msg.Online {
		return s.gatewayItf.GatewayOffline(ctx, g.Sn)
	}
	return s.gatewayItf.GatewayOnline(ctx, g.Sn, msg.Ip, msg.Ts)
}

// handleData 处理实时上报与批量补传：解析 → 幂等 → 逐设备落库
func (s *Service) handleData(ctx context.Context, g models.Gateway, payload []byte) error {
	var msg GatewayUplinkMessage
	if err := json.Unmarshal(payload, &msg); err != nil {
		return fmt.Errorf("数据报文解析失败: %w", err)
	}
	if msg.Ts <= 0 {
		msg.Ts = time.Now().UnixMilli()
	}

	// 幂等：同一网关同一 seq 只处理一次（网关补传会重发）
	if msg.Seq > 0 {
		exists, err := s.dbClient.WithCtx(ctx).GatewayIngestLogExists(g.Id, msg.Seq)
		if err != nil {
			return err
		}
		if exists {
			if s.lc != nil {
				s.lc.Debugf("网关 %s 的 seq=%d 已处理，忽略重复报文", g.Sn, msg.Seq)
			}
			return nil
		}
	}

	records := msg.Records
	if len(records) == 0 {
		// 实时单条：把顶层字段当成唯一一条记录
		records = []GatewayUplinkRecord{{
			Ts:         msg.Ts,
			DeviceKey:  msg.DeviceKey,
			ProductKey: msg.ProductKey,
			Properties: msg.Properties,
			Events:     msg.Events,
		}}
	}

	count := 0
	var firstErr error
	for _, record := range records {
		n, err := s.ingestRecord(ctx, g, record, msg.Ts)
		count += n
		if err != nil && firstErr == nil {
			firstErr = err
		}
	}

	// 记录幂等日志（seq>0 时），失败也要留痕便于对账
	if msg.Seq > 0 {
		status := 1
		errText := ""
		if firstErr != nil {
			status = 0
			errText = firstErr.Error()
		}
		if err := s.dbClient.WithCtx(ctx).AddGatewayIngestLog(models.GatewayIngestLog{
			GatewayId:  g.Id,
			Seq:        msg.Seq,
			MsgCount:   count,
			Status:     status,
			Error:      errText,
			ReceivedAt: time.Now().UnixMilli(),
		}); err != nil && s.lc != nil {
			// 唯一索引冲突说明并发下已被处理过，属正常
			s.lc.Debugf("写入网关上报日志失败(可能重复): %v", err)
		}
	}
	_ = s.dbClient.WithCtx(ctx).TouchGatewayReport(g.Id, msg.Ts)
	// 收到数据即视为在线
	_ = s.gatewayItf.GatewayOnline(ctx, g.Sn, "", msg.Ts)

	if firstErr != nil {
		return firstErr
	}
	return nil
}

// ingestRecord 把一条记录转换成平台既有的物模型批量上报消息并落库
func (s *Service) ingestRecord(ctx context.Context, g models.Gateway, record GatewayUplinkRecord, fallbackTs int64) (int, error) {
	if strings.TrimSpace(record.DeviceKey) == "" {
		return 0, fmt.Errorf("记录缺少 deviceKey")
	}
	device, err := s.resolveDevice(ctx, g, record.DeviceKey, record.ProductKey)
	if err != nil {
		return 0, err
	}
	ts := record.Ts
	if ts <= 0 {
		ts = fallbackTs
	}
	if ts <= 0 {
		ts = time.Now().UnixMilli()
	}

	properties := make(map[string]dtos.BatchProperty, len(record.Properties))
	for code, p := range record.Properties {
		if p.Time <= 0 {
			p.Time = ts
		}
		properties[code] = p
	}
	events := make(map[string]dtos.BatchEvent, len(record.Events))
	for code, e := range record.Events {
		if e.EventTime <= 0 {
			e.EventTime = ts
		}
		events[code] = e
	}
	batchReport := dtos.DeviceBatchReport{
		MsgId: fmt.Sprintf("%s-%d", g.Sn, ts),
		Time:  ts,
	}
	batchReport.Data.Properties = properties
	batchReport.Data.Events = events
	// 直接序列化批报：落库侧按 DeviceBatchReport 解析，
	// 推 eKuiper 侧（TransformMessageBus）取其中的 data 节点，两者共用这一种结构。
	body, err := json.Marshal(batchReport)
	if err != nil {
		return 0, err
	}

	// 复用既有上行管道：落库 + 推 eKuiper（告警/场景/规则照常生效）
	_, err = s.messageItf.ThingModelMsgReport(ctx, dtos.ThingModelMessage{
		Cid:    device.Id,
		OpType: int32(thingmodel.OperationType_DATA_BATCH_REPORT),
		Data:   string(body),
	})
	if err != nil {
		return 0, err
	}
	// 落库成功即认为设备在线（与网关侧「收到数据即视为在线」一致）
	s.markDeviceOnline(ctx, device.Id)
	return len(properties) + len(events), nil
}

// resolveDevice 按网关内的 deviceKey 定位设备；未注册时给出明确错误
func (s *Service) resolveDevice(ctx context.Context, g models.Gateway, deviceKey, productKey string) (models.Device, error) {
	device, err := s.dbClient.WithCtx(ctx).DeviceByGatewayAndKey(g.Id, deviceKey)
	if err == nil {
		return device, nil
	}
	// 兼容：设备可能先于网关绑定存在，按设备编码兜底
	if d, dErr := s.dbClient.WithCtx(ctx).DeviceBySn(deviceKey); dErr == nil {
		return d, nil
	}
	return models.Device{}, fmt.Errorf("网关 %s 上报的设备 %s 未注册（productKey=%s）", g.Sn, deviceKey, productKey)
}

// ParseTopic 解析上行主题，返回网关 SN 与类型
func ParseTopic(topic string) (sn, kind string, ok bool) {
	if !strings.HasPrefix(topic, gatewaymqtt.TopicPrefixUp) {
		return "", "", false
	}
	rest := strings.TrimPrefix(topic, gatewaymqtt.TopicPrefixUp)
	parts := strings.Split(rest, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
}
