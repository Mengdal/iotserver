package models

import (
	"github.com/beego/beego/v2/client/orm"
	"time"
)

type Product struct {
	Created         int64       `orm:"column(created);null" json:"created"`
	Modified        int64       `orm:"column(modified);null" json:"modified"`
	Id              int64       `orm:"pk;column(id)" json:"id"`
	Name            string      `orm:"column(name);null;size(255)" json:"name"`
	Key             string      `orm:"column(key);null;size(255)" json:"key,omitempty"`
	CloudProductId  string      `orm:"column(cloud_product_id);null;size(255)" json:"cloudProductId,omitempty"`
	CloudInstanceId string      `orm:"column(cloud_instance_id);null;size(255)" json:"cloudInstanceId,omitempty"`
	Platform        string      `orm:"column(platform);null;size(255)" json:"platform,omitempty"`
	Protocol        string      `orm:"column(protocol);null;size(255)" json:"protocol,omitempty"`
	NodeType        string      `orm:"column(node_type);null;size(255)" json:"nodeType,omitempty"`
	NetType         string      `orm:"column(net_type);null;size(255)" json:"netType,omitempty"`
	DataFormat      string      `orm:"column(data_format);null;size(255)" json:"dataFormat,omitempty"`
	LastSyncTime    int64       `orm:"column(last_sync_time);null" json:"lastSyncTime,omitempty"`
	Factory         string      `orm:"column(factory);null;size(255)" json:"factory,omitempty"`
	Description     string      `orm:"column(description);null;size(255)" json:"description"`
	Status          int         `orm:"column(status);null;default(0)" json:"status"`
	Extra           string      `orm:"column(extra);null;size(255)" json:"extra,omitempty"`
	Department      *Department `orm:"rel(fk);on_delete(cascade);null" json:"-"`
	CategoryId      int64       `orm:"default(0);" json:"categoryId"`

	Properties []*Properties `orm:"reverse(many)" json:"properties"` // 一对多关联
	Events     []*Events     `orm:"reverse(many)" json:"events"`
	Actions    []*Actions    `orm:"reverse(many)" json:"actions"`
}

func (p *Product) BeforeInsert() error {
	now := time.Now().Unix()
	if p.Created == 0 {
		p.Created = now
	}
	p.Modified = now
	return nil
}

func (p *Product) BeforeUpdate() error {
	p.Modified = time.Now().Unix()
	return nil
}

func init() {
	orm.RegisterModel(new(Product))
	orm.RegisterModel(new(Properties))
	orm.RegisterModel(new(Events))
	orm.RegisterModel(new(Actions))
}
