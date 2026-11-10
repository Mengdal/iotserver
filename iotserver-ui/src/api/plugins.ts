import { httpRequest } from '@/utils/request'
import { CommonResponse } from '@/types'

export interface PluginPackage {
  id: string
  name: string
  displayName: string
  version: string
  description: string
  category: string
  runtime: string
  arch: string
  protocols: string[]
  essential: boolean
  source: string
  packagePath: string
  packageHash: string
  updated: number
}

export interface PluginInstance {
  id: string
  name: string
  libraryId: string
  libraryName: string
  version: string
  runtime: string
  status: string
  healthStatus: string
  pid: number
  baseAddress: string
  installPath: string
  lastError: string
  config: Record<string, any>
  updated: number
}

export function uploadPluginPackage (file: File) {
  const form = new FormData()
  form.append('file', file)
  return httpRequest<any, CommonResponse<PluginPackage>>({
    url: '/api/v1/plugins/packages/upload',
    method: 'post',
    data: form,
  })
}

export function getPluginPackages () {
  return httpRequest<any, CommonResponse<PluginPackage[]>>({
    url: '/api/v1/plugins/packages',
    method: 'get',
  })
}

export function deletePluginPackage (id: string) {
  return httpRequest<any, CommonResponse<void>>({
    url: `/api/v1/plugins/packages/${id}`,
    method: 'delete',
  })
}

export function getPluginInstances () {
  return httpRequest<any, CommonResponse<PluginInstance[]>>({
    url: '/api/v1/plugins/instances',
    method: 'get',
  })
}

export function createPluginInstance (data: {
  libraryId: string
  name?: string
  config?: Record<string, any>
  autoStart?: boolean
}) {
  return httpRequest<any, CommonResponse<PluginInstance>>({
    url: '/api/v1/plugins/instances',
    method: 'post',
    data,
  })
}

export function startPluginInstance (id: string) {
  return httpRequest<any, CommonResponse<void>>({
    url: `/api/v1/plugins/instances/${id}/start`,
    method: 'post',
  })
}

export function stopPluginInstance (id: string) {
  return httpRequest<any, CommonResponse<void>>({
    url: `/api/v1/plugins/instances/${id}/stop`,
    method: 'post',
  })
}

export function restartPluginInstance (id: string) {
  return httpRequest<any, CommonResponse<void>>({
    url: `/api/v1/plugins/instances/${id}/restart`,
    method: 'post',
  })
}

export function deletePluginInstance (id: string) {
  return httpRequest<any, CommonResponse<void>>({
    url: `/api/v1/plugins/instances/${id}`,
    method: 'delete',
  })
}

export function getPluginInstanceLogs (id: string, tail = 200) {
  return httpRequest<any, CommonResponse<{ lines: string[] }>>({
    url: `/api/v1/plugins/instances/${id}/logs`,
    method: 'get',
    params: { tail },
  })
}
