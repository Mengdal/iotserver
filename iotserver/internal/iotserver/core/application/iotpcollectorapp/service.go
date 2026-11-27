// Package iotpcollectorapp 订阅 iotp 集群各节点的 ingest/<库名>/json，
// 把消息转换成平台的 MessageBus 报文后写入内部消息总线。
//
// 背景：iotp 每个节点有独立的 MQTT broker，平台规则引擎只有一条共享流
// （eventbus/in）。本服务负责把多节点数据汇入这条流，规则作者无需关心节点。
//
// 订阅的 broker 有两个来源：静态配置的 Nodes，以及受管软网关的采集引擎
// （source=ioserver）实际发布的 MQTT 目标。两者按地址去重，静态配置优先。
package iotpcollectorapp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"

	"iotserver/internal/dtos"
	"iotserver/internal/iotserver/core/config"
	"iotserver/internal/iotserver/core/container"
	interfaces "iotserver/internal/iotserver/core/interface"
	"iotserver/internal/models"
	pkgContainer "iotserver/internal/pkg/container"
	"iotserver/internal/pkg/di"
	"iotserver/internal/pkg/logger"
	"iotserver/internal/pkg/tenant"
	"iotserver/internal/tools/collectengine"
	"iotserver/internal/tools/iotp"
	pkgMQTT "iotserver/internal/tools/mqttclient"
)

const (
	defaultMessageType      = "PROPERTY_REPORT"
	defaultResyncSeconds    = 60
	defaultEngineTimeoutSec = 10
)

// nodeSpec 一个订阅目标。id 用于日志/ clientId，broker 是规范化后的地址。
type nodeSpec struct {
	Id     string
	Broker string
}

// nodeConn 一个节点 broker 连接。
type nodeConn struct {
	id     string
	broker string
	client mqtt.Client
}

// gatewayBroker 受管软网关最近一次成功发现的 MQTT 目标。
type gatewayBroker struct {
	Broker string
	Label  string
}

// deviceInfo iotp 设备 ID 对应的平台设备摘要及生效租户。
// 只保留转发消息需要的字段，租户优先取设备自身，设备没归属时取采集它的软网关。
type deviceInfo struct {
	Id          string
	Name        string
	ProductId   string
	ProductName string
	TenantId    string
}

// gatewayBinding 平台里的设备 → 采集网关绑定。
type gatewayBinding struct {
	GatewayId string
	TenantId  string
}

// Service iotp 多节点采集服务。
type Service struct {
	lc       logger.LoggingClient
	dbClient interfaces.DBClient

	collectCfg config.IotpCollectInfo
	mqCfg      config.MessageQueueInfo
	// enabled 由 [Databases.Data].BaseURL 决定：接入 iotp 即启用。
	enabled bool
	// staticNodes 启动时解析一次静态节点配置，刷新拓扑时复用。
	staticNodes []nodeSpec

	ctx context.Context

	mu       sync.RWMutex
	out      mqtt.Client
	nodes    map[string]*nodeConn // key: 规范化 broker 地址
	devices  map[int64]deviceInfo // iotp device id -> 平台设备
	outTopic string
	// subscribeTopic 由 [Databases.Data].Name 推导，形如 ingest/<db>/json。
	subscribeTopic string
	// discoverCh 网关 CRUD 触发的拓扑刷新信号；容量 1，多次触发合并。
	discoverCh chan struct{}
}

// New 创建服务；未启用时 Start 为 no-op。
func New(dic *di.Container) *Service {
	s := &Service{
		lc:         pkgContainer.LoggingClientFrom(dic.Get),
		nodes:      map[string]*nodeConn{},
		devices:    map[int64]deviceInfo{},
		discoverCh: make(chan struct{}, 1),
	}
	cfg := container.ConfigurationFrom(dic.Get)
	if cfg == nil {
		return s
	}
	s.collectCfg = cfg.Integrations.IotpCollect
	s.mqCfg = cfg.MessageQueue
	db, enabled := cfg.IotpConfig()
	s.enabled = enabled
	if name := db.IotpDatabaseName(); !strings.ContainsAny(name, "/+#") {
		s.subscribeTopic = "ingest/" + name + "/json"
	} else {
		s.lc.Errorf("iotp 采集未启用：[Databases.Data].Name 非法: %q", name)
		s.enabled = false
	}
	s.outTopic = strings.TrimSpace(s.mqCfg.PublishTopicPrefix)
	if s.outTopic == "" {
		s.outTopic = "eventbus/in"
	}
	if s.collectCfg.ResyncIntervalSec <= 0 {
		s.collectCfg.ResyncIntervalSec = defaultResyncSeconds
	}
	if db := container.DBClientFrom(dic.Get); db != nil {
		s.dbClient = db
	}
	return s
}

// Start 启动采集；未启用时直接返回。
//
// 节点来源有两种，按 broker 地址去重：
//   - Nodes 里的静态节点（优先级高，clientId 沿用 id）；
//   - 受管软网关的采集引擎：启动时发现一次，之后由网关 CRUD 事件触发刷新，
//     并按 ResyncIntervalSec 周期性重试，覆盖引擎晚启动或短暂不可达。
func (s *Service) Start(ctx context.Context) {
	if !s.enabled {
		s.lc.Infof("iotp 多节点采集未启用：[Databases.Data].BaseURL 为空")
		return
	}
	s.staticNodes = s.loadStaticNodes()
	s.ctx = ctx

	go s.syncDevices()
	go s.connectOutput()
	for _, spec := range s.staticNodes {
		s.ensureNode(spec.Id, spec.Broker)
	}
	go s.discoverLoop()
	s.lc.Infof("iotp 多节点采集已启动：静态节点=%d 自动发现=受管网关 topic=%s -> %s",
		len(s.staticNodes), s.subscribeTopic, s.outTopic)
}

// RefreshDiscovery 网关增删改后触发一次拓扑刷新；信号容量 1，多次调用会合并。
func (s *Service) RefreshDiscovery() {
	if s.discoverCh == nil {
		return
	}
	select {
	case s.discoverCh <- struct{}{}:
	default:
	}
}

func (s *Service) connectOutput() {
	opts := mqtt.NewClientOptions().
		AddBroker(s.mqCfg.URL()).
		SetClientID(pkgMQTT.InstanceClientID("iotserver-iotp-collect")).
		SetAutoReconnect(true).
		SetConnectRetry(true).
		SetConnectRetryInterval(3 * time.Second).
		SetConnectionLostHandler(func(_ mqtt.Client, err error) {
			s.lc.Warnf("iotp 采集输出连接断开: %v", err)
		}).
		SetOnConnectHandler(func(_ mqtt.Client) {
			s.lc.Infof("iotp 采集输出已连接: %s topic=%s", s.mqCfg.URL(), s.outTopic)
		})
	if s.mqCfg.Optional != nil {
		opts.SetUsername(s.mqCfg.Optional["Username"])
		opts.SetPassword(s.mqCfg.Optional["Password"])
	}
	client := mqtt.NewClient(opts)
	tok := client.Connect()
	tok.Wait()
	if tok.Error() != nil {
		s.lc.Errorf("iotp 采集输出连接失败: %v", tok.Error())
	}
	s.mu.Lock()
	s.out = client
	s.mu.Unlock()
}

// loadStaticNodes 解析 Nodes 配置，按 broker 去重；启动时调用一次。
func (s *Service) loadStaticNodes() []nodeSpec {
	specs, invalid := parseStaticNodes(s.collectCfg.Nodes)
	for _, entry := range invalid {
		s.lc.Warnf("iotp 采集节点配置非法，已跳过: %q", entry)
	}
	return specs
}

func parseStaticNodes(entries []string) (specs []nodeSpec, invalid []string) {
	seen := make(map[string]struct{}, len(entries))
	for _, entry := range entries {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		parts := strings.SplitN(entry, "=", 2)
		if len(parts) != 2 {
			invalid = append(invalid, entry)
			continue
		}
		id := strings.TrimSpace(parts[0])
		broker, err := brokerKey(parts[1])
		if id == "" || err != nil {
			invalid = append(invalid, entry)
			continue
		}
		if _, ok := seen[broker]; ok {
			continue
		}
		seen[broker] = struct{}{}
		specs = append(specs, nodeSpec{Id: id, Broker: broker})
	}
	return specs, invalid
}

// ensureNode 幂等建立一个节点连接；已存在同一 broker 时直接复用。
func (s *Service) ensureNode(id, broker string) {
	key, err := brokerKey(broker)
	if err != nil {
		s.lc.Warnf("iotp 采集节点地址非法，已跳过 %s: %v", broker, err)
		return
	}
	s.mu.Lock()
	if _, ok := s.nodes[key]; ok {
		s.mu.Unlock()
		return
	}
	opts := mqtt.NewClientOptions().
		AddBroker(key).
		SetClientID(pkgMQTT.InstanceClientID("iotserver-iotp-" + sanitizeClientID(id))).
		SetAutoReconnect(true).
		SetConnectRetry(true).
		SetConnectRetryInterval(3 * time.Second).
		SetConnectionLostHandler(func(_ mqtt.Client, err error) {
			s.lc.Warnf("iotp 节点 %s 连接断开: %v", id, err)
		}).
		SetOnConnectHandler(func(c mqtt.Client) {
			if !s.currentNode(key, c) {
				c.Disconnect(250)
				return
			}
			s.lc.Infof("iotp 节点 %s 已连接: %s", id, key)
			tok := c.Subscribe(s.subscribeTopic, 1, func(_ mqtt.Client, msg mqtt.Message) {
				s.handle(id, msg)
			})
			tok.Wait()
			if tok.Error() != nil {
				s.lc.Errorf("iotp 节点 %s 订阅失败: %v", id, tok.Error())
			}
		})
	if s.collectCfg.Username != "" {
		opts.SetUsername(s.collectCfg.Username)
		opts.SetPassword(s.collectCfg.Password)
	}
	client := mqtt.NewClient(opts)
	s.nodes[key] = &nodeConn{id: id, broker: key, client: client}
	s.mu.Unlock()

	s.lc.Infof("iotp 采集订阅节点 %s（%s）", id, key)
	go func() {
		tok := client.Connect()
		tok.Wait()
		if tok.Error() != nil {
			s.lc.Errorf("iotp 节点 %s 连接失败: %v", id, tok.Error())
		}
		if !s.currentNode(key, client) {
			client.Disconnect(250)
		}
	}()
}

func (s *Service) currentNode(key string, client mqtt.Client) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	conn, ok := s.nodes[key]
	return ok && conn.client == client
}

// reconcileNodes 让实际连接与期望节点集合一致：新增缺失的，断开多余的。
func (s *Service) reconcileNodes(desired map[string]nodeSpec) {
	for _, spec := range desired {
		s.ensureNode(spec.Id, spec.Broker)
	}

	s.mu.Lock()
	removed := make([]*nodeConn, 0)
	for key, conn := range s.nodes {
		if _, ok := desired[key]; ok {
			continue
		}
		delete(s.nodes, key)
		removed = append(removed, conn)
	}
	s.mu.Unlock()

	for _, conn := range removed {
		conn.client.Disconnect(250)
		s.lc.Infof("iotp 采集取消订阅节点 %s（%s）", conn.id, conn.broker)
	}
}

// discoverLoop 启动时发现一次，之后等待网关 CRUD 事件触发刷新，
// 并周期性重试：引擎启动晚于平台或短暂不可达时，恢复后不需要人工编辑网关。
func (s *Service) discoverLoop() {
	known := map[string]gatewayBroker{}
	s.refreshDiscovered(known)

	ticker := time.NewTicker(time.Duration(s.collectCfg.ResyncIntervalSec) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-s.discoverCh:
			s.refreshDiscovered(known)
		case <-ticker.C:
			s.refreshDiscovered(known)
		}
	}
}

// refreshDiscovered 读取所有受管软网关（source=ioserver）的采集引擎 MQTT 目标。
//
// 查询失败的网关沿用 known 里的上一次结果，避免引擎抖动导致订阅被摘除；
// 网关删除后不再出现在列表里，其地址会在本次对账中移除。
func (s *Service) refreshDiscovered(known map[string]gatewayBroker) {
	if s.dbClient == nil {
		s.lc.Warnf("iotp 自动发现不可用：数据库客户端为空")
		return
	}
	gateways, _, err := s.dbClient.WithCtx(tenant.Platform(s.ctx)).GatewaySearch(0, -1, dtos.GatewaySearchQueryRequest{
		BaseSearchConditionQuery: dtos.BaseSearchConditionQuery{IsAll: true},
		Source:                   models.GatewaySourceIoserver,
	})
	if err != nil {
		s.lc.Warnf("iotp 自动发现刷新网关列表失败，沿用上次结果: %v", err)
		return
	}

	next := make(map[string]gatewayBroker, len(gateways))
	for _, gw := range gateways {
		if strings.TrimSpace(gw.EngineBaseURL) == "" {
			continue
		}
		broker, err := s.engineBroker(gw)
		if err != nil {
			if previous, ok := known[gw.Id]; ok {
				next[gw.Id] = previous
			}
			s.lc.Warnf("iotp 自动发现网关 %s(%s) 的 MQTT 目标失败，沿用上次结果: %v", gw.Name, gw.Sn, err)
			continue
		}
		label := strings.TrimSpace(gw.Sn)
		if label == "" {
			label = gw.Id
		}
		next[gw.Id] = gatewayBroker{Broker: broker, Label: "gw-" + label}
		if previous, ok := known[gw.Id]; !ok || previous.Broker != broker {
			s.lc.Infof("iotp 自动发现网关 %s(%s) 的 MQTT 目标: %s", gw.Name, gw.Sn, broker)
		}
	}
	for id := range known {
		delete(known, id)
	}
	for id, info := range next {
		known[id] = info
	}

	s.reconcileNodes(mergeNodeSpecs(s.staticNodes, known))
	s.lc.Debugf("iotp 自动发现刷新完成：受管网关=%d", len(known))
}

// mergeNodeSpecs 合并静态节点与自动发现结果，按 broker 地址去重；静态节点优先。
// 网关按 ID 排序，保证多个网关指向同一 broker 时选中的标签稳定。
func mergeNodeSpecs(static []nodeSpec, discovered map[string]gatewayBroker) map[string]nodeSpec {
	desired := make(map[string]nodeSpec, len(static)+len(discovered))
	for _, spec := range static {
		desired[spec.Broker] = spec
	}
	ids := make([]string, 0, len(discovered))
	for id := range discovered {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		info := discovered[id]
		key, err := brokerKey(info.Broker)
		if err != nil {
			continue
		}
		if _, ok := desired[key]; ok {
			continue
		}
		desired[key] = nodeSpec{Id: info.Label, Broker: key}
	}
	return desired
}

// engineBroker 读取采集引擎实际发布的 MQTT broker 地址。
func (s *Service) engineBroker(gw models.Gateway) (string, error) {
	baseURL := strings.TrimSpace(gw.EngineBaseURL)
	if baseURL == "" {
		return "", fmt.Errorf("未配置采集引擎地址")
	}
	timeout := gw.EngineTimeoutSec
	if timeout <= 0 {
		timeout = defaultEngineTimeoutSec
	}
	client := collectengine.New(baseURL, time.Duration(timeout)*time.Second)
	cfg, err := client.GetMQTTConfig()
	if err != nil {
		return "", err
	}
	host := strings.TrimSpace(cfg.IP)
	if host == "" || cfg.Port <= 0 {
		return "", fmt.Errorf("引擎 MQTT 目标为空")
	}
	return "tcp://" + net.JoinHostPort(host, strconv.Itoa(cfg.Port)), nil
}

// brokerKey 规范化 broker 地址，作为去重键；缺省协议按 tcp 处理。
func brokerKey(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("地址为空")
	}
	if !strings.Contains(raw, "://") {
		raw = "tcp://" + raw
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" {
		return "", fmt.Errorf("非法 broker 地址: %s", raw)
	}
	scheme := strings.ToLower(parsed.Scheme)
	base := scheme + "://" + strings.ToLower(parsed.Host)
	switch scheme {
	case "tcp", "ssl":
		return base, nil
	case "ws", "wss":
		// WebSocket 的路径有语义（paho 会拼到握手请求里），保留用户写的 path。
		return base + parsed.EscapedPath(), nil
	default:
		return "", fmt.Errorf("不支持的 broker 协议: %s", parsed.Scheme)
	}
}

// sanitizeClientID 把节点标识限制为 MQTT clientId 可用的字符。
func sanitizeClientID(id string) string {
	var builder strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(id)) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			builder.WriteRune(r)
		} else {
			builder.WriteByte('-')
		}
		if builder.Len() >= 32 {
			break
		}
	}
	if builder.Len() == 0 {
		return "node"
	}
	return builder.String()
}

// parseIngestEnvelopes iotp 的 JSON ingest 既支持单个信封，也支持顶层数组批量。
// 复用 iotp.WriteRow，保证字段与写入接口保持一致（含可选的 timestamp）。
func parseIngestEnvelopes(payload []byte) ([]iotp.WriteRow, error) {
	trimmed := bytes.TrimSpace(payload)
	if len(trimmed) == 0 {
		return nil, fmt.Errorf("空消息")
	}
	if trimmed[0] == '[' {
		var batch []iotp.WriteRow
		if err := json.Unmarshal(trimmed, &batch); err != nil {
			return nil, err
		}
		return batch, nil
	}
	var row iotp.WriteRow
	if err := json.Unmarshal(trimmed, &row); err != nil {
		return nil, err
	}
	return []iotp.WriteRow{row}, nil
}

func (s *Service) handle(nodeID string, msg mqtt.Message) {
	envelopes, err := parseIngestEnvelopes(msg.Payload())
	if err != nil {
		s.lc.Warnf("iotp 节点 %s 消息解析失败: %v", nodeID, err)
		return
	}
	for _, env := range envelopes {
		s.forwardEnvelope(nodeID, env)
	}
}

func (s *Service) forwardEnvelope(nodeID string, env iotp.WriteRow) {
	iotpID := int64(0)
	if v, ok := env.Tags["device_id"]; ok {
		if n, ok := v.(float64); ok {
			iotpID = int64(n)
		}
	}

	if iotpID <= 0 {
		s.lc.Warnf("iotp 节点 %s 消息缺少有效 device_id，已跳过", nodeID)
		return
	}

	// 默认按 iotp 原始数字 ID 转发；匹配到平台设备时换成平台 deviceId。
	// 平台里不存在“野设备”，未命中通常只是设备映射还没刷新。
	deviceID := strconv.FormatInt(iotpID, 10)
	deviceName, productID, productName := "", "", ""
	tenantID := ""
	matched := false

	s.mu.RLock()
	if d, ok := s.devices[iotpID]; ok {
		deviceID = d.Id
		deviceName = d.Name
		productID = d.ProductId
		productName = d.ProductName
		tenantID = d.TenantId
		matched = true
	}
	out := s.out
	s.mu.RUnlock()
	if !matched {
		s.lc.Debugf("iotp 设备 %d 未匹配到平台设备，按原始 ID 转发", iotpID)
	}

	// iotp ingest 信封可能带 timestamp；缺省时按采集器收到并转发的时刻补齐，
	// 让告警 result 能带 start_at/end_at 跳转历史。
	reportTime := env.Timestamp
	if reportTime <= 0 {
		reportTime = time.Now().UnixMilli()
	}
	bus := dtos.MessageBus{
		DeviceId:    deviceID,
		DeviceName:  deviceName,
		ProductId:   productID,
		ProductName: productName,
		TenantId:    tenantID,
		MessageType: defaultMessageType,
		Time:        reportTime,
		Data:        env.Fields,
	}
	payload, _ := json.Marshal(bus)
	if out == nil || !out.IsConnected() {
		s.lc.Debugf("iotp 采集输出未连接，丢弃来自 %s 的消息", nodeID)
		return
	}
	tok := out.Publish(s.outTopic, 1, false, payload)
	// 不无限等待：回调运行在 paho 单线程 dispatch 上，输出 broker 长时间不回 PUBACK
	// 会阻塞该节点后续 ingest 消息的处理和确认。
	if !tok.WaitTimeout(5 * time.Second) {
		s.lc.Warnf("iotp 采集转发超时（节点 %s），输出 broker 未确认", nodeID)
		return
	}
	if tok.Error() != nil {
		s.lc.Warnf("iotp 采集转发失败（节点 %s）: %v", nodeID, tok.Error())
		return
	}
	s.lc.Debugf("iotp 采集转发（节点 %s）: %s", nodeID, payload)
}

func (s *Service) syncDevices() {
	s.refreshDevices()
	ticker := time.NewTicker(time.Duration(s.collectCfg.ResyncIntervalSec) * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.refreshDevices()
		}
	}
}

func (s *Service) refreshDevices() {
	if s.dbClient == nil {
		return
	}
	db := s.dbClient.WithCtx(tenant.Platform(s.ctx))
	devices, _, err := db.DevicesSearch(0, -1, dtos.DeviceSearchQueryRequest{
		BaseSearchConditionQuery: dtos.BaseSearchConditionQuery{IsAll: true},
	})
	if err != nil {
		s.lc.Warnf("iotp 采集刷新设备映射失败: %v", err)
		return
	}
	bindings, gatewayTenants := s.tenantMaps(db)
	next := make(map[int64]deviceInfo, len(devices))
	for _, d := range devices {
		next[iotp.DeviceID(d.Id)] = buildDeviceInfo(d, bindings, gatewayTenants)
	}
	s.mu.Lock()
	s.devices = next
	s.mu.Unlock()
}

func buildDeviceInfo(device models.Device, bindings map[int64]gatewayBinding, gatewayTenants map[string]string) deviceInfo {
	return deviceInfo{
		Id:          device.Id,
		Name:        device.Name,
		ProductId:   device.ProductId,
		ProductName: device.Product.Name,
		TenantId:    resolveDeviceTenant(device, bindings, gatewayTenants),
	}
}

// tenantMaps 读取设备→采集网关绑定和网关→租户映射。
// 辅助查询失败只告警，不影响设备映射本身。
func (s *Service) tenantMaps(db interfaces.DBClient) (map[int64]gatewayBinding, map[string]string) {
	bindings := map[int64]gatewayBinding{}
	if configs, err := db.CollectDeviceConfigs(); err != nil {
		s.lc.Warnf("iotp 采集读取设备网关绑定失败: %v", err)
	} else {
		for _, cfg := range configs {
			if strings.TrimSpace(cfg.DeviceId) == "" {
				continue
			}
			// 统一用 iotp.DeviceID 建立键，自动处理前导零与非数字 ID。
			bindings[iotp.DeviceID(cfg.DeviceId)] = gatewayBinding{
				GatewayId: strings.TrimSpace(cfg.GatewayId),
				TenantId:  strings.TrimSpace(cfg.TenantId),
			}
		}
	}
	gatewayTenants := map[string]string{}
	if gateways, _, err := db.GatewaySearch(0, -1, dtos.GatewaySearchQueryRequest{
		BaseSearchConditionQuery: dtos.BaseSearchConditionQuery{IsAll: true},
	}); err != nil {
		s.lc.Warnf("iotp 采集读取网关租户失败: %v", err)
	} else {
		for _, gw := range gateways {
			if gw.Id != "" && strings.TrimSpace(gw.TenantId) != "" {
				gatewayTenants[gw.Id] = strings.TrimSpace(gw.TenantId)
			}
		}
	}
	return bindings, gatewayTenants
}

// resolveDeviceTenant 设备自身租户优先；设备没租户时用采集网关的租户，
// 最后才用绑定记录上的租户。都没有则返回空（fail-closed，不猜测租户）。
func resolveDeviceTenant(device models.Device, bindings map[int64]gatewayBinding, gatewayTenants map[string]string) string {
	if t := strings.TrimSpace(device.TenantId); t != "" {
		return t
	}
	binding := bindings[iotp.DeviceID(device.Id)]
	if gatewayID := binding.GatewayId; gatewayID != "" {
		if t := gatewayTenants[gatewayID]; t != "" {
			return t
		}
	}
	return binding.TenantId
}
