import React, { useEffect, useMemo, useState } from 'react'
import { Alert, Empty, Spin } from 'antd'
import { getVizBoard, VizBoardDetail, VizWidget } from '@/api'
import GridCanvas, { GridCanvasItem } from '@/components/GridCanvas'
import WidgetCard from '@/components/WidgetCard'
import WidgetBody from '@/components/viz/WidgetBody'
import { useVizData } from '@/components/viz/useVizData'
import { getWidgetManifest } from '@/components/viz/registry'
import { isPlatformOnlyWidget } from '@/components/viz/platformScope'
import { useVizMetricDefs } from '@/components/viz/widgets/MetricsEditor'
import { isPlatformView } from '@/utils/platformViewGuard'
import { flattenVizTree, groupByParent, isVizContainer } from '@/utils/vizTree'
import './index.less'
import { t } from '@/i18n'

interface CanvasBoardProps {
  /** 看板 ID（二选一） */
  boardId?: string
  /** 已加载的看板（二选一，避免重复请求） */
  board?: VizBoardDetail
  className?: string
  /** 关掉自动取数（例如打印/导出场景） */
  disableData?: boolean
}

/**
 * 画布看板的**只读**渲染（数据看板页、系统监控页、只读分享页用）：
 *   - 布局：按保存的坐标把组件摆在 gridstack 网格上（容器套一层子网格）；
 *   - 数据：整块画布一次批量取数，按 canvas.refreshInterval 定时静默刷新；
 *   - 组件内容由注册表渲染，这里不认识任何具体组件类型。
 */
const CanvasBoard: React.FC<CanvasBoardProps> = ({ boardId, board: boardProp, className, disableData }) => {
  const [board, setBoard] = useState<VizBoardDetail | undefined>(boardProp)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const widgets: VizWidget[] = useMemo(() => {
    if (!board || !board.canvas) return []
    return board.canvas.widgets || []
  }, [board])
  // 拍平 + 按父级分组：容器里的组件也要渲染
  const flat = useMemo(() => flattenVizTree(widgets), [widgets])
  // 租户视图：绑的全是平台专属指标的组件整体隐藏（如系统监控看板里的 CPU/内存块），
  // 而不是渲染成空图；绑了部分平台专属指标的（如混着通用块的统计卡）由组件自己按块隐藏
  const defs = useVizMetricDefs()
  const visibleWidgets = useMemo(() => {
    if (isPlatformView()) return flat.widgets
    const hidden = new Set(flat.widgets.filter(w => isPlatformOnlyWidget(w, defs)).map(w => w.id))
    if (hidden.size === 0) return flat.widgets
    return flat.widgets.filter((w) => {
      // 容器被隐藏时，子组件跟着一起隐藏
      let parent = flat.parentOf[w.id] || ''
      while (parent) {
        if (hidden.has(parent)) return false
        parent = flat.parentOf[parent] || ''
      }
      return !hidden.has(w.id)
    })
  }, [flat, defs])
  const childrenByParent = useMemo(() => groupByParent(visibleWidgets, flat.parentOf), [visibleWidgets, flat])

  const refreshInterval = (board && board.canvas && board.canvas.refreshInterval) || 0
  const data = useVizData(visibleWidgets, refreshInterval, !disableData)

  useEffect(() => {
    if (boardProp) {
      setBoard(boardProp)
      return
    }
    if (!boardId) return
    setLoading(true)
    getVizBoard(boardId)
      .then((resp) => {
        if (resp.success) {
          setBoard(resp.result)
        } else {
          setError(resp.errorMsg || t('看板加载失败'))
        }
      })
      .catch((e: any) => setError(e?.message || t('看板加载失败')))
      .finally(() => setLoading(false))
  }, [boardId, boardProp])

  if (loading) {
    return <div style={{ padding: 48, textAlign: 'center' }}><Spin /></div>
  }
  if (error) {
    return <Alert type="error" showIcon message={t('看板加载失败')} description={error} />
  }
  if (!board) {
    return <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={t('没有配置看板')} />
  }
  if (visibleWidgets.length === 0) {
    return <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={t('这块看板还没有组件，去「看板管理」里搭建')} />
  }

  const cols = (board.canvas && board.canvas.cols) || 12
  const rowHeight = (board.canvas && board.canvas.rowHeight) || 40
  const boardKey = board.id || boardId || ''
  const byId = new Map(visibleWidgets.map(w => [w.id, w]))
  const toItems = (parentId: string): GridCanvasItem[] =>
    (childrenByParent.get(parentId) || []).map(w => ({ id: w.id, x: w.layout.x, y: w.layout.y, w: w.layout.w, h: w.layout.h }))

  // 组件内容：注册表渲染；容器由下面单独套子网格
  const renderWidgetContent = (w: VizWidget) => (
    <WidgetBody
      widget={w}
      ctx={{
        editable: false,
        readOnly: true,
        data: data.results[w.id],
        loading: data.loading,
        // 文本组件正文：不传的话所有文本组件在只读渲染下都会显示「（空文本）」
        text: w.text,
      }}
    />
  )

  // 容器组件：套一层子网格，递归渲染里面的组件
  const renderWidget = (item: GridCanvasItem): React.ReactNode => {
    const w = byId.get(item.id)
    if (!w) return null
    if (isVizContainer(w)) {
      return (
        <div className="canvas-board__container">
          <div className="canvas-board__container-title">{w.title || t('容器')}</div>
          <div className="canvas-board__container-body">
            <GridCanvas
              boardKey={`${boardKey}:${w.id}`}
              items={toItems(w.id)}
              cols={cols}
              rowHeight={rowHeight}
              readOnly
              className="canvas-board__subgrid"
              renderItem={renderWidget}
            />
          </div>
        </div>
      )
    }
    const manifest = getWidgetManifest(w.type)
    return (
      <WidgetCard id={w.id} title={w.title} type={manifest ? manifest.type : w.type} readOnly>
        {renderWidgetContent(w)}
      </WidgetCard>
    )
  }

  return (
    <div className={`canvas-board${className ? ` ${className}` : ''}`}>
      {data.error && !disableData
        ? <Alert type="warning" showIcon style={{ marginBottom: 8 }} message={t('部分数据获取失败')} description={data.error} />
        : null}
      <GridCanvas
        boardKey={boardKey}
        items={toItems('')}
        cols={cols}
        rowHeight={rowHeight}
        readOnly
        responsive
        renderItem={renderWidget}
      />
    </div>
  )
}

export default CanvasBoard
