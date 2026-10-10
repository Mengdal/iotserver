package models

// 登录状态
const (
	LoginStatusSuccess = 1
	LoginStatusFail    = 0
)

// LoginLog 登入日志
type LoginLog struct {
	Id        string `gorm:"column:id;primaryKey;type:string;size:64;comment:主键"`
	Username  string `gorm:"column:username;type:string;size:255;index;comment:登录账号"`
	Ip        string `gorm:"column:ip;type:string;size:64;comment:登录IP"`
	Browser   string `gorm:"column:browser;type:string;size:255;comment:浏览器"`
	Os        string `gorm:"column:os;type:string;size:255;comment:操作系统"`
	Status    int    `gorm:"column:status;type:int;default:1;comment:状态 1成功 0失败"`
	Message   string `gorm:"column:message;type:string;size:500;comment:提示消息"`
	LoginTime int64  `gorm:"column:login_time;index;comment:登录时间"`
}

func (table *LoginLog) Get() interface{} {
	return *table
}

func (table *LoginLog) TableName() string {
	return "sys_login_log"
}
