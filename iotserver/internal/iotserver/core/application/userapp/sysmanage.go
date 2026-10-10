package userapp

import (
	"context"
	"fmt"
	"strings"

	"golang.org/x/crypto/bcrypt"
	"iotserver/internal/dtos"
	"iotserver/internal/models"
	"iotserver/internal/pkg/errort"
	"iotserver/internal/pkg/seed"
	"iotserver/internal/pkg/utils"
)

/******************************* 用户管理 *******************************/

func (uapp *userApp) SysUserSearch(ctx context.Context, req dtos.SysUserSearchRequest) ([]dtos.SysUserResponse, uint32, error) {
	offset, limit := pageParams(req.Page, req.PageSize, req.IsAll)
	list, total, err := uapp.dbClient.SysUserSearch(offset, limit, req)
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
	user, err := uapp.dbClient.SysUserById(id)
	if err != nil {
		return dtos.SysUserResponse{}, err
	}
	return uapp.toSysUserResponse(user), nil
}

func (uapp *userApp) SysUserAdd(ctx context.Context, req dtos.SysUserAddRequest) error {
	if err := uapp.validatePassword(req.Password); err != nil {
		return err
	}
	if _, err := uapp.dbClient.GetUserByUserName(req.Username); err == nil {
		return errort.NewCommonEdgeX(errort.DefaultNameRepeat, fmt.Sprintf("username %s exists", req.Username), nil)
	}
	if req.DeptId != "" {
		if _, err := uapp.dbClient.SysDeptById(req.DeptId); err != nil {
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
	created, err := uapp.dbClient.AddSysUser(user)
	if err != nil {
		return err
	}
	if len(req.RoleIds) > 0 {
		if err = uapp.checkRolesExist(req.RoleIds); err != nil {
			return err
		}
		if err = uapp.dbClient.SetUserRoles(created.Id, req.RoleIds); err != nil {
			return err
		}
	}
	return nil
}

func (uapp *userApp) SysUserUpdate(ctx context.Context, req dtos.SysUserUpdateRequest) error {
	user, err := uapp.dbClient.SysUserById(req.Id)
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
		if _, err = uapp.dbClient.SysDeptById(req.DeptId); err != nil {
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
	if err = uapp.dbClient.UpdateSysUser(user); err != nil {
		return err
	}
	return uapp.dbClient.SetUserRoles(user.Id, req.RoleIds)
}

func (uapp *userApp) SysUserDelete(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return errort.NewCommonErr(errort.DefaultReqParamsError, fmt.Errorf("ids is empty"))
	}
	users, _, err := uapp.dbClient.SysUserSearch(0, -1, dtos.SysUserSearchRequest{IsAll: true})
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
	if err = uapp.dbClient.DeleteUserRolesByUserIds(ids); err != nil {
		return err
	}
	return uapp.dbClient.DeleteSysUserByIds(ids)
}

func (uapp *userApp) SysUserResetPassword(ctx context.Context, id string, req dtos.SysUserResetPwdRequest) error {
	if err := uapp.validatePassword(req.Password); err != nil {
		return err
	}
	user, err := uapp.dbClient.SysUserById(id)
	if err != nil {
		return err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_ = user
	return uapp.dbClient.ResetSysUserPassword(id, string(hash))
}

func (uapp *userApp) SysUserChangeStatus(ctx context.Context, id string, status int) error {
	user, err := uapp.dbClient.SysUserById(id)
	if err != nil {
		return err
	}
	if user.Username == DefaultUserName && status == 0 {
		return errort.NewCommonErr(errort.AppAdminNotAllowOperate, fmt.Errorf("admin account can not be disabled"))
	}
	user.Status = status
	user.Modified = nowMilli()
	return uapp.dbClient.UpdateSysUser(user)
}

func (uapp *userApp) checkRolesExist(roleIds []string) error {
	if len(roleIds) == 0 {
		return nil
	}
	roles, err := uapp.dbClient.SysRolesByIds(roleIds)
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
	list, total, err := uapp.dbClient.SysRoleSearch(offset, limit, req)
	if err != nil {
		return nil, 0, err
	}
	result := make([]dtos.SysRoleResponse, 0, len(list))
	for _, item := range list {
		menuIds, _ := uapp.dbClient.GetRoleMenuIds(item.Id)
		result = append(result, toRoleResponse(item, menuIds))
	}
	return result, total, nil
}

func (uapp *userApp) SysRoleById(ctx context.Context, id string) (dtos.SysRoleResponse, error) {
	role, err := uapp.dbClient.SysRoleById(id)
	if err != nil {
		return dtos.SysRoleResponse{}, err
	}
	menuIds, _ := uapp.dbClient.GetRoleMenuIds(id)
	return toRoleResponse(role, menuIds), nil
}

func (uapp *userApp) SysRoleAdd(ctx context.Context, req dtos.SysRoleAddRequest) error {
	if _, err := uapp.dbClient.SysRoleByName(req.Name); err == nil {
		return errort.NewCommonEdgeX(errort.DefaultNameRepeat, fmt.Sprintf("role %s exists", req.Name), nil)
	}
	role := models.Role{
		Id:         utils.GenUUID(),
		Name:       req.Name,
		RoleKey:    req.RoleKey,
		Sort:       req.Sort,
		DataScope:  req.DataScope,
		Status:     req.Status,
		Remark:     req.Remark,
		Timestamps: models.Timestamps{Created: nowMilli(), Modified: nowMilli()},
	}
	if role.DataScope == "" {
		role.DataScope = models.DataScopeAll
	}
	created, err := uapp.dbClient.AddSysRole(role)
	if err != nil {
		return err
	}
	if len(req.MenuIds) > 0 {
		if err = uapp.checkMenusExist(req.MenuIds); err != nil {
			return err
		}
		return uapp.dbClient.SetRoleMenus(created.Id, req.MenuIds)
	}
	return nil
}

func (uapp *userApp) SysRoleUpdate(ctx context.Context, req dtos.SysRoleUpdateRequest) error {
	role, err := uapp.dbClient.SysRoleById(req.Id)
	if err != nil {
		return err
	}
	if role.RoleKey == seed.RoleAdminKey && req.Status == 0 {
		return errort.NewCommonErr(errort.AppAdminNotAllowOperate, fmt.Errorf("admin role can not be disabled"))
	}
	if exist, eErr := uapp.dbClient.SysRoleByName(req.Name); eErr == nil && exist.Id != req.Id {
		return errort.NewCommonEdgeX(errort.DefaultNameRepeat, fmt.Sprintf("role %s exists", req.Name), nil)
	}
	if len(req.MenuIds) > 0 {
		if err = uapp.checkMenusExist(req.MenuIds); err != nil {
			return err
		}
	}
	role.Name = req.Name
	role.RoleKey = req.RoleKey
	role.Sort = req.Sort
	role.DataScope = req.DataScope
	role.Status = req.Status
	role.Remark = req.Remark
	role.Modified = nowMilli()
	if err = uapp.dbClient.UpdateSysRole(role); err != nil {
		return err
	}
	return uapp.dbClient.SetRoleMenus(role.Id, req.MenuIds)
}

func (uapp *userApp) SysRoleDelete(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return errort.NewCommonErr(errort.DefaultReqParamsError, fmt.Errorf("ids is empty"))
	}
	for _, id := range ids {
		if id == seed.RoleAdminId {
			return errort.NewCommonErr(errort.AppAdminNotAllowOperate, fmt.Errorf("admin role can not be deleted"))
		}
		role, err := uapp.dbClient.SysRoleById(id)
		if err != nil {
			return err
		}
		if role.RoleKey == seed.RoleAdminKey {
			return errort.NewCommonErr(errort.AppAdminNotAllowOperate, fmt.Errorf("admin role can not be deleted"))
		}
		userIds, err := uapp.dbClient.SysUserIdsByRoleId(id)
		if err != nil {
			return err
		}
		if len(userIds) > 0 {
			return errort.NewCommonEdgeX(errort.AppRoleInUse, fmt.Sprintf("role %s is assigned to users", role.Name), nil)
		}
	}
	if err := uapp.dbClient.DeleteUserRolesByRoleIds(ids); err != nil {
		return err
	}
	if err := uapp.dbClient.DeleteRoleMenusByRoleIds(ids); err != nil {
		return err
	}
	return uapp.dbClient.DeleteSysRoleByIds(ids)
}

func (uapp *userApp) SysRoleAll(ctx context.Context) ([]dtos.SysRoleResponse, error) {
	list, err := uapp.dbClient.SysRoleAll()
	if err != nil {
		return nil, err
	}
	result := make([]dtos.SysRoleResponse, 0, len(list))
	for _, item := range list {
		menuIds, _ := uapp.dbClient.GetRoleMenuIds(item.Id)
		result = append(result, toRoleResponse(item, menuIds))
	}
	return result, nil
}

func (uapp *userApp) SysRoleMenus(ctx context.Context, roleId string) ([]string, error) {
	if _, err := uapp.dbClient.SysRoleById(roleId); err != nil {
		return nil, err
	}
	return uapp.dbClient.GetRoleMenuIds(roleId)
}

func (uapp *userApp) SysRoleSetMenus(ctx context.Context, req dtos.SysRoleMenuRequest) error {
	if _, err := uapp.dbClient.SysRoleById(req.RoleId); err != nil {
		return err
	}
	if len(req.MenuIds) > 0 {
		if err := uapp.checkMenusExist(req.MenuIds); err != nil {
			return err
		}
	}
	return uapp.dbClient.SetRoleMenus(req.RoleId, req.MenuIds)
}

/******************************* 菜单管理 *******************************/

func (uapp *userApp) SysMenuSearch(ctx context.Context, req dtos.SysMenuSearchRequest) ([]dtos.SysMenuResponse, uint32, error) {
	offset, limit := pageParams(req.Page, req.PageSize, req.IsAll)
	list, total, err := uapp.dbClient.SysMenuSearch(offset, limit, req)
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
	list, err := uapp.dbClient.SysMenuAll()
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
	menu, err := uapp.dbClient.SysMenuById(id)
	if err != nil {
		return dtos.SysMenuResponse{}, err
	}
	return toMenuResponse(menu), nil
}

func (uapp *userApp) SysMenuAdd(ctx context.Context, req dtos.SysMenuAddRequest) error {
	if req.ParentId != "" && req.ParentId != "0" {
		if _, err := uapp.dbClient.SysMenuById(req.ParentId); err != nil {
			return err
		}
	}
	menu := models.Menu{
		Id:         utils.GenUUID(),
		ParentId:   normalizeParentId(req.ParentId),
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
	_, err := uapp.dbClient.AddSysMenu(menu)
	return err
}

func (uapp *userApp) SysMenuUpdate(ctx context.Context, req dtos.SysMenuUpdateRequest) error {
	menu, err := uapp.dbClient.SysMenuById(req.Id)
	if err != nil {
		return err
	}
	parentId := normalizeParentId(req.ParentId)
	if parentId == req.Id {
		return errort.NewCommonErr(errort.AppParentSelf, fmt.Errorf("parent menu can not be itself"))
	}
	if parentId != "0" {
		if _, err = uapp.dbClient.SysMenuById(parentId); err != nil {
			return err
		}
		// 防止把父节点挂到自己的子节点下
		descendants, err := uapp.menuDescendants(req.Id)
		if err != nil {
			return err
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
	menu.Remark = req.Remark
	menu.Modified = nowMilli()
	return uapp.dbClient.UpdateSysMenu(menu)
}

func (uapp *userApp) SysMenuDelete(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return errort.NewCommonErr(errort.DefaultReqParamsError, fmt.Errorf("ids is empty"))
	}
	childCount, err := uapp.dbClient.SysMenuCountByParentIds(ids)
	if err != nil {
		return err
	}
	if childCount > 0 {
		return errort.NewCommonEdgeX(errort.AppMenuHasChildren, "menu has children, please delete children first", nil)
	}
	if err = uapp.dbClient.DeleteRoleMenusByMenuIds(ids); err != nil {
		return err
	}
	return uapp.dbClient.DeleteSysMenuByIds(ids)
}

// MenuRoutesByUsername 当前用户可见菜单树
func (uapp *userApp) MenuRoutesByUsername(username string) ([]dtos.SysMenuResponse, error) {
	user, err := uapp.dbClient.GetUserByUserName(username)
	if err != nil {
		return nil, err
	}
	admin := uapp.IsAdminUsername(username)
	menus, err := uapp.menusOfUser(user.Id, admin)
	if err != nil {
		return nil, err
	}
	if !admin {
		// 角色授权时可能只勾了子菜单没勾父目录，这里补全祖先目录，否则子菜单挂不到树上
		if menus, err = uapp.completeMenuAncestors(menus); err != nil {
			return nil, err
		}
	}
	return buildMenuTree(menus), nil
}

// completeMenuAncestors 补全缺失的祖先目录（仅补状态正常的），保证下发的菜单树是连通的
func (uapp *userApp) completeMenuAncestors(menus []models.Menu) ([]models.Menu, error) {
	if len(menus) == 0 {
		return menus, nil
	}
	all, err := uapp.dbClient.SysMenuAll()
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

func (uapp *userApp) menuDescendants(rootId string) ([]string, error) {
	all, err := uapp.dbClient.SysMenuAll()
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

func (uapp *userApp) checkMenusExist(menuIds []string) error {
	menus, err := uapp.dbClient.SysMenusByIds(menuIds)
	if err != nil {
		return err
	}
	if len(menus) != len(dedupStrings(menuIds)) {
		return errort.NewCommonEdgeX(errort.DefaultResourcesNotFound, "some menus not found", nil)
	}
	return nil
}

/******************************* 部门管理 *******************************/

func (uapp *userApp) SysDeptSearch(ctx context.Context, req dtos.SysDeptSearchRequest) ([]dtos.SysDeptResponse, uint32, error) {
	offset, limit := pageParams(req.Page, req.PageSize, req.IsAll)
	list, total, err := uapp.dbClient.SysDeptSearch(offset, limit, req)
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
	list, err := uapp.dbClient.SysDeptAll()
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
	dept, err := uapp.dbClient.SysDeptById(id)
	if err != nil {
		return dtos.SysDeptResponse{}, err
	}
	return toDeptResponse(dept), nil
}

func (uapp *userApp) SysDeptAdd(ctx context.Context, req dtos.SysDeptAddRequest) error {
	parentId := normalizeParentId(req.ParentId)
	ancestors := "0"
	if parentId != "0" {
		parent, err := uapp.dbClient.SysDeptById(parentId)
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
	_, err := uapp.dbClient.AddSysDept(dept)
	return err
}

func (uapp *userApp) SysDeptUpdate(ctx context.Context, req dtos.SysDeptUpdateRequest) error {
	dept, err := uapp.dbClient.SysDeptById(req.Id)
	if err != nil {
		return err
	}
	parentId := normalizeParentId(req.ParentId)
	if parentId == req.Id {
		return errort.NewCommonErr(errort.AppParentSelf, fmt.Errorf("parent dept can not be itself"))
	}
	ancestors := "0"
	if parentId != "0" {
		parent, pErr := uapp.dbClient.SysDeptById(parentId)
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
	dept.Sort = req.Sort
	dept.Leader = req.Leader
	dept.Phone = req.Phone
	dept.Email = req.Email
	dept.Status = req.Status
	dept.Modified = nowMilli()
	return uapp.dbClient.UpdateSysDept(dept)
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
	childCount, err := uapp.dbClient.SysDeptCountByParentIds(ids)
	if err != nil {
		return err
	}
	if childCount > 0 {
		return errort.NewCommonEdgeX(errort.AppDeptHasChildren, "dept has children, please delete children first", nil)
	}
	userCount, err := uapp.dbClient.SysUserCountByDeptIds(ids)
	if err != nil {
		return err
	}
	if userCount > 0 {
		return errort.NewCommonEdgeX(errort.AppDeptInUse, "dept is assigned to users", nil)
	}
	return uapp.dbClient.DeleteSysDeptByIds(ids)
}

/******************************* 登入日志 *******************************/

func (uapp *userApp) SysLoginLogSearch(ctx context.Context, req dtos.SysLoginLogSearchRequest) ([]dtos.SysLoginLogResponse, uint32, error) {
	offset, limit := pageParams(req.Page, req.PageSize, req.IsAll)
	list, total, err := uapp.dbClient.SysLoginLogSearch(offset, limit, req)
	if err != nil {
		return nil, 0, err
	}
	result := make([]dtos.SysLoginLogResponse, 0, len(list))
	for _, item := range list {
		result = append(result, toLoginLogResponse(item))
	}
	return result, total, nil
}

func (uapp *userApp) SysLoginLogDelete(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return errort.NewCommonErr(errort.DefaultReqParamsError, fmt.Errorf("ids is empty"))
	}
	return uapp.dbClient.DeleteSysLoginLogByIds(ids)
}

func (uapp *userApp) SysLoginLogClear(ctx context.Context) error {
	return uapp.dbClient.ClearSysLoginLog()
}

/******************************* 系统配置 *******************************/

func (uapp *userApp) SysConfigSearch(ctx context.Context, req dtos.SysConfigSearchRequest) ([]dtos.SysConfigResponse, uint32, error) {
	offset, limit := pageParams(req.Page, req.PageSize, req.IsAll)
	list, total, err := uapp.dbClient.SysConfigSearch(offset, limit, req)
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
	cfg, err := uapp.dbClient.SysConfigById(id)
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
	_, err := uapp.dbClient.AddSysConfig(cfg)
	return err
}

func (uapp *userApp) SysConfigUpdate(ctx context.Context, req dtos.SysConfigUpdateRequest) error {
	cfg, err := uapp.dbClient.SysConfigById(req.Id)
	if err != nil {
		return err
	}
	if exist, eErr := uapp.dbClient.SysConfigByKey(req.ConfigKey); eErr == nil && exist.Id != req.Id {
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
	return uapp.dbClient.UpdateSysConfig(cfg)
}

func (uapp *userApp) SysConfigDelete(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return errort.NewCommonErr(errort.DefaultReqParamsError, fmt.Errorf("ids is empty"))
	}
	return uapp.dbClient.DeleteSysConfigByIds(ids)
}

func (uapp *userApp) SysSetup(ctx context.Context) ([]dtos.SysConfigResponse, error) {
	list, _, err := uapp.dbClient.SysConfigSearch(0, -1, dtos.SysConfigSearchRequest{IsAll: true})
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
	return uapp.dbClient.SaveSysConfigValues(req.Items)
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
		cfg, err := uapp.dbClient.SysConfigByKey(key)
		if err != nil {
			continue // 未配置则保留默认值
		}
		if cfg.ConfigValue != "" {
			*target = cfg.ConfigValue
		}
	}
	return res, nil
}
