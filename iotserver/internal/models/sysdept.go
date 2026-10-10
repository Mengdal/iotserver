package models

// Dept 部门
type Dept struct {
	Timestamps `gorm:"embedded"`
	Id         string `gorm:"column:id;primaryKey;type:string;size:64;comment:主键"`
	ParentId   string `gorm:"column:parent_id;type:string;size:64;index;comment:父部门ID"`
	Ancestors  string `gorm:"column:ancestors;type:string;size:500;comment:祖级列表"`
	Name       string `gorm:"column:name;type:string;size:255;comment:部门名称"`
	Sort       int    `gorm:"column:sort;type:int;default:0;comment:显示顺序"`
	Leader     string `gorm:"column:leader;type:string;size:128;comment:负责人"`
	Phone      string `gorm:"column:phone;type:string;size:64;comment:联系电话"`
	Email      string `gorm:"column:email;type:string;size:255;comment:邮箱"`
	Status     int    `gorm:"column:status;type:int;default:1;comment:状态 1正常 0停用"`
}

func (table *Dept) Get() interface{} {
	return *table
}

func (table *Dept) TableName() string {
	return "sys_dept"
}
