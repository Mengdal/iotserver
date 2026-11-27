import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { Button, Result, Space, Tag, Tooltip, message } from 'antd'
import {
  ArrowLeftOutlined,
  CheckCircleFilled,
  CloudUploadOutlined,
  CodeOutlined,
  ExclamationCircleOutlined,
  FullscreenExitOutlined,
  FullscreenOutlined,
  RedoOutlined,
  SafetyCertificateOutlined,
  SyncOutlined,
  UndoOutlined,
} from '@ant-design/icons'
import { useNavigate, useParams } from 'react-router-dom'
import '@xyflow/react/dist/style.css'
import { t } from '@/i18n'
import FlowCanvas from './components/FlowCanvas'
import Palette from './components/Palette'
import Inspector from './components/Inspector'
import BottomDock from './components/BottomDock'
import type { DockTab, RuntimeSnapshot } from './components/BottomDock'
import DeployDialog from './components/DeployDialog'
import type { CompiledArtifact, FlowDoc, FlowEdge, FlowNode } from './model/types'
import { compileFlow, flowSemanticHash } from './model/compile'
import { getNodeDef } from './model/nodes'
import { createNode, getFlow, newNodePosition, saveFlow } from './model/storage'
import { hasErrors, validateFlow } from './model/validate'
import './FlowStudio.less'

type SaveState = 'saved' | 'saving' | 'failed'

function clone<T> (value: T): T {
  return JSON.parse(JSON.stringify(value))
}

function isEditableTarget (target: EventTarget | null): boolean {
  const element = target as HTMLElement | null
  if (!element) return false
  const tag = element.tagName
  return tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT' || element.isContentEditable
}

const FlowStudioEditor: React.FC = () => {
  const { id = '' } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const [messageApi, contextHolder] = message.useMessage()

  const [doc, setDoc] = useState<FlowDoc | undefined>(() => getFlow(id))
  const [selectedNodeId, setSelectedNodeId] = useState<string | null>(null)
  const [artifact, setArtifact] = useState<CompiledArtifact | null>(null)
  const [saveState, setSaveState] = useState<SaveState>('saved')
  const [dockOpen, setDockOpen] = useState(true)
  const [dockTab, setDockTab] = useState<DockTab>('validation')
  const [deployOpen, setDeployOpen] = useState(false)
  const [runtime, setRuntime] = useState<RuntimeSnapshot | null>(null)
  const [historyState, setHistoryState] = useState({ undo: 0, redo: 0 })
  const [fullscreen, setFullscreen] = useState(false)

  const undoStack = useRef<FlowDoc[]>([])
  const redoStack = useRef<FlowDoc[]>([])
  const savedSnapshot = useRef<string>('')

  useEffect(() => {
    const loaded = getFlow(id)
    setDoc(loaded)
    setSelectedNodeId(null)
    setArtifact(null)
    undoStack.current = []
    redoStack.current = []
    setHistoryState({ undo: 0, redo: 0 })
    savedSnapshot.current = loaded ? JSON.stringify(loaded) : ''
  }, [id])

  const diagnostics = useMemo(() => (doc ? validateFlow(doc) : []), [doc])
  const errors = diagnostics.filter(item => item.severity === 'error')
  const selectedNode: FlowNode | null = doc
    ? doc.spec.nodes.find(node => node.id === selectedNodeId) || null
    : null

  const refreshHistory = useCallback(() => {
    setHistoryState({ undo: undoStack.current.length, redo: redoStack.current.length })
  }, [])

  const updateDoc = useCallback((updater: (prev: FlowDoc) => FlowDoc, history = true) => {
    setArtifact(null)
    setDoc(prev => {
      if (!prev) return prev
      if (history) {
        undoStack.current.push(clone(prev))
        if (undoStack.current.length > 50) undoStack.current.shift()
        redoStack.current = []
      }
      return updater(prev)
    })
    if (history) refreshHistory()
  }, [refreshHistory])

  // 自动保存：最后一次编辑后 600ms 写入 localStorage；等价于参考实现的草稿自动保存
  useEffect(() => {
    if (!doc) return undefined
    const snapshot = JSON.stringify(doc)
    if (snapshot === savedSnapshot.current) return undefined
    setSaveState('saving')
    const timer = window.setTimeout(() => {
      const ok = saveFlow(doc)
      savedSnapshot.current = snapshot
      setSaveState(ok ? 'saved' : 'failed')
    }, 600)
    return () => window.clearTimeout(timer)
  }, [doc])

  // 部署后的模拟运行指标：每 1.5s 自增，纯前端演示
  useEffect(() => {
    if (!doc || !doc.deployment) {
      setRuntime(null)
      return undefined
    }
    let tick = 0
    const build = (): RuntimeSnapshot => {
      const nodeStats: RuntimeSnapshot['nodeStats'] = {}
      let inputTotal = 0
      let outputTotal = 0
      doc.spec.nodes.forEach((node, index) => {
        const input = 120 + index * 37 + tick * 6
        const output = Math.round(input * (0.82 + (index % 3) * 0.04))
        nodeStats[node.id] = { input, output, error: 0 }
        inputTotal += input
        outputTotal += output
      })
      return { inputTotal, outputTotal, errorTotal: 0, activeNodes: doc.spec.nodes.length, nodeStats }
    }
    setRuntime(build())
    const timer = window.setInterval(() => {
      tick += 1
      setRuntime(build())
    }, 1500)
    return () => window.clearInterval(timer)
  }, [doc?.deployment?.deployedAt, doc?.spec.nodes.length])

  const undo = useCallback(() => {
    if (!undoStack.current.length) return
    setDoc(prev => {
      if (!prev) return prev
      const previous = undoStack.current.pop() as FlowDoc
      redoStack.current.push(clone(prev))
      return previous
    })
    setArtifact(null)
    refreshHistory()
  }, [refreshHistory])

  const redo = useCallback(() => {
    if (!redoStack.current.length) return
    setDoc(prev => {
      if (!prev) return prev
      const next = redoStack.current.pop() as FlowDoc
      undoStack.current.push(clone(prev))
      return next
    })
    setArtifact(null)
    refreshHistory()
  }, [refreshHistory])

  const deleteNode = useCallback((nodeId: string) => {
    updateDoc(prev => {
      const layoutNodes = { ...prev.layout.nodes }
      delete layoutNodes[nodeId]
      return {
        ...prev,
        spec: {
          ...prev.spec,
          nodes: prev.spec.nodes.filter(node => node.id !== nodeId),
          edges: prev.spec.edges.filter(edge => edge.sourceNodeId !== nodeId && edge.targetNodeId !== nodeId),
        },
        layout: { ...prev.layout, nodes: layoutNodes },
      }
    })
    setSelectedNodeId(current => (current === nodeId ? null : current))
  }, [updateDoc])

  useEffect(() => {
    const handleKey = (event: KeyboardEvent) => {
      if (event.key === 'Escape' && fullscreen) {
        setFullscreen(false)
        return
      }
      if (isEditableTarget(event.target)) return
      const meta = event.metaKey || event.ctrlKey
      if (meta && event.key.toLowerCase() === 'z') {
        event.preventDefault()
        if (event.shiftKey) redo()
        else undo()
        return
      }
      if ((event.key === 'Delete' || event.key === 'Backspace') && selectedNodeId) {
        event.preventDefault()
        deleteNode(selectedNodeId)
      }
    }
    window.addEventListener('keydown', handleKey)
    return () => window.removeEventListener('keydown', handleKey)
  }, [deleteNode, fullscreen, redo, selectedNodeId, undo])

  const addNode = useCallback((type: string, position?: { x: number, y: number }) => {
    const def = getNodeDef(type)
    if (!def || !doc) return
    const node = createNode(type)
    if (!node) return
    const point = position || newNodePosition(doc.spec.nodes.length)
    updateDoc(prev => ({
      ...prev,
      spec: { ...prev.spec, nodes: [...prev.spec.nodes, node] },
      layout: { ...prev.layout, nodes: { ...prev.layout.nodes, [node.id]: point } },
    }))
    setSelectedNodeId(node.id)
  }, [doc, updateDoc])

  const connectNodes = useCallback((edge: FlowEdge) => {
    updateDoc(prev => ({ ...prev, spec: { ...prev.spec, edges: [...prev.spec.edges, edge] } }))
  }, [updateDoc])

  const moveNode = useCallback((nodeId: string, position: { x: number, y: number }) => {
    updateDoc(prev => ({
      ...prev,
      layout: { ...prev.layout, nodes: { ...prev.layout.nodes, [nodeId]: position } },
    }))
  }, [updateDoc])

  const changeNodeName = useCallback((nodeId: string, name: string) => {
    updateDoc(prev => ({
      ...prev,
      spec: {
        ...prev.spec,
        nodes: prev.spec.nodes.map(node => node.id === nodeId ? { ...node, name } : node),
      },
    }), false)
  }, [updateDoc])

  const changeConfig = useCallback((nodeId: string, key: string, value: any) => {
    updateDoc(prev => ({
      ...prev,
      spec: {
        ...prev.spec,
        nodes: prev.spec.nodes.map(node => node.id === nodeId
          ? { ...node, config: { ...node.config, [key]: value } }
          : node),
      },
    }), false)
  }, [updateDoc])

  const changeFlowMeta = useCallback((patch: { name?: string, description?: string }) => {
    updateDoc(prev => ({ ...prev, ...patch }), false)
  }, [updateDoc])

  const changeOptions = useCallback((patch: Record<string, any>) => {
    updateDoc(prev => ({ ...prev, spec: { ...prev.spec, options: { ...prev.spec.options, ...patch } } }), false)
  }, [updateDoc])

  const compile = useCallback(() => {
    if (!doc) return
    const next = compileFlow(doc)
    setArtifact(next)
    setDockOpen(true)
    setDockTab('definition')
    if (hasErrors(diagnostics)) {
      messageApi.warning(t('已生成产物，但当前草稿仍有 {{count}} 个错误', { count: errors.length }))
    } else {
      messageApi.success(t('编译完成：{{ruleId}}', { ruleId: next.ruleId }))
    }
  }, [doc, diagnostics, errors.length, messageApi])

  const openDeploy = useCallback(() => {
    if (!doc) return
    setArtifact(compileFlow(doc))
    setDeployOpen(true)
  }, [doc])

  const handleDeployed = useCallback((compiled: CompiledArtifact) => {
    if (!doc) return
    const next: FlowDoc = {
      ...doc,
      deployment: {
        status: 'succeeded',
        ruleId: compiled.ruleId,
        semanticHash: compiled.semanticHash,
        deployedAt: new Date().toISOString(),
        targetName: 'local-ekuiper-2.4.2',
      },
    }
    setDoc(next)
    saveFlow(next)
    savedSnapshot.current = JSON.stringify(next)
    setSaveState('saved')
    setDockOpen(true)
    setDockTab('runtime')
    messageApi.success(t('部署成功（演示）'))
  }, [doc, messageApi])

  if (!doc) {
    return (
      <Result
        status="404"
        title={t('未找到流程')}
        subTitle={t('该流程不存在，或演示数据已被清理。')}
        extra={<Button type="primary" onClick={() => navigate('/advanced/rule/flow')}>{t('返回流程列表')}</Button>}
      />
    )
  }

  const deployed = doc.deployment
  const deployDirty = deployed ? deployed.semanticHash !== flowSemanticHash(doc) : false
  const saveText = saveState === 'saved' ? t('已保存') : saveState === 'saving' ? t('保存中…') : t('保存失败')
  const deployText = !deployed ? t('未部署') : deployDirty ? t('有未部署的更改') : t('已部署')

  return (
    <div className={`fs-root${fullscreen ? ' is-fullscreen' : ''}`}>
      {contextHolder}
      <header className="fs-header">
        <div className="fs-header__left">
          <Tooltip title={t('返回流程列表')}>
            <Button type="text" icon={<ArrowLeftOutlined />} onClick={() => navigate('/advanced/rule/flow')} />
          </Tooltip>
          <span className="fs-header__name">{doc.name}</span>
          <Tag color="blue">{t('演示')}</Tag>
        </div>

        <div className="fs-header__status">
          <span className={`fs-status fs-status--save-${saveState}`}>
            {saveState === 'saving' ? <SyncOutlined spin /> : saveState === 'failed' ? <ExclamationCircleOutlined /> : <CheckCircleFilled />}
            {saveText}
          </span>
          <span className={`fs-status ${deployed ? (deployDirty ? 'fs-status--dirty' : 'fs-status--deployed') : ''}`}>
            <span className="fs-status__dot" />
            {deployText}
          </span>
          <span className="fs-header__target">{t('目标')}: {t('本机 eKuiper 2.4.2')}</span>
        </div>

        <Space size={6} className="fs-header__actions">
          <Tooltip title={fullscreen ? t('退出全屏') : t('全屏编辑')}>
            <Button
              size="small"
              icon={fullscreen ? <FullscreenExitOutlined /> : <FullscreenOutlined />}
              onClick={() => setFullscreen(value => !value)}
            />
          </Tooltip>
          <Tooltip title={t('撤销')}>
            <Button size="small" icon={<UndoOutlined />} disabled={!historyState.undo} onClick={undo} />
          </Tooltip>
          <Tooltip title={t('重做')}>
            <Button size="small" icon={<RedoOutlined />} disabled={!historyState.redo} onClick={redo} />
          </Tooltip>
          <Button
            size="small"
            icon={<SafetyCertificateOutlined />}
            onClick={() => { setDockOpen(true); setDockTab('validation') }}
          >
            {t('校验')}{errors.length > 0 && <span className="fs-btn-count">{errors.length}</span>}
          </Button>
          <Button size="small" icon={<CodeOutlined />} onClick={compile}>{t('编译')}</Button>
          <Button size="small" type="primary" icon={<CloudUploadOutlined />} onClick={openDeploy}>{t('部署')}</Button>
        </Space>
      </header>

      <div className="fs-body">
        <Palette onAdd={type => addNode(type)} />
        <div className="fs-center">
          <div className="fs-canvas">
            <FlowCanvas
              flow={doc}
              diagnostics={diagnostics}
              selectedNodeId={selectedNodeId}
              onSelectNode={setSelectedNodeId}
              onAddNode={addNode}
              onConnect={connectNodes}
              onNodeDragStop={moveNode}
              fullscreen={fullscreen}
            />
          </div>
          <BottomDock
            open={dockOpen}
            tab={dockTab}
            diagnostics={diagnostics}
            artifact={artifact}
            flow={doc}
            deployment={deployed}
            runtime={runtime}
            onToggle={() => setDockOpen(value => !value)}
            onTabChange={setDockTab}
            onCompile={compile}
            onSelectNode={nodeId => setSelectedNodeId(nodeId)}
          />
        </div>
        <Inspector
          flow={doc}
          node={selectedNode}
          diagnostics={diagnostics}
          onChangeNodeName={changeNodeName}
          onChangeConfig={changeConfig}
          onDeleteNode={deleteNode}
          onChangeFlowMeta={changeFlowMeta}
          onChangeOptions={changeOptions}
        />
      </div>

      <DeployDialog
        open={deployOpen}
        flow={doc}
        diagnostics={diagnostics}
        artifact={artifact}
        onClose={() => setDeployOpen(false)}
        onDeployed={handleDeployed}
        onOpenRuntime={() => {
          setDeployOpen(false)
          setDockOpen(true)
          setDockTab('runtime')
        }}
      />
    </div>
  )
}

export default FlowStudioEditor
