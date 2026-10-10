import { httpRequest } from '@/utils/request'
import { CommonResponse } from '@/types'

/** 可视化配置里可选的列 */
export interface VizViewColumn {
  name: string
  /** dimension（维度）/ measure（指标）/ time（时间列） */
  role: 'dimension' | 'measure' | 'time'
  /** 界面上不展示（技术字段/外键/密钥/大 JSON），服务端仍可用 */
  hidden?: boolean
  /** 中文名（界面上显示这个） */
  label?: string
  /** 数值格式提示：int / percent / gb / float */
  format?: string
}

/** 可视化配置可选的数据视图（后端预置，对应 shaper/data/init.sql 里的视图） */
export interface VizView {
  view: string
  label: string
  /** 该表的默认时间列：时间范围/趋势自动用它，用户不用选 */
  timeColumn?: string
  columns: VizViewColumn[]
}

/** 一个指标：列 + 聚合方式 + 显示名（"*" 表示计数） */
export interface VizMeasure {
  column: string
  agg: 'count' | 'sum' | 'avg' | 'max' | 'min'
  alias?: string
}

/**
 * 可视化配置：用户在图形界面上选的东西（拖字段/点图表图标）。
 * 后端据此生成 SQL —— 用户不需要写 SQL。
 */
export interface VizBoardSpec {
  view: string
  /** value（数值卡）/ line / bar / pie / table */
  chart: 'value' | 'line' | 'bar' | 'pie' | 'table'
  /** 维度（X 轴）列；数值卡可为空 */
  dimension?: string
  measures: VizMeasure[]
  timeColumn?: string
  /** 1h / 24h / 7d / 30d / all */
  timeRange?: string
  sortDesc?: boolean
  limit?: number
  section?: string
}

/** 画布上组件的位置与尺寸（12 列网格） */
export interface VizLayout {
  x: number
  y: number
  w: number
  h: number
}

/** 组件类型：chart（图表，默认）| text（文本） */
export type VizWidgetType = 'chart' | 'text'

/** 画布上的一个组件（= 一张卡片 + 一份可视化配置 / 一段文本） */
export interface VizWidget {
  id: string
  title: string
  showTitle?: boolean
  /** 缺省按 chart 处理（兼容已有看板） */
  type?: VizWidgetType
  /** 文本组件正文（type=text 时用） */
  text?: string
  layout: VizLayout
  /** 图表组件的可视化配置（文本组件为空） */
  spec: VizBoardSpec
}

/** 画布（kind=canvas 的看板由平台渲染） */
export interface VizCanvas {
  cols: number
  rowHeight: number
  widgets: VizWidget[]
}

export type VizBoardKind = 'canvas' | 'shaper'

export interface VizBoard {
  /** 平台内看板 ID（canvas 类型用它；legacy shaper 看板等于 shaperId） */
  id: string
  kind: VizBoardKind
  /** Shaper 里的看板 ID（legacy 类型才有） */
  shaperId?: string
  name: string
  /** 看板图标名（列表里显示） */
  icon?: string
  /** 平台登记的 SQL 副本（legacy 用） */
  sql?: string
  /** 单看板可视化配置（legacy 用） */
  spec?: VizBoardSpec
  /** 画布内容（canvas 用） */
  canvas?: VizCanvas
  /** 画布上的组件数 */
  widgetCount?: number
  registered: boolean
  isDefault: boolean
  isHome: boolean
  updateAt?: number
  createdAt?: string
}

export interface VizBoardList {
  list: VizBoard[]
  total: number
}

export interface VizQueryResult {
  id?: string
  sql: string
  columns: string[]
  /** 与 columns 一一对应：int / float / percent / bytes / date / text */
  formats?: string[]
  rows: string[][]
  error?: string
}

/** 看板详情（画布编辑器加载用） */
export interface VizBoardDetail {
  id: string
  kind: VizBoardKind
  shaperId?: string
  name: string
  icon?: string
  sql?: string
  spec?: VizBoardSpec
  canvas?: VizCanvas
  isDefault: boolean
  isHome: boolean
  updateAt?: number
}

export type VizBoardPayload =
  | { name: string, kind: 'canvas', icon?: string, canvas: VizCanvas }
  | { name: string, kind?: 'shaper', spec: VizBoardSpec }

/** 看板列表（canvas 与 legacy shaper 看板合并返回） */
export function getVizBoards<T = VizBoardList> () {
  return httpRequest<any, CommonResponse<T>>({
    url: '/api/v1/viz/boards',
    method: 'get',
  })
}

/** 单个看板详情 */
export function getVizBoard<T = VizBoardDetail> (boardId: string) {
  return httpRequest<any, CommonResponse<T>>({
    url: `/api/v1/viz/boards/${boardId}`,
    method: 'get',
  })
}

/** 新建看板（canvas：只登记；shaper：在 Shaper 侧建看板） */
export function createVizBoard<T = { id?: string, shaperId?: string }> (data: VizBoardPayload) {
  return httpRequest<any, CommonResponse<T>>({
    url: '/api/v1/viz/boards',
    method: 'post',
    data,
  })
}

/** 更新看板（canvas 直接覆盖；shaper 由后端「删旧建新」） */
export function updateVizBoard<T = { id?: string, shaperId?: string }> (boardId: string, data: VizBoardPayload) {
  return httpRequest<any, CommonResponse<T>>({
    url: `/api/v1/viz/boards/${boardId}`,
    method: 'put',
    data,
  })
}

/** 删除看板 */
export function deleteVizBoard<T = { id?: string, shaperId?: string }> (boardId: string) {
  return httpRequest<any, CommonResponse<T>>({
    url: `/api/v1/viz/boards/${boardId}`,
    method: 'delete',
  })
}

/** 设为/取消「首页卡片」或「数据看板整页」的看板 */
export function setVizBoardActive<T = any> (boardId: string, slot: 'home' | 'default', value: boolean) {
  return httpRequest<any, CommonResponse<T>>({
    url: `/api/v1/viz/boards/${boardId}/active`,
    method: 'post',
    data: { slot, value },
  })
}

/** 可视化可选的数据视图与列 */
export function getVizDatasources<T = { views: VizView[] }> () {
  return httpRequest<any, CommonResponse<T>>({
    url: '/api/v1/viz/datasources',
    method: 'get',
  })
}

export interface VizPreviewResult {
  /** 完整看板 SQL（生成的，只读展示用） */
  sql: string
  /** 预览实际执行的那条查询 */
  previewSql: string
  columns: string[]
  rows: string[][]
}

/** 按可视化配置取数（看板上单个组件实时出图用） */
export function queryViz<T = VizQueryResult> (data: { name: string, spec: VizBoardSpec }) {
  return httpRequest<any, CommonResponse<T>>({
    url: '/api/v1/viz/query',
    method: 'post',
    data,
  })
}

/** 按可视化配置批量取数（整块画布一次请求） */
export function queryVizBatch<T = { results: VizQueryResult[] }> (items: { id: string, name: string, spec: VizBoardSpec }[]) {
  return httpRequest<any, CommonResponse<T>>({
    url: '/api/v1/viz/query/batch',
    method: 'post',
    data: { items },
  })
}

/** 兼容旧调用：预览数据（后端 /viz/preview） */
export function previewVizBoard<T = VizPreviewResult> (data: { name: string, spec: VizBoardSpec }) {
  return httpRequest<any, CommonResponse<T>>({
    url: '/api/v1/viz/preview',
    method: 'post',
    data,
  })
}
