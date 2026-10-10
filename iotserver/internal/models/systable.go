package models

// 系统管理相关表名常量
// 注意：TableName() 均为指针接收者，不能用 models.X{}.TableName() 形式取值
const (
	TableUser         = "user"
	TableUserRole     = "sys_user_role"
	TableRole         = "sys_role"
	TableRoleMenu     = "sys_role_menu"
	TableMenu         = "sys_menu"
	TableDept         = "sys_dept"
	TableLoginLog     = "sys_login_log"
	TableSystemConfig = "sys_config"
)
