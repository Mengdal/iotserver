package models

import (
	"database/sql/driver"
)

// TableDataResource 高级能力-资源管理表名
const TableDataResource = "data_resource"

// DataResourceOption 资源连接参数。
// 不同资源类型字段不同（HTTP: url/method/headers…；MQTT: server/topic/…），
// 因此按 JSON 文本存库，出参仍是对象，前端编辑弹窗可直接读取。
type DataResourceOption map[string]interface{}

func (o DataResourceOption) Value() (driver.Value, error) {
	return GormValueWrap(o)
}

func (o *DataResourceOption) Scan(value interface{}) error {
	return GormScanWrap(value, o)
}

// DataResource 数据资源实例（HTTP推送 / MQTT / Kafka / InfluxDB / TDengine）
type DataResource struct {
	Id string `json:"id" gorm:"column:id;primaryKey;type:string;size:64;comment:主键"`
	TenantField
	Name string `json:"name" gorm:"column:name;type:string;size:64;index;comment:实例名称"`
	Type string `json:"type" gorm:"column:type;type:string;size:64;index;comment:资源类型"`
	// 列名用 option_json：option 是 MySQL 保留字
	Option DataResourceOption `json:"option" gorm:"column:option_json;type:text;comment:连接参数(JSON)"`
	// Health 最近一次验证结果；nil 表示尚未验证（前端显示 '-'）
	Health *bool `json:"health" gorm:"column:health;comment:最近一次验证结果"`
	Timestamps
}

func (d *DataResource) TableName() string {
	return TableDataResource
}
