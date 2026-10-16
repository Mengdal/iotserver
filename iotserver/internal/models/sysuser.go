package models

// User 系统用户
// 兼容历史结构：username/password/lang/gateway_key/open_api_key 保留原字段名
type User struct {
	Timestamps  `gorm:"embedded"`
	TenantField `gorm:"embedded"`
	Id          string `gorm:"column:id;primaryKey;type:string;size:64;comment:主键"`
	Username    string `gorm:"column:username;uniqueIndex;type:string;size:255;comment:登录账号"`
	Password    string `gorm:"column:password;type:string;size:255;comment:密码"`
	Nickname    string `gorm:"column:nickname;type:string;size:255;comment:用户昵称"`
	Email       string `gorm:"column:email;type:string;size:255;comment:邮箱"`
	Phone       string `gorm:"column:phone;type:string;size:64;comment:手机号"`
	DeptId      string `gorm:"column:dept_id;type:string;size:64;index;comment:部门ID"`
	Status      int    `gorm:"column:status;type:int;default:1;comment:状态 1正常 0停用"`
	Lang        string `gorm:"column:lang;type:string;size:50;comment:语言"`
	Remark      string `gorm:"column:remark;type:string;size:500;comment:备注"`
	LastLogin   int64  `gorm:"column:last_login;comment:最后登录时间"`
	GatewayKey  string `gorm:"column:gateway_key;type:string;size:255;comment:密钥"`
	OpenAPIKey  string `gorm:"column:open_api_key;type:string;size:255;comment:密钥"`
}

func (table *User) Get() interface{} {
	return *table
}

func (table *User) TableName() string {
	return "user"
}

// UserRole 用户-角色关联
type UserRole struct {
	UserId string `gorm:"column:user_id;type:string;size:64;primaryKey;comment:用户ID"`
	RoleId string `gorm:"column:role_id;type:string;size:64;primaryKey;comment:角色ID"`
}

func (table *UserRole) Get() interface{} {
	return *table
}

func (table *UserRole) TableName() string {
	return "sys_user_role"
}
