package models

import "github.com/beego/beego/v2/client/orm"

type Menu struct {
	Id             int64  `orm:"auto;pk"`
	Name           string `orm:"size(255)"`
	Meta           string `orm:"type(text)"`
	Component      string `orm:"null;size(255)"`
	ParentId       *int64 `orm:"null"`
	Status         int    `orm:"default(0)"`
	Path           string `orm:"null;size(255)"`
	Redirect       string `orm:"null;size(255)"`
	Type           int    `orm:"null"`
	PermissionList string `orm:"null;type(text)"`
}

func init() {
	orm.RegisterModel(new(Menu))
}
