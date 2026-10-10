import React, { useEffect, useMemo, useState } from 'react'
import { Empty, Select, Space, Tag, Typography } from 'antd'
import {
  BarChartOutlined, FieldNumberOutlined, LineChartOutlined, PieChartOutlined, TableOutlined,
} from '@ant-design/icons'
import { getVizDatasources, VizBoardSpec, VizMeasure, VizView, VizViewColumn } from '@/api'
import Field from '@/components/Field'
import './index.less'

const { Text } = Typography

export type ChartType = VizBoardSpec['chart']

/** 图形类型：点图标切换 */
export const CHART_TYPES: { value: ChartType, label: string, icon: React.ReactNode }[] = [
  { value: 'value', label: '数值卡', icon: <FieldNumberOutlined /> },
  { value: 'line', label: '折线图', icon: <LineChartOutlined /> },
  { value: 'bar', label: '柱状图', icon: <BarChartOutlined /> },
  { value: 'pie', label: '饼图', icon: <PieChartOutlined /> },
  { value: 'table', label: '表格', icon: <TableOutlined /> },
]

const AGG_OPTIONS: { value: VizMeasure['agg'], label: string }[] = [
  { value: 'sum', label: '合计' },
  { value: 'avg', label: '平均' },
  { value: 'max', label: '最大' },
  { value: 'min', label: '最小' },
]

const TIME_RANGES = [
  { value: 'all', label: '不限' },
  { value: '1h', label: '1 小时' },
  { value: '24h', label: '24 小时' },
  { value: '7d', label: '7 天' },
  { value: '30d', label: '30 天' },
]

/** 新建组件的默认配置 */
export const defaultSpec = (view: string, chart: ChartType = 'value'): VizBoardSpec => ({
  view,
  chart,
  dimension: '',
  measures: [{ column: '*', agg: 'count', alias: chart === 'value' ? '记录数' : '数量' }],
  timeColumn: '',
  timeRange: 'all',
  sortDesc: chart === 'bar',
  limit: 20,
})

interface WidgetConfigProps {
  value: VizBoardSpec
  /** 数据表列表（不传就自己拉一次） */
  views?: VizView[]
  onChange: (next: VizBoardSpec) => void
}

/**
 * 最简图形化配置，只有四件事：
 *   1. 选数据表   2. 选图形   3. （可选）选分组字段   4. 选统计字段
 * 时间字段不用选：不选分组字段时，折线/柱状会自动按该表的记录时间分天。
 */
const WidgetConfig: React.FC<WidgetConfigProps> = ({ value, views: viewsProp, onChange }) => {
  const [views, setViews] = useState<VizView[]>(viewsProp || [])

  useEffect(() => {
    if (viewsProp) {
      setViews(viewsProp)
      return
    }
    getVizDatasources()
      .then((resp) => { if (resp.success) setViews(resp.result.views || []) })
      .catch(() => { /* 拿不到就保持空 */ })
  }, [viewsProp])

  const view = useMemo(() => views.find(v => v.view === value.view), [views, value.view])
  const columns = useMemo(() => ((view && view.columns) || []).filter(c => !c.hidden), [view])
  const dimensions = useMemo(() => columns.filter(c => c.role === 'dimension'), [columns])
  const measures = useMemo(() => columns.filter(c => c.role === 'measure'), [columns])

  const chart = value.chart || 'value'
  const measure: VizMeasure = (value.measures && value.measures[0]) || { column: '*', agg: 'count' }
  const isCount = !measure.column || measure.column === '*'
  const needDimension = chart === 'line' || chart === 'bar' || chart === 'pie' || chart === 'table'

  if (views.length === 0) {
    return <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="没有可用的数据表" />
  }

  const patch = (next: Partial<VizBoardSpec>) => onChange({ ...value, ...next })

  const switchView = (nextView: string) => {
    const target = views.find(v => v.view === nextView)
    const dims = ((target && target.columns) || []).filter(c => !c.hidden && c.role === 'dimension')
    onChange({
      ...defaultSpec(nextView, chart),
      dimension: dims.length > 0 ? dims[0].name : '',
      measures: [{ column: '*', agg: 'count', alias: chart === 'value' ? '记录数' : '数量' }],
      timeRange: value.timeRange || 'all',
      sortDesc: chart === 'bar',
    })
  }

  const setMeasure = (patchMeasure: Partial<{ column: string, agg: VizMeasure['agg'] }>) => {
    const next: { column: string, agg: VizMeasure['agg'] } = { column: measure.column, agg: measure.agg }
    if (patchMeasure.column !== undefined) next.column = patchMeasure.column
    if (patchMeasure.agg !== undefined) next.agg = patchMeasure.agg
    if (next.column === '*' || next.column === '') {
      next.agg = 'count'
    } else if (next.agg === 'count' || !next.agg) {
      next.agg = 'sum'
    }
    const prefix = { sum: '合计', avg: '平均', max: '最大', min: '最小', count: '数量' }[next.agg] || '数量'
    onChange({
      ...value,
      measures: [{ ...next, alias: next.column === '*' ? '数量' : `${prefix}·${next.column}` }],
    })
  }

  return (
    <div className="viz-form">
      <Field label="数据表">
        <Select
          showSearch
          style={{ width: '100%' }}
          value={value.view}
          optionFilterProp="label"
          onChange={switchView}
          options={views.map(v => ({ value: v.view, label: v.label }))}
        />
      </Field>

      <Field label="图形">
        <div className="viz-chart-types">
          {CHART_TYPES.map(item => (
            <button
              key={item.value}
              type="button"
              title={item.label}
              className={`viz-chart-type${chart === item.value ? ' viz-chart-type--active' : ''}`}
              onClick={() => patch({ chart: item.value })}
            >
              {item.icon}
              <span>{item.label}</span>
            </button>
          ))}
        </div>
      </Field>

      {needDimension
        ? (
          <Field label="分组" hint={view && view.timeColumn ? '不选=按时间' : undefined}>
            <Select
              allowClear
              showSearch
              style={{ width: '100%' }}
              placeholder={view && view.timeColumn ? '按记录时间（自动）' : '选择分组字段'}
              value={value.dimension || undefined}
              optionFilterProp="label"
              onChange={(next) => patch({ dimension: next || '' })}
              options={dimensions.map(c => ({ value: c.name, label: c.label || c.name }))}
            />
          </Field>
        )
        : null}

      <Field label="统计">
        <Space.Compact style={{ width: '100%' }}>
          <Select
            showSearch
            style={{ width: isCount ? '100%' : 'calc(100% - 104px)' }}
            value={isCount ? '*' : measure.column}
            optionFilterProp="label"
            onChange={(next) => setMeasure({ column: next })}
            options={[{ value: '*', label: '记录数（行数）' }].concat(
              measures.map((c: VizViewColumn) => ({ value: c.name, label: c.label || c.name })),
            )}
          />
          {isCount
            ? null
            : (
              <Select
                style={{ width: 104 }}
                value={measure.agg === 'count' ? 'sum' : measure.agg}
                onChange={(next) => setMeasure({ agg: next })}
                options={AGG_OPTIONS}
              />
            )}
        </Space.Compact>
      </Field>

      {view && view.timeColumn && chart !== 'value'
        ? (
          <Field label="时间范围">
            <div className="viz-time-tags">
              {TIME_RANGES.map(item => (
                <Tag.CheckableTag
                  key={item.value}
                  checked={(value.timeRange || 'all') === item.value}
                  onChange={() => patch({ timeRange: item.value })}
                >
                  {item.label}
                </Tag.CheckableTag>
              ))}
            </div>
          </Field>
        )
        : null}

      {needDimension && !value.dimension && view && view.timeColumn
        ? <Text type="secondary" style={{ fontSize: 12 }}>未选分组字段：按记录时间自动分天</Text>
        : null}
    </div>
  )
}

export default WidgetConfig
