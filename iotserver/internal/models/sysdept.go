package models

// 组织节点类型：租户内的层级为 公司 → 工厂 → 车间 → 产线
const (
	DeptTypeCompany = 1
	DeptTypeFactory = 2
	DeptTypeShop    = 3
	DeptTypeLine    = 4
)

// Dept 部门/组织节点
type Dept struct {
	Timestamps  `gorm:"embedded"`
	TenantField `gorm:"embedded"`
	Id          string `gorm:"column:id;primaryKey;type:string;size:64;comment:主键"`
	ParentId    string `gorm:"column:parent_id;type:string;size:64;index;comment:父部门ID"`
	Ancestors   string `gorm:"column:ancestors;type:string;size:500;comment:祖级列表"`
	Name        string `gorm:"column:name;type:string;size:255;comment:部门名称"`
	DeptType    int    `gorm:"column:dept_type;type:int;default:1;comment:节点类型 1公司 2工厂 3车间 4产线"`
	Sort        int    `gorm:"column:sort;type:int;default:0;comment:显示顺序"`
	Leader      string `gorm:"column:leader;type:string;size:128;comment:负责人"`
	Phone       string `gorm:"column:phone;type:string;size:64;comment:联系电话"`
	Email       string `gorm:"column:email;type:string;size:255;comment:邮箱"`
	Status      int    `gorm:"column:status;type:int;default:1;comment:状态 1正常 0停用"`
}

func (table *Dept) Get() interface{} {
	return *table
}

func (table *Dept) TableName() string {
	return "sys_dept"
}
