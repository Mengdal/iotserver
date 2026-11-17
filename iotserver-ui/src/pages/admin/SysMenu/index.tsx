import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react'
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
  Tag,
  TreeSelect,
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

const menuTypeOptions = [
  { label: t('目录'), value: 'M' },
  { label: t('菜单'), value: 'C' },
  { label: t('链接'), value: 'L' },
  { label: t('按钮'), value: 'F' },
]

const menuTypeTag: Record<string, { color: string, text: string }> = {
  M: { color: 'purple', text: t('目录') },
  C: { color: 'blue', text: t('菜单') },
  L: { color: 'cyan', text: t('链接') },
  F: { color: 'default', text: t('按钮') },
}

/** 菜单在管理页里的展示名：英文界面优先用菜单自带的英文名，其次词条翻译 */
function menuDisplayName (item: SysMenu, customName?: string): string {
  if (customName) return customName
  if (currentLocale() === 'en-US') return item.nameEn || tt(item.name)
  return item.name
}

interface ParentTreeNode {
  value: string
  title: string
  /** 保留为层级容器但当前类型不能选它（如按钮只能挂在菜单下时的目录节点）。 */
  disabled?: boolean
  children?: ParentTreeNode[]
}

function toParentTree (list: SysMenu[], allowed: string[]): ParentTreeNode[] {
  const result: ParentTreeNode[] = []
  list.forEach(item => {
    // 按钮、链接都是叶子，不能作为父级
    if (item.menuType === 'F' || item.menuType === 'L') return
    const children = item.children && item.children.length > 0
      ? toParentTree(item.children, allowed)
      : []
    if (!allowed.includes(item.menuType) && children.length === 0) return
    const node: ParentTreeNode = {
      value: item.id,
      title: menuDisplayName(item),
      disabled: !allowed.includes(item.menuType),
    }
    if (children.length > 0) node.children = children
    result.push(node)
  })
  return result
}

/** 同一父级下可排序的菜单（排除按钮），顺序沿用后端返回的 sort/created 顺序。 */
function siblingsOf (flat: SysMenu[], parentId: string): SysMenu[] {
  return flat.filter(item => (item.parentId || '0') === parentId && item.menuType !== 'F')
}

/** 新菜单默认追加到同级末尾：取最后一个同级菜单的 sort + 1。 */
function appendSortForParent (flat: SysMenu[], parentId: string): number {
  const siblings = siblingsOf(flat, parentId)
  return siblings.length === 0 ? 0 : siblings[siblings.length - 1].sort + 1
}

const SysMenuPage: React.FC = () => {
  const [messageApi, contextHolder] = useMessage()
  const [searchForm] = Form.useForm()
  const [editForm] = Form.useForm()
  const [renameForm] = Form.useForm()
  const watchedMenuType: string | undefined = Form.useWatch('menuType', editForm)
  const watchedIsFrame: number | undefined = Form.useWatch('isFrame', editForm)

  // 编辑范围在租户切换时会整页刷新，因此挂载时读取一次即可
  const [platformAdmin] = useState(() => isPlatformAdmin())
  const [tenantId] = useState(() => getCurrentTenantId())
  /** 租户视图：平台管理员已切换租户，或普通租户用户 */
  const tenantView = !platformAdmin || Boolean(tenantId)

  const [loading, setLoading] = useState(false)
  const [treeData, setTreeData] = useState<SysMenu[]>([])
  const [prefMap, setPrefMap] = useState<Record<string, TenantMenuPref>>({})
  const [filters, setFilters] = useState<{ name?: string, status?: string }>({})
  const [modalOpen, setModalOpen] = useState(false)
  const [editing, setEditing] = useState<SysMenu | null>(null)
  const prevMenuTypeRef = useRef<string | undefined>()
  const [renameTarget, setRenameTarget] = useState<SysMenu | null>(null)

  const canAdd = useAuth(MenuAdd)
  const canEdit = useAuth(MenuEdit)
  const canRemove = useAuth(MenuRemove)
  const canCustomPref = useAuth(MenuCustomPref)
  const canCustomReset = useAuth(MenuCustomReset)

  // 租户自建菜单不能设置权限码，按钮类型没有意义。
  const menuTypeChoices = useMemo(
    () => (tenantView ? menuTypeOptions.filter(option => option.value !== 'F') : menuTypeOptions),
    [tenantView],
  )

  const loadData = useCallback(async () => {
    try {
      setLoading(true)
      // 菜单树与展示定制互相独立，并发拉取；allSettled 避免可选的展示配置
      // 请求失败时把已经取回的菜单树一起丢掉。
      const [menuResult, prefResult] = await Promise.allSettled([
        getSysMenuTree({ isAll: true }),
        tenantView ? getSysMenuPrefs() : Promise.resolve(null),
      ])
      if (menuResult.status === 'fulfilled') {
        const menuResp = menuResult.value
        if (menuResp.success) {
          setTreeData(menuResp.result || [])
        } else {
          messageApi.open({ type: 'error', content: menuResp.errorMsg })
        }
      } else {
        messageApi.open({ type: 'error', content: t('加载失败') })
      }
      if (!tenantView) {
        setPrefMap({})
        return
      }
      if (prefResult.status === 'fulfilled') {
        const prefResp = prefResult.value
        if (!prefResp) {
          setPrefMap({})
        } else if (prefResp.success) {
          setPrefMap(Object.fromEntries((prefResp.result || []).map(item => [item.menuId, item])))
        } else {
          messageApi.open({ type: 'error', content: prefResp.errorMsg })
        }
      } else {
        // 展示定制失败不影响菜单树展示，只提示并保留已加载的树
        messageApi.open({ type: 'error', content: t('加载失败') })
      }
    } finally {
      setLoading(false)
    }
  }, [messageApi, tenantView])

  useEffect(() => {
    loadData()
  }, [loadData])

  // 整棵树只拍平一次，下面按 id / 父级查找都复用这份数据。
  const menuFlat = useMemo(() => collectNested<SysMenu>(treeData, item => item.id), [treeData])
  const menuById = useMemo(() => new Map(menuFlat.map(item => [item.id, item])), [menuFlat])

  // 平台视图只返回平台菜单；租户视图返回平台菜单 + 本租户自定义菜单，同树展示。
  const filterKey = useMemo(
    () => ({ ...filters, tenantView }),
    [filters, tenantView],
  )

  // 拍平 + 默认全部收起 + 整棵子树折叠
  const flatTable = useTreeTable<SysMenu>({
    tree: treeData,
    // 租户视图默认展开：自定义菜单可能挂在平台菜单下，收起时容易被误认为没生效。
    filtered: Boolean(filters.name || filters.status) || tenantView,
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

  const parentTreeData = useMemo(() => {
    // 目录只做顶级分组；菜单/链接可以挂在目录或菜单下；按钮只能挂在菜单下。
    if (watchedMenuType === 'M') return []
    const allowed = watchedMenuType === 'F' ? ['C'] : ['M', 'C']
    const nodes = toParentTree(treeData, allowed)
    // 按钮必须有归属菜单，不能是顶级。
    return watchedMenuType === 'F' ? nodes : [{ value: '0', title: t('顶级菜单') }, ...nodes]
  }, [treeData, watchedMenuType])

  // 只有平台管理员需要作用域提示；租户用户本来就只能编辑本租户内容。
  const scopeText = useMemo(() => {
    if (tenantId) return t('当前编辑：该租户的自定义菜单与显示定制 —— 仅该租户可见，不影响平台菜单')
    return t('当前编辑：平台原始菜单 —— 改动对所有租户生效')
  }, [tenantId])

  const openAdd = (parentId?: string) => {
    setEditing(null)
    prevMenuTypeRef.current = undefined
    editForm.resetFields()
    const targetParent = parentId || '0'
    // 在菜单下新增下级时：平台默认按钮，租户默认链接（租户不能给按钮配权限码）。
    const parentMenu = targetParent !== '0' ? menuById.get(targetParent) : undefined
    editForm.setFieldsValue({
      parentId: targetParent,
      menuType: parentMenu?.menuType === 'C' ? (tenantView ? 'L' : 'F') : 'C',
      // 默认追加到同级末尾，避免新菜单都挤在 sort=0。
      sort: appendSortForParent(menuFlat, targetParent),
      visible: 1,
      status: 1,
      isFrame: 1,
    })
    setModalOpen(true)
  }

  const openEdit = (record: SysMenu) => {
    setEditing(record)
    prevMenuTypeRef.current = undefined
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

  // 用户主动切换菜单类型时，把不再合法的上级菜单重置掉：
  // 目录→顶级；菜单→目录/顶级；链接→目录/菜单/顶级；按钮→必须挂在菜单下。
  useEffect(() => {
    const prev = prevMenuTypeRef.current
    prevMenuTypeRef.current = watchedMenuType
    if (!modalOpen || prev === undefined || prev === watchedMenuType) return
    const type = watchedMenuType || 'C'
    const current = (editForm.getFieldValue('parentId') as string | undefined) || '0'
    let next = current
    if (type === 'M') {
      next = '0'
    } else if (type === 'F') {
      if (current === '0' || menuById.get(current)?.menuType !== 'C') next = ''
    } else {
      // 菜单/链接可以挂在目录或菜单下；只有挂在按钮/链接（叶子）下时才重置。
      const currentType = current === '0' ? '' : menuById.get(current)?.menuType || ''
      if (current !== '0' && !['M', 'C'].includes(currentType)) next = '0'
    }
    if (next === current) return
    editForm.setFieldsValue({
      parentId: next,
      ...(editing ? {} : { sort: next ? appendSortForParent(menuFlat, next) : 0 }),
    })
  }, [watchedMenuType, modalOpen, menuById, menuFlat, editing, editForm])

  const handleSubmit = async () => {
    const values = await editForm.validateFields()
    const payload: Record<string, any> = { ...values }
    if (payload.menuType === 'L') {
      // 链接是纯导航项，不绑定组件与权限标识
      payload.component = ''
    }
    // 租户自建菜单不得携带权限标识（后端对非空 perms 直接报参数错误）
    if (tenantView || payload.menuType === 'L') {
      payload.perms = ''
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
            {tenantView && record.tenantId
              ? <Tag color="blue" style={{ marginLeft: 6 }}>{t('自定义')}</Tag>
              : null}
            {customName ? <Tag color="blue" style={{ marginLeft: 6 }}>{t('已改名')}</Tag> : null}
            {hidden ? <Tag style={{ marginLeft: 6 }}>{t('已隐藏')}</Tag> : null}
          </span>
        )
      },
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

  const actionColumn: ColumnsType<FlatMenu>[number] = {
    title: t('操作'),
    key: 'action',
    width: 300,
    fixed: 'right',
    render: (_, record) => {
      const addChild = canAdd && record.menuType !== 'F' && record.menuType !== 'L'
        ? <Button type="link" size="small" onClick={() => openAdd(record.id)}>{t('新增下级')}</Button>
        : null
      // 租户视图里的平台菜单：结构只读，只能做显示定制，以及在其下新增自定义菜单。
      if (tenantView && !record.tenantId) {
        return (
          <Space size={4} wrap={false} style={{ whiteSpace: 'nowrap' }}>
            {addChild}
            {canCustomPref
              ? (
                <>
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
                </>
              )
              : <span style={{ color: 'rgba(0, 0, 0, 0.45)' }}>{t('无定制权限')}</span>}
          </Space>
        )
      }
      // 平台视图的平台菜单，或租户视图里的自定义菜单：结构可编辑。
      return (
        <Space size={4} wrap={false} style={{ whiteSpace: 'nowrap' }}>
          {addChild}
          {canEdit && <Button type="link" size="small" onClick={() => openEdit(record)}>{t('编辑')}</Button>}
          {canRemove && (
            <Popconfirm title={t('确认删除该菜单？')} onConfirm={() => handleDelete(record.id)} okText={t('确定')} cancelText={t('取消')}>
              <Button type="link" size="small" danger>{t('删除')}</Button>
            </Popconfirm>
          )}
        </Space>
      )
    },
  }

  const columns: ColumnsType<FlatMenu> = [...baseColumns, actionColumn]

  return (
    <Card className="admin-page">
      {contextHolder}
      {platformAdmin && (
        <Alert
          type={tenantId ? 'warning' : 'info'}
          showIcon
          style={{ marginBottom: 12 }}
          message={scopeText}
        />
      )}

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
        {canAdd && (
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
          {watchedMenuType !== 'M' && (
            <Form.Item
              label={watchedMenuType === 'F' ? t('所属菜单') : t('上级菜单')}
              name="parentId"
              rules={watchedMenuType === 'F' ? [{ required: true, message: t('请选择所属菜单') }] : undefined}
            >
              <TreeSelect
                treeData={parentTreeData}
                placeholder={watchedMenuType === 'F' ? t('请选择所属菜单') : t('请选择上级菜单')}
                showSearch
                treeNodeFilterProp="title"
                treeDefaultExpandAll
                allowClear
                onChange={(value: string) => {
                  if (editing) return
                  // 换上级后自动排到新同级末尾；需要精确位置时再改下面的「显示排序」。
                  editForm.setFieldsValue({ sort: appendSortForParent(menuFlat, value || '0') })
                }}
              />
            </Form.Item>
          )}
          <Form.Item
            label={t('菜单类型')}
            name="menuType"
            rules={[{ required: true, message: t('请选择菜单类型') }]}
            extra={watchedMenuType === 'M'
              ? t('目录只作为分组，空目录不会显示在侧边栏；请在其下添加菜单或链接')
              : undefined}
          >
            <Radio.Group options={menuTypeChoices} optionType="button" buttonStyle="solid" />
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
              {watchedMenuType === 'L' && (
                <Form.Item
                  label={t('链接类型')}
                  name="isFrame"
                  extra={t('内链在当前页面内嵌入；外链在新窗口打开')}
                >
                  <Radio.Group>
                    <Radio value={1}>{t('内链')}</Radio>
                    <Radio value={0}>{t('外链')}</Radio>
                  </Radio.Group>
                </Form.Item>
              )}
              <Form.Item
                label={watchedMenuType === 'L' ? t('链接地址') : t('路由地址')}
                name="path"
                rules={watchedMenuType === 'L'
                  ? [
                    { required: true, message: t('请输入链接地址') },
                    {
                      validator: (_rule, value) => {
                        const target = String(value || '').trim()
                        if (!target) return Promise.resolve()
                        const isExternal = /^https?:\/\//i.test(target)
                        if (watchedIsFrame === 0) {
                          return isExternal
                            ? Promise.resolve()
                            : Promise.reject(new Error(t('外链必须以 http:// 或 https:// 开头')))
                        }
                        // 内链既可以是站内路径，也可以是外部 URL（用 iframe 嵌入）
                        return isExternal || target.startsWith('/')
                          ? Promise.resolve()
                          : Promise.reject(new Error(t('内链地址必须以 / 开头，或填写完整的 http://、https:// 地址')))
                      },
                    },
                  ]
                  : watchedMenuType === 'C'
                    ? [{ required: true, message: t('请输入路由地址') }]
                    : []}
              >
                <Input
                  placeholder={watchedMenuType === 'L'
                    ? (watchedIsFrame === 0 ? t('例如 https://example.com') : t('例如 /home/dashboard 或 https://example.com'))
                    : t('例如 /admin/sysUser')}
                />
              </Form.Item>
              {watchedMenuType !== 'L' && (
                <Form.Item label={t('组件路径')} name="component">
                  <Input placeholder={t('前端组件路径（可选）')} />
                </Form.Item>
              )}
            </>
          )}
          {watchedMenuType !== 'L' && (tenantView
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
            ))}
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
