import axios, { AxiosRequestConfig, AxiosResponse } from 'axios'
import { clearUser, getCurrentTenantId, getToken, isPlatformAdmin } from './auth'
import { message } from 'antd'

export const http = axios.create({
  timeout: 30 * 1000,
})

// 登录态失效（token 过期/无效，服务端四种情况统一返回 errorCode 10006）
//
// 这里**不能**用 `history.replace('/auth/login')`：应用走的是 createBrowserRouter，
// 那是另一套 history 实例，改了 URL 但路由不会切换，用户会停在报错页面上只看到一句错误。
// 所以直接整页跳到登录页（真实导航），并带上 ?path= 以便登录后跳回原页面。
const TOKEN_INVALID_CODE = 10006

let redirecting = false
const toLogin = () => {
  if (redirecting) return
  redirecting = true
  clearUser()
  // 已经在登录页：只清掉登录态，别再跳一次（避免自己刷自己）
  if (window.location.pathname === '/auth/login') return
  const from = window.location.pathname + window.location.search
  window.location.replace(`/auth/login?path=${encodeURIComponent(from)}`)
}

http.interceptors.request.use(
  (config) => {
    // 平台管理员切换「当前租户」后带 x-tenant-id 代管该租户（空值=平台视图，不带）。
    // 普通用户不带：后端只认 token 里的租户，前端也不该伪造。
    const tenantId = isPlatformAdmin() ? getCurrentTenantId() : ''
    config.headers = {
      ...config.headers,
      'x-token': getToken(),
      // Authorization: `Bearer ${getToken()}`,
      ...(tenantId ? { 'x-tenant-id': tenantId } : {}),
    }

    return config
  },
  (error) => {
    return Promise.reject(error)
  },
)

http.interceptors.response.use(
  (response) => {
    if (response.data && response.data.errorCode === TOKEN_INVALID_CODE) {
      // 并发请求会一起失败，提示只弹一次（redirecting 去重），并且稍等一下再跳，
      // 让用户来得及看到"为什么被登出"
      message.error(response.data.errorMsg || '登录已失效，请重新登录')
      window.setTimeout(toLogin, 600)
    }
    return response
  },
  (error) => {
    return Promise.reject(error)
  },
)

export function httpRequest <T = any, R = AxiosResponse<T>, D = any> (config: AxiosRequestConfig<D>): Promise<R> {
  return http<T, R, D>(config).then((resp: any) => {
    if (resp.statusText === 'OK' && resp.status === 200) {
      return resp.data
    }

    return resp
  })
}
