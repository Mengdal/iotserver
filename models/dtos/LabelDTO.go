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

// TagMultiAddRequest 单个设备多个标签
type TagMultiAddRequest struct {
	DeviceName string            `json:"deviceName"`
	Tags       map[string]string `json:"tags"` // {"tagA":"v1","tagB":"v2"}
}

// TagBatchAddRequest 多设备多个标签
type TagBatchAddRequest struct {
	Devices []TagMultiAddRequest `json:"devices"`
}

// ProductAddRequest 产品标签添加请求
type ProductAddRequest struct {
	ProductID  int64             `json:"productId"`  // 新增产品ID字段
	DeviceName []string          `json:"deviceName"` // 将设备ID改为数组
	Tags       map[string]string `json:"tags"`       // {"tagA":"v1","tagB":"v2"}
}

// TagRemoveRequest 标签删除请求
type TagRemoveRequest struct {
	DeviceName string `json:"deviceName"`
	TagName    string `json:"tagName"`
}
