package models

const (
	TableCollectChannel      = "collect_channel"
	TableCollectDeviceConfig = "collect_device_config"
	TableCollectPointConfig  = "collect_point_config"
)

// CollectChannel 采集通道。
//
// 通道表示一组物理/逻辑连接（串口、网口或驱动实例），一台网关可以配置多个通道，
// 设备点位通过 channel_id 归属到具体通道。当前只有一个网关，通道表按租户隔离。
type CollectChannel struct {
	Timestamps  `gorm:"embedded"`
	TenantField `gorm:"embedded"`
	Id          string `gorm:"column:id;primaryKey;type:string;size:64;comment:主键"`
	// GatewayId 采集该通道的网关；当前只有一个网关，可为空
	GatewayId string `gorm:"column:gateway_id;type:string;size:64;index;comment:采集网关ID"`
	// RemoteId 采集引擎中的通道 ID；下发过一次后保存，改名后仍按 ID 更新
	RemoteId int64 `gorm:"column:remote_id;type:bigint;comment:采集引擎通道ID"`
	// Name 通道名称，如 C1 / C2，租户内唯一
	Name string `gorm:"column:name;type:string;size:64;not null;index;comment:通道名称"`
	// ChannelType serialport | network
	ChannelType string `gorm:"column:channel_type;type:string;size:32;comment:通道类型"`
	// Protocol 采集协议，如 ModbusTCPClient / ModbusRTUClient / OPCUAClient
	Protocol string `gorm:"column:protocol;type:string;size:64;comment:采集协议"`
	// Config 连接参数（JSON）：网口 {ip,port} / 串口 {com,baudRate,...} / 驱动 {driver}
	Config string `gorm:"column:config;type:text;comment:连接参数JSON"`
	// PollIntervalMs 轮询周期（毫秒），0=默认
	PollIntervalMs int `gorm:"column:poll_interval_ms;type:int;comment:轮询周期(ms)"`
	// TimeoutMs 超时（毫秒），0=默认
	TimeoutMs int    `gorm:"column:timeout_ms;type:int;comment:超时(ms)"`
	Enabled   bool   `gorm:"column:enabled;type:bool;comment:是否启用"`
	Remark    string `gorm:"column:remark;type:string;size:500;comment:备注"`
}

func (m *CollectChannel) TableName() string {
	return TableCollectChannel
}

// CollectDeviceConfig 设备级采集配置（所属通道、开关与备注）。
//
// 通道 → 设备 → 点表：设备挂在某个通道下，点位参数在 CollectPointConfig。
type CollectDeviceConfig struct {
	Timestamps  `gorm:"embedded"`
	TenantField `gorm:"embedded"`
	Id          string `gorm:"column:id;primaryKey;type:string;size:64;comment:主键"`
	// DeviceId 平台设备 ID；一台设备一份采集配置（device_id 全局唯一）
	DeviceId string `gorm:"column:device_id;type:string;size:64;not null;uniqueIndex:uk_collect_device;comment:平台设备ID"`
	// ChannelId 所属采集通道；一个设备只属于一个通道
	ChannelId string `gorm:"column:channel_id;type:string;size:64;index;comment:所属通道ID"`
	// RemoteId 采集引擎中的设备 ID；下发过一次后保存，改名后仍按 ID 更新
	RemoteId int64 `gorm:"column:remote_id;type:bigint;comment:采集引擎设备ID"`
	// GatewayId 采集该设备的网关；当前只有一个网关，可为空
	GatewayId string `gorm:"column:gateway_id;type:string;size:64;index;comment:采集网关ID"`
	Enabled   bool   `gorm:"column:enabled;type:bool;comment:是否启用"`
	Remark    string `gorm:"column:remark;type:string;size:500;comment:备注"`
}

func (m *CollectDeviceConfig) TableName() string {
	return TableCollectDeviceConfig
}

// CollectPointConfig 设备点位采集参数（点表一行）。
//
// 与研发示例表一致：标签ID / 地址 / 数据类型 / 读写 / 倍率、偏移 / 存储。
// 通道由设备所属通道决定；点位不再绑定物模型属性，物模型只用于首次生成模板行。
type CollectPointConfig struct {
	Timestamps  `gorm:"embedded"`
	TenantField `gorm:"embedded"`
	Id          string `gorm:"column:id;primaryKey;type:string;size:64;comment:主键"`
	DeviceId    string `gorm:"column:device_id;type:string;size:64;not null;uniqueIndex:uk_collect_point;comment:平台设备ID"`
	// TagId 标签ID，如 D1.a；同一设备内唯一
	TagId string `gorm:"column:tag_id;type:string;size:128;not null;uniqueIndex:uk_collect_point;comment:标签ID"`
	// RemoteId 采集引擎中的点位 ID；下发过一次后保存，改名后仍按 ID 更新
	RemoteId int64 `gorm:"column:remote_id;type:bigint;comment:采集引擎点位ID"`
	// Address 协议地址原文，如 slave=1;addr=400001 / nsindex=5;nodeid=Random1
	Address string `gorm:"column:address;type:string;size:255;comment:协议地址"`
	// DataType 采集数据类型，如 int16 / uint16 / int32 / float32 / bool / string
	DataType string `gorm:"column:data_type;type:string;size:32;comment:数据类型"`
	// AccessMode R / W / RW
	AccessMode string `gorm:"column:access_mode;type:string;size:16;comment:读写"`
	// Multiplier 倍率
	Multiplier float64 `gorm:"column:multiplier;type:float;comment:倍率"`
	// Offset 偏移
	Offset float64 `gorm:"column:offset;type:float;comment:偏移"`
	// Storage 是否存储
	Storage bool `gorm:"column:storage;type:bool;comment:是否存储"`
	Enabled bool `gorm:"column:enabled;type:bool;comment:是否启用"`
	// LastWriteValue 最后一次通过平台写入的值（异步下发，不一定等于最终生效值）
	LastWriteValue string `gorm:"column:last_write_value;type:string;size:255;comment:最后设定值"`
	LastWriteTime  int64  `gorm:"column:last_write_time;type:bigint;comment:最后设定时间"`
	Sort           int    `gorm:"column:sort;type:int;comment:排序"`
}

func (m *CollectPointConfig) TableName() string {
	return TableCollectPointConfig
}
