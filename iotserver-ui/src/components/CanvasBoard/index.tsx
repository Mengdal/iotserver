import React, { useCallback, useEffect, useMemo, useState } from 'react'
import { Alert, Empty, Spin } from 'antd'
import GridLayout, { WidthProvider } from 'react-grid-layout'
import { getVizBoard, queryVizBatch, VizBoardDetail, VizQueryResult, VizWidget } from '@/api'
import WidgetCard from '@/components/WidgetCard'
import 'react-grid-layout/css/styles.css'
import 'react-resizable/css/styles.css'
import './index.less'

const AutoGrid = WidthProvider(GridLayout)

interface CanvasBoardProps {
  /** 看板 ID（二选一） */
  boardId?: string
  /** 已加载的看板（二选一，避免重复请求） */
  board?: VizBoardDetail
  className?: string
}

/**
 * 画布看板的**只读**渲染（数据看板页、系统监控页用）：
 * 按保存的布局把组件摆在网格上，数据走 /api/v1/viz/query/batch 一次性取回。
 */
const CanvasBoard: React.FC<CanvasBoardProps> = ({ boardId, board: boardProp, className }) => {
  const [board, setBoard] = useState<VizBoardDetail | undefined>(boardProp)
  const [results, setResults] = useState<Record<string, VizQueryResult>>({})
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const widgets: VizWidget[] = useMemo(() => {
    if (!board || !board.canvas) return []
    return board.canvas.widgets || []
  }, [board])

  const loadResults = useCallback(async (list: VizWidget[]) => {
    // 文本组件不参与取数
    const charts = list.filter(w => (w.type || 'chart') !== 'text')
    if (charts.length === 0) {
      setResults({})
      return
    }
    setError('')
    try {
      const resp = await queryVizBatch(charts.map(w => ({ id: w.id, name: w.title, spec: w.spec })))
      if (resp.success) {
        const next: Record<string, VizQueryResult> = {}
        ;(resp.result.results || []).forEach((item) => { next[item.id || ''] = item })
        setResults(next)
      } else {
        setError(resp.errorMsg || '取数失败')
      }
    } catch (e: any) {
      setError(e?.message || '取数失败')
    }
  }, [])

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
          setError(resp.errorMsg || '看板加载失败')
        }
      })
      .catch((e: any) => setError(e?.message || '看板加载失败'))
      .finally(() => setLoading(false))
  }, [boardId, boardProp])

  useEffect(() => {
    loadResults(widgets)
  }, [widgets, loadResults])

  if (loading) {
    return <div style={{ padding: 48, textAlign: 'center' }}><Spin /></div>
  }
  if (error) {
    return <Alert type="error" showIcon message="看板加载失败" description={error} />
  }
  if (!board) {
    return <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="没有配置看板" />
  }
  if (widgets.length === 0) {
    return <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="这块看板还没有组件，去「看板管理」里搭建" />
  }

  const cols = (board.canvas && board.canvas.cols) || 12
  const rowHeight = (board.canvas && board.canvas.rowHeight) || 40

  return (
    <div className={`canvas-board${className ? ` ${className}` : ''}`}>
      <AutoGrid
        className="canvas-board__grid"
        layout={widgets.map(w => ({ i: w.id, x: w.layout.x, y: w.layout.y, w: w.layout.w, h: w.layout.h }))}
        cols={cols}
        rowHeight={rowHeight}
        margin={[12, 12]}
        containerPadding={[0, 0]}
        isDraggable={false}
        isResizable={false}
        compactType={null}
        preventCollision
      >
        {widgets.map(w => (
          <div key={w.id}>
            <WidgetCard
              id={w.id}
              title={w.title}
              type={w.type || 'chart'}
              text={w.text}
              spec={w.spec}
              result={results[w.id]}
              loading={!results[w.id]}
              readOnly
            />
          </div>
        ))}
      </AutoGrid>
    </div>
  )
}

export default CanvasBoard
