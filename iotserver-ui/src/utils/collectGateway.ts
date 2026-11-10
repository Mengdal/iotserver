/**
 * 当前正在配置的软网关。
 *
 * 采集配置页从网关管理的右侧抽屉打开，页面里的接口都需要带上 gatewayId。
 * 用 sessionStorage 保存，抽屉关闭时清空；请求拦截器只对采集相关接口附加请求头。
 */
const STORAGE_KEY = 'collect-gateway'

export function setCollectGatewayId (gatewayId?: string) {
  if (gatewayId) {
    window.sessionStorage.setItem(STORAGE_KEY, gatewayId)
  } else {
    window.sessionStorage.removeItem(STORAGE_KEY)
  }
}

export function getCollectGatewayId (): string {
  return window.sessionStorage.getItem(STORAGE_KEY) || ''
}
