import React, { useCallback, useEffect, useMemo, useState } from 'react'
import {
  Alert,
  Button,
  Card,
  Drawer,
  Form,
  Input,
  InputNumber,
  Modal,
  Popconfirm,
  Radio,
  Select,
  Space,
  Table,
  Tabs,
  Tag,
  message,
} from 'antd'
import { PlusOutlined, ReloadOutlined, SearchOutlined, MinusSquareOutlined, PlusSquareOutlined } from '@ant-design/icons'
import type { ColumnsType } from 'antd/es/table'
import dayjs from 'dayjs'
import {
  getSysMenuTree,
  getSysMenuPrefs,
  saveSysMenuPref,
  resetSysMenuTenant,
  addSysMenu,
  updateSysMenu,
  deleteSysMenu,
  SysMenu,
  TenantMenuPref,
} from '@/api'
import { getCurrentTenantId, isPlatformAdmin, useAuth } from '@/utils/auth'
import { MenuAdd, MenuEdit, MenuRemove, MenuCustomPref, MenuCustomReset } from '@/routes/permissions'
import IconSelect, { renderMenuIcon } from '@/components/IconSelect'
import { useTreeTable } from '../utils/useTreeTable'
import { collectNested } from '../utils/treeTable'
import type { FlatMeta } from '../utils/treeTable'
import { currentLocale, t, tt } from '@/i18n'

const { useMessage } = message

type FlatMenu = SysMenu & FlatMeta

type TabKey = 'tenant' | 'platform'

const menuTypeOptions = [
  { label: t('目录'), value: 'M' },
  { label: t('菜单'), value: 'C' },
  { label: t('按钮'), value: 'F' },
]

const menuTypeTag: Record<string, { color: string, text: string }> = {
  M: { color: 'purple', text: t('目录') },
  C: { color: 'blue', text: t('菜单') },
  F: { color: 'default', text: t('按钮') },
}

/** 菜单在管理页里的展示名：英文界面优先用菜单自带的英文名，其次词条翻译 */
function menuDisplayName (item: SysMenu, customName?: string): string {
  if (customName) return customName
  if (currentLocale() === 'en-US') return item.nameEn || tt(item.name)
  return item.name
}

interface ParentOption {
  value: string
  title: string
  children?: ParentOption[]
}

function toParentOptions (list: SysMenu[]): ParentOption[] {
  const result: ParentOption[] = []
  list.forEach(item => {
    if (item.menuType === 'F') return
    const option: ParentOption = { value: item.id, title: menuDisplayName(item) }
    if (item.children && item.children.length > 0) {
      const children = toParentOptions(item.children)
      if (children.length > 0) option.children = children
    }
    result.push(option)
  })
  return result
}

/**
 * 按条件从菜单树中筛出节点并重建父子层级：
 * 父节点被过滤掉时，该节点提升为顶级，避免整棵子树丢失。
 */
function pickMenuTree (tree: SysMenu[], keep: (item: SysMenu) => boolean): SysMenu[] {
  const flat = collectNested<SysMenu>(tree, item => item.id).filter(keep)
  const nodeMap = new Map<string, SysMenu>()
  flat.forEach(item => {
    nodeMap.set(item.id, { ...item, children: [] })
  })
  const roots: SysMenu[] = []
  nodeMap.forEach(item => {
    const parent = item.parentId && item.parentId !== '0' ? nodeMap.get(item.parentId) : undefined
    if (parent) {
      parent.children = [...(parent.children || []), item]
    } else {
      roots.push(item)
    }
  })
  return roots
}

const SysMenuPage: React.FC = () => {
  const [messageApi, contextHolder] = useMessage()
  const [searchForm] = Form.useForm()
  const [editForm] = Form.useForm()
  const [renameForm] = Form.useForm()
  const watchedMenuType: string | undefined = Form.useWatch('menuType', editForm)

  // 编辑范围在租户切换时会整页刷新，因此挂载时读取一次即可
  const [platformAdmin] = useState(() => isPlatformAdmin())
  const [tenantId] = useState(() => getCurrentTenantId())
  /** 租户视图：平台管理员已切换租户，或普通租户用户 */
  const tenantView = !platformAdmin || Boolean(tenantId)

  const [activeTab, setActiveTab] = useState<TabKey>(tenantView ? 'tenant' : 'platform')
  const [loading, setLoading] = useState(false)
  const [treeData, setTreeData] = useState<SysMenu[]>([])
  const [prefMap, setPrefMap] = useState<Record<string, TenantMenuPref>>({})
  const [filters, setFilters] = useState<{ name?: string, status?: string }>({})
  const [modalOpen, setModalOpen] = useState(false)
  const [editing, setEditing] = useState<SysMenu | null>(null)
  const [renameTarget, setRenameTarget] = useState<SysMenu | null>(null)

  const canAdd = useAuth(MenuAdd)
  const canEdit = useAuth(MenuEdit)
  const canRemove = useAuth(MenuRemove)
  const canCustomPref = useAuth(MenuCustomPref)
  const canCustomReset = useAuth(MenuCustomReset)

  const loadData = useCallback(async () => {
    try {
      setLoading(true)
      const menuResp = await getSysMenuTree({ isAll: true })
      if (menuResp.success) {
        setTreeData(menuResp.result || [])
      } else {
        messageApi.open({ type: 'error', content: menuResp.errorMsg })
      }
      if (!tenantView) {
        setPrefMap({})
        return
      }
      const prefResp = await getSysMenuPrefs()
      if (prefResp.success) {
        const list = prefResp.result || []
        const map: Record<string, TenantMenuPref> = {}
        list.forEach(item => {
          map[item.menuId] = item
        })
        setPrefMap(map)
      } else {
        messageApi.open({ type: 'error', content: prefResp.errorMsg })
      }
    } finally {
      setLoading(false)
    }
  }, [messageApi, tenantView])

  useEffect(() => {
    loadData()
  }, [loadData])

  // 平台视图：接口本身只返回平台菜单；租户视图：按归属拆成两个页签的数据
  const platformTree = useMemo(
    () => (tenantView ? pickMenuTree(treeData, item => !item.tenantId) : treeData),
    [treeData, tenantView],
  )
  const tenantTree = useMemo(
    () => (tenantView ? pickMenuTree(treeData, item => Boolean(item.tenantId)) : []),
    [treeData, tenantView],
  )

  /** 「平台菜单」页签在租户视图下为只读（只做隐藏/改名定制） */
  const readonlyTab = tenantView && activeTab === 'platform'
  const activeTree = activeTab === 'tenant' && tenantView ? tenantTree : platformTree

  const filterKey = useMemo(
    () => ({ ...filters, activeTab, tenantView }),
    [filters, activeTab, tenantView],
  )

  // 拍平 + 默认全部收起 + 整棵子树折叠
  const flatTable = useTreeTable<SysMenu>({
    tree: activeTree,
    filtered: Boolean(filters.name || filters.status),
    filterKey,
    getId: item => item.id,
    getParentId: item => item.parentId || '0',
    match: item => {
      // 租户视图下列表展示的是覆盖后的名称，按展示名称与平台名称都能搜到
      const displayName = menuDisplayName(item, prefMap[item.id]?.customName)
      const nameHit = !filters.name
        || displayName.includes(filters.name)
        || item.name.includes(filters.name) || tt(item.name).includes(filters.name)
      const statusHit = !filters.status || String(item.status) === filters.status
      return nameHit && statusHit
    },
  })
  const rows = flatTable.rows as unknown as FlatMenu[]

  const parentOptions = useMemo(() => [{ value: '0', title: t('顶级菜单') }, ...toParentOptions(treeData)], [treeData])

  const scopeText = useMemo(() => {
    if (!platformAdmin) return t('当前编辑：本租户的自定义菜单与显示定制 —— 仅本租户可见')
    if (tenantId) return t('当前编辑：该租户的自定义菜单与显示定制 —— 仅该租户可见，不影响平台菜单')
    return t('当前编辑：平台原始菜单 —— 改动对所有租户生效')
  }, [platformAdmin, tenantId])

  const tabItems = useMemo(() => {
    if (!tenantView) return [{ key: 'platform', label: t('平台菜单') }]
    return [
      { key: 'tenant', label: t('本租户自定义菜单') },
      { key: 'platform', label: t('平台菜单') },
    ]
  }, [tenantView])

  const openAdd = (parentId?: string) => {
    setEditing(null)
    editForm.resetFields()
    editForm.setFieldsValue({
      parentId: parentId || '0',
      menuType: 'C',
      sort: 0,
      visible: 1,
      status: 1,
      isFrame: 1,
    })
    setModalOpen(true)
  }

  const openEdit = (record: SysMenu) => {
    setEditing(record)
    editForm.resetFields()
    editForm.setFieldsValue({
      parentId: record.parentId === '0' ? '0' : record.parentId,
      name: record.name,
      // 英文界面下把词条翻译预填到「英文名称」，避免编辑内置菜单时看到的全是中文
      nameEn: record.nameEn || (currentLocale() === 'en-US' ? tt(record.name) : ''),
      path: record.path,
      component: record.component,
      icon: record.icon,
      menuType: record.menuType,
      perms: record.perms,
      sort: record.sort,
      visible: record.visible,
      status: record.status,
      isFrame: record.isFrame,
      remark: record.remark,
    })
    setModalOpen(true)
  }

  const handleSubmit = async () => {
    const values = await editForm.validateFields()
    const payload: Record<string, any> = { ...values }
    if (tenantView) {
      // 租户自建菜单不得携带权限标识（后端对非空 perms 直接报参数错误）
      delete payload.perms
    }
    const resp = editing
      ? await updateSysMenu({ ...payload, id: editing.id })
      : await addSysMenu(payload)
    if (resp.success) {
      messageApi.open({ type: 'success', content: editing ? t('修改成功') : t('新增成功') })
      setModalOpen(false)
      loadData()
    } else {
      messageApi.open({ type: 'error', content: resp.errorMsg })
    }
  }

  const handleDelete = async (id: string) => {
    const resp = await deleteSysMenu([id])
    if (resp.success) {
      messageApi.open({ type: 'success', content: t('删除成功') })
      loadData()
    } else {
      // 后端会给出带数量的拒绝原因（如「有 N 个租户在它下面挂了自建菜单」），原样展示
      messageApi.open({ type: 'error', content: resp.errorMsg })
    }
  }

  /** 写入当前租户的展示层覆盖（隐藏/改名/图标/排序），只影响本租户 */
  const applyPref = async (menuId: string, patch: Partial<TenantMenuPref>) => {
    const current = prefMap[menuId]
    const resp = await saveSysMenuPref({
      menuId,
      hidden: patch.hidden ?? current?.hidden ?? 0,
      customName: patch.customName ?? current?.customName ?? '',
      customIcon: patch.customIcon ?? current?.customIcon ?? '',
      sort: patch.sort ?? current?.sort ?? 0,
    })
    if (resp.success) {
      messageApi.open({ type: 'success', content: t('菜单定制已保存') })
      loadData()
      return true
    }
    messageApi.open({ type: 'error', content: resp.errorMsg })
    return false
  }

  const handleToggleHidden = (record: SysMenu) => {
    const hidden = prefMap[record.id]?.hidden === 1
    applyPref(record.id, { hidden: hidden ? 0 : 1 })
  }

  const openRename = (record: SysMenu) => {
    setRenameTarget(record)
    renameForm.resetFields()
    renameForm.setFieldsValue({ customName: prefMap[record.id]?.customName || '' })
  }

  const handleRenameSubmit = async () => {
    if (!renameTarget) return
    const values = await renameForm.validateFields()
    const ok = await applyPref(renameTarget.id, { customName: String(values.customName || '').trim() })
    if (ok) setRenameTarget(null)
  }

  const handleRestoreDefault = (record: SysMenu) => {
    applyPref(record.id, { hidden: 0, customName: '', customIcon: '', sort: 0 })
  }

  const handleTenantReset = async () => {
    const resp = await resetSysMenuTenant(tenantId)
    if (resp.success) {
      messageApi.open({ type: 'success', content: t('已重置该租户的菜单定制') })
      loadData()
    } else {
      messageApi.open({ type: 'error', content: resp.errorMsg })
    }
  }

  const baseColumns: ColumnsType<FlatMenu> = [
    {
      title: '',
      key: 'fold',
      width: 40,
      render: (_, record) => (record.hasChildren
        ? (
          <Button
            type="text"
            size="small"
            style={{ padding: 0, width: 20, height: 20 }}
            data-node={record.id}
            data-name={record.name}
            aria-label={flatTable.isExpanded(record.id) ? t('折叠') : t('展开')}
            onClick={() => flatTable.toggleExpand(record)}
          >
            {flatTable.isExpanded(record.id) ? <MinusSquareOutlined /> : <PlusSquareOutlined />}
          </Button>
        )
        : null),
    },
    {
      title: t('菜单名称'),
      dataIndex: 'name',
      width: 280,
      render: (_value: string, record) => {
        const pref = prefMap[record.id]
        const customName = pref?.customName || ''
        const hidden = pref?.hidden === 1
        return (
          <span
            style={{
              paddingLeft: record.level * 24,
              color: hidden ? 'rgba(0, 0, 0, 0.35)' : undefined,
            }}
          >
            {menuDisplayName(record, customName)}
            {customName ? <Tag color="blue" style={{ marginLeft: 6 }}>{t('已改名')}</Tag> : null}
            {hidden ? <Tag style={{ marginLeft: 6 }}>{t('已隐藏')}</Tag> : null}
          </span>
        )
      },
    },
    {
      title: t('按钮'),
      key: 'isButton',
      width: 70,
      render: (_, record) => (record.menuType === 'F' ? <Tag>{t('是')}</Tag> : <Tag color="blue">{t('否')}</Tag>),
    },
    {
      title: t('类型'),
      dataIndex: 'menuType',
      width: 80,
      render: (value: string) => {
        const item = menuTypeTag[value] || { color: 'default', text: value }
        return <Tag color={item.color}>{item.text}</Tag>
      },
    },
    {
      title: t('图标'),
      dataIndex: 'icon',
      width: 140,
      render: (value: string, record) => {
        const icon = prefMap[record.id]?.customIcon || value
        return (icon
          ? (
            <Space size={6}>
              {renderMenuIcon(icon)}
              <span>{icon}</span>
            </Space>
          )
          : '-')
      },
    },
    { title: t('路由地址'), dataIndex: 'path', width: 200, render: (value: string) => value || '-' },
    { title: t('权限标识'), dataIndex: 'perms', width: 200, render: (value: string) => value || '-' },
    { title: t('排序'), dataIndex: 'sort', width: 80 },
    {
      title: t('显示'),
      dataIndex: 'visible',
      width: 80,
      render: (value: number) => (value === 1 ? t('显示') : t('隐藏')),
    },
    {
      title: t('状态'),
      dataIndex: 'status',
      width: 90,
      render: (value: number) => (value === 1 ? <Tag color="success">{t('正常')}</Tag> : <Tag color="default">{t('停用')}</Tag>),
    },
    {
      title: t('创建时间'),
      dataIndex: 'created',
      width: 180,
      render: (value: number) => (value ? dayjs(value).format('YYYY-MM-DD HH:mm:ss') : '-'),
    },
  ]

  const actionColumn: ColumnsType<FlatMenu>[number] = readonlyTab
    ? {
      title: t('操作（仅本租户显示定制）'),
      key: 'action',
      width: 260,
      fixed: 'right',
      render: (_, record) => (canCustomPref
        ? (
          <Space size={4} wrap={false} style={{ whiteSpace: 'nowrap' }}>
            <Button type="link" size="small" onClick={() => handleToggleHidden(record)}>
              {prefMap[record.id]?.hidden === 1 ? t('显示') : t('隐藏')}
            </Button>
            <Button type="link" size="small" onClick={() => openRename(record)}>{t('改名')}</Button>
            <Button
              type="link"
              size="small"
              disabled={!prefMap[record.id]}
              onClick={() => handleRestoreDefault(record)}
            >
              {t('恢复默认')}
            </Button>
          </Space>
        )
        : <span style={{ color: 'rgba(0, 0, 0, 0.45)' }}>{t('无定制权限')}</span>),
    }
    : {
      title: t('操作'),
      key: 'action',
      width: 240,
      fixed: 'right',
      render: (_, record) => (
        <Space size={4} wrap={false} style={{ whiteSpace: 'nowrap' }}>
          {canAdd && record.menuType !== 'F' && (
            <Button type="link" size="small" onClick={() => openAdd(record.id)}>{t('新增下级')}</Button>
          )}
          {canEdit && <Button type="link" size="small" onClick={() => openEdit(record)}>{t('编辑')}</Button>}
          {canRemove && (
            <Popconfirm title={t('确认删除该菜单？')} onConfirm={() => handleDelete(record.id)} okText={t('确定')} cancelText={t('取消')}>
              <Button type="link" size="small" danger>{t('删除')}</Button>
            </Popconfirm>
          )}
        </Space>
      ),
    }

  const columns: ColumnsType<FlatMenu> = [...baseColumns, actionColumn]

  return (
    <Card className="admin-page">
      {contextHolder}
      <Alert
        type={tenantView ? 'warning' : 'info'}
        showIcon
        style={{ marginBottom: 12 }}
        message={scopeText}
      />

      <Tabs
        activeKey={activeTab}
        onChange={key => setActiveTab(key as TabKey)}
        items={tabItems}
        style={{ marginBottom: 8 }}
      />

      <Form
        form={searchForm}
        layout="inline"
        style={{ marginBottom: 16, rowGap: 8 }}
        onFinish={values => setFilters({ name: values.name, status: values.status })}
      >
        <Form.Item name="name" label={t('菜单名称')}>
          <Input placeholder={t('请输入菜单名称')} allowClear style={{ width: 180 }} />
        </Form.Item>
        <Form.Item name="status" label={t('状态')}>
          <Select placeholder={t('全部')} allowClear style={{ width: 120 }}>
            <Select.Option value="1">{t('正常')}</Select.Option>
            <Select.Option value="0">{t('停用')}</Select.Option>
          </Select>
        </Form.Item>
        <Form.Item>
          <Space>
            <Button type="primary" icon={<SearchOutlined />} htmlType="submit">{t('查询')}</Button>
            <Button
              icon={<ReloadOutlined />}
              onClick={() => {
                searchForm.resetFields()
                setFilters({})
              }}
            >
              {t('重置')}
            </Button>
          </Space>
        </Form.Item>
      </Form>

      <Space style={{ marginBottom: 12 }}>
        {!readonlyTab && canAdd && (
          <Button type="primary" icon={<PlusOutlined />} onClick={() => openAdd()}>{t('新增菜单')}</Button>
        )}
        <Button icon={<ReloadOutlined />} onClick={loadData}>{t('刷新')}</Button>
        {platformAdmin && Boolean(tenantId) && canCustomReset && (
          <Popconfirm
            title={t('将清空该租户的自建菜单与全部显示定制，恢复平台默认，是否继续？')}
            onConfirm={handleTenantReset}
            okText={t('确定')}
            cancelText={t('取消')}
          >
            <Button danger>{t('重置该租户的菜单定制')}</Button>
          </Popconfirm>
        )}
      </Space>

      <Table<FlatMenu>
        rowKey="id"
        size="small"
        bordered
        loading={loading}
        columns={columns}
        dataSource={rows}
        rowClassName={record => `tree-row tree-level-${record.level}`}
        onRow={record => ({ 'data-parent': record.parentKey, 'data-key': record.id } as React.HTMLAttributes<HTMLElement>)}
        pagination={false}
        scroll={{ x: 1520 }}
      />

      <Drawer
        open={modalOpen}
        title={editing ? t('编辑菜单') : t('新增菜单')}
        placement="right"
        width={600}
        onClose={() => setModalOpen(false)}
        footer={
          <Space style={{ display: 'flex', justifyContent: 'flex-end' }}>
            <Button onClick={() => setModalOpen(false)}>{t('取消')}</Button>
            <Button type="primary" onClick={handleSubmit}>{t('确定')}</Button>
          </Space>
        }
      >
        <Form form={editForm} layout="vertical">
          <Form.Item label={t('上级菜单')} name="parentId">
            <Select
              options={parentOptions}
              placeholder={t('请选择上级菜单')}
              showSearch
              optionFilterProp="title"
            />
          </Form.Item>
          <Form.Item label={t('菜单类型')} name="menuType" rules={[{ required: true, message: t('请选择菜单类型') }]}>
            <Radio.Group options={menuTypeOptions} optionType="button" buttonStyle="solid" />
          </Form.Item>
          <Form.Item label={t('菜单名称')} name="name" rules={[{ required: true, message: t('请输入菜单名称') }]}>
            <Input placeholder={t('请输入菜单名称')} />
          </Form.Item>
          <Form.Item label={t('英文名称')} name="nameEn">
            <Input placeholder={t('可选；英文界面优先显示，留空则使用系统词条翻译')} maxLength={64} />
          </Form.Item>
          {watchedMenuType !== 'F' && (
            <>
              <Form.Item label={t('图标')} name="icon">
                <IconSelect placeholder={t('请选择图标')} />
              </Form.Item>
              <Form.Item label={t('路由地址')} name="path">
                <Input placeholder={t('例如 /admin/sysUser')} />
              </Form.Item>
              <Form.Item label={t('组件路径')} name="component">
                <Input placeholder={t('前端组件路径（可选）')} />
              </Form.Item>
            </>
          )}
          {tenantView
            ? (
              <Form.Item label={t('权限标识')}>
                <span style={{ color: 'rgba(0, 0, 0, 0.45)' }}>
                  {t('租户自建菜单只作为导航入口，不携带权限标识（权限由平台菜单与角色决定）')}
                </span>
              </Form.Item>
            )
            : (
              <Form.Item label={t('权限标识')} name="perms">
                <Input placeholder={t('例如 system:user:list')} />
              </Form.Item>
            )}
          <Form.Item label={t('显示排序')} name="sort">
            <InputNumber min={0} style={{ width: '100%' }} />
          </Form.Item>
          {watchedMenuType !== 'F' && (
            <Form.Item label={t('显示状态')} name="visible">
              <Radio.Group>
                <Radio value={1}>{t('显示')}</Radio>
                <Radio value={0}>{t('隐藏')}</Radio>
              </Radio.Group>
            </Form.Item>
          )}
          <Form.Item label={t('菜单状态')} name="status">
            <Radio.Group>
              <Radio value={1}>{t('正常')}</Radio>
              <Radio value={0}>{t('停用')}</Radio>
            </Radio.Group>
          </Form.Item>
          <Form.Item label={t('备注')} name="remark">
            <Input.TextArea rows={2} placeholder={t('请输入备注')} />
          </Form.Item>
        </Form>
      </Drawer>

      <Modal
        open={Boolean(renameTarget)}
        title={t('修改本租户显示名称')}
        okText={t('确定')}
        cancelText={t('取消')}
        onCancel={() => setRenameTarget(null)}
        onOk={handleRenameSubmit}
      >
        <Form form={renameForm} layout="vertical">
          <Form.Item
            label={t('租户自定义名称')}
            name="customName"
            extra={t('留空表示恢复平台菜单名称；只影响本租户的显示，不修改平台菜单本体。')}
          >
            <Input placeholder={t('平台名称：{{p1}}', { p1: renameTarget?.name || '' })} allowClear maxLength={64} />
          </Form.Item>
        </Form>
      </Modal>
    </Card>
  )
}

export default SysMenuPage
