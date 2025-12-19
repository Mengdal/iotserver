package models

import (
	"database/sql"
	"github.com/beego/beego/v2/client/orm"
)

type Role struct {
	Id           int64         `orm:"auto;pk" json:"id"`
	DepartmentId sql.NullInt64 `orm:"null" json:"-"`
	Name         string        `orm:"size(255)" json:"name"`
	Description  string        `orm:"null;size(255)" json:"description"`
	Permission   string        `orm:"null;type(text)" json:"permission"`
}

func init() {
	orm.RegisterModel(new(Role))
}
