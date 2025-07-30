package models

import "github.com/beego/beego/v2/client/orm"

type ThingModel struct {
	Id             int64  `orm:"auto;pk" json:"id"`
	CategoryKey    string `orm:"null;type(text)" json:"categoryKey"`
	CategoryName   string `orm:"null;type(text)" json:"categoryName"`
	ThingModelJson string `orm:"null;type(text)" json:"thingModelJson"`
	Created        int64  `orm:"null" json:"created"`
	Modified       int64  `orm:"null" json:"modified"`
}

func init() {
	// 注册模型
	orm.RegisterModel(new(ThingModel))
}
