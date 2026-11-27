import type { FlowDoc, FlowEdge, FlowNode } from './types'
import { defaultConfig, getNodeDef } from './nodes'

const STORAGE_KEY = 'iotserver:flow-studio-demo:v3'

export function makeNodeId (): string {
  return `n_${Date.now().toString(36)}_${Math.random().toString(36).slice(2, 7)}`
}

export function makeEdgeId (): string {
  return `e_${Date.now().toString(36)}_${Math.random().toString(36).slice(2, 7)}`
}

export function makeFlowId (): string {
  return `flow_${Date.now().toString(36)}_${Math.random().toString(36).slice(2, 6)}`
}

export function createNode (type: string, name?: string, config?: Record<string, any>): FlowNode | null {
  const def = getNodeDef(type)
  if (!def) return null
  return {
    id: makeNodeId(),
    type: def.type,
    typeVersion: def.version,
    name: name || def.displayName,
    config: config ? { ...defaultConfig(def), ...config } : defaultConfig(def),
  }
}

export function createEdge (source: string, sourcePort: string, target: string, targetPort: string): FlowEdge {
  return {
    id: makeEdgeId(),
    sourceNodeId: source,
    sourcePortId: sourcePort,
    targetNodeId: target,
    targetPortId: targetPort,
  }
}

function buildFlow (
  id: string,
  name: string,
  description: string,
  nodes: Array<{ type: string, name: string, config?: Record<string, any>, x: number, y: number }>,
  links: Array<[number, string, number, string]>,
): FlowDoc {
  const builtNodes = nodes.map(item => createNode(item.type, item.name, item.config)).filter(Boolean) as FlowNode[]
  const layout: FlowDoc['layout'] = { nodes: {} }
  nodes.forEach((item, index) => {
    layout.nodes[builtNodes[index].id] = { x: item.x, y: item.y }
  })
  const edges = links.map(([sourceIndex, sourcePort, targetIndex, targetPort]) =>
    createEdge(builtNodes[sourceIndex].id, sourcePort, builtNodes[targetIndex].id, targetPort))
  return {
    id,
    name,
    description,
    spec: {
      nodes: builtNodes,
      edges,
      options: { concurrency: 1 },
    },
    layout,
    updatedAt: new Date().toISOString(),
  }
}

export function seedFlows (): FlowDoc[] {
  return [
    buildFlow(
      'demo_high_temp',
      '高温告警联动',
      '设备温度超过 30℃ 时转发到告警主题；演示过滤与目标节点。',
      [
        { type: 'mqtt-source', name: '设备数据源', config: { topic: 'eventbus/in', qos: 0 }, x: 40, y: 140 },
        { type: 'filter', name: '高温过滤', config: { expression: 'temperature > 30' }, x: 300, y: 140 },
        { type: 'mqtt-sink', name: '告警转发', config: { server: 'tcp://mqtt-broker:58090', topic: 'alarm/high-temp', qos: 1 }, x: 560, y: 140 },
      ],
      [[0, 'out', 1, 'in'], [1, 'out', 2, 'in']],
    ),
    buildFlow(
      'demo_window_agg',
      '窗口聚合统计',
      '按设备分组计算每分钟平均温度，再推送到 HTTP 接口；演示窗口、分组与聚合。',
      [
        { type: 'mqtt-source', name: '设备数据源', config: { topic: 'eventbus/in' }, x: 40, y: 60 },
        { type: 'window', name: '一分钟窗口', config: { length: 1, timeUnit: 'MI' }, x: 300, y: 60 },
        { type: 'group-by', name: '按设备分组', config: { keys: 'deviceId' }, x: 560, y: 60 },
        { type: 'aggregate', name: '平均温度', config: { fields: 'avg(temperature) AS avgTemp' }, x: 560, y: 240 },
        { type: 'rest-sink', name: '推送统计结果', config: { url: 'http://127.0.0.1:3000/api/v1/ekuiper/alert', method: 'POST' }, x: 820, y: 240 },
      ],
      [[0, 'out', 1, 'in'], [1, 'out', 2, 'in'], [2, 'out', 3, 'in'], [3, 'out', 4, 'in']],
    ),
    buildFlow(
      'demo_incomplete',
      '待完善流程',
      '只有数据源、还没有目标节点；打开后可以直接看到校验报错。',
      [
        { type: 'mqtt-source', name: '设备数据源', config: { topic: 'eventbus/in' }, x: 60, y: 140 },
        { type: 'filter', name: '高温过滤', config: { expression: 'temperature > 30' }, x: 330, y: 140 },
      ],
      [[0, 'out', 1, 'in']],
    ),
  ]
}

export function loadFlows (): FlowDoc[] {
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    if (raw) {
      const parsed = JSON.parse(raw)
      if (Array.isArray(parsed) && parsed.length > 0) return parsed as FlowDoc[]
    }
  } catch (error) {
    // 无痕模式 / 存储损坏：回落到种子数据，保证 demo 可打开
  }
  const seeded = seedFlows()
  saveFlows(seeded)
  return seeded
}

export function saveFlows (flows: FlowDoc[]): boolean {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(flows))
    return true
  } catch (error) {
    return false
  }
}

export function saveFlow (flow: FlowDoc): boolean {
  const flows = loadFlows()
  const index = flows.findIndex(item => item.id === flow.id)
  const next = flows.slice()
  const stored = { ...flow, updatedAt: new Date().toISOString() }
  if (index >= 0) next[index] = stored
  else next.unshift(stored)
  return saveFlows(next)
}

export function deleteFlow (id: string): boolean {
  return saveFlows(loadFlows().filter(item => item.id !== id))
}

export function getFlow (id: string): FlowDoc | undefined {
  return loadFlows().find(item => item.id === id)
}

export function newNodePosition (index: number): { x: number, y: number } {
  const column = index % 4
  const row = Math.floor(index / 4)
  return { x: 80 + column * 260, y: 100 + row * 160 }
}
