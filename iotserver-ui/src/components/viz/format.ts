import dayjs from 'dayjs'
import type { VizDataPoint, VizDataSeries } from '@/api'

/** 数值展示：按小数位格式化；非数值原样返回 */
export function formatValue (value: number | string | null | undefined, decimals = 0): string {
  if (value === null || value === undefined || value === '') return '--'
  const num = typeof value === 'number' ? value : Number(value)
  if (!Number.isFinite(num)) return String(value)
  return num.toFixed(Math.max(0, Math.min(decimals, 6)))
}

export function formatTime (time?: number, pattern = 'MM-DD HH:mm:ss'): string {
  if (!time) return '--'
  return dayjs(time).format(pattern)
}

/** 序列名：后端已经补好"设备名·属性名"，这里只做兜底 */
export function seriesLabel (s?: VizDataSeries): string {
  if (!s) return ''
  return s.name || [s.deviceName, s.code].filter(Boolean).join('·') || s.code
}

/** 取最新一个点（后端 latest 模式只返回一个点，历史模式返回正序列表） */
export function latestPoint (s?: VizDataSeries): VizDataPoint | undefined {
  if (!s || !s.points || s.points.length === 0) return undefined
  return s.points[s.points.length - 1]
}

/** 单位：配置里手填的优先，否则用属性自带的 */
export function seriesUnit (s: VizDataSeries | undefined, override?: string): string {
  const unit = (override || '').trim() || (s && s.unit) || ''
  return unit === '-' ? '' : unit
}

/** 折线图的宽表：把多条序列摊平成 [{time, value, series}] */
export function toSeriesRows (list: VizDataSeries[] | undefined) {
  const rows: { time: string; value: number; series: string }[] = []
  ;(list || []).forEach((s) => {
    const label = seriesLabel(s)
    ;(s.points || []).forEach((p) => {
      const num = typeof p.value === 'number' ? p.value : Number(p.value)
      if (!Number.isFinite(num)) return
      rows.push({ time: formatTime(p.time, 'MM-DD HH:mm'), value: num, series: label })
    })
  })
  return rows
}
