package models

// 菜单类型
const (
	MenuTypeDir    = "M" // 目录
	MenuTypeMenu   = "C" // 菜单
	MenuTypeButton = "F" // 按钮
)

// Menu 菜单（同时承载前端菜单树与按钮权限）
type Menu struct {
	Timestamps `gorm:"embedded"`
	Id         string `gorm:"column:id;primaryKey;type:string;size:64;comment:主键"`
	ParentId   string `gorm:"column:parent_id;type:string;size:64;index;comment:父菜单ID"`
	Name       string `gorm:"column:name;type:string;size:255;comment:菜单名称"`
	Path       string `gorm:"column:path;type:string;size:255;comment:路由地址"`
	Component  string `gorm:"column:component;type:string;size:255;comment:组件路径"`
	Icon       string `gorm:"column:icon;type:string;size:128;comment:菜单图标"`
	MenuType   string `gorm:"column:menu_type;type:string;size:8;default:C;comment:菜单类型 M目录 C菜单 F按钮"`
	Perms      string `gorm:"column:perms;type:string;size:255;comment:权限标识"`
	Sort       int    `gorm:"column:sort;type:int;default:0;comment:显示顺序"`
	Visible    int    `gorm:"column:visible;type:int;default:1;comment:是否显示 1显示 0隐藏"`
	Status     int    `gorm:"column:status;type:int;default:1;comment:状态 1正常 0停用"`
	IsFrame    int    `gorm:"column:is_frame;type:int;default:1;comment:是否外链 1否 0是"`
	Remark     string `gorm:"column:remark;type:string;size:500;comment:备注"`
}

func (table *Menu) Get() interface{} {
	return *table
}

func (table *Menu) TableName() string {
	return "sys_menu"
}
