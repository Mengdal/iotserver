package models

// 审查日志类型
const (
	// AuditTypeLogin 登录记录
	AuditTypeLogin = "login"
	// AuditTypeOperation 操作记录
	AuditTypeOperation = "operation"
)

// 审查日志状态（沿用历史命名，登录与操作共用）
const (
	LoginStatusSuccess = 1
	LoginStatusFail    = 0
)

// 审查日志常用模块/动作（登录链路固定值）
const (
	AuditModuleAuth  = "认证"
	AuditActionLogin = "登录"
)

// AuditLog 审查日志：单表 + 类型字段（login / operation）。
//
// 由历史表 sys_login_log 改名而来（不建新表、不搬数据），
// 登录记录同时写 login_time 兼容旧字段，created 才是统一的事件时间。
type AuditLog struct {
	TenantField `gorm:"embedded"`
	Timestamps  `gorm:"embedded"`
	Id          string `gorm:"column:id;primaryKey;type:string;size:64;comment:主键"`
	Type        string `gorm:"column:type;type:string;size:20;index;comment:类型 login/operation"`
	Username    string `gorm:"column:username;type:string;size:255;index;comment:登录账号/操作人"`
	Module      string `gorm:"column:module;type:string;size:64;comment:模块"`
	Action      string `gorm:"column:action;type:string;size:64;comment:动作"`
	Target      string `gorm:"column:target;type:string;size:255;comment:操作对象"`
	Method      string `gorm:"column:method;type:string;size:16;comment:HTTP方法"`
	Path        string `gorm:"column:path;type:string;size:500;comment:HTTP路径"`
	Ip          string `gorm:"column:ip;type:string;size:64;comment:IP"`
	Browser     string `gorm:"column:browser;type:string;size:255;comment:浏览器"`
	Os          string `gorm:"column:os;type:string;size:255;comment:操作系统"`
	// 注意：status 不能带 default 标签 —— GORM 会跳过带默认值的零值字段，
	// 导致失败记录（status=0）被数据库默认值改写成 1（历史登入日志的缺陷）。
	Status     int    `gorm:"column:status;type:int;index;comment:状态 1成功 0失败"`
	ErrorCode  string `gorm:"column:error_code;type:string;size:32;comment:业务错误码"`
	Message    string `gorm:"column:message;type:string;size:500;comment:提示消息"`
	Params     string `gorm:"column:params;type:string;size:2048;comment:操作参数(脱敏+截断2KB)"`
	DurationMs int64  `gorm:"column:duration_ms;comment:耗时(毫秒)"`
	LoginTime  int64  `gorm:"column:login_time;index;comment:登录时间(兼容登录记录)"`
}

func (table *AuditLog) Get() interface{} {
	return *table
}

func (table *AuditLog) TableName() string {
	return TableAuditLog
}
