import React, { useState } from 'react'
import { Button, Divider, Empty, Input, InputNumber, Select, Switch, Tag } from 'antd'
import { DeleteOutlined } from '@ant-design/icons'
import { t } from '@/i18n'
import type { Diagnostic, FlowDoc, FlowNode, PropertyDef } from '../model/types'
import { getNodeDef } from '../model/nodes'

export interface InspectorProps {
  flow: FlowDoc
  node: FlowNode | null
  diagnostics: Diagnostic[]
  onChangeNodeName: (nodeId: string, name: string) => void
  onChangeConfig: (nodeId: string, key: string, value: any) => void
  onDeleteNode: (nodeId: string) => void
  onChangeFlowMeta: (patch: { name?: string, description?: string }) => void
  onChangeOptions: (patch: Record<string, any>) => void
}

const JsonField: React.FC<{ value: any, onChange: (value: any) => void }> = ({ value, onChange }) => {
  const [text, setText] = useState(() => JSON.stringify(value ?? {}, null, 2))
  const [error, setError] = useState('')
  const commit = (next: string) => {
    setText(next)
    try {
      const parsed = JSON.parse(next)
      setError('')
      onChange(parsed)
    } catch (err) {
      setError(t('JSON 无效'))
    }
  }
  return (
    <div>
      <Input.TextArea rows={4} value={text} onChange={event => commit(event.target.value)} className="fs-mono" />
      {error && <div className="fs-field-error">{error}</div>}
    </div>
  )
}

const PropertyField: React.FC<{
  prop: PropertyDef
  value: any
  onChange: (value: any) => void
}> = ({ prop, value, onChange }) => {
  let control: React.ReactNode
  if (prop.type === 'number') {
    control = (
      <InputNumber
        style={{ width: '100%' }}
        value={value === '' || value === undefined || value === null ? undefined : Number(value)}
        min={prop.min}
        max={prop.max}
        step={prop.step}
        placeholder={prop.placeholder}
        onChange={next => onChange(next === null ? undefined : next)}
      />
    )
  } else if (prop.type === 'boolean') {
    control = <Switch checked={Boolean(value)} onChange={checked => onChange(checked)} />
  } else if (prop.type === 'select') {
    control = (
      <Select
        style={{ width: '100%' }}
        value={value === '' || value === undefined ? undefined : value}
        placeholder={prop.placeholder}
        options={prop.options}
        onChange={next => onChange(next)}
      />
    )
  } else if (prop.type === 'json') {
    control = <JsonField value={value} onChange={onChange} />
  } else if (prop.type === 'expression') {
    control = (
      <Input.TextArea
        rows={3}
        className="fs-mono"
        value={value === undefined || value === null ? '' : String(value)}
        placeholder={prop.placeholder}
        onChange={event => onChange(event.target.value)}
      />
    )
  } else {
    control = (
      <Input
        value={value === undefined || value === null ? '' : String(value)}
        placeholder={prop.placeholder}
        onChange={event => onChange(event.target.value)}
      />
    )
  }

  return (
    <div className="fs-field" data-property={prop.key}>
      <div className="fs-field__label">
        {prop.required && <span className="fs-field__required">*</span>}
        {t(prop.label)}
      </div>
      {control}
      {prop.hint && <div className="fs-field__hint">{t(prop.hint)}</div>}
    </div>
  )
}

function diagnosticsForNode (flow: FlowDoc, diagnostics: Diagnostic[], nodeId: string): Diagnostic[] {
  const relatedEdges = flow.spec.edges
    .filter(edge => edge.sourceNodeId === nodeId || edge.targetNodeId === nodeId)
    .map(edge => edge.id)
  return diagnostics.filter(item => item.nodeId === nodeId || (item.edgeId && relatedEdges.includes(item.edgeId)))
}

const Inspector: React.FC<InspectorProps> = ({
  flow,
  node,
  diagnostics,
  onChangeNodeName,
  onChangeConfig,
  onDeleteNode,
  onChangeFlowMeta,
  onChangeOptions,
}) => {
  if (!node) {
    const options = flow.spec.options || {}
    return (
      <aside className="fs-inspector" aria-label={t('流程设置')}>
        <div className="fs-inspector__head">
          <div className="fs-inspector__title">{t('流程设置')}</div>
          <Tag color="blue">{t('演示')}</Tag>
        </div>
        <div className="fs-inspector__body">
          <div className="fs-field">
            <div className="fs-field__label">{t('流程名称')}</div>
            <Input value={flow.name} onChange={event => onChangeFlowMeta({ name: event.target.value })} />
          </div>
          <div className="fs-field">
            <div className="fs-field__label">{t('流程描述')}</div>
            <Input.TextArea
              rows={2}
              value={flow.description}
              onChange={event => onChangeFlowMeta({ description: event.target.value })}
            />
          </div>
          <Divider plain className="fs-divider">{t('规则选项')}</Divider>
          <div className="fs-field">
            <div className="fs-field__label">{t('并发度')}</div>
            <InputNumber
              style={{ width: '100%' }}
              min={0}
              value={options.concurrency}
              onChange={value => onChangeOptions({ concurrency: value === null ? undefined : value })}
            />
            <div className="fs-field__hint">{t('规则执行的并行度，0 表示引擎默认值。')}</div>
          </div>
          <div className="fs-field">
            <div className="fs-field__label">{t('缓存区长度')}</div>
            <InputNumber
              style={{ width: '100%' }}
              min={0}
              value={options.bufferLength}
              onChange={value => onChangeOptions({ bufferLength: value === null ? undefined : value })}
            />
          </div>
          <div className="fs-field">
            <div className="fs-field__label">{t('队列 QoS')}</div>
            <Select
              allowClear
              style={{ width: '100%' }}
              placeholder={t('引擎默认值')}
              value={options.qos}
              options={[{ label: '0', value: 0 }, { label: '1', value: 1 }, { label: '2', value: 2 }]}
              onChange={value => onChangeOptions({ qos: value })}
            />
          </div>
          <div className="fs-field fs-field--switch">
            <div>
              <div className="fs-field__label">{t('事件时间')}</div>
              <div className="fs-field__hint">{t('使用事件时间而非处理时间。')}</div>
            </div>
            <Switch checked={Boolean(options.isEventTime)} onChange={checked => onChangeOptions({ isEventTime: checked })} />
          </div>
          <div className="fs-field fs-field--switch">
            <div>
              <div className="fs-field__label">{t('向目标发送元数据')}</div>
              <div className="fs-field__hint">{t('将消息元数据一并发送给目标。')}</div>
            </div>
            <Switch checked={Boolean(options.sendMetaToSink)} onChange={checked => onChangeOptions({ sendMetaToSink: checked })} />
          </div>
          <Divider plain className="fs-divider">{t('部署目标')}</Divider>
          <div className="fs-target-card">
            <div className="fs-target-card__dot" />
            <div>
              <div className="fs-target-card__name">{t('本机 eKuiper 2.4.2')}</div>
              <div className="fs-target-card__hint">{t('演示模式：不会真正下发规则')}</div>
            </div>
          </div>
          <div className="fs-inspector__hint">{t('选择一个节点以编辑它的属性。')}</div>
        </div>
      </aside>
    )
  }

  const def = getNodeDef(node.type, node.typeVersion)
  const items = diagnosticsForNode(flow, diagnostics, node.id)

  return (
    <aside className="fs-inspector" aria-label={t('节点检查器')}>
      <div className="fs-inspector__head">
        <div className="fs-inspector__title">{t('节点检查器')}</div>
        <Tag color="blue">{t('演示')}</Tag>
      </div>
      <div className="fs-inspector__body">
        <div className="fs-inspector__node">
          <span className="fs-inspector__node-icon" style={{ background: def?.accent || '#8c8c8c' }}>{def?.icon || '?'}</span>
          <div>
            <div className="fs-inspector__node-name">{t(def?.displayName || node.type)}</div>
            <div className="fs-inspector__node-type">{node.type}@{node.typeVersion}</div>
          </div>
        </div>

        {items.length > 0 && (
          <div className="fs-inspector__diagnostics">
            {items.map((item, index) => (
              <div className={`fs-diagnostic fs-diagnostic--${item.severity}`} key={`${item.code}-${index}`}>
                <Tag color={item.severity === 'error' ? 'red' : item.severity === 'warning' ? 'orange' : 'blue'}>
                  {item.severity === 'error' ? t('错误') : item.severity === 'warning' ? t('警告') : t('提示')}
                </Tag>
                <span>{item.message}</span>
              </div>
            ))}
          </div>
        )}

        <div className="fs-field">
          <div className="fs-field__label">{t('显示名称')}</div>
          <Input value={node.name} onChange={event => onChangeNodeName(node.id, event.target.value)} />
        </div>

        {def && def.properties.length > 0 && (
          <>
            <Divider plain className="fs-divider">{t('节点属性')}</Divider>
            {def.properties.map(prop => (
              <PropertyField
                key={prop.key}
                prop={prop}
                value={node.config ? node.config[prop.key] : undefined}
                onChange={value => onChangeConfig(node.id, prop.key, value)}
              />
            ))}
          </>
        )}

        {def && def.properties.length === 0 && (
          <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={t('该节点没有可配置的属性')} />
        )}

        <Divider plain className="fs-divider" />
        <Button danger block icon={<DeleteOutlined />} onClick={() => onDeleteNode(node.id)}>
          {t('删除节点')}
        </Button>
      </div>
    </aside>
  )
}

export default Inspector
