import React, { useEffect, useMemo, useRef, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import {
  Alert, Button, Drawer, Dropdown, Empty, Input, MenuProps, Segmented, Select, Space, Spin, Typography, message,
} from 'antd'
import {
  ArrowLeftOutlined, EditOutlined, EyeOutlined, FullscreenOutlined, MoreOutlined, ReloadOutlined,
  SaveOutlined, SettingOutlined,
} from '@ant-design/icons'
import { GridStack as GridStackCore } from 'gridstack'
import {
  createVizBoard, getVizBoard, updateVizBoard,
  VizCanvas, VizWidget,
} from '@/api'
import GridCanvas, { GridCanvasDropNode, GridCanvasItem } from '@/components/GridCanvas'
import WidgetCard, { WidgetActions } from '@/components/WidgetCard'
import WidgetBody from '@/components/viz/WidgetBody'
import { effectiveInterval, useVizData } from '@/components/viz/useVizData'
import { getWidgetManifest, paletteManifests } from '@/components/viz/registry'
import type { VizWidgetManifest } from '@/components/viz/types'
import { SettingGroup, SettingRow } from '@/components/viz/widgets/SettingsLayout'
import { collectDescendantIds, flattenVizTree, groupByParent, isVizContainer } from '@/utils/vizTree'
import './index.less'
import type { GridStackNode } from 'gridstack'
import { t, tt, currentLocale } from '@/i18n'

const { Text } = Typography

const COLS = 12
const ROW_HEIGHT = 40

/** 自动刷新间隔可选项（秒，0 = 关）；真正的展示页按它轮询取数 */
const REFRESH_OPTIONS = [
  { label: t('不自动刷新'), value: 0 },
  { label: t('10 秒'), value: 10 },
  { label: t('30 秒'), value: 30 },
  { label: t('1 分钟'), value: 60 },
  { label: t('5 分钟'), value: 300 },
  { label: t('15 分钟'), value: 900 },
  { label: t('1 小时'), value: 3600 },
]

/** 组件级刷新间隔：0 = 跟随看板（组件不配就跟整页一致） */
const WIDGET_REFRESH_OPTIONS = [
  { label: t('跟随看板'), value: 0 },
  ...REFRESH_OPTIONS.slice(1),
]

/** setupDragIn 的选择器（只有组件库里的模板项能拖进画布） */
const PALETTE_SELECTOR = '.board-editor__palette-item--draggable'

/**
 * 拖拽把手：卡片用标题栏，容器用自己的标题栏。
 * gridstack 会把"最近 .grid-stack-item 不是自己"的把手过滤掉，所以子网格里的卡片标题
 * 不会误触发外层容器的拖动。
 */
const DRAG_HANDLE = '.ant-card-head, .board-editor__container-head'

const newWidgetId = () => `w${Date.now().toString(36)}${Math.random().toString(36).slice(2, 6)}`

/** 模板项 → 新组件（落位/点击添加共用）：默认标题与配置都来自组件注册表 */
const buildWidgetFromManifest = (
  manifest: VizWidgetManifest,
  id: string,
  layout: { x: number, y: number, w: number, h: number },
): VizWidget => {
  const widget: VizWidget = {
    id,
    type: manifest.type,
    title: manifest.label,
    showTitle: true,
    layout,
    config: manifest.defaultConfig(),
  }
  if (manifest.type === 'text') {
    widget.text = ''
  }
  if (manifest.layoutOnly) widget.children = []
  return widget
}

/**
 * 看板编辑器：
 *   - 左侧「组件库」面板：点一下添加，或把模板项拖到画布上落位（组件清单来自注册表）
 *   - 拖动标题栏移动、拖右下角改大小
 *   - 组件卡片右上角菜单 →「配置本组件」：抽屉里是各组件自己的配置面板
 *   - 数据类组件在编辑态也会取一次数，方便边配边看
 */
const BoardEdit: React.FC = () => {
  const { id = '' } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const isNew = id === 'new'

  // 看板正文按语言分开存：英文编辑 textEn，中文编辑 text
  const english = currentLocale() === 'en-US'
  const [name, setName] = useState(t('新建看板'))
  // widgets 是**扁平**的（含所有层级），归属关系单独放 parentOf：跨子网格拖拽只改归属
  const [widgets, setWidgets] = useState<VizWidget[]>([])
  const [parentOf, setParentOf] = useState<Record<string, string>>({})
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [dirty, setDirty] = useState(false)
  const [readOnly, setReadOnly] = useState(false)
  const [error, setError] = useState('')
  // 整页自动刷新间隔（秒）：只影响展示，编辑态不轮询
  const [refreshInterval, setRefreshInterval] = useState(0)
  // 组件库面板：常驻左侧（可收起），模板项可直接拖到画布上
  const [paletteOpen, setPaletteOpen] = useState(true)
  // 网格就绪计数：每次有网格（含子网格）挂好，就重新注册一次"组件库可拖入"
  const [gridTick, setGridTick] = useState(0)
  // 正在配置的组件 id（右侧配置抽屉）
  const [settingsId, setSettingsId] = useState('')
  // 拖拽刚落下的那一下 click 要吞掉，否则会"拖入一个 + 点击再加一个"
  const dragJustEndedRef = useRef(false)

  const palette = useMemo(() => paletteManifests(), [])
  const childrenByParent = useMemo(() => groupByParent(widgets, parentOf), [widgets, parentOf])
  const ordered = childrenByParent.get('') || []
  const widgetById = useMemo(() => new Map(widgets.map(w => [w.id, w])), [widgets])

  // 取数：编辑态不轮询（refreshInterval 传 0），手动点「刷新数据」；只读态按看板配置轮询
  const data = useVizData(widgets, readOnly ? refreshInterval : 0, true)

  /* ---------- 加载 ---------- */

  useEffect(() => {
    if (isNew) {
      setWidgets([])
      setParentOf({})
      setName(t('新建看板'))
      setRefreshInterval(0)
      setLoading(false)
      return
    }
    if (!id) return
    setLoading(true)
    getVizBoard(id)
      .then((resp) => {
        if (resp.success) {
          const flat = flattenVizTree((resp.result.canvas && resp.result.canvas.widgets) || [])
          setName(resp.result.name)
          setRefreshInterval((resp.result.canvas && resp.result.canvas.refreshInterval) || 0)
          setWidgets(flat.widgets)
          setParentOf(flat.parentOf)
        } else {
          setError(resp.errorMsg || t('看板加载失败'))
        }
      })
      .catch((e: any) => setError(e?.message || t('看板加载失败')))
      .finally(() => setLoading(false))
  }, [id, isNew])

  /* ---------- 组件增删 ---------- */

  const markDirty = () => setDirty(true)

  /** 顶层最后一个组件的下方 */
  const nextLayout = (w: number, h: number) => {
    const last = ordered.length > 0 ? ordered[ordered.length - 1] : null
    return { x: 0, y: last ? last.layout.y + last.layout.h : 0, w, h }
  }

  /** 面板项点击 = 落到画布底部（拖入不可用时的保底路径） */
  const addPaletteWidget = (manifest: VizWidgetManifest) => {
    if (dragJustEndedRef.current) return
    const layout = nextLayout(manifest.defaultSize.w, manifest.defaultSize.h)
    const widget = buildWidgetFromManifest(manifest, newWidgetId(), layout)
    setWidgets(prev => prev.concat([widget]))
    setParentOf(prev => ({ ...prev, [widget.id]: '' }))
    markDirty()
    // 数据类组件直接打开配置抽屉，省得用户再找
    if (manifest.data && manifest.Settings) setSettingsId(widget.id)
  }

  /* ---------- 网格事件 ---------- */

  /** 网格就绪（顶层与子网格都会回调）：触发一次拖入注册 */
  const handleGridReady = () => {
    setGridTick(t => t + 1)
  }

  // 组件库 → 画布 拖入。setupDragIn 对已注册过的元素会跳过，重复调用是安全的；
  // 面板收起时元素被卸载，重新展开后需要再注册一次，所以依赖里带上 paletteOpen。
  useEffect(() => {
    if (!paletteOpen || !gridTick || readOnly) return undefined
    GridStackCore.setupDragIn(
      PALETTE_SELECTOR,
      { helper: 'clone', appendTo: 'body', scroll: true },
      palette.map(m => ({
        w: m.defaultSize.w,
        h: m.defaultSize.h,
        autoPosition: true,
        props: { template: m.type },
      })),
    )
    // setupDragIn 结束时会在源元素上派发 dragstop；用它标记"这一次是拖拽，不是点击"
    const nodes = Array.from(document.querySelectorAll<HTMLElement>(PALETTE_SELECTOR))
    const onDragStart = () => { dragJustEndedRef.current = true }
    const onDragStop = () => {
      window.setTimeout(() => { dragJustEndedRef.current = false }, 300)
    }
    nodes.forEach((node) => {
      node.addEventListener('dragstart', onDragStart)
      node.addEventListener('dragstop', onDragStop)
    })
    return () => {
      nodes.forEach((node) => {
        node.removeEventListener('dragstart', onDragStart)
        node.removeEventListener('dragstop', onDragStop)
      })
    }
  }, [paletteOpen, gridTick, readOnly, palette])

  /**
   * 从组件库拖入 / 跨网格搬迁后收口：gridstack 已经把节点放在落点上了。
   *   - 没有 previousNode：外部拖入，补 id 并转成新组件
   *   - 有 previousNode：在两块网格之间搬，改 parentOf 归属即可（DOM 已由 gridstack 挪过去）
   */
  const handleDroppedWidget = (node: GridCanvasDropNode, previousNode: GridStackNode | undefined, containerId: string) => {
    if (readOnly) return
    if (previousNode) {
      const movedId = previousNode.id !== undefined && previousNode.id !== null ? String(previousNode.id) : ''
      if (!movedId) return
      // 跨网格搬迁：DOM 已由 gridstack 挪过去，这里同步归属与落点坐标
      // （坐标必须一起更新，否则 GridStackItem 挂载时会用旧坐标把组件挪回去）
      const layout = { x: node.x || 0, y: node.y || 0, w: node.w || 1, h: node.h || 1 }
      setWidgets(prev => prev.map(w => (w.id === movedId ? { ...w, layout } : w)))
      setParentOf(prev => ({ ...prev, [movedId]: containerId }))
      markDirty()
      return
    }
    const raw = node.props ? node.props.template : undefined
    const template = typeof raw === 'string' ? raw : ''
    const manifest = palette.find(m => m.type === template)
    if (!manifest) return
    const widgetId = newWidgetId()
    node.id = widgetId
    if (node.el) node.el.setAttribute('gs-id', widgetId)
    const widget = buildWidgetFromManifest(manifest, widgetId, {
      x: node.x || 0,
      y: node.y || 0,
      w: node.w || manifest.defaultSize.w,
      h: node.h || manifest.defaultSize.h,
    })
    setWidgets(prev => prev.concat([widget]))
    setParentOf(prev => ({ ...prev, [widgetId]: containerId }))
    markDirty()
    if (manifest.data && manifest.Settings) setSettingsId(widgetId)
  }

  /**
   * 拖动 / 缩放结束：把网格里的实际位置写回（含被顺带上浮的其它组件）。
   * containerId 是回报的这块网格属于哪个容器（顶层为空串），据此修正归属。
   */
  const handleLayoutChange = (next: GridCanvasItem[], containerId: string) => {
    if (readOnly) return
    const byId = new Map(next.map(n => [n.id, n]))
    let changed = false
    const merged = widgets.map((w) => {
      const hit = byId.get(w.id)
      if (!hit) return w
      if (hit.x === w.layout.x && hit.y === w.layout.y && hit.w === w.layout.w && hit.h === w.layout.h) return w
      changed = true
      return { ...w, layout: { x: hit.x, y: hit.y, w: hit.w, h: hit.h } }
    })
    const reparented = next.some(n => widgetById.has(n.id) && (parentOf[n.id] || '') !== containerId)
    if (!changed && !reparented) return
    if (changed) setWidgets(merged)
    if (reparented) {
      setParentOf((prev) => {
        const copy = { ...prev }
        next.forEach((n) => {
          if (!widgetById.has(n.id)) return
          if (containerId) copy[n.id] = containerId
          else delete copy[n.id]
        })
        return copy
      })
    }
    setDirty(true)
  }

  const updateWidget = (widgetId: string, patch: Partial<VizWidget>) => {
    setWidgets(prev => prev.map(w => (w.id === widgetId ? { ...w, ...patch } : w)))
    markDirty()
  }

  /** 组件自己的配置面板改配置：合并进 widget.config */
  const patchWidgetConfig = (widgetId: string, patch: Record<string, any>) => {
    setWidgets(prev => prev.map(w => (
      w.id === widgetId ? { ...w, config: { ...(w.config || {}), ...patch } } : w
    )))
    markDirty()
  }

  const duplicateById = (widgetId: string) => {
    const widget = widgets.find(w => w.id === widgetId)
    if (widget) duplicateWidget(widget)
  }

  const removeById = (widgetId: string) => removeWidget(widgetId)

  const setTextById = (widgetId: string, text: string) => updateWidget(widgetId, english ? { textEn: text } : { text })

  // 事件处理放进 ref：actions 本身引用不变，React.memo 才能挡住拖动时的重渲染
  const handlersRef = useRef({ duplicateById, removeById })
  useEffect(() => {
    handlersRef.current = { duplicateById, removeById }
  })

  const actions = useMemo<WidgetActions>(() => ({
    duplicate: (widgetId: string) => handlersRef.current.duplicateById(widgetId),
    remove: (widgetId: string) => handlersRef.current.removeById(widgetId),
  }), [])

  const duplicateWidget = (widget: VizWidget) => {
    // 容器要连子级一起复制（全部换新 id，归属改到副本上）
    const idMap: Record<string, string> = {}
    const copies: VizWidget[] = []
    const localParents: Record<string, string> = {}

    const walk = (src: VizWidget, parentId: string) => {
      const nextId = newWidgetId()
      idMap[src.id] = nextId
      const copy: VizWidget = {
        ...src,
        id: nextId,
        config: src.config ? JSON.parse(JSON.stringify(src.config)) : src.config,
      }
      if (isVizContainer(src)) copy.children = []
      copies.push(copy)
      localParents[nextId] = parentId
      ;(src.children || []).forEach(child => walk(child, nextId))
    }
    walk(widget, parentOf[widget.id] || '')

    const rootId = idMap[widget.id]
    // 顶层副本挪到最后；容器内部的相对布局原样保留
    copies.forEach((copy) => {
      if (copy.id === rootId) copy.layout = nextLayout(widget.layout.w, widget.layout.h)
    })

    setWidgets(prev => prev.concat(copies))
    setParentOf(prev => ({ ...prev, ...localParents }))
    markDirty()
  }

  const removeWidget = (widgetId: string) => {
    // 容器连同子级一起删（后端也是整棵树整体覆盖）
    const doomed = [widgetId].concat(collectDescendantIds(widgetId, parentOf))
    const doomedSet = new Set(doomed)
    setWidgets(prev => prev.filter(w => !doomedSet.has(w.id)))
    setParentOf((prev) => {
      const copy = { ...prev }
      doomed.forEach(wid => delete copy[wid])
      return copy
    })
    if (settingsId && doomedSet.has(settingsId)) setSettingsId('')
    markDirty()
  }

  /** 容器：把子级提到容器的父级，再删掉容器本身 */
  const unwrapContainer = (containerId: string) => {
    const parent = parentOf[containerId] || ''
    const kids = childrenByParent.get(containerId) || []
    const kidIds = kids.map(k => k.id)
    const doomed = collectDescendantIds(containerId, parentOf)
    const doomedSet = new Set([containerId].concat(doomed))
    setWidgets(prev => prev.filter(w => !doomedSet.has(w.id)))
    setParentOf((prev) => {
      const copy = { ...prev }
      doomedSet.forEach(wid => delete copy[wid])
      kidIds.forEach(wid => { copy[wid] = parent })
      return copy
    })
    message.success(t('已取消容器，里面的组件回到了上一层'))
    markDirty()
  }

  /** 拖入容器的合法性：容器不能再套容器（后端最多 2 层） */
  const acceptIntoContainer = (el: Element) => {
    const node = (el as HTMLElement & { gridstackNode?: { id?: string, props?: Record<string, unknown> } }).gridstackNode
    if (!node) return true
    // 组件库模板：容器不能拖进容器，其它模板随便放
    if (node.props && node.props.template !== undefined) return node.props.template !== 'container'
    const draggedId = node.id !== undefined && node.id !== null ? String(node.id) : ''
    if (!draggedId) return true
    return !isVizContainer(widgetById.get(draggedId))
  }

  /* ---------- 保存 ---------- */

  const canvasPayload = (): VizCanvas => {
    const clamp = (w: VizWidget): VizWidget => ({
      ...w,
      type: w.type || 'text',
      layout: {
        x: Math.max(0, Math.min(w.layout.x, COLS - 1)),
        y: Math.max(0, w.layout.y),
        w: Math.max(1, Math.min(w.layout.w, COLS)),
        h: Math.max(2, w.layout.h),
      },
    })
    // 扁平状态 → 嵌套树（只按归属组装，children 由我们算出来，不用 state 里那份）
    const build = (parentId: string): VizWidget[] => (childrenByParent.get(parentId) || []).map((w) => {
      const next = clamp({ ...w })
      if (isVizContainer(w)) next.children = build(w.id)
      else delete next.children
      return next
    })
    return { cols: COLS, rowHeight: ROW_HEIGHT, refreshInterval, widgets: build('') }
  }

  const save = async () => {
    if (!name.trim()) {
      message.warning(t('请先给看板起个名字'))
      return
    }
    setSaving(true)
    try {
      const payload = { name: name.trim(), canvas: canvasPayload() }
      const resp = isNew ? await createVizBoard(payload) : await updateVizBoard(id, payload)
      if (resp.success) {
        message.success(t('看板已保存'))
        setDirty(false)
        const newId = resp.result && resp.result.id
        if (isNew && newId) {
          navigate(`/viz/boards/${newId}/edit`, { replace: true })
        }
      } else {
        message.error(resp.errorMsg || t('保存失败'))
      }
    } catch (e: any) {
      message.error(e?.message || t('保存失败'))
    } finally {
      setSaving(false)
    }
  }

  const goBack = () => {
    if (dirty && !window.confirm(t('有未保存的修改，确定离开吗？'))) return
    navigate('/viz/boards')
  }

  if (loading) {
    return <div style={{ padding: 48, textAlign: 'center' }}><Spin /></div>
  }

  /** 某一层（顶层 parentId=''）的组件 → 网格项 */
  const toItems = (parentId: string): GridCanvasItem[] =>
    (childrenByParent.get(parentId) || []).map(w => ({
      id: w.id, x: w.layout.x, y: w.layout.y, w: w.layout.w, h: w.layout.h,
    }))

  /** 渲染一个组件：容器套一层子网格并递归，其它类型交给注册表 */
  const renderWidget = (item: GridCanvasItem): React.ReactNode => {
    const w = widgetById.get(item.id)
    if (!w) return null
    const manifest = getWidgetManifest(w.type)
    if (isVizContainer(w)) {
      const kids = toItems(w.id)
      const menu: NonNullable<MenuProps['items']> = []
      if (!readOnly) {
        menu.push(
          { key: 'configure', label: t('重命名容器'), onClick: () => setSettingsId(w.id) },
          { type: 'divider' },
          { key: 'unwrap', label: t('取消容器（组件回到上一层）'), onClick: () => unwrapContainer(w.id) },
          { type: 'divider' },
          { key: 'remove', label: t('删除容器（含里面的组件）'), danger: true, onClick: () => removeWidget(w.id) },
        )
      }
      return (
        <div className="board-editor__container">
          <div className="board-editor__container-head">
            <span className="board-editor__container-title">{tt(w.title) || t('容器')}</span>
            {menu.length > 0
              ? (
                <Dropdown menu={{ items: menu }} trigger={['click']}>
                  <Button size="small" type="text" icon={<MoreOutlined />} />
                </Dropdown>
              )
              : null}
          </div>
          <div className="board-editor__container-body">
            <GridCanvas
              boardKey={`${id}:${w.id}`}
              items={kids}
              cols={COLS}
              rowHeight={ROW_HEIGHT}
              readOnly={readOnly}
              className="board-editor__subgrid"
              dragHandle={DRAG_HANDLE}
              acceptWidgets={acceptIntoContainer}
              onReady={handleGridReady}
              onLayoutChange={(next) => handleLayoutChange(next, w.id)}
              onDroppedWidget={(node, previous) => handleDroppedWidget(node, previous, w.id)}
              renderItem={renderWidget}
            />
            {kids.length === 0 ? <div className="board-editor__container-empty">{t('把组件拖进来')}</div> : null}
          </div>
        </div>
      )
    }

    return (
      <WidgetCard
        id={w.id}
        actions={actions}
        title={w.title}
        type={manifest ? manifest.type : w.type}
        editable={!readOnly}
        readOnly={readOnly}
        onConfigure={manifest ? (widgetId: string) => setSettingsId(widgetId) : undefined}
      >
        <WidgetBody
          widget={w}
          ctx={{
            editable: !readOnly,
            readOnly,
            data: data.results[w.id],
            loading: data.loading,
            text: english ? (w.textEn || tt(w.text || '')) : (w.text || ''),
            // 编辑态不轮询；只读态由排行榜这类自带取数的组件自行按间隔刷新
            refreshInterval: readOnly ? effectiveInterval(w.refreshInterval, refreshInterval) : 0,
            onTextChange: (text: string) => setTextById(w.id, text),
            onConfigChange: (patch: Record<string, any>) => patchWidgetConfig(w.id, patch),
          }}
        />
      </WidgetCard>
    )
  }

  const settingsWidget = settingsId ? widgetById.get(settingsId) : undefined
  const settingsManifest = settingsWidget ? getWidgetManifest(settingsWidget.type) : undefined

  return (
    <div className="board-editor">
      <div className="board-editor__toolbar">
        <Space size={8}>
          <Button icon={<ArrowLeftOutlined />} onClick={goBack}>{t('返回')}</Button>
          {readOnly
            ? <Text strong style={{ fontSize: 15 }}>{name}</Text>
            : (
              <Input
                value={name}
                onChange={(e) => { setName(e.target.value); markDirty() }}
                style={{ width: 240 }}
                placeholder={t('看板名称')}
              />
            )}
          {dirty ? <Text type="warning" style={{ fontSize: 12 }}>{t('未保存')}</Text> : null}
        </Space>
        <Space size={12}>
          <Space size={4} style={{ flexShrink: 0 }}>
            {/* 英文比中文长：不加 nowrap 会被挤成一列单字 */}
            <Text type="secondary" style={{ fontSize: 12, whiteSpace: 'nowrap' }}>{t('自动刷新')}</Text>
            <Select
              size="small"
              style={{ width: 130 }}
              value={refreshInterval}
              options={REFRESH_OPTIONS}
              onChange={(v) => { setRefreshInterval(v); markDirty() }}
            />
          </Space>
          <Button icon={<ReloadOutlined />} onClick={data.refresh} loading={data.loading}>{t('刷新数据')}</Button>
          {!isNew
            ? (
              <Button
                icon={<FullscreenOutlined />}
                disabled={dirty}
                title={dirty ? t('请先保存再打开大屏') : t('在新窗口打开只读大屏')}
                onClick={() => window.open(`/viz/view/${id}`, '_blank')}
              >
                {t('大屏展示')}
              </Button>
            )
            : null}
          <Segmented
            value={readOnly ? 'view' : 'edit'}
            onChange={value => setReadOnly(String(value) === 'view')}
            options={[
              { label: <span><EditOutlined /> {t('编辑')}</span>, value: 'edit' },
              { label: <span><EyeOutlined /> {t('查看')}</span>, value: 'view' },
            ]}
          />
          <Button
            icon={<SettingOutlined />}
            type={paletteOpen ? 'primary' : 'default'}
            disabled={readOnly}
            onClick={() => setPaletteOpen(open => !open)}
          >
            {t('组件库')}
          </Button>
          <Button icon={<SaveOutlined />} loading={saving} disabled={readOnly} onClick={save}>{t('保存看板')}</Button>
        </Space>
      </div>

      {error ? <Alert type="error" showIcon style={{ margin: '0 16px 12px' }} message={error} /> : null}
      {data.error
        ? <Alert type="warning" showIcon style={{ margin: '0 16px 12px' }} message={t('部分数据获取失败')} description={data.error} />
        : null}

      <div className="board-editor__main">
        {paletteOpen && !readOnly
          ? (
            <aside className="board-editor__palette">
              <div className="board-editor__palette-group">{t('新建组件（可拖到画布或容器里）')}</div>
              <div className="board-editor__palette-list">
                {palette.map(manifest => (
                  <div
                    key={manifest.type}
                    className="board-editor__palette-item board-editor__palette-item--draggable"
                    onClick={() => addPaletteWidget(manifest)}
                  >
                    {manifest.icon}
                    <span>{manifest.label}</span>
                  </div>
                ))}
              </div>
            </aside>
          )
          : null}
        <div className="board-editor__body">
          <GridCanvas
            boardKey={id}
            items={toItems('')}
            cols={COLS}
            rowHeight={ROW_HEIGHT}
            margin={12}
            readOnly={readOnly}
            className="board-editor__grid"
            dragHandle={DRAG_HANDLE}
            onReady={handleGridReady}
            onLayoutChange={(next) => handleLayoutChange(next, '')}
            onDroppedWidget={(node, previous) => handleDroppedWidget(node, previous, '')}
            renderItem={renderWidget}
          />
          {widgets.length === 0
            ? (
              <div className="board-editor__empty">
                <Empty
                  image={Empty.PRESENTED_IMAGE_SIMPLE}
                  description={t('空看板 —— 从左侧「组件库」点一下，或把组件拖进来')}
                />
              </div>
            )
            : null}
        </div>
      </div>

      <Drawer
        title={settingsWidget && settingsManifest
          ? (
            <span className="board-editor__settings-title">
              {settingsManifest.icon}
              <span>{settingsManifest.label}</span>
              {settingsWidget.title
                ? <span className="board-editor__settings-title-sub">· {tt(settingsWidget.title)}</span>
                : null}
            </span>
          )
          : t('配置组件')}
        width={440}
        open={!!(settingsWidget && settingsManifest)}
        onClose={() => setSettingsId('')}
        bodyStyle={{ padding: '16px 20px 24px' }}
        destroyOnClose
      >
        {settingsWidget && settingsManifest
          ? (
            <div className="board-editor__settings">
              {/* 标题与刷新间隔是"组件"的通用属性（不属于各组件自己的 config），统一放第一张卡 */}
              <SettingGroup title={t('基本设置')}>
                <SettingRow label={t('标题')} hint={t('显示在组件卡片（容器）顶部；留空则用组件默认名。')}>
                  <Input
                    style={{ width: '100%' }}
                    placeholder={settingsManifest.label}
                    value={settingsWidget.title || ''}
                    allowClear
                    onChange={e => updateWidget(settingsWidget.id, { title: e.target.value })}
                    onBlur={() => {
                      // 空标题在保存时会被后端填回默认名，这里失焦即回退，所见即保存后的结果
                      if (!(settingsWidget.title || '').trim()) {
                        updateWidget(settingsWidget.id, { title: settingsManifest.label })
                      }
                    }}
                    onPressEnter={e => (e.target as HTMLInputElement).blur()}
                  />
                </SettingRow>
                {settingsManifest.data
                  ? (
                    <SettingRow
                      label={t('刷新间隔')}
                      hint={t('跟随看板 = 用顶部「自动刷新」的间隔（当前 {{p1}}）', { p1: refreshInterval ? t('{{count}} 秒', { count: refreshInterval }) : t('不自动刷新') })}
                    >
                      <Select
                        style={{ width: '100%' }}
                        value={settingsWidget.refreshInterval || 0}
                        options={WIDGET_REFRESH_OPTIONS}
                        onChange={(v) => updateWidget(settingsWidget.id, { refreshInterval: v })}
                      />
                    </SettingRow>
                  )
                  : null}
              </SettingGroup>
              {settingsManifest.Settings
                ? (
                  <settingsManifest.Settings
                    config={{ ...settingsManifest.defaultConfig(), ...(settingsWidget.config || {}) }}
                    onChange={(patch: Record<string, any>) => patchWidgetConfig(settingsWidget.id, patch)}
                  />
                )
                : null}
            </div>
          )
          : null}
      </Drawer>
    </div>
  )
}

export default BoardEdit
