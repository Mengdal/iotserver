import type React from 'react'
import type { VizDataItem, VizDeviceQueryItem } from '@/api'

/**
 * 看板组件契约（widget manifest）
 *
 * 与 OpenRemote 的 `or-dashboard-builder` 是同一个套路：每个组件类型自己声明
 *   「叫什么 / 图标 / 默认尺寸 / 默认配置 / 怎么渲染 / 怎么配置 / 怎么取数」，
 * 由注册表（registry.ts）统一登记，看板编辑器与展示页只认注册表、不认识具体组件。
 *
 * 这样新增一种组件只需要加一个文件（写 manifest + 在 registry.ts 里 import 一次），
 * 不用再改编辑器、组件库、默认尺寸三处。
 */

/** 组件渲染上下文：由看板（编辑器 / 展示页）注入 */
export interface VizWidgetContext {
  /** 编辑态（画布编辑器里） */
  editable: boolean
  /** 只读（查看模式 / 数据看板页） */
  readOnly: boolean
  /** 该组件的取数结果（数据类组件才有） */
  data?: VizDataItem
  /** 该组件正在取数 */
  loading?: boolean
  /**
   * 文本组件正文与写回（text 是看板组件的字段、不属于 config，
   * 所以单独放在上下文里，避免把文本组件也塞进自由 config）
   */
  text?: string
  onTextChange?: (text: string) => void
  /** 编辑态：把 config 的局部改动写回看板（组件内部的小控件用，如阈值调色） */
  onConfigChange?: (patch: Record<string, any>) => void
  /**
   * 该组件生效的自动刷新间隔（秒，0 = 不自动刷新）。
   * 数据类组件由 useVizData 统一取数，不需要这个字段；
   * 自带独立取数接口的组件（如排行榜）据此自行轮询。
   */
  refreshInterval?: number
}

export interface VizWidgetRenderProps<C = any> {
  /** 组件 id（一般用不到，取数结果在 ctx.data 里） */
  id: string
  config: C
  ctx: VizWidgetContext
}

export interface VizWidgetSettingsProps<C = any> {
  config: C
  /** 局部更新配置（会写回看板并标脏） */
  onChange: (patch: Partial<C>) => void
}

export interface VizWidgetManifest<C = any> {
  /** 类型 id，与后端 vizWidgetRegistry 的 key 对应 */
  type: string
  /** 组件库与默认标题里显示的名字 */
  label: string
  icon: React.ReactNode
  /** 从组件库新建时的默认尺寸（列 / 行） */
  defaultSize: { w: number; h: number }
  /** 最小尺寸（透传给 gridstack 的 minW/minH） */
  minSize?: { w: number; h: number }
  /** 布局类组件（容器）：渲染由看板编辑器负责，这里只登记元信息 */
  layoutOnly?: boolean
  /** 是否数据类组件（参与批量取数） */
  data?: boolean
  defaultConfig: () => C
  render: React.FC<VizWidgetRenderProps<C>>
  /** 配置面板（编辑态右侧抽屉）；没有配置项的组件可以不实现 */
  Settings?: React.FC<VizWidgetSettingsProps<C>>
  /**
   * 数据类组件必填：把 config 翻译成取数参数。
   * 返回空 series 时该组件不参与本次取数（例如还没绑定设备）。
   */
  buildQuery?: (config: C) => Omit<VizDeviceQueryItem, 'id'>
}
