package interfaces

import "iotserver/internal/models"

// IotpSyncItf 把平台元数据同步到 iotp 时序库。
//
// 实现必须是异步、失败隔离的：iotp 不可用不能阻塞产品/设备创建。
type IotpSyncItf interface {
	Enabled() bool
	// ProductCreated 产品创建/发布：建库建表、同步基础标签与物模型字段。
	ProductCreated(product models.Product, tenantID string)
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
}
