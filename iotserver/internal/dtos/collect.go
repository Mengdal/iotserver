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
	Enabled     bool   `json:"enabled"`
}

// CollectConfigEnabledRequest 设备级采集使能开关。
type CollectConfigEnabledRequest struct {
	DeviceId int64 `json:"deviceId" binding:"required"`
	Enabled  bool  `json:"enabled"`
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
	DeviceId    int64  `json:"deviceId"`
	DeviceName  string `json:"deviceName"`
	ProductId   string `json:"productId"`
	ProductName string `json:"productName"`
	ChannelId   int64  `json:"channelId"`
	ChannelName string `json:"channelName"`
	Enabled     bool   `json:"enabled"`
	// Configured 是否已有保存过的点位（不是物模型模板）
	Configured bool              `json:"configured"`
	Points     []CollectPointDTO `json:"points"`
}

// CollectConfigSaveRequest 保存设备点表。
type CollectConfigSaveRequest struct {
	DeviceId int64             `json:"deviceId" binding:"required"`
	Enabled  bool              `json:"enabled"`
	Points   []CollectPointDTO `json:"points"`
}

// CollectMonitorValueDTO 采集实时监控的一个点位。
type CollectMonitorValueDTO struct {
	TagId          string `json:"tagId"`
	Value          any    `json:"value"`
	RawValue       any    `json:"rawValue"`
	Time           int64  `json:"time"`
	Status         string `json:"status"` // normal | stale | no_data | disabled | error
	DataType       string `json:"dataType,omitempty"`
	LastWriteValue string `json:"lastWriteValue,omitempty"`
	LastWriteTime  int64  `json:"lastWriteTime,omitempty"`
}

// CollectMonitorResponse 设备实时监控结果。
type CollectMonitorResponse struct {
	Available bool                     `json:"available"`
	Message   string                   `json:"message,omitempty"`
	Items     []CollectMonitorValueDTO `json:"items"`
}

// CollectImportDevicePreview 批量导入预览中的一台设备。
type CollectImportDevicePreview struct {
	DeviceId    int64  `json:"deviceId"`
	DeviceName  string `json:"deviceName"`
	ChannelId   int64  `json:"channelId"`
	ChannelName string `json:"channelName"`
	Points      int    `json:"points"`
}

// CollectImportRowError 批量导入的行级错误。
type CollectImportRowError struct {
	Row     int    `json:"row"`
	Message string `json:"message"`
}

// CollectImportPreview 批量导入预览结果（不写库）。
type CollectImportPreview struct {
	Devices []CollectImportDevicePreview `json:"devices"`
	Errors  []CollectImportRowError      `json:"errors"`
}

// CollectImportDeviceError 批量导入应用阶段单台设备的失败原因。
type CollectImportDeviceError struct {
	DeviceId   int64  `json:"deviceId"`
	DeviceName string `json:"deviceName"`
	Message    string `json:"message"`
}

// CollectImportResult 批量导入应用结果。
type CollectImportResult struct {
	Devices int                        `json:"devices"`
	Points  int                        `json:"points"`
	Failed  []CollectImportDeviceError `json:"failed"`
}

// CollectOverviewChannelDTO 采集概览中的通道汇总。
type CollectOverviewChannelDTO struct {
	Id          int64  `json:"id"`
	Name        string `json:"name"`
	ChannelType string `json:"channelType"`
	Protocol    string `json:"protocol"`
	DeviceCount int    `json:"deviceCount"`
	PointCount  int    `json:"pointCount"`
	Enabled     bool   `json:"enabled"`
}

// CollectOverviewDTO 采集配置总体概览。
type CollectOverviewDTO struct {
	ChannelCount        int                         `json:"channelCount"`
	DeviceCount         int                         `json:"deviceCount"`
	PointCount          int                         `json:"pointCount"`
	EnabledDeviceCount  int                         `json:"enabledDeviceCount"`
	DisabledDeviceCount int                         `json:"disabledDeviceCount"`
	Channels            []CollectOverviewChannelDTO `json:"channels"`
}

// CollectRealtimeValueDTO 采集实时数据查询的一行。
type CollectRealtimeValueDTO struct {
	IotpId         int64  `json:"iotpId"`
	DeviceName     string `json:"deviceName"`
	ChannelId      int64  `json:"channelId"`
	ChannelName    string `json:"channelName"`
	TagId          string `json:"tagId"`
	DataType       string `json:"dataType"`
	Value          string `json:"value"`
	RawValue       any    `json:"rawValue"`
	Status         string `json:"status"`
	Time           int64  `json:"time"`
	LastWriteValue string `json:"lastWriteValue,omitempty"`
	LastWriteTime  int64  `json:"lastWriteTime,omitempty"`
}

// CollectWriteRequest 点位写入请求。
type CollectWriteRequest struct {
	DeviceId int64  `json:"deviceId" binding:"required"`
	TagId    string `json:"tagId" binding:"required"`
	Value    string `json:"value"`
}
