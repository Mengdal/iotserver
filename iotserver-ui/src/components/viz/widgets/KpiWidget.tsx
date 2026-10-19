import React from 'react'
import { Empty, Input, InputNumber, Spin, Typography } from 'antd'
import { FieldNumberOutlined } from '@ant-design/icons'
import type { VizSeriesRef } from '@/api'
import type { VizWidgetManifest, VizWidgetSettingsProps } from '../types'
import { formatTime, formatValue, latestPoint, seriesLabel, seriesUnit } from '../format'
import DataBindingEditor from './MetricsEditor'
import { SettingGroup, SettingRow } from './SettingsLayout'
import { t, tt } from '@/i18n'

const { Text } = Typography

export interface KpiConfig {
  series: VizSeriesRef[]
  /** 指标绑定（与 series 二选一；只取第一个） */
  metrics?: string[]
  decimals: number
  /** 覆盖单位：留空用属性/指标自带的单位 */
  unit: string
}

const KpiWidget: React.FC<{ config: KpiConfig; ctx: any }> = ({ config, ctx }) => {
  const series = (ctx.data && ctx.data.series && ctx.data.series[0]) || undefined
  const metric = (ctx.data && ctx.data.metrics && ctx.data.metrics[0]) || undefined
  const metricKey = (config.metrics || [])[0]
  // 绑了指标就按指标展示；没绑指标走原来的设备测点路径（历史看板行为不变）
  const useMetric = !!metricKey
  if (!useMetric && (!config.series || config.series.length === 0)) {
    return <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={t('还没绑定数据（右上角菜单 → 配置本组件）')} />
  }
  if (ctx.loading && !series && !metric) return <div style={{ textAlign: 'center', paddingTop: 12 }}><Spin /></div>

  if (useMetric) {
    const unit = ((metric && metric.unit) || config.unit || '').trim()
    return (
      <div style={{ height: '100%', display: 'flex', flexDirection: 'column', justifyContent: 'center', alignItems: 'center', gap: 4 }}>
        <Text type="secondary" style={{ fontSize: 12 }} ellipsis>{tt(metric ? (metric.label || metricKey) : metricKey)}</Text>
        <div style={{ display: 'flex', alignItems: 'baseline', gap: 6 }}>
          <span style={{ fontSize: 34, fontWeight: 600, lineHeight: 1.1 }}>
            {metric ? formatValue(metric.value, config.decimals) : '--'}
          </span>
          {unit ? <Text type="secondary" style={{ fontSize: 14 }}>{tt(unit)}</Text> : null}
        </div>
        <Text type="secondary" style={{ fontSize: 12 }}>
          {metric ? t('实时统计') : t('暂无数据')}
        </Text>
      </div>
    )
  }

  if (series && series.error) return <Text type="danger">{series.error}</Text>

  const point = latestPoint(series)
  const unit = seriesUnit(series, config.unit)
  return (
    <div style={{ height: '100%', display: 'flex', flexDirection: 'column', justifyContent: 'center', alignItems: 'center', gap: 4 }}>
      <Text type="secondary" style={{ fontSize: 12 }} ellipsis>{tt(seriesLabel(series))}</Text>
      <div style={{ display: 'flex', alignItems: 'baseline', gap: 6 }}>
        <span style={{ fontSize: 34, fontWeight: 600, lineHeight: 1.1 }}>
          {formatValue(point ? point.value : undefined, config.decimals)}
        </span>
        {unit ? <Text type="secondary" style={{ fontSize: 14 }}>{tt(unit)}</Text> : null}
      </div>
      <Text type="secondary" style={{ fontSize: 12 }}>
        {point ? formatTime(point.time) : t('暂无数据')}
      </Text>
    </div>
  )
}

const KpiSettings: React.FC<VizWidgetSettingsProps<KpiConfig>> = ({ config, onChange }) => (
  <>
    <SettingGroup title={t('数据绑定')}>
      <DataBindingEditor
        single
        series={config.series || []}
        metrics={config.metrics}
        onChange={patch => onChange(patch)}
      />
    </SettingGroup>
    <SettingGroup title={t('显示设置')}>
      <SettingRow label={t('小数位')}>
        <InputNumber
          min={0}
          max={6}
          style={{ width: '100%' }}
          value={config.decimals}
          onChange={v => onChange({ decimals: Number(v) || 0 })}
        />
      </SettingRow>
      <SettingRow label={t('单位')}>
        <Input
          style={{ width: '100%' }}
          placeholder={t('留空用属性/指标单位')}
          value={config.unit || ''}
          onChange={e => onChange({ unit: e.target.value })}
        />
      </SettingRow>
    </SettingGroup>
  </>
)

export const kpiManifest: VizWidgetManifest<KpiConfig> = {
  type: 'kpi',
  label: t('数值'),
  icon: <FieldNumberOutlined />,
  defaultSize: { w: 3, h: 4 },
  minSize: { w: 1, h: 2 },
  data: true,
  defaultConfig: () => ({ series: [], metrics: [], decimals: 0, unit: '' }),
  buildQuery: config => ({ series: config.series || [], metrics: config.metrics || [], latest: true }),
  render: ({ config, ctx }) => <KpiWidget config={config} ctx={ctx} />,
  Settings: KpiSettings,
}
