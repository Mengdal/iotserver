/*******************************************************************************
 * Copyright 2017.
 *
 * Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software distributed under the License
 * is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express
 * or implied. See the License for the specific language governing permissions and limitations under
 * the License.
 *******************************************************************************/

package sqlite

import (
	"fmt"
	"time"

	"gorm.io/gorm"
	"iotserver/internal/dtos"
	"iotserver/internal/models"
	"iotserver/internal/pkg/errort"
	"iotserver/internal/pkg/utils"
)

/******************************* 用户 *******************************/

func addSysUser(c *Client, u models.User) (models.User, error) {
	if len(u.Id) == 0 {
		u.Id = utils.RandomNum()
	}
	// username 已存在校验
	var count int64
	if err := c.Pool.Table(models.TableUser).Where("username = ?", u.Username).Count(&count).Error; err != nil {
		return u, errort.NewCommonEdgeX(errort.DefaultSystemError, "user query failed", err)
	}
	if count > 0 {
		return u, errort.NewCommonEdgeX(errort.DefaultNameRepeat, fmt.Sprintf("username %s exists", u.Username), nil)
	}
	if err := c.Pool.Table(u.TableName()).Create(&u).Error; err != nil {
		return u, errort.NewCommonEdgeX(errort.DefaultSystemError, "user creation failed", err)
	}
	return u, nil
}

func sysUserById(c *Client, id string) (models.User, error) {
	var u models.User
	err := c.Pool.Table(u.TableName()).Where("id = ?", id).First(&u).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return u, errort.NewCommonEdgeX(errort.DefaultResourcesNotFound, fmt.Sprintf("user %s not found", id), err)
		}
		return u, errort.NewCommonEdgeX(errort.DefaultSystemError, "user query failed", err)
	}
	return u, nil
}

func sysUserByUsername(c *Client, username string) (models.User, error) {
	var u models.User
	err := c.Pool.Table(u.TableName()).Where("username = ?", username).First(&u).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return u, errort.NewCommonEdgeX(errort.DefaultResourcesNotFound, fmt.Sprintf("username %s not found", username), err)
		}
		return u, errort.NewCommonEdgeX(errort.DefaultSystemError, "user query failed", err)
	}
	return u, nil
}

func updateSysUser(c *Client, u models.User) error {
	err := c.Pool.Table(u.TableName()).Where("id = ?", u.Id).Omit("created").Save(&u).Error
	if err != nil {
		return errort.NewCommonEdgeX(errort.DefaultSystemError, "user update failed", err)
	}
	return nil
}

func deleteSysUserByIds(c *Client, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	if err := c.Pool.Table(models.TableUser).Where("id IN ?", ids).Delete(&models.User{}).Error; err != nil {
		return errort.NewCommonEdgeX(errort.DefaultSystemError, "user delete failed", err)
	}
	return nil
}

func sysUserSearch(c *Client, offset, limit int, req dtos.SysUserSearchRequest) ([]models.User, uint32, error) {
	var (
		list  []models.User
		total int64
	)
	tx := c.Pool.Table(models.TableUser)
	if req.Username != "" {
		tx = tx.Where("username LIKE ?", "%"+req.Username+"%")
	}
	if req.Nickname != "" {
		tx = tx.Where("nickname LIKE ?", "%"+req.Nickname+"%")
	}
	if req.Phone != "" {
		tx = tx.Where("phone LIKE ?", "%"+req.Phone+"%")
	}
	if req.Status != "" {
		tx = tx.Where("status = ?", req.Status)
	}
	if req.DeptId != "" {
		tx = tx.Where("dept_id = ?", req.DeptId)
	}
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, errort.NewCommonEdgeX(errort.DefaultSystemError, "user query failed", err)
	}
	tx = tx.Order("created DESC")
	if !req.IsAll {
		tx = tx.Offset(offset).Limit(limit)
	}
	if err := tx.Find(&list).Error; err != nil {
		return nil, 0, errort.NewCommonEdgeX(errort.DefaultSystemError, "user query failed", err)
	}
	return list, uint32(total), nil
}

func sysUserCountByDeptIds(c *Client, deptIds []string) (int64, error) {
	if len(deptIds) == 0 {
		return 0, nil
	}
	var total int64
	if err := c.Pool.Table(models.TableUser).Where("dept_id IN ?", deptIds).Count(&total).Error; err != nil {
		return 0, errort.NewCommonEdgeX(errort.DefaultSystemError, "user query failed", err)
	}
	return total, nil
}

func updateSysUserLastLogin(c *Client, id string, ts int64) error {
	return c.Pool.Table(models.TableUser).Where("id = ?", id).Update("last_login", ts).Error
}

func resetSysUserPassword(c *Client, id, password string) error {
	return c.Pool.Table(models.TableUser).Where("id = ?", id).Update("password", password).Error
}

/******************************* 用户-角色 *******************************/

func getSysUserRoleIds(c *Client, userId string) ([]string, error) {
	var list []models.UserRole
	if err := c.Pool.Table(models.TableUserRole).Where("user_id = ?", userId).Find(&list).Error; err != nil {
		return nil, errort.NewCommonEdgeX(errort.DefaultSystemError, "user role query failed", err)
	}
	ids := make([]string, 0, len(list))
	for _, item := range list {
		ids = append(ids, item.RoleId)
	}
	return ids, nil
}

func setSysUserRoles(c *Client, userId string, roleIds []string) error {
	return c.Pool.Transaction(func(tx *gorm.DB) error {
		if err := tx.Table(models.TableUserRole).Where("user_id = ?", userId).Delete(&models.UserRole{}).Error; err != nil {
			return err
		}
		if len(roleIds) == 0 {
			return nil
		}
		records := make([]models.UserRole, 0, len(roleIds))
		for _, roleId := range roleIds {
			if roleId == "" {
				continue
			}
			records = append(records, models.UserRole{UserId: userId, RoleId: roleId})
		}
		if len(records) == 0 {
			return nil
		}
		return tx.Table(models.TableUserRole).Create(&records).Error
	})
}

func deleteSysUserRolesByUserIds(c *Client, userIds []string) error {
	if len(userIds) == 0 {
		return nil
	}
	return c.Pool.Table(models.TableUserRole).Where("user_id IN ?", userIds).Delete(&models.UserRole{}).Error
}

func deleteSysUserRolesByRoleIds(c *Client, roleIds []string) error {
	if len(roleIds) == 0 {
		return nil
	}
	return c.Pool.Table(models.TableUserRole).Where("role_id IN ?", roleIds).Delete(&models.UserRole{}).Error
}

func sysUserIdsByRoleId(c *Client, roleId string) ([]string, error) {
	var list []models.UserRole
	if err := c.Pool.Table(models.TableUserRole).Where("role_id = ?", roleId).Find(&list).Error; err != nil {
		return nil, errort.NewCommonEdgeX(errort.DefaultSystemError, "user role query failed", err)
	}
	ids := make([]string, 0, len(list))
	for _, item := range list {
		ids = append(ids, item.UserId)
	}
	return ids, nil
}

/******************************* 角色 *******************************/

func addSysRole(c *Client, r models.Role) (models.Role, error) {
	if len(r.Id) == 0 {
		r.Id = utils.RandomNum()
	}
	if r.Status == 0 {
		r.Status = 1
	}
	if r.DataScope == "" {
		r.DataScope = models.DataScopeAll
	}
	var count int64
	if err := c.Pool.Table(models.TableRole).Where("name = ?", r.Name).Count(&count).Error; err != nil {
		return r, errort.NewCommonEdgeX(errort.DefaultSystemError, "role query failed", err)
	}
	if count > 0 {
		return r, errort.NewCommonEdgeX(errort.DefaultNameRepeat, fmt.Sprintf("role %s exists", r.Name), nil)
	}
	if err := c.Pool.Table(r.TableName()).Create(&r).Error; err != nil {
		return r, errort.NewCommonEdgeX(errort.DefaultSystemError, "role creation failed", err)
	}
	return r, nil
}

func sysRoleById(c *Client, id string) (models.Role, error) {
	var r models.Role
	err := c.Pool.Table(r.TableName()).Where("id = ?", id).First(&r).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return r, errort.NewCommonEdgeX(errort.DefaultResourcesNotFound, fmt.Sprintf("role %s not found", id), err)
		}
		return r, errort.NewCommonEdgeX(errort.DefaultSystemError, "role query failed", err)
	}
	return r, nil
}

// sysRoleByNameInTenant / sysRoleByKeyInTenant 显式按租户查重：
// 平台视图下 GORM 租户插件不过滤，用 WithCtx 会跨租户误判，这里直接带上 tenant_id。
func sysRoleByNameInTenant(c *Client, name, tenantId string) (models.Role, error) {
	var r models.Role
	err := c.Pool.Table(r.TableName()).Where("name = ? AND tenant_id = ?", name, tenantId).First(&r).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return r, errort.NewCommonEdgeX(errort.DefaultResourcesNotFound,
				fmt.Sprintf("role %s not found in tenant %q", name, tenantId), err)
		}
		return r, errort.NewCommonEdgeX(errort.DefaultSystemError, "role query failed", err)
	}
	return r, nil
}

func sysRoleByKeyInTenant(c *Client, roleKey, tenantId string) (models.Role, error) {
	var r models.Role
	err := c.Pool.Table(r.TableName()).Where("role_key = ? AND tenant_id = ?", roleKey, tenantId).First(&r).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return r, errort.NewCommonEdgeX(errort.DefaultResourcesNotFound,
				fmt.Sprintf("role key %s not found in tenant %q", roleKey, tenantId), err)
		}
		return r, errort.NewCommonEdgeX(errort.DefaultSystemError, "role query failed", err)
	}
	return r, nil
}

func sysRoleByName(c *Client, name string) (models.Role, error) {
	var r models.Role
	err := c.Pool.Table(r.TableName()).Where("name = ?", name).First(&r).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return r, errort.NewCommonEdgeX(errort.DefaultResourcesNotFound, fmt.Sprintf("role %s not found", name), err)
		}
		return r, errort.NewCommonEdgeX(errort.DefaultSystemError, "role query failed", err)
	}
	return r, nil
}

func updateSysRole(c *Client, r models.Role) error {
	err := c.Pool.Table(r.TableName()).Where("id = ?", r.Id).Omit("created").Save(&r).Error
	if err != nil {
		return errort.NewCommonEdgeX(errort.DefaultSystemError, "role update failed", err)
	}
	return nil
}

func deleteSysRoleByIds(c *Client, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	if err := c.Pool.Table(models.TableRole).Where("id IN ?", ids).Delete(&models.Role{}).Error; err != nil {
		return errort.NewCommonEdgeX(errort.DefaultSystemError, "role delete failed", err)
	}
	return nil
}

func sysRoleSearch(c *Client, offset, limit int, req dtos.SysRoleSearchRequest) ([]models.Role, uint32, error) {
	var (
		list  []models.Role
		total int64
	)
	tx := c.Pool.Table(models.TableRole)
	if req.Name != "" {
		tx = tx.Where("name = ?", req.Name)
	}
	if req.NameLike != "" {
		tx = tx.Where("name LIKE ?", "%"+req.NameLike+"%")
	}
	if req.RoleKey != "" {
		tx = tx.Where("role_key = ?", req.RoleKey)
	}
	if req.Status != "" {
		tx = tx.Where("status = ?", req.Status)
	}
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, errort.NewCommonEdgeX(errort.DefaultSystemError, "role query failed", err)
	}
	tx = tx.Order("sort ASC, created DESC")
	if !req.IsAll {
		tx = tx.Offset(offset).Limit(limit)
	}
	if err := tx.Find(&list).Error; err != nil {
		return nil, 0, errort.NewCommonEdgeX(errort.DefaultSystemError, "role query failed", err)
	}
	return list, uint32(total), nil
}

func sysRoleAll(c *Client) ([]models.Role, error) {
	var list []models.Role
	if err := c.Pool.Table(models.TableRole).Order("sort ASC, created DESC").Find(&list).Error; err != nil {
		return nil, errort.NewCommonEdgeX(errort.DefaultSystemError, "role query failed", err)
	}
	return list, nil
}

func sysRolesByIds(c *Client, ids []string) ([]models.Role, error) {
	var list []models.Role
	if len(ids) == 0 {
		return list, nil
	}
	if err := c.Pool.Table(models.TableRole).Where("id IN ?", ids).Find(&list).Error; err != nil {
		return nil, errort.NewCommonEdgeX(errort.DefaultSystemError, "role query failed", err)
	}
	return list, nil
}

/******************************* 角色-菜单 *******************************/

func getRoleMenuIds(c *Client, roleId string) ([]string, error) {
	var list []models.RoleMenu
	if err := c.Pool.Table(models.TableRoleMenu).Where("role_id = ?", roleId).Find(&list).Error; err != nil {
		return nil, errort.NewCommonEdgeX(errort.DefaultSystemError, "role menu query failed", err)
	}
	ids := make([]string, 0, len(list))
	for _, item := range list {
		ids = append(ids, item.MenuId)
	}
	return ids, nil
}

func setRoleMenus(c *Client, roleId string, menuIds []string) error {
	return c.Pool.Transaction(func(tx *gorm.DB) error {
		if err := tx.Table(models.TableRoleMenu).Where("role_id = ?", roleId).Delete(&models.RoleMenu{}).Error; err != nil {
			return err
		}
		if len(menuIds) == 0 {
			return nil
		}
		records := make([]models.RoleMenu, 0, len(menuIds))
		for _, menuId := range menuIds {
			if menuId == "" {
				continue
			}
			records = append(records, models.RoleMenu{RoleId: roleId, MenuId: menuId})
		}
		if len(records) == 0 {
			return nil
		}
		return tx.Table(models.TableRoleMenu).Create(&records).Error
	})
}

func deleteRoleMenusByRoleIds(c *Client, roleIds []string) error {
	if len(roleIds) == 0 {
		return nil
	}
	return c.Pool.Table(models.TableRoleMenu).Where("role_id IN ?", roleIds).Delete(&models.RoleMenu{}).Error
}

func deleteRoleMenusByMenuIds(c *Client, menuIds []string) error {
	if len(menuIds) == 0 {
		return nil
	}
	return c.Pool.Table(models.TableRoleMenu).Where("menu_id IN ?", menuIds).Delete(&models.RoleMenu{}).Error
}

func roleNamesByUserId(c *Client, userId string) ([]string, error) {
	roles, err := roleListByUserId(c, userId)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(roles))
	for _, r := range roles {
		names = append(names, r.Name)
	}
	return names, nil
}

func roleListByUserId(c *Client, userId string) ([]models.Role, error) {
	var list []models.Role
	sql := fmt.Sprintf("SELECT r.* FROM %s r INNER JOIN %s ur ON ur.role_id = r.id WHERE ur.user_id = ? AND r.status = 1",
		models.TableRole, models.TableUserRole)
	if err := c.Pool.Raw(sql, userId).Scan(&list).Error; err != nil {
		return nil, errort.NewCommonEdgeX(errort.DefaultSystemError, "role query failed", err)
	}
	return list, nil
}

func roleMenusByUserId(c *Client, userId, tenantId string) ([]models.Menu, error) {
	var list []models.Menu
	// 菜单不是行级隔离表（未登记进 tenantScopedModels），原生 SQL 又绕过 GORM 插件，
	// 所以作用域条件必须手写在这里：平台视图只看 ''，租户视图看 '' + 本租户。
	tenantCond := "AND m.tenant_id = ''"
	args := []interface{}{userId}
	if tenantId != "" {
		tenantCond = "AND m.tenant_id IN ('', ?)"
		args = append(args, tenantId)
	}
	sql := fmt.Sprintf("SELECT DISTINCT m.* FROM %s m INNER JOIN %s rm ON rm.menu_id = m.id INNER JOIN %s ur ON ur.role_id = rm.role_id WHERE ur.user_id = ? AND m.status = 1 %s ORDER BY m.sort ASC",
		models.TableMenu, models.TableRoleMenu, models.TableUserRole, tenantCond)
	if err := c.Pool.Raw(sql, args...).Scan(&list).Error; err != nil {
		return nil, errort.NewCommonEdgeX(errort.DefaultSystemError, "menu query failed", err)
	}
	return list, nil
}

/******************************* 菜单 *******************************/

// menuScopeWhere 菜单作用域过滤：菜单是「全局默认 + 租户自建」表，由应用层显式拼接。
// tenantId 为空 = 平台视图，只看平台菜单；非空 = 平台菜单 + 该租户自建菜单。
func menuScopeWhere(tx *gorm.DB, tenantId string) *gorm.DB {
	if tenantId == "" {
		return tx.Where("tenant_id = ''")
	}
	return tx.Where("tenant_id IN ?", []string{"", tenantId})
}

func addSysMenu(c *Client, m models.Menu) (models.Menu, error) {
	if len(m.Id) == 0 {
		m.Id = utils.RandomNum()
	}
	if m.MenuType == "" {
		m.MenuType = models.MenuTypeMenu
	}
	if m.Status == 0 {
		m.Status = 1
	}
	if err := c.Pool.Table(m.TableName()).Create(&m).Error; err != nil {
		return m, errort.NewCommonEdgeX(errort.DefaultSystemError, "menu creation failed", err)
	}
	return m, nil
}

func sysMenuById(c *Client, id string) (models.Menu, error) {
	var m models.Menu
	err := c.Pool.Table(m.TableName()).Where("id = ?", id).First(&m).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return m, errort.NewCommonEdgeX(errort.DefaultResourcesNotFound, fmt.Sprintf("menu %s not found", id), err)
		}
		return m, errort.NewCommonEdgeX(errort.DefaultSystemError, "menu query failed", err)
	}
	return m, nil
}

func updateSysMenu(c *Client, m models.Menu) error {
	err := c.Pool.Table(m.TableName()).Where("id = ?", m.Id).Omit("created").Save(&m).Error
	if err != nil {
		return errort.NewCommonEdgeX(errort.DefaultSystemError, "menu update failed", err)
	}
	return nil
}

func deleteSysMenuByIds(c *Client, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	if err := c.Pool.Table(models.TableMenu).Where("id IN ?", ids).Delete(&models.Menu{}).Error; err != nil {
		return errort.NewCommonEdgeX(errort.DefaultSystemError, "menu delete failed", err)
	}
	return nil
}

func sysMenuSearch(c *Client, offset, limit int, tenantId string, req dtos.SysMenuSearchRequest) ([]models.Menu, uint32, error) {
	var (
		list  []models.Menu
		total int64
	)
	tx := menuScopeWhere(c.Pool.Table(models.TableMenu), tenantId)
	if req.Name != "" {
		tx = tx.Where("name = ?", req.Name)
	}
	if req.NameLike != "" {
		tx = tx.Where("name LIKE ?", "%"+req.NameLike+"%")
	}
	if req.Status != "" {
		tx = tx.Where("status = ?", req.Status)
	}
	if req.MenuType != "" {
		tx = tx.Where("menu_type = ?", req.MenuType)
	}
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, errort.NewCommonEdgeX(errort.DefaultSystemError, "menu query failed", err)
	}
	tx = tx.Order("sort ASC, created ASC")
	if !req.IsAll {
		tx = tx.Offset(offset).Limit(limit)
	}
	if err := tx.Find(&list).Error; err != nil {
		return nil, 0, errort.NewCommonEdgeX(errort.DefaultSystemError, "menu query failed", err)
	}
	return list, uint32(total), nil
}

func sysMenuAll(c *Client, tenantId string) ([]models.Menu, error) {
	var list []models.Menu
	if err := menuScopeWhere(c.Pool.Table(models.TableMenu), tenantId).
		Order("sort ASC, created ASC").Find(&list).Error; err != nil {
		return nil, errort.NewCommonEdgeX(errort.DefaultSystemError, "menu query failed", err)
	}
	return list, nil
}

func sysMenusByIds(c *Client, ids []string) ([]models.Menu, error) {
	var list []models.Menu
	if len(ids) == 0 {
		return list, nil
	}
	if err := c.Pool.Table(models.TableMenu).Where("id IN ?", ids).Find(&list).Error; err != nil {
		return nil, errort.NewCommonEdgeX(errort.DefaultSystemError, "menu query failed", err)
	}
	return list, nil
}

func sysMenusByTenant(c *Client, tenantId string) ([]models.Menu, error) {
	var list []models.Menu
	if tenantId == "" {
		return list, nil
	}
	if err := c.Pool.Table(models.TableMenu).Where("tenant_id = ?", tenantId).
		Order("sort ASC, created ASC").Find(&list).Error; err != nil {
		return nil, errort.NewCommonEdgeX(errort.DefaultSystemError, "menu query failed", err)
	}
	return list, nil
}

func sysMenuCountByParentIds(c *Client, parentIds []string) (int64, error) {
	if len(parentIds) == 0 {
		return 0, nil
	}
	var total int64
	if err := c.Pool.Table(models.TableMenu).Where("parent_id IN ?", parentIds).Count(&total).Error; err != nil {
		return 0, errort.NewCommonEdgeX(errort.DefaultSystemError, "menu query failed", err)
	}
	return total, nil
}

// sysMenuCountTenantChildrenByParentIds 统计挂在给定菜单（通常是平台菜单）下的租户自建子菜单数量
func sysMenuCountTenantChildrenByParentIds(c *Client, parentIds []string) (int64, error) {
	if len(parentIds) == 0 {
		return 0, nil
	}
	var total int64
	if err := c.Pool.Table(models.TableMenu).
		Where("parent_id IN ? AND tenant_id <> ''", parentIds).Count(&total).Error; err != nil {
		return 0, errort.NewCommonEdgeX(errort.DefaultSystemError, "menu query failed", err)
	}
	return total, nil
}

/******************************* 租户菜单展示层覆盖 *******************************/

func sysMenuPrefsByTenant(c *Client, tenantId string) ([]models.TenantMenuPref, error) {
	list := make([]models.TenantMenuPref, 0)
	if tenantId == "" {
		return list, nil
	}
	if err := c.Pool.Table(models.TableTenantMenuPref).
		Where("tenant_id = ?", tenantId).Order("menu_id ASC").Find(&list).Error; err != nil {
		return nil, errort.NewCommonEdgeX(errort.DefaultSystemError, "menu pref query failed", err)
	}
	return list, nil
}

// upsertSysMenuPref 按 (tenant_id, menu_id) 幂等写入；用「先查后写」而不是方言 upsert。
func upsertSysMenuPref(c *Client, p models.TenantMenuPref) (models.TenantMenuPref, error) {
	var exist models.TenantMenuPref
	err := c.Pool.Table(models.TableTenantMenuPref).
		Where("tenant_id = ? AND menu_id = ?", p.TenantId, p.MenuId).First(&exist).Error
	if err == gorm.ErrRecordNotFound {
		if p.Id == "" {
			p.Id = utils.RandomNum()
		}
		now := time.Now().UnixMilli()
		p.Timestamps = models.Timestamps{Created: now, Modified: now}
		if cErr := c.Pool.Table(models.TableTenantMenuPref).Create(&p).Error; cErr != nil {
			return p, errort.NewCommonEdgeX(errort.DefaultSystemError, "menu pref creation failed", cErr)
		}
		return p, nil
	}
	if err != nil {
		return p, errort.NewCommonEdgeX(errort.DefaultSystemError, "menu pref query failed", err)
	}
	updates := map[string]interface{}{
		"hidden":      p.Hidden,
		"custom_name": p.CustomName,
		"custom_icon": p.CustomIcon,
		"sort":        p.Sort,
		"modified":    time.Now().UnixMilli(),
	}
	if uErr := c.Pool.Table(models.TableTenantMenuPref).Where("id = ?", exist.Id).Updates(updates).Error; uErr != nil {
		return exist, errort.NewCommonEdgeX(errort.DefaultSystemError, "menu pref update failed", uErr)
	}
	exist.Hidden = p.Hidden
	exist.CustomName = p.CustomName
	exist.CustomIcon = p.CustomIcon
	exist.Sort = p.Sort
	return exist, nil
}

func deleteSysMenuPref(c *Client, tenantId, menuId string) error {
	if err := c.Pool.Table(models.TableTenantMenuPref).
		Where("tenant_id = ? AND menu_id = ?", tenantId, menuId).Delete(&models.TenantMenuPref{}).Error; err != nil {
		return errort.NewCommonEdgeX(errort.DefaultSystemError, "menu pref delete failed", err)
	}
	return nil
}

func deleteSysMenuPrefsByTenant(c *Client, tenantId string) (int64, error) {
	if tenantId == "" {
		return 0, nil
	}
	res := c.Pool.Table(models.TableTenantMenuPref).Where("tenant_id = ?", tenantId).Delete(&models.TenantMenuPref{})
	if res.Error != nil {
		return 0, errort.NewCommonEdgeX(errort.DefaultSystemError, "menu pref delete failed", res.Error)
	}
	return res.RowsAffected, nil
}

// deleteSysMenuPrefsByMenuIds 跨租户清理这些菜单的覆盖记录（平台删除菜单时的级联）
func deleteSysMenuPrefsByMenuIds(c *Client, menuIds []string) error {
	if len(menuIds) == 0 {
		return nil
	}
	if err := c.Pool.Table(models.TableTenantMenuPref).
		Where("menu_id IN ?", menuIds).Delete(&models.TenantMenuPref{}).Error; err != nil {
		return errort.NewCommonEdgeX(errort.DefaultSystemError, "menu pref delete failed", err)
	}
	return nil
}

/******************************* 部门 *******************************/

func addSysDept(c *Client, d models.Dept) (models.Dept, error) {
	if len(d.Id) == 0 {
		d.Id = utils.RandomNum()
	}
	if d.Status == 0 {
		d.Status = 1
	}
	if err := c.Pool.Table(d.TableName()).Create(&d).Error; err != nil {
		return d, errort.NewCommonEdgeX(errort.DefaultSystemError, "dept creation failed", err)
	}
	return d, nil
}

func sysDeptById(c *Client, id string) (models.Dept, error) {
	var d models.Dept
	err := c.Pool.Table(d.TableName()).Where("id = ?", id).First(&d).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return d, errort.NewCommonEdgeX(errort.DefaultResourcesNotFound, fmt.Sprintf("dept %s not found", id), err)
		}
		return d, errort.NewCommonEdgeX(errort.DefaultSystemError, "dept query failed", err)
	}
	return d, nil
}

func updateSysDept(c *Client, d models.Dept) error {
	err := c.Pool.Table(d.TableName()).Where("id = ?", d.Id).Omit("created").Save(&d).Error
	if err != nil {
		return errort.NewCommonEdgeX(errort.DefaultSystemError, "dept update failed", err)
	}
	return nil
}

func deleteSysDeptByIds(c *Client, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	if err := c.Pool.Table(models.TableDept).Where("id IN ?", ids).Delete(&models.Dept{}).Error; err != nil {
		return errort.NewCommonEdgeX(errort.DefaultSystemError, "dept delete failed", err)
	}
	return nil
}

func sysDeptSearch(c *Client, offset, limit int, req dtos.SysDeptSearchRequest) ([]models.Dept, uint32, error) {
	var (
		list  []models.Dept
		total int64
	)
	tx := c.Pool.Table(models.TableDept)
	if req.Name != "" {
		tx = tx.Where("name = ?", req.Name)
	}
	if req.NameLike != "" {
		tx = tx.Where("name LIKE ?", "%"+req.NameLike+"%")
	}
	if req.Status != "" {
		tx = tx.Where("status = ?", req.Status)
	}
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, errort.NewCommonEdgeX(errort.DefaultSystemError, "dept query failed", err)
	}
	tx = tx.Order("sort ASC, created ASC")
	if !req.IsAll {
		tx = tx.Offset(offset).Limit(limit)
	}
	if err := tx.Find(&list).Error; err != nil {
		return nil, 0, errort.NewCommonEdgeX(errort.DefaultSystemError, "dept query failed", err)
	}
	return list, uint32(total), nil
}

func sysDeptAll(c *Client) ([]models.Dept, error) {
	var list []models.Dept
	if err := c.Pool.Table(models.TableDept).Order("sort ASC, created ASC").Find(&list).Error; err != nil {
		return nil, errort.NewCommonEdgeX(errort.DefaultSystemError, "dept query failed", err)
	}
	return list, nil
}

func sysDeptCountByParentIds(c *Client, parentIds []string) (int64, error) {
	if len(parentIds) == 0 {
		return 0, nil
	}
	var total int64
	if err := c.Pool.Table(models.TableDept).Where("parent_id IN ?", parentIds).Count(&total).Error; err != nil {
		return 0, errort.NewCommonEdgeX(errort.DefaultSystemError, "dept query failed", err)
	}
	return total, nil
}

/******************************* 审查日志（登录 + 操作） *******************************/

func addSysAuditLog(c *Client, l models.AuditLog) error {
	if len(l.Id) == 0 {
		l.Id = utils.RandomNum()
	}
	if l.Created == 0 {
		l.Created = time.Now().UnixMilli()
	}
	if err := c.Pool.Table(l.TableName()).Create(&l).Error; err != nil {
		return errort.NewCommonEdgeX(errort.DefaultSystemError, "audit log creation failed", err)
	}
	return nil
}

// addSysAuditLogs 批量写入（同一租户作用域内的记录一次插入；空列表直接返回）
func addSysAuditLogs(c *Client, list []models.AuditLog) error {
	if len(list) == 0 {
		return nil
	}
	now := time.Now().UnixMilli()
	for i := range list {
		if len(list[i].Id) == 0 {
			list[i].Id = utils.RandomNum()
		}
		if list[i].Created == 0 {
			list[i].Created = now
		}
	}
	if err := c.Pool.Table(models.TableAuditLog).Create(&list).Error; err != nil {
		return errort.NewCommonEdgeX(errort.DefaultSystemError, "audit log batch creation failed", err)
	}
	return nil
}

func sysAuditLogSearch(c *Client, offset, limit int, req dtos.SysAuditLogSearchRequest) ([]models.AuditLog, uint32, error) {
	var (
		list  []models.AuditLog
		total int64
	)
	tx := c.Pool.Table(models.TableAuditLog)
	if req.Username != "" {
		tx = tx.Where("username LIKE ?", "%"+req.Username+"%")
	}
	if req.Type != "" {
		tx = tx.Where("type = ?", req.Type)
	}
	if req.Module != "" {
		tx = tx.Where("module = ?", req.Module)
	}
	if req.Ip != "" {
		tx = tx.Where("ip LIKE ?", "%"+req.Ip+"%")
	}
	if req.Status != "" {
		tx = tx.Where("status = ?", req.Status)
	}
	if req.StartTime > 0 {
		tx = tx.Where("created >= ?", req.StartTime)
	}
	if req.EndTime > 0 {
		tx = tx.Where("created <= ?", req.EndTime)
	}
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, errort.NewCommonEdgeX(errort.DefaultSystemError, "audit log query failed", err)
	}
	tx = tx.Order("created DESC, login_time DESC")
	if !req.IsAll {
		tx = tx.Offset(offset).Limit(limit)
	}
	if err := tx.Find(&list).Error; err != nil {
		return nil, 0, errort.NewCommonEdgeX(errort.DefaultSystemError, "audit log query failed", err)
	}
	return list, uint32(total), nil
}

func deleteSysAuditLogByIds(c *Client, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	if err := c.Pool.Table(models.TableAuditLog).Where("id IN ?", ids).Delete(&models.AuditLog{}).Error; err != nil {
		return errort.NewCommonEdgeX(errort.DefaultSystemError, "audit log delete failed", err)
	}
	return nil
}

// clearSysAuditLog 按当前租户作用域清空（必须走 GORM 删除以便租户插件追加条件）
func clearSysAuditLog(c *Client) error {
	if err := c.Pool.Table(models.TableAuditLog).Where("1 = 1").Delete(&models.AuditLog{}).Error; err != nil {
		return errort.NewCommonEdgeX(errort.DefaultSystemError, "audit log clear failed", err)
	}
	return nil
}

// deleteSysAuditLogBefore 删除 created < ts 的记录，返回删除条数（平台作用域跨租户清理）
func deleteSysAuditLogBefore(c *Client, ts int64) (int64, error) {
	res := c.Pool.Table(models.TableAuditLog).Where("created < ?", ts).Delete(&models.AuditLog{})
	if res.Error != nil {
		return 0, errort.NewCommonEdgeX(errort.DefaultSystemError, "audit log cleanup failed", res.Error)
	}
	return res.RowsAffected, nil
}

/******************************* 系统配置 *******************************/

func addSysConfig(c *Client, cfg models.SystemConfig) (models.SystemConfig, error) {
	if len(cfg.Id) == 0 {
		cfg.Id = utils.RandomNum()
	}
	if cfg.Type == "" {
		cfg.Type = "string"
	}
	var count int64
	if err := c.Pool.Table(models.TableSystemConfig).Where("config_key = ?", cfg.ConfigKey).Count(&count).Error; err != nil {
		return cfg, errort.NewCommonEdgeX(errort.DefaultSystemError, "config query failed", err)
	}
	if count > 0 {
		return cfg, errort.NewCommonEdgeX(errort.DefaultResourcesRepeat, fmt.Sprintf("config key %s exists", cfg.ConfigKey), nil)
	}
	if err := c.Pool.Table(cfg.TableName()).Create(&cfg).Error; err != nil {
		return cfg, errort.NewCommonEdgeX(errort.DefaultSystemError, "config creation failed", err)
	}
	return cfg, nil
}

func sysConfigById(c *Client, id string) (models.SystemConfig, error) {
	var cfg models.SystemConfig
	err := c.Pool.Table(cfg.TableName()).Where("id = ?", id).First(&cfg).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return cfg, errort.NewCommonEdgeX(errort.DefaultResourcesNotFound, fmt.Sprintf("config %s not found", id), err)
		}
		return cfg, errort.NewCommonEdgeX(errort.DefaultSystemError, "config query failed", err)
	}
	return cfg, nil
}

func sysConfigByKey(c *Client, key string) (models.SystemConfig, error) {
	var cfg models.SystemConfig
	err := c.Pool.Table(cfg.TableName()).Where("config_key = ?", key).First(&cfg).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return cfg, errort.NewCommonEdgeX(errort.DefaultResourcesNotFound, fmt.Sprintf("config %s not found", key), err)
		}
		return cfg, errort.NewCommonEdgeX(errort.DefaultSystemError, "config query failed", err)
	}
	return cfg, nil
}

func updateSysConfig(c *Client, cfg models.SystemConfig) error {
	err := c.Pool.Table(cfg.TableName()).Where("id = ?", cfg.Id).Omit("created").Save(&cfg).Error
	if err != nil {
		return errort.NewCommonEdgeX(errort.DefaultSystemError, "config update failed", err)
	}
	return nil
}

func deleteSysConfigByIds(c *Client, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	if err := c.Pool.Table(models.TableSystemConfig).Where("id IN ?", ids).Delete(&models.SystemConfig{}).Error; err != nil {
		return errort.NewCommonEdgeX(errort.DefaultSystemError, "config delete failed", err)
	}
	return nil
}

func sysConfigSearch(c *Client, offset, limit int, req dtos.SysConfigSearchRequest) ([]models.SystemConfig, uint32, error) {
	var (
		list  []models.SystemConfig
		total int64
	)
	tx := c.Pool.Table(models.TableSystemConfig)
	if req.ConfigKey != "" {
		tx = tx.Where("config_key LIKE ?", "%"+req.ConfigKey+"%")
	}
	if req.NameLike != "" {
		tx = tx.Where("name LIKE ?", "%"+req.NameLike+"%")
	}
	if req.GroupName != "" {
		tx = tx.Where("group_name = ?", req.GroupName)
	}
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, errort.NewCommonEdgeX(errort.DefaultSystemError, "config query failed", err)
	}
	tx = tx.Order("sort ASC, created ASC")
	if !req.IsAll {
		tx = tx.Offset(offset).Limit(limit)
	}
	if err := tx.Find(&list).Error; err != nil {
		return nil, 0, errort.NewCommonEdgeX(errort.DefaultSystemError, "config query failed", err)
	}
	return list, uint32(total), nil
}

func saveSysConfigValues(c *Client, items []dtos.SysSetupItem) error {
	return c.Pool.Transaction(func(tx *gorm.DB) error {
		for _, item := range items {
			var cfg models.SystemConfig
			err := tx.Table(models.TableSystemConfig).Where("config_key = ?", item.ConfigKey).First(&cfg).Error
			if err == gorm.ErrRecordNotFound {
				cfg = models.SystemConfig{
					Id:          utils.RandomNum(),
					ConfigKey:   item.ConfigKey,
					ConfigValue: item.ConfigValue,
					Name:        item.ConfigKey,
					GroupName:   "basic",
					Type:        "string",
				}
				if err = tx.Table(models.TableSystemConfig).Create(&cfg).Error; err != nil {
					return err
				}
				continue
			}
			if err != nil {
				return err
			}
			if err = tx.Table(models.TableSystemConfig).Where("id = ?", cfg.Id).
				Update("config_value", item.ConfigValue).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
