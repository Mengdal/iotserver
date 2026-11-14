package interfaces

import "iotserver/internal/models"

// IotpSyncItf 把平台元数据同步到 iotp 时序库。
//
// 实现必须是异步、失败隔离的：iotp 不可用不能阻塞产品/设备创建。
type IotpSyncItf interface {
	Enabled() bool
	// ProductCreated 产品创建/发布：建库建表、同步基础标签与物模型字段。
	ProductCreated(product models.Product, tenantID string)
	// ProductUpdated 产品资料更新：刷新表字段元数据（product_id 字段的 product_name）。
	ProductUpdated(product models.Product, tenantID string)
	// PropertyChanged 物模型属性新增/修改：同步字段。
	PropertyChanged(productID string, property models.Properties, tenantID string)
	// EventChanged 物模型事件新增/修改：同步 evt_ 字段。
	EventChanged(productID string, event models.Events, tenantID string)
	// ActionChanged 物模型服务新增/修改：同步 act_ 字段。
	ActionChanged(productID string, action models.Actions, tenantID string)
	// DeviceCreated 设备创建：注册 iotp 设备实例。
	DeviceCreated(device models.Device, product models.Product, tenantID string)
	// DeviceDeleted 设备删除：删除 iotp 设备实例（历史数据保留）。
	DeviceDeleted(device models.Device)

	// GatewaySyncEnabled 网关归属同步是否启用。
	//
	// 与 Enabled() 不同：只有时序后端真正是 iotp（[Databases.Data].Type='iotp'）
	// 时才启用。DuckDB 后端下网关/设备归属只存在平台元数据库，不需要同步。
	GatewaySyncEnabled() bool
	// GatewayCreated 软网关创建：在 iotp 声明网关。
	GatewayCreated(gateway models.Gateway)
	// GatewayDeleted 软网关删除：清理设备归属并删除 iotp 网关声明。
	GatewayDeleted(gateway models.Gateway, deviceIDs []int64)
	// DeviceGatewayBound 设备绑定到软网关：确保设备实例存在并设置归属。
	DeviceGatewayBound(device models.Device, gatewaySn string)
	// DeviceGatewayUnbound 设备从软网关解绑：清除 iotp 设备归属。
	DeviceGatewayUnbound(iotpDeviceID int64)
	// ReconcileGateways 启动/定期对账：补齐缺失的网关声明与设备归属。
	ReconcileGateways()
}
