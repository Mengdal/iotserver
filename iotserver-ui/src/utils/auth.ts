
const USER_KEY = '$user$'
/** 平台管理员当前「代管/查看」的租户：空值代表平台视图（全部租户） */
const TENANT_KEY = '$tenant$'

export { USER_KEY, TENANT_KEY }

export const setUser = (user: any) => localStorage.setItem(USER_KEY, JSON.stringify(user))
export const getUser = () => {
  const userJson = localStorage.getItem(USER_KEY)
  if (userJson) {
    try {
      return JSON.parse(userJson)
    } catch (error) {
      console.error(error)
    }
  }
  return null
}
export const clearUser = () => {
  localStorage.removeItem(USER_KEY)
  // 退出登录时一并清掉「当前租户」，避免下个账号沿用上一个平台的代管租户
  clearCurrentTenantId()
}
export const getToken = () => getUser()?.token

/** 当前登录用户信息（登录接口返回的 user 节点） */
export const getUserInfo = () => getUser()?.user || null

/** 是否超级管理员 */
export const isAdmin = (): boolean => {
  const user = getUser()
  if (user?.user?.admin === true || user?.admin === true) return true
  return false
}

/** 是否平台管理员：只有平台管理员能看「租户管理」和切换租户 */
export const isPlatformAdmin = (): boolean => {
  const user = getUser()
  if (user?.user?.platformAdmin === true || user?.platformAdmin === true) return true
  return false
}

/** 平台管理员当前代管的租户 id；空字符串表示平台视图（全部租户） */
export const getCurrentTenantId = (): string => {
  try {
    return localStorage.getItem(TENANT_KEY) || ''
  } catch (error) {
    return ''
  }
}

export const setCurrentTenantId = (tenantId: string) => {
  if (tenantId) {
    localStorage.setItem(TENANT_KEY, tenantId)
  } else {
    localStorage.removeItem(TENANT_KEY)
  }
}

export const clearCurrentTenantId = () => localStorage.removeItem(TENANT_KEY)

/** 当前用户权限标识集合 */
export const getPermissions = (): string[] => {
  const user = getUser()
  const direct = user?.permissions
  if (Array.isArray(direct) && direct.length > 0) return direct
  const info = getUserInfo()
  return Array.isArray(info?.permissions) ? info.permissions : []
}

/** 权限判断，管理员恒为 true；支持数组或多权限（满足其一） */
export const hasPermission = (perm?: string | string[]): boolean => {
  if (!perm || (Array.isArray(perm) && perm.length === 0)) return true
  if (isAdmin()) return true
  const owned = getPermissions()
  if (owned.length === 0) return false
  const list = Array.isArray(perm) ? perm : String(perm).split(',').map(item => item.trim())
  return list.some(item => item !== '' && owned.includes(item))
}

/** 权限判断，语义化别名 */
export const useAuth = (perm?: string | string[]): boolean => hasPermission(perm)
