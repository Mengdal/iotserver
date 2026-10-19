import { httpRequest } from '@/utils/request'
import { CommonResponse } from '@/types'

/** 画布上组件的位置与尺寸（12 列网格） */
export interface VizLayout {
  x: number
  y: number
  w: number
  h: number
}

/**
 * 组件类型（后端是开放字符串，这里列已知类型）：
 *   text      文本组件
 *   container 容器组件（装子网格）
 *   stats     统计卡         ┐
 *   kpi       数值卡         │ 数据类组件：绑设备属性 / 指标，配置放在 config 里
 *   gauge     仪表盘         │
 *   line      折线图         │
 *   table     表格           ┘
 *   chart     早期图表组件：已下线，只用于识别旧记录并给占位提示
 *
 * 未知类型不会被丢弃：渲染时走注册表的兜底（提示"当前版本不支持该组件"），
 * 保存时后端也原样保留，所以新版本建的看板在旧版本上只是显示不全，不会丢数据。
 */
export type VizWidgetType =
  | 'text' | 'container' | 'chart'
  | 'stats' | 'kpi' | 'gauge' | 'line' | 'table'
  | 'rank'

/** 数据类组件绑定的一条数据序列：设备 + 属性 */
export interface VizSeriesRef {
  deviceId: string
  code: string
  /** 图例名；留空时后端补"设备名·属性名" */
  name?: string
}

/**
 * 可绑定的指标（平台/租户统计指标，如 product.total）。
 * 清单由后端下发（GET /viz/metrics），前端不维护第二份。
 */
export interface VizMetricDef {
  key: string
  label: string
  /** 单位：空串表示无单位 */
  unit?: string
  /** 小数位（缺省 0）：计数类 0 位，百分比/容量 1 位 */
  precision?: number
  /**
   * 指标类别：缺省 = 瞬时值（单个数字，给数值卡/仪表盘/表格）；
   * 'series' = 时序值（按天的趋势，取数时后端把它补进 series，给折线图这类画序列的组件）。
   */
  kind?: string
  /** 平台专属：租户视图下不展示、也不返回值（绑它的组件/块会自动隐藏） */
  platformOnly?: boolean
}

/** 一个指标的取值结果（与设备测点的 series 并列） */
export interface VizMetricValue {
  key: string
  label?: string
  /** 值为 0 也会正常返回 */
  value: number
  /** 小数位（缺省 0） */
  precision?: number
  unit?: string
}

/** 画布上的一个组件（= 一张卡片 + 一段文本 / 一个容器 / 一份数据配置） */
export interface VizWidget {
  id: string
  title: string
  showTitle?: boolean
  /** 缺省按文本处理（兼容已有记录） */
  type?: VizWidgetType | string
  /** 文本组件正文（type=text 时用） */
  text?: string
  layout: VizLayout
  /**
   * 数据类组件的配置：每个类型自己的 schema（见 src/components/viz/widgets）。
   * 后端只做默认值合并与大小校验，不认识具体字段。
   */
  config?: Record<string, any>
  /**
   * 组件自己的刷新间隔（秒）：0/缺省 = 跟随看板的 refreshInterval。
   * 看板里"秒级的当前值"和"小时级的曲线"可以并存，各自按自己的节奏取数。
   */
  refreshInterval?: number
  /** 容器组件的子网格内容（type=container 时用，同一套列坐标系） */
  children?: VizWidget[]
}

/** 画布（看板由平台渲染） */
export interface VizCanvas {
  cols: number
  rowHeight: number
  /** 整页自动刷新间隔（秒），0/缺省 = 不自动刷新；数据类组件按它轮询取数 */
  refreshInterval?: number
  widgets: VizWidget[]
}

export interface VizBoard {
  /** 平台内看板 ID */
  id: string
  name: string
  /** 看板图标名（列表里显示） */
  icon?: string
  /** 画布内容 */
  canvas?: VizCanvas
  /** 画布上的组件数（含容器里的） */
  widgetCount?: number
  isDefault: boolean
  isHome: boolean
  updateAt?: number
}

export interface VizBoardList {
  list: VizBoard[]
  total: number
}

/** 看板详情（画布编辑器加载用） */
export interface VizBoardDetail {
  id: string
  name: string
  icon?: string
  canvas?: VizCanvas
  isDefault: boolean
  isHome: boolean
  updateAt?: number
}

export type VizBoardPayload = { name: string, icon?: string, canvas: VizCanvas }

/* ---------------------------- 看板取数（设备属性） ---------------------------- */

export interface VizDataPoint {
  time: number
  value: number | string | null
}

/** 一条数据序列的查询结果 */
export interface VizDataSeries {
  deviceId: string
  deviceName?: string
  code: string
  name?: string
  unit?: string
  dataType?: string
  /** 时间窗内总条数（分页前的数量） */
  total?: number
  /** 按时间正序 */
  points: VizDataPoint[]
  error?: string
}

export interface VizDataItem {
  id: string
  series: VizDataSeries[]
  /** 指标取值（只包含已登记、且当前视图可见的指标） */
  metrics?: VizMetricValue[]
  error?: string
}

/** 一个组件的取数请求 */
export interface VizDeviceQueryItem {
  id: string
  series: VizSeriesRef[]
  /**
   * 指标 key 列表（与 series 并列，同一项二选一）：
   * 未登记的 key 后端直接忽略；platformOnly 指标在租户视图下返回空；
   * 时序指标（kind=series）取数后落在响应的 series 里，不进 metrics。
   */
  metrics?: string[]
  /** 毫秒时间戳 [from, to]，给了就以它为准 */
  range?: [number, number]
  /** 1h / 24h / 7d / 30d / all（缺省 24h） */
  timeRange?: string
  /** 聚合间隔（秒），0 = 原始点 */
  aggInterval?: number
  /** avg / max / min / sum / count / first / last */
  aggMethod?: string
  /** 单条序列最多返回点数（默认 500，上限 2000） */
  limit?: number
  /** 只取最新值（数值卡 / 仪表盘 / 表格当前值） */
  latest?: boolean
}

/** 看板列表 */
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

/** 新建看板 */
export function createVizBoard<T = { id?: string }> (data: VizBoardPayload) {
  return httpRequest<any, CommonResponse<T>>({
    url: '/api/v1/viz/boards',
    method: 'post',
    data,
  })
}

/** 更新看板 */
export function updateVizBoard<T = { id?: string }> (boardId: string, data: VizBoardPayload) {
  return httpRequest<any, CommonResponse<T>>({
    url: `/api/v1/viz/boards/${boardId}`,
    method: 'put',
    data,
  })
}

/** 删除看板 */
export function deleteVizBoard<T = { id?: string }> (boardId: string) {
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

/**
 * 看板批量取数：画布上所有数据类组件的查询一次性发给后端。
 *
 * 走这一个接口而不是每个组件各自请求，是为了：
 *   - 首屏只占一个 HTTP 连接（十几个组件会打满浏览器并发）；
 *   - 数据层多为单连接，后端集中限流比前端各查各的更稳。
 */
export function queryVizDeviceData<T = { results: VizDataItem[] }> (items: VizDeviceQueryItem[]) {
  return httpRequest<any, CommonResponse<T>>({
    url: '/api/v1/viz/device-query',
    method: 'post',
    data: { items },
  })
}

/* ---------------------------- 看板取数（指标清单） ---------------------------- */

/** 可绑定的指标清单（权限与取数一致） */
export function getVizMetrics<T = { list: VizMetricDef[] }> () {
  return httpRequest<any, CommonResponse<T>>({
    url: '/api/v1/viz/metrics',
    method: 'get',
  })
}

/**
 * 指标清单的进程内缓存：一块看板上多个组件（kpi/gauge/table 的配置面板）
 * 共用一次请求，不各拉一次。
 *
 * 失败时降级为空列表（下拉为空但不报错、也不会反复发散错误请求）；
 * 需要重新拉取（如切换租户）时调用 resetVizMetricsCache()。
 */
let vizMetricsCache: Promise<VizMetricDef[]> | null = null

export function loadVizMetrics (): Promise<VizMetricDef[]> {
  if (!vizMetricsCache) {
    vizMetricsCache = getVizMetrics()
      .then(resp => {
        const list = resp && resp.success && resp.result ? resp.result.list : undefined
        return (list || []).filter(m => m && m.key)
      })
      .catch(() => [])
  }
  return vizMetricsCache
}

/** 清掉指标清单缓存（下次使用会重新请求） */
export function resetVizMetricsCache (): void {
  vizMetricsCache = null
}

/* ---------------------------- 设备消息量排行 ---------------------------- */

export interface VizRankItem {
  deviceId: string
  deviceName: string
  /** 时间窗内的消息数（属性 + 事件 + 服务） */
  count: number
}

export interface VizRankResult {
  items: VizRankItem[]
  /** 时间窗内全部设备的消息总数（算占比用） */
  total: number
  timeRange: string
}

/** 设备消息量排行（首页看板「设备消息量排名」组件） */
export function getDeviceMessageRank (params: { timeRange?: string, limit?: number }) {
  return httpRequest<any, CommonResponse<VizRankResult>>({
    url: '/api/v1/viz/device-message-rank',
    method: 'get',
    params,
  })
}
