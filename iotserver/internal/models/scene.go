package models

import (
	"database/sql/driver"
)

// 高级能力-场景联动表名
const (
	TableScene    = "scene"
	TableSceneLog = "scene_log"
)

// SceneCondition 场景触发条件
type SceneCondition struct {
	// ConditionType notify=设备触发 / timer=定时触发
	ConditionType string `json:"condition_type"`
	// Option 条件参数：notify 用 trigger/device_id/code/decide_condition/value_type/value_cycle/type，
	// timer 用 cron_expression
	Option map[string]string `json:"option"`
}

type SceneConditions []SceneCondition

func (c SceneConditions) Value() (driver.Value, error) {
	return GormValueWrap(c)
}

func (c *SceneConditions) Scan(value interface{}) error {
	return GormScanWrap(value, c)
}

// SceneAction 场景执行动作（设置设备属性）
type SceneAction struct {
	Code        string `json:"code"`
	DataType    string `json:"data_type"`
	DeviceId    string `json:"device_id"`
	DeviceName  string `json:"device_name"`
	ProductId   string `json:"product_id"`
	ProductName string `json:"product_name"`
	Value       string `json:"value"`
}

type SceneActions []SceneAction

func (a SceneActions) Value() (driver.Value, error) {
	return GormValueWrap(a)
}

func (a *SceneActions) Scan(value interface{}) error {
	return GormScanWrap(value, a)
}

// Scene 场景联动
type Scene struct {
	Timestamps
	TenantField
	Id       string `json:"id" gorm:"column:id;primaryKey;type:string;size:64;comment:主键"`
	Name     string `json:"name" gorm:"column:name;type:string;size:64;index;comment:场景名称"`
	Describe string `json:"description" gorm:"column:description;type:string;size:255;comment:场景描述"`
	// Status running/stopped
	Status     string          `json:"status" gorm:"column:status;type:string;size:32;index;comment:运行状态"`
	Conditions SceneConditions `json:"conditions" gorm:"column:conditions;type:text;comment:触发条件(JSON)"`
	Actions    SceneActions    `json:"actions" gorm:"column:actions;type:text;comment:执行动作(JSON)"`
}

func (s *Scene) TableName() string {
	return TableScene
}

// SceneLog 场景联动日志
type SceneLog struct {
	Timestamps
	TenantField
	Id string `json:"id" gorm:"column:id;primaryKey;type:string;size:64;comment:主键"`
	// Name 场景名称（日志页「联动规则名称」列）
	Name string `json:"name" gorm:"column:name;type:string;size:64;index;comment:场景名称"`
	// SceneId 场景ID
	SceneId string `json:"scene_id" gorm:"column:scene_id;type:string;size:64;index;comment:场景ID"`
	// ExecRes 执行结果（日志页「执行结果」列）
	ExecRes string `json:"exec_res" gorm:"column:exec_res;type:string;size:512;comment:执行结果"`
	// Success 是否全部动作执行成功
	Success bool `json:"success" gorm:"column:success;comment:是否成功"`
	// TriggerTime 触发时间（毫秒）
	TriggerTime int64 `json:"trigger_time" gorm:"column:trigger_time;comment:触发时间"`
}

func (s *SceneLog) TableName() string {
	return TableSceneLog
}
