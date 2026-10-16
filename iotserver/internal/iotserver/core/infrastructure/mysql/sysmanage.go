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

package mysql

import (
	"iotserver/internal/dtos"
	"iotserver/internal/models"
)

/******************************* 用户 *******************************/

func (c *Client) AddSysUser(u models.User) (models.User, error) {
	return addSysUser(c, u)
}

func (c *Client) SysUserById(id string) (models.User, error) {
	return sysUserById(c, id)
}

func (c *Client) SysUserByUsername(username string) (models.User, error) {
	return sysUserByUsername(c, username)
}

func (c *Client) UpdateSysUser(u models.User) error {
	return updateSysUser(c, u)
}

func (c *Client) DeleteSysUserByIds(ids []string) error {
	return deleteSysUserByIds(c, ids)
}

func (c *Client) SysUserSearch(offset, limit int, req dtos.SysUserSearchRequest) ([]models.User, uint32, error) {
	return sysUserSearch(c, offset, limit, req)
}

func (c *Client) SysUserCountByDeptIds(deptIds []string) (int64, error) {
	return sysUserCountByDeptIds(c, deptIds)
}

func (c *Client) UpdateSysUserLastLogin(id string, ts int64) error {
	return updateSysUserLastLogin(c, id, ts)
}

func (c *Client) ResetSysUserPassword(id, password string) error {
	return resetSysUserPassword(c, id, password)
}

func (c *Client) GetUserRoleIds(userId string) ([]string, error) {
	return getSysUserRoleIds(c, userId)
}

func (c *Client) SetUserRoles(userId string, roleIds []string) error {
	return setSysUserRoles(c, userId, roleIds)
}

func (c *Client) DeleteUserRolesByUserIds(userIds []string) error {
	return deleteSysUserRolesByUserIds(c, userIds)
}

func (c *Client) DeleteUserRolesByRoleIds(roleIds []string) error {
	return deleteSysUserRolesByRoleIds(c, roleIds)
}

func (c *Client) SysUserIdsByRoleId(roleId string) ([]string, error) {
	return sysUserIdsByRoleId(c, roleId)
}

/******************************* 角色 *******************************/

func (c *Client) AddSysRole(r models.Role) (models.Role, error) {
	return addSysRole(c, r)
}

func (c *Client) SysRoleById(id string) (models.Role, error) {
	return sysRoleById(c, id)
}

func (c *Client) SysRoleByNameInTenant(name, tenantId string) (models.Role, error) {
	return sysRoleByNameInTenant(c, name, tenantId)
}

func (c *Client) SysRoleByKeyInTenant(roleKey, tenantId string) (models.Role, error) {
	return sysRoleByKeyInTenant(c, roleKey, tenantId)
}

func (c *Client) SysRoleByName(name string) (models.Role, error) {
	return sysRoleByName(c, name)
}

func (c *Client) UpdateSysRole(r models.Role) error {
	return updateSysRole(c, r)
}

func (c *Client) DeleteSysRoleByIds(ids []string) error {
	return deleteSysRoleByIds(c, ids)
}

func (c *Client) SysRoleSearch(offset, limit int, req dtos.SysRoleSearchRequest) ([]models.Role, uint32, error) {
	return sysRoleSearch(c, offset, limit, req)
}

func (c *Client) SysRoleAll() ([]models.Role, error) {
	return sysRoleAll(c)
}

func (c *Client) SysRolesByIds(ids []string) ([]models.Role, error) {
	return sysRolesByIds(c, ids)
}

func (c *Client) GetRoleMenuIds(roleId string) ([]string, error) {
	return getRoleMenuIds(c, roleId)
}

func (c *Client) SetRoleMenus(roleId string, menuIds []string) error {
	return setRoleMenus(c, roleId, menuIds)
}

func (c *Client) DeleteRoleMenusByRoleIds(roleIds []string) error {
	return deleteRoleMenusByRoleIds(c, roleIds)
}

func (c *Client) DeleteRoleMenusByMenuIds(menuIds []string) error {
	return deleteRoleMenusByMenuIds(c, menuIds)
}

func (c *Client) RoleNamesByUserId(userId string) ([]string, error) {
	return roleNamesByUserId(c, userId)
}

func (c *Client) RoleMenusByUserId(userId, tenantId string) ([]models.Menu, error) {
	return roleMenusByUserId(c, userId, tenantId)
}

/******************************* 菜单 *******************************/

func (c *Client) AddSysMenu(m models.Menu) (models.Menu, error) {
	return addSysMenu(c, m)
}

func (c *Client) SysMenuById(id string) (models.Menu, error) {
	return sysMenuById(c, id)
}

func (c *Client) UpdateSysMenu(m models.Menu) error {
	return updateSysMenu(c, m)
}

func (c *Client) DeleteSysMenuByIds(ids []string) error {
	return deleteSysMenuByIds(c, ids)
}

func (c *Client) SysMenuSearch(offset, limit int, tenantId string, req dtos.SysMenuSearchRequest) ([]models.Menu, uint32, error) {
	return sysMenuSearch(c, offset, limit, tenantId, req)
}

func (c *Client) SysMenuAll(tenantId string) ([]models.Menu, error) {
	return sysMenuAll(c, tenantId)
}

func (c *Client) SysMenusByIds(ids []string) ([]models.Menu, error) {
	return sysMenusByIds(c, ids)
}

func (c *Client) SysMenusByTenant(tenantId string) ([]models.Menu, error) {
	return sysMenusByTenant(c, tenantId)
}

func (c *Client) SysMenuCountByParentIds(parentIds []string) (int64, error) {
	return sysMenuCountByParentIds(c, parentIds)
}

func (c *Client) SysMenuCountTenantChildrenByParentIds(parentIds []string) (int64, error) {
	return sysMenuCountTenantChildrenByParentIds(c, parentIds)
}

func (c *Client) SysMenuPrefsByTenant(tenantId string) ([]models.TenantMenuPref, error) {
	return sysMenuPrefsByTenant(c, tenantId)
}

func (c *Client) UpsertSysMenuPref(p models.TenantMenuPref) (models.TenantMenuPref, error) {
	return upsertSysMenuPref(c, p)
}

func (c *Client) DeleteSysMenuPref(tenantId, menuId string) error {
	return deleteSysMenuPref(c, tenantId, menuId)
}

func (c *Client) DeleteSysMenuPrefsByTenant(tenantId string) (int64, error) {
	return deleteSysMenuPrefsByTenant(c, tenantId)
}

func (c *Client) DeleteSysMenuPrefsByMenuIds(menuIds []string) error {
	return deleteSysMenuPrefsByMenuIds(c, menuIds)
}

/******************************* 部门 *******************************/

func (c *Client) AddSysDept(d models.Dept) (models.Dept, error) {
	return addSysDept(c, d)
}

func (c *Client) SysDeptById(id string) (models.Dept, error) {
	return sysDeptById(c, id)
}

func (c *Client) UpdateSysDept(d models.Dept) error {
	return updateSysDept(c, d)
}

func (c *Client) DeleteSysDeptByIds(ids []string) error {
	return deleteSysDeptByIds(c, ids)
}

func (c *Client) SysDeptSearch(offset, limit int, req dtos.SysDeptSearchRequest) ([]models.Dept, uint32, error) {
	return sysDeptSearch(c, offset, limit, req)
}

func (c *Client) SysDeptAll() ([]models.Dept, error) {
	return sysDeptAll(c)
}

func (c *Client) SysDeptCountByParentIds(parentIds []string) (int64, error) {
	return sysDeptCountByParentIds(c, parentIds)
}

/******************************* 审查日志（登录 + 操作） *******************************/

func (c *Client) AddSysAuditLog(l models.AuditLog) error {
	return addSysAuditLog(c, l)
}

func (c *Client) AddSysAuditLogs(list []models.AuditLog) error {
	return addSysAuditLogs(c, list)
}

func (c *Client) SysAuditLogSearch(offset, limit int, req dtos.SysAuditLogSearchRequest) ([]models.AuditLog, uint32, error) {
	return sysAuditLogSearch(c, offset, limit, req)
}

func (c *Client) DeleteSysAuditLogByIds(ids []string) error {
	return deleteSysAuditLogByIds(c, ids)
}

func (c *Client) ClearSysAuditLog() error {
	return clearSysAuditLog(c)
}

func (c *Client) DeleteSysAuditLogBefore(ts int64) (int64, error) {
	return deleteSysAuditLogBefore(c, ts)
}

/******************************* 系统配置 *******************************/

func (c *Client) AddSysConfig(cfg models.SystemConfig) (models.SystemConfig, error) {
	return addSysConfig(c, cfg)
}

func (c *Client) SysConfigById(id string) (models.SystemConfig, error) {
	return sysConfigById(c, id)
}

func (c *Client) SysConfigByKey(key string) (models.SystemConfig, error) {
	return sysConfigByKey(c, key)
}

func (c *Client) UpdateSysConfig(cfg models.SystemConfig) error {
	return updateSysConfig(c, cfg)
}

func (c *Client) DeleteSysConfigByIds(ids []string) error {
	return deleteSysConfigByIds(c, ids)
}

func (c *Client) SysConfigSearch(offset, limit int, req dtos.SysConfigSearchRequest) ([]models.SystemConfig, uint32, error) {
	return sysConfigSearch(c, offset, limit, req)
}

func (c *Client) SaveSysConfigValues(items []dtos.SysSetupItem) error {
	return saveSysConfigValues(c, items)
}
