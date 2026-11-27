// Package iotpcollectorapp 订阅 iotp 集群各节点的 ingest/#，
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
	bootstrapConfig "iotserver/internal/pkg/config"
	pkgContainer "iotserver/internal/pkg/container"
	"iotserver/internal/pkg/di"
	"iotserver/internal/pkg/logger"
	"iotserver/internal/pkg/tenant"
	"iotserver/internal/tools/collectengine"
	"iotserver/internal/tools/iotp"
	pkgMQTT "iotserver/internal/tools/mqttclient"
)

const (
	defaultSubscribeTopic   = "ingest/#"
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

// deviceInfo iotp 设备 ID 对应的平台设备及其生效租户。
// 租户优先取设备自身，设备没归属时取采集它的软网关。
type deviceInfo struct {
	Device      models.Device
	TenantId    string
	ProductName string
}

// gatewayBinding 平台里的设备 → 采集网关绑定。
type gatewayBinding struct {
	GatewayId string
	TenantId  string
}

// Service iotp 多节点采集服务。
type Service struct {
	dic      *di.Container
	lc       logger.LoggingClient
	dbClient interfaces.DBClient

	collectCfg config.IotpCollectInfo
	mqCfg      config.MessageQueueInfo
	// enabled 由 [Databases.Data].BaseURL 决定：接入 iotp 即启用。
	enabled bool

	ctx    context.Context
	cancel context.CancelFunc

	mu       sync.RWMutex
	closed   bool
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
		dic:        dic,
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
	s.enabled = iotpCollectEnabled(cfg.GetDataDatabaseInfo())
	s.outTopic = strings.TrimSpace(s.mqCfg.PublishTopicPrefix)
	if s.outTopic == "" {
		s.outTopic = "eventbus/in"
	}
	s.subscribeTopic = ingestTopicForDB(cfg.GetDataDatabaseInfo().Name)
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
//   - 受管软网关的采集引擎：启动时发现一次，之后由网关 CRUD 事件触发刷新。
func (s *Service) Start(ctx context.Context) {
	if !s.enabled {
		s.lc.Infof("iotp 多节点采集未启用：[Databases.Data].BaseURL 为空")
		return
	}
	static := s.staticNodeSpecs()

	s.ctx, s.cancel = context.WithCancel(ctx)
	go s.syncDevices()
	go s.connectOutput()
	for _, spec := range static {
		s.ensureNode(spec.Id, spec.Broker)
	}
	go s.discoverLoop()
	s.lc.Infof("iotp 多节点采集已启动：静态节点=%d 自动发现=受管网关 topic=%s -> %s",
		len(static), s.subscribeTopic, s.outTopic)
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

// Stop 断开所有连接。
func (s *Service) Stop() {
	if s.cancel != nil {
		s.cancel()
	}
	s.mu.Lock()
	s.closed = true
	out := s.out
	nodes := make([]*nodeConn, 0, len(s.nodes))
	for key, conn := range s.nodes {
		nodes = append(nodes, conn)
		delete(s.nodes, key)
	}
	s.mu.Unlock()

	if out != nil {
		out.Disconnect(250)
	}
	for _, conn := range nodes {
		conn.client.Disconnect(250)
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
	if s.closed {
		s.mu.Unlock()
		client.Disconnect(250)
		return
	}
	s.out = client
	s.mu.Unlock()
}

// staticNodeSpecs 解析 Nodes 配置，按 broker 去重。
func (s *Service) staticNodeSpecs() []nodeSpec {
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
	if strings.TrimSpace(id) == "" {
		id = "node-" + shortHash(key)
	}

	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
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
	s.mu.RLock()
	closed := s.closed
	s.mu.RUnlock()
	if closed {
		return
	}
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

	interval := time.Duration(s.collectCfg.ResyncIntervalSec) * time.Second
	if interval <= 0 {
		interval = defaultResyncSeconds * time.Second
	}
	ticker := time.NewTicker(interval)
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

	s.reconcileNodes(mergeNodeSpecs(s.staticNodeSpecs(), known))
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
		label := info.Label
		if label == "" {
			label = "gw-" + id
		}
		desired[key] = nodeSpec{Id: label, Broker: key}
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

// shortHash 给空标识生成稳定的短后缀。
func shortHash(value string) string {
	var hash uint32 = 2166136261
	for i := 0; i < len(value); i++ {
		hash ^= uint32(value[i])
		hash *= 16777619
	}
	return fmt.Sprintf("%08x", hash)
}

type ingestEnvelope struct {
	Measurement string         `json:"measurement"`
	Tags        map[string]any `json:"tags"`
	Fields      map[string]any `json:"fields"`
}

// parseIngestEnvelopes iotp 的 JSON ingest 既支持单个信封，也支持顶层数组批量。
func parseIngestEnvelopes(payload []byte) ([]ingestEnvelope, error) {
	trimmed := bytes.TrimSpace(payload)
	if len(trimmed) == 0 {
		return nil, fmt.Errorf("空消息")
	}
	if trimmed[0] == '[' {
		var batch []ingestEnvelope
		if err := json.Unmarshal(trimmed, &batch); err != nil {
			return nil, err
		}
		return batch, nil
	}
	var env ingestEnvelope
	if err := json.Unmarshal(trimmed, &env); err != nil {
		return nil, err
	}
	return []ingestEnvelope{env}, nil
}

func (s *Service) handle(nodeID string, msg mqtt.Message) {
	parts := strings.Split(msg.Topic(), "/")
	if len(parts) != 3 || parts[0] != "ingest" || parts[2] != "json" {
		return // 只处理 ingest/<db>/json；LP 不解析
	}
	envelopes, err := parseIngestEnvelopes(msg.Payload())
	if err != nil {
		s.lc.Warnf("iotp 节点 %s 消息解析失败: %v", nodeID, err)
		return
	}
	for _, env := range envelopes {
		s.forwardEnvelope(nodeID, env)
	}
}

func (s *Service) forwardEnvelope(nodeID string, env ingestEnvelope) {
	iotpID := int64(0)
	if v, ok := env.Tags["device_id"]; ok {
		switch n := v.(type) {
		case float64:
			iotpID = int64(n)
		case json.Number:
			iotpID, _ = n.Int64()
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
		deviceID = d.Device.Id
		deviceName = d.Device.Name
		productID = d.Device.ProductId
		productName = d.ProductName
		tenantID = d.TenantId
		matched = true
	}
	out := s.out
	s.mu.RUnlock()
	if !matched {
		s.lc.Debugf("iotp 设备 %d 未匹配到平台设备，按原始 ID 转发", iotpID)
	}

	bus := dtos.MessageBus{
		DeviceId:    deviceID,
		DeviceName:  deviceName,
		ProductId:   productID,
		ProductName: productName,
		TenantId:    tenantID,
		MessageType: defaultMessageType,
		// iotp ingest 信封没有逐点时间，按采集器收到并转发的时刻补齐，
		// 让告警 result 能带 start_at/end_at 跳转历史。
		ReportTime: time.Now().UnixMilli(),
		Data:       env.Fields,
	}
	payload, _ := json.Marshal(bus)
	if out == nil || !out.IsConnected() {
		s.lc.Debugf("iotp 采集输出未连接，丢弃来自 %s 的消息", nodeID)
		return
	}
	tok := out.Publish(s.outTopic, 1, false, payload)
	// 不无限等待：回调运行在 paho 单线程 dispatch 上，输出 broker 长时间不回 PUBACK
	// 会阻塞该节点后续 ingest 消息的处理和确认。
	select {
	case <-tok.Done():
		if tok.Error() != nil {
			s.lc.Warnf("iotp 采集转发失败（节点 %s）: %v", nodeID, tok.Error())
			return
		}
	case <-time.After(5 * time.Second):
		s.lc.Warnf("iotp 采集转发超时（节点 %s），输出 broker 未确认", nodeID)
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
	productNames := s.productNames(db)
	next := make(map[int64]deviceInfo, len(devices))
	for _, d := range devices {
		next[iotp.DeviceID(d.Id)] = buildDeviceInfo(d, productNames, bindings, gatewayTenants)
	}
	s.mu.Lock()
	s.devices = next
	s.mu.Unlock()
}

// productNames 读取产品 ID → 产品名称，供消息里附带 productName。
func (s *Service) productNames(db interfaces.DBClient) map[string]string {
	products, _, err := db.ProductsSearch(0, -1, false, dtos.ProductSearchQueryRequest{
		BaseSearchConditionQuery: dtos.BaseSearchConditionQuery{IsAll: true},
	})
	if err != nil {
		s.lc.Warnf("iotp 采集读取产品名称失败: %v", err)
		return map[string]string{}
	}
	names := make(map[string]string, len(products))
	for _, product := range products {
		if product.Id != "" {
			names[product.Id] = product.Name
		}
	}
	return names
}

func buildDeviceInfo(device models.Device, productNames map[string]string, bindings map[string]gatewayBinding, gatewayTenants map[string]string) deviceInfo {
	return deviceInfo{
		Device:      device,
		TenantId:    resolveDeviceTenant(device, bindings, gatewayTenants),
		ProductName: productNames[device.ProductId],
	}
}

// tenantMaps 读取设备→采集网关绑定和网关→租户映射。
// 辅助查询失败只告警，不影响设备映射本身。
func (s *Service) tenantMaps(db interfaces.DBClient) (map[string]gatewayBinding, map[string]string) {
	bindings := map[string]gatewayBinding{}
	if configs, err := db.CollectDeviceConfigs(); err != nil {
		s.lc.Warnf("iotp 采集读取设备网关绑定失败: %v", err)
	} else {
		for _, cfg := range configs {
			if strings.TrimSpace(cfg.DeviceId) == "" {
				continue
			}
			// 采集配置里的 device_id 是 iotp 数字键（前导零已去掉），
			// 平台设备 ID 可能保留前导零；两边统一成去前导零的数字字符串。
			key := normalizeDeviceTenantKey(cfg.DeviceId)
			bindings[key] = gatewayBinding{
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
// normalizeDeviceTenantKey 统一数字 ID 的前导零；非数字 ID 原样保留。
func normalizeDeviceTenantKey(id string) string {
	trimmed := strings.TrimSpace(id)
	if n, err := strconv.ParseInt(trimmed, 10, 64); err == nil && n > 0 {
		return strconv.FormatInt(n, 10)
	}
	return trimmed
}

func resolveDeviceTenant(device models.Device, bindings map[string]gatewayBinding, gatewayTenants map[string]string) string {
	if t := strings.TrimSpace(device.TenantId); t != "" {
		return t
	}
	binding := bindings[normalizeDeviceTenantKey(device.Id)]
	if gatewayID := binding.GatewayId; gatewayID != "" {
		if t := gatewayTenants[gatewayID]; t != "" {
			return t
		}
	}
	return binding.TenantId
}

// iotpCollectEnabled 只要配置了 [Databases.Data].BaseURL 就认为接入了 iotp。
func iotpCollectEnabled(db bootstrapConfig.Database) bool {
	return strings.TrimSpace(db.BaseURL) != ""
}

// ingestTopicForDB 订阅平台自己的 iotp 库，避免把同实例其他库的数据也转进总线。
// 库名非法或缺失时回落到 ingest/#。
func ingestTopicForDB(dbName string) string {
	dbName = strings.TrimSpace(dbName)
	if dbName == "" || strings.ContainsAny(dbName, "/+#") {
		return defaultSubscribeTopic
	}
	return "ingest/" + dbName + "/json"
}
