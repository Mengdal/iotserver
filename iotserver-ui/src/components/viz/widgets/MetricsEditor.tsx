import React, { useEffect, useMemo, useState } from 'react'
import { Empty, Segmented, Select } from 'antd'
import { loadVizMetrics, VizMetricDef, VizSeriesRef } from '@/api'
import { isPlatformView } from '@/utils/platformViewGuard'
import SeriesEditor from './SeriesEditor'
import { SettingRow } from './SettingsLayout'

export type VizBindMode = 'series' | 'metrics'

/**
 * 指标类别：
 *   - instant 瞬时值（单个数字）——数值卡 / 仪表盘 / 表格；
 *   - series  时序值（按天的趋势）——折线图这类画序列的组件（取数时后端补进 series）。
 * 两类互不通用，下拉按类别过滤，避免绑出一个画不出来的组合。
 */
export type VizMetricKind = 'instant' | 'series'

/**
 * 指标清单（后端 GET /viz/metrics 下发，api 层做了进程内缓存/去重）：
 * 失败时降级为空下拉——不报错、不崩、也不会反复发请求。
 */
function useVizMetricList (): { list: VizMetricDef[], loading: boolean } {
  const [list, setList] = useState<VizMetricDef[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    let alive = true
    loadVizMetrics()
      .then((all) => { if (alive) setList(all || []) })
      .catch(() => { if (alive) setList([]) })
      .finally(() => { if (alive) setLoading(false) })
    return () => { alive = false }
  }, [])

  return { list, loading }
}

/**
 * 指标下拉选项。
 * 平台专属指标（platformOnly）在租户视图下不展示；后端在租户视图下本来也不下发，
 * 这里再兜一层，避免切换视图后拿到缓存里的旧清单。
 */
export function useVizMetricsOptions (kind: VizMetricKind = 'instant'): { options: { label: string, value: string }[], loading: boolean } {
  const { list, loading } = useVizMetricList()
  const options = useMemo(() => (list || [])
    .filter(m => m && m.key && (!m.platformOnly || isPlatformView()))
    .filter(m => (kind === 'series' ? m.kind === 'series' : m.kind !== 'series'))
    .map(m => ({ label: m.label ? `${m.label}（${m.key}）` : m.key, value: m.key })), [list, kind])

  return { options, loading }
}

/**
 * 指标定义表（key → 定义）：platformOnly / 小数位等元信息用。
 * 序列组件与统计卡在租户视图下要靠它隐藏平台专属内容，见 components/viz/platformScope。
 */
export function useVizMetricDefs (): Record<string, VizMetricDef> {
  const { list } = useVizMetricList()
  return useMemo(() => {
    const map: Record<string, VizMetricDef> = {}
    ;(list || []).forEach((m) => { if (m && m.key) map[m.key] = m })
    return map
  }, [list])
}

export interface MetricsEditorProps {
  /** config.metrics */
  value?: string[]
  onChange: (next: string[]) => void
  /** 只允许一个指标（数值卡 / 仪表盘） */
  single?: boolean
  /** 指标类别（缺省瞬时值）；折线图这类序列组件传 'series' */
  kind?: VizMetricKind
  disabled?: boolean
}

/**
 * 指标选择器：选项来自后端清单（label + key）。
 * 单值组件（kpi/gauge）用单选，表格可多选；kind 决定只列瞬时值还是时序值。
 */
export const MetricsEditor: React.FC<MetricsEditorProps> = ({ value, onChange, single, kind, disabled }) => {
  const { options, loading } = useVizMetricsOptions(kind)
  const selected = value || []
  const placeholder = kind === 'series'
    ? '选择指标（按天趋势）'
    : (single ? '选择指标' : '选择指标（可多选）')
  return (
    <Select
      style={{ width: '100%' }}
      mode={single ? undefined : 'multiple'}
      allowClear
      showSearch
      optionFilterProp="label"
      placeholder={placeholder}
      loading={loading}
      disabled={disabled}
      value={single ? (selected[0] || undefined) : selected}
      options={options}
      notFoundContent={loading ? undefined : <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无可绑定指标" />}
      onChange={(v: any) => onChange(single ? (v ? [String(v)] : []) : (v || []).map((x: any) => String(x)))}
    />
  )
}

export interface DataBindingEditorProps {
  /** 已绑定的设备测点 */
  series: VizSeriesRef[]
  /** 已绑定的指标 */
  metrics?: string[]
  /** 只允许一条绑定（kpi/gauge） */
  single?: boolean
  /** 指标类别（缺省瞬时值）；折线图传 'series' 只列按天的时序指标 */
  metricKind?: VizMetricKind
  disabled?: boolean
  onChange: (patch: { series?: VizSeriesRef[], metrics?: string[] }) => void
}

/**
 * 数据绑定编辑器：两种方式二选一。
 *
 *   - 设备测点 → 现有的 SeriesEditor（不改动）；
 *   - 指标     → 平台/租户统计指标（product.total 等）；列表按 metricKind 过滤
 *     （数值卡/仪表盘/表格用瞬时值，折线图用时序值）。
 *
 * 切换方式时会清空另一侧：取数接口里 series 与 metrics 是同一项的两个字段，
 * 同时留着会让一个组件既查设备属性又查指标，语义与开销都不清楚。
 * 「没配 metrics 就是设备测点」这条缺省规则与历史看板完全一致。
 */
const DataBindingEditor: React.FC<DataBindingEditorProps> = ({ series, metrics, single, metricKind, disabled, onChange }) => {
  const boundMetrics = metrics || []
  // 每次打开配置面板（Drawer destroyOnClose）按当前配置重新判定方式
  const [mode, setMode] = useState<VizBindMode>(() => (boundMetrics.length > 0 ? 'metrics' : 'series'))

  const switchMode = (next: VizBindMode) => {
    if (next === mode) return
    setMode(next)
    if (next === 'metrics') {
      onChange({ metrics: boundMetrics, series: [] })
    } else {
      onChange({ metrics: [], series: series || [] })
    }
  }

  const metricsHint = metricKind === 'series'
    ? '平台指标按天统计（每日任务汇总），用「时间范围」选要看的跨度。'
    : '统计指标是瞬时聚合值（按当前租户/平台视图统计），没有历史曲线。'

  return (
    <>
      <Segmented
        block
        value={mode}
        disabled={disabled}
        onChange={v => switchMode(String(v) as VizBindMode)}
        options={[
          { label: '设备测点', value: 'series' },
          { label: '指标', value: 'metrics' },
        ]}
      />
      {mode === 'series'
        ? <SeriesEditor single={single} value={series || []} onChange={next => onChange({ series: next })} disabled={disabled} />
        : (
          <SettingRow hint={metricsHint}>
            <MetricsEditor single={single} kind={metricKind} value={boundMetrics} onChange={next => onChange({ metrics: next })} disabled={disabled} />
          </SettingRow>
        )}
    </>
  )
}

export default DataBindingEditor
