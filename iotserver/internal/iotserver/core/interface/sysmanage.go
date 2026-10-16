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
	// SysRoleByNameInTenant / SysRoleByKeyInTenant 显式按租户查重。
	// 说明：角色是租户隔离表，但"平台视图"下（Platform 作用域）插件不过滤，
	// 用 WithCtx 查重会跨租户误判，因此这里显式带上 tenant_id（tenantId="" 表示平台级角色）。
	SysRoleByNameInTenant(name, tenantId string) (models.Role, error)
	SysRoleByKeyInTenant(roleKey, tenantId string) (models.Role, error)
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
	// RoleMenusByUserId 走原生 SQL，绕过了 GORM 租户插件，
	// 因此菜单作用域（tenant_id IN ('', tenantId)）必须手写进 SQL。
	RoleMenusByUserId(userId, tenantId string) ([]models.Menu, error)
}

// SysMenuDB 系统管理-菜单仓储
//
// 菜单是「全局默认 + 租户自建」表，不登记进 tenantScopedModels，
// 所有读取都显式带 tenantId 参数（空 = 平台视图，只看 tenant_id=”）。
type SysMenuDB interface {
	AddSysMenu(m models.Menu) (models.Menu, error)
	SysMenuById(id string) (models.Menu, error)
	UpdateSysMenu(m models.Menu) error
	DeleteSysMenuByIds(ids []string) error
	SysMenuSearch(offset, limit int, tenantId string, req dtos.SysMenuSearchRequest) ([]models.Menu, uint32, error)
	// SysMenuAll 按菜单作用域返回全量：tenantId="" 只返回平台菜单；
	// 非空返回「平台菜单 + 该租户自建菜单」。
	SysMenuAll(tenantId string) ([]models.Menu, error)
	SysMenusByIds(ids []string) ([]models.Menu, error)
	// SysMenusByTenant 返回某个租户的自建菜单（用于平台重置租户菜单定制）
	SysMenusByTenant(tenantId string) ([]models.Menu, error)
	SysMenuCountByParentIds(parentIds []string) (int64, error)
	// SysMenuCountTenantChildrenByParentIds 统计挂在给定菜单下的「租户自建」子菜单数量
	// （删除平台菜单前的孤儿检查）
	SysMenuCountTenantChildrenByParentIds(parentIds []string) (int64, error)

	// ===== 租户菜单展示层覆盖 tenant_menu_pref =====
	SysMenuPrefsByTenant(tenantId string) ([]models.TenantMenuPref, error)
	UpsertSysMenuPref(p models.TenantMenuPref) (models.TenantMenuPref, error)
	DeleteSysMenuPref(tenantId, menuId string) error
	DeleteSysMenuPrefsByTenant(tenantId string) (int64, error)
	DeleteSysMenuPrefsByMenuIds(menuIds []string) error
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

// SysAuditLogDB 系统管理-审查日志仓储（登录 + 操作，单表 sys_audit_log）
type SysAuditLogDB interface {
	AddSysAuditLog(l models.AuditLog) error
	// AddSysAuditLogs 批量写入（同一租户作用域内的记录合并一次插入）
	AddSysAuditLogs(list []models.AuditLog) error
	SysAuditLogSearch(offset, limit int, req dtos.SysAuditLogSearchRequest) ([]models.AuditLog, uint32, error)
	DeleteSysAuditLogByIds(ids []string) error
	ClearSysAuditLog() error
	// DeleteSysAuditLogBefore 删除 created < ts 的记录（平台作用域跨租户清理）
	DeleteSysAuditLogBefore(ts int64) (int64, error)
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
	SysAuditLogDB
	SysConfigDB
	// MigrateSystemTables 自动建表并写入初始化数据
	MigrateSystemTables() error
}
