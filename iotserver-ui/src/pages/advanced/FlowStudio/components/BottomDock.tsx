import React from 'react'
import { Button, Empty, Table, Tag, message } from 'antd'
import {
  CheckCircleOutlined,
  CodeOutlined,
  CopyOutlined,
  DownOutlined,
  ExclamationCircleOutlined,
  PlayCircleOutlined,
  UpOutlined,
  WarningOutlined,
} from '@ant-design/icons'
import { t } from '@/i18n'
import type { CompiledArtifact, Diagnostic, FlowDoc, FlowDeployment } from '../model/types'

export type DockTab = 'validation' | 'definition' | 'runtime'

export interface RuntimeSnapshot {
  inputTotal: number
  outputTotal: number
  errorTotal: number
  activeNodes: number
  nodeStats: Record<string, { input: number, output: number, error: number }>
}

export interface BottomDockProps {
  open: boolean
  tab: DockTab
  diagnostics: Diagnostic[]
  artifact: CompiledArtifact | null
  flow: FlowDoc
  deployment?: FlowDeployment
  runtime: RuntimeSnapshot | null
  onToggle: () => void
  onTabChange: (tab: DockTab) => void
  onCompile: () => void
  onSelectNode: (nodeId: string) => void
}

const BottomDock: React.FC<BottomDockProps> = ({
  open,
  tab,
  diagnostics,
  artifact,
  flow,
  deployment,
  runtime,
  onToggle,
  onTabChange,
  onCompile,
  onSelectNode,
}) => {
  const [messageApi, contextHolder] = message.useMessage()
  const errors = diagnostics.filter(item => item.severity === 'error')
  const warnings = diagnostics.filter(item => item.severity === 'warning')

  const copyDefinition = async () => {
    if (!artifact) return
    try {
      await navigator.clipboard.writeText(JSON.stringify(artifact.ruleDefinition, null, 2))
      messageApi.success(t('已复制'))
    } catch (error) {
      messageApi.error(t('复制失败，请手动选择文本'))
    }
  }

  if (!open) {
    return (
      <div className="fs-dock fs-dock--collapsed">
        {contextHolder}
        <button type="button" className="fs-dock__bar" onClick={onToggle}>
          <span className={`fs-dock__summary ${errors.length ? 'is-error' : warnings.length ? 'is-warning' : 'is-ok'}`}>
            {errors.length
              ? <><ExclamationCircleOutlined /> {t('{{count}} 个错误', { count: errors.length })}</>
              : warnings.length
                ? <><WarningOutlined /> {t('{{count}} 个警告', { count: warnings.length })}</>
                : <><CheckCircleOutlined /> {t('校验通过')}</>}
          </span>
          <span className="fs-dock__summary-hint">{t('展开校验、编译与运行状态')}</span>
          <UpOutlined />
        </button>
      </div>
    )
  }

  const tabs: Array<{ key: DockTab, label: string, icon: React.ReactNode, extra?: React.ReactNode }> = [
    {
      key: 'validation',
      label: t('校验'),
      icon: <CheckCircleOutlined />,
      extra: errors.length ? <span className="fs-dock__count is-error">{errors.length}</span> : null,
    },
    { key: 'definition', label: t('编译产物'), icon: <CodeOutlined /> },
    { key: 'runtime', label: t('运行状态'), icon: <PlayCircleOutlined /> },
  ]

  const runtimeColumns = [
    { title: t('节点'), dataIndex: 'name', render: (value: string) => <span>{value}</span> },
    { title: t('输入'), dataIndex: 'input', width: 100 },
    { title: t('输出'), dataIndex: 'output', width: 100 },
    { title: t('异常'), dataIndex: 'error', width: 100 },
  ]
  const runtimeRows = runtime
    ? flow.spec.nodes.map(node => ({
      key: node.id,
      name: node.name,
      input: runtime.nodeStats[node.id]?.input ?? 0,
      output: runtime.nodeStats[node.id]?.output ?? 0,
      error: runtime.nodeStats[node.id]?.error ?? 0,
    }))
    : []

  return (
    <div className="fs-dock">
      {contextHolder}
      <div className="fs-dock__head">
        <div className="fs-dock__tabs">
          {tabs.map(item => (
            <button
              key={item.key}
              type="button"
              className={`fs-dock__tab${tab === item.key ? ' is-active' : ''}`}
              onClick={() => onTabChange(item.key)}
            >
              {item.icon}
              {item.label}
              {item.extra}
            </button>
          ))}
        </div>
        <div className="fs-dock__actions">
          <Button size="small" type="text" icon={<DownOutlined />} onClick={onToggle}>{t('收起')}</Button>
        </div>
      </div>

      <div className="fs-dock__body">
        {tab === 'validation' && (
          <div className="fs-validation">
            {diagnostics.length === 0 && (
              <div className="fs-empty fs-empty--ok">
                <CheckCircleOutlined />
                <span>{t('未发现问题。')}</span>
              </div>
            )}
            {errors.length > 0 && (
              <div className="fs-validation__group">
                <div className="fs-validation__title is-error">{t('错误')} · {errors.length}</div>
                {errors.map((item, index) => (
                  <button
                    type="button"
                    key={`${item.code}-${index}`}
                    className="fs-validation__item is-error"
                    onClick={() => item.nodeId && onSelectNode(item.nodeId)}
                  >
                    <ExclamationCircleOutlined />
                    <span>{item.message}</span>
                    {item.nodeId && <span className="fs-validation__loc">{t('定位')}</span>}
                  </button>
                ))}
              </div>
            )}
            {warnings.length > 0 && (
              <div className="fs-validation__group">
                <div className="fs-validation__title is-warning">{t('警告')} · {warnings.length}</div>
                {warnings.map((item, index) => (
                  <button
                    type="button"
                    key={`${item.code}-${index}`}
                    className="fs-validation__item is-warning"
                    onClick={() => item.nodeId && onSelectNode(item.nodeId)}
                  >
                    <WarningOutlined />
                    <span>{item.message}</span>
                    {item.nodeId && <span className="fs-validation__loc">{t('定位')}</span>}
                  </button>
                ))}
              </div>
            )}
          </div>
        )}

        {tab === 'definition' && (
          <div className="fs-definition">
            {!artifact && (
              <div className="fs-empty">
                <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={t('尚未生成编译产物')} />
                <Button type="primary" size="small" icon={<CodeOutlined />} onClick={onCompile}>{t('编译当前草稿')}</Button>
              </div>
            )}
            {artifact && (
              <>
                <div className="fs-definition__meta">
                  <Tag color="blue">{artifact.ruleId}</Tag>
                  <span>compiler v{artifact.compilerVersion}</span>
                  <span>{t('语义哈希')}: {artifact.semanticHash}</span>
                  <Button size="small" icon={<CopyOutlined />} onClick={copyDefinition}>{t('复制 JSON')}</Button>
                </div>
                <pre className="fs-code">{JSON.stringify(artifact.ruleDefinition, null, 2)}</pre>
              </>
            )}
          </div>
        )}

        {tab === 'runtime' && (
          <div className="fs-runtime">
            {!deployment && (
              <div className="fs-empty">
                <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={t('部署后可查看运行状态')} />
              </div>
            )}
            {deployment && runtime && (
              <>
                <div className="fs-runtime__summary">
                  <div className="fs-runtime__state">
                    <span className="fs-runtime__dot" />
                    <span>{t('运行中（模拟）')}</span>
                  </div>
                  <Tag color="blue">{deployment.ruleId}</Tag>
                  <span className="fs-runtime__meta">{t('部署时间')}: {new Date(deployment.deployedAt).toLocaleString()}</span>
                </div>
                <div className="fs-runtime__cards">
                  <div className="fs-runtime__card"><span>{t('输入')}</span><b>{runtime.inputTotal.toLocaleString()}</b></div>
                  <div className="fs-runtime__card"><span>{t('输出')}</span><b>{runtime.outputTotal.toLocaleString()}</b></div>
                  <div className="fs-runtime__card"><span>{t('异常')}</span><b>{runtime.errorTotal.toLocaleString()}</b></div>
                  <div className="fs-runtime__card"><span>{t('活跃节点')}</span><b>{runtime.activeNodes}</b></div>
                </div>
                <Table
                  size="small"
                  pagination={false}
                  columns={runtimeColumns}
                  dataSource={runtimeRows}
                  className="fs-runtime__table"
                />
              </>
            )}
          </div>
        )}
      </div>
    </div>
  )
}

export default BottomDock
