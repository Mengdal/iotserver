import type { VizWidget } from '@/api'
import type { VizWidgetManifest } from './types'
import { textManifest } from './widgets/TextWidget'
import { containerManifest, legacyChartManifest } from './widgets/misc'
import { statsManifest } from './widgets/StatsWidget'
import { kpiManifest } from './widgets/KpiWidget'
import { gaugeManifest } from './widgets/GaugeWidget'
import { lineManifest } from './widgets/LineWidget'
import { tableManifest } from './widgets/TableWidget'
import { rankManifest } from './widgets/RankWidget'

/**
 * 看板组件注册表
 *
 * 新增一种组件 = 新建一个 widget 文件（导出 manifest）+ 在这里 import 一行登记。
 * 组件库、默认标题、配置面板、默认尺寸、取数都由 manifest 驱动，不需要再改编辑器。
 */
const manifests: VizWidgetManifest[] = [
  textManifest,
  containerManifest,
  statsManifest,
  kpiManifest,
  gaugeManifest,
  lineManifest,
  tableManifest,
  rankManifest,
  // 已下线类型放在最后：只用于打开老看板时给占位提示
  legacyChartManifest,
]

const registry = new Map<string, VizWidgetManifest>(
  manifests.map(m => [m.type, m]),
)

/** 按类型取 manifest；未知类型返回 undefined，由调用方渲染兜底提示 */
export function getWidgetManifest (type?: string): VizWidgetManifest | undefined {
  return registry.get(String(type || DEFAULT_WIDGET_TYPE).toLowerCase())
}

/** 全部已登记类型（顺序即组件库顺序） */
export function allWidgetManifests (): VizWidgetManifest[] {
  return manifests
}

/** 组件库里可新建的组件：排除已下线类型（容器带 layoutOnly 标记，渲染由编辑器负责） */
export function paletteManifests (): VizWidgetManifest[] {
  return manifests.filter(m => m.type !== legacyChartManifest.type)
}

/** 是否是数据类组件（要参与批量取数） */
export function isDataWidget (w: VizWidget): boolean {
  const manifest = getWidgetManifest(w.type)
  return !!(manifest && manifest.data && manifest.buildQuery)
}

/** 缺省类型：老记录没有 type 字段时按文本处理 */
export const DEFAULT_WIDGET_TYPE = 'text'
