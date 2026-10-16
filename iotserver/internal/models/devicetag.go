package models

// DeviceTag 来源（DeviceTag.Source）
const (
	DeviceTagSourceAuto   = "auto"   // 上报/清单里自动发现
	DeviceTagSourceManual = "manual" // 人工校准（补单位、改类型）
)

// DeviceTag 设备级测点定义。
//
// 为什么需要它：平台的物模型是**产品级**的（Product.Properties/Events/Actions），
// 而随网关清单自动接入的子设备可能**没有产品**——参考系统的上行协议里根本没有
// 产品维度（数据报文只有 dn + properties，接入报文只有 deviceCode/tagCode）。
// 这类设备落库本身不需要物模型（时序写入不校验物模型），但**读取侧**要单位与类型
// （见 persistence.propertyUnit 从物模型取单位的逻辑），无产品时取不到。
//
// 因此本表承接「无物模型设备」的测点定义，读路径按
// 「产品物模型 > 设备级测点（本表）」回落，调用方无需关心设备有没有物模型。
type DeviceTag struct {
	Timestamps  `gorm:"embedded"`
	TenantField `gorm:"embedded"`
	Id          string `gorm:"column:id;primaryKey;type:string;size:64;comment:主键"`
	DeviceId    string `gorm:"column:device_id;type:string;size:255;index;comment:设备ID"`
	// Code 测点标识（源报文的 tagCode；实测可含空格，如 "DRoom Temp"）
	Code string `gorm:"column:code;type:string;size:255;comment:测点标识"`
	// Name 测点显示名
	Name string `gorm:"column:name;type:string;size:255;comment:测点名称"`
	// Type 数据类型：float/int/bool/text/unknown（自动发现时按首值推断）
	Type string `gorm:"column:type;type:string;size:32;default:'unknown';comment:数据类型"`
	// Unit 单位。自动发现时留空，待人工校准
	Unit string `gorm:"column:unit;type:string;size:64;comment:单位"`
	// AccessMode 读写权限 r/w/rw，由源报文 objectType 推导
	AccessMode string `gorm:"column:access_mode;type:string;size:8;comment:读写权限"`
	// Source 来源，取值见 DeviceTagSource*
	Source string `gorm:"column:source;type:string;size:16;default:'auto';comment:来源"`
	// DiscoveredAt 首次发现时间（毫秒）
	DiscoveredAt int64 `gorm:"column:discovered_at;comment:首次发现时间"`
}

func (table *DeviceTag) TableName() string {
	return TableDeviceTag
}

func (table *DeviceTag) Get() interface{} {
	return *table
}

// TableDeviceTag 设备级测点表名
const TableDeviceTag = "device_tag"
