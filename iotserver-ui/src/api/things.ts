import { httpRequest } from '@/utils/request'
import { CommonResponse } from '@/types'

export interface ThingsProject {
  id: number
  name: string
  description?: string
  createTime?: string
}

export interface ThingsConnectionStatus {
  baseUrl: string
  username: string
  projectCount: number
}

export interface ThingsReconcileResult {
  tenants: number
  gateways: number
  missing: number
  failed: number
}

/** 测试 things 连接（平台管理员） */
export function testThingsConnection<T = ThingsConnectionStatus> () {
  return httpRequest<any, CommonResponse<T>>({
    url: '/api/v1/things/test',
    method: 'post',
  })
}

/** things 项目列表（平台管理员，绑定存量租户时使用） */
export function getThingsProjects<T = ThingsProject[]> (keyword?: string) {
  return httpRequest<any, CommonResponse<T>>({
    url: '/api/v1/things/projects',
    method: 'get',
    params: keyword ? { keyword } : undefined,
  })
}

/** 全量对账 */
export function reconcileThings<T = ThingsReconcileResult> () {
  return httpRequest<any, CommonResponse<T>>({
    url: '/api/v1/things/reconcile',
    method: 'post',
  })
}

/** 为存量租户绑定已有项目（projectId）或创建新项目（name） */
export function bindTenantThingsProject<T = void> (
  tenantId: string,
  data: { projectId?: number, name?: string, description?: string },
) {
  return httpRequest<any, CommonResponse<T>>({
    url: `/api/v1/tenant/${tenantId}/things-project`,
    method: 'post',
    data,
  })
}

/** 重命名租户绑定的 things 项目 */
export function renameTenantThingsProject<T = void> (tenantId: string, name: string) {
  return httpRequest<any, CommonResponse<T>>({
    url: `/api/v1/tenant/${tenantId}/things-project`,
    method: 'put',
    data: { name },
  })
}

/** 对当前租户执行一次 things 网关对账 */
export function reconcileTenantThings<T = ThingsReconcileResult> () {
  return httpRequest<any, CommonResponse<T>>({
    url: '/api/v1/gateway/things-reconcile',
    method: 'post',
  })
}
