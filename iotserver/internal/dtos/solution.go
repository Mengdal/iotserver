package dtos

/************** 方案（行业方案包）**************/

// SolutionSearchRequest 模板列表查询
type SolutionSearchRequest struct {
	Industry string `schema:"industry,omitempty" form:"industry"`
}

// SolutionResponse 模板详情（含物模型/告警/看板预设）
type SolutionResponse struct {
	Key            string `json:"key"`
	Name           string `json:"name"`
	NameEn         string `json:"nameEn"`
	Industry       string `json:"industry"`
	Version        int    `json:"version"`
	BuiltIn        bool   `json:"builtIn"`
	Icon           string `json:"icon"`
	Description    string `json:"description"`
	DescriptionEn  string `json:"descriptionEn"`
	ThingModelJson string `json:"thingModelJson"`
	AlarmRulesJson string `json:"alarmRulesJson"`
	DashboardJson  string `json:"dashboardJson"`
	MetricsJson    string `json:"metricsJson"`
}

// SolutionApplyRequest 一键应用模板
type SolutionApplyRequest struct {
	// ProductName 生成的产品名称（为空则用模板名称）
	ProductName string `json:"productName"`
	// DeptId 可选：把生成的设备挂到某个组织节点
	DeptId string `json:"deptId"`
	// DeviceCount >0 时同时创建对应数量的设备（名称 = 产品名-序号）
	DeviceCount int `json:"deviceCount"`
	// CreateAlarmRules 是否按模板预设创建告警规则（默认 true）
	CreateAlarmRules *bool `json:"createAlarmRules"`
}

// SolutionApplyResponse 应用结果摘要
type SolutionApplyResponse struct {
	ProductId    string   `json:"productId"`
	ProductName  string   `json:"productName"`
	DeviceIds    []string `json:"deviceIds"`
	AlarmRuleIds []string `json:"alarmRuleIds"`
	Skipped      []string `json:"skipped"`
}

/************** 方案物模型（TSL）条目维护 **************/

// 物模型能力类型：property→properties、event→events、action→services
const (
	SolutionItemTypeProperty = "property"
	SolutionItemTypeEvent    = "event"
	SolutionItemTypeAction   = "action"
)

// SolutionThingModelItemRequest 新增/编辑物模型条目
//
// type=property|event|action；identifier 在编辑时指向**被替换的旧标识符**；
// item 为完整条目对象（必须带非空 identifier）。
type SolutionThingModelItemRequest struct {
	Type       string                 `json:"type"`
	Identifier string                 `json:"identifier"`
	Item       map[string]interface{} `json:"item"`
}

// SolutionThingModelItemDeleteRequest 删除物模型条目（query 参数）
type SolutionThingModelItemDeleteRequest struct {
	Type       string `form:"type" json:"type"`
	Identifier string `form:"identifier" json:"identifier"`
}
