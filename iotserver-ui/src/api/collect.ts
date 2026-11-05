import { http, httpRequest } from '@/utils/request'
import { CommonResponse } from '@/types'

export type CollectMonitorStatus = 'normal' | 'stale' | 'no_data' | 'disabled' | 'error'

/** 采集概览中的通道汇总 */
export interface CollectOverviewChannel {
  id: number
  name: string
  channelType: string
  protocol: string
  deviceCount: number
  pointCount: number
  enabled: boolean
}

export interface CollectOverview {
  channelCount: number
  deviceCount: number
  pointCount: number
  enabledDeviceCount: number
  disabledDeviceCount: number
  channels: CollectOverviewChannel[]
}

/** 实时数据查询的一行 */
export interface CollectRealtimeValue {
  iotpId: number
  deviceName: string
  channelId: number
  channelName: string
  tagId: string
  dataType: string
  value: string
  rawValue?: number | string | null
  status: CollectMonitorStatus
  time: number
  lastWriteValue?: string
  lastWriteTime?: number
}

/** 查询采集实时数据 */
export function getCollectRealtime (params: { channelId?: number; deviceId?: number; keyword?: string }) {
  return httpRequest<any, CommonResponse<CollectRealtimeValue[]>>({
    url: '/api/v1/collect/realtime',
    method: 'get',
    params,
  })
}

/** 写入点位值 */
export function writeCollectTag (deviceId: number, tagId: string, value: string) {
  return httpRequest<any, CommonResponse<void>>({
    url: '/api/v1/collect/config/write',
    method: 'post',
    data: { deviceId, tagId, value },
  })
}

/** 采集配置总体概览 */
export function getCollectOverview () {
  return httpRequest<any, CommonResponse<CollectOverview>>({
    url: '/api/v1/collect/overview',
    method: 'get',
  })
}

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
  /** 设备级采集使能 */
  enabled: boolean
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
  /** 是否已有保存过的点位（不是物模型模板） */
  configured: boolean
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

export interface CollectMonitorValue {
  tagId: string
  value: number | string | null
  rawValue?: number | string | null
  time: number
  status: CollectMonitorStatus
  dataType?: string
  lastWriteValue?: string
  lastWriteTime?: number
}

export interface CollectMonitorResult {
  /** 时序库是否可用；false 时看 message */
  available: boolean
  message?: string
  items: CollectMonitorValue[]
}

/** 切换设备级采集使能 */
export function setCollectDeviceEnabled (deviceId: number, enabled: boolean) {
  return httpRequest<any, CommonResponse<void>>({
    url: '/api/v1/collect/config/enabled',
    method: 'put',
    data: { deviceId, enabled },
  })
}

/** 读取设备点位实时值 */
export function getCollectLatest (deviceId: number) {
  return httpRequest<any, CommonResponse<CollectMonitorResult>>({
    url: '/api/v1/collect/config/latest',
    method: 'get',
    params: { deviceId },
  })
}

/** 批量导入预览中的设备分组 */
export interface CollectImportDevicePreview {
  deviceId: number
  deviceName: string
  channelId: number
  channelName: string
  points: number
}

export interface CollectImportRowError {
  row: number
  message: string
}

export interface CollectImportPreview {
  devices: CollectImportDevicePreview[]
  errors: CollectImportRowError[]
}

export interface CollectImportDeviceError {
  deviceId: number
  deviceName: string
  message: string
}

export interface CollectImportResult {
  devices: number
  points: number
  failed: CollectImportDeviceError[]
}

/** 导出全部设备点位；channelId 可选 */
export function exportAllCollectPoints (channelId?: number) {
  return http({
    url: '/api/v1/collect/config/export-all',
    method: 'get',
    params: channelId ? { channelId } : undefined,
    responseType: 'blob',
  })
}

/** 批量导入预览：只解析校验，不写库 */
export function previewCollectImport (file: File) {
  const form = new FormData()
  form.append('file', file)
  return httpRequest<any, CommonResponse<CollectImportPreview>>({
    url: '/api/v1/collect/config/import-all/preview',
    method: 'post',
    data: form,
  })
}

/** 批量导入应用：覆盖文件中出现的设备点位 */
export function importAllCollectPoints (file: File) {
  const form = new FormData()
  form.append('file', file)
  return httpRequest<any, CommonResponse<CollectImportResult>>({
    url: '/api/v1/collect/config/import-all',
    method: 'post',
    data: form,
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
