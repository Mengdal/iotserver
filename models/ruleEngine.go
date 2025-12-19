package models

import (
	"github.com/beego/beego/v2/client/orm"
	"time"
)

type RuleEngine struct {
	Id           int64         `orm:"auto;pk" json:"id"`
	Created      int64         `orm:"null" json:"created"`
	Modified     int64         `orm:"null" json:"modified"`
	Name         string        `orm:"size(255);unique" json:"name"`
	Description  string        `orm:"null;type(text)" json:"description"`
	Status       string        `orm:"null;type(text)" json:"status"`
	Filter       string        `orm:"null;type(text)" json:"filter"`
	Department   *Department   `orm:"rel(fk);on_delete(cascade);null" json:"-"`
	DataResource *DataResource `orm:"rel(fk);column(data_resource_id);on_delete(cascade);on_update(do_nothing);null" json:"data_resource,omitempty"`
}

func init() {
	// 注册模型
	orm.RegisterModel(new(RuleEngine))
}

// BeforeInsert 插入前钩子
func (a *RuleEngine) BeforeInsert() error {
	now := time.Now().Unix()
	if a.Created == 0 {
		a.Created = now
	}
	a.Modified = now
	return nil
}

// BeforeUpdate 更新前钩子
func (a *RuleEngine) BeforeUpdate() error {
	a.Modified = time.Now().Unix()
	return nil
}
