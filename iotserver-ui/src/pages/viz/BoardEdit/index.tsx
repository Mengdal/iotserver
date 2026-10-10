import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import {
  Alert, Button, Divider, Drawer, Empty, Input, Popover, Segmented, Space, Spin, Typography, message,
} from 'antd'
import {
  ArrowLeftOutlined, BarChartOutlined, EyeOutlined, EditOutlined, FileTextOutlined,
  PlusOutlined, ReloadOutlined, SaveOutlined,
} from '@ant-design/icons'
import GridLayout, { WidthProvider } from 'react-grid-layout'
import {
  createVizBoard, getVizBoard, getVizBoards, getVizDatasources, queryVizBatch, updateVizBoard,
  VizBoardDetail, VizBoardSpec, VizCanvas, VizQueryResult, VizView, VizWidget, VizWidgetType,
} from '@/api'
import WidgetCard, { WidgetActions } from '@/components/WidgetCard'
import WidgetConfig, { defaultSpec } from '@/components/WidgetConfig'
import Field from '@/components/Field'
import 'react-grid-layout/css/styles.css'
import 'react-resizable/css/styles.css'
import './index.less'

const AutoGrid = WidthProvider(GridLayout)
const { Text } = Typography

const COLS = 12
const ROW_HEIGHT = 40

/** 新建图表组件的默认尺寸 */
const DEFAULT_SIZE: Record<string, { w: number, h: number }> = {
  value: { w: 3, h: 4 },
  line: { w: 12, h: 7 },
  bar: { w: 6, h: 7 },
  pie: { w: 6, h: 7 },
  table: { w: 12, h: 8 },
}

const newWidgetId = () => `w${Date.now().toString(36)}${Math.random().toString(36).slice(2, 6)}`

/**
 * 看板编辑器（最简版）：
 *   - 只能通过「+ 添加组件」创建：新建组件 / 从已有组件里挑 / 加文本
 *   - 拖动卡片标题栏移动、拖右下角改大小，没有其它排版按钮
 *   - 组件配置只有四件事：数据表 / 图形 / 维度 / 统计（时间字段自动）
 *   - 界面上没有 SQL、没有筛选器、没有参数
 */
const BoardEdit: React.FC = () => {
  const { id = '' } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const isNew = id === 'new'

  const [board, setBoard] = useState<VizBoardDetail | null>(null)
  const [name, setName] = useState('新建看板')
  const [widgets, setWidgets] = useState<VizWidget[]>([])
  const [results, setResults] = useState<Record<string, VizQueryResult>>({})
  const [views, setViews] = useState<VizView[]>([])
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [dirty, setDirty] = useState(false)
  const [readOnly, setReadOnly] = useState(false)
  const [selectedId, setSelectedId] = useState('')
  const [error, setError] = useState('')
  // 「添加组件」面板：已有组件列表（其它看板里配置好的，可直接复用）
  const [addOpen, setAddOpen] = useState(false)
  const [libQuery, setLibQuery] = useState('')
  const [library, setLibrary] = useState<{ key: string, title: string, type: VizWidgetType, text?: string, spec?: VizBoardSpec }[]>([])

  const queriedRef = useRef<Record<string, string>>({})

  const ordered = useMemo(() => widgets
    .map((w, index) => ({ w, index }))
    .sort((a, b) => (a.w.layout.y - b.w.layout.y) || (a.w.layout.x - b.w.layout.x) || (a.index - b.index))
    .map(item => item.w), [widgets])
  const selected = useMemo(() => widgets.find(w => w.id === selectedId), [widgets, selectedId])
  const selectedType: VizWidgetType = selected ? (selected.type || 'chart') : 'chart'

  /* ---------- 加载 ---------- */

  useEffect(() => {
    getVizDatasources()
      .then((resp) => { if (resp.success) setViews(resp.result.views || []) })
      .catch(() => { /* 配置面板会提示没有可用数据表 */ })
  }, [])

  useEffect(() => {
    getVizBoards()
      .then((resp) => {
        if (!resp.success) return
        const seen = new Set<string>()
        const items: { key: string, title: string, type: VizWidgetType, text?: string, spec?: VizBoardSpec }[] = []
        ;(resp.result.list || []).forEach((b) => {
          ((b.canvas && b.canvas.widgets) || []).forEach((w) => {
            const key = `${w.title}|${JSON.stringify(w.spec || {})}|${w.text || ''}`
            if (seen.has(key)) return
            seen.add(key)
            items.push({ key, title: w.title, type: (w.type || 'chart') as VizWidgetType, text: w.text, spec: w.spec })
          })
        })
        setLibrary(items)
      })
      .catch(() => { /* 拉不到只影响"从已有组件添加" */ })
  }, [id])

  useEffect(() => {
    if (isNew) {
      setBoard(null)
      setWidgets([])
      setName('新建看板')
      setLoading(false)
      return
    }
    if (!id) return
    setLoading(true)
    getVizBoard(id)
      .then((resp) => {
        if (resp.success) {
          setBoard(resp.result)
          setName(resp.result.name)
          setWidgets((resp.result.canvas && resp.result.canvas.widgets) || [])
        } else {
          setError(resp.errorMsg || '看板加载失败')
        }
      })
      .catch((e: any) => setError(e?.message || '看板加载失败'))
      .finally(() => setLoading(false))
  }, [id, isNew])

  /* ---------- 取数（spec 变了才请求） ---------- */

  const fetchChanged = useCallback(async (list: VizWidget[]) => {
    const charts = list.filter(w => (w.type || 'chart') !== 'text')
    const signature = (w: VizWidget) => JSON.stringify(w.spec)
    const changed = charts.filter(w => queriedRef.current[w.id] !== signature(w))
    if (changed.length === 0) return
    try {
      const resp = await queryVizBatch(changed.map(w => ({ id: w.id, name: w.title, spec: w.spec })))
      if (resp.success) {
        const next: Record<string, VizQueryResult> = {}
        ;(resp.result.results || []).forEach((item) => { next[item.id || ''] = item })
        changed.forEach((w) => { queriedRef.current[w.id] = signature(w) })
        setResults(prev => ({ ...prev, ...next }))
      } else {
        message.error(resp.errorMsg || '取数失败')
      }
    } catch (e: any) {
      message.error(e?.message || '取数失败')
    }
  }, [])

  useEffect(() => {
    if (widgets.length === 0) return undefined
    const timer = setTimeout(() => { fetchChanged(widgets) }, 300)
    return () => clearTimeout(timer)
  }, [widgets, fetchChanged])

  /* ---------- 组件增删 ---------- */

  const markDirty = () => setDirty(true)

  const nextLayout = (w: number, h: number) => {
    const last = ordered.length > 0 ? ordered[ordered.length - 1] : null
    return { x: 0, y: last ? last.layout.y + last.layout.h : 0, w, h }
  }

  const addChartWidget = (chart: VizBoardSpec['chart'] = 'value') => {
    const size = DEFAULT_SIZE[chart] || DEFAULT_SIZE.value
    const widget: VizWidget = {
      id: newWidgetId(),
      type: 'chart',
      title: '新组件',
      showTitle: true,
      layout: nextLayout(size.w, size.h),
      spec: defaultSpec(views.length > 0 ? views[0].view : '', chart),
    }
    setWidgets(prev => prev.concat([widget]))
    setSelectedId(widget.id)
    setAddOpen(false)
    markDirty()
  }

  const addTextWidget = () => {
    const widget: VizWidget = {
      id: newWidgetId(),
      type: 'text',
      title: '文本',
      showTitle: true,
      text: '',
      layout: nextLayout(6, 4),
      spec: defaultSpec(views.length > 0 ? views[0].view : ''),
    }
    setWidgets(prev => prev.concat([widget]))
    setSelectedId('')
    setAddOpen(false)
    markDirty()
  }

  const addFromLibrary = (item: { title: string, type: VizWidgetType, text?: string, spec?: VizBoardSpec }) => {
    const size = item.type === 'text' ? { w: 6, h: 4 } : (DEFAULT_SIZE[item.spec ? item.spec.chart : 'value'] || DEFAULT_SIZE.value)
    const widget: VizWidget = {
      id: newWidgetId(),
      type: item.type,
      title: item.title,
      showTitle: true,
      text: item.text || '',
      layout: nextLayout(size.w, size.h),
      spec: item.spec || defaultSpec(views.length > 0 ? views[0].view : ''),
    }
    setWidgets(prev => prev.concat([widget]))
    setSelectedId(widget.type === 'chart' ? widget.id : '')
    setAddOpen(false)
    markDirty()
  }

  const updateWidget = (widgetId: string, patch: Partial<VizWidget>) => {
    setWidgets(prev => prev.map(w => (w.id === widgetId ? { ...w, ...patch } : w)))
    markDirty()
  }

  const duplicateById = (widgetId: string) => {
    const widget = widgets.find(w => w.id === widgetId)
    if (widget) duplicateWidget(widget)
  }

  const removeById = (widgetId: string) => removeWidget(widgetId)

  const refreshById = (widgetId: string) => {
    const widget = widgets.find(w => w.id === widgetId)
    if (!widget) return
    delete queriedRef.current[widgetId]
    fetchChanged([widget])
  }

  const setTextById = (widgetId: string, text: string) => updateWidget(widgetId, { text })

  // 事件处理放进 ref：actions 本身引用不变，React.memo 才能挡住拖动时的重渲染
  const handlersRef = useRef({
    duplicateById,
    removeById,
    refreshById,
    setTextById,
  })
  useEffect(() => {
    handlersRef.current = {
      duplicateById,
      removeById,
      refreshById,
      setTextById,
    }
  })

  const actions = useMemo<WidgetActions>(() => ({
    config: (widgetId: string) => setSelectedId(widgetId),
    duplicate: (widgetId: string) => handlersRef.current.duplicateById(widgetId),
    remove: (widgetId: string) => handlersRef.current.removeById(widgetId),
    refresh: (widgetId: string) => handlersRef.current.refreshById(widgetId),
    textChange: (widgetId: string, text: string) => handlersRef.current.setTextById(widgetId, text),
  }), [])

  const duplicateWidget = (widget: VizWidget) => {
    const copy: VizWidget = {
      ...widget,
      id: newWidgetId(),
      layout: nextLayout(widget.layout.w, widget.layout.h),
    }
    setWidgets(prev => prev.concat([copy]))
    setSelectedId(copy.id)
    markDirty()
  }

  const removeWidget = (widgetId: string) => {
    setWidgets(prev => prev.filter(w => w.id !== widgetId))
    delete queriedRef.current[widgetId]
    if (selectedId === widgetId) setSelectedId('')
    markDirty()
  }

  const refreshAll = () => {
    queriedRef.current = {}
    fetchChanged(widgets)
  }

  const filteredLibrary = library.filter(item => item.title.toLowerCase().indexOf(libQuery.trim().toLowerCase()) >= 0)

  const addPanel = (
    <div className="board-editor__add-panel">
      <Input
        size="small"
        allowClear
        placeholder="搜索已有组件"
        value={libQuery}
        onChange={e => setLibQuery(e.target.value)}
      />
      <div className="board-editor__add-group">用已有组件</div>
      <div className="board-editor__add-list">
        {filteredLibrary.map(item => (
          <div key={item.key} className="board-editor__add-item" onClick={() => addFromLibrary(item)}>
            {item.type === 'text' ? <FileTextOutlined /> : <BarChartOutlined />}
            <span>{item.title}</span>
          </div>
        ))}
        {filteredLibrary.length === 0 ? <div className="board-editor__add-empty">还没有可复用的组件</div> : null}
      </div>
      <Divider style={{ margin: '8px 0' }} />
      <div className="board-editor__add-item" onClick={() => addChartWidget('value')}>
        <PlusOutlined />
        <span>新建组件</span>
      </div>
      <div className="board-editor__add-item" onClick={addTextWidget}>
        <FileTextOutlined />
        <span>添加文本</span>
      </div>
    </div>
  )

  /* ---------- 保存 ---------- */

  const canvasPayload = (): VizCanvas => ({
    cols: COLS,
    rowHeight: ROW_HEIGHT,
    widgets: widgets.map(w => ({
      ...w,
      type: w.type || 'chart',
      layout: {
        x: Math.max(0, Math.min(w.layout.x, COLS - 1)),
        y: Math.max(0, w.layout.y),
        w: Math.max(1, Math.min(w.layout.w, COLS)),
        h: Math.max(2, w.layout.h),
      },
    })),
  })

  const save = async () => {
    if (!name.trim()) {
      message.warning('请先给看板起个名字')
      return
    }
    setSaving(true)
    try {
      const payload = { name: name.trim(), kind: 'canvas' as const, canvas: canvasPayload() }
      const resp = isNew ? await createVizBoard(payload) : await updateVizBoard(id, payload)
      if (resp.success) {
        message.success('看板已保存')
        setDirty(false)
        const newId = resp.result && resp.result.id
        if (isNew && newId) {
          navigate(`/viz/boards/${newId}/edit`, { replace: true })
        } else {
          setBoard(prev => (prev ? { ...prev, name: name.trim(), canvas: canvasPayload() } : prev))
        }
      } else {
        message.error(resp.errorMsg || '保存失败')
      }
    } catch (e: any) {
      message.error(e?.message || '保存失败')
    } finally {
      setSaving(false)
    }
  }

  const goBack = () => {
    if (dirty && !window.confirm('有未保存的修改，确定离开吗？')) return
    navigate('/viz/boards')
  }

  if (loading) {
    return <div style={{ padding: 48, textAlign: 'center' }}><Spin /></div>
  }

  return (
    <div className="board-editor">
      <div className="board-editor__toolbar">
        <Space size={8}>
          <Button icon={<ArrowLeftOutlined />} onClick={goBack}>返回</Button>
          {readOnly
            ? <Text strong style={{ fontSize: 15 }}>{name}</Text>
            : (
              <Input
                value={name}
                onChange={(e) => { setName(e.target.value); markDirty() }}
                style={{ width: 240 }}
                placeholder="看板名称"
              />
            )}
          {dirty ? <Text type="warning" style={{ fontSize: 12 }}>未保存</Text> : null}
        </Space>
        <Space size={12}>
          <Segmented
            value={readOnly ? 'view' : 'edit'}
            onChange={value => setReadOnly(String(value) === 'view')}
            options={[
              { label: <span><EditOutlined /> 编辑</span>, value: 'edit' },
              { label: <span><EyeOutlined /> 查看</span>, value: 'view' },
            ]}
          />
          <Popover
            content={addPanel}
            trigger="click"
            placement="bottomRight"
            open={readOnly ? false : addOpen}
            onOpenChange={setAddOpen}
          >
            <Button type="primary" icon={<PlusOutlined />} disabled={readOnly}>添加组件</Button>
          </Popover>
          <Button icon={<ReloadOutlined />} onClick={refreshAll}>刷新数据</Button>
          <Button icon={<SaveOutlined />} loading={saving} disabled={readOnly} onClick={save}>保存看板</Button>
        </Space>
      </div>

      {board && board.kind !== 'canvas'
        ? (
          <Alert
            type="info"
            showIcon
            style={{ margin: '0 16px 12px' }}
            message="这是早期用 SQL 建的 Shaper 看板"
            description="当前编辑器只编辑平台看板；保存后会写成看板（画布）格式。"
          />
        )
        : null}
      {error ? <Alert type="error" showIcon style={{ margin: '0 16px 12px' }} message={error} /> : null}

      <div className="board-editor__body">
        {widgets.length === 0
          ? (
            <Empty
              image={Empty.PRESENTED_IMAGE_SIMPLE}
              description="空看板 —— 用右上角「添加组件」开始"
              style={{ marginTop: 120 }}
            />
          )
          : null}
        <AutoGrid
          className="board-editor__grid"
          layout={ordered.map(w => ({ i: w.id, x: w.layout.x, y: w.layout.y, w: w.layout.w, h: w.layout.h }))}
          cols={COLS}
          rowHeight={ROW_HEIGHT}
          margin={[12, 12]}
          containerPadding={[12, 12]}
          isDraggable={!readOnly}
          isResizable={!readOnly}
          draggableHandle=".ant-card-head"
          draggableCancel=".ant-btn,button,input,textarea,.ant-dropdown-trigger,.ant-select,.ant-segmented"
          compactType="vertical"
          onLayoutChange={(layout) => {
            if (readOnly) return
            let moved = false
            setWidgets(prev => prev.map((w) => {
              const hit = layout.find(l => l.i === w.id)
              if (!hit) return w
              if (hit.x === w.layout.x && hit.y === w.layout.y && hit.w === w.layout.w && hit.h === w.layout.h) return w
              moved = true
              return { ...w, layout: { x: hit.x, y: hit.y, w: hit.w, h: hit.h } }
            }))
            // 拖动/缩放也是改动，要标"未保存"，否则离开时不会提示、可能丢布局
            if (moved) setDirty(true)
          }}
        >
          {ordered.map(w => (
            <div
              key={w.id}
              data-widget-id={w.id}
              className={selectedId === w.id && !readOnly ? 'board-editor__item--selected' : undefined}
            >
              <WidgetCard
                id={w.id}
                actions={actions}
                title={w.title}
                type={w.type || 'chart'}
                text={w.text}
                spec={w.spec}
                result={results[w.id]}
                loading={(w.type || 'chart') === 'chart' && !results[w.id]}
                error={results[w.id] ? results[w.id].error : ''}
                editable={!readOnly}
                readOnly={readOnly}
              />
            </div>
          ))}
        </AutoGrid>
      </div>

      <Drawer
        title={selected ? `配置组件：${selected.title}` : '配置组件'}
        width={480}
        open={!!selected && selectedType === 'chart'}
        onClose={() => setSelectedId('')}
        destroyOnClose
      >
        {selected && selectedType === 'chart'
          ? (
            <div className="viz-form">
              <Field label="名称">
                <Input
                  value={selected.title}
                  onChange={(e) => updateWidget(selected.id, { title: e.target.value })}
                />
              </Field>
              <WidgetConfig
                value={selected.spec}
                views={views}
                onChange={(next) => updateWidget(selected.id, { spec: next })}
              />
            </div>
          )
          : null}
      </Drawer>
    </div>
  )
}

export default BoardEdit
