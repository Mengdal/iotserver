import React, { useEffect, useMemo, useRef, useState } from 'react'
import { Alert, Button, Card, Dropdown, Empty, Input, MenuProps, Modal, Spin, Table, Typography } from 'antd'
import {
  CopyOutlined, DeleteOutlined, MoreOutlined, ReloadOutlined, SettingOutlined,
} from '@ant-design/icons'
import { Column, Line, Pie } from '@ant-design/plots'
import { VizBoardSpec, VizQueryResult, VizWidgetType } from '@/api'

const { Paragraph, Text } = Typography

/**
 * 按列的格式渲染数值，避免出现 37.43027867601241 / 3.17e+10 这种原始值。
 * 格式由后端随查询结果给出（int / float / percent / bytes / date / text）。
 */
export const formatValue = (raw: string | number | undefined | null, format?: string): string => {
  if (raw === undefined || raw === null || raw === '') return '-'
  const num = typeof raw === 'number' ? raw : Number(raw)
  if (Number.isNaN(num)) return String(raw)
  switch (format) {
    case 'bytes': {
      const units = ['B', 'KB', 'MB', 'GB', 'TB']
      let value = Math.abs(num)
      let unit = 0
      while (value >= 1024 && unit < units.length - 1) {
        value /= 1024
        unit += 1
      }
      const signed = num < 0 ? -value : value
      return `${unit === 0 ? Math.round(signed) : signed.toFixed(2)} ${units[unit]}`
    }
    case 'int':
      return Math.round(num).toLocaleString('zh-CN')
    case 'percent':
    case 'float':
      return String(Number(num.toFixed(2)))
    default:
      return String(raw)
  }
}

/** 图表高度跟随卡片（不再固定 220px，避免大卡片下方留白） */
const ChartBox: React.FC<{ children: (height: number) => React.ReactNode }> = ({ children }) => {
  const ref = useRef<HTMLDivElement>(null)
  const [height, setHeight] = useState(220)

  useEffect(() => {
    const el = ref.current
    if (!el) return undefined
    const measure = () => {
      const next = el.clientHeight
      if (next > 100) setHeight(next)
    }
    measure()
    if (typeof ResizeObserver === 'undefined') return undefined
    const observer = new ResizeObserver(measure)
    observer.observe(el)
    return () => observer.disconnect()
  }, [])

  return <div className="widget-card__chart" ref={ref}>{children(height)}</div>
}

/**
 * 组件操作：统一走 actions + id。
 * 这样父组件可以用一个引用稳定的对象（useMemo）传下来，配合 React.memo
 * 让"拖动布局"时卡片不重渲染（否则每帧重画图表，20+ 组件会卡）。
 */
export interface WidgetActions {
  config?: (id: string) => void
  duplicate?: (id: string) => void
  remove?: (id: string) => void
  refresh?: (id: string) => void
  textChange?: (id: string, text: string) => void
}

export interface WidgetCardProps {
  /** 组件 id（操作回调都按 id 派发） */
  id: string
  title?: string
  /** 组件类型：chart（默认）/ text */
  type?: VizWidgetType
  /** 文本组件正文 */
  text?: string
  spec?: VizBoardSpec
  result?: VizQueryResult
  loading?: boolean
  error?: string
  /** 编辑态：显示配置/复制/删除等入口 */
  editable?: boolean
  /** 只读态（查看模式 / 数据看板页）：不显示任何编辑入口 */
  readOnly?: boolean
  /** 操作集合（引用需稳定） */
  actions?: WidgetActions
  className?: string
  style?: React.CSSProperties
}

/**
 * 看板上的一个组件卡片（参考 app.dash.builders 的 Dashboard widget）：
 * 标题栏（名称 + 操作）+ 正文（文本 / 数值卡 / 折线 / 柱状 / 饼图 / 表格）。
 * 排版由外层网格负责，这里不带拖拽；数据由调用方取好传入（result）。
 */
const WidgetCard: React.FC<WidgetCardProps> = ({
  id,
  title,
  type = 'chart',
  text,
  spec,
  result,
  loading,
  error,
  editable,
  readOnly,
  actions,
  className,
  style,
}) => {
  const [sqlOpen, setSqlOpen] = useState(false)
  const isText = type === 'text'

  // 文本组件：本地暂存，失焦/卸载时提交，避免每敲一个字就整板重渲染
  const [draft, setDraft] = useState(text || '')
  const draftRef = useRef(text || '')
  const committedRef = useRef(text || '')
  const changeRef = useRef(actions && actions.textChange)
  useEffect(() => { draftRef.current = draft }, [draft])
  useEffect(() => { changeRef.current = actions && actions.textChange }, [actions])
  useEffect(() => {
    committedRef.current = text || ''
    setDraft(text || '')
  }, [text])
  // 失焦、切到只读态、组件卸载时都提交草稿，避免文字丢失
  const flushDraft = () => {
    if (changeRef.current && draftRef.current !== committedRef.current) {
      changeRef.current(id, draftRef.current)
    }
  }
  useEffect(() => {
    if (readOnly) flushDraft()
  }, [readOnly])
  useEffect(() => () => flushDraft(), [])

  const columns = result ? result.columns : []
  const formats = (result && result.formats) || []
  const rows = result ? result.rows : []

  const chartRows = useMemo(() => rows.map(row => ({
    __x: row[0],
    __y: Number(row[1]),
  })), [rows])

  // 折线/柱状/饼图：第 2 列是指标，按它的格式渲染坐标轴与 tooltip
  const valueFormat = formats.length > 1 ? formats[1] : formats[0]
  const axisFormatter = (value: unknown) => formatValue(value as number, valueFormat)
  const tooltipFormatter = (datum: Record<string, unknown>) => ({
    name: columns[1] || '值',
    value: formatValue(datum.__y as number, valueFormat),
  })

  const body = () => {
    if (isText) {
      if (readOnly) {
        return (
          <Paragraph style={{ whiteSpace: 'pre-wrap', marginBottom: 0 }}>
            {text || '（空文本）'}
          </Paragraph>
        )
      }
      return (
        <Input.TextArea
          value={draft}
          placeholder="写点说明，例如：数据来源、口径、更新频率…"
          autoSize={{ minRows: 3, maxRows: 14 }}
          onChange={e => setDraft(e.target.value)}
          onBlur={() => {
            if (actions && actions.textChange && draft !== (text || '')) actions.textChange(id, draft)
          }}
          style={{ border: 'none', padding: 0, boxShadow: 'none', resize: 'none' }}
        />
      )
    }
    if (loading) {
      return <div style={{ padding: 36, textAlign: 'center' }}><Spin /></div>
    }
    if (error) {
      return <Alert type="warning" showIcon message="数据加载失败" description={error} />
    }
    if (!spec) {
      return <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="组件还没配置" />
    }
    if (!result || rows.length === 0) {
      return <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="这个条件下没有数据" />
    }
    if (spec.chart === 'value') {
      return (
        <div className="widget-card__values">
          {rows.map((row, index) => (
            <div className="widget-card__value" key={columns[index] || index}>
              <div className="widget-card__value-num">{formatValue(row[0], formats[index])}</div>
              <Text type="secondary" style={{ fontSize: 12 }}>{columns[index] || ''}</Text>
            </div>
          ))}
        </div>
      )
    }
    if (spec.chart === 'table') {
      return (
        <Table
          size="small"
          rowKey={(_, index) => String(index)}
          pagination={false}
          scroll={{ x: 'max-content', y: 200 }}
          columns={columns.map(col => ({ title: col, dataIndex: col, key: col }))}
          dataSource={rows.map(row => {
            const item: Record<string, string> = {}
            columns.forEach((col, index) => { item[col] = formatValue(row[index], formats[index]) })
            return item
          })}
        />
      )
    }
    if (spec.chart === 'pie') {
      return (
        <ChartBox>
          {height => (
            <Pie
              data={chartRows}
              angleField="__y"
              colorField="__x"
              height={height}
              legend={false}
              label={false}
              tooltip={{ formatter: (datum: Record<string, unknown>) => ({ name: String(datum.__x || ''), value: formatValue(datum.__y as number, valueFormat) }) }}
            />
          )}
        </ChartBox>
      )
    }
    if (spec.chart === 'line') {
      return (
        <ChartBox>
          {height => (
            <Line
              data={chartRows}
              xField="__x"
              yField="__y"
              height={height}
              yAxis={{ label: { formatter: axisFormatter } }}
              tooltip={{ formatter: tooltipFormatter }}
            />
          )}
        </ChartBox>
      )
    }
    return (
      <ChartBox>
        {height => (
          <Column
            data={chartRows}
            xField="__x"
            yField="__y"
            height={height}
            yAxis={{ label: { formatter: axisFormatter } }}
            tooltip={{ formatter: tooltipFormatter }}
          />
        )}
      </ChartBox>
    )
  }

  const menuItems: NonNullable<MenuProps['items']> = []
  if (!isText && result && result.sql) {
    menuItems.push({ key: 'sql', label: '查看生成的 SQL', onClick: () => setSqlOpen(true) })
  }
  if (editable && !readOnly) {
    if (actions && actions.duplicate) {
      if (menuItems.length > 0) menuItems.push({ type: 'divider' })
      menuItems.push({
        key: 'duplicate',
        icon: <CopyOutlined />,
        label: '复制组件',
        onClick: () => actions.duplicate && actions.duplicate(id),
      })
    }
    if (actions && actions.remove) {
      menuItems.push({
        key: 'delete',
        icon: <DeleteOutlined />,
        label: '删除组件',
        danger: true,
        onClick: () => actions.remove && actions.remove(id),
      })
    }
  }

  const showExtra = !readOnly && (editable || !!(actions && actions.refresh))

  return (
    <Card
      size="small"
      className={`widget-card${isText ? ' widget-card--text' : ''}${className ? ` ${className}` : ''}`}
      style={style}
      title={<span>{title || (isText ? '文本' : '未命名组件')}</span>}
      extra={showExtra
        ? (
          <>
            {editable && actions && actions.config
              ? (
                <Button
                  size="small"
                  type="text"
                  icon={<SettingOutlined />}
                  onClick={() => actions.config && actions.config(id)}
                  title="配置这个组件"
                />
              )
              : null}
            {actions && actions.refresh
              ? (
                <Button
                  size="small"
                  type="text"
                  icon={<ReloadOutlined />}
                  onClick={() => actions.refresh && actions.refresh(id)}
                  title="重新取数"
                />
              )
              : null}
            {menuItems.length > 0
              ? (
                <Dropdown menu={{ items: menuItems }} trigger={['click']}>
                  <Button size="small" type="text" icon={<MoreOutlined />} />
                </Dropdown>
              )
              : null}
          </>
        )
        : undefined}
    >
      {body()}
      <Modal
        title="平台生成的 SQL（只读）"
        open={sqlOpen}
        footer={null}
        onCancel={() => setSqlOpen(false)}
      >
        <Paragraph style={{ whiteSpace: 'pre-wrap', fontFamily: 'Menlo, Consolas, monospace', fontSize: 12 }}>
          {result && result.sql ? result.sql : '（还没有取到 SQL）'}
        </Paragraph>
      </Modal>
    </Card>
  )
}

// 拖动布局时 props 不变 → 跳过重渲染（图表不重画）
export default React.memo(WidgetCard)
