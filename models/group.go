package models

import "github.com/beego/beego/v2/client/orm"

// Group 设备分组表
type Group struct {
	Id          int64    `orm:"pk;auto" json:"id"`                                            // 设备分组ID
	Name        string   `orm:"size(255);null" json:"name"`                                   // 分组名称
	Description string   `orm:"size(255);null" json:"description"`                            // 描述
	Project     *Project `orm:"rel(fk);column(project_id);on_delete(cascade)" json:"project"` // 项目ID
	Type        int8     `orm:"null" json:"type"`                                             // 分组类型 0:包含 1: 不包含
	Sort        int64    `orm:"null" json:"sort"`
}

func init() {
	// 注册模型
	orm.RegisterModel(new(Group))
}
