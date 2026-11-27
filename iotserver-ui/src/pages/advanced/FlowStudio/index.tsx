import React, { useState } from 'react'
import { Alert, Button, Card, Empty, Form, Input, Modal, Tag, message } from 'antd'
import {
  ApartmentOutlined,
  ClockCircleOutlined,
  DeleteOutlined,
  PlusOutlined,
  RocketOutlined,
} from '@ant-design/icons'
import { useNavigate } from 'react-router-dom'
import dayjs from 'dayjs'
import { t } from '@/i18n'
import type { FlowDoc } from './model/types'
import { deleteFlow, loadFlows, makeFlowId, saveFlows, seedFlows } from './model/storage'
import { flowSemanticHash } from './model/compile'
import { getNodeDef } from './model/nodes'
import './FlowStudio.less'

const FlowStudioList: React.FC = () => {
  const navigate = useNavigate()
  const [messageApi, contextHolder] = message.useMessage()
  const [flows, setFlows] = useState<FlowDoc[]>(() => loadFlows())
  const [createOpen, setCreateOpen] = useState(false)
  const [form] = Form.useForm()

  const refresh = () => setFlows(loadFlows())

  const createFlow = async () => {
    const values = await form.validateFields()
    const now = new Date().toISOString()
    const flow: FlowDoc = {
      id: makeFlowId(),
      name: String(values.name).trim(),
      description: '',
      spec: { nodes: [], edges: [], options: { concurrency: 1 } },
      layout: { nodes: {} },
      updatedAt: now,
    }
    saveFlows([flow, ...loadFlows()])
    setCreateOpen(false)
    form.resetFields()
    messageApi.success(t('创建成功'))
    navigate(`/advanced/rule/flow/${flow.id}`)
  }

  const removeFlow = (flow: FlowDoc) => {
    Modal.confirm({
      title: t('删除流程'),
      content: t('确认删除流程「{{name}}」？该操作只影响本地演示数据。', { name: flow.name }),
      okText: t('删除'),
      okButtonProps: { danger: true },
      cancelText: t('取消'),
      onOk: () => {
        deleteFlow(flow.id)
        refresh()
        messageApi.success(t('已删除'))
      },
    })
  }

  const resetDemo = () => {
    Modal.confirm({
      title: t('重置演示数据'),
      content: t('将恢复三条示例流程，当前本地修改会丢失。'),
      okText: t('重置'),
      cancelText: t('取消'),
      onOk: () => {
        saveFlows(seedFlows())
        refresh()
        messageApi.success(t('已重置'))
      },
    })
  }

  return (
    <div className="fs-list">
      {contextHolder}
      <header className="fs-list__header">
        <div>
          <div className="fs-list__title">
            <ApartmentOutlined /> {t('流程编排')}
            <Tag color="blue">{t('演示')}</Tag>
          </div>
          <div className="fs-list__desc">
            {t('用画布编排流处理流程，编译为 eKuiper 图规则。演示版数据保存在浏览器本地，不写入后端。')}
          </div>
        </div>
        <div className="fs-list__actions">
          <Button onClick={resetDemo}>{t('重置演示数据')}</Button>
          <Button type="primary" icon={<PlusOutlined />} onClick={() => setCreateOpen(true)}>{t('新建流程')}</Button>
        </div>
      </header>

      <Alert
        className="fs-list__notice"
        type="info"
        showIcon
        message={t('演示模式')}
        description={t('本页面不调用后端：草稿、部署状态与运行指标均为本地模拟；编译产物是可直接下发给 eKuiper 2.4.2 的 graph rule JSON。')}
      />

      {flows.length === 0 && (
        <Card className="fs-list__empty">
          <Empty description={t('暂无流程，先创建一个。')}>
            <Button type="primary" icon={<PlusOutlined />} onClick={() => setCreateOpen(true)}>{t('新建流程')}</Button>
          </Empty>
        </Card>
      )}

      <div className="fs-list__grid">
        {flows.map(flow => {
          const deployed = flow.deployment
          const dirty = deployed ? deployed.semanticHash !== flowSemanticHash(flow) : false
          const statusText = !deployed ? t('未部署') : dirty ? t('有未部署的更改') : t('已部署')
          const statusColor = !deployed ? 'default' : dirty ? 'orange' : 'green'
          const types = Array.from(new Set(flow.spec.nodes
            .map(node => getNodeDef(node.type, node.typeVersion)?.displayName)
            .filter(Boolean))) as string[]
          return (
            <Card
              key={flow.id}
              className="fs-flow-card"
              hoverable
              onClick={() => navigate(`/advanced/rule/flow/${flow.id}`)}
              actions={[
                <span key="open"><RocketOutlined /> {t('打开')}</span>,
                <span
                  key="delete"
                  className="fs-flow-card__delete"
                  onClick={event => {
                    event.stopPropagation()
                    removeFlow(flow)
                  }}
                >
                  <DeleteOutlined /> {t('删除')}
                </span>,
              ]}
            >
              <div className="fs-flow-card__head">
                <span className="fs-flow-card__icon"><ApartmentOutlined /></span>
                <div className="fs-flow-card__title-wrap">
                  <div className="fs-flow-card__title">{flow.name}</div>
                  <div className="fs-flow-card__desc">{flow.description || t('暂无描述')}</div>
                </div>
                <Tag color={statusColor}>{statusText}</Tag>
              </div>
              <div className="fs-flow-card__meta">
                <span>{t('{{count}} 个节点', { count: flow.spec.nodes.length })}</span>
                <span>{t('{{count}} 条连线', { count: flow.spec.edges.length })}</span>
                <span><ClockCircleOutlined /> {dayjs(flow.updatedAt).format('YYYY-MM-DD HH:mm')}</span>
              </div>
              <div className="fs-flow-card__types">
                {types.slice(0, 6).map(name => <Tag key={name}>{name}</Tag>)}
                {types.length > 6 && <Tag>+{types.length - 6}</Tag>}
              </div>
            </Card>
          )
        })}
      </div>

      <Modal
        title={t('新建流程')}
        open={createOpen}
        onCancel={() => setCreateOpen(false)}
        onOk={createFlow}
        okText={t('创建')}
        cancelText={t('取消')}
        destroyOnClose
      >
        <Form form={form} layout="vertical" preserve={false}>
          <Form.Item
            name="name"
            label={t('流程名称')}
            rules={[{ required: true, message: t('请输入流程名称') }]}
          >
            <Input placeholder={t('例如：高温告警联动')} maxLength={40} onPressEnter={createFlow} />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  )
}

export default FlowStudioList
