import React, { useCallback, useEffect, useMemo, useState } from 'react'
import {
  Background,
  BackgroundVariant,
  Controls,
  MarkerType,
  Panel,
  ReactFlow,
  ReactFlowProvider,
  applyNodeChanges,
  useReactFlow,
} from '@xyflow/react'
import type { Connection, Edge, Node, NodeChange } from '@xyflow/react'
import { t } from '@/i18n'
import type { Diagnostic, FlowDoc, FlowEdge } from '../model/types'
import { NODE_DEF_MAP, getNodeDef } from '../model/nodes'
import { newNodePosition } from '../model/storage'
import { checkConnection } from '../model/validate'
import FlowNodeView from './FlowNodeView'
import type { FlowNodeData } from './FlowNodeView'

const nodeTypes = { flowNode: FlowNodeView }

export interface FlowCanvasProps {
  flow: FlowDoc
  diagnostics: Diagnostic[]
  selectedNodeId: string | null
  onSelectNode: (id: string | null) => void
  onAddNode: (type: string, position: { x: number, y: number }) => void
  onConnect: (edge: FlowEdge) => void
  onNodeDragStop: (nodeId: string, position: { x: number, y: number }) => void
  fullscreen?: boolean
}

function CanvasInner ({
  flow,
  diagnostics,
  selectedNodeId,
  onSelectNode,
  onAddNode,
  onConnect,
  onNodeDragStop,
  fullscreen,
}: FlowCanvasProps) {
  const { screenToFlowPosition, fitView } = useReactFlow()
  const [nodes, setNodes] = useState<Node[]>([])

  const diagnosticsForNode = useCallback((nodeId: string): Diagnostic[] => {
    const direct = diagnostics.filter(item => item.nodeId === nodeId)
    const relatedEdges = flow.spec.edges
      .filter(edge => edge.sourceNodeId === nodeId || edge.targetNodeId === nodeId)
      .map(edge => edge.id)
    const edgeItems = diagnostics.filter(item => item.edgeId && relatedEdges.includes(item.edgeId))
    return direct.concat(edgeItems)
  }, [diagnostics, flow.spec.edges])

  const buildNodes = useCallback((): Node[] => flow.spec.nodes.map((node, index) => {
    const def = getNodeDef(node.type, node.typeVersion)
    const position = flow.layout.nodes[node.id] || newNodePosition(index)
    return {
      id: node.id,
      type: 'flowNode',
      position,
      selected: node.id === selectedNodeId,
      data: {
        name: node.name,
        def: def || NODE_DEF_MAP.filter,
        config: node.config,
        diagnostics: diagnosticsForNode(node.id),
      } as FlowNodeData,
    }
  }), [flow.spec.nodes, flow.layout.nodes, selectedNodeId, diagnosticsForNode])

  useEffect(() => {
    setNodes(buildNodes())
  }, [buildNodes])

  // 全屏切换后容器尺寸变了，重新适配一次视图
  useEffect(() => {
    const timer = window.setTimeout(() => fitView({ padding: 0.25 }), 80)
    return () => window.clearTimeout(timer)
  }, [fullscreen, fitView])

  const edges: Edge[] = useMemo(() => flow.spec.edges.map(edge => ({
    id: edge.id,
    source: edge.sourceNodeId,
    target: edge.targetNodeId,
    sourceHandle: edge.sourcePortId,
    targetHandle: edge.targetPortId,
    type: 'smoothstep',
    markerEnd: { type: MarkerType.ArrowClosed, width: 15, height: 15, color: '#94a3b8' },
    style: { stroke: '#94a3b8', strokeWidth: 1.6 },
  })), [flow.spec.edges])

  const handleNodesChange = useCallback((changes: NodeChange[]) => {
    const positionChanges = changes.filter(change => change.type === 'position')
    if (positionChanges.length) {
      setNodes(current => applyNodeChanges(positionChanges, current))
    }
  }, [])

  const handleConnect = useCallback((connection: Connection) => {
    if (!connection.source || !connection.target) return
    const check = checkConnection(flow, connection)
    if (!check.ok) return
    onConnect({
      id: `e_${Date.now().toString(36)}_${Math.random().toString(36).slice(2, 7)}`,
      sourceNodeId: connection.source,
      sourcePortId: connection.sourceHandle || 'out',
      targetNodeId: connection.target,
      targetPortId: connection.targetHandle || 'in',
    })
  }, [flow, onConnect])

  const isValidConnection = useCallback((connection: Connection | Edge) => {
    return checkConnection(flow, connection).ok
  }, [flow])

  const handleDrop = useCallback((event: React.DragEvent) => {
    event.preventDefault()
    const type = event.dataTransfer.getData('application/x-iotserver-flow-node')
    if (!type || !NODE_DEF_MAP[type]) return
    onAddNode(type, screenToFlowPosition({ x: event.clientX, y: event.clientY }))
  }, [onAddNode, screenToFlowPosition])

  return (
    <ReactFlow
      nodes={nodes}
      edges={edges}
      nodeTypes={nodeTypes}
      onNodesChange={handleNodesChange}
      onNodesDelete={undefined}
      onNodeClick={(_, node) => onSelectNode(node.id)}
      onPaneClick={() => onSelectNode(null)}
      onNodeDragStop={(_, node) => onNodeDragStop(node.id, node.position)}
      onConnect={handleConnect}
      isValidConnection={isValidConnection}
      onDragOver={event => {
        event.preventDefault()
        event.dataTransfer.dropEffect = 'move'
      }}
      onDrop={handleDrop}
      deleteKeyCode={null}
      selectionOnDrag={false}
      fitView
      fitViewOptions={{ padding: 0.25 }}
      minZoom={0.3}
      maxZoom={1.6}
      proOptions={{ hideAttribution: true }}
      className="fs-canvas-flow"
    >
      <Background variant={BackgroundVariant.Dots} gap={18} size={1} color="#d7dee9" />
      <Controls showInteractive={false} position="bottom-left" />
      {!flow.spec.nodes.length && (
        <Panel position="top-center">
          <div className="fs-canvas-empty">{t('从左侧拖入节点开始编排')}</div>
        </Panel>
      )}
    </ReactFlow>
  )
}

const FlowCanvas: React.FC<FlowCanvasProps> = props => (
  <ReactFlowProvider>
    <CanvasInner {...props} />
  </ReactFlowProvider>
)

export default FlowCanvas
