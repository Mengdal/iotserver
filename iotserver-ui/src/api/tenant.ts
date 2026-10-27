import { httpRequest } from '@/utils/request'
import { CommonResponse, TableReponseType } from '@/types'

// ===== 类型定义 =====

/** 租户：平台级数据，字段与后端 dtos.TenantResponse 对齐 */
export interface Tenant {
  id: string
  /** 租户编码（新增后不可修改） */
  code: string
  /** 租户名称 */
  name: string
  /** 状态 1启用 0停用 */
  status: number
  contactName: string
  contactPhone: string
  contactEmail: string
  /** 到期时间（毫秒），0 表示永久 */
  expireAt: number
  /** 配额，0 表示不限 */
  maxDevices: number
  maxGateways: number
  maxUsers: number
  /** 时序数据保留天数：>0 按天数，0 = 默认 30 天，<0 = 永久 */
  retentionDays: number
  remark: string
  /** things 项目绑定（未启用/未绑定时为空） */
  thingsProjectId?: number
  thingsProjectName?: string
  thingsBindStatus?: string
  thingsLastError?: string
  thingsLastSyncAt?: number
  created?: number
  modified?: number
}

// ===== 租户管理：平台级 =====

export function getTenants<T = TableReponseType<Tenant>> (params?: Record<string, any>) {
  return httpRequest<any, CommonResponse<T>>({
    url: '/api/v1/tenant',
    method: 'get',
    params,
  })
}

export function getTenantInfo<T = Tenant> (id: string) {
  return httpRequest<any, CommonResponse<T>>({
    url: `/api/v1/tenant/${id}`,
    method: 'get',
  })
}

export function addTenant<T = Tenant> (data: Record<string, any>) {
  return httpRequest<any, CommonResponse<T>>({
    url: '/api/v1/tenant',
    method: 'post',
    data,
  })
}

export function updateTenant<T = void> (data: Record<string, any>) {
  return httpRequest<any, CommonResponse<T>>({
    url: '/api/v1/tenant',
    method: 'put',
    data,
  })
}

export function deleteTenant<T = void> (id: string) {
  return httpRequest<any, CommonResponse<T>>({
    url: `/api/v1/tenant/${id}`,
    method: 'delete',
  })
}
