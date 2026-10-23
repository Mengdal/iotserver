package dtos

// 高级能力-资源管理（HTTP推送 / MQTT / Kafka / InfluxDB）

// DataResourceAddRequest 新增资源实例
type DataResourceAddRequest struct {
	Name   string                 `json:"name"`
	Type   string                 `json:"type"`
	Option map[string]interface{} `json:"option"`
}

// DataResourceUpdateRequest 修改资源实例
type DataResourceUpdateRequest struct {
	Id     string                 `json:"id"`
	Name   string                 `json:"name"`
	Type   string                 `json:"type"`
	Option map[string]interface{} `json:"option"`
}

// DataResourceSearchRequest 资源实例查询（按类型过滤 + 分页）
type DataResourceSearchRequest struct {
	BaseSearchConditionQuery `schema:",inline"`
	Type                     string `schema:"type,omitempty" form:"type"`
}

// DataResourceResponse 资源实例出参
type DataResourceResponse struct {
	Id       string                 `json:"id"`
	Name     string                 `json:"name"`
	Type     string                 `json:"type"`
	Option   map[string]interface{} `json:"option"`
	Health   *bool                  `json:"health"`
	Created  int64                  `json:"created"`
	Modified int64                  `json:"modified"`
}

// DataResourceVerifyResponse 验证结果
type DataResourceVerifyResponse struct {
	Health  bool   `json:"health"`
	Message string `json:"message"`
}
