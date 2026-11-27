/** Flow Studio 演示版的领域模型。
 *
 * 设计参考 ekuiper-manager 的 flow.ekuiper-manager.io/v1alpha1：
 * 语义（nodes/edges/options）与布局（坐标）分开存，布局变化不影响编译结果。
 */

export type NodeCategory = 'source' | 'transform' | 'streaming' | 'routing' | 'sink'

export type PortKind = 'stream' | 'collection' | 'table' | 'any'

export type PropertyType = 'string' | 'number' | 'boolean' | 'select' | 'expression' | 'json'

export interface PortDef {
  id: string
  label: string
  kind: PortKind
}

export interface PropertyOption {
  label: string
  value: string | number
}

export interface PropertyDef {
  key: string
  label: string
  type: PropertyType
  required?: boolean
  placeholder?: string
  hint?: string
  min?: number
  max?: number
  step?: number
  options?: PropertyOption[]
}

export type RuntimeKind = 'source' | 'operator' | 'sink'

export interface NodeDef {
  type: string
  version: number
  displayName: string
  description: string
  category: NodeCategory
  icon: string
  /** 节点分类色，与画布节点底部的色条一致 */
  accent: string
  inputs: PortDef[]
  outputs: PortDef[]
  properties: PropertyDef[]
  /** 节点副标题取哪个配置字段，例如 topic/url/expression */
  subtitleKey?: string
  runtimeKind: RuntimeKind
  /** 编译后的 eKuiper nodeType */
  nodeType: string
}

export interface FlowNode {
  id: string
  type: string
  typeVersion: number
  name: string
  config: Record<string, any>
}

export interface FlowEdge {
  id: string
  sourceNodeId: string
  sourcePortId: string
  targetNodeId: string
  targetPortId: string
}

export interface FlowOptions {
  concurrency?: number
  bufferLength?: number
  qos?: number
  isEventTime?: boolean
  sendMetaToSink?: boolean
  sendError?: boolean
}

export interface FlowSpec {
  nodes: FlowNode[]
  edges: FlowEdge[]
  options: FlowOptions
}

export interface FlowLayout {
  nodes: Record<string, { x: number; y: number }>
  viewport?: { x: number; y: number; zoom: number }
}

export interface FlowDeployment {
  status: 'succeeded'
  ruleId: string
  semanticHash: string
  deployedAt: string
  targetName: string
}

export interface FlowDoc {
  id: string
  name: string
  description: string
  spec: FlowSpec
  layout: FlowLayout
  updatedAt: string
  deployment?: FlowDeployment
}

export type DiagnosticSeverity = 'error' | 'warning' | 'info'

export interface Diagnostic {
  code: string
  severity: DiagnosticSeverity
  message: string
  nodeId?: string
  edgeId?: string
  propertyPath?: string
}

export interface EKuiperGraphNode {
  type: RuntimeKind
  nodeType: string
  props: Record<string, any>
}

export interface EKuiperGraph {
  nodes: Record<string, EKuiperGraphNode>
  topo: {
    sources: string[]
    edges: Record<string, string[] | string[][]>
  }
}

export interface CompiledArtifact {
  compilerVersion: number
  semanticHash: string
  ruleId: string
  ruleDefinition: {
    graph: EKuiperGraph
    options?: FlowOptions
  }
  runtimeNodeMap: Record<string, string>
}
