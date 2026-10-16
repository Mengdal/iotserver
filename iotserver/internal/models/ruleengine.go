package models

import (
	"database/sql/driver"
)

// TableRuleEngine 高级能力-规则引擎表名
const TableRuleEngine = "rule_engine"

// RuleEngineFilter 规则的条件过滤配置（前端 filter 表单）
type RuleEngineFilter struct {
	ProductId     string `json:"product_id"`
	MessageSource string `json:"message_source"`
	SelectName    string `json:"select_name"`
	Condition     string `json:"condition"`
	Sql           string `json:"sql"`
}

func (f RuleEngineFilter) Value() (driver.Value, error) {
	return GormValueWrap(f)
}

func (f *RuleEngineFilter) Scan(value interface{}) error {
	return GormScanWrap(value, f)
}

// RuleEngine 规则引擎规则：SQL 从 mqtt_stream 取数，动作转发到绑定的数据资源
type RuleEngine struct {
	Timestamps
	TenantField
	Id       string `json:"id" gorm:"column:id;primaryKey;type:string;size:64;comment:主键"`
	Name     string `json:"name" gorm:"column:name;type:string;size:64;index;comment:规则名称"`
	Describe string `json:"description" gorm:"column:description;type:string;size:255;comment:规则描述"`
	Status   string `json:"status" gorm:"column:status;type:string;size:32;index;comment:运行状态 running/stopped"`
	// DataResourceId 规则动作的目标数据资源（高级能力-资源管理）
	DataResourceId string `json:"data_resource_id" gorm:"column:data_resource_id;type:string;size:64;index;comment:目标数据资源ID"`
	// ResourceType 目标资源类型（冗余存储，列表「转发方式」列直接展示）
	ResourceType string `json:"resource_type" gorm:"column:resource_type;type:string;size:64;comment:目标资源类型"`
	// Filter 条件过滤（消息源/查询字段/条件/SQL）
	Filter RuleEngineFilter `json:"filter" gorm:"column:filter;type:text;comment:条件过滤(JSON)"`
}

func (r *RuleEngine) TableName() string {
	return TableRuleEngine
}
