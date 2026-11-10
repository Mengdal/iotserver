import { httpRequest } from '@/utils/request'
import { CommonResponse, TableReponseType } from '@/types'

// ===== 类型定义 =====

/**
 * 网关接入平台所需的 MQTT 参数，字段与后端 dtos.GatewayMqttInfo 对齐。
 * 只会在「新增网关」的响应里返回一次，password 之后无法再次读取。
 */
export interface GatewayMqttInfo {
  /** MQTT 接入地址 */
  broker: string
  /** 客户端 ID（即网关 SN） */
  clientId: string
  username: string
  /** 接入密钥：仅返回一次，需提示用户立即保存 */
  password: string
  /** 上行 topic 约定 */
  topicUp: string
  /** 下行 topic 约定 */
  topicDown: string
}

/** 第三方工业网关，字段与后端 dtos.GatewayResponse 对齐 */
export interface Gateway {
  id: string
  name: string
  /** 网关序列号，同时是 MQTT clientId，创建后不可修改 */
  sn: string
  deptId: string
  /** 部门名：列表接口不保证返回，取不到时用 deptId 兜底 */
  deptName?: string
  vendor: string
  model: string
  /** 接入协议：gatewaymqtt（平台自有）/ edge（参考系统） */
  protocol?: string
  /** 来源：local 平台自有 / things 由 things 管理 / ioserver 软网关 */
  source?: 'local' | 'things' | 'ioserver' | string
  /** 软网关对应的采集引擎地址 */
  engineBaseUrl?: string
  /** 采集引擎超时（秒） */
  engineTimeoutSec?: number
  firmwareVersion: string
  /** 采集/上报配置（JSON 字符串） */
  config: string
  /** 配置版本号，每下发一次 +1 */
  configVersion: number
  /** 状态 0离线 1在线 2从未上线 */
  status: number
  /** 最近在线时间（毫秒），0 表示从未上线 */
  lastOnlineAt: number
  /** 最近上报时间（毫秒），0 表示无上报 */
  lastReportAt: number
  ip: string
  remark: string
  /** 已绑定设备数 */
  deviceCount: number
  created?: number
  modified?: number
  /** 接入信息：仅新增网关时返回 */
  mqtt?: GatewayMqttInfo
  /** things 绑定与同步状态 */
  thingsGatewayId?: number
  thingsProjectId?: number
  thingsSyncStatus?: string
  thingsLastSyncAt?: number
  thingsLastError?: string
  /** 网关内不接入的设备标识（逗号分隔）；删除网关子设备时会自动追加 */
  excludeDeviceKeys?: string
}

/** 可下发的网关命令 */
export type GatewayCommandType = 'restart' | 'sync'

// ===== 网关管理 =====

export function getGateways<T = TableReponseType<Gateway>> (params?: Record<string, any>) {
  return httpRequest<any, CommonResponse<T>>({
    url: '/api/v1/gateways',
    method: 'get',
    params,
  })
}

export function getGatewayInfo<T = Gateway> (id: string) {
  return httpRequest<any, CommonResponse<T>>({
    url: `/api/v1/gateway/${id}`,
    method: 'get',
  })
}

/** 新增网关：响应里一次性带回 mqtt 接入信息 */
export function addGateway<T = Gateway> (data: Record<string, any>) {
  return httpRequest<any, CommonResponse<T>>({
    url: '/api/v1/gateway',
    method: 'post',
    data,
  })
}

/** 修改网关：SN 不可改，请求体不带 sn */
export function updateGateway<T = void> (data: Record<string, any>) {
  return httpRequest<any, CommonResponse<T>>({
    url: '/api/v1/gateway',
    method: 'put',
    data,
  })
}

export function deleteGateway<T = void> (id: string) {
  return httpRequest<any, CommonResponse<T>>({
    url: `/api/v1/gateway/${id}`,
    method: 'delete',
  })
}

/** 下发命令：restart / sync（ota 已下线） */
export function sendGatewayCommand<T = void> (id: string, command: GatewayCommandType, payload?: string) {
  return httpRequest<any, CommonResponse<T>>({
    url: '/api/v1/gateway/command',
    method: 'post',
    data: { id, command, payload },
  })
}

/* ---------------------------- 参考系统（/edge）网关控制 ---------------------------- */

/** 重启参考系统网关（/edge/restart/{pk}/{sn}/notify） */
export function edgeGatewayRestart<T = void> (id: string) {
  return httpRequest<any, CommonResponse<T>>({
    url: '/api/v1/gateway/edge/restart',
    method: 'post',
    data: { id },
  })
}

/** 写设备属性（/edge/property/{pk}/{sn}/set） */
export function edgeGatewayControl<T = { status: boolean, message?: string }> (
  id: string, dn: string, identifier: string, value: string,
) {
  return httpRequest<any, CommonResponse<T>>({
    url: '/api/v1/gateway/edge/control',
    method: 'post',
    data: { id, dn, identifier, value },
  })
}

/** 拉取参考系统网关设备/测点树（/edge/scadaGetTagsTree/.../get） */
export function getEdgeGatewayDevices<T = any> (id: string) {
  return httpRequest<any, CommonResponse<T>>({
    url: '/api/v1/gateway/edge/devices',
    method: 'get',
    params: { id },
  })
}

export interface EdgeDeviceSyncResult {
  /** 网关返回的测点条目数 */
  total: number
  /** 新增 / 更新的设备数 */
  created: number
  updated: number
  /** 新增测点数 */
  tags: number
  /** 清单里已不存在、被标记摘除的设备数 */
  absent: number
  /** 命中「不接入的设备」名单而跳过的条目数 */
  excluded: number
}

/**
 * 同步参考系统网关的设备：拉取设备树并注册进设备管理。
 * 设备列表随即出现，计数与后端 dtos.EdgeDeviceSyncResult 对齐。
 */
export function syncEdgeGatewayDevices<T = EdgeDeviceSyncResult> (id: string) {
  return httpRequest<any, CommonResponse<T>>({
    url: '/api/v1/gateway/edge/sync',
    method: 'post',
    data: { id },
  })
}

export interface EdgeDeviceTag {
  /** 属性标识（不含设备名前缀），如 tag0001 */
  identifier: string
  /** 展示名，如 K1 */
  label: string
}

export interface EdgeDeviceGroup {
  /** 网关内设备标识（dn），如 user */
  dn: string
  /** 展示名，如 用户点 */
  label: string
  tags: EdgeDeviceTag[]
}
