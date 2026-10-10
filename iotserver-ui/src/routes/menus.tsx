import React from 'react'
import {
  DashboardOutlined,
  GlobalOutlined,
  FieldTimeOutlined,
  BookOutlined,
  SettingOutlined,
  UserOutlined,
  TeamOutlined,
  MenuOutlined,
  ApartmentOutlined,
  FileSearchOutlined,
  ToolOutlined,
  DatabaseOutlined,
} from '@ant-design/icons'
import type { MenuItemType, SubMenuType } from 'antd/es/menu/hooks/useItems'
import { renderMenuIcon } from '@/components/IconSelect'
import { USER_KEY } from '@/utils/auth'
import * as P from './permissions'

/**
 * 菜单定义
 * perm: 访问该菜单所需的权限标识；不配置表示登录即可访问
 */
export interface AppMenuItem {
  label: string
  key: string
  icon?: React.ReactNode
  perm?: string
  children?: AppMenuItem[]
}

const menus: AppMenuItem[] = [
  {
    label: '首页',
    key: '/home/dashboard',
    icon: <DashboardOutlined />,
    perm: P.HomeView,
  },
  {
    label: '设备接入',
    key: '/gateway',
    icon: <GlobalOutlined />,
    children: [
      { label: '产品管理', key: '/gateway/product/manage', perm: P.ProductList },
      { label: '设备管理', key: '/gateway/device/manage', perm: P.DeviceList },
      { label: '驱动镜像', key: '/gateway/drive/image', perm: P.ImageList },
      { label: '我的驱动', key: '/gateway/my/drive', perm: P.DriveList },
    ],
  },
  {
    label: '数据中心',
    key: '/data',
    icon: <DatabaseOutlined />,
    children: [
      { label: '历史查询', key: '/data/history', perm: P.DeviceQuery },
    ],
  },
  {
    label: '运维监控',
    key: '/monitor',
    icon: <FieldTimeOutlined />,
    children: [
      { label: '系统监控', key: '/monitor/systemMonitor', perm: P.MonitorView },
      { label: '告警中心', key: '/monitor/alarmCenter/list', perm: P.AlertList },
    ],
  },
  {
    // 顶层目录叫「系统管理」，与其下的「系统配置」菜单区分开（与后端菜单一致）
    label: '系统管理',
    key: '/admin',
    icon: <SettingOutlined />,
    children: [
      { label: '用户管理', key: '/admin/sysUser', icon: <UserOutlined />, perm: P.UserList },
      { label: '角色管理', key: '/admin/sysRole', icon: <TeamOutlined />, perm: P.RoleList },
      { label: '菜单管理', key: '/admin/sysMenu', icon: <MenuOutlined />, perm: P.MenuList },
      { label: '部门管理', key: '/admin/sysDept', icon: <ApartmentOutlined />, perm: P.DeptList },
      { label: '登入日志', key: '/admin/sysLoginLog', icon: <FileSearchOutlined />, perm: P.LoginLogList },
      { label: '系统配置', key: '/admin/sysSet', icon: <ToolOutlined />, perm: P.ConfigList },
    ],
  },
  {
    label: '工具与文档',
    key: '/documents/documents',
    icon: <BookOutlined />,
    perm: P.DocsView,
  },
]

/** 个人中心：独立入口，不加入侧边导航 */
export const profileMenu: AppMenuItem = {
  label: '个人中心',
  key: '/common/userProfile',
}

export interface BuildMenuOptions {
  /** 是否超级管理员 */
  admin?: boolean
  /** 已拥有的权限标识 */
  permissions?: string[]
}

function canAccess (item: AppMenuItem, options: BuildMenuOptions): boolean {
  if (options.admin) return true
  if (!item.perm) return true
  const owned = options.permissions || []
  return item.perm.split(',').some(perm => perm !== '' && owned.includes(perm.trim()))
}

/**
 * 前端内置菜单（兜底用）。
 * 正常情况下侧边栏由后端菜单渲染（见 buildMenusFromBackend），
 * 只有在登录态里完全没有 menus 字段（老版本登录态、接口异常）时才回落到这里。
 */
export function buildMenus (options: BuildMenuOptions = {}): Array<SubMenuType | MenuItemType> {
  const walk = (items: AppMenuItem[]): Array<SubMenuType | MenuItemType> => {
    const result: Array<SubMenuType | MenuItemType> = []
    items.forEach(item => {
      if (!canAccess(item, options)) return
      if (Array.isArray(item.children)) {
        const children = walk(item.children)
        if (children.length === 0) return
        result.push({
          label: item.label,
          key: item.key,
          icon: item.icon,
          children,
        } as SubMenuType)
        return
      }
      result.push({
        label: item.label,
        key: item.key,
        icon: item.icon,
      } as MenuItemType)
    })
    return result
  }
  return walk(menus)
}

/** 后端菜单（侧边栏数据源） */

/** 后端菜单树节点（登录接口 / 用户信息接口返回的 menus） */
export interface BackendMenuItem {
  id: string
  parentId?: string
  name: string
  path?: string
  icon?: string
  menuType?: string
  perms?: string
  visible?: number
  status?: number
  sort?: number
  isFrame?: number
  children?: BackendMenuItem[]
}

/** 后端菜单里 http(s):// 开头视为外链 */
export function isExternalLink (path?: string): boolean {
  return !!path && /^https?:\/\//i.test(path)
}

/**
 * 按后端菜单树生成侧边栏：层级、顺序、图标、显示/隐藏全部以「菜单管理」为准。
 * 过滤规则：按钮(F)不下发导航；visible=0 隐藏；status=0 停用；目录下没有可见子项则丢弃。
 */
export function buildMenusFromBackend (list?: BackendMenuItem[]): Array<SubMenuType | MenuItemType> {
  if (!Array.isArray(list) || list.length === 0) return []
  const walk = (items: BackendMenuItem[]): Array<SubMenuType | MenuItemType> => {
    const sorted = [...items].sort((a, b) => (a?.sort || 0) - (b?.sort || 0))
    const result: Array<SubMenuType | MenuItemType> = []
    sorted.forEach(item => {
      if (!item || item.menuType === 'F') return
      if (Number(item.visible) === 0 || Number(item.status) === 0) return
      const children = walk(item.children || [])
      const icon = renderMenuIcon(item.icon)
      if (children.length > 0) {
        result.push({
          label: item.name,
          key: item.path || item.id,
          icon,
          children,
        } as SubMenuType)
        return
      }
      if (!item.path) return
      result.push({
        label: item.name,
        key: item.path,
        icon,
      } as MenuItemType)
    })
    return result
  }
  return walk(list)
}

/** 后端菜单 path → 名称（标题/面包屑用） */
export function buildMenuNameMap (list?: BackendMenuItem[]): Record<string, string> {
  const map: Record<string, string> = {}
  const walk = (items?: BackendMenuItem[]) => {
    if (!Array.isArray(items)) return
    items.forEach(item => {
      if (item?.path) map[item.path] = item.name
      walk(item?.children)
    })
  }
  walk(list)
  return map
}

let cachedUserRaw: string | null = null
let cachedMenuNames: Record<string, string> = {}

/** 当前登录态里的「path → 后端菜单名」（按 localStorage 原文缓存） */
export function currentMenuNameMap (): Record<string, string> {
  let raw: string | null = null
  try {
    raw = localStorage.getItem(USER_KEY)
  } catch (error) {
    return {}
  }
  if (raw !== cachedUserRaw) {
    cachedUserRaw = raw
    let backendMenus: BackendMenuItem[] = []
    try {
      const stored = raw ? JSON.parse(raw) : null
      backendMenus = stored?.menus || stored?.user?.menus || []
    } catch (error) {
      backendMenus = []
    }
    cachedMenuNames = buildMenuNameMap(backendMenus)
  }
  return cachedMenuNames
}

/** 标题/面包屑取名：后端菜单优先，取不到再回落到前端内置映射 */
export function lookupMenuName (path: string): string | undefined {
  return currentMenuNameMap()[path] || breadcrumbNameMap[path]
}

/** 面包屑与页面标题映射 */
export function buildBreadcrumbMap (): Record<string, string> {
  const map: Record<string, string> = {}
  const loop = (arr: AppMenuItem[]) => {
    arr.forEach(item => {
      map[item.key] = item.label
      if (Array.isArray(item.children)) {
        loop(item.children)
      }
    })
  }
  loop(menus)
  map[profileMenu.key] = profileMenu.label
  return map
}

const breadcrumbNameMap: Record<string, string> = buildBreadcrumbMap()

export type { MenuItemType, SubMenuType }

export {
  menus,
  breadcrumbNameMap,
}
