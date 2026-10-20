import React, { useEffect, useMemo, useState } from 'react'
import { Empty, Select, Spin, Switch, Table, Typography } from 'antd'
import { TableOutlined } from '@ant-design/icons'
import { getDeviceMessageRank } from '@/api'
import type { VizDataSeries, VizMetricValue, VizRankItem, VizRankResult, VizSeriesRef } from '@/api'
import type { VizWidgetManifest, VizWidgetSettingsProps } from '../types'
import { formatTime, formatValue, latestPoint, seriesLabel, seriesUnit } from '../format'
import DataBindingEditor from './MetricsEditor'
import { SettingGroup, SettingRow } from './SettingsLayout'
import { t, tt } from '@/i18n'

const { Text } = Typography

export interface TableConfig {
  series: VizSeriesRef[]
  /** 指标绑定（与 series 二选一；每个指标一行） */
  metrics?: string[]
  /**
   * 数据来源：缺省 = 设备测点 / 指标；
   * deviceMessageRank = 设备消息量排行（复用表格组件渲染排名）
   */
  source?: 'deviceMessageRank'
  timeRange: string
  /** 排行显示条数（source=deviceMessageRank 时生效） */
  limit?: number
  /** true = 当前值（每行一条数据序列）；false = 历史明细（按时间合并） */
  latest: boolean
  pageSize: number
}

const RANGE_OPTIONS = [
  { label: t('近 1 小时'), value: '1h' },
  { label: t('近 24 小时'), value: '24h' },
  { label: t('近 7 天'), value: '7d' },
  { label: t('近 30 天'), value: '30d' },
  { label: t('不限'), value: 'all' },
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

const RANK_LIMIT_OPTIONS = [3, 5, 10, 20].map(n => ({ label: String(n), value: n }))

/**
 * 设备消息量排行：复用「表格」组件的数据来源，不单独注册组件类型。
 * 排行榜不走 useVizData 的批量取数，按看板/组件生效间隔自行轮询。
 */
const RankTable: React.FC<{ timeRange: string; limit: number; refreshInterval: number }> = ({
  timeRange, limit, refreshInterval,
}) => {
  const [data, setData] = useState<VizRankResult | null>(null)
  const [loading, setLoading] = useState(false)
  const [failed, setFailed] = useState(false)

  useEffect(() => {
    let alive = true
    let timer: number | undefined
    // showLoading=false 是后台静默刷新：保留旧数据，失败也不把已展示的榜单替换成错误态
    const load = (showLoading: boolean) => {
      if (showLoading) {
        setLoading(true)
        setFailed(false)
      }
      getDeviceMessageRank({ timeRange, limit })
        .then(resp => {
          if (!alive) return
          if (resp.success && resp.result) {
            setData(resp.result)
          } else if (showLoading) {
            setFailed(true)
          }
        })
        .catch(() => { if (alive && showLoading) setFailed(true) })
        .finally(() => { if (alive && showLoading) setLoading(false) })
    }
    load(true)
    if (refreshInterval > 0) {
      timer = window.setInterval(() => load(false), refreshInterval * 1000)
    }
    return () => {
      alive = false
      if (timer) window.clearInterval(timer)
    }
  }, [timeRange, limit, refreshInterval])

  if (loading && !data) {
    return <div style={{ textAlign: 'center', paddingTop: 12 }}><Spin /></div>
  }
  if (failed && !data) {
    return <Text type="danger">{t('加载失败')}</Text>
  }
  const items = (data && data.items) || []
  if (items.length === 0) {
    return <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={t('暂无设备消息数据')} />
  }
  const total = Math.max(data?.total || 0, 1)
  return (
    <Table<VizRankItem>
      size="small"
      rowKey="deviceId"
      dataSource={items}
      pagination={false}
      columns={[
        { title: t('排名'), width: 60, render: (_value, _record, index) => index + 1 },
        {
          title: t('设备'),
          dataIndex: 'deviceName',
          ellipsis: true,
          render: (value: string, record) => value || record.deviceId,
        },
        { title: t('消息数'), dataIndex: 'count', align: 'right' as const },
        {
          title: t('占比'),
          width: 80,
          align: 'right' as const,
          render: (_value, record) => `${Math.round(record.count / total * 1000) / 10}%`,
        },
      ]}
    />
  )
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
        metric: tt((hit && hit.label) || key),
        value: hit ? formatValue(hit.value, 2) : '--',
        unit: tt((hit && hit.unit) || ''),
      }
    })
  }, [metricKeys, metricValues])

  // 设备消息量排行：仍由「表格」组件渲染，只是数据来自排行接口
  if (config.source === 'deviceMessageRank') {
    return (
      <RankTable
        timeRange={config.timeRange || '7d'}
        limit={config.limit || 5}
        refreshInterval={ctx.refreshInterval || 0}
      />
    )
  }

  if (!useMetric && (!config.series || config.series.length === 0)) {
    return <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={t('还没绑定数据（右上角菜单 → 配置本组件）')} />
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
          { title: t('指标'), dataIndex: 'metric', ellipsis: true },
          { title: t('值'), dataIndex: 'value', align: 'right' },
          { title: t('单位'), dataIndex: 'unit', width: 70 },
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
          { title: t('设备'), dataIndex: 'device', ellipsis: true },
          { title: t('属性'), dataIndex: 'property', ellipsis: true },
          { title: t('当前值'), dataIndex: 'value', align: 'right' },
          { title: t('单位'), dataIndex: 'unit', width: 70 },
          { title: t('采样时间'), dataIndex: 'time', width: 150 },
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
        { title: t('时间'), dataIndex: 'time', width: 150, fixed: 'left' as const },
        ...history.columns.map(label => ({ title: label, dataIndex: label, align: 'right' as const })),
      ]}
    />
  )
}

const TableSettings: React.FC<VizWidgetSettingsProps<TableConfig>> = ({ config, onChange }) => {
  const isRank = config.source === 'deviceMessageRank'
  const useMetric = !isRank && (config.metrics || []).length > 0
  return (
    <>
      <SettingGroup title={t('数据来源')}>
        <SettingRow label={t('数据来源')}>
          <Select
            style={{ width: '100%' }}
            value={isRank ? 'deviceMessageRank' : 'device'}
            options={[
              { label: t('设备测点 / 指标'), value: 'device' },
              { label: t('设备消息量排行'), value: 'deviceMessageRank' },
            ]}
            onChange={v => onChange(v === 'deviceMessageRank'
              ? { source: 'deviceMessageRank', series: [], metrics: [], timeRange: '7d', limit: 5 }
              : { source: undefined })}
          />
        </SettingRow>
      </SettingGroup>
      {isRank
        ? (
          <SettingGroup title={t('数据设置')}>
            <SettingRow label={t('时间范围')}>
              <Select
                style={{ width: '100%' }}
                value={config.timeRange || '7d'}
                options={RANGE_OPTIONS}
                onChange={v => onChange({ timeRange: v })}
              />
            </SettingRow>
            <SettingRow label={t('显示条数')}>
              <Select
                style={{ width: '100%' }}
                value={config.limit || 5}
                options={RANK_LIMIT_OPTIONS}
                onChange={v => onChange({ limit: v })}
              />
            </SettingRow>
          </SettingGroup>
        )
        : (
          <>
            <SettingGroup title={t('数据绑定')}>
              <DataBindingEditor
                series={config.series || []}
                metrics={config.metrics}
                onChange={patch => onChange(patch)}
              />
            </SettingGroup>
            <SettingGroup title={t('显示设置')}>
              {/* 「当前值 / 历史明细」只对设备测点有意义：指标是聚合值，没有历史明细 */}
              {useMetric
                ? <SettingRow hint={t('指标表格每个指标一行；「当前值 / 历史明细」只对设备测点生效。')} />
                : (
                  <>
                    <SettingRow
                      label={t('当前值')}
                      inline
                      hint={t('每行显示一条数据序列的最新上报；关掉后按时间列出历史明细。')}
                    >
                      <Switch size="small" checked={config.latest !== false} onChange={v => onChange({ latest: v })} />
                    </SettingRow>
                    <SettingRow label={t('时间范围')}>
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
              <SettingRow label={t('每页条数')}>
                <Select
                  style={{ width: '100%' }}
                  value={config.pageSize || 10}
                  options={[10, 20, 50, 100].map(v => ({ label: String(v), value: v }))}
                  onChange={v => onChange({ pageSize: v })}
                />
              </SettingRow>
            </SettingGroup>
          </>
        )}
    </>
  )
}

export const tableManifest: VizWidgetManifest<TableConfig> = {
  type: 'table',
  label: t('表格'),
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
  buildQuery: config => (
    config.source === 'deviceMessageRank'
      // 排行数据由组件自己调 /viz/device-message-rank，不参与设备数据批量查询
      ? { series: [], metrics: [] }
      : {
        series: config.series || [],
        metrics: config.metrics || [],
        latest: config.latest !== false,
        timeRange: config.timeRange || '24h',
        limit: 500,
      }
  ),
  render: ({ config, ctx }) => <TableWidget config={config} ctx={ctx} />,
  Settings: TableSettings,
}
