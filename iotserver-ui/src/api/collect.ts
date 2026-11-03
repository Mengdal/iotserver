import { http, httpRequest } from '@/utils/request'
import { CommonResponse } from '@/types'

/** 采集通道（采集引擎中的 Channel） */
export interface CollectChannel {
  id?: number
  name: string
  channelType: string
  protocol: string
  config: Record<string, any>
  pollIntervalMs: number
  timeoutMs: number
  enabled: boolean
}

/**
 * 通道下的 IOTP 设备绑定。
 * iotpId = 采集引擎侧的 IOTP ID；deviceId = 系统内部设备 ID（按 iotpId 映射）。
 */
export interface CollectDevice {
  iotpId: number
  channelId: number
  deviceId: string
  deviceName: string
  productId: string
  productName: string
}

/** 点表一行：标签ID / 地址 / 数据类型 / 读写 / 倍率、偏移 / 存储 / 使能 */
export interface CollectPoint {
  /** 采集引擎点位 ID；新建行为空 */
  id?: number
  tagId: string
  address: string
  dataType: string
  accessMode: string
  multiplier: number
  offset: number
  storage: boolean
  /** 是否使能；未使能的点位不下发采集引擎 */
  enabled: boolean
  sort: number
}

/** 设备点表配置；deviceId 即 iotpId（原来的 deviceId） */
export interface CollectConfig {
  deviceId: number
  deviceName: string
  productId: string
  productName: string
  channelId: number
  channelName: string
  enabled: boolean
  points: CollectPoint[]
}

export function getCollectConfig (deviceId: number) {
  return httpRequest<any, CommonResponse<CollectConfig>>({
    url: '/api/v1/collect/config',
    method: 'get',
    params: { deviceId },
  })
}

/** 采集实时值状态 */
export type CollectMonitorStatus = 'normal' | 'stale' | 'no_data' | 'disabled'

export interface CollectMonitorValue {
  tagId: string
  value: number | string | null
  time: number
  status: CollectMonitorStatus
  dataType?: string
}

export interface CollectMonitorResult {
  /** 时序库是否可用；false 时看 message */
  available: boolean
  message?: string
  items: CollectMonitorValue[]
}

/** 读取设备点位实时值 */
export function getCollectLatest (deviceId: number) {
  return httpRequest<any, CommonResponse<CollectMonitorResult>>({
    url: '/api/v1/collect/config/latest',
    method: 'get',
    params: { deviceId },
  })
}

/** 导出设备点表 Excel */
export function exportCollectPoints (deviceId: number) {
  return http({
    url: '/api/v1/collect/config/export',
    method: 'get',
    params: { deviceId },
    responseType: 'blob',
  })
}

/** 解析导入的点表 Excel，返回点位（需要再保存） */
export function importCollectPoints (deviceId: number, file: File) {
  const form = new FormData()
  form.append('deviceId', String(deviceId))
  form.append('file', file)
  return httpRequest<any, CommonResponse<CollectPoint[]>>({
    url: '/api/v1/collect/config/import',
    method: 'post',
    data: form,
  })
}

/** 按设备产品的物模型生成点表模板 */
export function getCollectTemplate (deviceId: number) {
  return httpRequest<any, CommonResponse<CollectPoint[]>>({
    url: '/api/v1/collect/config/template',
    method: 'get',
    params: { deviceId },
  })
}

export function saveCollectConfig (data: {
  deviceId: number
  enabled: boolean
  points: CollectPoint[]
}) {
  return httpRequest<any, CommonResponse<void>>({
    url: '/api/v1/collect/config',
    method: 'put',
    data,
  })
}

export function getCollectChannels () {
  return httpRequest<any, CommonResponse<CollectChannel[]>>({
    url: '/api/v1/collect/channels',
    method: 'get',
  })
}

export function saveCollectChannel (data: CollectChannel) {
  return httpRequest<any, CommonResponse<CollectChannel>>({
    url: '/api/v1/collect/channels',
    method: 'put',
    data,
  })
}

export function deleteCollectChannel (id: number) {
  return httpRequest<any, CommonResponse<void>>({
    url: `/api/v1/collect/channels/${id}`,
    method: 'delete',
  })
}

/** 所有已绑定通道的设备（采集引擎 + 系统设备的映射） */
export function getCollectDevices () {
  return httpRequest<any, CommonResponse<CollectDevice[]>>({
    url: '/api/v1/iotp_devices',
    method: 'get',
  })
}

/** 把系统内部设备绑定到通道 */
export function bindChannelDevices (channelId: number, deviceIds: string[]) {
  return httpRequest<any, CommonResponse<void>>({
    url: `/api/v1/collect/channels/${channelId}/devices`,
    method: 'post',
    data: { deviceIds },
  })
}

/** 解除设备与通道的绑定（点位挂通道，不会被删除） */
export function unbindChannelDevice (channelId: number, iotpId: number) {
  return httpRequest<any, CommonResponse<void>>({
    url: `/api/v1/collect/channels/${channelId}/devices/${iotpId}`,
    method: 'delete',
  })
}
