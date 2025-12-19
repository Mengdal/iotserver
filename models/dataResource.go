package models

import (
	"github.com/beego/beego/v2/client/orm"
	"time"
)

type DataResource struct {
	Id         int64       `orm:"auto;pk" json:"id"`
	Created    int64       `orm:"null" json:"created"`
	Modified   int64       `orm:"null" json:"modified"`
	Name       string      `orm:"type(text)" json:"name"`
	Type       string      `orm:"null;type(text)" json:"type"`
	Health     string      `orm:"null;type(text)" json:"health"`
	Option     string      `orm:"null;type(text)" json:"option"`
	Department *Department `orm:"rel(fk);on_delete(cascade);null" json:"-"`
}

func init() {
	// 注册模型
	orm.RegisterModel(new(DataResource))
}

// BeforeInsert 插入前钩子
func (a *DataResource) BeforeInsert() error {
	now := time.Now().Unix()
	if a.Created == 0 {
		a.Created = now
	}
	a.Modified = now
	return nil
}

// BeforeUpdate 更新前钩子
func (a *DataResource) BeforeUpdate() error {
	a.Modified = time.Now().Unix()
	return nil
}
