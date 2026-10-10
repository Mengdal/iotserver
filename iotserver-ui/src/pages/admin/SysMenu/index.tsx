import React, { useCallback, useEffect, useMemo, useState } from 'react'
import {
  Button,
  Card,
  Drawer,
  Form,
  Input,
  InputNumber,
  Popconfirm,
  Radio,
  Select,
  Space,
  Table,
  Tag,
  message,
} from 'antd'
import { PlusOutlined, ReloadOutlined, SearchOutlined, MinusSquareOutlined, PlusSquareOutlined } from '@ant-design/icons'
import type { ColumnsType } from 'antd/es/table'
import dayjs from 'dayjs'
import {
  getSysMenuTree,
  addSysMenu,
  updateSysMenu,
  deleteSysMenu,
  SysMenu,
} from '@/api'
import { useAuth } from '@/utils/auth'
import IconSelect, { renderMenuIcon } from '@/components/IconSelect'
import { useTreeTable } from '../utils/useTreeTable'
import type { FlatMeta } from '../utils/treeTable'

const { useMessage } = message

type FlatMenu = SysMenu & FlatMeta

const menuTypeOptions = [
  { label: '目录', value: 'M' },
  { label: '菜单', value: 'C' },
  { label: '按钮', value: 'F' },
]

const menuTypeTag: Record<string, { color: string, text: string }> = {
  M: { color: 'purple', text: '目录' },
  C: { color: 'blue', text: '菜单' },
  F: { color: 'default', text: '按钮' },
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
    const option: ParentOption = { value: item.id, title: item.name }
    if (item.children && item.children.length > 0) {
      const children = toParentOptions(item.children)
      if (children.length > 0) option.children = children
    }
    result.push(option)
  })
  return result
}

const SysMenuPage: React.FC = () => {
  const [messageApi, contextHolder] = useMessage()
  const [searchForm] = Form.useForm()
  const [editForm] = Form.useForm()
  const watchedMenuType: string | undefined = Form.useWatch('menuType', editForm)

  const [loading, setLoading] = useState(false)
  const [treeData, setTreeData] = useState<SysMenu[]>([])
  const [filters, setFilters] = useState<{ name?: string, status?: string }>({})
  const [modalOpen, setModalOpen] = useState(false)
  const [editing, setEditing] = useState<SysMenu | null>(null)

  const canAdd = useAuth('system:menu:add')
  const canEdit = useAuth('system:menu:edit')
  const canRemove = useAuth('system:menu:remove')

  const loadData = useCallback(async () => {
    try {
      setLoading(true)
      const resp = await getSysMenuTree({ isAll: true })
      if (resp.success) {
        setTreeData(resp.result || [])
      } else {
        messageApi.open({ type: 'error', content: resp.errorMsg })
      }
    } finally {
      setLoading(false)
    }
  }, [messageApi])

  useEffect(() => {
    loadData()
  }, [loadData])

  // 拍平 + 默认全部收起 + 整棵子树折叠
  const flatTable = useTreeTable<SysMenu>({
    tree: treeData,
    filtered: Boolean(filters.name || filters.status),
    filterKey: filters,
    getId: item => item.id,
    getParentId: item => item.parentId || '0',
    match: item => {
      const nameHit = !filters.name || item.name.includes(filters.name)
      const statusHit = !filters.status || String(item.status) === filters.status
      return nameHit && statusHit
    },
  })
  const rows = flatTable.rows as unknown as FlatMenu[]

  const parentOptions = useMemo(() => [{ value: '0', title: '顶级菜单' }, ...toParentOptions(treeData)], [treeData])

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
    const resp = editing
      ? await updateSysMenu({ ...values, id: editing.id })
      : await addSysMenu(values)
    if (resp.success) {
      messageApi.open({ type: 'success', content: editing ? '修改成功' : '新增成功' })
      setModalOpen(false)
      loadData()
    } else {
      messageApi.open({ type: 'error', content: resp.errorMsg })
    }
  }

  const handleDelete = async (id: string) => {
    const resp = await deleteSysMenu([id])
    if (resp.success) {
      messageApi.open({ type: 'success', content: '删除成功' })
      loadData()
    } else {
      messageApi.open({ type: 'error', content: resp.errorMsg })
    }
  }

  const columns: ColumnsType<FlatMenu> = [
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
            aria-label={flatTable.isExpanded(record.id) ? '折叠' : '展开'}
            onClick={() => flatTable.toggleExpand(record)}
          >
            {flatTable.isExpanded(record.id) ? <MinusSquareOutlined /> : <PlusSquareOutlined />}
          </Button>
        )
        : null),
    },
    {
      title: '菜单名称',
      dataIndex: 'name',
      width: 260,
      render: (value: string, record) => (
        <span style={{ paddingLeft: record.level * 24 }}>
          {value}
        </span>
      ),
    },
    {
      title: '按钮',
      key: 'isButton',
      width: 70,
      render: (_, record) => (record.menuType === 'F' ? <Tag>是</Tag> : <Tag color="blue">否</Tag>),
    },
    {
      title: '类型',
      dataIndex: 'menuType',
      width: 80,
      render: (value: string) => {
        const item = menuTypeTag[value] || { color: 'default', text: value }
        return <Tag color={item.color}>{item.text}</Tag>
      },
    },
    {
      title: '图标',
      dataIndex: 'icon',
      width: 140,
      render: (value: string) => (value
        ? (
          <Space size={6}>
            {renderMenuIcon(value)}
            <span>{value}</span>
          </Space>
        )
        : '-'),
    },
    { title: '路由地址', dataIndex: 'path', width: 200, render: (value: string) => value || '-' },
    { title: '权限标识', dataIndex: 'perms', width: 200, render: (value: string) => value || '-' },
    { title: '排序', dataIndex: 'sort', width: 80 },
    {
      title: '显示',
      dataIndex: 'visible',
      width: 80,
      render: (value: number) => (value === 1 ? '显示' : '隐藏'),
    },
    {
      title: '状态',
      dataIndex: 'status',
      width: 90,
      render: (value: number) => (value === 1 ? <Tag color="success">正常</Tag> : <Tag color="default">停用</Tag>),
    },
    {
      title: '创建时间',
      dataIndex: 'created',
      width: 180,
      render: (value: number) => (value ? dayjs(value).format('YYYY-MM-DD HH:mm:ss') : '-'),
    },
    {
      title: '操作',
      key: 'action',
      width: 200,
      fixed: 'right',
      render: (_, record) => (
        <Space size={4}>
          {canAdd && record.menuType !== 'F' && (
            <Button type="link" size="small" onClick={() => openAdd(record.id)}>新增下级</Button>
          )}
          {canEdit && <Button type="link" size="small" onClick={() => openEdit(record)}>编辑</Button>}
          {canRemove && (
            <Popconfirm title="确认删除该菜单？" onConfirm={() => handleDelete(record.id)} okText="确定" cancelText="取消">
              <Button type="link" size="small" danger>删除</Button>
            </Popconfirm>
          )}
        </Space>
      ),
    },
  ]

  return (
    <Card className="admin-page">
      {contextHolder}
      <Form
        form={searchForm}
        layout="inline"
        style={{ marginBottom: 16, rowGap: 8 }}
        onFinish={values => setFilters({ name: values.name, status: values.status })}
      >
        <Form.Item name="name" label="菜单名称">
          <Input placeholder="请输入菜单名称" allowClear style={{ width: 180 }} />
        </Form.Item>
        <Form.Item name="status" label="状态">
          <Select placeholder="全部" allowClear style={{ width: 120 }}>
            <Select.Option value="1">正常</Select.Option>
            <Select.Option value="0">停用</Select.Option>
          </Select>
        </Form.Item>
        <Form.Item>
          <Space>
            <Button type="primary" icon={<SearchOutlined />} htmlType="submit">查询</Button>
            <Button
              icon={<ReloadOutlined />}
              onClick={() => {
                searchForm.resetFields()
                setFilters({})
              }}
            >
              重置
            </Button>
          </Space>
        </Form.Item>
      </Form>

      <Space style={{ marginBottom: 12 }}>
        {canAdd && <Button type="primary" icon={<PlusOutlined />} onClick={() => openAdd()}>新增菜单</Button>}
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
        title={editing ? '编辑菜单' : '新增菜单'}
        placement="right"
        width={600}
        onClose={() => setModalOpen(false)}
        footer={
          <Space style={{ display: 'flex', justifyContent: 'flex-end' }}>
            <Button onClick={() => setModalOpen(false)}>取消</Button>
            <Button type="primary" onClick={handleSubmit}>确定</Button>
          </Space>
        }
      >
        <Form form={editForm} layout="vertical">
          <Form.Item label="上级菜单" name="parentId">
            <Select
              options={parentOptions}
              placeholder="请选择上级菜单"
              showSearch
              optionFilterProp="title"
            />
          </Form.Item>
          <Form.Item label="菜单类型" name="menuType" rules={[{ required: true, message: '请选择菜单类型' }]}>
            <Radio.Group options={menuTypeOptions} optionType="button" buttonStyle="solid" />
          </Form.Item>
          <Form.Item label="菜单名称" name="name" rules={[{ required: true, message: '请输入菜单名称' }]}>
            <Input placeholder="请输入菜单名称" />
          </Form.Item>
          {watchedMenuType !== 'F' && (
            <>
              <Form.Item label="图标" name="icon">
                <IconSelect placeholder="请选择图标" />
              </Form.Item>
              <Form.Item label="路由地址" name="path">
                <Input placeholder="例如 /admin/sysUser" />
              </Form.Item>
              <Form.Item label="组件路径" name="component">
                <Input placeholder="前端组件路径（可选）" />
              </Form.Item>
            </>
          )}
          <Form.Item label="权限标识" name="perms">
            <Input placeholder="例如 system:user:list" />
          </Form.Item>
          <Form.Item label="显示排序" name="sort">
            <InputNumber min={0} style={{ width: '100%' }} />
          </Form.Item>
          {watchedMenuType !== 'F' && (
            <Form.Item label="显示状态" name="visible">
              <Radio.Group>
                <Radio value={1}>显示</Radio>
                <Radio value={0}>隐藏</Radio>
              </Radio.Group>
            </Form.Item>
          )}
          <Form.Item label="菜单状态" name="status">
            <Radio.Group>
              <Radio value={1}>正常</Radio>
              <Radio value={0}>停用</Radio>
            </Radio.Group>
          </Form.Item>
          <Form.Item label="备注" name="remark">
            <Input.TextArea rows={2} placeholder="请输入备注" />
          </Form.Item>
        </Form>
      </Drawer>
    </Card>
  )
}

export default SysMenuPage
