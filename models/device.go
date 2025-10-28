package models

import (
	"github.com/beego/beego/v2/client/orm"
	"time"
)

// Device 设备主表
type Device struct {
	Id          int64    `orm:"pk;auto" json:"id"` // 设备ID
	Name        string   `orm:"size(255);unique" json:"name"`
	Description string   `orm:"type(text);null" json:"description"`
	Status      string   `orm:"type(text);null" json:"status"`
	Created     int64    `orm:"null" json:"created"`
	Modified    int64    `orm:"null" json:"modified"`
	CategoryKey string   `orm:"type(text);null" json:"productKey"`                      // 产品模型，超级表
	Product     *Product `orm:"rel(fk);column(product_id);on_delete(cascade)" json:"-"` // 产品ID
}

func init() {
	// 注册模型
	orm.RegisterModel(new(Device))
}

// BeforeUpdate 更新前钩子
func (a *Device) BeforeUpdate() error {
	a.Modified = time.Now().Unix()
	return nil
}

// BeforeInsert 插入前钩子
func (a *Device) BeforeInsert() error {
	now := time.Now().Unix()
	if a.Created == 0 {
		a.Created = now
	}
	a.Modified = now
	return nil
}
