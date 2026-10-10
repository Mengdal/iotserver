package interfaces

import (
	"context"
	"iotserver/internal/dtos"
)

// UserItf 用户与系统管理（用户/角色/菜单/部门/登入日志/系统配置）
type UserItf interface {
	/*********** 认证 ***********/
	UserLogin(ctx context.Context, req dtos.LoginRequest) (res dtos.LoginResponse, err error)
	InitInfo() (res dtos.InitInfoResponse, err error)
	InitPassword(ctx context.Context, req dtos.InitPasswordRequest) error
	UpdateUserPassword(ctx context.Context, username string, req dtos.UpdatePasswordRequest) error
	OpenApiUserLogin(ctx context.Context, req dtos.LoginRequest) (res *dtos.TokenDetail, err error)
	CreateTokenDetail(userName string) (*dtos.TokenDetail, error)
	InitJwtKey()
	MigrateSystemData() error

	/*********** 站点信息 ***********/
	SiteInfo(ctx context.Context) (dtos.SiteInfoResponse, error)

	/*********** 当前登录用户 ***********/
	CurrentUserProfile(ctx context.Context, username string) (dtos.SysUserProfileResponse, error)
	UpdateCurrentUserProfile(ctx context.Context, username string, req dtos.SysUserProfileUpdateRequest) error
	PermissionsByUsername(username string) ([]string, error)
	IsAdminUsername(username string) bool

	/*********** 用户管理 ***********/
	SysUserSearch(ctx context.Context, req dtos.SysUserSearchRequest) ([]dtos.SysUserResponse, uint32, error)
	SysUserById(ctx context.Context, id string) (dtos.SysUserResponse, error)
	SysUserAdd(ctx context.Context, req dtos.SysUserAddRequest) error
	SysUserUpdate(ctx context.Context, req dtos.SysUserUpdateRequest) error
	SysUserDelete(ctx context.Context, ids []string) error
	SysUserResetPassword(ctx context.Context, id string, req dtos.SysUserResetPwdRequest) error
	SysUserChangeStatus(ctx context.Context, id string, status int) error

	/*********** 角色管理 ***********/
	SysRoleSearch(ctx context.Context, req dtos.SysRoleSearchRequest) ([]dtos.SysRoleResponse, uint32, error)
	SysRoleById(ctx context.Context, id string) (dtos.SysRoleResponse, error)
	SysRoleAdd(ctx context.Context, req dtos.SysRoleAddRequest) error
	SysRoleUpdate(ctx context.Context, req dtos.SysRoleUpdateRequest) error
	SysRoleDelete(ctx context.Context, ids []string) error
	SysRoleAll(ctx context.Context) ([]dtos.SysRoleResponse, error)
	SysRoleMenus(ctx context.Context, roleId string) ([]string, error)
	SysRoleSetMenus(ctx context.Context, req dtos.SysRoleMenuRequest) error

	/*********** 菜单管理 ***********/
	SysMenuSearch(ctx context.Context, req dtos.SysMenuSearchRequest) ([]dtos.SysMenuResponse, uint32, error)
	SysMenuTree(ctx context.Context, req dtos.SysMenuSearchRequest) ([]dtos.SysMenuResponse, error)
	SysMenuById(ctx context.Context, id string) (dtos.SysMenuResponse, error)
	SysMenuAdd(ctx context.Context, req dtos.SysMenuAddRequest) error
	SysMenuUpdate(ctx context.Context, req dtos.SysMenuUpdateRequest) error
	SysMenuDelete(ctx context.Context, ids []string) error
	MenuRoutesByUsername(username string) ([]dtos.SysMenuResponse, error)

	/*********** 部门管理 ***********/
	SysDeptSearch(ctx context.Context, req dtos.SysDeptSearchRequest) ([]dtos.SysDeptResponse, uint32, error)
	SysDeptTree(ctx context.Context, req dtos.SysDeptSearchRequest) ([]dtos.SysDeptResponse, error)
	SysDeptById(ctx context.Context, id string) (dtos.SysDeptResponse, error)
	SysDeptAdd(ctx context.Context, req dtos.SysDeptAddRequest) error
	SysDeptUpdate(ctx context.Context, req dtos.SysDeptUpdateRequest) error
	SysDeptDelete(ctx context.Context, ids []string) error

	/*********** 登入日志 ***********/
	SysLoginLogSearch(ctx context.Context, req dtos.SysLoginLogSearchRequest) ([]dtos.SysLoginLogResponse, uint32, error)
	SysLoginLogDelete(ctx context.Context, ids []string) error
	SysLoginLogClear(ctx context.Context) error

	/*********** 系统配置 ***********/
	SysConfigSearch(ctx context.Context, req dtos.SysConfigSearchRequest) ([]dtos.SysConfigResponse, uint32, error)
	SysConfigById(ctx context.Context, id string) (dtos.SysConfigResponse, error)
	SysConfigAdd(ctx context.Context, req dtos.SysConfigAddRequest) error
	SysConfigUpdate(ctx context.Context, req dtos.SysConfigUpdateRequest) error
	SysConfigDelete(ctx context.Context, ids []string) error
	SysSetup(ctx context.Context) ([]dtos.SysConfigResponse, error)
	SysSetupSave(ctx context.Context, req dtos.SysSetupRequest) error
}
