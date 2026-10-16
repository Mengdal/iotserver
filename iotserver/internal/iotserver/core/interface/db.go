//
// Copyright (C) 2020-2021 IOTech Ltd
//
// SPDX-License-Identifier: Apache-2.0

package interfaces

import (
	"context"

	"gorm.io/gorm"
	"iotserver/internal/dtos"
	"iotserver/internal/models"
)

type DBClient interface {
	CloseSession()
	GetDBInstance() *gorm.DB

	// WithCtx 返回一个把 ctx 绑定到数据库会话的客户端副本。
	// 租户隔离依赖它：GORM 租户插件从 Statement.Context 读取租户上下文
	// （internal/pkg/tenant）。业务方法应使用 app.dbClient.WithCtx(ctx) 得到的实例，
	// 后台任务与系统路径使用 tenant.Platform(ctx) 构造的平台作用域上下文。
	WithCtx(ctx context.Context) DBClient

	// ===== 租户=====
	AddTenant(t models.Tenant) (models.Tenant, error)
	TenantById(id string) (models.Tenant, error)
	TenantByCode(code string) (models.Tenant, error)
	TenantSearch(offset int, limit int, req dtos.TenantSearchQueryRequest) ([]models.Tenant, uint32, error)
	UpdateTenant(t models.Tenant) error
	DeleteTenant(id string) error

	// ===== 租户资源统计（配额校验 / 删租户保护）=====
	//
	// 计数一律显式带 tenant_id 条件（平台作用域下 GORM 租户插件不过滤），
	// 调用方需用 WithCtx 把作用域切到目标租户，避免插件叠加出互斥条件。
	TenantUserCount(tenantId string) (int64, error)
	TenantDeviceCount(tenantId string) (int64, error)
	TenantGatewayCount(tenantId string) (int64, error)
	TenantProductCount(tenantId string) (int64, error)
	TenantVizBoardCount(tenantId string) (int64, error)
	// CleanupTenantMenuRecords 清理某租户的菜单定制数据（tenant_menu / tenant_menu_pref）。
	// 菜单定制表由菜单任务引入，表不存在时静默跳过，不报错。
	CleanupTenantMenuRecords(tenantId string) error

	// ===== 网关（第三方工业网关接入）=====
	AddGateway(g models.Gateway) (models.Gateway, error)
	GatewayById(id string) (models.Gateway, error)
	GatewayBySn(sn string) (models.Gateway, error)
	GatewaySearch(offset int, limit int, req dtos.GatewaySearchQueryRequest) ([]models.Gateway, uint32, error)
	UpdateGateway(g models.Gateway) error
	DeleteGateway(id string) error
	UpdateGatewayStatus(id string, status int, ip string, onlineAt int64) error
	// MarkStaleGatewaysOffline 把「最后在线时间早于 beforeMs」的在线网关批量置为离线，返回影响行数
	MarkStaleGatewaysOffline(beforeMs int64) (int64, error)
	TouchGatewayReport(id string, ts int64) error
	UpdateGatewayConfig(id, config string, version int) error
	GatewayIngestLogExists(gatewayId string, seq int64) (bool, error)
	AddGatewayIngestLog(log models.GatewayIngestLog) error
	GatewayDeviceCount(gatewayId string) (int64, error)

	// ===== 网关接入：设备定位 =====
	DeviceByGatewayAndKey(gatewayId, deviceKey string) (models.Device, error)
	DeviceBySn(deviceSn string) (models.Device, error)

	// ===== 参考系统（/edge 协议）接入 =====
	// GatewayByPkSn 按「外部协议产品标识 + 序列号」定位网关（空 pk 视为任意）
	GatewayByPkSn(pk, sn string) (models.Gateway, error)
	// UpsertGatewayDevice 按 (gateway_id, device_key) 插入或更新设备，返回是否新建
	UpsertGatewayDevice(d models.Device) (models.Device, bool, error)
	// ListGatewayDevices 取网关下已注册设备，按 device_key 索引（全量清单比对用）
	ListGatewayDevices(gatewayId string) (map[string]models.Device, error)
	// MarkDevicesAbsent 把全量清单里已消失的设备标记为摘除（不删行）
	MarkDevicesAbsent(ids []string) (int64, error)
	// UpsertDeviceTags 写入/更新设备级测点定义（无物模型设备的读取侧来源）
	UpsertDeviceTags(deviceId string, tags []models.DeviceTag) (int, error)
	ListDeviceTags(deviceId string) ([]models.DeviceTag, error)

	// ===== 画布看板（落库，按租户隔离）=====
	VizBoards() ([]models.VizBoard, error)
	VizBoardsReplace(list []models.VizBoard) error

	// ===== 方案（行业方案包）=====
	BatchUpsertSolution(list []models.Solution) (int64, error)
	SolutionSearch(tenantId, industry string) ([]models.Solution, error)
	SolutionByKey(tenantId, key string) (models.Solution, error)
	// AddSolution 新增一条方案记录（租户副本克隆用，id 由调用方给出新随机值）
	AddSolution(s models.Solution) error
	// UpdateSolutionThingModel 只更新指定记录的物模型 JSON（按主键定位，内置行不受影响）
	UpdateSolutionThingModel(id, thingModelJson string) error

	QuickNavigationSearch(offset int, limit int, req dtos.QuickNavigationSearchQueryRequest) ([]models.QuickNavigation, uint32, error)
	DocsSearch(offset int, limit int, req dtos.DocsSearchQueryRequest) (docs []models.Doc, total uint32, edgeXErr error)
	BatchUpsertDocsTemplate(d []models.Doc) (int64, error)
	BatchUpsertQuickNavigationTemplate(ds []models.QuickNavigation) (int64, error)
	DeleteQuickNavigation(id string) error

	AddDeviceService(ds models.DeviceService) (models.DeviceService, error)
	DeviceServiceById(id string) (models.DeviceService, error)
	DeleteDeviceServiceById(id string) error
	UpdateDeviceService(ds models.DeviceService) error
	DeviceServicesSearch(offset int, limit int, req dtos.DeviceServiceSearchQueryRequest) ([]models.DeviceService, uint32, error)

	AddProduct(ds models.Product) (models.Product, error)
	ProductsSearch(offset int, limit int, preload bool, req dtos.ProductSearchQueryRequest) ([]models.Product, uint32, error)
	ProductById(id string) (models.Product, error)
	ProductByCloudId(id string) (models.Product, error)
	BatchUpsertProduct(d []models.Product) (int64, error)
	BatchSaveProduct(p []models.Product) error
	BatchDeleteProduct(products []models.Product) error
	BatchDeleteProperties(propertiesId []string) error
	BatchDeleteSystemProperties() error
	BatchInsertSystemProperties(p []models.Properties) (int64, error)
	BatchDeleteSystemEvents() error
	BatchInsertSystemEvents(p []models.Events) (int64, error)
	BatchDeleteEvents(eventId []string) error
	BatchDeleteActions(actionId []string) error
	DeleteProductById(id string) error
	BatchDeleteSystemActions() error
	BatchInsertSystemActions(p []models.Actions) (int64, error)
	DeleteProductObject(d models.Product) error
	UpdateProduct(ds models.Product) error
	AssociationsUpdateProduct(ds models.Product) error
	AssociationsDeleteProductObject(ds models.Product) error

	AddThingModelProperty(ds models.Properties) (models.Properties, error)
	BatchUpsertThingModel(ds interface{}) (int64, error)
	AddThingModelEvent(ds models.Events) (models.Events, error)
	AddThingModelAction(ds models.Actions) (models.Actions, error)
	UpdateThingModelProperty(ds models.Properties) error
	UpdateThingModelEvent(ds models.Events) error
	UpdateThingModelAction(ds models.Actions) error
	ThingModelDeleteProperty(id string) error
	ThingModelDeleteEvent(id string) error
	ThingModelDeleteAction(id string) error
	ThingModelPropertyById(id string) (models.Properties, error)
	ThingModelEventById(id string) (models.Events, error)
	ThingModelActionsById(id string) (models.Actions, error)
	SystemThingModelSearch(modelType, modelName string) (interface{}, error)

	CategoryTemplateSearch(offset int, limit int, req dtos.CategoryTemplateRequest) ([]models.CategoryTemplate, uint32, error)
	CategoryTemplateById(id string) (models.CategoryTemplate, error)
	BatchUpsertCategoryTemplate(d []models.CategoryTemplate) (int64, error)
	ThingModelTemplateSearch(offset int, limit int, req dtos.ThingModelTemplateRequest) ([]models.ThingModelTemplate, uint32, error)
	ThingModelTemplateByCategoryKey(categoryKey string) (models.ThingModelTemplate, error)
	BatchUpsertThingModelTemplate(d []models.ThingModelTemplate) (int64, error)

	UnitSearch(offset int, limit int, req dtos.UnitRequest) ([]models.Unit, uint32, error)
	BatchUpsertUnitTemplate(d []models.Unit) (int64, error)

	AddDevice(d models.Device) (string, error)
	DeviceById(id string) (models.Device, error)
	DeviceOnlineById(id string) (edgeXErr error)
	DeviceOfflineById(id string) (edgeXxErr error)
	DeviceOfflineByCloudInstanceId(id string) (edgeXErr error)
	// MarkStaleDevicesOffline 把「最后在线时间早于 beforeMs」的在线设备批量置为离线，返回影响行数
	MarkStaleDevicesOffline(beforeMs int64) (int64, error)
	MsgReportDeviceById(id string) (device models.Device, edgeXErr error)
	DeviceByCloudId(id string) (models.Device, error)
	DevicesSearch(offset int, limit int, req dtos.DeviceSearchQueryRequest) ([]models.Device, uint32, error)
	DeviceMqttAuthInfo(id string) (device models.MqttAuth, edgeXErr error)
	DriverMqttAuthInfo(id string) (device models.MqttAuth, edgeXErr error)
	AddMqttAuthInfo(auth models.MqttAuth) (string, error)
	AddOrUpdateAuth(auth models.MqttAuth) error
	BatchUpsertDevice(d []models.Device) (int64, error)
	BatchDeleteDevice(deviceIds []string) error
	BatchUnBindDevice(ids []string) error
	BatchBindDevice(ids []string, driverInstanceId string) error
	DeleteDeviceById(id string) error
	UpdateDevice(ds models.Device) error
	DeleteDeviceByCloudInstanceId(cloudInstanceId string) error
	AddDeviceLibrary(dl models.DeviceLibrary) (models.DeviceLibrary, error)
	DeviceLibraryById(id string) (models.DeviceLibrary, error)
	DeleteDeviceLibraryById(id string) error
	DeviceLibrariesSearch(offset int, limit int, req dtos.DeviceLibrarySearchQueryRequest) ([]models.DeviceLibrary, uint32, error)
	UpdateDeviceLibrary(dl models.DeviceLibrary) error

	DriverClassifySearch(offset int, limit int, req dtos.DriverClassifyQueryRequest) ([]models.DriverClassify, uint32, error)

	DockerConfigAdd(cfg models.DockerConfig) (models.DockerConfig, error)
	DockerConfigById(id string) (models.DockerConfig, error)
	DockerConfigUpdate(cfg models.DockerConfig) error
	DockerConfigDelete(id string) error
	DockerConfigsSearch(offset int, limit int, req dtos.DockerConfigSearchQueryRequest) ([]models.DockerConfig, uint32, error)

	AbilityByCode(model interface{}, code, productId string) (interface{}, error)

	AddMsgGather(msgGather models.MsgGather) error
	MsgGatherSearch(offset int, limit int, req dtos.MsgGatherSearchQueryRequest) (msgGather []models.MsgGather, count uint32, edgeXErr error)

	LanguageSdkByName(name string) (cloudService models.LanguageSdk, edgeXErr error)
	LanguageSearch(offset int, limit int, req dtos.LanguageSDKSearchQueryRequest) (languages []models.LanguageSdk, count uint32, edgeXErr error)
	AddLanguageSdk(cs models.LanguageSdk) (language models.LanguageSdk, edgeXErr error)
	UpdateLanguageSdk(ls models.LanguageSdk) error

	DeviceAlert
	UserDB
	SystemManageDB
	AdvancedDB
	SystemMonitor
}

type SystemMonitor interface {
	UpdateSystemMetrics(stats dtos.SystemMetrics) error
	GetSystemMetrics(start, end int64) ([]dtos.SystemMetrics, error)
	RemoveRangeSystemMetrics(min, max string) error
}

type UserDB interface {
	GetUserByUserName(username string) (models.User, error)
	UpdateUser(u models.User) error
	AddUser(u models.User) (models.User, error)
}

type DeviceAlert interface {
	AddAlertRule(rule models.AlertRule) (models.AlertRule, error)
	UpdateAlertRule(rule models.AlertRule) error
	AlertRuleById(id string) (models.AlertRule, error)
	AlertRuleSearch(offset int, limit int, req dtos.AlertRuleSearchQueryRequest) (alertRules []models.AlertRule, total uint32, edgeXErr error)
	DeleteAlertRuleById(id string) error
	AlertRuleStart(id string) error
	AlertRuleStop(id string) error

	AlertListLastSend(alertRuleId string) (alertList models.AlertList, edgeXErr error)
	AddAlertList(alertRule models.AlertList) (models.AlertList, error)
	AlertPlate(beforeTime int64) (plate []dtos.AlertPlateQueryResponse, err error)
	AlertListSearch(offset int, limit int, req dtos.AlertSearchQueryRequest) (alertList []dtos.AlertSearchQueryResponse, total uint32, edgeXErr error)
	AlertIgnore(id string) (edgeXErr error)
	TreatedIgnore(id, message string) (edgeXErr error)
}
