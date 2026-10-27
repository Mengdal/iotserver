// Package edgeingest 接入参考系统（iotddc / things）的 /edge 协议。
//
// 设计见 docs/device-access-design.md。与平台自有网关协议（application/gatewayingest，
// 主题 iot/up/{sn}/...）**并存且互不影响**：本服务持有自己的 MQTT 连接与订阅，
// 可以单独关闭而不动用自有协议。
//
// 与自有协议的三处关键差异（都来自实测）：
//
//  1. 数据报文**没有 seq 字段**，幂等改用「整包时间戳（毫秒）」当 seq，
//     直接复用 (gateway_id, seq) 唯一索引，无需新增表或改索引。
//  2. 设备匹配键是 dn（落 device_key），**不是设备名**。参考系统按设备名匹配
//     （PropertyPostProcessor: d.getName().equals(dn)），改个名字就断链；
//     我们按 key 匹配，name 只作展示。
//  3. 设备**允许没有产品与物模型**：清单（/edge/connect 的 tags[]）与数据报文里的
//     测点写入 device_tag 表，读取侧的单位/类型从那里回落。
//
// 落库仍复用平台既有上行管道（MessageItf.ThingModelMsgReport），因此告警规则、
// 场景联动、看板等上层能力对兼容接入的设备照常生效。
package edgeingest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/LMgateways/iotserver-driver-proto/thingmodel"
	mqtt "github.com/eclipse/paho.mqtt.golang"

	"iotserver/internal/dtos"
	coreConfig "iotserver/internal/iotserver/core/config"
	coreContainer "iotserver/internal/iotserver/core/container"
	interfaces "iotserver/internal/iotserver/core/interface"
	"iotserver/internal/models"
	"iotserver/internal/pkg/constants"
	pkgcontainer "iotserver/internal/pkg/container"
	"iotserver/internal/pkg/di"
	"iotserver/internal/pkg/errort"
	"iotserver/internal/pkg/logger"
	"iotserver/internal/pkg/tenant"
	"iotserver/internal/tools/edgeprotocol"
	pkgMQTT "iotserver/internal/tools/mqttclient"
)

const (
	// mqttClientIDPrefix 订阅参考系统上行所用的 clientId 前缀。
	// 实际 clientId 会拼上实例后缀，避免多个平台实例连同一个 broker 时互相踢下线。
	mqttClientIDPrefix = "iotserver-core-edge"
	// deviceOnlineThrottle 同一设备最多多久写一次「置在线」。
	// 网关是按包上报的，同一设备不必每包都写一次元数据库。
	deviceOnlineThrottle = 30 * time.Second
	// defaultResyncInterval 订阅对齐的默认间隔（配置未指定时）。
	// 它决定「界面新增网关 → 多久开始收它的数据」。
	defaultResyncInterval = 60 * time.Second
)

// errSkipDevice 表示「该设备按配置被跳过」，属于预期情况而非故障。
//
// 单独用一个哨兵值区分出来，是因为它对**整包**的影响不同：
// 一个被排除的设备（如实测里网关把自己上报成 dn=system）不该让同一包里
// 其它设备的数据一起算失败，也不该每包刷一条 error 日志。
var errSkipDevice = errors.New("device skipped by configuration")

// Service 参考系统接入服务
type Service struct {
	dic        *di.Container
	lc         logger.LoggingClient
	dbClient   interfaces.DBClient
	messageItf interfaces.MessageItf
	gatewayItf interfaces.GatewayItf
	mqttClient pkgMQTT.MQTTClient

	onlineMu   sync.Mutex
	onlineSeen map[string]time.Time

	// subMu / subscribed 当前已订阅的主题集合，用于增量对齐（退订消失的网关）
	subMu      sync.Mutex
	subscribed map[string]struct{}
	// stopCh 关闭 resync 循环
	stopCh chan struct{}
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

func (s *Service) config() *coreConfig.ConfigurationStruct {
	return coreContainer.ConfigurationFrom(s.dic.Get)
}

// Start 连接参考系统的 broker，并按已登记网关逐个订阅 /edge 上行主题。
//
// broker 取 [EdgeIngest].Broker（未配置则回落到 MessageQueue），主题**按网关逐个拼**，
// 不用通配——理由见 edgeprotocol.TopicsForGateway。
func (s *Service) Start() error {
	cfg := s.config()
	username, password := cfg.EdgeIngest.Credentials(cfg.MessageQueue)
	req := dtos.NewMQTTClient{
		Broker:   cfg.EdgeIngest.BrokerURL(cfg.MessageQueue),
		ClientId: pkgMQTT.InstanceClientID(mqttClientIDPrefix),
		Username: username,
		Password: password,
		// 初始不订阅：主题要按网关表逐个拼，见下面的 syncSubscriptions
	}
	handler := func(_ mqtt.Client, message mqtt.Message) {
		if err := s.HandleMessage(message.Topic(), message.Payload()); err != nil && s.lc != nil {
			s.lc.Errorf("参考系统上报处理失败 topic=%s err=%v", message.Topic(), err)
		}
	}
	client, err := pkgMQTT.NewMQTTClient(req, s.lc, handler,
		func(ctx context.Context) {
			s.lc.Info("参考系统接入 mqtt connected")
			// 重连后立即对齐一次订阅（客户端也会重订已知主题，这里兜底新增的网关）
			if err := s.syncSubscriptions(); err != nil {
				s.warnf("参考系统接入：重连后对齐订阅失败: %v", err)
			}
		},
		func(ctx context.Context, msg dtos.CallbackMessage) { s.lc.Info("参考系统接入 mqtt disconnected") })
	if err != nil {
		return errort.NewMessageErr("参考系统接入 MQTT 连接失败: {{.p1}}", map[string]interface{}{"p1": err})
	}
	s.mqttClient = client
	s.stopCh = make(chan struct{})

	if err := s.syncSubscriptions(); err != nil {
		// 订阅失败不阻断启动：网关可能还没建，后续 resync 会补上
		s.warnf("参考系统接入：初始订阅失败（等待下次对齐）: %v", err)
	}
	go s.resyncLoop(resyncInterval(cfg.EdgeIngest.ResyncIntervalSec))
	return nil
}

// resyncInterval 订阅刷新间隔：配置 <=0 时用默认值
func resyncInterval(sec int) time.Duration {
	if sec <= 0 {
		return defaultResyncInterval
	}
	return time.Duration(sec) * time.Second
}

// resyncLoop 周期性对齐订阅，使界面上增删网关无需重启即可生效
func (s *Service) resyncLoop(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			if err := s.syncSubscriptions(); err != nil {
				s.warnf("参考系统接入：订阅对齐失败: %v", err)
			}
		}
	}
}

// syncSubscriptions 按当前网关表对齐订阅：新网关订阅、已删网关退订。
//
// 只订阅**平台里登记过的网关**。参考系统 broker 上还有别的客户的网关，
// 通配订阅会把它们的数据一并拉进来（该 broker 实测允许匿名订阅）。
func (s *Service) syncSubscriptions() error {
	// 没有租户上下文：这里订的是「哪些网关属于本平台」，用平台作用域。
	ctx := tenant.Platform(context.Background())
	gateways, _, err := s.dbClient.WithCtx(ctx).GatewaySearch(0, -1, dtos.GatewaySearchQueryRequest{})
	if err != nil {
		return errort.NewMessageErr("读取网关列表失败: {{.p1}}", map[string]interface{}{"p1": err})
	}

	desired := make(map[string]struct{}, len(gateways)*4)
	for _, g := range gateways {
		// things 中已删除的网关只保留本地镜像/历史数据，不再订阅新数据。
		if g.ThingsSyncStatus == models.GatewayThingsSyncMissing {
			continue
		}
		for _, topic := range edgeprotocol.TopicsForGateway(g.Pk, g.Sn) {
			desired[topic] = struct{}{}
		}
	}

	s.subMu.Lock()
	if s.subscribed == nil {
		s.subscribed = map[string]struct{}{}
	}
	var added, removed []string
	for topic := range desired {
		if _, ok := s.subscribed[topic]; !ok {
			added = append(added, topic)
		}
	}
	for topic := range s.subscribed {
		if _, ok := desired[topic]; !ok {
			removed = append(removed, topic)
		}
	}
	s.subMu.Unlock()

	if len(added) == 0 && len(removed) == 0 {
		return nil
	}
	if s.mqttClient != nil {
		if len(added) > 0 {
			if err := s.mqttClient.Subscribe(added...); err != nil {
				return errort.NewMessageErr("订阅失败: {{.p1}}", map[string]interface{}{"p1": err})
			}
		}
		if len(removed) > 0 {
			if err := s.mqttClient.Unsubscribe(removed...); err != nil {
				// 退订失败不致命：多收一个主题不会出错，下次对齐还会再试
				s.warnf("参考系统接入：退订失败: %v", err)
			}
		}
	}

	s.subMu.Lock()
	s.subscribed = desired
	s.subMu.Unlock()
	s.infof("参考系统接入：订阅已对齐 —— 网关 %d 台，主题 %d 个（新增 %d、退订 %d）",
		len(gateways), len(desired), len(added), len(removed))
	return nil
}

// HandleMessage 处理一条 /edge 上行报文（与传输解耦，便于单测直接调用）
func (s *Service) HandleMessage(topic string, payload []byte) error {
	kind, pk, sn, ok := edgeprotocol.ParseTopic(topic)
	if !ok {
		return errort.NewMessageErr("非法的参考系统主题: {{.p1}}", map[string]interface{}{"p1": topic})
	}
	// 此刻还没有租户上下文，先靠 (pk, sn) 定位网关
	platformCtx := tenant.Platform(context.Background())
	gateway, err := s.dbClient.WithCtx(platformCtx).GatewayByPkSn(pk, sn)
	if err != nil {
		// 未注册网关：记录后拒绝，避免任意设备注入数据
		return errort.NewMessageErr("未注册的网关 pk={{.p1}} sn={{.p2}}", map[string]interface{}{"p1": pk, "p2": sn})
	}
	// 之后的读写都在该网关所属租户的作用域内进行
	tenantCtx := tenant.With(platformCtx, tenant.Context{TenantID: gateway.TenantId, Platform: false})

	switch kind {
	case edgeprotocol.KindConnect:
		return s.handleConnect(tenantCtx, gateway, payload)
	case edgeprotocol.KindProperty, edgeprotocol.KindPropertyHistory:
		return s.handleProperty(tenantCtx, gateway, payload)
	case edgeprotocol.KindLastwill:
		return s.gatewayItf.GatewayOffline(tenantCtx, gateway.Sn)
	default:
		return errort.NewMessageErr("未支持的参考系统上行类型: {{.p1}}", map[string]interface{}{"p1": kind})
	}
}

// handleConnect 处理网关接入报文：更新在线状态 + 按 tags[] 全量对齐设备与测点。
//
// 清单被视为**权威全集**（与参考系统一致）：清单里没有的设备标记为摘除。
// 与参考系统的差别是**不物理删除**——时序历史在时序库里删不掉，
// 而删除元数据会断掉告警规则/组态的引用。
func (s *Service) handleConnect(ctx context.Context, g models.Gateway, payload []byte) error {
	msg, err := edgeprotocol.ParseConnect(payload)
	if err != nil {
		return err
	}
	now := time.Now().UnixMilli()
	// 接入即在线；ip 由报文给出
	if err := s.gatewayItf.GatewayOnline(ctx, g.Sn, msg.Ip, now); err != nil {
		s.warnf("参考系统接入：网关 %s 置在线失败: %v", g.Sn, err)
	}

	if g.AutoProvision == models.GatewayAutoProvisionOff {
		s.infof("参考系统接入：网关 %s 关闭了自动注册，跳过 %d 条设备清单", g.Sn, len(msg.Tags))
		return nil
	}

	excluded := excludeSet(g.ExcludeDeviceKeys)
	// 按 deviceCode 分组：同一台设备的多条 tag 归到一起
	type devGroup struct {
		description string
		tags        []edgeprotocol.Tag
	}
	groups := map[string]*devGroup{}
	order := make([]string, 0, len(msg.Tags))
	skipped := 0
	for _, t := range msg.Tags {
		if _, skip := excluded[t.DeviceCode]; skip {
			skipped++
			continue
		}
		grp, ok := groups[t.DeviceCode]
		if !ok {
			grp = &devGroup{description: t.DeviceDescription}
			groups[t.DeviceCode] = grp
			order = append(order, t.DeviceCode)
		}
		if grp.description == "" {
			grp.description = t.DeviceDescription
		}
		if t.TagCode != "" {
			grp.tags = append(grp.tags, t)
		}
	}

	existing, err := s.dbClient.WithCtx(ctx).ListGatewayDevices(g.Id)
	if err != nil {
		return err
	}

	created, updated, tagsCreated := 0, 0, 0
	var firstErr error
	for _, deviceKey := range order {
		grp := groups[deviceKey]
		device, isNew, err := s.dbClient.WithCtx(ctx).UpsertGatewayDevice(models.Device{
			GatewayId:       g.Id,
			DeviceKey:       deviceKey,
			DeviceSn:        deviceKey,
			Name:            displayName(grp.description, deviceKey),
			Description:     grp.description,
			DeptId:          g.DeptId,
			Platform:        constants.IotPlatform_Things,
			Status:          constants.DeviceStatusOffline,
			ProvisionSource: models.DeviceSourceGateway,
			ProvisionState:  models.DeviceStateActive,
		})
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		if isNew {
			created++
		} else {
			updated++
		}
		delete(existing, deviceKey)

		tags := make([]models.DeviceTag, 0, len(grp.tags))
		for _, t := range grp.tags {
			tags = append(tags, models.DeviceTag{
				Code:       t.TagCode,
				Name:       displayName(t.Description, t.TagCode),
				AccessMode: edgeprotocol.AccessMode(t.ObjectType),
				Source:     models.DeviceTagSourceAuto,
			})
		}
		n, err := s.dbClient.WithCtx(ctx).UpsertDeviceTags(device.Id, tags)
		tagsCreated += n
		if err != nil && firstErr == nil {
			firstErr = err
		}
	}

	// 清单外设备 → 标记摘除
	absentIds := make([]string, 0, len(existing))
	for _, d := range existing {
		absentIds = append(absentIds, d.Id)
	}
	absent, err := s.dbClient.WithCtx(ctx).MarkDevicesAbsent(absentIds)
	if err != nil && firstErr == nil {
		firstErr = err
	}

	s.infof("参考系统接入：网关 %s 清单 %d 条 → 新增设备 %d、更新 %d、新增测点 %d、摘除 %d、排除 %d",
		g.Sn, len(msg.Tags), created, updated, tagsCreated, absent, skipped)
	if firstErr != nil {
		return firstErr
	}
	return nil
}

// handleProperty 处理数据报文（实时与历史补传同路，区别只在 topic）。
func (s *Service) handleProperty(ctx context.Context, g models.Gateway, payload []byte) error {
	reports, err := edgeprotocol.ParsePropertyPost(payload)
	if err != nil {
		return err
	}
	if len(reports) == 0 {
		return nil
	}

	// 幂等：兼容协议没有 seq，用整包时间戳（毫秒）当 seq。
	// 实测整包一个时间、周期 60 秒，粒度足够；缺时间时用当前时间兜底，
	// 保证 seq > 0（seq=0 会与其它同网关报文在 (gateway_id, seq) 上冲突）。
	seq := reports[0].TimeMs
	if seq <= 0 {
		seq = time.Now().UnixMilli()
	}
	exists, err := s.dbClient.WithCtx(ctx).GatewayIngestLogExists(g.Id, seq)
	if err != nil {
		return err
	}
	if exists {
		s.debugf("参考系统接入：网关 %s 的报文(seq=%d)已处理，忽略重复", g.Sn, seq)
		return nil
	}

	pointCount := 0
	var firstErr error
	for _, r := range reports {
		n, err := s.ingestReport(ctx, g, r)
		pointCount += n
		if err != nil && firstErr == nil {
			firstErr = err
		}
	}

	status, errText := 1, ""
	if firstErr != nil {
		status, errText = 0, firstErr.Error()
	}
	if err := s.dbClient.WithCtx(ctx).AddGatewayIngestLog(models.GatewayIngestLog{
		GatewayId:  g.Id,
		Seq:        seq,
		MsgCount:   pointCount,
		Status:     status,
		Error:      errText,
		ReceivedAt: time.Now().UnixMilli(),
	}); err != nil {
		// 唯一索引冲突说明并发下已被处理过，属正常
		s.debugf("参考系统接入：写上报日志失败(可能重复): %v", err)
	}
	// 存活信号用**接收时间**，不能用报文里的采集时间 seq：
	// /edge/property/history/... 这条通道正是为断点续传存在的——网关掉线几小时后
	// 重连，补传的是几小时前的数据，时间戳自然是旧的。若拿它写 last_online_at，
	// 离线扫描会立刻把这个**正在积极上报**的网关判为离线。
	// （实测：发一条 1 小时前时间戳的报文，网关当场从「在线」变「离线」。）
	// 采集时间仍然用于数据行本身（见 ingestReport），两件事必须分开。
	receivedAt := time.Now().UnixMilli()
	_ = s.dbClient.WithCtx(ctx).TouchGatewayReport(g.Id, receivedAt)
	// 收到数据即视为在线
	_ = s.gatewayItf.GatewayOnline(ctx, g.Sn, "", receivedAt)

	if firstErr != nil {
		return firstErr
	}
	return nil
}

// ingestReport 把一包中的一台设备转换成平台既有的批量上报消息并落库。
func (s *Service) ingestReport(ctx context.Context, g models.Gateway, r edgeprotocol.DeviceReport) (int, error) {
	if len(r.Properties) == 0 {
		return 0, nil
	}
	device, err := s.resolveOrProvision(ctx, g, r)
	if err != nil {
		// 按配置跳过（排除名单 / 关闭自动注册）属预期情况：
		// 跳过该设备，但不能连累同一包里其它设备的数据
		if errors.Is(err, errSkipDevice) {
			return 0, nil
		}
		return 0, err
	}
	ts := r.TimeMs
	if ts <= 0 {
		ts = time.Now().UnixMilli()
	}

	properties := make(map[string]dtos.BatchProperty, len(r.Properties))
	for code, raw := range r.Properties {
		// 源报文的值全是字符串，必须转成数值再落库：
		// 时序层对 string 一律写进 text 列，数值查询（历史曲线/min/max/avg）会查不到。
		properties[code] = dtos.BatchProperty{Value: edgeprotocol.NumericOrString(raw), Time: ts}
	}
	batchReport := dtos.DeviceBatchReport{
		MsgId: fmt.Sprintf("%s-%s-%d", g.Sn, device.DeviceKey, ts),
		Time:  ts,
	}
	batchReport.Data.Properties = properties
	body, err := json.Marshal(batchReport)
	if err != nil {
		return 0, err
	}

	// 复用既有上行管道：落时序库 + 推 eKuiper（告警/场景/规则照常生效）
	if _, err := s.messageItf.ThingModelMsgReport(ctx, dtos.ThingModelMessage{
		Cid:    device.Id,
		OpType: int32(thingmodel.OperationType_DATA_BATCH_REPORT),
		Data:   string(body),
	}); err != nil {
		return 0, err
	}
	s.markDeviceOnline(ctx, device.Id)
	return len(properties), nil
}

// resolveOrProvision 定位设备；未注册时**惰性注册**后返回。
//
// 惰性注册是必需的：MQTT 只保证至少一次投递，清单报文可能丢失、也可能晚于数据到达，
// 而数据报文本身已带够注册所需的最小信息（dn）。若此处直接拒绝，
// 就会出现「清单没到 → 数据静默丢弃」。
func (s *Service) resolveOrProvision(ctx context.Context, g models.Gateway, r edgeprotocol.DeviceReport) (models.Device, error) {
	device, err := s.dbClient.WithCtx(ctx).DeviceByGatewayAndKey(g.Id, r.DeviceKey)
	if err == nil {
		return device, nil
	}
	// 只有「查不到」才继续注册；数据库故障等其它错误必须原样上抛，否则会掩盖问题
	if !errort.Is(errort.DeviceNotExist, err) {
		return models.Device{}, err
	}
	if g.AutoProvision == models.GatewayAutoProvisionOff {
		s.debugf("参考系统接入：网关 %s 关闭了自动注册，跳过未注册设备 %s", g.Sn, r.DeviceKey)
		return models.Device{}, errSkipDevice
	}
	if _, excluded := excludeSet(g.ExcludeDeviceKeys)[r.DeviceKey]; excluded {
		s.debugf("参考系统接入：网关 %s 的设备 %s 在排除名单中，跳过", g.Sn, r.DeviceKey)
		return models.Device{}, errSkipDevice
	}

	device, isNew, err := s.dbClient.WithCtx(ctx).UpsertGatewayDevice(models.Device{
		GatewayId:       g.Id,
		DeviceKey:       r.DeviceKey,
		DeviceSn:        r.DeviceKey,
		Name:            r.DeviceKey,
		DeptId:          g.DeptId,
		Platform:        constants.IotPlatform_Things,
		Status:          constants.DeviceStatusOffline,
		ProvisionSource: models.DeviceSourceGateway,
		ProvisionState:  models.DeviceStateActive,
	})
	if err != nil {
		return models.Device{}, err
	}
	if isNew {
		s.infof("参考系统接入：网关 %s 的数据报文自动注册设备 %s（清单未到或未覆盖）", g.Sn, r.DeviceKey)
		// 顺手记下这包观测到的测点，让设备至少有一份可读的测点定义。
		// 不在每包都写：那会给热路径增加一次元数据库往返。
		tags := make([]models.DeviceTag, 0, len(r.Properties))
		for code, raw := range r.Properties {
			tags = append(tags, models.DeviceTag{
				Code:   code,
				Name:   code,
				Type:   inferTagType(raw, edgeprotocol.NumericOrString(raw)),
				Source: models.DeviceTagSourceAuto,
			})
		}
		if _, err := s.dbClient.WithCtx(ctx).UpsertDeviceTags(device.Id, tags); err != nil {
			s.warnf("参考系统接入：设备 %s 自动登记测点失败: %v", r.DeviceKey, err)
		}
	}
	return device, nil
}

// markDeviceOnline 收到数据即视为设备在线（带节流，避免高频上报写爆元数据库）。
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
	// 防止 map 无界增长
	if len(s.onlineSeen) > 10000 {
		for id, t := range s.onlineSeen {
			if now.Sub(t) >= deviceOnlineThrottle {
				delete(s.onlineSeen, id)
			}
		}
	}
	s.onlineMu.Unlock()

	if err := s.dbClient.WithCtx(ctx).DeviceOnlineById(deviceID); err != nil {
		s.warnf("参考系统接入：标记设备在线失败 device=%s: %v", deviceID, err)
	}
}

/* ------------------------------ 小工具 ------------------------------ */

// excludeSet 解析「排除的设备标识」名单（逗号分隔，如 "system,user"）。
//
// 实测网关会把自己的时钟上报为设备（dn=system），这类不应进入设备列表。
func excludeSet(raw string) map[string]struct{} {
	out := map[string]struct{}{}
	for _, part := range strings.Split(raw, ",") {
		if p := strings.TrimSpace(part); p != "" {
			out[p] = struct{}{}
		}
	}
	return out
}

// displayName 校验展示名：优先用描述，缺失时退回标识本身。
func displayName(description, fallback string) string {
	if d := strings.TrimSpace(description); d != "" {
		return d
	}
	return fallback
}

// inferTagType 按源报文里的**原始字符串形态**推断测点类型。
//
// 必须用原始字符串而不是转换后的数值：转换会丢掉形态信息——"30.0000" 经
// ParseFloat 变成 30，看起来像整数，但源数据显然带小数位。
// 单位无法从数据推断，留空待人工校准。
func inferTagType(raw string, v interface{}) string {
	switch v.(type) {
	case float64:
		if strings.ContainsAny(strings.TrimSpace(raw), ".eE") {
			return "float"
		}
		return "int"
	case bool:
		return "bool"
	default:
		return "text"
	}
}

func (s *Service) infof(format string, args ...interface{}) {
	if s.lc != nil {
		s.lc.Infof(format, args...)
	}
}

func (s *Service) warnf(format string, args ...interface{}) {
	if s.lc != nil {
		s.lc.Warnf(format, args...)
	}
}

func (s *Service) debugf(format string, args ...interface{}) {
	if s.lc != nil {
		s.lc.Debugf(format, args...)
	}
}
