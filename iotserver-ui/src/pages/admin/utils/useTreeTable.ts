import { useCallback, useEffect, useMemo, useState } from 'react'
import type { FlatMeta, FlatTableResult, TreeNodeLike } from './treeTable'
import { allParentKeys, buildFlatTable, collectAncestorKeys, collectNested, visibleRows } from './treeTable'

interface Options<T extends TreeNodeLike> {
  /** 接口返回的嵌套树 */
  tree: T[] | undefined | null
  /** 是否处于过滤状态；过滤时自动展开命中分支，便于直接看到结果 */
  filtered: boolean
  /** 过滤条件（用于判断数据/条件变化） */
  filterKey: unknown
  match?(node: T): boolean
  getId(node: T): string
  getParentId(node: T): string
}

/**
 * 树形表格数据：拍平 + 默认全部收起 + 整棵子树展开/收起。
 *
 * 状态用 expandedKeys（已展开节点）表达：
 *   - 默认空集合 = 只显示顶级节点（全部收起）；
 *   - 展开某节点 = 把该节点及其祖先加入集合（保证它和它的子节点可见）；
 *   - 收起某节点 = 把它及其所有后代移出集合。
 * 这样「展开/收起」的判定永远只看节点自身是否在集合里，不会出现判断错位。
 */
export function useTreeTable<T extends TreeNodeLike> (options: Options<T>) {
  const { tree, filtered, filterKey, match, getId, getParentId } = options

  const flat: FlatTableResult<T> = useMemo(
    () => buildFlatTable(collectNested<T>(tree, getId), { getId, getParentId, match }),
    // match/getId/getParentId 由调用方以稳定引用传入；
    // flat 变化只跟随 tree / filterKey
    // eslint-disable-next-line
    [tree, filterKey],
  )

  const [expandedKeys, setExpandedKeys] = useState<string[]>([])

  // 数据或过滤条件变化：默认全部收起；处于过滤状态时展开全部父节点，直接展示命中结果
  useEffect(() => {
    setExpandedKeys(filtered ? allParentKeys(flat) : [])
    // eslint-disable-next-line
  }, [tree, filterKey, filtered])

  const rows = useMemo(
    () => visibleRows(flat, expandedKeys, getId) as Array<T & FlatMeta>,
    // eslint-disable-next-line
    [flat, expandedKeys],
  )

  const isExpanded = useCallback(
    (id: string) => expandedKeys.includes(id),
    [expandedKeys],
  )

  /** 展开/收起整棵子树 */
  const toggleExpand = useCallback((record: { id: string }) => {
    const id = record.id
    setExpandedKeys(prev => {
      if (prev.includes(id)) {
        // 收起：把该节点及其所有后代移出集合
        const own = flat.rows.filter(row => getId(row) === id || row.topKey === id).map(row => getId(row))
        return prev.filter(key => !own.includes(key))
      }
      // 展开：连同祖先一起加入，保证该节点及其子节点都可见
      return Array.from(new Set([...prev, id, ...collectAncestorKeys(flat, id, getId)]))
    })
    // eslint-disable-next-line
  }, [flat])

  return { flat, rows, expandedKeys, toggleExpand, isExpanded }
}
