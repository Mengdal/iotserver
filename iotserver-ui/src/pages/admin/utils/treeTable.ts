/**
 * 树形表格工具
 *
 * 背景：antd Table 若在 dataSource 行上发现 children 字段，会自行按树渲染；
 * 若同时又手工扁平化，就会出现「重复行 + 展开状态错乱」。
 * 而剥离 children 后 antd 又不愿渲染展开列。因此这里统一：
 *   1) collectNested  把接口返回的嵌套树拍平成一维数组（剥离 children）
 *   2) buildFlatTable 计算 level / 父链 / 可见性
 *   3) 页面自绘折叠列（依据 hasChildren + collapsedKeys）
 * 展开状态由 collapsedKeys 完全掌控，行为可预期。
 */

export interface TreeNodeLike {
  children?: TreeNodeLike[] | null
  [key: string]: any
}

export interface FlatMeta {
  /** 深度，0 为根 */
  level: number
  /** 直接父节点 key */
  parentKey: string
  /** 所在子树的顶级节点 key（折叠以子树为单位） */
  topKey: string
  /** 是否存在子节点 */
  hasChildren: boolean
  /** 自身或后代是否命中过滤条件 */
  matched: boolean
}

export interface FlatTableResult<T> {
  rows: Array<T & FlatMeta>
  allKeys: string[]
  /** 需要展示的 key；过滤时含命中节点的祖先链 */
  visibleKeys: string[]
  /** 拥有子节点的 key */
  parentKeys: string[]
}

/** 根节点统一父键，必须与实际数据里的根 parentId 保持一致 */
export const ROOT_KEY = '0'

/** 把接口返回的嵌套树拍平成一维数组，并剥离 children */
export function collectNested<T extends TreeNodeLike> (
  tree: T[] | undefined | null,
  getId: (node: T) => string,
  out: T[] = [],
  guard = 0,
): T[] {
  if (!tree || guard > 32) return out
  tree.forEach(node => {
    const children = Array.isArray(node.children) ? (node.children as T[]) : []
    const copy = { ...(node as any) }
    delete copy.children
    out.push(copy as T)
    if (children.length > 0) {
      collectNested(children, getId, out, guard + 1)
    }
  })
  return out
}

interface BuildOptions<T> {
  getId(node: T): string
  getParentId(node: T): string
  /** 过滤条件；自身未命中但后代命中的节点会保留 */
  match?(node: T): boolean
}

export function buildFlatTable<T> (nodes: T[], options: BuildOptions<T>): FlatTableResult<T> {
  const { getId, getParentId, match } = options

  const childrenOf = new Map<string, T[]>()
  const known = new Set<string>()
  nodes.forEach(node => known.add(getId(node)))
  nodes.forEach(node => {
    const raw = getParentId(node) || ''
    // 根节点的 parentId 可能是 '0' 或 ''，统一归一到 ROOT_KEY；
    // 父节点不存在（脏数据）时也挂到根级，避免整棵子树丢失
    let pid = ROOT_KEY
    if (raw !== '' && raw !== ROOT_KEY) {
      pid = known.has(raw) ? raw : ROOT_KEY
    }
    if (!childrenOf.has(pid)) childrenOf.set(pid, [])
    childrenOf.get(pid)!.push(node)
  })

  const rows: Array<T & FlatMeta> = []
  const parentKeys: string[] = []
  const visible = new Set<string>()

  const walk = (parentId: string, level: number, ancestors: string[], topKey: string): boolean => {
    const children = childrenOf.get(parentId) || []
    let anyHit = false
    for (const node of children) {
      const id = getId(node)
      const kids = childrenOf.get(id) || []
      const selfTop = level === 0 ? id : topKey

      // 先登记父节点，再递归其子节点：保证输出是「父在前、子随后」的深度优先顺序
      const row: T & FlatMeta = {
        ...(node as any),
        level,
        parentKey: parentId,
        topKey: selfTop,
        hasChildren: kids.length > 0,
        matched: false,
      }
      rows.push(row)
      if (kids.length > 0) parentKeys.push(id)

      const childHit = kids.length > 0 ? walk(id, level + 1, [...ancestors, id], selfTop) : false
      const selfHit = match ? match(node) : true
      row.matched = selfHit || childHit

      if (row.matched) {
        visible.add(id)
        ancestors.forEach(ancestor => visible.add(ancestor))
        anyHit = true
      }
    }
    return anyHit
  }

  walk(ROOT_KEY, 0, [], '')

  // 父节点缺失等原因未遍历到的节点，兜底挂到根级
  const visited = new Set(rows.map(row => getId(row)))
  nodes.forEach(node => {
    const id = getId(node)
    if (visited.has(id)) return
    const kids = childrenOf.get(id) || []
    rows.push({
      ...(node as any),
      level: 0,
      parentKey: ROOT_KEY,
      topKey: id,
      hasChildren: kids.length > 0,
      matched: true,
    })
    if (kids.length > 0) parentKeys.push(id)
    visible.add(id)
  })

  const allKeys = rows.map(row => getId(row))
  return {
    rows,
    allKeys,
    visibleKeys: match ? allKeys.filter(key => visible.has(key)) : allKeys,
    parentKeys,
  }
}

/** 某节点的全部祖先 key（用于折叠时收敛、展开时保证可见） */
export function collectAncestorKeys<T> (
  result: FlatTableResult<T>,
  nodeKey: string,
  getId: (row: T) => string,
): string[] {
  const byKey = new Map<string, T & FlatMeta>()
  result.rows.forEach(row => byKey.set(getId(row), row))
  const out: string[] = []
  let cursor = byKey.get(nodeKey)?.parentKey
  let guard = 0
  while (cursor && cursor !== ROOT_KEY && guard++ < 64) {
    out.push(cursor)
    cursor = byKey.get(cursor)?.parentKey
  }
  return out
}

/**
 * 可见行
 *
 * expandedKeys 记录「已展开的节点」。行的可见规则：
 *   - 顶级行（topKey === 自身 id）永远可见；
 *   - 其余行需要其直接父节点已展开。
 * 过滤命中的节点由 buildFlatTable 写入 visibleKeys，因此过滤时不会漏掉命中分支。
 */
export function visibleRows<T> (
  result: FlatTableResult<T>,
  expandedKeys: string[],
  getId: (row: T) => string,
): Array<T & FlatMeta> {
  const expanded = new Set(expandedKeys)
  const visibleSet = new Set(result.visibleKeys)
  return result.rows.filter(row => {
    if (!visibleSet.has(getId(row))) return false
    if (row.parentKey === ROOT_KEY) return true
    return expanded.has(row.parentKey)
  })
}

/** 全部拥有子节点的 key（用于过滤时自动展开命中分支） */
export function allParentKeys<T> (result: FlatTableResult<T>): string[] {
  return result.parentKeys
}
