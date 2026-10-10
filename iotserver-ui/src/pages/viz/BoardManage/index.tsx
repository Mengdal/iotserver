import React, { useCallback, useEffect, useMemo, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import {
  Button, Card, Dropdown, Empty, Form, Input, Modal, Popconfirm, Spin, Tag, Typography, message,
} from 'antd'
import type { MenuProps } from 'antd'
import {
  AppstoreOutlined, AreaChartOutlined, BarChartOutlined, BlockOutlined, DashboardOutlined,
  FundOutlined, HeatMapOutlined, LineChartOutlined, MoreOutlined, PieChartOutlined,
  PlusOutlined, ThunderboltOutlined,
} from '@ant-design/icons'
import { defaultSpec } from '@/components/WidgetConfig'
import {
  createVizBoard, deleteVizBoard, getVizBoards, setVizBoardActive, VizBoard,
} from '@/api'
import './index.less'

const { Paragraph, Text } = Typography

/** 新建看板时可选的图标（存图标名，列表里再映射回组件） */
const ICON_OPTIONS: { key: string, icon: React.ReactNode }[] = [
  { key: 'bar-chart', icon: <BarChartOutlined /> },
  { key: 'line-chart', icon: <LineChartOutlined /> },
  { key: 'area-chart', icon: <AreaChartOutlined /> },
  { key: 'pie-chart', icon: <PieChartOutlined /> },
  { key: 'dashboard', icon: <DashboardOutlined /> },
  { key: 'fund', icon: <FundOutlined /> },
  { key: 'heatmap', icon: <HeatMapOutlined /> },
  { key: 'block', icon: <BlockOutlined /> },
]

const iconOf = (key?: string, kind?: string) => {
  if (kind === 'shaper') return <ThunderboltOutlined />
  const hit = ICON_OPTIONS.find(i => i.key === key)
  return hit ? hit.icon : <AppstoreOutlined />
}

const KIND_LABEL: Record<string, string> = {
  canvas: '画布看板',
  shaper: 'Shaper 看板',
}

const formatTime = (ts?: number) => {
  if (!ts) return ''
  const d = new Date(ts)
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`
}

/** 图标选择器：受控（配合 Form.Item） */
const IconPicker: React.FC<{ value?: string, onChange?: (value: string) => void }> = ({ value, onChange }) => (
  <div className="board-icon-picker">
    {ICON_OPTIONS.map(item => (
      <div
        key={item.key}
        className={`board-icon-picker__item${value === item.key ? ' board-icon-picker__item--active' : ''}`}
        onClick={() => onChange && onChange(item.key)}
      >
        {item.icon}
      </div>
    ))}
  </div>
)

/**
 * 看板管理（参考 app.dash.builders/projects 的项目列表形态）：
 * 卡片网格展示所有看板；新建看板＝名称 + 图标 → 进入画布编辑器搭建；
 * 平台看板由平台渲染（图形化搭建），早期用 SQL 建的看板可一键「转为平台看板」。
 */
const BoardManage: React.FC = () => {
  const navigate = useNavigate()
  const [list, setList] = useState<VizBoard[]>([])
  const [loading, setLoading] = useState(false)
  const [createOpen, setCreateOpen] = useState(false)
  const [creating, setCreating] = useState(false)
  const [form] = Form.useForm<{ name: string, icon: string }>()

  const load = useCallback(async () => {
    setLoading(true)
    try {
      const resp = await getVizBoards()
      if (resp.success) {
        setList(resp.result.list || [])
      } else {
        message.error(resp.errorMsg || '获取看板列表失败')
      }
    } catch (e: any) {
      message.error(e?.message || '获取看板列表失败')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    load()
  }, [load])

  const openCreate = () => {
    form.setFieldsValue({ name: '', icon: 'bar-chart' })
    setCreateOpen(true)
  }

  const submitCreate = async () => {
    const values = await form.validateFields()
    setCreating(true)
    try {
      const resp = await createVizBoard({
        name: values.name.trim(),
        kind: 'canvas',
        icon: values.icon,
        canvas: { cols: 12, rowHeight: 40, widgets: [] },
      })
      if (resp.success) {
        setCreateOpen(false)
        const newId = resp.result && resp.result.id
        if (newId) {
          navigate(`/viz/boards/${newId}/edit`)
        } else {
          load()
        }
      } else {
        message.error(resp.errorMsg || '新建失败')
      }
    } catch (e: any) {
      message.error(e?.message || '新建失败')
    } finally {
      setCreating(false)
    }
  }

  /** 把早期用 SQL 建的看板转成平台看板（先铺一个组件，进去再改） */
  const convertToCanvas = async (record: VizBoard) => {
    const resp = await createVizBoard({
      name: record.name,
      kind: 'canvas',
      icon: record.icon,
      canvas: {
        cols: 12,
        rowHeight: 40,
        widgets: [{
          id: `w${Date.now().toString(36)}`,
          type: 'chart',
          title: record.name,
          showTitle: true,
          layout: { x: 0, y: 0, w: 12, h: 8 },
          spec: record.spec || defaultSpec(''),
        }],
      },
    })
    if (resp.success) {
      message.success('已转为平台看板')
      const newId = resp.result && resp.result.id
      if (newId) {
        navigate(`/viz/boards/${newId}/edit`)
      } else {
        load()
      }
    } else {
      message.error(resp.errorMsg || '转换失败')
    }
  }

  const remove = async (record: VizBoard) => {
    const resp = await deleteVizBoard(record.id)
    if (resp.success) {
      message.success('已删除')
      load()
    } else {
      message.error(resp.errorMsg || '删除失败')
    }
  }

  const setActive = async (record: VizBoard, slot: 'home' | 'default', value: boolean) => {
    const resp = await setVizBoardActive(record.id, slot, value)
    if (resp.success) {
      message.success('已更新')
      load()
    } else {
      message.error(resp.errorMsg || '设置失败')
    }
  }

  const menuFor = (record: VizBoard): MenuProps['items'] => {
    const items: NonNullable<MenuProps['items']> = []
    if (record.kind === 'canvas') {
      items.push({ key: 'canvas', label: '编辑看板', onClick: () => navigate(`/viz/boards/${record.id}/edit`) })
    } else {
      items.push({ key: 'convert', label: '转为平台看板', onClick: () => convertToCanvas(record) })
    }
    items.push({
      key: 'default',
      label: record.isDefault ? '取消数据看板整页' : '设为数据看板整页',
      onClick: () => setActive(record, 'default', !record.isDefault),
    })
    items.push({ type: 'divider' })
    items.push({
      key: 'delete',
      danger: true,
      label: (
        <Popconfirm title="确定删除这块看板？" onConfirm={() => remove(record)}>
          <span>删除</span>
        </Popconfirm>
      ),
    })
    return items
  }

  const cards = useMemo(() => list.map(record => (
    <div
      className="board-card"
      key={record.id}
      onClick={() => navigate(`/viz/boards/${record.id}/edit`)}
    >
      <div className="board-card__head">
        <span className="board-card__icon">{iconOf(record.icon, record.kind)}</span>
        <span className="board-card__name">{record.name}</span>
        <span style={{ marginLeft: 'auto' }} onClick={e => e.stopPropagation()}>
          <Dropdown menu={{ items: menuFor(record) }} trigger={['click']}>
            <Button size="small" type="text" icon={<MoreOutlined />} />
          </Dropdown>
        </span>
      </div>
      <div className="board-card__meta">
        {KIND_LABEL[record.kind] || record.kind}
        {record.kind === 'canvas' ? ` · ${record.widgetCount || 0} 个组件` : ' · SQL 驱动'}
        {record.updateAt ? ` · ${formatTime(record.updateAt)}` : ''}
      </div>
      <div className="board-card__tags">
        {record.isDefault ? <Tag color="green">数据看板整页</Tag> : null}
      </div>
    </div>
  )), [list])

  return (
    <Card
      className="admin-page"
      title="看板管理"
      extra={<Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>新建看板</Button>}
    >
      <Paragraph type="secondary">
        新建看板后进入编辑器：右上角<Text strong>「添加组件」</Text>加图表或文本，在组件配置里选<Text strong>数据表</Text>和
        <Text strong>图形</Text>即可出图；组件<Text strong>拖标题栏</Text>换位置、<Text strong>拖右下角</Text>改大小。
      </Paragraph>

      {loading
        ? <div style={{ padding: 48, textAlign: 'center' }}><Spin /></div>
        : (
          <div className="board-grid">
            <div className="board-card board-card--new" onClick={openCreate}>
              <PlusOutlined style={{ fontSize: 20 }} />
              <div>新建看板</div>
            </div>
            {cards}
            {list.length === 0
              ? <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="还没有看板" style={{ gridColumn: '1 / -1' }} />
              : null}
          </div>
        )}

      {/* 新建看板：名称 + 图标（参考站的项目创建弹窗） */}
      <Modal
        title="新建看板"
        open={createOpen}
        confirmLoading={creating}
        okText="创建并搭建"
        onOk={submitCreate}
        onCancel={() => setCreateOpen(false)}
        destroyOnClose
      >
        <Form form={form} layout="vertical" initialValues={{ icon: 'bar-chart' }}>
          <Form.Item name="name" label="看板名称" rules={[{ required: true, message: '请输入看板名称' }]}>
            <Input placeholder="例如：设备运行看板" />
          </Form.Item>
          <Form.Item name="icon" label="图标">
            <IconPicker />
          </Form.Item>
        </Form>
      </Modal>

    </Card>
  )
}

export default BoardManage
