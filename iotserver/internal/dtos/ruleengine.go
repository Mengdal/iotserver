package dtos

import (
	"bytes"
	"encoding/json"
	"strings"

	"iotserver/internal/models"
)

/** ******************************* 高级能力 - 规则引擎 *******************************/

// RuleEngineSearchQueryRequest 规则列表查询
type RuleEngineSearchQueryRequest struct {
	BaseSearchConditionQuery `schema:",inline"`
	Status                   string `schema:"status,omitempty" form:"status"`
	NameLike                 string `schema:"nameLike,omitempty" form:"nameLike"`
	// Keyword ID/名称/标签模糊搜索
	Keyword string `schema:"keyword,omitempty" form:"keyword"`
}

// RuleEngineActionRequest 规则动作（引用一个平台数据资源）
type RuleEngineActionRequest struct {
	ResourceId string `json:"resource_id"`
}

// RuleEngineAddRequest 新增规则（前端提交整个表单）
type RuleEngineAddRequest struct {
	// Id 可选自定义规则 ID；为空时由服务端生成
	Id          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	// Tags 规则标签
	Tags []string `json:"tags"`
	// Actions 动作列表；为空时回落到 data_resource_id（兼容旧接口）
	Actions        []RuleEngineActionRequest `json:"actions"`
	DataResourceId string                    `json:"data_resource_id"`
	// Method 前端 Radio 的资源类型字段名
	Method string `json:"method"`
	// ResourceType 编辑回填时也会带上
	ResourceType string                   `json:"resource_type"`
	Status       string                   `json:"status"`
	Filter       models.RuleEngineFilter  `json:"filter"`
	Options      models.RuleEngineOptions `json:"options"`
	// RunImmediately 保存后立即启动
	RunImmediately bool `json:"run_immediately"`
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

// RuleEngineActionInfo 规则动作出参（不回传敏感配置）
type RuleEngineActionInfo struct {
	ResourceId   string `json:"resource_id"`
	ResourceName string `json:"resource_name"`
	ResourceType string `json:"resource_type"`
}

// RuleEngineSearchQueryResponse 规则列表项与详情
type RuleEngineSearchQueryResponse struct {
	Id           string                   `json:"id"`
	Name         string                   `json:"name"`
	Description  string                   `json:"description"`
	Tags         []string                 `json:"tags"`
	Actions      []RuleEngineActionInfo   `json:"actions"`
	Options      models.RuleEngineOptions `json:"options"`
	ResourceType string                   `json:"resource_type"`
	Status       string                   `json:"status"`
	Created      int64                    `json:"created"`
	Modified     int64                    `json:"modified"`
	Filter       models.RuleEngineFilter  `json:"filter"`
	// DataResourceId 供编辑表单回填资源下拉（旧 swagger 未含，属兼容性补充）
	DataResourceId string `json:"data_resource_id"`
	// DataResource 列表「转发方式」列直接读 dataResource.type，必须是对象（不能为 null）
	DataResource DataResourceInfo `json:"dataResource"`
}

// RuleEngineBatchRequest 批量启停
type RuleEngineBatchRequest struct {
	Ids []string `json:"ids"`
}

// RuleEngineBatchFailure 批量操作失败项
type RuleEngineBatchFailure struct {
	Id      string `json:"id"`
	Message string `json:"message"`
}

// RuleEngineBatchResult 批量操作结果
type RuleEngineBatchResult struct {
	Success int                      `json:"success"`
	Failed  []RuleEngineBatchFailure `json:"failed"`
}

// RuleEngineValidateRequest SQL 校验
type RuleEngineValidateRequest struct {
	Sql string `json:"sql"`
}

// RuleEngineValidateResponse SQL 校验结果
type RuleEngineValidateResponse struct {
	Valid       bool   `json:"valid"`
	Message     string `json:"message"`
	CompiledSql string `json:"compiled_sql"`
	TenantId    string `json:"tenant_id,omitempty"`
}

// RuleEngineDebugData 试运行模拟数据。
// 兼容前端提交的单条对象（会被当成一条记录）和对象数组。
type RuleEngineDebugData []map[string]interface{}

func (d *RuleEngineDebugData) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		*d = nil
		return nil
	}
	if trimmed[0] == '{' {
		var one map[string]interface{}
		if err := json.Unmarshal(trimmed, &one); err != nil {
			return err
		}
		*d = RuleEngineDebugData{one}
		return nil
	}
	var list []map[string]interface{}
	if err := json.Unmarshal(trimmed, &list); err != nil {
		return err
	}
	*d = RuleEngineDebugData(list)
	return nil
}

// RuleEngineDebugRequest 规则试运行（模拟数据源）
type RuleEngineDebugRequest struct {
	Sql      string              `json:"sql"`
	Data     RuleEngineDebugData `json:"data"`
	Interval int                 `json:"interval"`
	Loop     *bool               `json:"loop"`
	// DataTemplate 可选：模拟 Sink 的 Data Template，对 SQL 输出再做一次格式化
	DataTemplate string `json:"data_template"`
	// MockEnabled 是否启用模拟数据源；false 时试运行直接使用真实数据源
	MockEnabled *bool `json:"mock_enabled"`
	// ExecuteActions 调试时是否按当前规则的动作配置真实推送数据
	ExecuteActions *bool `json:"execute_actions"`
	// ActionResourceIDs 调试时需要执行的数据资源 ID（与规则动作顺序一致）
	ActionResourceIDs []string `json:"action_resource_ids"`
}

// RuleEngineDebugActionResult 调试动作执行结果（以 SSE action 事件返回前端）
type RuleEngineDebugActionResult struct {
	Ok         bool   `json:"ok"`
	ResourceID string `json:"resource_id"`
	Name       string `json:"name"`
	Type       string `json:"type"`
	StatusCode int    `json:"status_code,omitempty"`
	DurationMs int64  `json:"duration_ms"`
	Error      string `json:"error,omitempty"`
}

// RuleEngineDebugResponse 规则试运行创建结果
type RuleEngineDebugResponse struct {
	Id string `json:"id"`
}

// RuleEngineExportAction 导出动作（按资源名称，便于跨环境导入）
type RuleEngineExportAction struct {
	ResourceName string `json:"resource_name"`
	ResourceType string `json:"resource_type"`
}

// RuleEngineExportRule 导出规则
type RuleEngineExportRule struct {
	Name        string                   `json:"name"`
	Description string                   `json:"description"`
	Tags        []string                 `json:"tags"`
	Filter      models.RuleEngineFilter  `json:"filter"`
	Actions     []RuleEngineExportAction `json:"actions"`
	Options     models.RuleEngineOptions `json:"options"`
}

// RuleEngineExportFile 规则导出文件
type RuleEngineExportFile struct {
	Version int                    `json:"version"`
	Rules   []RuleEngineExportRule `json:"rules"`
}

// RuleEngineImportRequest 规则导入
type RuleEngineImportRequest struct {
	Rules []RuleEngineExportRule `json:"rules"`
}

// RuleEngineImportFailure 导入失败项
type RuleEngineImportFailure struct {
	Name    string `json:"name"`
	Message string `json:"message"`
}

// RuleEngineImportResult 导入结果
type RuleEngineImportResult struct {
	Imported int                       `json:"imported"`
	Failed   []RuleEngineImportFailure `json:"failed"`
}
