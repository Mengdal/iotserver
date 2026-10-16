import { VizWidget } from '@/api'

/**
 * 看板结构的扁平化与分组：编辑器用「扁平数组 + parentOf」保存状态，
 * 只在保存时组装回嵌套树；这样跨子网格拖拽只需要改归属，不用搬树。
 */

/** 组件是不是容器（容器本身不取数，只装别的组件） */
export const isVizContainer = (w?: VizWidget | null): boolean => !!w && w.type === 'container'

export interface VizFlatTree {
  /** 所有层级的组件（前序：父在子前） */
  widgets: VizWidget[]
  /** 子 id -> 父 id；顶层为空串 */
  parentOf: Record<string, string>
}

/** 把后端返回的嵌套树拍平 */
export function flattenVizTree (list: VizWidget[] = [], parentId = ''): VizFlatTree {
  const widgets: VizWidget[] = []
  const parentOf: Record<string, string> = {}
  const walk = (nodes: VizWidget[], pid: string) => {
    nodes.forEach((w) => {
      widgets.push(w)
      parentOf[w.id] = pid
      if (w.children && w.children.length > 0) walk(w.children, w.id)
    })
  }
  walk(list, parentId)
  return { widgets, parentOf }
}

/** 按父级分组并按 (y, x) 排序（sort 稳定，同位置保持插入顺序） */
export function groupByParent (widgets: VizWidget[], parentOf: Record<string, string>): Map<string, VizWidget[]> {
  const map = new Map<string, VizWidget[]>()
  widgets.forEach((w) => {
    const pid = parentOf[w.id] || ''
    const list = map.get(pid)
    if (list) list.push(w)
    else map.set(pid, [w])
  })
  map.forEach((list) => {
    list.sort((a, b) => (a.layout.y - b.layout.y) || (a.layout.x - b.layout.x))
  })
  return map
}

/** 某个组件的所有后代 id（删容器时级联清理用） */
export function collectDescendantIds (id: string, parentOf: Record<string, string>): string[] {
  const out: string[] = []
  const stack = [id]
  while (stack.length > 0) {
    const current = stack.pop() as string
    Object.keys(parentOf).forEach((childId) => {
      if (parentOf[childId] !== current) return
      out.push(childId)
      stack.push(childId)
    })
  }
  return out
}
