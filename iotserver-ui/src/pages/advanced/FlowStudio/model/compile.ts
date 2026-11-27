import type {
  CompiledArtifact,
  EKuiperGraph,
  EKuiperGraphNode,
  FlowDoc,
  FlowNode,
  NodeDef,
} from './types'
import { getNodeDef } from './nodes'

/** 稳定的 JSON 序列化：对象键排序，保证同一份语义文档得到同一个 hash */
export function stableStringify (value: any): string {
  if (value === null || typeof value !== 'object') return JSON.stringify(value)
  if (Array.isArray(value)) return `[${value.map(stableStringify).join(',')}]`
  const keys = Object.keys(value).sort()
  return `{${keys.map(key => `${JSON.stringify(key)}:${stableStringify(value[key])}`).join(',')}}`
}

/** FNV-1a 32 位哈希，拼两轮得到 12 位十六进制；仅用于演示的确定性 ID */
export function hash12 (input: string): string {
  let h1 = 0x811c9dc5
  let h2 = 0x01000193
  for (let i = 0; i < input.length; i += 1) {
    const code = input.charCodeAt(i)
    h1 ^= code
    h1 = Math.imul(h1, 0x01000193) >>> 0
    h2 ^= code + i
    h2 = Math.imul(h2, 0x85ebca6b) >>> 0
  }
  const a = (`00000000${h1.toString(16)}`).slice(-8)
  const b = (`0000${h2.toString(16)}`).slice(-4)
  return a + b
}

export function flowSemanticHash (flow: FlowDoc): string {
  return hash12(stableStringify(flow.spec))
}

function runtimePrefix (def: NodeDef): string {
  if (def.runtimeKind === 'source') return 'source'
  if (def.runtimeKind === 'sink') return 'sink'
  return def.nodeType
}

export function runtimeIdFor (def: NodeDef, flowNodeId: string): string {
  return `${runtimePrefix(def)}_${hash12(flowNodeId)}`
}

function splitList (raw: any): string[] {
  return String(raw || '')
    .split(',')
    .map(item => item.trim())
    .filter(Boolean)
}

function buildProps (node: FlowNode, def: NodeDef): Record<string, any> {
  const config = node.config || {}
  switch (def.type) {
    case 'mqtt-source':
    case 'memory-source':
      return {
        datasource: config.topic,
        ...(config.qos !== undefined ? { qos: Number(config.qos) } : {}),
        ...(config.protocolVersion ? { protocolVersion: config.protocolVersion } : {}),
      }
    case 'filter':
      return { expr: config.expression }
    case 'window':
      return {
        type: 'tumblingwindow',
        unit: config.timeUnit,
        size: Number(config.length),
      }
    case 'group-by':
      return { dimensions: splitList(config.keys) }
    case 'aggregate':
      return { expr: config.fields }
    case 'switch':
      return {
        cases: splitList(config.cases),
        stopAtFirstMatch: true,
      }
    case 'mqtt-sink':
      return {
        server: config.server,
        topic: config.topic,
        ...(config.qos !== undefined ? { qos: Number(config.qos) } : {}),
      }
    case 'rest-sink':
      return {
        url: config.url,
        method: config.method || 'POST',
        bodyType: 'json',
        sendSingle: true,
      }
    case 'log-sink':
      return {}
    default:
      return {}
  }
}

function buildOptions (flow: FlowDoc): Record<string, any> | undefined {
  const options = flow.spec.options || {}
  const out: Record<string, any> = {}
  if (options.concurrency !== undefined) out.concurrency = Number(options.concurrency)
  if (options.bufferLength !== undefined) out.bufferLength = Number(options.bufferLength)
  if (options.qos !== undefined) out.qos = Number(options.qos)
  if (options.isEventTime !== undefined) out.isEventTime = Boolean(options.isEventTime)
  if (options.sendMetaToSink !== undefined) out.sendMetaToSink = Boolean(options.sendMetaToSink)
  if (options.sendError !== undefined) out.sendError = Boolean(options.sendError)
  return Object.keys(out).length ? out : undefined
}

export function toSafeRuleId (flowId: string): string {
  const safe = String(flowId).replace(/[^A-Za-z0-9_]/g, '_')
  return `flow_${/^[A-Za-z]/.test(safe) ? safe : `r_${safe}`}`.slice(0, 64)
}

/** 把 Flow 文档编译为 eKuiper graph rule；不读取 layout，保证布局变化不改变产物 */
export function compileFlow (flow: FlowDoc): CompiledArtifact {
  const nodes: Record<string, EKuiperGraphNode> = {}
  const runtimeNodeMap: Record<string, string> = {}

  flow.spec.nodes.forEach(node => {
    const def = getNodeDef(node.type, node.typeVersion)
    if (!def) return
    const runtimeId = runtimeIdFor(def, node.id)
    runtimeNodeMap[node.id] = runtimeId
    nodes[runtimeId] = {
      type: def.runtimeKind,
      nodeType: def.nodeType,
      props: buildProps(node, def),
    }
  })

  const outgoing: Record<string, typeof flow.spec.edges> = {}
  flow.spec.edges.forEach(edge => {
    if (!outgoing[edge.sourceNodeId]) outgoing[edge.sourceNodeId] = []
    outgoing[edge.sourceNodeId].push(edge)
  })

  const edges: EKuiperGraph['topo']['edges'] = {}
  flow.spec.nodes.forEach(node => {
    const def = getNodeDef(node.type, node.typeVersion)
    if (!def || def.runtimeKind === 'sink') return
    const runtimeId = runtimeNodeMap[node.id]
    const list = outgoing[node.id] || []
    if (!list.length) return

    if (def.type === 'switch') {
      const branch = (port: string) => list
        .filter(edge => edge.sourcePortId === port)
        .map(edge => runtimeNodeMap[edge.targetNodeId])
        .filter(Boolean)
      edges[runtimeId] = [branch('branch-1'), branch('branch-2')]
      return
    }

    const targets = Array.from(new Set(list.map(edge => runtimeNodeMap[edge.targetNodeId]).filter(Boolean))).sort()
    if (targets.length) edges[runtimeId] = targets
  })

  const sources = flow.spec.nodes
    .filter(node => {
      const def = getNodeDef(node.type, node.typeVersion)
      return def && def.runtimeKind === 'source'
    })
    .map(node => runtimeNodeMap[node.id])
    .filter(Boolean)
    .sort()

  const options = buildOptions(flow)
  const ruleDefinition: CompiledArtifact['ruleDefinition'] = {
    graph: {
      nodes,
      topo: { sources, edges },
    },
    ...(options ? { options } : {}),
  }

  return {
    compilerVersion: 1,
    semanticHash: flowSemanticHash(flow),
    ruleId: toSafeRuleId(flow.id),
    ruleDefinition,
    runtimeNodeMap,
  }
}
