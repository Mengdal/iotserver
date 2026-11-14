import { httpRequest } from '@/utils/request'
import { CommonResponse } from '@/types'

/** 嵌入式部署下 iotpGW 的 .so 驱动 */
export interface SODriver {
  id: string
  name: string
  version: string
  protocol: string
  arch: string
  abi: string
  fileName: string
  description: string
  installed: boolean
  size: number
  hash: string
  updated: number
}

export interface SODriverMeta {
  name?: string
  version?: string
  protocol?: string
  arch?: string
  abi?: string
  description?: string
  sha256?: string
}

export function uploadSODriver (file: File, meta: SODriverMeta) {
  const form = new FormData()
  form.append('file', file)
  Object.entries(meta).forEach(([key, value]) => {
    if (value !== undefined && value !== '') form.append(key, String(value))
  })
  return httpRequest<any, CommonResponse<SODriver>>({
    url: '/api/v1/drivers/so',
    method: 'post',
    data: form,
  })
}

export function getSODrivers () {
  return httpRequest<any, CommonResponse<SODriver[]>>({
    url: '/api/v1/drivers/so',
    method: 'get',
  })
}

export function deleteSODriver (id: string) {
  return httpRequest<any, CommonResponse<void>>({
    url: `/api/v1/drivers/so/${id}`,
    method: 'delete',
  })
}
