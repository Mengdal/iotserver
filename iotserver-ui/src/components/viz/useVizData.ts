import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { queryVizDeviceData, VizDataItem, VizDeviceQueryItem, VizWidget } from '@/api'
import { getWidgetManifest } from './registry'
import { t } from '@/i18n'

export interface VizDataState {
  /** 组件 id → 取数结果 */
  results: Record<string, VizDataItem>
  loading: boolean
  error: string
  /** 手动刷新（编辑器的「刷新数据」按钮用它） */
  refresh: () => void
}

type QueryItem = Omit<VizDeviceQueryItem, 'id'>

interface PlannedItem {
  id: string
  query: QueryItem
  /** 生效的刷新间隔（秒），0 = 不自动刷新 */
  intervalSec: number
}

/**
 * 生效刷新间隔：组件自己配了就用组件的，否则跟随看板。
 * 这样一块看板上"秒级的当前值"和"小时级的曲线"可以并存，各自按自己的节奏取数。
 */
export function effectiveInterval (widgetSec: number | undefined, boardSec: number): number {
  if (widgetSec && widgetSec > 0) return widgetSec
  return boardSec > 0 ? boardSec : 0
}

/**
 * 看板取数：把画布上所有数据类组件的查询合并成一次请求。
 *
 * - 只认注册表：组件 manifest 的 buildQuery 决定"查什么、要不要 latest、时间范围与聚合"；
 * - 用「查询计划签名」判重：拖动布局会不停产生新的 widgets 数组，但签名不变就不重新取数；
 * - 刷新按**组件**调度：每秒检查一次哪些组件到期，把同一轮到期的合并成一次请求（静默，不显示 loading）。
 */
export function useVizData (
  widgets: VizWidget[],
  boardIntervalSec = 0,
  enabled = true,
): VizDataState {
  const [results, setResults] = useState<Record<string, VizDataItem>>({})
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')

  // 查询计划：绑定完整（设备 + 测点，或至少一个指标）的组件才参与
  const plan = useMemo<PlannedItem[]>(() => {
    const items: PlannedItem[] = []
    widgets.forEach((w) => {
      const manifest = getWidgetManifest(w.type)
      if (!manifest || !manifest.data || !manifest.buildQuery) return
      const query = manifest.buildQuery(w.config || {})
      const usable = (query.series || []).filter(s => s.deviceId && s.code)
      // 指标是聚合值：只要登记了 key 就参与取数（key 是否有效由后端白名单决定）
      const metrics = (query.metrics || []).map(m => String(m || '').trim()).filter(Boolean)
      if (usable.length === 0 && metrics.length === 0) return
      items.push({
        id: w.id,
        query: {
          ...query,
          series: usable,
          // 没绑指标时不带这个字段：历史看板的查询计划签名保持不变
          metrics: metrics.length > 0 ? metrics : undefined,
        },
        intervalSec: effectiveInterval(w.refreshInterval, boardIntervalSec),
      })
    })
    return items
  }, [widgets, boardIntervalSec])

  // 签名用于判重：配置或绑定变了才整体重新取数（纯改刷新频率不该触发取数）
  const signature = useMemo(
    () => JSON.stringify(plan.map(p => [p.id, p.query])),
    [plan],
  )
  const planRef = useRef(plan)
  planRef.current = plan
  // 每个组件最近一次取数成功的时间，用于判断到期
  const lastFetchRef = useRef<Record<string, number>>({})

  const request = useCallback(async (items: PlannedItem[], silent: boolean, replace: boolean) => {
    if (items.length === 0) {
      if (replace) {
        setResults({})
        setError('')
      }
      setLoading(false)
      return
    }
    // 先打时间戳再发请求：不论成功、失败还是抛异常，都按各自间隔重试
    // （否则失败的组件会因为 lastFetch 一直是 0 而被每秒重试一次）
    const startedAt = Date.now()
    items.forEach((p) => { lastFetchRef.current[p.id] = startedAt })
    if (!silent) setLoading(true)
    try {
      const resp = await queryVizDeviceData(items.map(p => ({ id: p.id, ...p.query })))
      if (resp.success) {
        const next: Record<string, VizDataItem> = {}
        ;((resp.result && resp.result.results) || []).forEach((item) => {
          if (item && item.id) next[item.id] = item
        })
        // 整体重取就替换（顺带清掉已删除组件的旧数据），按到期补取就合并
        setResults(prev => (replace ? next : { ...prev, ...next }))
        setError('')
      } else {
        setError(resp.errorMsg || t('取数失败'))
      }
    } catch (e: any) {
      setError(e?.message || t('取数失败'))
    } finally {
      if (!silent) setLoading(false)
    }
  }, [])

  const loadAll = useCallback((silent: boolean) => request(planRef.current, silent, true), [request])

  // 查询计划变化（绑定了新设备 / 改了时间范围）→ 整体重新取数
  useEffect(() => {
    if (!enabled) return
    loadAll(false)
  }, [signature, enabled, loadAll])

  const hasAutoRefresh = plan.some(p => p.intervalSec > 0)

  // 组件级调度：每秒挑出到期的组件，合成一次请求
  useEffect(() => {
    if (!enabled || !hasAutoRefresh) return undefined
    const timer = window.setInterval(() => {
      const now = Date.now()
      const due = planRef.current.filter(p => (
        p.intervalSec > 0 && now - (lastFetchRef.current[p.id] || 0) >= p.intervalSec * 1000
      ))
      if (due.length > 0) request(due, true, false)
    }, 1000)
    return () => window.clearInterval(timer)
  }, [enabled, hasAutoRefresh, request])

  const refresh = useCallback(() => { loadAll(false) }, [loadAll])

  return { results, loading, error, refresh }
}
