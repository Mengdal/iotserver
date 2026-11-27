import { t } from '@/i18n'
import type { Diagnostic, FlowDoc, FlowEdge, NodeDef, PortDef } from './types'
import { getNodeDef } from './nodes'

export interface ConnectionLike {
  source?: string | null
  sourceHandle?: string | null
  target?: string | null
  targetHandle?: string | null
}

export interface ConnectionCheck {
  ok: boolean
  reason?: string
}

function findNode (flow: FlowDoc, id?: string | null) {
  if (!id) return undefined
  return flow.spec.nodes.find(node => node.id === id)
}

function findPort (ports: PortDef[], id?: string | null): PortDef | undefined {
  if (!id) return undefined
  return ports.find(port => port.id === id)
}

export function portCompatible (sourceKind: string, targetKind: string): boolean {
  return sourceKind === 'any' || targetKind === 'any' || sourceKind === targetKind
}

/** 连线预检：拖线过程中和落线时都用同一套规则，避免“能连上但校验不过” */
export function checkConnection (flow: FlowDoc, connection: ConnectionLike): ConnectionCheck {
  const source = findNode(flow, connection.source)
  const target = findNode(flow, connection.target)
  if (!source || !target) return { ok: false, reason: t('节点不存在或已被删除') }
  if (source.id === target.id) return { ok: false, reason: t('不能连接到自身') }
  if (flow.spec.edges.some(edge =>
    edge.sourceNodeId === source.id
    && edge.sourcePortId === connection.sourceHandle
    && edge.targetNodeId === target.id
    && edge.targetPortId === connection.targetHandle)) {
    return { ok: false, reason: t('重复的连线') }
  }

  const sourceDef = getNodeDef(source.type, source.typeVersion)
  const targetDef = getNodeDef(target.type, target.typeVersion)
  if (!sourceDef || !targetDef) return { ok: false, reason: t('未知节点类型「{{type}}」', { type: source.type }) }

  const sourcePort = findPort(sourceDef.outputs, connection.sourceHandle)
  const targetPort = findPort(targetDef.inputs, connection.targetHandle)
  if (!sourcePort || !targetPort) return { ok: false, reason: t('端口不存在') }
  if (!portCompatible(sourcePort.kind, targetPort.kind)) {
    return {
      ok: false,
      reason: t('端口类型不兼容：{{source}} → {{target}}', {
        source: sourcePort.kind,
        target: targetPort.kind,
      }),
    }
  }
  if (flow.spec.edges.some(edge => edge.targetNodeId === target.id && edge.targetPortId === targetPort.id)) {
    return { ok: false, reason: t('输入端口已经有连线') }
  }
  return { ok: true }
}

function hasCycle (flow: FlowDoc): boolean {
  const graph: Record<string, string[]> = {}
  flow.spec.nodes.forEach(node => { graph[node.id] = [] })
  flow.spec.edges.forEach(edge => {
    if (graph[edge.sourceNodeId]) graph[edge.sourceNodeId].push(edge.targetNodeId)
  })
  const visiting = new Set<string>()
  const visited = new Set<string>()
  const walk = (id: string): boolean => {
    if (visiting.has(id)) return true
    if (visited.has(id)) return false
    visiting.add(id)
    const found = (graph[id] || []).some(walk)
    visiting.delete(id)
    visited.add(id)
    return found
  }
  return flow.spec.nodes.some(node => walk(node.id))
}

function reachableFromSources (flow: FlowDoc): Set<string> {
  const def = (node: { type: string, typeVersion: number }): NodeDef | undefined => getNodeDef(node.type, node.typeVersion)
  const queue = flow.spec.nodes
    .filter(node => def(node)?.runtimeKind === 'source')
    .map(node => node.id)
  const reachable = new Set<string>(queue)
  while (queue.length) {
    const current = queue.shift() as string
    flow.spec.edges
      .filter(edge => edge.sourceNodeId === current)
      .forEach(edge => {
        if (!reachable.has(edge.targetNodeId)) {
          reachable.add(edge.targetNodeId)
          queue.push(edge.targetNodeId)
        }
      })
  }
  return reachable
}

export function validateFlow (flow: FlowDoc): Diagnostic[] {
  const diagnostics: Diagnostic[] = []
  const push = (item: Diagnostic) => diagnostics.push(item)

  const ids = new Set<string>()
  flow.spec.nodes.forEach(node => {
    if (ids.has(node.id)) {
      push({ code: 'FLOW_DUPLICATE_NODE', severity: 'error', message: t('节点 ID 重复'), nodeId: node.id })
    }
    ids.add(node.id)
  })

  const sourceNodes = flow.spec.nodes.filter(node => getNodeDef(node.type, node.typeVersion)?.runtimeKind === 'source')
  const sinkNodes = flow.spec.nodes.filter(node => getNodeDef(node.type, node.typeVersion)?.runtimeKind === 'sink')
  if (!sourceNodes.length) push({ code: 'FLOW_NO_SOURCE', severity: 'error', message: t('流程至少需要一个源节点') })
  if (!sinkNodes.length) push({ code: 'FLOW_NO_SINK', severity: 'error', message: t('流程至少需要一个目标节点') })

  if (hasCycle(flow)) {
    push({ code: 'FLOW_CYCLE_UNSUPPORTED', severity: 'error', message: t('流程中存在环，无法部署') })
  }

  flow.spec.nodes.forEach(node => {
    const def = getNodeDef(node.type, node.typeVersion)
    if (!def) {
      push({
        code: 'FLOW_UNKNOWN_NODE_TYPE',
        severity: 'error',
        message: t('未知节点类型「{{type}}」', { type: node.type }),
        nodeId: node.id,
      })
      return
    }

    def.properties.forEach(prop => {
      if (!prop.required) return
      const value = node.config ? node.config[prop.key] : undefined
      if (value === undefined || value === null || String(value).trim() === '') {
        push({
          code: 'FLOW_REQUIRED_PROPERTY_MISSING',
          severity: 'error',
          message: t('属性「{{name}}」不能为空', { name: prop.label }),
          nodeId: node.id,
          propertyPath: prop.key,
        })
      }
    })

    if (def.type === 'window') {
      const length = Number(node.config.length)
      if (!Number.isFinite(length) || length <= 0) {
        push({
          code: 'FLOW_INVALID_WINDOW',
          severity: 'error',
          message: t('窗口长度必须大于 0'),
          nodeId: node.id,
          propertyPath: 'length',
        })
      }
    }

    if (def.type === 'switch') {
      const cases = String(node.config.cases || '').split(',').map(item => item.trim()).filter(Boolean)
      if (cases.length !== 2) {
        push({
          code: 'FLOW_SWITCH_CASES_INVALID',
          severity: 'error',
          message: t('条件分支需要恰好两条条件'),
          nodeId: node.id,
          propertyPath: 'cases',
        })
      }
    }

    if (def.type === 'aggregate') {
      const incoming = flow.spec.edges.filter(edge => edge.targetNodeId === node.id)
      const upstream = incoming.length ? flow.spec.nodes.find(item => item.id === incoming[0].sourceNodeId) : undefined
      if (!upstream || upstream.type !== 'group-by') {
        push({
          code: 'FLOW_AGGREGATE_REQUIRES_GROUP_BY',
          severity: 'error',
          message: t('聚合节点前需要先经过分组节点'),
          nodeId: node.id,
        })
      }
    }

    const incomingCount = flow.spec.edges.filter(edge => edge.targetNodeId === node.id).length
    if (incomingCount > 1) {
      push({
        code: 'FLOW_MULTIPLE_INPUTS',
        severity: 'error',
        message: t('节点存在多条输入连线'),
        nodeId: node.id,
      })
    }
  })

  flow.spec.edges.forEach((edge: FlowEdge) => {
    const source = findNode(flow, edge.sourceNodeId)
    const target = findNode(flow, edge.targetNodeId)
    if (!source || !target) {
      push({ code: 'FLOW_EDGE_ENDPOINT_MISSING', severity: 'error', message: t('连线端点不存在'), edgeId: edge.id })
      return
    }
    if (source.id === target.id) {
      push({ code: 'FLOW_SELF_EDGE_UNSUPPORTED', severity: 'error', message: t('不能连接到自身'), edgeId: edge.id })
      return
    }
    if (source.type === 'switch' && edge.sourcePortId === 'default') {
      push({
        code: 'FLOW_SWITCH_DEFAULT_UNSUPPORTED',
        severity: 'error',
        message: t('条件分支的默认分支暂不支持部署'),
        edgeId: edge.id,
        nodeId: source.id,
      })
    }
    const sourceDef = getNodeDef(source.type, source.typeVersion)
    const targetDef = getNodeDef(target.type, target.typeVersion)
    const sourcePort = sourceDef ? findPort(sourceDef.outputs, edge.sourcePortId) : undefined
    const targetPort = targetDef ? findPort(targetDef.inputs, edge.targetPortId) : undefined
    if (!sourcePort || !targetPort) {
      push({ code: 'FLOW_PORT_MISSING', severity: 'error', message: t('端口不存在'), edgeId: edge.id })
      return
    }
    if (!portCompatible(sourcePort.kind, targetPort.kind)) {
      push({
        code: 'FLOW_PORT_INCOMPATIBLE',
        severity: 'error',
        message: t('端口类型不兼容：{{source}} → {{target}}', { source: sourcePort.kind, target: targetPort.kind }),
        edgeId: edge.id,
      })
    }
  })

  const reachable = reachableFromSources(flow)
  flow.spec.nodes.forEach(node => {
    const def = getNodeDef(node.type, node.typeVersion)
    if (!def || def.runtimeKind === 'source') return
    if (!reachable.has(node.id)) {
      push({
        code: 'FLOW_DISCONNECTED_NODE',
        severity: 'warning',
        message: t('节点未连接到数据链路'),
        nodeId: node.id,
      })
    }
  })

  return diagnostics
}

export function hasErrors (diagnostics: Diagnostic[]): boolean {
  return diagnostics.some(item => item.severity === 'error')
}
