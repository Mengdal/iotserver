package models

// 数据权限范围
const (
	DataScopeAll       = "1" // 全部数据
	DataScopeDept      = "2" // 本部门数据
	DataScopeDeptChild = "3" // 本部门及以下数据
	DataScopeSelf      = "4" // 仅本人数据
)

// Role 角色。name 的全局唯一索引在迁移中改为 (tenant_id, name) 唯一，
// 使不同租户可以各自拥有「管理员」等同名角色。
type Role struct {
	Timestamps  `gorm:"embedded"`
	TenantField `gorm:"embedded"`
	Id          string `gorm:"column:id;primaryKey;type:string;size:64;comment:主键"`
	Name        string `gorm:"column:name;type:string;size:255;comment:角色名称"`
	RoleKey     string `gorm:"column:role_key;type:string;size:128;comment:角色权限字符串"`
	Sort        int    `gorm:"column:sort;type:int;default:0;comment:显示顺序"`
	DataScope   string `gorm:"column:data_scope;type:string;size:8;default:1;comment:数据范围"`
	Status      int    `gorm:"column:status;type:int;default:1;comment:状态 1正常 0停用"`
	Remark      string `gorm:"column:remark;type:string;size:500;comment:备注"`
}

func (table *Role) Get() interface{} {
	return *table
}

func (table *Role) TableName() string {
	return "sys_role"
}

// RoleMenu 角色-菜单关联
type RoleMenu struct {
	RoleId string `gorm:"column:role_id;type:string;size:64;primaryKey;comment:角色ID"`
	MenuId string `gorm:"column:menu_id;type:string;size:64;primaryKey;comment:菜单ID"`
}

func (table *RoleMenu) Get() interface{} {
	return *table
}

func (table *RoleMenu) TableName() string {
	return "sys_role_menu"
}
