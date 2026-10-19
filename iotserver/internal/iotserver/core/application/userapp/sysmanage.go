package userapp

import (
	"context"
	"fmt"
	"strings"

	"golang.org/x/crypto/bcrypt"
	"iotserver/internal/dtos"
	"iotserver/internal/iotserver/core/application/tenantquota"
	"iotserver/internal/models"
	"iotserver/internal/pkg/errort"
	"iotserver/internal/pkg/seed"
	"iotserver/internal/pkg/tenant"
	"iotserver/internal/pkg/utils"
)

/******************************* 用户管理 *******************************/

func (uapp *userApp) SysUserSearch(ctx context.Context, req dtos.SysUserSearchRequest) ([]dtos.SysUserResponse, uint32, error) {
	offset, limit := pageParams(req.Page, req.PageSize, req.IsAll)
	list, total, err := uapp.dbClient.WithCtx(ctx).SysUserSearch(offset, limit, req)
	if err != nil {
		return nil, 0, err
	}
	result := make([]dtos.SysUserResponse, 0, len(list))
	for _, item := range list {
		result = append(result, uapp.toSysUserResponse(item))
	}
	return result, total, nil
}

func (uapp *userApp) SysUserById(ctx context.Context, id string) (dtos.SysUserResponse, error) {
	user, err := uapp.dbClient.WithCtx(ctx).SysUserById(id)
	if err != nil {
		return dtos.SysUserResponse{}, err
	}
	return uapp.toSysUserResponse(user), nil
}

func (uapp *userApp) SysUserAdd(ctx context.Context, req dtos.SysUserAddRequest) error {
	// 配额校验：目标租户的用户数 + 1 是否超过 max_users（0=不限；平台视图不校验）
	if err := tenantquota.Ensure(ctx, uapp.dbClient, uapp.lc, tenantquota.Users); err != nil {
		return err
	}
	if err := uapp.validatePassword(req.Password); err != nil {
		return err
	}
	if _, err := uapp.dbClient.WithCtx(ctx).GetUserByUserName(req.Username); err == nil {
		return errort.NewCommonEdgeX(errort.DefaultNameRepeat, fmt.Sprintf("username %s exists", req.Username), nil)
	}
	if req.DeptId != "" {
		if _, err := uapp.dbClient.WithCtx(ctx).SysDeptById(req.DeptId); err != nil {
			return err
		}
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	user := models.User{
		Id:         utils.GenUUID(),
		Username:   req.Username,
		Password:   string(hash),
		Nickname:   req.Nickname,
		Email:      req.Email,
		Phone:      req.Phone,
		DeptId:     req.DeptId,
		Status:     req.Status,
		Remark:     req.Remark,
		Lang:       DefaultLang,
		Timestamps: models.Timestamps{Created: nowMilli(), Modified: nowMilli()},
	}
	if user.Nickname == "" {
		user.Nickname = user.Username
	}
	created, err := uapp.dbClient.WithCtx(ctx).AddSysUser(user)
	if err != nil {
		return err
	}
	if len(req.RoleIds) > 0 {
		if err = uapp.checkRolesExist(req.RoleIds); err != nil {
			return err
		}
		if err = uapp.dbClient.WithCtx(ctx).SetUserRoles(created.Id, req.RoleIds); err != nil {
			return err
		}
	}
	return nil
}

func (uapp *userApp) SysUserUpdate(ctx context.Context, req dtos.SysUserUpdateRequest) error {
	user, err := uapp.dbClient.WithCtx(ctx).SysUserById(req.Id)
	if err != nil {
		return err
	}
	if user.Username == DefaultUserName {
		// 内置管理员不允许停用
		if req.Status == 0 {
			return errort.NewCommonErr(errort.AppAdminNotAllowOperate, fmt.Errorf("admin account can not be disabled"))
		}
	}
	if req.DeptId != "" {
		if _, err = uapp.dbClient.WithCtx(ctx).SysDeptById(req.DeptId); err != nil {
			return err
		}
	}
	if len(req.RoleIds) > 0 {
		if err = uapp.checkRolesExist(req.RoleIds); err != nil {
			return err
		}
	}
	user.Nickname = req.Nickname
	user.Email = req.Email
	user.Phone = req.Phone
	user.DeptId = req.DeptId
	user.Status = req.Status
	user.Remark = req.Remark
	user.Modified = nowMilli()
	if err = uapp.dbClient.WithCtx(ctx).UpdateSysUser(user); err != nil {
		return err
	}
	return uapp.dbClient.WithCtx(ctx).SetUserRoles(user.Id, req.RoleIds)
}

func (uapp *userApp) SysUserDelete(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return errort.NewCommonErr(errort.DefaultReqParamsError, fmt.Errorf("ids is empty"))
	}
	users, _, err := uapp.dbClient.WithCtx(ctx).SysUserSearch(0, -1, dtos.SysUserSearchRequest{IsAll: true})
	if err != nil {
		return err
	}
	valid := make(map[string]struct{}, len(users))
	for _, u := range users {
		valid[u.Id] = struct{}{}
		if u.Username == DefaultUserName {
			for _, id := range ids {
				if id == u.Id {
					return errort.NewCommonErr(errort.AppAdminNotAllowOperate, fmt.Errorf("admin account can not be deleted"))
				}
			}
		}
	}
	for _, id := range ids {
		if _, ok := valid[id]; !ok {
			return errort.NewCommonEdgeX(errort.DefaultResourcesNotFound, fmt.Sprintf("user %s not found", id), nil)
		}
	}
	if err = uapp.dbClient.WithCtx(ctx).DeleteUserRolesByUserIds(ids); err != nil {
		return err
	}
	return uapp.dbClient.WithCtx(ctx).DeleteSysUserByIds(ids)
}

func (uapp *userApp) SysUserResetPassword(ctx context.Context, id string, req dtos.SysUserResetPwdRequest) error {
	if err := uapp.validatePassword(req.Password); err != nil {
		return err
	}
	user, err := uapp.dbClient.WithCtx(ctx).SysUserById(id)
	if err != nil {
		return err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_ = user
	return uapp.dbClient.WithCtx(ctx).ResetSysUserPassword(id, string(hash))
}

func (uapp *userApp) SysUserChangeStatus(ctx context.Context, id string, status int) error {
	user, err := uapp.dbClient.WithCtx(ctx).SysUserById(id)
	if err != nil {
		return err
	}
	if user.Username == DefaultUserName && status == 0 {
		return errort.NewCommonErr(errort.AppAdminNotAllowOperate, fmt.Errorf("admin account can not be disabled"))
	}
	user.Status = status
	user.Modified = nowMilli()
	return uapp.dbClient.WithCtx(ctx).UpdateSysUser(user)
}

func (uapp *userApp) checkRolesExist(roleIds []string) error {
	if len(roleIds) == 0 {
		return nil
	}
	// 平台作用域：系统路径，不按租户过滤
	roles, err := uapp.dbClient.WithCtx(tenant.Platform(context.Background())).SysRolesByIds(roleIds)
	if err != nil {
		return err
	}
	if len(roles) != len(dedupStrings(roleIds)) {
		return errort.NewCommonEdgeX(errort.DefaultResourcesNotFound, "some roles not found", nil)
	}
	return nil
}

/******************************* 角色管理 *******************************/

func (uapp *userApp) SysRoleSearch(ctx context.Context, req dtos.SysRoleSearchRequest) ([]dtos.SysRoleResponse, uint32, error) {
	offset, limit := pageParams(req.Page, req.PageSize, req.IsAll)
	list, total, err := uapp.dbClient.WithCtx(ctx).SysRoleSearch(offset, limit, req)
	if err != nil {
		return nil, 0, err
	}
	result := make([]dtos.SysRoleResponse, 0, len(list))
	for _, item := range list {
		menuIds, _ := uapp.dbClient.WithCtx(ctx).GetRoleMenuIds(item.Id)
		result = append(result, toRoleResponse(item, menuIds))
	}
	return result, total, nil
}

func (uapp *userApp) SysRoleById(ctx context.Context, id string) (dtos.SysRoleResponse, error) {
	role, err := uapp.dbClient.WithCtx(ctx).SysRoleById(id)
	if err != nil {
		return dtos.SysRoleResponse{}, err
	}
	menuIds, _ := uapp.dbClient.WithCtx(ctx).GetRoleMenuIds(id)
	return toRoleResponse(role, menuIds), nil
}

func (uapp *userApp) SysRoleAdd(ctx context.Context, req dtos.SysRoleAddRequest) error {
	// 角色按租户隔离：查重必须限定在"这个角色将要归属的租户"内。
	// 平台视图下插件不过滤，所以显式带 tenant_id，避免与别的租户的角色误判重复。
	tenantId := tenant.FromContext(ctx).TenantID
	roleKey := strings.TrimSpace(req.RoleKey)
	if roleKey == "" {
		return errort.NewCommonEdgeX(errort.DefaultReqParamsError, "roleKey is required", nil)
	}
	if _, err := uapp.dbClient.WithCtx(ctx).SysRoleByNameInTenant(req.Name, tenantId); err == nil {
		return errort.NewCommonEdgeX(errort.DefaultNameRepeat,
			fmt.Sprintf("角色名称 %s 在该租户下已存在", req.Name), nil)
	}
	if _, err := uapp.dbClient.WithCtx(ctx).SysRoleByKeyInTenant(roleKey, tenantId); err == nil {
		return errort.NewCommonEdgeX(errort.DefaultNameRepeat,
			fmt.Sprintf("权限字符 %s 在该租户下已存在", roleKey), nil)
	}
	role := models.Role{
		Id:         utils.GenUUID(),
		Name:       req.Name,
		RoleKey:    strings.TrimSpace(req.RoleKey),
		Sort:       req.Sort,
		DataScope:  req.DataScope,
		Status:     req.Status,
		Remark:     req.Remark,
		Timestamps: models.Timestamps{Created: nowMilli(), Modified: nowMilli()},
	}
	if role.DataScope == "" {
		role.DataScope = models.DataScopeAll
	}
	created, err := uapp.dbClient.WithCtx(ctx).AddSysRole(role)
	if err != nil {
		return err
	}
	if len(req.MenuIds) > 0 {
		if err = uapp.checkMenusExist(ctx, req.MenuIds); err != nil {
			return err
		}
		return uapp.dbClient.WithCtx(ctx).SetRoleMenus(created.Id, req.MenuIds)
	}
	return nil
}

func (uapp *userApp) SysRoleUpdate(ctx context.Context, req dtos.SysRoleUpdateRequest) error {
	role, err := uapp.dbClient.WithCtx(ctx).SysRoleById(req.Id)
	if err != nil {
		return err
	}
	if role.RoleKey == seed.RoleAdminKey && req.Status == 0 {
		return errort.NewCommonErr(errort.AppAdminNotAllowOperate, fmt.Errorf("admin role can not be disabled"))
	}
	updateTenantId := tenant.FromContext(ctx).TenantID
	updateRoleKey := strings.TrimSpace(req.RoleKey)
	if updateRoleKey == "" {
		return errort.NewCommonEdgeX(errort.DefaultReqParamsError, "roleKey is required", nil)
	}
	if exist, eErr := uapp.dbClient.WithCtx(ctx).SysRoleByNameInTenant(req.Name, updateTenantId); eErr == nil && exist.Id != req.Id {
		return errort.NewCommonEdgeX(errort.DefaultNameRepeat,
			fmt.Sprintf("角色名称 %s 在该租户下已存在", req.Name), nil)
	}
	if exist, eErr := uapp.dbClient.WithCtx(ctx).SysRoleByKeyInTenant(updateRoleKey, updateTenantId); eErr == nil && exist.Id != req.Id {
		return errort.NewCommonEdgeX(errort.DefaultNameRepeat,
			fmt.Sprintf("权限字符 %s 在该租户下已存在", updateRoleKey), nil)
	}

	if len(req.MenuIds) > 0 {
		if err = uapp.checkMenusExist(ctx, req.MenuIds); err != nil {
			return err
		}
	}
	role.Name = req.Name
	role.RoleKey = strings.TrimSpace(req.RoleKey)
	role.Sort = req.Sort
	role.DataScope = req.DataScope
	role.Status = req.Status
	role.Remark = req.Remark
	role.Modified = nowMilli()
	if err = uapp.dbClient.WithCtx(ctx).UpdateSysRole(role); err != nil {
		return err
	}
	return uapp.dbClient.WithCtx(ctx).SetRoleMenus(role.Id, req.MenuIds)
}

func (uapp *userApp) SysRoleDelete(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return errort.NewCommonErr(errort.DefaultReqParamsError, fmt.Errorf("ids is empty"))
	}
	for _, id := range ids {
		if id == seed.RoleAdminId {
			return errort.NewCommonErr(errort.AppAdminNotAllowOperate, fmt.Errorf("admin role can not be deleted"))
		}
		role, err := uapp.dbClient.WithCtx(ctx).SysRoleById(id)
		if err != nil {
			return err
		}
		if role.RoleKey == seed.RoleAdminKey {
			return errort.NewCommonErr(errort.AppAdminNotAllowOperate, fmt.Errorf("admin role can not be deleted"))
		}
		userIds, err := uapp.dbClient.WithCtx(ctx).SysUserIdsByRoleId(id)
		if err != nil {
			return err
		}
		if len(userIds) > 0 {
			return errort.NewCommonEdgeX(errort.AppRoleInUse, fmt.Sprintf("role %s is assigned to users", role.Name), nil)
		}
	}
	if err := uapp.dbClient.WithCtx(ctx).DeleteUserRolesByRoleIds(ids); err != nil {
		return err
	}
	if err := uapp.dbClient.WithCtx(ctx).DeleteRoleMenusByRoleIds(ids); err != nil {
		return err
	}
	return uapp.dbClient.WithCtx(ctx).DeleteSysRoleByIds(ids)
}

func (uapp *userApp) SysRoleAll(ctx context.Context) ([]dtos.SysRoleResponse, error) {
	list, err := uapp.dbClient.WithCtx(ctx).SysRoleAll()
	if err != nil {
		return nil, err
	}
	result := make([]dtos.SysRoleResponse, 0, len(list))
	for _, item := range list {
		menuIds, _ := uapp.dbClient.WithCtx(ctx).GetRoleMenuIds(item.Id)
		result = append(result, toRoleResponse(item, menuIds))
	}
	return result, nil
}

func (uapp *userApp) SysRoleMenus(ctx context.Context, roleId string) ([]string, error) {
	if _, err := uapp.dbClient.WithCtx(ctx).SysRoleById(roleId); err != nil {
		return nil, err
	}
	return uapp.dbClient.WithCtx(ctx).GetRoleMenuIds(roleId)
}

func (uapp *userApp) SysRoleSetMenus(ctx context.Context, req dtos.SysRoleMenuRequest) error {
	if _, err := uapp.dbClient.WithCtx(ctx).SysRoleById(req.RoleId); err != nil {
		return err
	}
	if len(req.MenuIds) > 0 {
		if err := uapp.checkMenusExist(ctx, req.MenuIds); err != nil {
			return err
		}
	}
	return uapp.dbClient.WithCtx(ctx).SetRoleMenus(req.RoleId, req.MenuIds)
}

/******************************* 菜单管理 *******************************/

// 菜单是「全局默认（tenant_id=''）+ 租户自建（tenant_id=X）」表，不做 GORM 行级隔离，
// 作用域统一由 targetTenant(ctx) 给出（平台视图 → ''，租户视图 → 当前租户）。
// 详见 docs/multi-tenant-menu-plan.md §2.2（读取合并）与 §2.3（写入归属）。

func (uapp *userApp) SysMenuSearch(ctx context.Context, req dtos.SysMenuSearchRequest) ([]dtos.SysMenuResponse, uint32, error) {
	offset, limit := pageParams(req.Page, req.PageSize, req.IsAll)
	list, total, err := uapp.dbClient.WithCtx(ctx).SysMenuSearch(offset, limit, targetTenant(ctx), req)
	if err != nil {
		return nil, 0, err
	}
	result := make([]dtos.SysMenuResponse, 0, len(list))
	for _, item := range list {
		result = append(result, toMenuResponse(item))
	}
	return result, total, nil
}

func (uapp *userApp) SysMenuTree(ctx context.Context, req dtos.SysMenuSearchRequest) ([]dtos.SysMenuResponse, error) {
	list, err := uapp.dbClient.WithCtx(ctx).SysMenuAll(targetTenant(ctx))
	if err != nil {
		return nil, err
	}
	result := make([]dtos.SysMenuResponse, 0, len(list))
	for _, item := range list {
		result = append(result, toMenuResponse(item))
	}
	return buildMenuChildren(result, "0"), nil
}

func (uapp *userApp) SysMenuById(ctx context.Context, id string) (dtos.SysMenuResponse, error) {
	menu, err := uapp.dbClient.WithCtx(ctx).SysMenuById(id)
	if err != nil {
		return dtos.SysMenuResponse{}, err
	}
	// 只允许读平台菜单或本租户自建菜单，避免读到别的租户的菜单
	if !menuInScope(menu, targetTenant(ctx)) {
		return dtos.SysMenuResponse{}, errort.NewCommonEdgeX(errort.DefaultResourcesNotFound,
			fmt.Sprintf("menu %s not found", id), nil)
	}
	return toMenuResponse(menu), nil
}

// SysMenuAdd 新增菜单：写入归属由 ctx 决定（平台视图 → 平台菜单；租户视图 → 该租户自建菜单）。
// 租户自建菜单不允许设置 perms（防自助提权）。
func (uapp *userApp) SysMenuAdd(ctx context.Context, req dtos.SysMenuAddRequest) error {
	target := targetTenant(ctx)
	if err := requireMenuWriteScope(ctx, target); err != nil {
		return err
	}
	if err := requireNoTenantPerms(target, req.Perms); err != nil {
		return err
	}
	db := uapp.dbClient.WithCtx(ctx)
	parentId := normalizeParentId(req.ParentId)
	if parentId != "0" {
		parent, err := db.SysMenuById(parentId)
		if err != nil {
			return err
		}
		// 允许把自建菜单挂在平台菜单下；但不能挂到其他租户的菜单下
		if !menuInScope(parent, target) {
			return errort.NewCommonErr(errort.AppPermissionDenied,
				errort.NewMessageErr("父菜单 {{.p1}} 不在当前作用域内", map[string]interface{}{"p1": parentId}))
		}
	}
	menu := models.Menu{
		Id:         utils.GenUUID(),
		ParentId:   parentId,
		Name:       req.Name,
		Path:       req.Path,
		Component:  req.Component,
		Icon:       req.Icon,
		MenuType:   normalizeMenuType(req.MenuType),
		Perms:      req.Perms,
		Sort:       req.Sort,
		Visible:    req.Visible,
		Status:     req.Status,
		IsFrame:    req.IsFrame,
		TenantId:   target,
		Remark:     req.Remark,
		Timestamps: models.Timestamps{Created: nowMilli(), Modified: nowMilli()},
	}
	if menu.IsFrame == 0 {
		menu.IsFrame = 1
	}
	if menu.Visible == 0 {
		menu.Visible = 1
	}
	if menu.Status == 0 {
		menu.Status = 1
	}
	_, err := db.AddSysMenu(menu)
	return err
}

func (uapp *userApp) SysMenuUpdate(ctx context.Context, req dtos.SysMenuUpdateRequest) error {
	db := uapp.dbClient.WithCtx(ctx)
	menu, err := db.SysMenuById(req.Id)
	if err != nil {
		return err
	}
	target := targetTenant(ctx)
	// 归属校验：改平台菜单必须是平台管理员；改租户菜单必须属于该租户（不做静默忽略）
	if err = requireMenuRowOwnership(ctx, menu, target); err != nil {
		return err
	}
	if err = requireNoTenantPerms(target, req.Perms); err != nil {
		return err
	}
	parentId := normalizeParentId(req.ParentId)
	if parentId == req.Id {
		return errort.NewCommonErr(errort.AppParentSelf, fmt.Errorf("parent menu can not be itself"))
	}
	if parentId != "0" {
		parent, pErr := db.SysMenuById(parentId)
		if pErr != nil {
			return pErr
		}
		if !menuInScope(parent, target) {
			return errort.NewCommonErr(errort.AppPermissionDenied,
				errort.NewMessageErr("父菜单 {{.p1}} 不在当前作用域内", map[string]interface{}{"p1": parentId}))
		}
		// 防止把父节点挂到自己的子节点下
		descendants, dErr := uapp.menuDescendants(req.Id, target)
		if dErr != nil {
			return dErr
		}
		for _, id := range descendants {
			if id == parentId {
				return errort.NewCommonErr(errort.AppParentSelf, fmt.Errorf("parent menu can not be its child"))
			}
		}
	}
	menu.ParentId = parentId
	menu.Name = req.Name
	menu.Path = req.Path
	menu.Component = req.Component
	menu.Icon = req.Icon
	menu.MenuType = normalizeMenuType(req.MenuType)
	menu.Perms = req.Perms
	menu.Sort = req.Sort
	menu.Visible = req.Visible
	menu.Status = req.Status
	menu.IsFrame = req.IsFrame
	// 归属不可变（已校验与 target 相等，这里显式写回，避免 Save 把归属改掉）
	menu.TenantId = target
	menu.Remark = req.Remark
	menu.Modified = nowMilli()
	return db.UpdateSysMenu(menu)
}

// SysMenuDelete 删除菜单（§2.5）：
//   - 目标租户内的每一行都要校验归属；
//   - 平台删除全局菜单时，若有租户自建子菜单挂在它下面 → 拒绝并提示数量（避免孤儿菜单）；
//   - 只有覆盖记录时允许删除，并级联清理 tenant_menu_pref。
func (uapp *userApp) SysMenuDelete(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return errort.NewCommonErr(errort.DefaultReqParamsError, fmt.Errorf("ids is empty"))
	}
	db := uapp.dbClient.WithCtx(ctx)
	target := targetTenant(ctx)
	for _, id := range ids {
		menu, err := db.SysMenuById(id)
		if err != nil {
			return err
		}
		if err = requireMenuRowOwnership(ctx, menu, target); err != nil {
			return err
		}
	}
	if target == "" {
		tenantChildCount, err := db.SysMenuCountTenantChildrenByParentIds(ids)
		if err != nil {
			return err
		}
		if tenantChildCount > 0 {
			return errort.NewCommonEdgeX(errort.AppMenuHasTenantChildren,
				fmt.Sprintf("有 %d 个租户自建菜单挂在该平台菜单下，请先由租户删除或改用停用", tenantChildCount), nil)
		}
	}
	childCount, err := db.SysMenuCountByParentIds(ids)
	if err != nil {
		return err
	}
	if childCount > 0 {
		return errort.NewCommonEdgeX(errort.AppMenuHasChildren, "menu has children, please delete children first", nil)
	}
	if err = db.DeleteRoleMenusByMenuIds(ids); err != nil {
		return err
	}
	// 级联清理所有租户对该菜单的展示层覆盖
	if err = db.DeleteSysMenuPrefsByMenuIds(ids); err != nil {
		return err
	}
	return db.DeleteSysMenuByIds(ids)
}

// MenuRoutesByUsername 当前用户可见菜单树
func (uapp *userApp) MenuRoutesByUsername(username string) ([]dtos.SysMenuResponse, error) {
	sysDb := uapp.dbClient.WithCtx(tenant.Platform(context.Background()))
	user, err := sysDb.GetUserByUserName(username)
	if err != nil {
		return nil, err
	}
	// 与 buildLoginUser 用同一套判定：租户内的 admin 角色同样是该租户的管理员
	admin := uapp.hasAdminRole(sysDb, user.Id)
	menus, err := uapp.menusOfUser(user.Id, user.TenantId, admin)
	if err != nil {
		return nil, err
	}
	// 平台级菜单（platform_only=1，如「租户管理」「系统配置」）只下发给平台管理员：
	// 与 buildLoginUser / PermissionsByUsername 保持一致，否则租户侧菜单树会多出
	// 自己无权限访问（或不该看到）的平台菜单。
	// 注意必须在 completeMenuAncestors 之后过滤：补全祖先时可能把平台级父菜单加回来。
	if !admin {
		// 角色授权时可能只勾了子菜单没勾父目录，这里补全祖先目录，否则子菜单挂不到树上
		if menus, err = uapp.completeMenuAncestors(menus, user.TenantId); err != nil {
			return nil, err
		}
	}
	if platformAdmin := admin && user.TenantId == ""; !platformAdmin {
		menus = filterPlatformOnlyMenus(menus)
	}
	if menus, err = uapp.applyMenuPrefs(user.TenantId, menus); err != nil {
		return nil, err
	}
	return buildMenuTree(menus), nil
}

// completeMenuAncestors 补全缺失的祖先目录（仅补状态正常的），保证下发的菜单树是连通的
func (uapp *userApp) completeMenuAncestors(menus []models.Menu, tenantId string) ([]models.Menu, error) {
	if len(menus) == 0 {
		return menus, nil
	}
	db := uapp.dbClient.WithCtx(tenant.Platform(context.Background()))
	all, err := db.SysMenuAll(tenantId)
	if err != nil {
		return nil, err
	}
	byId := make(map[string]models.Menu, len(all))
	for _, m := range all {
		byId[m.Id] = m
	}
	seen := make(map[string]struct{}, len(menus))
	result := make([]models.Menu, 0, len(menus))
	for _, m := range menus {
		seen[m.Id] = struct{}{}
		result = append(result, m)
	}
	for _, m := range menus {
		parentId := m.ParentId
		for parentId != "" && parentId != "0" {
			parent, ok := byId[parentId]
			if !ok || parent.Status != 1 {
				// 父目录不存在或已停用：子菜单保持挂空（buildMenuTree 会自动丢弃）
				break
			}
			if _, ok := seen[parent.Id]; !ok {
				seen[parent.Id] = struct{}{}
				result = append(result, parent)
			}
			parentId = parent.ParentId
		}
	}
	return result, nil
}

func (uapp *userApp) menuDescendants(rootId, tenantId string) ([]string, error) {
	all, err := uapp.dbClient.WithCtx(tenant.Platform(context.Background())).SysMenuAll(tenantId)
	if err != nil {
		return nil, err
	}
	result := make([]string, 0)
	queue := []string{rootId}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		for _, m := range all {
			if m.ParentId == current {
				result = append(result, m.Id)
				queue = append(queue, m.Id)
			}
		}
	}
	return result, nil
}

// checkMenusExist 校验角色要授权的菜单都在当前作用域内：
// 可以是平台菜单或本租户自建菜单，但不能是其他租户的菜单。
func (uapp *userApp) checkMenusExist(ctx context.Context, menuIds []string) error {
	menus, err := uapp.dbClient.WithCtx(ctx).SysMenusByIds(menuIds)
	if err != nil {
		return err
	}
	if len(menus) != len(dedupStrings(menuIds)) {
		return errort.NewCommonEdgeX(errort.DefaultResourcesNotFound, "some menus not found", nil)
	}
	target := targetTenant(ctx)
	for _, m := range menus {
		if !menuInScope(m, target) {
			return errort.NewCommonErr(errort.AppPermissionDenied,
				errort.NewMessageErr("菜单 {{.p1}} 不属于当前作用域，不能授权", map[string]interface{}{"p1": m.Id}))
		}
	}
	return nil
}

/******************************* 租户菜单展示层覆盖 *******************************/

// SysMenuPrefSave 保存当前租户的菜单展示层覆盖（隐藏/改名/图标/排序）。
// 只写 tenant_menu_pref：path / perms / component 永不被覆盖。
func (uapp *userApp) SysMenuPrefSave(ctx context.Context, req dtos.SysMenuPrefSaveRequest) error {
	target := targetTenant(ctx)
	if target == "" {
		return errort.NewCommonErr(errort.DefaultReqParamsError,
			errort.NewMessageErr("菜单定制属于租户视图，请先切换到租户后再操作", nil))
	}
	db := uapp.dbClient.WithCtx(ctx)
	menu, err := db.SysMenuById(req.MenuId)
	if err != nil {
		return err
	}
	if !menuInScope(menu, target) {
		return errort.NewCommonEdgeX(errort.DefaultResourcesNotFound,
			fmt.Sprintf("menu %s not found", req.MenuId), nil)
	}
	pref := models.TenantMenuPref{
		TenantId:   target,
		MenuId:     req.MenuId,
		Hidden:     req.Hidden,
		CustomName: strings.TrimSpace(req.CustomName),
		CustomIcon: strings.TrimSpace(req.CustomIcon),
		Sort:       req.Sort,
	}
	if pref.Hidden != 0 {
		pref.Hidden = 1
	}
	// 全空 = 恢复平台默认展示，直接清掉覆盖记录
	if pref.Hidden == 0 && pref.CustomName == "" && pref.CustomIcon == "" && pref.Sort == 0 {
		return db.DeleteSysMenuPref(target, req.MenuId)
	}
	_, err = db.UpsertSysMenuPref(pref)
	return err
}

// SysMenuPrefs 当前作用域的菜单覆盖记录（平台视图返回空：覆盖只属于租户）
func (uapp *userApp) SysMenuPrefs(ctx context.Context) ([]dtos.SysMenuPrefResponse, error) {
	target := targetTenant(ctx)
	if target == "" {
		return []dtos.SysMenuPrefResponse{}, nil
	}
	list, err := uapp.menuPrefs(target)
	if err != nil {
		return nil, err
	}
	result := make([]dtos.SysMenuPrefResponse, 0, len(list))
	for _, item := range list {
		result = append(result, dtos.SysMenuPrefResponse{
			MenuId:     item.MenuId,
			Hidden:     item.Hidden,
			CustomName: item.CustomName,
			CustomIcon: item.CustomIcon,
			Sort:       item.Sort,
		})
	}
	return result, nil
}

// SysMenuTenantReset 平台重置某租户的菜单定制：清空该租户的自建菜单与全部展示层覆盖，
// 恢复平台默认（§2.5）。仅平台管理员可调用。
func (uapp *userApp) SysMenuTenantReset(ctx context.Context, tenantId string) error {
	if !isPlatformAdminCtx(ctx) {
		return errort.NewCommonErr(errort.AppPermissionDenied,
			errort.NewMessageErr("只有平台管理员可以重置租户菜单定制", nil))
	}
	tenantId = strings.TrimSpace(tenantId)
	if tenantId == "" {
		return errort.NewCommonErr(errort.DefaultReqParamsError, fmt.Errorf("tenantId is required"))
	}
	sysDb := uapp.dbClient.WithCtx(tenant.Platform(context.Background()))
	if _, err := sysDb.TenantById(tenantId); err != nil {
		return err
	}
	db := uapp.dbClient.WithCtx(ctx)
	menus, err := db.SysMenusByTenant(tenantId)
	if err != nil {
		return err
	}
	ids := make([]string, 0, len(menus))
	for _, m := range menus {
		ids = append(ids, m.Id)
	}
	if len(ids) > 0 {
		if err = db.DeleteRoleMenusByMenuIds(ids); err != nil {
			return err
		}
		if err = db.DeleteSysMenuPrefsByMenuIds(ids); err != nil {
			return err
		}
		if err = db.DeleteSysMenuByIds(ids); err != nil {
			return err
		}
	}
	_, err = db.DeleteSysMenuPrefsByTenant(tenantId)
	return err
}

/******************************* 部门管理 *******************************/

func (uapp *userApp) SysDeptSearch(ctx context.Context, req dtos.SysDeptSearchRequest) ([]dtos.SysDeptResponse, uint32, error) {
	offset, limit := pageParams(req.Page, req.PageSize, req.IsAll)
	list, total, err := uapp.dbClient.WithCtx(ctx).SysDeptSearch(offset, limit, req)
	if err != nil {
		return nil, 0, err
	}
	result := make([]dtos.SysDeptResponse, 0, len(list))
	for _, item := range list {
		result = append(result, toDeptResponse(item))
	}
	return result, total, nil
}

func (uapp *userApp) SysDeptTree(ctx context.Context, req dtos.SysDeptSearchRequest) ([]dtos.SysDeptResponse, error) {
	list, err := uapp.dbClient.WithCtx(ctx).SysDeptAll()
	if err != nil {
		return nil, err
	}
	result := make([]dtos.SysDeptResponse, 0, len(list))
	for _, item := range list {
		result = append(result, toDeptResponse(item))
	}
	return buildDeptTree(result), nil
}

func (uapp *userApp) SysDeptById(ctx context.Context, id string) (dtos.SysDeptResponse, error) {
	dept, err := uapp.dbClient.WithCtx(ctx).SysDeptById(id)
	if err != nil {
		return dtos.SysDeptResponse{}, err
	}
	return toDeptResponse(dept), nil
}

func (uapp *userApp) SysDeptAdd(ctx context.Context, req dtos.SysDeptAddRequest) error {
	parentId := normalizeParentId(req.ParentId)
	ancestors := "0"
	if parentId != "0" {
		parent, err := uapp.dbClient.WithCtx(ctx).SysDeptById(parentId)
		if err != nil {
			return err
		}
		ancestors = parent.Ancestors + "," + parent.Id
	}
	dept := models.Dept{
		Id:         utils.GenUUID(),
		ParentId:   parentId,
		Ancestors:  ancestors,
		Name:       req.Name,
		DeptType:   normalizeDeptType(req.DeptType),
		Sort:       req.Sort,
		Leader:     req.Leader,
		Phone:      req.Phone,
		Email:      req.Email,
		Status:     req.Status,
		Timestamps: models.Timestamps{Created: nowMilli(), Modified: nowMilli()},
	}
	if dept.Status == 0 {
		dept.Status = 1
	}
	_, err := uapp.dbClient.WithCtx(ctx).AddSysDept(dept)
	return err
}

func (uapp *userApp) SysDeptUpdate(ctx context.Context, req dtos.SysDeptUpdateRequest) error {
	dept, err := uapp.dbClient.WithCtx(ctx).SysDeptById(req.Id)
	if err != nil {
		return err
	}
	parentId := normalizeParentId(req.ParentId)
	if parentId == req.Id {
		return errort.NewCommonErr(errort.AppParentSelf, fmt.Errorf("parent dept can not be itself"))
	}
	ancestors := "0"
	if parentId != "0" {
		parent, pErr := uapp.dbClient.WithCtx(ctx).SysDeptById(parentId)
		if pErr != nil {
			return pErr
		}
		if parent.Ancestors+","+parent.Id == dept.Ancestors {
			return errort.NewCommonErr(errort.AppParentSelf, fmt.Errorf("parent dept can not be its child"))
		}
		ancestors = parent.Ancestors + "," + parent.Id
	}
	dept.ParentId = parentId
	dept.Ancestors = ancestors
	dept.Name = req.Name
	if req.DeptType != 0 {
		dept.DeptType = normalizeDeptType(req.DeptType)
	}
	dept.Sort = req.Sort
	dept.Leader = req.Leader
	dept.Phone = req.Phone
	dept.Email = req.Email
	dept.Status = req.Status
	dept.Modified = nowMilli()
	return uapp.dbClient.WithCtx(ctx).UpdateSysDept(dept)
}

func (uapp *userApp) SysDeptDelete(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return errort.NewCommonErr(errort.DefaultReqParamsError, fmt.Errorf("ids is empty"))
	}
	for _, id := range ids {
		if id == seed.DeptRootId {
			return errort.NewCommonErr(errort.AppAdminNotAllowOperate, fmt.Errorf("root dept can not be deleted"))
		}
	}
	childCount, err := uapp.dbClient.WithCtx(ctx).SysDeptCountByParentIds(ids)
	if err != nil {
		return err
	}
	if childCount > 0 {
		return errort.NewCommonEdgeX(errort.AppDeptHasChildren, "dept has children, please delete children first", nil)
	}
	userCount, err := uapp.dbClient.WithCtx(ctx).SysUserCountByDeptIds(ids)
	if err != nil {
		return err
	}
	if userCount > 0 {
		return errort.NewCommonEdgeX(errort.AppDeptInUse, "dept is assigned to users", nil)
	}
	return uapp.dbClient.WithCtx(ctx).DeleteSysDeptByIds(ids)
}

/******************************* 审查日志（登录 + 操作） *******************************/

func (uapp *userApp) SysAuditLogSearch(ctx context.Context, req dtos.SysAuditLogSearchRequest) ([]dtos.SysAuditLogResponse, uint32, error) {
	offset, limit := pageParams(req.Page, req.PageSize, req.IsAll)
	list, total, err := uapp.dbClient.WithCtx(ctx).SysAuditLogSearch(offset, limit, req)
	if err != nil {
		return nil, 0, err
	}
	result := make([]dtos.SysAuditLogResponse, 0, len(list))
	for _, item := range list {
		result = append(result, toAuditLogResponse(item))
	}
	return result, total, nil
}

func (uapp *userApp) SysAuditLogDelete(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return errort.NewCommonErr(errort.DefaultReqParamsError, fmt.Errorf("ids is empty"))
	}
	return uapp.dbClient.WithCtx(ctx).DeleteSysAuditLogByIds(ids)
}

func (uapp *userApp) SysAuditLogClear(ctx context.Context) error {
	return uapp.dbClient.WithCtx(ctx).ClearSysAuditLog()
}

/******************************* 系统配置 *******************************/

func (uapp *userApp) SysConfigSearch(ctx context.Context, req dtos.SysConfigSearchRequest) ([]dtos.SysConfigResponse, uint32, error) {
	offset, limit := pageParams(req.Page, req.PageSize, req.IsAll)
	list, total, err := uapp.dbClient.WithCtx(ctx).SysConfigSearch(offset, limit, req)
	if err != nil {
		return nil, 0, err
	}
	result := make([]dtos.SysConfigResponse, 0, len(list))
	for _, item := range list {
		result = append(result, toConfigResponse(item))
	}
	return result, total, nil
}

func (uapp *userApp) SysConfigById(ctx context.Context, id string) (dtos.SysConfigResponse, error) {
	cfg, err := uapp.dbClient.WithCtx(ctx).SysConfigById(id)
	if err != nil {
		return dtos.SysConfigResponse{}, err
	}
	return toConfigResponse(cfg), nil
}

func (uapp *userApp) SysConfigAdd(ctx context.Context, req dtos.SysConfigAddRequest) error {
	cfg := models.SystemConfig{
		Id:          utils.GenUUID(),
		ConfigKey:   req.ConfigKey,
		ConfigValue: req.ConfigValue,
		Name:        req.Name,
		GroupName:   req.GroupName,
		Type:        req.Type,
		Sort:        req.Sort,
		Remark:      req.Remark,
		Timestamps:  models.Timestamps{Created: nowMilli(), Modified: nowMilli()},
	}
	if cfg.Type == "" {
		cfg.Type = "string"
	}
	if cfg.GroupName == "" {
		cfg.GroupName = "basic"
	}
	_, err := uapp.dbClient.WithCtx(ctx).AddSysConfig(cfg)
	return err
}

func (uapp *userApp) SysConfigUpdate(ctx context.Context, req dtos.SysConfigUpdateRequest) error {
	cfg, err := uapp.dbClient.WithCtx(ctx).SysConfigById(req.Id)
	if err != nil {
		return err
	}
	if exist, eErr := uapp.dbClient.WithCtx(ctx).SysConfigByKey(req.ConfigKey); eErr == nil && exist.Id != req.Id {
		return errort.NewCommonEdgeX(errort.DefaultNameRepeat, fmt.Sprintf("config key %s exists", req.ConfigKey), nil)
	}
	cfg.ConfigKey = req.ConfigKey
	cfg.ConfigValue = req.ConfigValue
	cfg.Name = req.Name
	cfg.GroupName = req.GroupName
	cfg.Type = req.Type
	cfg.Sort = req.Sort
	cfg.Remark = req.Remark
	cfg.Modified = nowMilli()
	return uapp.dbClient.WithCtx(ctx).UpdateSysConfig(cfg)
}

func (uapp *userApp) SysConfigDelete(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return errort.NewCommonErr(errort.DefaultReqParamsError, fmt.Errorf("ids is empty"))
	}
	return uapp.dbClient.WithCtx(ctx).DeleteSysConfigByIds(ids)
}

func (uapp *userApp) SysSetup(ctx context.Context) ([]dtos.SysConfigResponse, error) {
	list, _, err := uapp.dbClient.WithCtx(ctx).SysConfigSearch(0, -1, dtos.SysConfigSearchRequest{IsAll: true})
	if err != nil {
		return nil, err
	}
	result := make([]dtos.SysConfigResponse, 0, len(list))
	for _, item := range list {
		result = append(result, toConfigResponse(item))
	}
	return result, nil
}

func (uapp *userApp) SysSetupSave(ctx context.Context, req dtos.SysSetupRequest) error {
	if len(req.Items) == 0 {
		return errort.NewCommonErr(errort.DefaultReqParamsError, fmt.Errorf("items is empty"))
	}
	return uapp.dbClient.WithCtx(ctx).SaveSysConfigValues(req.Items)
}

/******************************* 公共 *******************************/

func pageParams(page, pageSize int, isAll bool) (int, int) {
	if isAll {
		return 0, -1
	}
	if page <= 0 {
		page = dtos.PageDefault
	}
	if pageSize <= 0 || pageSize > dtos.PageSizeMaxDefault {
		pageSize = dtos.PageSizeDefault
	}
	return (page - 1) * pageSize, pageSize
}

// normalizeDeptType 组织节点类型：非法值按「公司」处理
func normalizeDeptType(deptType int) int {
	switch deptType {
	case models.DeptTypeCompany, models.DeptTypeFactory, models.DeptTypeShop, models.DeptTypeLine:
		return deptType
	default:
		return models.DeptTypeCompany
	}
}

func normalizeParentId(parentId string) string {
	if strings.TrimSpace(parentId) == "" {
		return "0"
	}
	return parentId
}

func normalizeMenuType(menuType string) string {
	switch menuType {
	case models.MenuTypeDir, models.MenuTypeMenu, models.MenuTypeButton:
		return menuType
	default:
		return models.MenuTypeMenu
	}
}

func dedupStrings(items []string) []string {
	seen := make(map[string]struct{}, len(items))
	result := make([]string, 0, len(items))
	for _, item := range items {
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		result = append(result, item)
	}
	return result
}

// SiteInfo 站点品牌信息（登录页免鉴权使用）
func (uapp *userApp) SiteInfo(ctx context.Context) (dtos.SiteInfoResponse, error) {
	res := dtos.SiteInfoResponse{
		Name:            "iotserver 物联网平台",
		Description:     "iotserver 物联网平台",
		Copyright:       "© iotserver",
		Logo:            "",
		Favicon:         "",
		LoginBackground: "",
	}
	pairs := map[string]*string{
		"platform.name":            &res.Name,
		"platform.description":     &res.Description,
		"platform.copyright":       &res.Copyright,
		"platform.logo":            &res.Logo,
		"platform.favicon":         &res.Favicon,
		"platform.loginBackground": &res.LoginBackground,
	}
	for key, target := range pairs {
		cfg, err := uapp.dbClient.WithCtx(ctx).SysConfigByKey(key)
		if err != nil {
			continue // 未配置则保留默认值
		}
		if cfg.ConfigValue != "" {
			*target = cfg.ConfigValue
		}
	}
	return res, nil
}
