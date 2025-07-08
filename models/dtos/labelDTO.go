package dtos

// TagQueryRequest 标签查询请求
type TagQueryRequest struct {
	TagName  string `json:"tagName"`
	TagValue string `json:"tagValue"`
}

// DeviceQueryRequest 设备查询请求
type DeviceQueryRequest struct {
	DeviceName string `json:"deviceName"`
}

// TagValueRequest 标签值查询请求
type TagValueRequest struct {
	DeviceName string `json:"deviceName"`
	TagName    string `json:"tagName"`
}

// TagAddRequest 标签添加请求
type TagAddRequest struct {
	DeviceName string `json:"deviceName"`
	TagName    string `json:"tagName"`
	TagValue   string `json:"tagValue"`
}

// TagRemoveRequest 标签删除请求
type TagRemoveRequest struct {
	DeviceName string `json:"deviceName"`
	TagName    string `json:"tagName"`
}
