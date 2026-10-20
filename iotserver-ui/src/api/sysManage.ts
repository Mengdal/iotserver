import { httpRequest } from '@/utils/request'
import { CommonResponse, TableReponseType } from '@/types'

// ===== 类型定义 =====

export interface SysUser {
  id: string
  username: string
  nickname: string
  email: string
  phone: string
  deptId: string
  deptName?: string
  status: number
  remark: string
  roleIds: string[]
  roleNames?: string[]
  lastLogin?: number
  created?: number
  modified?: number
  admin?: boolean
}

export interface SysRole {
  id: string
  name: string
  roleKey: string
  sort: number
  dataScope: string
  status: number
  remark: string
  menuIds: string[]
  created?: number
  modified?: number
}

export interface SysMenu {
  id: string
  parentId: string
  name: string
  /** 英文菜单名（英文界面优先显示） */
  nameEn?: string
  path: string
  component: string
  icon: string
  menuType: string
  perms: string
  sort: number
  visible: number
  status: number
  isFrame: number
  remark: string
  /** 菜单归属：'' = 平台原始菜单；非空 = 该租户的自建菜单 */
  tenantId?: string
  created?: number
  modified?: number
  children?: SysMenu[]
}

/** 租户菜单展示层覆盖（隐藏/改名/图标/排序） */
export interface TenantMenuPref {
  menuId: string
  hidden: number
  customName: string
  customIcon: string
  sort: number
}

export interface SysDept {
  id: string
  parentId: string
  ancestors: string
  name: string
  /** 节点类型 1公司 2工厂 3车间 4产线 */
  deptType: number
  sort: number
  leader: string
  phone: string
  email: string
  status: number
  created?: number
  modified?: number
  children?: SysDept[]
}

/** 审查日志：同时承载登录记录（type=login）与操作记录（type=operation） */
export interface SysAuditLog {
  id: string
  /** login 登录 / operation 操作 */
  type: 'login' | 'operation' | string
  username: string
  module: string
  action: string
  target: string
  method: string
  path: string
  /** 1 成功 / 0 失败 */
  status: number
  errorCode: number
  message: string
  ip: string
  browser: string
  os: string
  params: string
  durationMs: number
  created: number
  loginTime: number
}

/** @deprecated 旧「登入日志」类型，兼容别名 → SysAuditLog */
export type SysLoginLog = SysAuditLog

export interface SysConfig {
  id: string
  configKey: string
  configValue: string
  name: string
  groupName: string
  type: string
  sort: number
  remark: string
  created?: number
  modified?: number
}

export interface LoginUserInfo {
  id: string
  username: string
  nickname: string
  email: string
  phone: string
  avatar?: string
  deptId: string
  deptName: string
  lang: string
  admin: boolean
  roles: string[]
  roleIds: string[]
  permissions: string[]
  menus: SysMenu[]
}

export interface SysUserProfile {
  user: SysUser
  permissions: string[]
  roles: string[]
  admin: boolean
}

// ===== 用户管理 =====

export function getSysUsers<T = TableReponseType<SysUser>> (params?: Record<string, any>) {
  return httpRequest<any, CommonResponse<T>>({
    url: '/api/v1/sys-user',
    method: 'get',
    params,
  })
}

export function getSysUserInfo<T = SysUser> (id: string) {
  return httpRequest<any, CommonResponse<T>>({
    url: `/api/v1/sys-user/${id}`,
    method: 'get',
  })
}

export function addSysUser<T = void> (data: Record<string, any>) {
  return httpRequest<any, CommonResponse<T>>({
    url: '/api/v1/sys-user',
    method: 'post',
    data,
  })
}

export function updateSysUser<T = void> (data: Record<string, any>) {
  return httpRequest<any, CommonResponse<T>>({
    url: '/api/v1/sys-user',
    method: 'put',
    data,
  })
}

export function deleteSysUser<T = void> (ids: string[]) {
  return httpRequest<any, CommonResponse<T>>({
    url: '/api/v1/sys-user',
    method: 'delete',
    data: { ids },
  })
}

export function resetSysUserPassword<T = void> (id: string, password: string) {
  return httpRequest<any, CommonResponse<T>>({
    url: `/api/v1/pwd/reset/${id}`,
    method: 'put',
    data: { password },
  })
}

export function changeSysUserStatus<T = void> (id: string, status: number) {
  return httpRequest<any, CommonResponse<T>>({
    url: `/api/v1/sys-user/status/${id}`,
    method: 'put',
    data: { status },
  })
}

// ===== 角色管理 =====

export function getSysRoles<T = TableReponseType<SysRole>> (params?: Record<string, any>) {
  return httpRequest<any, CommonResponse<T>>({
    url: '/api/v1/role',
    method: 'get',
    params,
  })
}

export function getAllSysRoles<T = SysRole[]> () {
  return httpRequest<any, CommonResponse<T>>({
    url: '/api/v1/role/all',
    method: 'get',
  })
}

export function getSysRoleInfo<T = SysRole> (id: string) {
  return httpRequest<any, CommonResponse<T>>({
    url: `/api/v1/role/${id}`,
    method: 'get',
  })
}

export function addSysRole<T = void> (data: Record<string, any>) {
  return httpRequest<any, CommonResponse<T>>({
    url: '/api/v1/role',
    method: 'post',
    data,
  })
}

export function updateSysRole<T = void> (data: Record<string, any>) {
  return httpRequest<any, CommonResponse<T>>({
    url: '/api/v1/role',
    method: 'put',
    data,
  })
}

export function deleteSysRole<T = void> (ids: string[]) {
  return httpRequest<any, CommonResponse<T>>({
    url: '/api/v1/role',
    method: 'delete',
    data: { ids },
  })
}

export function getRoleMenus<T = { roleId: string, menuIds: string[] }> (roleId: string) {
  return httpRequest<any, CommonResponse<T>>({
    url: '/api/v1/menurole',
    method: 'get',
    params: { roleId },
  })
}

export function setRoleMenus<T = void> (roleId: string, menuIds: string[]) {
  return httpRequest<any, CommonResponse<T>>({
    url: '/api/v1/menurole',
    method: 'post',
    data: { roleId, menuIds },
  })
}

// ===== 菜单管理 =====

export function getSysMenus<T = TableReponseType<SysMenu>> (params?: Record<string, any>) {
  return httpRequest<any, CommonResponse<T>>({
    url: '/api/v1/menu',
    method: 'get',
    params,
  })
}

export function getSysMenuTree<T = SysMenu[]> (params?: Record<string, any>) {
  return httpRequest<any, CommonResponse<T>>({
    url: '/api/v1/menu/tree',
    method: 'get',
    params,
  })
}

export function getSysMenuInfo<T = SysMenu> (id: string) {
  return httpRequest<any, CommonResponse<T>>({
    url: `/api/v1/menu/${id}`,
    method: 'get',
  })
}

export function addSysMenu<T = void> (data: Record<string, any>) {
  return httpRequest<any, CommonResponse<T>>({
    url: '/api/v1/menu',
    method: 'post',
    data,
  })
}

export function updateSysMenu<T = void> (data: Record<string, any>) {
  return httpRequest<any, CommonResponse<T>>({
    url: '/api/v1/menu',
    method: 'put',
    data,
  })
}

export function deleteSysMenu<T = void> (ids: string[]) {
  return httpRequest<any, CommonResponse<T>>({
    url: '/api/v1/menu',
    method: 'delete',
    data: { ids },
  })
}

/** 当前租户的菜单展示层覆盖列表（平台视图返回空数组） */
export function getSysMenuPrefs<T = TenantMenuPref[]> () {
  return httpRequest<any, CommonResponse<T>>({
    url: '/api/v1/menu/pref',
    method: 'get',
  })
}

/**
 * 保存当前租户的菜单展示层覆盖（隐藏/改名/图标/排序）。
 * 只写覆盖层，不改菜单本体；平台视图调用会被后端拒绝。
 */
export function saveSysMenuPref<T = void> (data: {
  menuId: string
  hidden?: number
  customName?: string
  customIcon?: string
  sort?: number
}) {
  return httpRequest<any, CommonResponse<T>>({
    url: '/api/v1/menu/pref',
    method: 'put',
    data,
  })
}

/** 平台重置某租户的菜单定制：清空自建菜单与全部覆盖，恢复平台默认（仅平台管理员） */
export function resetSysMenuTenant<T = void> (tenantId: string) {
  return httpRequest<any, CommonResponse<T>>({
    url: '/api/v1/menu/tenant-reset',
    method: 'post',
    data: { tenantId },
  })
}

// ===== 部门管理 =====

export function getSysDepts<T = TableReponseType<SysDept>> (params?: Record<string, any>) {
  return httpRequest<any, CommonResponse<T>>({
    url: '/api/v1/dept',
    method: 'get',
    params,
  })
}

export function getSysDeptTree<T = SysDept[]> (params?: Record<string, any>) {
  return httpRequest<any, CommonResponse<T>>({
    url: '/api/v1/dept-tree',
    method: 'get',
    params,
  })
}

export function getSysDeptInfo<T = SysDept> (id: string) {
  return httpRequest<any, CommonResponse<T>>({
    url: `/api/v1/dept/${id}`,
    method: 'get',
  })
}

export function addSysDept<T = void> (data: Record<string, any>) {
  return httpRequest<any, CommonResponse<T>>({
    url: '/api/v1/dept',
    method: 'post',
    data,
  })
}

export function updateSysDept<T = void> (data: Record<string, any>) {
  return httpRequest<any, CommonResponse<T>>({
    url: '/api/v1/dept',
    method: 'put',
    data,
  })
}

export function deleteSysDept<T = void> (ids: string[]) {
  return httpRequest<any, CommonResponse<T>>({
    url: '/api/v1/dept',
    method: 'delete',
    data: { ids },
  })
}

// ===== 审查日志（登录 + 操作） =====

export function getSysAuditLogs<T = TableReponseType<SysAuditLog>> (params?: Record<string, any>) {
  return httpRequest<any, CommonResponse<T>>({
    url: '/api/v1/sys-audit-log',
    method: 'get',
    params,
  })
}

export function deleteSysAuditLogs<T = void> (ids: string[]) {
  return httpRequest<any, CommonResponse<T>>({
    url: '/api/v1/sys-audit-log',
    method: 'delete',
    data: { ids },
  })
}

export function clearSysAuditLogs<T = void> () {
  return httpRequest<any, CommonResponse<T>>({
    url: '/api/v1/sys-audit-log/clear',
    method: 'delete',
  })
}

// ===== 登入日志（旧接口，兼容别名 → 审查日志；后端旧路由仍保留一版） =====

export function getSysLoginLogs<T = TableReponseType<SysAuditLog>> (params?: Record<string, any>) {
  return getSysAuditLogs<T>(params)
}

export function deleteSysLoginLog<T = void> (ids: string[]) {
  return deleteSysAuditLogs<T>(ids)
}

export function clearSysLoginLog<T = void> () {
  return clearSysAuditLogs<T>()
}

// ===== 系统配置 =====

export function getSysConfigs<T = TableReponseType<SysConfig>> (params?: Record<string, any>) {
  return httpRequest<any, CommonResponse<T>>({
    url: '/api/v1/system-setup',
    method: 'get',
    params,
  })
}

export function getSysConfigInfo<T = SysConfig> (id: string) {
  return httpRequest<any, CommonResponse<T>>({
    url: `/api/v1/system-setup/${id}`,
    method: 'get',
  })
}

export function addSysConfig<T = void> (data: Record<string, any>) {
  return httpRequest<any, CommonResponse<T>>({
    url: '/api/v1/system-setup',
    method: 'post',
    data,
  })
}

export function updateSysConfig<T = void> (data: Record<string, any>) {
  return httpRequest<any, CommonResponse<T>>({
    url: '/api/v1/system-setup',
    method: 'put',
    data,
  })
}

export function deleteSysConfig<T = void> (ids: string[]) {
  return httpRequest<any, CommonResponse<T>>({
    url: '/api/v1/system-setup',
    method: 'delete',
    data: { ids },
  })
}

export function saveSysConfigs<T = void> (items: Array<{ configKey: string, configValue: string }>) {
  return httpRequest<any, CommonResponse<T>>({
    url: '/api/v1/system-setup/save',
    method: 'post',
    data: { items },
  })
}

// ===== 个人中心 =====

export function getUserProfile<T = SysUserProfile> () {
  return httpRequest<any, CommonResponse<T>>({
    url: '/api/v1/user/profile',
    method: 'get',
  })
}

export function updateUserProfile<T = void> (data: Record<string, any>) {
  return httpRequest<any, CommonResponse<T>>({
    url: '/api/v1/user/profile',
    method: 'put',
    data,
  })
}

export function getUserGetInfo<T = { menus: SysMenu[], permissions: string[], admin: boolean }> () {
  return httpRequest<any, CommonResponse<T>>({
    url: '/api/v1/user/getinfo',
    method: 'get',
  })
}

/** 站点品牌信息（免鉴权，登录页也会用） */
export interface SiteInfo {
  name: string
  logo: string
  favicon: string
  loginBackground: string
  copyright: string
  description: string
}

export function getSiteInfo<T = SiteInfo> () {
  return httpRequest<any, CommonResponse<T>>({
    url: '/api/v1/site-info',
    method: 'get',
  })
}

/** 保存当前用户的语言偏好（跟随账号） */
export function updateUserLang (lang: string) {
  return httpRequest<any, CommonResponse<any>>({
    url: '/api/v1/user/lang',
    method: 'put',
    data: { lang },
  })
}
