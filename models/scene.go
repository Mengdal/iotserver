package models

import (
	"github.com/beego/beego/v2/client/orm"
	"time"
)

// Scene 场景联动主表
type Scene struct {
	Id          int64  `orm:"pk;auto" json:"id"` // 场景ID
	Name        string `orm:"type(text)" json:"name"`
	Description string `orm:"type(text);null" json:"description"`
	Status      string `orm:"type(text);null" json:"status"`
	Created     int64  `orm:"null" json:"created"`
	Modified    int64  `orm:"null" json:"modified"`
	Condition   string `orm:"type(text);null" json:"condition"` // 场景触发条件
	Action      string `orm:"type(text);null" json:"action"`    // 场景动作
}

func init() {
	// 注册模型
	orm.RegisterModel(new(Scene))
}

// BeforeUpdate 更新前钩子
func (a *Scene) BeforeUpdate() error {
	a.Modified = time.Now().Unix()
	return nil
}

// BeforeInsert 插入前钩子
func (a *Scene) BeforeInsert() error {
	now := time.Now().Unix()
	if a.Created == 0 {
		a.Created = now
	}
	a.Modified = now
	return nil
}
