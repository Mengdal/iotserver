import { httpRequest } from '@/utils/request'
import { CommonResponse } from '@/types'

export interface ThingsConnectionStatus {
  baseUrl: string
  username: string
  projectCount: number
}

/** 测试 things 连接（平台管理员） */
export function testThingsConnection<T = ThingsConnectionStatus> () {
  return httpRequest<any, CommonResponse<T>>({
    url: '/api/v1/things/test',
    method: 'post',
  })
}
