package interfaces

import (
	"iotserver/internal/dtos"
	"iotserver/internal/models"
)

// SysUserDB 系统管理-用户仓储
type SysUserDB interface {
	AddSysUser(u models.User) (models.User, error)
	SysUserById(id string) (models.User, error)
	SysUserByUsername(username string) (models.User, error)
	UpdateSysUser(u models.User) error
	DeleteSysUserByIds(ids []string) error
	SysUserSearch(offset, limit int, req dtos.SysUserSearchRequest) ([]models.User, uint32, error)
	SysUserCountByDeptIds(deptIds []string) (int64, error)
	UpdateSysUserLastLogin(id string, ts int64) error
	ResetSysUserPassword(id, password string) error

	GetUserRoleIds(userId string) ([]string, error)
	SetUserRoles(userId string, roleIds []string) error
	DeleteUserRolesByUserIds(userIds []string) error
	DeleteUserRolesByRoleIds(roleIds []string) error
	SysUserIdsByRoleId(roleId string) ([]string, error)
}

// SysRoleDB 系统管理-角色仓储
type SysRoleDB interface {
	AddSysRole(r models.Role) (models.Role, error)
	SysRoleById(id string) (models.Role, error)
	SysRoleByName(name string) (models.Role, error)
	UpdateSysRole(r models.Role) error
	DeleteSysRoleByIds(ids []string) error
	SysRoleSearch(offset, limit int, req dtos.SysRoleSearchRequest) ([]models.Role, uint32, error)
	SysRoleAll() ([]models.Role, error)
	SysRolesByIds(ids []string) ([]models.Role, error)

	GetRoleMenuIds(roleId string) ([]string, error)
	SetRoleMenus(roleId string, menuIds []string) error
	DeleteRoleMenusByRoleIds(roleIds []string) error
	DeleteRoleMenusByMenuIds(menuIds []string) error
	RoleNamesByUserId(userId string) ([]string, error)
	RoleMenusByUserId(userId string) ([]models.Menu, error)
}

// SysMenuDB 系统管理-菜单仓储
type SysMenuDB interface {
	AddSysMenu(m models.Menu) (models.Menu, error)
	SysMenuById(id string) (models.Menu, error)
	UpdateSysMenu(m models.Menu) error
	DeleteSysMenuByIds(ids []string) error
	SysMenuSearch(offset, limit int, req dtos.SysMenuSearchRequest) ([]models.Menu, uint32, error)
	SysMenuAll() ([]models.Menu, error)
	SysMenusByIds(ids []string) ([]models.Menu, error)
	SysMenuCountByParentIds(parentIds []string) (int64, error)
}

// SysDeptDB 系统管理-部门仓储
type SysDeptDB interface {
	AddSysDept(d models.Dept) (models.Dept, error)
	SysDeptById(id string) (models.Dept, error)
	UpdateSysDept(d models.Dept) error
	DeleteSysDeptByIds(ids []string) error
	SysDeptSearch(offset, limit int, req dtos.SysDeptSearchRequest) ([]models.Dept, uint32, error)
	SysDeptAll() ([]models.Dept, error)
	SysDeptCountByParentIds(parentIds []string) (int64, error)
}

// SysLoginLogDB 系统管理-登入日志仓储
type SysLoginLogDB interface {
	AddSysLoginLog(l models.LoginLog) error
	SysLoginLogSearch(offset, limit int, req dtos.SysLoginLogSearchRequest) ([]models.LoginLog, uint32, error)
	DeleteSysLoginLogByIds(ids []string) error
	ClearSysLoginLog() error
}

// SysConfigDB 系统管理-系统配置仓储
type SysConfigDB interface {
	AddSysConfig(c models.SystemConfig) (models.SystemConfig, error)
	SysConfigById(id string) (models.SystemConfig, error)
	SysConfigByKey(key string) (models.SystemConfig, error)
	UpdateSysConfig(c models.SystemConfig) error
	DeleteSysConfigByIds(ids []string) error
	SysConfigSearch(offset, limit int, req dtos.SysConfigSearchRequest) ([]models.SystemConfig, uint32, error)
	SaveSysConfigValues(items []dtos.SysSetupItem) error
}

// SystemManageDB 系统管理仓储聚合
type SystemManageDB interface {
	SysUserDB
	SysRoleDB
	SysMenuDB
	SysDeptDB
	SysLoginLogDB
	SysConfigDB
	// MigrateSystemTables 自动建表并写入初始化数据
	MigrateSystemTables() error
}
