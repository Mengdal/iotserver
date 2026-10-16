package models

// VizBoard 画布看板。
//
// 历史实现把看板登记在单个 JSON 文件（.viz-boards.json）里，多租户下无法隔离、
// 多实例也无法共享，因此改为落库：按租户隔离，字段与文件版一一对应，
// 前端接口契约保持不变。
type VizBoard struct {
	Timestamps  `gorm:"embedded"`
	TenantField `gorm:"embedded"`
	Id          string `gorm:"column:id;primaryKey;type:string;size:64;comment:看板ID"`
	Name        string `gorm:"column:name;type:string;size:128;comment:看板名称"`
	Icon        string `gorm:"column:icon;type:string;size:64;comment:图标"`
	// Canvas 画布内容（布局 + 组件）JSON
	Canvas string `gorm:"column:canvas;type:text;comment:画布(JSON)"`
	// IsDefault 1=整页默认看板（租户内唯一）
	IsDefault int `gorm:"column:is_default;type:int;default:0;comment:整页默认 1是 0否"`
	// IsHome 1=首页卡片看板（租户内唯一）
	IsHome int `gorm:"column:is_home;type:int;default:0;comment:首页默认 1是 0否"`
	// UpdateAt 前端可见的最后更新时间（毫秒）
	UpdateAt int64 `gorm:"column:update_at;comment:最后更新时间"`
}

func (table *VizBoard) TableName() string {
	return "viz_board"
}

func (table *VizBoard) Get() interface{} {
	return *table
}

// TableVizBoard 看板表名
const TableVizBoard = "viz_board"
