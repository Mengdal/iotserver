package models

import (
	"github.com/beego/beego/v2/client/orm"
	"time"
)

// Device 设备主表
type Device struct {
	Id          int64       `orm:"pk;auto" json:"id"` // 设备ID
	Name        string      `orm:"size(255);unique" json:"name"`
	Description string      `orm:"type(text);null" json:"description"`
	Status      string      `orm:"type(text);null" json:"status"`
	Created     int64       `orm:"null" json:"created"`
	Modified    int64       `orm:"null" json:"modified"`
	GWSN        string      `orm:"type(text);column(sn);null" json:"GWSN"`
	LastOnline  string      `orm:"type(text);null" json:"lastOnline"`
	CategoryKey string      `orm:"type(text);null" json:"productKey"`                               // 产品模型，超级表
	Product     *Product    `orm:"rel(fk);column(product_id);on_delete(set_null);null" json:"-"`    // 产品
	Position    *Position   `orm:"rel(fk);column(position_id);on_delete(set_null);null" json:"-"`   // 位置ID
	Group       *Group      `orm:"rel(fk);column(group_id);on_delete(set_null);null" json:"-"`      // 分组ID
	Department  *Department `orm:"rel(fk);column(department_id);on_delete(set_null);null" json:"-"` // 部门ID
	Tenant      int64       `orm:"column(tenant_id);null" json:"tenantId"`                          // 租户ID

	ProjectId    int64  `orm:"-" json:"project_id"`    // 项目Id
	ProductId    int64  `orm:"-" json:"product_id"`    // 产品Id
	ProductName  string `orm:"-" json:"productName"`   // 产品名称
	PositionId   int64  `orm:"-" json:"position_id"`   // 位置Id
	PositionName string `orm:"-" json:"position_name"` // 位置Name
	GroupId      int64  `orm:"-" json:"group_id"`      // 标签Id
	GroupName    string `orm:"-" json:"group_name"`    // 标签Name
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
