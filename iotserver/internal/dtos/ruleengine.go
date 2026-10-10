package dtos

import (
	"strings"

	"iotserver/internal/models"
)

/** ******************************* 高级能力 - 规则引擎 *******************************/

// RuleEngineSearchQueryRequest 规则列表查询
type RuleEngineSearchQueryRequest struct {
	BaseSearchConditionQuery `schema:",inline"`
	Status                   string `schema:"status,omitempty" form:"status"`
	NameLike                 string `schema:"nameLike,omitempty" form:"nameLike"`
}

// RuleEngineAddRequest 新增规则（前端提交整个表单）
type RuleEngineAddRequest struct {
	Name           string `json:"name"`
	Description    string `json:"description"`
	DataResourceId string `json:"data_resource_id"`
	// Method 前端 Radio 的资源类型字段名
	Method string `json:"method"`
	// ResourceType 编辑回填时也会带上
	ResourceType string                  `json:"resource_type"`
	Status       string                  `json:"status"`
	Filter       models.RuleEngineFilter `json:"filter"`
}

// ResourceTypeValue 资源类型取值：优先 resource_type，兼容早期前端使用的 method 字段
func (r RuleEngineAddRequest) ResourceTypeValue() string {
	if strings.TrimSpace(r.ResourceType) != "" {
		return r.ResourceType
	}
	return r.Method
}

// RuleEngineUpdateRequest 修改规则
type RuleEngineUpdateRequest struct {
	RuleEngineAddRequest
	Id string `json:"id"`
}

// DataResourceInfo 规则绑定的数据资源信息（与参考站/旧 swagger 契约一致）
type DataResourceInfo struct {
	Name   string                 `json:"name"`
	Option map[string]interface{} `json:"option"`
	Type   string                 `json:"type"`
}

// RuleEngineSearchQueryResponse 规则列表项与详情
type RuleEngineSearchQueryResponse struct {
	Id           string                  `json:"id"`
	Name         string                  `json:"name"`
	Description  string                  `json:"description"`
	ResourceType string                  `json:"resource_type"`
	Status       string                  `json:"status"`
	Created      int64                   `json:"created"`
	Modified     int64                   `json:"modified"`
	Filter       models.RuleEngineFilter `json:"filter"`
	// DataResourceId 供编辑表单回填资源下拉（旧 swagger 未含，属兼容性补充）
	DataResourceId string `json:"data_resource_id"`
	// DataResource 列表「转发方式」列直接读 dataResource.type，必须是对象（不能为 null）
	DataResource DataResourceInfo `json:"dataResource"`
}
