package models

import "github.com/beego/beego/v2/client/orm"

type Category struct {
	Id           int64  `orm:"auto;pk" json:"id"`
	Scene        string `orm:"null;type(text)" json:"scene"`
	CategoryKey  string `orm:"null;type(text)" json:"categoryKey"`
	CategoryName string `orm:"null;type(text)" json:"categoryName"`
	Created      int64  `orm:"null" json:"created"`
	Modified     int64  `orm:"null" json:"modified"`
}

func init() {
	// 注册模型
	orm.RegisterModel(new(Category))
}
