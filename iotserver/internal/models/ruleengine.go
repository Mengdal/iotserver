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
	// Sql 用户编辑的原始 SQL（不含平台注入的租户条件）
	Sql string `json:"sql"`
	// CompiledSql 实际下发到 eKuiper 的 SQL（含平台注入的租户条件）。
	// 存量规则没有该字段，读取时回落到 Sql。
	CompiledSql string `json:"compiled_sql,omitempty"`
}

func (f RuleEngineFilter) Value() (driver.Value, error) {
	return GormValueWrap(f)
}

func (f *RuleEngineFilter) Scan(value interface{}) error {
	return GormScanWrap(value, f)
}

// RuleEngineAction 规则动作：引用一个数据资源
type RuleEngineAction struct {
	ResourceId   string `json:"resource_id"`
	ResourceName string `json:"resource_name,omitempty"`
	ResourceType string `json:"resource_type,omitempty"`
}

// RuleEngineActions 动作列表 JSON 列
type RuleEngineActions []RuleEngineAction

func (a RuleEngineActions) Value() (driver.Value, error) {
	return GormValueWrap(a)
}

func (a *RuleEngineActions) Scan(value interface{}) error {
	return GormScanWrap(value, a)
}

// RuleEngineTags 规则标签 JSON 列
type RuleEngineTags []string

func (t RuleEngineTags) Value() (driver.Value, error) {
	return GormValueWrap(t)
}

func (t *RuleEngineTags) Scan(value interface{}) error {
	return GormScanWrap(value, t)
}

// RuleEngineRestartStrategy 规则重试策略（eKuiper 2.4.2 仅 attempts 生效，其余字段透传）
type RuleEngineRestartStrategy struct {
	Attempts     int     `json:"attempts,omitempty"`
	Delay        int     `json:"delay,omitempty"`
	MaxDelay     int     `json:"maxDelay,omitempty"`
	Multiplier   float64 `json:"multiplier,omitempty"`
	JitterFactor float64 `json:"jitterFactor,omitempty"`
}

// RuleEngineOptions eKuiper 规则选项（规则选项 Tab）
type RuleEngineOptions struct {
	Debug                    *bool                      `json:"debug,omitempty"`
	IsEventTime              *bool                      `json:"isEventTime,omitempty"`
	LateTolerance            int64                      `json:"lateTolerance,omitempty"`
	Concurrency              int                        `json:"concurrency,omitempty"`
	BufferLength             int                        `json:"bufferLength,omitempty"`
	SendMetaToSink           *bool                      `json:"sendMetaToSink,omitempty"`
	SendNilField             *bool                      `json:"sendNilField,omitempty"`
	SendError                *bool                      `json:"sendError,omitempty"`
	Qos                      int                        `json:"qos,omitempty"`
	CheckpointInterval       int64                      `json:"checkpointInterval,omitempty"`
	DisableBufferFullDiscard *bool                      `json:"disableBufferFullDiscard,omitempty"`
	RestartStrategy          *RuleEngineRestartStrategy `json:"restartStrategy,omitempty"`
	Cron                     string                     `json:"cron,omitempty"`
	Duration                 string                     `json:"duration,omitempty"`
}

func (o RuleEngineOptions) Value() (driver.Value, error) {
	return GormValueWrap(o)
}

func (o *RuleEngineOptions) Scan(value interface{}) error {
	return GormScanWrap(value, o)
}

// RuleEngine 规则引擎规则：SQL 从 mqtt_stream 取数，动作转发到绑定的数据资源
type RuleEngine struct {
	Timestamps
	TenantField
	Id       string `json:"id" gorm:"column:id;primaryKey;type:string;size:64;comment:主键"`
	Name     string `json:"name" gorm:"column:name;type:string;size:64;index;comment:规则名称"`
	Describe string `json:"description" gorm:"column:description;type:string;size:255;comment:规则描述"`
	Status   string `json:"status" gorm:"column:status;type:string;size:32;index;comment:运行状态 running/stopped"`
	// Tags 规则标签（eKuiper tags）
	Tags RuleEngineTags `json:"tags" gorm:"column:tags;type:text;comment:规则标签(JSON)"`
	// Actions 动作列表；一张规则可绑定多个数据资源
	Actions RuleEngineActions `json:"actions" gorm:"column:actions;type:text;comment:动作列表(JSON)"`
	// Options eKuiper 规则选项
	Options RuleEngineOptions `json:"options" gorm:"column:options;type:text;comment:规则选项(JSON)"`
	// DataResourceId 首个动作的目标数据资源（兼容旧接口/旧前端）
	DataResourceId string `json:"data_resource_id" gorm:"column:data_resource_id;type:string;size:64;index;comment:目标数据资源ID"`
	// ResourceType 首个动作的资源类型（冗余存储，列表「转发方式」列直接展示）
	ResourceType string `json:"resource_type" gorm:"column:resource_type;type:string;size:64;comment:目标资源类型"`
	// Filter 条件过滤（消息源/查询字段/条件/SQL）
	Filter RuleEngineFilter `json:"filter" gorm:"column:filter;type:text;comment:条件过滤(JSON)"`
}

func (r *RuleEngine) TableName() string {
	return TableRuleEngine
}
