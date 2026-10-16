import type { VizMetricDef, VizWidget } from '@/api'

/**
 * 平台专属内容的判定：租户视图下自动隐藏（而不是显示成空的 "--"）。
 *
 * 后端指标里的 platformOnly（系统资源 system.*、租户总数 tenant.total 等）只在平台视图下发；
 * 绑了这类指标的统计块 / 整个组件在租户视图下应该整体隐藏。
 * 判定依赖指标清单（useVizMetricDefs），所以调用方通常在渲染层用它做一次过滤。
 */

/** 指标是否平台专属（清单里带 platformOnly） */
export function isPlatformOnlyMetric (key: string | undefined, defs: Record<string, VizMetricDef>): boolean {
  const k = (key || '').trim()
  if (!k) return false
  const def = defs[k]
  return !!(def && def.platformOnly)
}

/** 组件绑定的全部指标 key：数据类组件的 config.metrics + 统计卡的 tiles（含子项） */
export function widgetMetricKeys (w: VizWidget): string[] {
  const cfg = (w.config || {}) as Record<string, any>
  const keys: string[] = []
  const push = (k: any) => {
    const v = String(k || '').trim()
    if (v) keys.push(v)
  }
  ;(cfg.metrics || []).forEach(push)
  ;(cfg.tiles || []).forEach((tile: any) => {
    if (!tile) return
    push(tile.metric)
    ;(tile.tags || []).forEach((tag: any) => push(tag && tag.metric))
  })
  return keys
}

/**
 * 是否「平台专属组件」：绑了指标、且**全部**指标都是 platformOnly。
 *
 * 只绑了部分平台专属指标时不算（例如系统监控看板里混着通用指标的那张统计卡），
 * 这种由组件自己按块隐藏（统计卡逐块判断）。
 */
export function isPlatformOnlyWidget (w: VizWidget, defs: Record<string, VizMetricDef>): boolean {
  const keys = widgetMetricKeys(w)
  if (keys.length === 0) return false
  return keys.every(k => isPlatformOnlyMetric(k, defs))
}
