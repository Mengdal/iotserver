import React from 'react'
import { Button, Card, Dropdown, MenuProps } from 'antd'
import { CopyOutlined, DeleteOutlined, MoreOutlined, SettingOutlined } from '@ant-design/icons'
import './index.less'

/**
 * 组件操作：统一走 actions + id。
 * 这样父组件可以用一个引用稳定的对象（useMemo）传下来，配合 React.memo
 * 让"拖动布局"时卡片不重渲染。
 */
export interface WidgetActions {
  duplicate?: (id: string) => void
  remove?: (id: string) => void
}

export interface WidgetCardProps {
  /** 组件 id（操作回调都按 id 派发） */
  id: string
  title?: string
  /** 组件类型：只用于挑样式（数据类组件去掉内边距，让图表铺满） */
  type?: string
  /** 编辑态：显示配置/复制/删除等入口 */
  editable?: boolean
  /** 只读态（查看模式 / 数据看板页）：不显示任何编辑入口 */
  readOnly?: boolean
  /** 操作集合（引用需稳定） */
  actions?: WidgetActions
  /** 打开配置面板（已登记类型的组件都传：标题编辑对所有组件可用） */
  onConfigure?: (id: string) => void
  className?: string
  style?: React.CSSProperties
  /** 组件内容：由注册表渲染（见 components/viz/WidgetBody） */
  children?: React.ReactNode
}

/**
 * 看板上的一个组件卡片：**只负责外框**（标题栏 + 操作菜单 + 内容容器）。
 *
 * 内容由组件注册表渲染（text / kpi / gauge / line / table …），
 * 这里不认识任何具体组件类型 —— 新增组件类型不需要改这个文件。
 * 容器组件的子网格外框由看板编辑器单独渲染，不走这里。
 */
const WidgetCard: React.FC<WidgetCardProps> = ({
  id,
  title,
  type,
  editable,
  readOnly,
  actions,
  onConfigure,
  className,
  style,
  children,
}) => {
  const menuItems: NonNullable<MenuProps['items']> = []
  if (editable && !readOnly) {
    if (onConfigure) {
      menuItems.push({
        key: 'configure',
        icon: <SettingOutlined />,
        label: '配置本组件',
        onClick: () => onConfigure && onConfigure(id),
      })
    }
    if (actions && actions.duplicate) {
      if (menuItems.length > 0) menuItems.push({ type: 'divider' })
      menuItems.push({
        key: 'duplicate',
        icon: <CopyOutlined />,
        label: '复制组件',
        onClick: () => actions.duplicate && actions.duplicate(id),
      })
    }
    if (actions && actions.remove) {
      if (menuItems.length > 0) menuItems.push({ type: 'divider' })
      menuItems.push({
        key: 'delete',
        icon: <DeleteOutlined />,
        label: '删除组件',
        danger: true,
        onClick: () => actions.remove && actions.remove(id),
      })
    }
  }

  const classes = [
    'widget-card',
    type === 'text' ? 'widget-card--text' : 'widget-card--data',
    className,
  ].filter(Boolean).join(' ')

  return (
    <Card
      size="small"
      className={classes}
      style={style}
      title={<span>{title || ''}</span>}
      extra={menuItems.length > 0
        ? (
          <Dropdown menu={{ items: menuItems }} trigger={['click']}>
            <Button size="small" type="text" icon={<MoreOutlined />} />
          </Dropdown>
        )
        : undefined}
    >
      {children}
    </Card>
  )
}

// 拖动布局时 props 不变 → 跳过重渲染
export default React.memo(WidgetCard)
