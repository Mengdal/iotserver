import React, { useCallback, useEffect, useLayoutEffect, useMemo, useRef } from 'react'
import type { GridStack as GridStackInstance, GridStackNode, Responsive } from 'gridstack'
import { GridStack, GridStackItem } from 'gridstack/dist/react'
import type { GridStackHandle, GridStackOptions } from 'gridstack/dist/react'
import 'gridstack/dist/gridstack.css'
import './index.less'

/** 画布上一个组件的位置与尺寸（与后端 VizLayout 一致） */
export interface GridCanvasItem {
  id: string
  x: number
  y: number
  w: number
  h: number
}

/** 外部拖入的节点：react wrapper 会把组件模板挂到 props 上，核心类型里没有这个字段 */
export interface GridCanvasDropNode extends GridStackNode {
  props?: Record<string, unknown>
}

export interface GridCanvasProps {
  /** 组件列表（调用方需按 y/x 排序，保证插入顺序稳定） */
  items: GridCanvasItem[]
  /** 基准列数（= board.canvas.cols） */
  cols: number
  /** 行高 px（= board.canvas.rowHeight） */
  rowHeight: number
  /** 相邻组件之间的空隙 px，默认 12（与 react-grid-layout 的 margin 同义） */
  margin?: number
  /** 只读：staticGrid，不可拖、不可缩 */
  readOnly?: boolean
  /** 窄屏自适应列数（只在只读态生效，避免把移动端排布写回库） */
  responsive?: boolean
  /** 变化时重挂网格：gridstack 的 init 只在挂载时执行一次 */
  boardKey?: string
  /** 拖拽把手选择器，默认卡片标题栏 */
  dragHandle?: string
  /** 不触发拖拽的元素选择器 */
  dragCancel?: string
  /** 组件最少行数，默认 2（与后端 normalize 的 h >= 2 一致） */
  minH?: number
  /**
   * 在内容底部额外保留一行空白，作为“拖到画布空白处”的落点。
   * 顶层画布开启；容器子网格不开启，避免容器自己越拖越高。
   */
  extraRow?: boolean
  /**
   * 是否接收外部/跨网格拖入的组件（只读态强制 false）。
   * 传函数可用于拦掉非法落位（例如不允许把容器拖进容器）。
   */
  acceptWidgets?: boolean | ((el: Element) => boolean)
  className?: string
  renderItem: (item: GridCanvasItem) => React.ReactNode
  /** 网格就绪（注册 setupDragIn 等命令式 API 用） */
  onReady?: (grid: GridStackInstance) => void
  /** 拖拽 / 缩放结束后回报**全量**位置（结构变化不回报，避免加载期把中间态写回） */
  onLayoutChange?: (next: GridCanvasItem[]) => void
  /** 外部拖入（previousNode 有值表示是跨网格搬过来的，不是新组件） */
  onDroppedWidget?: (node: GridCanvasDropNode, previousNode?: GridStackNode) => void
}

const DEFAULT_MARGIN = 12
const DEFAULT_MIN_H = 2
const DEFAULT_DRAG_HANDLE = '.ant-card-head'
const DEFAULT_DRAG_CANCEL = '.ant-btn,.ant-dropdown-trigger,.ant-select,.ant-segmented'
const DEFAULT_ACCEPT_WIDGETS = () => true

/**
 * 窄屏断点：只在只读展示态用。
 * 注意 gridstack 要求 breakpoints 从大到小排列；不传 columnOpts 时完全不响应式（= 现在的固定列行为）。
 */
function buildResponsive (cols: number): Responsive {
  const clamp = (c: number) => Math.max(1, Math.min(Math.round(c), cols))
  return {
    columnMax: cols,
    layout: 'moveScale',
    breakpoints: [
      { w: 1280, c: clamp(cols) },
      { w: 900, c: clamp(8) },
      { w: 600, c: clamp(4) },
      { w: 0, c: 1 },
    ],
  }
}

/**
 * 断开 gridstack 对「容器里的子网格」的自动认领。
 *
 * gridstack 的构造函数会把任何位于 `.grid-stack-item` 内的 `.grid-stack` 自动当成子网格：
 *   inner.parentGridNode = 外层容器节点、外层节点.subGrid = inner
 * 我们是**用 React 自己做的嵌套**（容器内容里再挂一个独立 GridCanvas），这条内部链路没有用处，
 * 反而会在切页面时炸：
 *   1) React 卸载是自上而下 —— 外层 `<GridStack>` 先 destroy，它的 opts/engine/el 都被删除；
 *   2) 内层此时仍然存活，继续更新时走 _updateContainerHeight() →
 *      shouldSizeToContent(parentGridNode) → 读 parentNode.grid.opts.sizeToContent →
 *      TypeError: Cannot read properties of undefined (reading 'sizeToContent')，
 *      表现为 "Unhandled Thrown Error!" 整页报错（曾复现于 /viz/boards 路由）。
 *
 * 只断 JS 关联，保留 gridstack 加的 CSS class（`grid-stack-nested` / `grid-stack-sub-grid`），
 * 布局样式不受影响；容器内拖拽/缩放仍由内层网格自己负责。
 */
const unlinkParentGrid = (grid: GridStackInstance) => {
  const parent = grid.parentGridNode
  if (!parent) return
  delete parent.subGrid
  delete grid.parentGridNode
}

/**
 * 画布网格封装（gridstack 14）：只读展示（数据看板 / 系统监控）与看板编辑器共用。
 *
 * 设计要点：
 *   - 用 `<GridStackItem>` 承装每个组件的真实 React 子树（portal 进 .grid-stack-item-content），
 *     所以数据仍是普通 props，增删 = 挂载/卸载，不用把 spec/result/actions 塞进要 JSON 序列化的 props。
 *   - **不监听 change**：拖动中每帧回调会和布局引擎互相打架；只在 dragstop/resizestop 之后
 *     一次性读全量位置回报（结构变化的中间态不回报，否则加载期会把半成品布局写回去）。
 *   - 位置读 `grid.engine.nodes`（真实值），不用 `grid.save()`——后者会省略等于默认值的 w/h。
 *   - options 里只放标量/字符串（react wrapper 用 JSON.stringify 比对 options，函数会被丢掉也不会重挂）。
 */
const GridCanvas: React.FC<GridCanvasProps> = ({
  items,
  cols,
  rowHeight,
  margin = DEFAULT_MARGIN,
  readOnly,
  responsive,
  boardKey,
  dragHandle = DEFAULT_DRAG_HANDLE,
  dragCancel = DEFAULT_DRAG_CANCEL,
  minH = DEFAULT_MIN_H,
  extraRow = false,
  acceptWidgets,
  className,
  renderItem,
  onReady,
  onLayoutChange,
  onDroppedWidget,
}) => {
  const gridRef = useRef<GridStackHandle>(null)
  // 回调统一走 ref：避免每次渲染都让 react wrapper 重新解绑/绑定 gridstack 事件
  const latest = useRef({ onReady, onLayoutChange, onDroppedWidget })
  latest.current = { onReady, onLayoutChange, onDroppedWidget }

  const baseCols = Math.max(1, Math.floor(cols) || 12)
  const baseRowHeight = Math.max(1, Math.floor(rowHeight) || 40)

  // 几何对齐 react-grid-layout：
  //   RGL   卡片高度 = h * rowHeight + (h - 1) * gap
  //   gridstack 卡片高度 = h * cellHeight - (marginTop + marginBottom)
  // 令 cellHeight = rowHeight + gap、四周边距 = gap / 2，两者完全相等，
  // 且相邻组件的空隙、水平空隙都等于 gap，已有看板的高度观感不变。
  const gap = Number.isFinite(margin) && margin > 0 ? margin : DEFAULT_MARGIN
  const halfGap = gap / 2
  const contentRows = useMemo(
    () => items.reduce((max, item) => Math.max(max, item.y + item.h), 0),
    [items],
  )

  const options = useMemo<GridStackOptions>(() => {
    const next: GridStackOptions = {
      column: baseCols,
      cellHeight: baseRowHeight + gap,
      margin: halfGap,
      // 顶层留一行空白，保证可以把组件拖到所有内容的下方；
      // 容器子网格不加，否则容器会被这行空白撑高。
      minRow: Math.max(1, contentRows + (extraRow && !readOnly ? 4 : 0)),
      // 画布按保存的 x/y 原样展示；编辑时也是自由落位，不做自动上浮/重排。
      // 否则拖动后 gridstack 会把组件重新压实回原来的空位，看起来像“拖动没生效”。
      mode: 'float',
      animate: true,
      staticGrid: !!readOnly,
      disableDrag: !!readOnly,
      disableResize: !!readOnly,
      // 顶层画布默认接受组件库拖入。不能传 true：gridstack 对 true 会要求
      // 拖拽元素本身带 .grid-stack-item，而组件库项没有这个 class，会被拒绝。
      acceptWidgets: readOnly ? false : (acceptWidgets === undefined ? DEFAULT_ACCEPT_WIDGETS : acceptWidgets),
      draggable: { handle: dragHandle, cancel: dragCancel, appendTo: 'body', scroll: true },
      resizable: { handles: 'se', autoHide: true },
    }
    // 窄屏断点只在**只读展示态**启用：编辑态派生列数会让拖拽结果按窄屏坐标写回库里
    if (responsive && readOnly) next.columnOpts = buildResponsive(baseCols)
    return next
  }, [baseCols, baseRowHeight, gap, halfGap, contentRows, extraRow, readOnly, responsive, acceptWidgets, dragHandle, dragCancel])

  /** 读全量位置回报给调用方（只在拖动 / 缩放**结束**时调） */
  const syncLayout = useCallback(() => {
    const grid = gridRef.current?.getGrid()
    const cb = latest.current.onLayoutChange
    if (!grid || !cb) return
    const nodes = grid.engine ? grid.engine.nodes : []
    cb(nodes
      .filter(n => n.id !== undefined && n.id !== null && n.id !== '')
      .map(n => ({
        id: String(n.id),
        x: n.x || 0,
        y: n.y || 0,
        w: n.w || 1,
        h: n.h || 1,
      })))
  }, [])

  const handleDropped = useCallback((_event: Event, previous: GridStackNode | undefined, node: GridStackNode) => {
    if (node) latest.current.onDroppedWidget?.(node as GridCanvasDropNode, previous)
  }, [])

  // 初始化后立刻解除 gridstack 的 subGrid 认领（见 unlinkParentGrid）。
  // 放在 layout 阶段：要尽可能早于任何可能的卸载，否则外层先销毁、内层后更新时会崩。
  useLayoutEffect(() => {
    const grid = gridRef.current?.getGrid()
    if (grid) unlinkParentGrid(grid)
  }, [boardKey])

  // 网格就绪回调（boardKey 变化会重挂，所以依赖带上它）
  useEffect(() => {
    const grid = gridRef.current?.getGrid()
    if (grid) latest.current.onReady?.(grid)
  }, [boardKey])

  /**
   * 组件从 items 里消失时，必须由我们显式把 DOM 也摘掉：
   * GridStackItem 卸载时调的是 removeWidget(el, false)，只摘引擎节点、不删 DOM，
   * 会在画布上留下一个占位的空盒子。这里传 true 走 addRemoveCB 的移除分支，元素立即被移除，
   * 随后 GridStackItem 的清理逻辑（微任务）查不到节点会直接返回。
   */
  const aliveIdsRef = useRef<string[]>([])
  useLayoutEffect(() => {
    const grid = gridRef.current?.getGrid()
    const current = items.map(item => item.id)
    const previous = aliveIdsRef.current
    aliveIdsRef.current = current
    if (!grid || !grid.engine) return
    const alive = new Set(current)
    previous.forEach((id) => {
      if (alive.has(id)) return
      const node = grid.engine.nodes.find(n => String(n.id) === id)
      if (node && node.el) grid.removeWidget(node.el, true)
    })
  }, [items])

  const rootClass = [
    'grid-canvas',
    readOnly ? 'grid-canvas--readonly' : 'grid-canvas--editable',
    className,
  ].filter(Boolean).join(' ')

  return (
    <GridStack
      key={boardKey}
      ref={gridRef}
      className={rootClass}
      options={options}
      onDragStop={syncLayout}
      onResizeStop={syncLayout}
      onDropped={handleDropped}
    >
      {items.map(item => (
        <GridStackItem
          key={item.id}
          id={item.id}
          options={{ x: item.x, y: item.y, w: item.w, h: item.h, minW: 1, minH }}
        >
          {renderItem(item)}
        </GridStackItem>
      ))}
    </GridStack>
  )
}

export default GridCanvas
