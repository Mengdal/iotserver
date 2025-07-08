package models

import (
	"github.com/beego/beego/v2/client/orm"
	"time"
)

type User struct {
	Id         int64     `orm:"auto;pk" json:"id"`
	Email      string    `orm:"null;size(255)" json:"email"`
	Password   string    `orm:"size(255);not null" valid:"Required;MinSize(6)" json:"password"`
	Username   string    `orm:"size(255);unique" json:"username"`
	ParentId   *int64    `orm:"null" json:"parent_id"`
	RoleId     *int64    `orm:"null" json:"role_id"`
	WebToken   string    `orm:"null;size(255)" json:"web_token"`
	CreateTime time.Time `orm:"auto_now_add;type(datetime)" json:"create_time"`
}

func init() {
	orm.RegisterModel(new(User))
}
