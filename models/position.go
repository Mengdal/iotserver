package models

import "github.com/beego/beego/v2/client/orm"

// Position 位置表
type Position struct {
	Id             int64       `orm:"pk;auto" json:"id"`                                                       // 位置ID
	FullName       string      `orm:"size(255);null" json:"fullName"`                                          // 位置全称
	Name           string      `orm:"size(255);null" json:"name"`                                              // 位置名称
	ParentPosition *Position   `orm:"rel(fk);column(parent_position);null"`                                    // 父节点
	Department     *Department `orm:"rel(fk);column(department_id);on_delete(cascade);null" json:"department"` // 部门ID
}

func init() {
	// 注册模型
	orm.RegisterModel(new(Position))
}
