import React, { useMemo } from 'react'
import { Empty, Select, Spin, Switch, Typography } from 'antd'
import { LineChartOutlined } from '@ant-design/icons'
import { Area, Line } from '@ant-design/plots'
import type { VizSeriesRef } from '@/api'
import type { VizWidgetManifest, VizWidgetSettingsProps } from '../types'
import { toSeriesRows } from '../format'
import DataBindingEditor from './MetricsEditor'
import { SettingGroup, SettingRow } from './SettingsLayout'
import { t } from '@/i18n'

const { Text } = Typography

export interface LineConfig {
  series: VizSeriesRef[]
  /** 平台时序指标 key（如 report.daily），与 series 二选一 */
  metrics?: string[]
  /** 1h / 24h / 7d / 30d / all */
  timeRange: string
  /** 聚合间隔（秒），0 = 原始点 */
  aggInterval: number
  /** avg / max / min / sum / count / first / last */
  aggMethod: string
  showLegend: boolean
  smooth: boolean
  /** 面积填充（折线换面积图，共用同一份点集） */
  fill: boolean
  stacked: boolean
}

const RANGE_OPTIONS = [
  { label: t('近 1 小时'), value: '1h' },
  { label: t('近 24 小时'), value: '24h' },
  { label: t('近 7 天'), value: '7d' },
  { label: t('近 30 天'), value: '30d' },
  { label: t('不限'), value: 'all' },
]

const INTERVAL_OPTIONS = [
  { label: t('不聚合（原始点）'), value: 0 },
  { label: t('1 分钟'), value: 60 },
  { label: t('5 分钟'), value: 300 },
  { label: t('15 分钟'), value: 900 },
  { label: t('1 小时'), value: 3600 },
  { label: t('1 天'), value: 86400 },
]

const METHOD_OPTIONS = [
  { label: t('平均'), value: 'avg' },
  { label: t('最大'), value: 'max' },
  { label: t('最小'), value: 'min' },
  { label: t('合计'), value: 'sum' },
  { label: t('次数'), value: 'count' },
  { label: t('第一个'), value: 'first' },
  { label: t('最后一个'), value: 'last' },
]

const LineWidget: React.FC<{ config: LineConfig; ctx: any }> = ({ config, ctx }) => {
  // 绑定的是设备测点还是平台时序指标；两种来源后端都会落到 series，渲染不用区分
  const hasBinding = (config.series && config.series.length > 0) || (config.metrics && config.metrics.length > 0)
  const series = (ctx.data && ctx.data.series) || []
  const rows = useMemo(() => toSeriesRows(series), [series])
  const errors = series.filter((s: any) => s.error)

  if (!hasBinding) {
    return <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={t('还没绑定数据（右上角菜单 → 配置本组件）')} />
  }
  if (ctx.loading && rows.length === 0) return <div style={{ textAlign: 'center', paddingTop: 12 }}><Spin /></div>
  if (errors.length > 0 && rows.length === 0) {
    return <Text type="danger">{errors.map((s: any) => `${s.name || s.code}: ${s.error}`).join('；')}</Text>
  }
  if (rows.length === 0) return <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={t('时间范围内没有数据')} />

  // 折线/面积同源（G2Plot），只有填充差异：一个变量切图型，下面这组配置两者共用
  const Chart: React.ComponentType<any> = config.fill ? Area : Line

  return (
    <div style={{ height: '100%', width: '100%' }}>
      <Chart
        data={rows}
        xField="time"
        yField="value"
        seriesField="series"
        smooth={!!config.smooth}
        isStack={!!config.stacked}
        legend={config.showLegend === false ? false : { position: 'top' }}
        xAxis={{ label: { autoHide: true, autoRotate: false } }}
        yAxis={{ label: { formatter: (v: string) => v } }}
        tooltip={{ showMarkers: true }}
        animation={false}
        autoFit
      />
    </div>
  )
}

const LineSettings: React.FC<VizWidgetSettingsProps<LineConfig>> = ({ config, onChange }) => (
  <>
    <SettingGroup title={t('数据绑定')}>
      <DataBindingEditor
        series={config.series || []}
        metrics={config.metrics}
        metricKind="series"
        onChange={patch => onChange(patch)}
      />
    </SettingGroup>
    <SettingGroup title={t('显示设置')}>
      <SettingRow label={t('时间范围')}>
        <Select
          style={{ width: '100%' }}
          value={config.timeRange || '24h'}
          options={RANGE_OPTIONS}
          onChange={v => onChange({ timeRange: v })}
        />
      </SettingRow>
      <SettingRow label={t('聚合间隔')}>
        <Select
          style={{ width: '100%' }}
          value={config.aggInterval ?? 0}
          options={INTERVAL_OPTIONS}
          onChange={v => onChange({ aggInterval: v })}
        />
      </SettingRow>
      <SettingRow
        label={t('聚合方法')}
        hint={config.aggInterval ? t('聚合由数据层按时间桶计算，返回点数上限 2000') : undefined}
      >
        <Select
          style={{ width: '100%' }}
          value={config.aggMethod || 'avg'}
          options={METHOD_OPTIONS}
          onChange={v => onChange({ aggMethod: v })}
          disabled={!config.aggInterval}
        />
      </SettingRow>
      <SettingRow label={t('图例')} inline>
        <Switch size="small" checked={config.showLegend !== false} onChange={v => onChange({ showLegend: v })} />
      </SettingRow>
      <SettingRow label={t('平滑')} inline>
        <Switch size="small" checked={!!config.smooth} onChange={v => onChange({ smooth: v })} />
      </SettingRow>
      <SettingRow label={t('面积填充')} inline>
        <Switch size="small" checked={!!config.fill} onChange={v => onChange({ fill: v })} />
      </SettingRow>
      <SettingRow label={t('堆叠')} inline>
        <Switch size="small" checked={!!config.stacked} onChange={v => onChange({ stacked: v })} />
      </SettingRow>
    </SettingGroup>
  </>
)

export const lineManifest: VizWidgetManifest<LineConfig> = {
  type: 'line',
  label: t('折线图'),
  icon: <LineChartOutlined />,
  defaultSize: { w: 8, h: 8 },
  minSize: { w: 3, h: 4 },
  data: true,
  defaultConfig: () => ({
    series: [],
    metrics: [],
    timeRange: '24h',
    aggInterval: 300,
    aggMethod: 'avg',
    showLegend: true,
    smooth: false,
    fill: false,
    stacked: false,
  }),
  buildQuery: config => ({
    series: config.series || [],
    metrics: config.metrics || [],
    timeRange: config.timeRange || '24h',
    aggInterval: config.aggInterval || 0,
    aggMethod: config.aggMethod || 'avg',
    limit: 500,
  }),
  render: ({ config, ctx }) => <LineWidget config={config} ctx={ctx} />,
  Settings: LineSettings,
}
