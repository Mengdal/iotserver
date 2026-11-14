// Package iotpsync 把平台的「产品/物模型/设备」同步到 iotp 的元数据。
//
// 只做元数据：产品 → 库/表，物模型属性 → 字段，设备 → 设备实例。
// 时序数据写入不在本包；同步全部异步执行，iotp 不可用不会阻塞平台的产品/设备创建。
package iotpsync

import (
	"context"
	"strconv"
	"strings"
	"sync"
	"time"

	"iotserver/internal/dtos"
	"iotserver/internal/iotserver/core/container"
	interfaces "iotserver/internal/iotserver/core/interface"
	"iotserver/internal/models"
	"iotserver/internal/pkg/constants"
	pkgContainer "iotserver/internal/pkg/container"
	"iotserver/internal/pkg/di"
	"iotserver/internal/pkg/logger"
	"iotserver/internal/pkg/tenant"
	"iotserver/internal/tools/iotp"
)

const (
	defaultTimeout          = 30 * time.Second
	gatewayReconcileTimeout = 2 * time.Minute
	gatewayReconcilePeriod  = 10 * time.Minute
	gatewayReconcileDelay   = 10 * time.Second
	gatewayAssignBatch      = 500
)

// Service iotp 元数据同步服务。
type Service struct {
	dic      *di.Container
	lc       logger.LoggingClient
	dbClient interfaces.DBClient
	enabled  bool
	// dataType 时序后端类型；只有 Type=iotp 时才同步网关归属。
	dataType string
	client   *iotp.Client

	// gatewayMu 防止多次对账重叠。网关声明/盖章本身幂等，允许与事件触发并发。
	gatewayMu sync.Mutex
	// warnForbiddenOnce 避免缺 admin 权限时每个设备绑定都刷日志。
	warnForbiddenOnce sync.Once
}

func New(dic *di.Container) *Service {
	cfg := container.ConfigurationFrom(dic.Get)
	lc := pkgContainer.LoggingClientFrom(dic.Get)
	service := &Service{
		dic:      dic,
		lc:       lc,
		dbClient: container.DBClientFrom(dic.Get),
	}
	if cfg == nil {
		lc.Infof("iotp 元数据同步未启用")
		return service
	}
	db, ok := cfg.IotpConfig()
	if !ok {
		lc.Infof("iotp 元数据同步未启用")
		return service
	}
	service.enabled = true
	service.dataType = strings.ToLower(strings.TrimSpace(cfg.GetDataDatabaseInfo().Type))
	service.client = iotp.New(db.BaseURL, db.Token, db.IotpDatabaseName(), db.IotpTimeout())
	lc.Infof("iotp 元数据同步已启用: %s db=%s 数据层=%s", db.BaseURL, db.IotpDatabaseName(), service.dataType)
	if service.dataType == string(constants.Iotp) {
		lc.Infof("iotp 网关归属同步已启用（启动后对账，周期 %s）", gatewayReconcilePeriod)
	} else {
		lc.Infof("iotp 网关归属同步未启用（仅数据层为 iotp 时同步软网关与设备归属）")
	}
	return service
}

func (s *Service) Enabled() bool {
	return s != nil && s.enabled && s.client != nil
}

// ProductCreated 产品创建/发布后同步整张表（基础标签 + 物模型属性 + 事件/服务字段）。
func (s *Service) ProductCreated(product models.Product, tenantID string) {
	if !s.Enabled() {
		return
	}
	go s.run("product:"+product.Id, func(ctx context.Context) error {
		return s.syncProduct(ctx, product, tenantID)
	})
}

// ProductUpdated 产品资料更新后刷新表字段元数据（product_id 字段的 product_name）。
func (s *Service) ProductUpdated(product models.Product, tenantID string) {
	if !s.Enabled() {
		return
	}
	go s.run("product-update:"+product.Id, func(ctx context.Context) error {
		return s.syncProduct(ctx, product, tenantID)
	})
}

// PropertyChanged 物模型属性新增/修改后同步字段。
func (s *Service) PropertyChanged(productID string, property models.Properties, tenantID string) {
	if !s.Enabled() || strings.TrimSpace(productID) == "" {
		return
	}
	go s.run("property:"+productID+":"+property.Code, func(ctx context.Context) error {
		return s.syncProperty(ctx, productID, property)
	})
}

// EventChanged 物模型事件新增/修改后同步字段。
func (s *Service) EventChanged(productID string, event models.Events, tenantID string) {
	s.changeField(productID, "evt_"+iotp.FieldName(event.Code))
}

// ActionChanged 物模型服务新增/修改后同步字段。
func (s *Service) ActionChanged(productID string, action models.Actions, tenantID string) {
	s.changeField(productID, "act_"+iotp.FieldName(action.Code))
}

func (s *Service) changeField(productID, fieldName string) {
	if !s.Enabled() || strings.TrimSpace(productID) == "" || strings.TrimSpace(fieldName) == "" {
		return
	}
	go s.run("field:"+productID+":"+fieldName, func(ctx context.Context) error {
		table := iotp.TableName(productID)
		if err := s.client.EnsureDatabase(ctx); err != nil {
			return err
		}
		if err := s.client.EnsureTable(ctx, table, baseFieldsWithProductName(iotp.BaseFields(), s.productNameByID(ctx, productID))); err != nil {
			return err
		}
		return s.client.AddField(ctx, table, iotp.Field{Name: fieldName, Type: iotp.TypeString})
	})
}

// DeviceCreated 设备创建后注册 iotp 设备。
func (s *Service) DeviceCreated(device models.Device, product models.Product, tenantID string) {
	if !s.Enabled() {
		return
	}
	go s.run("device:"+device.Id, func(ctx context.Context) error {
		return s.syncDevice(ctx, device, product, tenantID)
	})
}

// DeviceDeleted 设备删除后删除 iotp 设备（历史数据保留）。
func (s *Service) DeviceDeleted(device models.Device) {
	if !s.Enabled() {
		return
	}
	go s.run("device-delete:"+device.Id, func(ctx context.Context) error {
		return s.client.DeleteDevice(ctx, iotp.DeviceID(device.Id))
	})
}

/* ------------------------------ 网关归属同步 ------------------------------ */

// GatewaySyncEnabled 只有时序后端真正是 iotp 时才同步网关归属。
//
// DuckDB 后端没有 iotp 网关概念；即使配置了 iotp BaseURL（用于实时值或元数据），
// 也不应该把平台网关推送到 iotp。
func (s *Service) GatewaySyncEnabled() bool {
	return s.Enabled() && s.dataType == string(constants.Iotp)
}

// GatewayCreated 软网关创建后在 iotp 声明网关。
func (s *Service) GatewayCreated(gateway models.Gateway) {
	if !s.GatewaySyncEnabled() || !isSoftGateway(gateway) {
		return
	}
	go s.run("gateway:"+gatewayKey(gateway), func(ctx context.Context) error {
		return s.ensureGateway(ctx, gateway)
	})
}

// GatewayDeleted 软网关删除：先清设备归属，再删除 iotp 网关声明。
//
// 清章失败只告警不阻断删除：iotp 删除网关后悬挂章语义上按未归属处理，
// 历史时序数据不受影响。
func (s *Service) GatewayDeleted(gateway models.Gateway, deviceIDs []int64) {
	if !s.GatewaySyncEnabled() || !isSoftGateway(gateway) {
		return
	}
	go s.run("gateway-delete:"+gatewayKey(gateway), func(ctx context.Context) error {
		if len(deviceIDs) > 0 {
			if err := s.assignDeviceGateway(ctx, deviceIDs, ""); err != nil {
				s.lc.Warnf("iotp 清除网关 %s 的设备归属失败（继续删除网关声明）: %v", gatewayKey(gateway), err)
			}
		}
		return s.client.DeleteGateway(ctx, gatewayKey(gateway))
	})
}

// DeviceGatewayBound 设备绑定软网关：确保设备实例存在，并设置 iotp 归属。
func (s *Service) DeviceGatewayBound(device models.Device, gatewaySn string) {
	if !s.GatewaySyncEnabled() || strings.TrimSpace(device.Id) == "" || strings.TrimSpace(gatewaySn) == "" {
		return
	}
	go s.run("gateway-bind:"+device.Id, func(ctx context.Context) error {
		if err := s.ensureGateway(ctx, models.Gateway{Sn: gatewaySn}); err != nil {
			return err
		}
		if err := s.ensureDevice(ctx, device); err != nil {
			return err
		}
		return s.assignDeviceGateway(ctx, []int64{iotp.DeviceID(device.Id)}, gatewayKey(models.Gateway{Sn: gatewaySn}))
	})
}

// DeviceGatewayUnbound 设备解绑软网关：清除 iotp 归属。
func (s *Service) DeviceGatewayUnbound(iotpDeviceID int64) {
	if !s.GatewaySyncEnabled() || iotpDeviceID <= 0 {
		return
	}
	go s.run("gateway-unbind", func(ctx context.Context) error {
		err := s.assignDeviceGateway(ctx, []int64{iotpDeviceID}, "")
		// 设备已被删除时清章返回 404，属于正常情况。
		if iotp.IsNotFound(err) {
			return nil
		}
		return err
	})
}

// ReconcileGateways 触发一次对账（异步）。并发调用时后到的请求直接跳过。
func (s *Service) ReconcileGateways() {
	if !s.GatewaySyncEnabled() {
		return
	}
	if !s.gatewayMu.TryLock() {
		return
	}
	go func() {
		defer s.gatewayMu.Unlock()
		s.runWithTimeout("gateway-reconcile", gatewayReconcileTimeout, s.reconcileGateways)
	}()
}

// Start 启动网关对账：启动后延迟执行一次，之后按固定周期执行。
func (s *Service) Start(ctx context.Context) {
	if s.Enabled() {
		go s.startProductBackfill(ctx)
	}
	if s.GatewaySyncEnabled() {
		go s.startGatewayReconcile(ctx)
	}
}

// startProductBackfill 启动后补齐所有产品的表字段。
//
// 表名规则变更（例如去掉 p_ 前缀）后，已有产品的表可能只有基础标签字段；
// 这里按物模型重新 EnsureTable，把缺失的测量字段补回去。
func (s *Service) startProductBackfill(ctx context.Context) {
	select {
	case <-ctx.Done():
		return
	case <-time.After(gatewayReconcileDelay):
	}
	s.runWithTimeout("product-backfill", gatewayReconcileTimeout, func(runCtx context.Context) error {
		if s.dbClient == nil {
			return nil
		}
		products, _, err := s.dbClient.WithCtx(tenant.Platform(runCtx)).ProductsSearch(
			0, -1, true, dtos.ProductSearchQueryRequest{
				BaseSearchConditionQuery: dtos.BaseSearchConditionQuery{IsAll: true},
			})
		if err != nil {
			return err
		}
		for _, product := range products {
			if err := s.syncProduct(runCtx, product, product.TenantId); err != nil {
				s.lc.Errorf("iotp 产品表字段补齐失败 product=%s: %v", product.Id, err)
			}
		}
		return nil
	})
}

func (s *Service) startGatewayReconcile(ctx context.Context) {
	select {
	case <-ctx.Done():
		return
	case <-time.After(gatewayReconcileDelay):
	}
	s.ReconcileGateways()

	ticker := time.NewTicker(gatewayReconcilePeriod)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.ReconcileGateways()
		}
	}
}

// ensureGateway 声明网关。对象只带 SN 时 note 为空，用于设备绑定时的兜底声明。
func (s *Service) ensureGateway(ctx context.Context, gateway models.Gateway) error {
	err := s.client.EnsureGateway(ctx, gatewayKey(gateway), strings.TrimSpace(gateway.Name))
	if iotp.IsForbidden(err) {
		s.warnForbiddenOnce.Do(func() {
			s.lc.Warnf("iotp 令牌缺少 admin 权限，无法同步网关归属；请为令牌增加 admin:* 规则")
		})
	}
	return err
}

// assignDeviceGateway 批量盖章；失败时逐台重试，避免一台设备不存在拖垮整批。
func (s *Service) assignDeviceGateway(ctx context.Context, deviceIDs []int64, gateway string) error {
	for start := 0; start < len(deviceIDs); start += gatewayAssignBatch {
		end := start + gatewayAssignBatch
		if end > len(deviceIDs) {
			end = len(deviceIDs)
		}
		batch := deviceIDs[start:end]
		if err := s.client.AssignDeviceGateway(ctx, batch, gateway); err != nil {
			if iotp.IsNotFound(err) && len(batch) > 1 {
				for _, id := range batch {
					if oneErr := s.client.AssignDeviceGateway(ctx, []int64{id}, gateway); oneErr != nil && !iotp.IsNotFound(oneErr) {
						s.lc.Warnf("iotp 设置设备 %d 归属失败: %v", id, oneErr)
					}
				}
				continue
			}
			return err
		}
	}
	return nil
}

// reconcileGateways 以平台元数据为准，补齐 iotp 侧缺失的网关声明和设备归属。
//
// 只做补齐，不删除平台不认识的网关，也不清理由其它系统管理的设备章。
func (s *Service) reconcileGateways(ctx context.Context) error {
	if s.dbClient == nil {
		return nil
	}
	db := s.dbClient.WithCtx(tenant.Platform(ctx))

	gatewayList, _, err := db.GatewaySearch(0, -1, dtos.GatewaySearchQueryRequest{
		BaseSearchConditionQuery: dtos.BaseSearchConditionQuery{IsAll: true},
		Source:                   models.GatewaySourceIoserver,
	})
	if err != nil {
		return err
	}
	if len(gatewayList) == 0 {
		return nil
	}

	configs, err := db.CollectDeviceConfigs()
	if err != nil {
		return err
	}
	devices, _, err := db.DevicesSearch(0, -1, dtos.DeviceSearchQueryRequest{
		BaseSearchConditionQuery: dtos.BaseSearchConditionQuery{IsAll: true},
	})
	if err != nil {
		return err
	}
	deviceByIotpID := make(map[int64]models.Device, len(devices))
	for _, device := range devices {
		deviceByIotpID[iotp.DeviceID(device.Id)] = device
	}
	gatewayByID := make(map[string]models.Gateway, len(gatewayList))
	for _, gateway := range gatewayList {
		gatewayByID[gateway.Id] = gateway
	}

	declared, err := s.client.Gateways(ctx)
	if err != nil {
		if iotp.IsForbidden(err) {
			s.warnForbiddenOnce.Do(func() {
				s.lc.Warnf("iotp 令牌缺少 admin 权限，无法同步网关归属；请为令牌增加 admin:* 规则")
			})
		}
		return err
	}
	declaredSet := make(map[string]struct{}, len(declared))
	for _, item := range declared {
		declaredSet[item.Name] = struct{}{}
	}

	declaredCount := 0
	for _, gateway := range gatewayList {
		key := gatewayKey(gateway)
		if _, ok := declaredSet[key]; ok {
			continue
		}
		if err := s.ensureGateway(ctx, gateway); err != nil {
			return err
		}
		declaredCount++
	}

	// 按表聚合需要归属的本地设备；同一设备只属于一个软网关。
	type boundDevice struct {
		device  models.Device
		iotpID  int64
		gateway string
	}
	boundByTable := make(map[string][]boundDevice)
	skipped := 0
	for _, cfg := range configs {
		gateway, ok := gatewayByID[cfg.GatewayId]
		if !ok {
			continue
		}
		device, ok := deviceByIotpID[parseDeviceID(cfg.DeviceId)]
		if !ok {
			// 设备已删除但采集配置残留：不能凭配置复活设备。
			skipped++
			continue
		}
		table := iotp.DeviceTable(device.ProductId)
		boundByTable[table] = append(boundByTable[table], boundDevice{
			device:  device,
			iotpID:  iotp.DeviceID(device.Id),
			gateway: gatewayKey(gateway),
		})
	}

	// 每张表只拉一次设备归属，在内存里和本地绑定做 diff：
	// 稳定状态下不产生任何 ensure/register 请求。
	registered, stamped := 0, 0
	for table, items := range boundByTable {
		remote, listErr := s.client.TableDevices(ctx, table)
		if listErr != nil {
			// 表可能还不存在：退化为逐台 ensure，由 EnsureTable 补建后再注册。
			s.lc.Warnf("iotp 对账读取表 %s 失败，逐台补齐: %v", table, listErr)
			for _, item := range items {
				if err := s.ensureDevice(ctx, item.device); err != nil {
					s.lc.Warnf("iotp 对账注册设备 %s 失败: %v", item.device.Id, err)
					skipped++
					continue
				}
				registered++
				if err := s.assignDeviceGateway(ctx, []int64{item.iotpID}, item.gateway); err != nil {
					return err
				}
				stamped++
			}
			continue
		}
		remoteGateway := make(map[int64]string, len(remote))
		for _, record := range remote {
			gateway := ""
			if record.Gateway != nil {
				gateway = strings.TrimSpace(*record.Gateway)
			}
			remoteGateway[record.ID] = gateway
		}

		idsByGateway := make(map[string][]int64)
		for _, item := range items {
			gateway, exists := remoteGateway[item.iotpID]
			if !exists {
				if err := s.ensureDevice(ctx, item.device); err != nil {
					s.lc.Warnf("iotp 对账注册设备 %s 失败: %v", item.device.Id, err)
					skipped++
					continue
				}
				registered++
			} else if gateway == item.gateway {
				// 设备已存在且归属正确，跳过。
				continue
			}
			idsByGateway[item.gateway] = append(idsByGateway[item.gateway], item.iotpID)
		}
		for gateway, ids := range idsByGateway {
			if err := s.assignDeviceGateway(ctx, ids, gateway); err != nil {
				return err
			}
			stamped += len(ids)
		}
	}
	if declaredCount > 0 || registered > 0 || stamped > 0 || skipped > 0 {
		s.lc.Infof("iotp 网关对账完成: 新增声明 %d，注册设备 %d，盖章 %d，跳过 %d", declaredCount, registered, stamped, skipped)
	}
	return nil
}

func isSoftGateway(gateway models.Gateway) bool {
	source := strings.TrimSpace(gateway.Source)
	return source == models.GatewaySourceIoserver
}

// gatewayKey iotp 网关键：优先用 SN（软网关自动生成的 10 位数字，全局唯一且创建后不可改），
// SN 为空时退回 ID。这里不做字符替换，声明和盖章必须使用同一个 key。
func gatewayKey(gateway models.Gateway) string {
	key := strings.TrimSpace(gateway.Sn)
	if key == "" {
		key = "gw-" + strings.TrimSpace(gateway.Id)
	}
	return key
}

func parseDeviceID(value string) int64 {
	id, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil || id <= 0 || id >= iotp.DerivedDeviceIDStart {
		return 0
	}
	return id
}

func (s *Service) run(what string, fn func(context.Context) error) {
	s.runWithTimeout(what, defaultTimeout, fn)
}

func (s *Service) runWithTimeout(what string, timeout time.Duration, fn func(context.Context) error) {
	defer func() {
		if r := recover(); r != nil {
			s.lc.Errorf("iotp 同步 %s panic: %v", what, r)
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	if err := fn(ctx); err != nil {
		s.lc.Errorf("iotp 同步 %s 失败: %v", what, err)
		return
	}
	s.lc.Infof("iotp 同步 %s 完成", what)
}

func (s *Service) syncProduct(ctx context.Context, product models.Product, tenantID string) error {
	table := iotp.TableName(product.Id)
	fields := baseFieldsWithProductName(iotp.BaseFields(), s.productName(ctx, product))
	for _, property := range product.Properties {
		fields = append(fields, propertyField(property))
	}
	for _, event := range product.Events {
		fields = append(fields, iotp.Field{Name: "evt_" + iotp.FieldName(event.Code), Type: iotp.TypeString})
	}
	for _, action := range product.Actions {
		fields = append(fields, iotp.Field{Name: "act_" + iotp.FieldName(action.Code), Type: iotp.TypeString})
	}
	if err := s.client.EnsureDatabase(ctx); err != nil {
		return err
	}
	return s.client.EnsureTable(ctx, table, iotp.DedupeFields(fields))
}

func (s *Service) syncProperty(ctx context.Context, productID string, property models.Properties) error {
	table := iotp.TableName(productID)
	if err := s.client.EnsureDatabase(ctx); err != nil {
		return err
	}
	if err := s.client.EnsureTable(ctx, table, baseFieldsWithProductName(iotp.BaseFields(), s.productNameByID(ctx, productID))); err != nil {
		return err
	}
	return s.client.AddField(ctx, table, propertyField(property))
}

func (s *Service) syncDevice(ctx context.Context, device models.Device, product models.Product, tenantID string) error {
	return s.ensureDeviceWithProduct(ctx, device, product, tenantID)
}

// ensureDevice 按设备元数据补登 iotp 设备实例。
//
// 无产品设备（网关子设备）注册到 default；有产品设备注册到 <productId>。
// 用于设备绑定软网关、对账等只有 models.Device 的路径。
func (s *Service) ensureDevice(ctx context.Context, device models.Device) error {
	var product models.Product
	if strings.TrimSpace(device.ProductId) != "" {
		if s.dbClient == nil {
			return nil
		}
		p, err := s.dbClient.WithCtx(tenant.Platform(ctx)).ProductById(device.ProductId)
		if err != nil {
			return err
		}
		product = p
	}
	return s.ensureDeviceWithProduct(ctx, device, product, device.TenantId)
}

func (s *Service) ensureDeviceWithProduct(ctx context.Context, device models.Device, product models.Product, tenantID string) error {
	table := iotp.DefaultTable
	if strings.TrimSpace(product.Id) != "" {
		table = iotp.TableName(product.Id)
	}
	if err := s.client.EnsureDatabase(ctx); err != nil {
		return err
	}
	if err := s.client.EnsureTable(ctx, table, baseFieldsWithProductName(iotp.BaseFields(), s.productName(ctx, product))); err != nil {
		return err
	}

	deviceID := iotp.DeviceID(device.Id)
	// 设备可能已在 default，后来绑定了产品（或反过来）。iotp 的设备实例不能跨表，
	// 目标表变化时先删旧实例再注册；历史时序数据不受影响。
	if existing, err := s.client.Device(ctx, deviceID); err == nil {
		existingTable := existing.Table
		if existingTable != table && existingTable != s.client.Database()+"."+table {
			if err := s.client.DeleteDevice(ctx, deviceID); err != nil {
				return err
			}
		}
	} else if !iotp.IsNotFound(err) {
		return err
	}

	deviceKey := strings.TrimSpace(device.DeviceKey)
	if deviceKey == "" {
		deviceKey = strings.TrimSpace(device.DeviceSn)
	}
	if deviceKey == "" {
		deviceKey = device.Id
	}
	if tenantID == "" {
		tenantID = device.TenantId
	}
	tags := map[string]string{
		"tenant_id":  tenantID,
		"product_id": device.ProductId,
		"device_sn":  device.DeviceSn,
		"device_key": deviceKey,
		"dept_id":    device.DeptId,
	}
	return s.client.RegisterDevice(ctx, table, iotp.Device{
		DeviceID: deviceID,
		Name:     device.Name,
		Tags:     tags,
	})
}

// productNameAttrKey 写在 product_id 字段 attrs 上的产品名称键。
const productNameAttrKey = "product_name"

// baseFieldsWithProductName 给 product_id 标签字段补充产品名称元数据。
// 产品名称放在字段 attrs 上，每张产品表一份，不往每台设备实例的 attrs 里复制。
func baseFieldsWithProductName(fields []iotp.Field, productName string) []iotp.Field {
	name := strings.TrimSpace(productName)
	for i := range fields {
		if fields[i].Name != "product_id" {
			continue
		}
		if name == "" {
			fields[i].Attrs = nil
		} else {
			fields[i].Attrs = map[string]any{productNameAttrKey: name}
		}
	}
	return fields
}

// productName 取产品名称；物模型增量回调只带 Id 时回查平台元数据。
func (s *Service) productName(ctx context.Context, product models.Product) string {
	if name := strings.TrimSpace(product.Name); name != "" {
		return name
	}
	return s.productNameByID(ctx, product.Id)
}

func (s *Service) productNameByID(ctx context.Context, productID string) string {
	if s.dbClient == nil || strings.TrimSpace(productID) == "" {
		return ""
	}
	product, err := s.dbClient.WithCtx(tenant.Platform(ctx)).ProductById(productID)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(product.Name)
}

func propertyField(property models.Properties) iotp.Field {
	return iotp.Field{
		Name: iotp.FieldName(property.Code),
		Type: iotp.FieldType(property.TypeSpec.Type),
	}
}
