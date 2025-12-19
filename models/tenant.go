package models

import (
	"github.com/beego/beego/v2/client/orm"
	"time"
)

// Tenant 租户表
type Tenant struct {
	Id             int64      `orm:"pk;auto" json:"id"`                                               // UUID
	User           *User      `orm:"rel(fk);column(user_id);null;on_delete(cascade)" json:"-"`        // 租户拥有者
	Name           string     `orm:"size(255);null" json:"name"`                                      // 名称
	Enable         bool       `orm:"null" json:"enable"`                                              // 是否激活（0：失效，1：激活）
	Address        string     `orm:"size(255);null" json:"address"`                                   // 地址
	Area           string     `orm:"size(255);null" json:"area"`                                      // 区域面积
	DeviceNum      int        `orm:"null;default(0)" json:"deviceNum"`                                // 设备数量
	Images         string     `orm:"size(255);null;default(/images/image_default.png)" json:"images"` // 首页大图
	Logo           string     `orm:"size(255);null;default(/images/logo.png)" json:"logo"`            // 企业LOGO
	PersonNum      string     `orm:"size(255);null" json:"personNum"`                                 // 企业人数
	Ranges         string     `orm:"size(255);null;default(-1d)" json:"ranges"`                       // 日期范围
	ActiveTime     time.Time  `orm:"type(timestamp);auto_now_add" json:"activeTime"`                  // 激活时间
	Icon           string     `orm:"size(255);null;default(/images/icon_default.png)" json:"icon"`    // 企业图标
	IndexType      string     `orm:"size(255);null;default(/home)" json:"indexType"`                  // 首页类型
	PeerProjectId  int64      `orm:"column(peer_project_id);null" json:"peerProjectId"`               // 租户ID
	ExpirationTime *time.Time `orm:"type(timestamp);null" json:"expirationTime"`
}

func init() {
	// 注册模型
	orm.RegisterModel(new(Tenant))
}
