package dtos

import "iotserver/internal/models"

/** ******************************* 高级能力 - 场景联动 *******************************/

// SceneAddRequest 新建场景（前端「新建场景」只提交名称与描述，随后再编辑配置）
type SceneAddRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// SceneUpdateRequest 编辑场景：配置触发条件与执行动作（前端整体提交）
type SceneUpdateRequest struct {
	Id          string                  `json:"id"`
	Name        string                  `json:"name"`
	Description string                  `json:"description"`
	Conditions  []models.SceneCondition `json:"conditions"`
	Actions     []models.SceneAction    `json:"actions"`
	Status      string                  `json:"status"`
}

// SceneSearchQueryRequest 场景列表查询（status 为空串时不过滤）
type SceneSearchQueryRequest struct {
	BaseSearchConditionQuery `schema:",inline"`
	Status                   string `schema:"status,omitempty" form:"status"`
}

// SceneSearchQueryResponse 场景列表项/详情
type SceneSearchQueryResponse struct {
	Id          string                  `json:"id"`
	Name        string                  `json:"name"`
	Description string                  `json:"description"`
	Status      string                  `json:"status"`
	Created     int64                   `json:"created"`
	Modified    int64                   `json:"modified"`
	Conditions  []models.SceneCondition `json:"conditions"`
	Actions     []models.SceneAction    `json:"actions"`
}

// SceneLogSearchQueryRequest 场景日志查询（按场景 + 时间范围 + 分页）
type SceneLogSearchQueryRequest struct {
	BaseSearchConditionQuery `schema:",inline"`
	SceneId                  string `schema:"-" form:"-"`
	StartTime                int64  `schema:"start_time,omitempty" form:"start_time"`
	EndTime                  int64  `schema:"end_time,omitempty" form:"end_time"`
}

// SceneLogResponse 场景日志出参（前端列：联动规则名称/执行结果/触发时间）
type SceneLogResponse struct {
	Id          string `json:"id"`
	Name        string `json:"name"`
	SceneId     string `json:"scene_id"`
	ExecRes     string `json:"exec_res"`
	Success     bool   `json:"success"`
	TriggerTime int64  `json:"trigger_time"`
	Created     int64  `json:"created"`
}
