package models

// Solution 行业方案。
//
// 模板 = 物模型 + 告警规则 + 看板 + 控制命令 + OEE 指标映射，
// 一键应用即可生成「产品 + 告警规则 + 看板」，供租户按行业/机型快速复制。
//
// 归属语义（区别于严格的租户隔离表）：tenant_id 为空表示**平台内置模板**，
// 所有租户可见；非空表示某租户自定义的模板，仅该租户可见。
// 因此查询需显式过滤 `tenant_id IN (”, <当前租户>)`，不适用 GORM 租户插件的 fail-closed 规则。
type Solution struct {
	Timestamps `gorm:"embedded"`
	Id         string `gorm:"column:id;primaryKey;type:string;size:64;comment:主键"`
	// Key 模板标识（如 injection_molding），全局唯一
	Key string `gorm:"column:key;type:string;size:64;index;comment:模板标识"`
	// TenantId 空=平台内置模板
	TenantId string `gorm:"column:tenant_id;type:string;size:64;index;comment:租户ID(空=内置)"`
	Name     string `gorm:"column:name;type:string;size:128;comment:模板名称"`
	Industry string `gorm:"column:industry;type:string;size:64;comment:所属行业"`
	Version  int    `gorm:"column:version;type:int;default:1;comment:版本"`
	// BuiltIn 1=平台内置（不可删除）
	BuiltIn     int    `gorm:"column:built_in;type:int;default:0;comment:是否内置 1是 0否"`
	Icon        string `gorm:"column:icon;type:string;size:64;comment:图标"`
	Description string `gorm:"column:description;type:string;size:500;comment:描述"`
	// ThingModelJson 物模型：{properties,events,services}
	ThingModelJson string `gorm:"column:thing_model_json;type:text;comment:物模型(JSON)"`
	// AlarmRulesJson 告警规则预设：[{name,propertyCode,operator,value,alertLevel,alertType,silenceTime,description}]
	AlarmRulesJson string `gorm:"column:alarm_rules_json;type:text;comment:告警规则预设(JSON)"`
	// DashboardJson 看板预设：{name,widgets:[{title,type,propertyCode,...}]}
	DashboardJson string `gorm:"column:dashboard_json;type:text;comment:看板预设(JSON)"`
	// MetricsJson OEE 指标字段映射：{running,output,good,reject,cycleTime}
	MetricsJson string `gorm:"column:metrics_json;type:text;comment:OEE指标映射(JSON)"`
}

func (table *Solution) TableName() string {
	return "solution"
}

func (table *Solution) Get() interface{} {
	return *table
}

// TableSolution 方案表名
const TableSolution = "solution"
