import React, { useMemo, useState } from 'react'
import { Button, Card, Collapse, Empty, Input, Modal, Space, Tabs, Tag, Tooltip, Typography, message } from 'antd'
import {
  ApiOutlined,
  CloudServerOutlined,
  CopyOutlined,
  DatabaseOutlined,
  InboxOutlined,
  PlusOutlined,
  ThunderboltOutlined,
} from '@ant-design/icons'
import { getDataResource } from '@/api'
import type { RuleEngineAction } from '@/api/RuleEngine'
import useRequest from '@/hooks/useRequest'
import { t, tt } from '@/i18n'
import { MQTT_STREAM } from '../editorUtils'

interface DataResourceItem {
  id: string
  name: string
  type: string
  health: boolean | null
  option?: Record<string, unknown>
}

interface ActionPanelProps {
  actions: RuleEngineAction[]
  onChange: (actions: RuleEngineAction[]) => void
  disabled?: boolean
  readOnly?: boolean
  // 数据源示例里的「使用 SQL」：把示例 SQL 填入左侧编辑器
  onUseSql?: (sql: string) => void
}

const SAMPLE_SQL = `SELECT *\nFROM ${MQTT_STREAM}`

const TYPE_ICON: Record<string, React.ReactNode> = {
  HTTP推送: <ApiOutlined />,
  消息对队列MQTT: <ThunderboltOutlined />,
  消息队列Kafka: <InboxOutlined />,
  InfluxDB: <DatabaseOutlined />,
}

function actionTypeLabel (type: string): string {
  return tt(type) || type
}

export const ActionPanel: React.FC<ActionPanelProps> = ({ actions, onChange, disabled, readOnly, onUseSql }) => {
  const [pickerOpen, setPickerOpen] = useState(false)
  const [keyword, setKeyword] = useState('')
  const [selectedId, setSelectedId] = useState<string | undefined>(actions[0]?.resource_id)
  const resourceApi = useRequest(getDataResource, { manual: true })
  const [messageApi, contextHolder] = message.useMessage()

  const copyText = async (text: string) => {
    try {
      await navigator.clipboard.writeText(text)
      messageApi.success(t('已复制'))
    } catch (error) {
      messageApi.error(t('复制失败，请手动选中复制'))
    }
  }

  const useSampleSql = () => {
    if (!onUseSql) return
    onUseSql(SAMPLE_SQL)
    messageApi.success(t('已应用到 SQL 编辑器'))
  }

  const resources: DataResourceItem[] = useMemo(() => (resourceApi.data?.list || []) as DataResourceItem[], [resourceApi.data])
  const selectedIds = useMemo(() => new Set(actions.map(action => action.resource_id)), [actions])
  const filtered = useMemo(() => {
    const text = keyword.trim().toLowerCase()
    if (!text) return resources
    return resources.filter(item => `${item.name} ${item.type}`.toLowerCase().includes(text))
  }, [keyword, resources])

  const openPicker = () => {
    setSelectedId(undefined)
    setKeyword('')
    setPickerOpen(true)
    resourceApi.run({ page: 1, pageSize: 200 })
  }

  const appendAction = () => {
    const resource = resources.find(item => item.id === selectedId)
    if (!resource) {
      messageApi.warning(t('请选择应用'))
      return
    }
    if (selectedIds.has(resource.id)) {
      messageApi.warning(t('该应用已添加'))
      return
    }
    if (actions.length >= 10) {
      messageApi.warning(t('应用数量不能超过 {{num}} 个', { num: 10 }))
      return
    }
    onChange([...actions, { resource_id: resource.id, resource_name: resource.name, resource_type: resource.type }])
    setPickerOpen(false)
  }

  const removeAction = (id: string) => {
    onChange(actions.filter(action => action.resource_id !== id))
  }

  const actionTab = (
    <div className="rule-action-list">
      <div className="rule-action-toolbar">
        <span className="rule-action-count">{t('已添加 {{num}} 个应用', { num: actions.length })}</span>
        {!readOnly && (
          <Button type="link" size="small" icon={<PlusOutlined />} disabled={disabled} onClick={openPicker}>
            {t('添加应用')}
          </Button>
        )}
      </div>
      {actions.length === 0
        ? <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={t('暂未添加应用')} />
        : actions.map(action => (
          <Card key={action.resource_id} size="small" className="rule-action-card">
            <div className="rule-action-card-main">
              <div className="rule-action-info">
                <div className="rule-action-name">
                  <Tooltip title={action.resource_name || action.resource_id}>
                    <Typography.Link
                      ellipsis
                      href={`/advanced/resource/manage?type=${encodeURIComponent(action.resource_type)}`}
                      target="_blank"
                      rel="noopener noreferrer"
                    >
                      {action.resource_name || action.resource_id}
                    </Typography.Link>
                  </Tooltip>
                </div>
              </div>
              {!readOnly && (
                <Typography.Link
                  type="danger"
                  onClick={disabled ? undefined : () => removeAction(action.resource_id)}
                  style={disabled ? { opacity: 0.45, pointerEvents: 'none' } : undefined}
                >
                  {t('移除')}
                </Typography.Link>
              )}
            </div>
          </Card>
        ))}
    </div>
  )

  const sampleTab = (
    <div className="rule-source-sample">
      <div className="rule-source-sample-title">{t('已创建的数据源，以及用法示例')}</div>
      <Collapse bordered={false} className="rule-source-collapse">
        <Collapse.Panel
          key={MQTT_STREAM}
          header={<span className="rule-source-name">{MQTT_STREAM}</span>}
        >
          <div className="rule-source-type">{t('数据源类型')}: {t('平台消息总线')}</div>
          <div className="rule-source-section-head">
            <span className="rule-source-section-title">{t('SQL 语句')}</span>
            <Button size="small" type="primary" onClick={useSampleSql}>{t('使用 SQL')}</Button>
          </div>
          <div className="rule-source-code">
            <pre className="rule-sql-example-code">{SAMPLE_SQL}</pre>
            <Button
              type="text"
              size="small"
              className="rule-source-code-copy"
              icon={<CopyOutlined />}
              onClick={() => copyText(SAMPLE_SQL)}
            />
          </div>
        </Collapse.Panel>
      </Collapse>
    </div>
  )

  return (
    <>
      {contextHolder}
      <Tabs
        className="rule-action-tabs"
        items={[
          { key: 'actions', label: t('应用'), children: actionTab },
          { key: 'sample', label: t('数据源示例'), children: sampleTab },
        ]}
      />
      <Modal
        title={t('添加应用')}
        open={pickerOpen}
        onCancel={() => setPickerOpen(false)}
        onOk={appendAction}
        okText={t('添加')}
        cancelText={t('取消')}
        width={760}
        destroyOnClose
      >
        <Input.Search
          allowClear
          value={keyword}
          onChange={event => setKeyword(event.target.value)}
          placeholder={t('按名称/类型搜索应用')}
          className="mb15"
        />
        <div className="rule-resource-picker">
          {resourceApi.loading && <div className="rule-resource-loading">{t('加载中...')}</div>}
          {!resourceApi.loading && filtered.length === 0 && (
            <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={t('暂无可用应用，请先到北向应用添加')} />
          )}
          {filtered.map(item => {
            const active = selectedId === item.id
            const disabledItem = selectedIds.has(item.id) || item.health === false
            return (
              <div
                key={item.id}
                className={`rule-resource-item${active ? ' is-active' : ''}${disabledItem ? ' is-disabled' : ''}`}
                onClick={() => {
                  if (!disabledItem) setSelectedId(item.id)
                }}
              >
                <div className="rule-resource-icon">{TYPE_ICON[item.type] || <CloudServerOutlined />}</div>
                <div className="rule-resource-info">
                  <div className="rule-resource-name">{item.name}</div>
                  <div className="rule-resource-meta">
                    <Tag color="blue">{actionTypeLabel(item.type)}</Tag>
                    {item.health === false && <Tag color="red">{t('不可用')}</Tag>}
                    {item.health === null && <Tag color="orange">{t('未验证')}</Tag>}
                  </div>
                </div>
                {disabledItem && <span className="rule-resource-selected">{t('已添加')}</span>}
              </div>
            )
          })}
        </div>
        <Space className="mt15">
          <Typography.Link href="/advanced/resource/manage" target="_blank">{t('去北向应用添加')}</Typography.Link>
        </Space>
      </Modal>
    </>
  )
}
