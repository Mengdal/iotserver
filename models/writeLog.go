package models

import (
	"github.com/beego/beego/v2/client/orm"
	"time"
)

// WriteLog 记录操作日志
type WriteLog struct {
	Seq        string      `orm:"size(255);index;pk" json:"seq"`
	UserId     int64       `orm:"null"`
	Sn         string      `orm:"size(255)" json:"sn"`
	Dn         string      `orm:"size(255)" json:"dn"`
	Tag        string      `orm:"size(255)" json:"tag"`
	Val        string      `orm:"size(255)" json:"val"`
	Status     string      `orm:"size(255)" json:"status"`
	Channel    string      `orm:"size(255)" json:"channel"`
	RespTime   int64       `orm:"null" json:"respTime"`
	Created    int64       `orm:"null" json:"created"`
	Department *Department `orm:"rel(fk);on_delete(cascade);null" json:"-"`
}

// 初始化函数
func init() {
	orm.RegisterModel(new(WriteLog))
}

// BeforeInsert 实现时间戳自动赋值
func (w *WriteLog) BeforeInsert() error {
	now := time.Now().Unix()
	if w.Created == 0 {
		w.Created = now
	}
	return nil
}
func (p *WriteLog) BeforeUpdate() error {
	p.RespTime = time.Now().Unix()
	return nil
}
