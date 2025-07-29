package dtos

import "time"

type SceneCreate struct {
	Id          int64  `json:"id"`
	Description string `json:"description" example:"描述"`
	Name        string `json:"name" example:"场景名称"`
}

type SceneUpdateRequest struct {
	Id        int64       `json:"Id" example:"48023899"` // 场景ID
	Condition []Condition `json:"condition" `
	Action    []Action    `json:"action" `
}

// Condition 场景触发条件
type Condition struct {
	ConditionType string            ` json:"condition_type" example:"timer"`
	Option        map[string]string `json:"option" example:"{\"cron_expression\":\"00 00 * * 0,1,2,3,4\",\"decide_condition\":\"= undefined\"}"` // 存储为JSON字符串
}

// Action 场景动作
type Action struct {
	ProductId   string `json:"product_id" example:"83114221"`
	ProductName string `json:"product_name" example:"扭蛋机"`
	DeviceId    string `json:"device_id" example:"73763730"`
	DeviceName  string `json:"device_name" example:"1111"`
	Code        string `json:"code" example:"Count"`
	DataType    string `json:"data_type" example:"int"`
	Value       string `json:"value" example:"12"`
}

// SceneQueryRequest 查询场景请求
type SceneQueryRequest struct {
	Page     int    `json:"page" example:"1"`
	PageSize int    `json:"page_size" example:"10"`
	Name     string `json:"name" example:"场景名称"`
	Status   string `json:"status" example:"running"`
}

// SceneResponse 场景响应
type SceneResponse struct {
	Id          int64       `json:"id"`
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Status      string      `json:"status"`
	Created     int64       `json:"created"`
	Modified    int64       `json:"modified"`
	Conditions  []Condition `json:"conditions"`
	Actions     []Action    `json:"actions"`
}

// SceneLogQueryRequest 查询场景日志请求
type SceneLogQueryRequest struct {
	Page     int   `json:"page" example:"1"`
	PageSize int   `json:"page_size" example:"10"`
	SceneId  int64 `json:"scene_id" example:"1"`
}

// SceneLogResponse 场景日志响应
type SceneLogResponse struct {
	Id      int64  `json:"id"`
	SceneId int64  `json:"scene_id"`
	Name    string `json:"name"`
	ExecRes string `json:"exec_res"`
	Created int64  `json:"created"`
}

// OperateSceneReq 场景动作
type OperateSceneReq struct {
	Action  string `json:"action"  example:"stop"` // start 或 stop
	SceneId int64  `json:"sceneId"  example:"1"`
}

// SceneExecuteResponse 场景执行响应
type SceneExecuteResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// SceneListResponse 场景列表响应
type SceneListResponse struct {
	Total int64           `json:"total"`
	List  []SceneResponse `json:"list"`
}

// SceneLogListResponse 场景日志列表响应
type SceneLogListResponse struct {
	Total int64              `json:"total"`
	List  []SceneLogResponse `json:"list"`
}

type SceneStatus struct {
	SceneID string    `json:"scene_id"`
	EntryID int       `json:"entry_id"`
	Next    time.Time `json:"next"`
	Prev    time.Time `json:"prev"`
}
