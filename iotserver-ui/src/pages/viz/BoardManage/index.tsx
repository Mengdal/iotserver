import React, { useCallback, useEffect, useMemo, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import {
  Button, Card, Dropdown, Empty, Form, Input, Modal, Spin, Tag, Typography, message,
} from 'antd'
import type { MenuProps } from 'antd'
import {
  AppstoreOutlined, AreaChartOutlined, BarChartOutlined, BlockOutlined, DashboardOutlined,
  FundOutlined, HeatMapOutlined, LineChartOutlined, MoreOutlined, PieChartOutlined,
  PlusOutlined,
} from '@ant-design/icons'
import {
  createVizBoard, deleteVizBoard, getVizBoards, setVizBoardActive, VizBoard,
} from '@/api'
import './index.less'
import { t, tt } from '@/i18n'

const { Paragraph } = Typography

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

const iconOf = (key?: string) => {
  const hit = ICON_OPTIONS.find(i => i.key === key)
  return hit ? hit.icon : <AppstoreOutlined />
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
        message.error(resp.errorMsg || t('获取看板列表失败'))
      }
    } catch (e: any) {
      message.error(e?.message || t('获取看板列表失败'))
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
        message.error(resp.errorMsg || t('新建失败'))
      }
    } catch (e: any) {
      message.error(e?.message || t('新建失败'))
    } finally {
      setCreating(false)
    }
  }

  const remove = async (record: VizBoard) => {
    try {
      const resp = await deleteVizBoard(record.id)
      if (resp.success) {
        message.success(t('已删除'))
        load()
      } else {
        message.error(resp.errorMsg || t('删除失败'))
      }
    } catch (e: any) {
      // 没有 catch 的话，接口 4xx/5xx（axios 会 reject）会变成静默的 unhandled rejection，
      // 用户看到的就是"点了删除没反应"
      message.error(e?.message || t('删除失败'))
    }
  }

  /**
   * 二次确认用 Modal.confirm，不要用 Popconfirm 塞进 Dropdown 菜单项里：
   * 菜单项一点，Dropdown 就关闭并卸载菜单内容，跟着被卸载的确认弹层根本没机会显示，
   * 表现出来就是"点删除没反应"。
   */
  const confirmRemove = (record: VizBoard) => {
    Modal.confirm({
      title: t('删除看板「{{name}}」？', { name: tt(record.name) }),
      content: t('看板上的组件会一起删掉，删除后不能恢复。'),
      okText: t('删除'),
      okButtonProps: { danger: true },
      cancelText: t('取消'),
      onOk: () => remove(record),
    })
  }

  const setActive = async (record: VizBoard, slot: 'home' | 'default', value: boolean) => {
    try {
      const resp = await setVizBoardActive(record.id, slot, value)
      if (resp.success) {
        message.success(t('已更新'))
        load()
      } else {
        message.error(resp.errorMsg || t('设置失败'))
      }
    } catch (e: any) {
      message.error(e?.message || t('设置失败'))
    }
  }

  const menuFor = (record: VizBoard): MenuProps['items'] => {
    const items: NonNullable<MenuProps['items']> = []
    items.push({ key: 'canvas', label: t('编辑看板'), onClick: () => navigate(`/viz/boards/${record.id}/edit`) })
    // 只读大屏：不套后台外壳，可直接投到墙上/分享给只看数据的同事
    items.push({
      key: 'view',
      label: t('大屏展示（只读）'),
      onClick: () => window.open(`/viz/view/${record.id}`, '_blank'),
    })
    // 首页看板：首页（/home/dashboard）整页渲染它；没设时首页走内置静态页
    items.push({
      key: 'home',
      label: record.isHome ? t('取消首页看板') : t('设为首页看板'),
      onClick: () => setActive(record, 'home', !record.isHome),
    })
    items.push({
      key: 'default',
      label: record.isDefault ? t('取消数据看板整页') : t('设为数据看板整页'),
      onClick: () => setActive(record, 'default', !record.isDefault),
    })
    items.push({ type: 'divider' })
    items.push({
      key: 'delete',
      danger: true,
      label: t('删除'),
      onClick: () => confirmRemove(record),
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
        <span className="board-card__icon">{iconOf(record.icon)}</span>
        <span className="board-card__name">{tt(record.name)}</span>
        <span style={{ marginLeft: 'auto' }} onClick={e => e.stopPropagation()}>
          <Dropdown menu={{ items: menuFor(record) }} trigger={['click']}>
            <Button size="small" type="text" icon={<MoreOutlined />} />
          </Dropdown>
        </span>
      </div>
      <div className="board-card__meta">
        {t('画布看板 ·')} {record.widgetCount || 0} {t('个组件')}
        {record.updateAt ? ` · ${formatTime(record.updateAt)}` : ''}
      </div>
      <div className="board-card__tags">
        {record.isHome ? <Tag color="blue">{t('首页看板')}</Tag> : null}
        {record.isDefault ? <Tag color="green">{t('数据看板整页')}</Tag> : null}
      </div>
    </div>
  )), [list])

  return (
    <Card
      className="admin-page"
      title={t('看板管理')}
      extra={<Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>{t('新建看板')}</Button>}
    >
      {/* 整句一起翻译：中文可以逐段拼接，英文拼接会粘在一起（缺少空格/语序不同） */}
      <Paragraph type="secondary">
        {t('新建看板后进入编辑器：左侧「{{lib}}」里点一下或拖进画布来放{{text}}和{{container}}；组件可拖标题栏换位置、拖右下角改大小。卡片右上角菜单可把看板设为{{home}}（首页整页展示这块看板）或{{full}}（「数据看板」页展示）。', {
          lib: t('组件库'),
          text: t('文本'),
          container: t('容器'),
          home: t('首页看板'),
          full: t('数据看板整页'),
        })}
      </Paragraph>

      {loading
        ? <div style={{ padding: 48, textAlign: 'center' }}><Spin /></div>
        : (
          <div className="board-grid">
            <div className="board-card board-card--new" onClick={openCreate}>
              <PlusOutlined style={{ fontSize: 20 }} />
              <div>{t('新建看板')}</div>
            </div>
            {cards}
            {list.length === 0
              ? <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={t('还没有看板')} style={{ gridColumn: '1 / -1' }} />
              : null}
          </div>
        )}

      {/* 新建看板：名称 + 图标（参考站的项目创建弹窗） */}
      <Modal
        title={t('新建看板')}
        open={createOpen}
        confirmLoading={creating}
        okText={t('创建并搭建')}
        onOk={submitCreate}
        onCancel={() => setCreateOpen(false)}
        destroyOnClose
      >
        <Form form={form} layout="vertical" initialValues={{ icon: 'bar-chart' }}>
          <Form.Item name="name" label={t('看板名称')} rules={[{ required: true, message: t('请输入看板名称') }]}>
            <Input placeholder={t('例如：设备运行看板')} />
          </Form.Item>
          <Form.Item name="icon" label={t('图标')}>
            <IconPicker />
          </Form.Item>
        </Form>
      </Modal>

    </Card>
  )
}

export default BoardManage
