import React from 'react'
import { Button, Empty, Input, InputNumber, Progress, Spin, Typography } from 'antd'
import { DeleteOutlined, PlusOutlined, DashboardOutlined } from '@ant-design/icons'
import type { VizSeriesRef } from '@/api'
import type { VizWidgetManifest, VizWidgetSettingsProps } from '../types'
import { formatTime, formatValue, latestPoint, seriesLabel, seriesUnit } from '../format'
import DataBindingEditor from './MetricsEditor'
import { SettingGroup, SettingRow } from './SettingsLayout'

const { Text } = Typography

export interface GaugeConfig {
  series: VizSeriesRef[]
  /** 指标绑定（与 series 二选一；只取第一个） */
  metrics?: string[]
  min: number
  max: number
  decimals: number
  /** 覆盖单位：留空用属性/指标自带的单位 */
  unit: string
  /** 阈值配色：[[阈值, 颜色]]，按阈值升序生效（与 OpenRemote 的 gauge 阈值同构） */
  thresholds: [number, string][]
}

const DEFAULT_THRESHOLDS: [number, string][] = [
  [0, '#4caf50'],
  [75, '#ff9800'],
  [90, '#ef5350'],
]

/** 阈值（绝对值）→ antd Progress 需要的百分比分段色 */
function toStrokeColor (thresholds: [number, string][], min: number, max: number) {
  const span = max - min
  if (!Number.isFinite(span) || span <= 0) return undefined
  return thresholds
    .slice()
    .sort((a, b) => a[0] - b[0])
    .map(([value, color]) => ({ percent: Math.max(0, Math.min(100, ((value - min) / span) * 100)), color }))
}

const GaugeWidget: React.FC<{ config: GaugeConfig; ctx: any }> = ({ config, ctx }) => {
  const series = (ctx.data && ctx.data.series && ctx.data.series[0]) || undefined
  const metric = (ctx.data && ctx.data.metrics && ctx.data.metrics[0]) || undefined
  const metricKey = (config.metrics || [])[0]
  // 绑了指标就按指标驱动指针；没绑指标走原来的设备测点路径（量程仍取 config 的 min/max）
  const useMetric = !!metricKey
  if (!useMetric && (!config.series || config.series.length === 0)) {
    return <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="还没绑定数据（右上角菜单 → 配置本组件）" />
  }
  if (ctx.loading && !series && !metric) return <div style={{ textAlign: 'center', paddingTop: 12 }}><Spin /></div>
  if (!useMetric && series && series.error) return <Text type="danger">{series.error}</Text>

  const point = useMetric ? undefined : latestPoint(series)
  const raw = useMetric ? (metric ? Number(metric.value) : NaN) : (point ? Number(point.value) : NaN)
  const min = Number(config.min) || 0
  const max = Number(config.max) || 100
  const span = max - min
  const percent = Number.isFinite(raw) && span > 0
    ? Math.max(0, Math.min(100, ((raw - min) / span) * 100))
    : 0
  const unit = useMetric
    ? ((metric && metric.unit) || config.unit || '').trim()
    : seriesUnit(series, config.unit)
  const label = useMetric ? (metric ? (metric.label || metricKey) : metricKey) : seriesLabel(series)

  return (
    <div style={{ height: '100%', display: 'flex', flexDirection: 'column', justifyContent: 'center', alignItems: 'center' }}>
      <Text type="secondary" style={{ fontSize: 12 }} ellipsis>{label}</Text>
      <Progress
        type="dashboard"
        percent={Math.round(percent * 10) / 10}
        // antd 的类型只声明了 string / string[] / 渐变，实际支持"按百分比分段配色"，这里显式放行
        strokeColor={toStrokeColor(
          config.thresholds && config.thresholds.length ? config.thresholds : DEFAULT_THRESHOLDS,
          min,
          max,
        ) as any}
        format={() => (
          <span style={{ fontSize: 20, fontWeight: 600 }}>
            {Number.isFinite(raw) ? formatValue(raw, config.decimals) : '--'}
            {unit ? <span style={{ fontSize: 12, marginLeft: 2 }}>{unit}</span> : null}
          </span>
        )}
      />
      <Text type="secondary" style={{ fontSize: 12 }}>
        {(useMetric ? metric : point) ? (useMetric ? '实时统计' : formatTime(point && point.time)) : '暂无数据'} · 量程 {min}~{max}
      </Text>
    </div>
  )
}

const GaugeSettings: React.FC<VizWidgetSettingsProps<GaugeConfig>> = ({ config, onChange }) => {
  const thresholds = config.thresholds || DEFAULT_THRESHOLDS
  const patchThreshold = (index: number, value?: number, color?: string) => {
    const list = thresholds.map((t, i) => (
      i === index ? ([value ?? t[0], color ?? t[1]] as [number, string]) : t
    ))
    onChange({ thresholds: list })
  }
  return (
    <>
      <SettingGroup title="数据绑定">
        <DataBindingEditor
          single
          series={config.series || []}
          metrics={config.metrics}
          onChange={patch => onChange(patch)}
        />
      </SettingGroup>
      <SettingGroup title="显示设置">
        <SettingRow label="最小值">
          <InputNumber style={{ width: '100%' }} value={config.min} onChange={v => onChange({ min: Number(v) || 0 })} />
        </SettingRow>
        <SettingRow label="最大值">
          <InputNumber style={{ width: '100%' }} value={config.max} onChange={v => onChange({ max: Number(v) || 0 })} />
        </SettingRow>
        <SettingRow label="小数位">
          <InputNumber
            min={0}
            max={6}
            style={{ width: '100%' }}
            value={config.decimals}
            onChange={v => onChange({ decimals: Number(v) || 0 })}
          />
        </SettingRow>
        <SettingRow label="单位">
          <Input
            style={{ width: '100%' }}
            placeholder="留空用属性/指标单位"
            value={config.unit || ''}
            onChange={e => onChange({ unit: e.target.value })}
          />
        </SettingRow>
      </SettingGroup>
      <SettingGroup
        title="阈值配色"
        extra={(
          <Button
            type="link"
            size="small"
            icon={<PlusOutlined />}
            onClick={() => onChange({ thresholds: thresholds.concat([[0, '#4caf50']]) })}
          >
            添加阈值
          </Button>
        )}
      >
        <SettingRow hint="大于等于该值显示对应颜色；未配置时用默认阈值。" />
        {thresholds.map(([value, color], index) => (
          <SettingRow key={index}>
            <InputNumber
              style={{ flex: 1, minWidth: 0 }}
              value={value}
              onChange={v => patchThreshold(index, Number(v) || 0)}
            />
            <input
              type="color"
              className="viz-settings__color"
              value={color}
              onChange={e => patchThreshold(index, undefined, e.target.value)}
            />
            <Text type="secondary" style={{ fontSize: 12 }}>{color}</Text>
            <Button
              type="text"
              danger
              icon={<DeleteOutlined />}
              onClick={() => onChange({ thresholds: thresholds.filter((_, i) => i !== index) })}
            />
          </SettingRow>
        ))}
      </SettingGroup>
    </>
  )
}

export const gaugeManifest: VizWidgetManifest<GaugeConfig> = {
  type: 'gauge',
  label: '仪表盘',
  icon: <DashboardOutlined />,
  defaultSize: { w: 3, h: 6 },
  minSize: { w: 2, h: 4 },
  data: true,
  defaultConfig: () => ({
    series: [],
    metrics: [],
    min: 0,
    max: 100,
    decimals: 0,
    unit: '',
    thresholds: DEFAULT_THRESHOLDS,
  }),
  buildQuery: config => ({ series: config.series || [], metrics: config.metrics || [], latest: true }),
  render: ({ config, ctx }) => <GaugeWidget config={config} ctx={ctx} />,
  Settings: GaugeSettings,
}
