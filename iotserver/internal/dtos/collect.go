package dtos

// CollectChannelDTO 采集通道（采集引擎中的 Channel）。
type CollectChannelDTO struct {
	Id             int64          `json:"id"`
	Name           string         `json:"name"`
	ChannelType    string         `json:"channelType"`
	Protocol       string         `json:"protocol"`
	Config         map[string]any `json:"config"`
	PollIntervalMs int            `json:"pollIntervalMs"`
	TimeoutMs      int            `json:"timeoutMs"`
	Enabled        bool           `json:"enabled"`
}

// CollectChannelSaveRequest 新增/修改采集通道。
type CollectChannelSaveRequest struct {
	Id             int64          `json:"id"`
	Name           string         `json:"name" binding:"required"`
	ChannelType    string         `json:"channelType"`
	Protocol       string         `json:"protocol"`
	Config         map[string]any `json:"config"`
	PollIntervalMs int            `json:"pollIntervalMs"`
	TimeoutMs      int            `json:"timeoutMs"`
	Enabled        bool           `json:"enabled"`
}

// CollectDeviceDTO 通道下的 IOTP 设备绑定。
// IotpId 是采集引擎侧的 IOTP ID；DeviceId/DeviceName/Product 由系统设备表按 IOTP ID 映射。
type CollectDeviceDTO struct {
	IotpId      int64  `json:"iotpId"`
	ChannelId   int64  `json:"channelId"`
	DeviceId    string `json:"deviceId"`
	DeviceName  string `json:"deviceName"`
	ProductId   string `json:"productId"`
	ProductName string `json:"productName"`
}

// CollectChannelBindRequest 把系统设备绑定到通道（系统设备 ID 列表）。
type CollectChannelBindRequest struct {
	DeviceIds []string `json:"deviceIds" binding:"required"`
}

// CollectPointDTO 点表一行：标签ID / 地址 / 数据类型 / 读写 / 倍率、偏移 / 存储。
// Id 是采集引擎点位 ID。
type CollectPointDTO struct {
	Id         int64   `json:"id"`
	TagId      string  `json:"tagId"`
	Address    string  `json:"address"`
	DataType   string  `json:"dataType"`
	AccessMode string  `json:"accessMode"`
	Multiplier float64 `json:"multiplier"`
	Offset     float64 `json:"offset"`
	Storage    bool    `json:"storage"`
	Enabled    bool    `json:"enabled"`
	Sort       int     `json:"sort"`
}

// CollectConfigResponse 设备点表配置。
// DeviceId 即 IOTP ID（原来的 deviceId）。
type CollectConfigResponse struct {
	DeviceId    int64             `json:"deviceId"`
	DeviceName  string            `json:"deviceName"`
	ProductId   string            `json:"productId"`
	ProductName string            `json:"productName"`
	ChannelId   int64             `json:"channelId"`
	ChannelName string            `json:"channelName"`
	Enabled     bool              `json:"enabled"`
	Points      []CollectPointDTO `json:"points"`
}

// CollectConfigSaveRequest 保存设备点表。
type CollectConfigSaveRequest struct {
	DeviceId int64             `json:"deviceId" binding:"required"`
	Enabled  bool              `json:"enabled"`
	Points   []CollectPointDTO `json:"points"`
}

// CollectMonitorValueDTO 采集实时监控的一个点位。
type CollectMonitorValueDTO struct {
	TagId    string `json:"tagId"`
	Value    any    `json:"value"`
	Time     int64  `json:"time"`
	Status   string `json:"status"` // normal | stale | no_data | disabled
	DataType string `json:"dataType,omitempty"`
}

// CollectMonitorResponse 设备实时监控结果。
type CollectMonitorResponse struct {
	Available bool                     `json:"available"`
	Message   string                   `json:"message,omitempty"`
	Items     []CollectMonitorValueDTO `json:"items"`
}
