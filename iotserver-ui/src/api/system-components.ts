import { httpRequest } from '@/utils/request'
import { CommonResponse } from '@/types'

/** 内置系统组件（Mosquitto / eKuiper 等） */
export interface SystemComponent {
  id: string
  name: string
  version: string
  description: string
  runtime: string
  essential: boolean
  /** 实际监听地址，来自平台配置 */
  endpoint: string
  /** running / stopped / unknown */
  status: string
  /** 是否配置了平台可读取的日志文件 */
  logConfigured: boolean
  /** 日志文件路径 */
  logPath: string
  updated: number
}

export interface SystemComponentLogs {
  componentId: string
  path: string
  exists: boolean
  size: number
  modified: number
  lines: string[]
}

export function getSystemComponents<T = SystemComponent[]> () {
  return httpRequest<any, CommonResponse<T>>({
    url: '/api/v1/system-components',
    method: 'get',
  })
}

export function getSystemComponentLogs<T = SystemComponentLogs> (id: string, tail = 300) {
  return httpRequest<any, CommonResponse<T>>({
    url: `/api/v1/system-components/${id}/logs`,
    method: 'get',
    params: { tail },
  })
}
