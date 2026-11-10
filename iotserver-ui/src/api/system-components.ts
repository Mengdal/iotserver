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
  updated: number
}

export function getSystemComponents<T = SystemComponent[]> () {
  return httpRequest<any, CommonResponse<T>>({
    url: '/api/v1/system-components',
    method: 'get',
  })
}
