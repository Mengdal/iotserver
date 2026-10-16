import React, { useMemo } from 'react'
import { Empty, Select, Spin, Switch, Table, Typography } from 'antd'
import { TableOutlined } from '@ant-design/icons'
import type { VizDataSeries, VizMetricValue, VizSeriesRef } from '@/api'
import type { VizWidgetManifest, VizWidgetSettingsProps } from '../types'
import { formatTime, formatValue, latestPoint, seriesLabel, seriesUnit } from '../format'
import DataBindingEditor from './MetricsEditor'
import { SettingGroup, SettingRow } from './SettingsLayout'

const { Text } = Typography

export interface TableConfig {
  series: VizSeriesRef[]
  /** 指标绑定（与 series 二选一；每个指标一行） */
  metrics?: string[]
  timeRange: string
  /** true = 当前值（每行一条数据序列）；false = 历史明细（按时间合并） */
  latest: boolean
  pageSize: number
}

const RANGE_OPTIONS = [
  { label: '近 1 小时', value: '1h' },
  { label: '近 24 小时', value: '24h' },
  { label: '近 7 天', value: '7d' },
  { label: '近 30 天', value: '30d' },
  { label: '不限', value: 'all' },
]

interface LatestRow {
  key: string
  device: string
  property: string
  value: string
  unit: string
  time: string
}

interface HistoryRow {
  key: string
  time: string
  [series: string]: string | number
}

interface MetricRow {
  key: string
  metric: string
  value: string
  unit: string
}

const TableWidget: React.FC<{ config: TableConfig; ctx: any }> = ({ config, ctx }) => {
  const series: VizDataSeries[] = (ctx.data && ctx.data.series) || []
  const metricValues: VizMetricValue[] = (ctx.data && ctx.data.metrics) || []
  const metricKeys = config.metrics || []
  // 绑了指标就按「每个指标一行」渲染；没绑指标走原来的设备测点路径（当前值/历史明细两种模式不变）
  const useMetric = metricKeys.length > 0

  const latestRows = useMemo<LatestRow[]>(() => series.map((s, index) => {
    const point = latestPoint(s)
    return {
      key: `${s.deviceId}-${s.code}-${index}`,
      device: s.deviceName || s.deviceId,
      property: s.name || s.code,
      value: formatValue(point ? point.value : undefined, 2),
      unit: seriesUnit(s, ''),
      time: point ? formatTime(point.time) : '--',
    }
  }), [series])

  const history = useMemo(() => {
    const columns = series.map(s => seriesLabel(s))
    const byTime = new Map<number, HistoryRow>()
    series.forEach((s) => {
      const label = seriesLabel(s)
      ;(s.points || []).forEach((p) => {
        const row = byTime.get(p.time) || { key: String(p.time), time: formatTime(p.time) } as HistoryRow
        row[label] = formatValue(p.value as any, 2)
        byTime.set(p.time, row)
      })
    })
    const rows = Array.from(byTime.values()).sort((a, b) => (a.key < b.key ? 1 : -1))
    return { columns, rows }
  }, [series])

  // 每个指标一行：按绑定的 key 顺序列出，取不到值（未登记/视图不可见）显示 --
  const metricRows = useMemo<MetricRow[]>(() => {
    const byKey = new Map(metricValues.map(m => [m.key, m]))
    return metricKeys.map((key, index) => {
      const hit = byKey.get(key)
      return {
        key: `${key}-${index}`,
        metric: (hit && hit.label) || key,
        value: hit ? formatValue(hit.value, 2) : '--',
        unit: (hit && hit.unit) || '',
      }
    })
  }, [metricKeys, metricValues])

  if (!useMetric && (!config.series || config.series.length === 0)) {
    return <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="还没绑定数据（右上角菜单 → 配置本组件）" />
  }
  if (ctx.loading && series.length === 0 && metricValues.length === 0) {
    return <div style={{ textAlign: 'center', paddingTop: 12 }}><Spin /></div>
  }

  const pagination = { pageSize: config.pageSize || 10, size: 'small' as const, showSizeChanger: false }

  if (useMetric) {
    return (
      <Table<MetricRow>
        size="small"
        rowKey="key"
        dataSource={metricRows}
        pagination={metricRows.length > (config.pageSize || 10) ? pagination : false}
        columns={[
          { title: '指标', dataIndex: 'metric', ellipsis: true },
          { title: '值', dataIndex: 'value', align: 'right' },
          { title: '单位', dataIndex: 'unit', width: 70 },
        ]}
      />
    )
  }

  const errors = series.filter(s => s.error)
  if (errors.length > 0) {
    return <Text type="danger">{errors.map(s => `${seriesLabel(s)}: ${s.error}`).join('；')}</Text>
  }

  if (config.latest !== false) {
    return (
      <Table<LatestRow>
        size="small"
        rowKey="key"
        dataSource={latestRows}
        pagination={latestRows.length > (config.pageSize || 10) ? pagination : false}
        columns={[
          { title: '设备', dataIndex: 'device', ellipsis: true },
          { title: '属性', dataIndex: 'property', ellipsis: true },
          { title: '当前值', dataIndex: 'value', align: 'right' },
          { title: '单位', dataIndex: 'unit', width: 70 },
          { title: '采样时间', dataIndex: 'time', width: 150 },
        ]}
      />
    )
  }

  return (
    <Table<HistoryRow>
      size="small"
      rowKey="key"
      dataSource={history.rows}
      pagination={pagination}
      scroll={{ y: undefined }}
      columns={[
        { title: '时间', dataIndex: 'time', width: 150, fixed: 'left' as const },
        ...history.columns.map(label => ({ title: label, dataIndex: label, align: 'right' as const })),
      ]}
    />
  )
}

const TableSettings: React.FC<VizWidgetSettingsProps<TableConfig>> = ({ config, onChange }) => {
  const useMetric = (config.metrics || []).length > 0
  return (
    <>
      <SettingGroup title="数据绑定">
        <DataBindingEditor
          series={config.series || []}
          metrics={config.metrics}
          onChange={patch => onChange(patch)}
        />
      </SettingGroup>
      <SettingGroup title="显示设置">
        {/* 「当前值 / 历史明细」只对设备测点有意义：指标是聚合值，没有历史明细 */}
        {useMetric
          ? <SettingRow hint="指标表格每个指标一行；「当前值 / 历史明细」只对设备测点生效。" />
          : (
            <>
              <SettingRow
                label="当前值"
                inline
                hint="每行显示一条数据序列的最新上报；关掉后按时间列出历史明细。"
              >
                <Switch size="small" checked={config.latest !== false} onChange={v => onChange({ latest: v })} />
              </SettingRow>
              <SettingRow label="时间范围">
                <Select
                  style={{ width: '100%' }}
                  value={config.timeRange || '24h'}
                  options={RANGE_OPTIONS}
                  disabled={config.latest !== false}
                  onChange={v => onChange({ timeRange: v })}
                />
              </SettingRow>
            </>
          )}
        <SettingRow label="每页条数">
          <Select
            style={{ width: '100%' }}
            value={config.pageSize || 10}
            options={[10, 20, 50, 100].map(v => ({ label: String(v), value: v }))}
            onChange={v => onChange({ pageSize: v })}
          />
        </SettingRow>
      </SettingGroup>
    </>
  )
}

export const tableManifest: VizWidgetManifest<TableConfig> = {
  type: 'table',
  label: '表格',
  icon: <TableOutlined />,
  defaultSize: { w: 6, h: 8 },
  minSize: { w: 3, h: 3 },
  data: true,
  defaultConfig: () => ({
    series: [],
    metrics: [],
    timeRange: '24h',
    latest: true,
    pageSize: 10,
  }),
  buildQuery: config => ({
    series: config.series || [],
    metrics: config.metrics || [],
    latest: config.latest !== false,
    timeRange: config.timeRange || '24h',
    limit: 500,
  }),
  render: ({ config, ctx }) => <TableWidget config={config} ctx={ctx} />,
  Settings: TableSettings,
}
