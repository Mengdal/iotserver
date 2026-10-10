import { httpRequest } from '@/utils/request'
import { CommonResponse } from '@/types'
import { LoginUserInfo } from './sysManage'

export interface User {
  username: string
  password: string
}

export interface LoginResult {
  user: LoginUserInfo
  token: string
  expiresAt: number
  isInit: boolean
  roles: string[]
  permissions: string[]
  menus: any[]
}

export function Login (data: User) {
  return httpRequest<any, CommonResponse<LoginResult>>({
    url: '/api/v1/auth/login',
    method: 'post',
    data,
  })
}

export function changePassword <T> (data: any) {
  return httpRequest<any, CommonResponse<T>>({
    url: '/api/v1/auth/password',
    method: 'put',
    data,
  })
}

export function getInitInfo<T> () {
  return httpRequest<any, CommonResponse<T>>({
    url: '/api/v1/auth/initInfo',
    method: 'get',
  })
}

export function initPassword<T> (data: any) {
  return httpRequest<any, CommonResponse<T>>({
    url: '/api/v1/auth/init-password',
    method: 'post',
    data,
  })
}
