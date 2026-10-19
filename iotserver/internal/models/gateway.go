package models

// 网关状态
const (
	GatewayStatusOffline = 0 // 离线
	GatewayStatusOnline  = 1 // 在线
	GatewayStatusNever   = 2 // 从未上线
)

// 网关接入协议。
//
// native 用平台自有协议（iot/up/{sn}/{data|batch|status|offline} 上行，
// 见 docs/gateway-integration.md）；edge 兼容参考系统（iotddc / things）的
// /edge/{connect,property,lastwill} 协议（见 docs/device-access-design.md）。
const (
	GatewayProtocolNative = "gatewaymqtt"
	GatewayProtocolEdge   = "edge"
)

// 子设备自动注册开关
const (
	GatewayAutoProvisionOff = 0 // 只记录清单，不注册设备
	GatewayAutoProvisionOn  = 1 // 清单里的设备自动注册并绑定（默认）
)

// Gateway 工业网关（第三方网关产品，负责 PLC 协议采集与断点续传）
//
// 与 Device 的关系：一个网关下挂多台设备（Device.GatewayId / Device.DeviceKey），
// 网关通过 MQTT 上行数据；平台不再依赖驱动容器做协议采集。
type Gateway struct {
	Timestamps  `gorm:"embedded"`
	TenantField `gorm:"embedded"`
	Id          string `gorm:"column:id;primaryKey;type:string;size:64;comment:主键"`
	// DeptId 所属组织节点（工厂/车间/产线）
	DeptId string `gorm:"column:dept_id;type:string;size:64;index;comment:组织节点ID"`
	Name   string `gorm:"column:name;type:string;size:255;comment:网关名称"`
	// Sn 网关序列号，同时作为 MQTT clientId / username（租户内唯一）
	Sn string `gorm:"column:sn;type:string;size:128;index;comment:网关序列号"`
	// Pk 外部协议的产品标识。参考系统的网关身份是 /edge/{kind}/{pk}/{sn}/...，
	// 即 pk 与 sn 两段共同定位（实测两者取值可相同）；自有协议不使用该字段。
	Pk string `gorm:"column:pk;type:string;size:128;index;comment:外部协议产品标识(pk)"`
	// Protocol 接入协议，取值见 GatewayProtocolNative / GatewayProtocolEdge
	Protocol string `gorm:"column:protocol;type:string;size:32;default:'gatewaymqtt';comment:接入协议"`
	// AutoProvision 子设备是否随清单自动注册，取值见 GatewayAutoProvisionOn/Off
	AutoProvision int `gorm:"column:auto_provision;type:int;default:1;comment:子设备自动注册 1开 0关"`
	// ExcludeDeviceKeys 不注册的网关内设备标识名单（逗号分隔）。
	// 实测网关会把自己的时钟等上报为设备（dn=system/user），这类应排除，避免污染设备列表。
	ExcludeDeviceKeys string `gorm:"column:exclude_device_keys;type:string;size:1000;comment:排除的网关内设备标识(逗号分隔)"`
	Vendor            string `gorm:"column:vendor;type:string;size:128;comment:厂商"`
	Model             string `gorm:"column:model;type:string;size:128;comment:型号"`
	FirmwareVersion   string `gorm:"column:firmware_version;type:string;size:64;comment:固件版本"`
	// Secret MQTT 接入密钥（平台生成，不明文回显）
	Secret string `gorm:"column:secret;type:string;size:255;comment:接入密钥"`
	// Config 采集/上报配置（JSON，由平台下发）
	Config string `gorm:"column:config;type:text;comment:网关配置(JSON)"`
	// ConfigVersion 配置版本号，每次下发 +1；网关回执后确认生效
	ConfigVersion int `gorm:"column:config_version;type:int;default:0;comment:配置版本"`
	// Status 1在线 0离线 2从未上线
	Status       int    `gorm:"column:status;type:int;default:2;comment:状态 1在线 0离线 2从未上线"`
	LastOnlineAt int64  `gorm:"column:last_online_at;comment:最后在线时间"`
	LastReportAt int64  `gorm:"column:last_report_at;comment:最后上报时间"`
	Ip           string `gorm:"column:ip;type:string;size:64;comment:最近一次上报IP"`
	Remark       string `gorm:"column:remark;type:string;size:500;comment:备注"`
}

func (table *Gateway) TableName() string {
	return "gateway"
}

func (table *Gateway) Get() interface{} {
	return *table
}

// TableGateway 网关表名
const TableGateway = "gateway"

// GatewayIngestLog 网关上行消息的幂等/对账记录。
//
// 唯一索引 (gateway_id, seq)：网关补传时重复发送同一 seq 只会落一条记录，
// 平台据此丢弃重复报文（断点续传的幂等基础）。
type GatewayIngestLog struct {
	TenantField
	Id         string `gorm:"column:id;primaryKey;type:string;size:64;comment:主键"`
	GatewayId  string `gorm:"column:gateway_id;type:string;size:64;index;comment:网关ID"`
	Seq        int64  `gorm:"column:seq;comment:网关消息序号"`
	MsgCount   int    `gorm:"column:msg_count;comment:本批包含的数据点数量"`
	Status     int    `gorm:"column:status;type:int;comment:1成功 0失败"`
	Error      string `gorm:"column:error;type:string;size:500;comment:失败原因"`
	ReceivedAt int64  `gorm:"column:received_at;comment:平台接收时间"`
}

func (table *GatewayIngestLog) TableName() string {
	return "gateway_ingest_log"
}

// TableGatewayIngestLog 网关上行日志表名
const TableGatewayIngestLog = "gateway_ingest_log"
