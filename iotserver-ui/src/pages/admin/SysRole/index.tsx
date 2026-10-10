import React, { useCallback, useEffect, useState } from 'react'
import {
  Button,
  Card,
  Drawer,
  Form,
  Input,
  InputNumber,
  Popconfirm,
  Select,
  Space,
  Table,
  Tag,
  Tree,
  message,
} from 'antd'
import { PlusOutlined, ReloadOutlined, SearchOutlined } from '@ant-design/icons'
import type { ColumnsType } from 'antd/es/table'
import dayjs from 'dayjs'
import {
  getSysRoles,
  addSysRole,
  updateSysRole,
  deleteSysRole,
  getSysMenuTree,
  getRoleMenus,
  setRoleMenus,
  SysRole,
  SysMenu,
} from '@/api'
import { useAuth } from '@/utils/auth'

const { useMessage } = message

interface MenuTreeNode {
  key: string
  title: string
  children?: MenuTreeNode[]
}

function toTreeData (list: SysMenu[]): MenuTreeNode[] {
  return list.map(item => {
    const node: MenuTreeNode = { key: item.id, title: item.menuType === 'F' ? `${item.name}（按钮）` : item.name }
    if (item.children && item.children.length > 0) {
      node.children = toTreeData(item.children)
    }
    return node
  })
}

function collectKeys (list: SysMenu[]): string[] {
  const keys: string[] = []
  const loop = (items: SysMenu[]) => {
    items.forEach(item => {
      keys.push(item.id)
      if (item.children) loop(item.children)
    })
  }
  loop(list)
  return keys
}

const dataScopeOptions = [
  { label: '全部数据', value: '1' },
  { label: '本部门数据', value: '2' },
  { label: '本部门及以下数据', value: '3' },
  { label: '仅本人数据', value: '4' },
]

const SysRolePage: React.FC = () => {
  const [messageApi, contextHolder] = useMessage()
  const [form] = Form.useForm()
  const [editForm] = Form.useForm()
  const watchedMenuIds: string[] | undefined = Form.useWatch('menuIds', editForm)

  const [loading, setLoading] = useState(false)
  const [dataSource, setDataSource] = useState<SysRole[]>([])
  const [pagination, setPagination] = useState({ current: 1, pageSize: 10 })
  const [total, setTotal] = useState(0)
  const [selectedRowKeys, setSelectedRowKeys] = useState<React.Key[]>([])

  const [modalOpen, setModalOpen] = useState(false)
  const [editing, setEditing] = useState<SysRole | null>(null)
  const [menuModalOpen, setMenuModalOpen] = useState(false)
  const [menuRole, setMenuRole] = useState<SysRole | null>(null)

  const [menuTreeData, setMenuTreeData] = useState<MenuTreeNode[]>([])
  const [menuTreeRaw, setMenuTreeRaw] = useState<SysMenu[]>([])
  const [checkedKeys, setCheckedKeys] = useState<string[]>([])
  const [expandedKeys, setExpandedKeys] = useState<string[]>([])
  const [halfCheckedKeys, setHalfCheckedKeys] = useState<string[]>([])

  const canAdd = useAuth('system:role:add')
  const canEdit = useAuth('system:role:edit')
  const canRemove = useAuth('system:role:remove')

  const loadData = useCallback(async (page = pagination.current, pageSize = pagination.pageSize) => {
    try {
      setLoading(true)
      const values = await form.validateFields()
      const resp = await getSysRoles({ ...values, page, pageSize })
      if (resp.success) {
        setDataSource(resp.result.list || [])
        setTotal(resp.result.total || 0)
      } else {
        messageApi.open({ type: 'error', content: resp.errorMsg })
      }
    } catch (error) {
      // ignore
    } finally {
      setLoading(false)
    }
  }, [form, pagination.current, pagination.pageSize])

  const loadMenuTree = useCallback(async () => {
    const resp = await getSysMenuTree({ isAll: true })
    if (resp.success) {
      setMenuTreeRaw(resp.result || [])
      setMenuTreeData(toTreeData(resp.result || []))
      setExpandedKeys(collectKeys(resp.result || []))
    }
  }, [])

  useEffect(() => {
    loadData(1, pagination.pageSize)
    loadMenuTree()
  }, [])

  const openAdd = () => {
    setEditing(null)
    editForm.resetFields()
    editForm.setFieldsValue({ status: 1, sort: 0, dataScope: '1', menuIds: [] })
    setModalOpen(true)
  }

  const openEdit = async (record: SysRole) => {
    setEditing(record)
    editForm.resetFields()
    editForm.setFieldsValue({
      name: record.name,
      roleKey: record.roleKey,
      sort: record.sort,
      dataScope: record.dataScope,
      status: record.status,
      remark: record.remark,
      menuIds: record.menuIds || [],
    })
    setModalOpen(true)
  }

  const handleSubmit = async () => {
    const values = await editForm.validateFields()
    const resp = editing
      ? await updateSysRole({ ...values, id: editing.id })
      : await addSysRole(values)
    if (resp.success) {
      messageApi.open({ type: 'success', content: editing ? '修改成功' : '新增成功' })
      setModalOpen(false)
      loadData()
    } else {
      messageApi.open({ type: 'error', content: resp.errorMsg })
    }
  }

  const handleDelete = async (ids: string[]) => {
    const resp = await deleteSysRole(ids)
    if (resp.success) {
      messageApi.open({ type: 'success', content: '删除成功' })
      setSelectedRowKeys([])
      loadData()
    } else {
      messageApi.open({ type: 'error', content: resp.errorMsg })
    }
  }

  const openMenus = async (record: SysRole) => {
    setMenuRole(record)
    setCheckedKeys([])
    setHalfCheckedKeys([])
    const resp = await getRoleMenus(record.id)
    if (resp.success) {
      const owned = resp.result.menuIds || []
      // 只保留叶子节点为选中，父节点由 Tree 自身推导，避免全选父节点
      const leafKeys = new Set<string>()
      const walk = (items: SysMenu[]) => {
        items.forEach(item => {
          if (item.children && item.children.length > 0) {
            walk(item.children)
          } else {
            leafKeys.add(item.id)
          }
        })
      }
      walk(menuTreeRaw)
      setCheckedKeys(owned.filter(key => leafKeys.has(key)))
    }
    setMenuModalOpen(true)
  }

  const handleSaveMenus = async () => {
    if (!menuRole) return
    const menuIds = Array.from(new Set([...checkedKeys, ...halfCheckedKeys]))
    const resp = await setRoleMenus(menuRole.id, menuIds)
    if (resp.success) {
      messageApi.open({ type: 'success', content: '授权成功' })
      setMenuModalOpen(false)
      loadData()
    } else {
      messageApi.open({ type: 'error', content: resp.errorMsg })
    }
  }

  const columns: ColumnsType<SysRole> = [
    { title: '角色名称', dataIndex: 'name', width: 180 },
    { title: '权限字符', dataIndex: 'roleKey', width: 160, render: (value: string) => <Tag color="blue">{value}</Tag> },
    { title: '显示顺序', dataIndex: 'sort', width: 100 },
    {
      title: '数据范围',
      dataIndex: 'dataScope',
      width: 160,
      render: (value: string) => dataScopeOptions.find(item => item.value === value)?.label || value || '-',
    },
    {
      title: '状态',
      dataIndex: 'status',
      width: 90,
      render: (value: number) => (value === 1 ? <Tag color="success">正常</Tag> : <Tag color="default">停用</Tag>),
    },
    { title: '备注', dataIndex: 'remark', ellipsis: true, render: (value: string) => value || '-' },
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
          {canEdit && <Button type="link" size="small" onClick={() => openEdit(record)}>编辑</Button>}
          {canEdit && <Button type="link" size="small" onClick={() => openMenus(record)}>菜单权限</Button>}
          {canRemove && record.roleKey !== 'admin' && (
            <Popconfirm title="确认删除该角色？" onConfirm={() => handleDelete([record.id])} okText="确定" cancelText="取消">
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
      <Form form={form} layout="inline" style={{ marginBottom: 16, rowGap: 8 }}>
        <Form.Item name="nameLike" label="角色名称">
          <Input placeholder="请输入角色名称" allowClear style={{ width: 180 }} />
        </Form.Item>
        <Form.Item name="roleKey" label="权限字符">
          <Input placeholder="请输入权限字符" allowClear style={{ width: 180 }} />
        </Form.Item>
        <Form.Item name="status" label="状态">
          <Select placeholder="全部" allowClear style={{ width: 120 }}>
            <Select.Option value="1">正常</Select.Option>
            <Select.Option value="0">停用</Select.Option>
          </Select>
        </Form.Item>
        <Form.Item>
          <Space>
            <Button
              type="primary"
              icon={<SearchOutlined />}
              onClick={() => {
                setPagination({ ...pagination, current: 1 })
                loadData(1, pagination.pageSize)
              }}
            >
              查询
            </Button>
            <Button
              icon={<ReloadOutlined />}
              onClick={() => {
                form.resetFields()
                setPagination({ ...pagination, current: 1 })
                loadData(1, pagination.pageSize)
              }}
            >
              重置
            </Button>
          </Space>
        </Form.Item>
      </Form>

      <Space style={{ marginBottom: 12 }}>
        {canAdd && <Button type="primary" icon={<PlusOutlined />} onClick={openAdd}>新增角色</Button>}
        {canRemove && (
          <Popconfirm
            title={`确认删除选中的 ${selectedRowKeys.length} 个角色？`}
            onConfirm={() => handleDelete(selectedRowKeys as string[])}
            okText="确定"
            cancelText="取消"
            disabled={selectedRowKeys.length === 0}
          >
            <Button danger disabled={selectedRowKeys.length === 0}>批量删除</Button>
          </Popconfirm>
        )}
      </Space>

      <Table<SysRole>
        rowKey="id"
        size="small"
        bordered
        loading={loading}
        columns={columns}
        dataSource={dataSource}
        scroll={{ x: 1200 }}
        rowSelection={{
          selectedRowKeys,
          onChange: keys => setSelectedRowKeys(keys),
          getCheckboxProps: record => ({ disabled: record.roleKey === 'admin' }),
        }}
        pagination={{
          size: 'small',
          showQuickJumper: true,
          showSizeChanger: true,
          showTotal: t => `共 ${t} 条`,
          total,
          current: pagination.current,
          pageSize: pagination.pageSize,
          onChange: (current, pageSize) => {
            setPagination({ current, pageSize })
            loadData(current, pageSize)
          },
        }}
      />

      <Drawer
        open={modalOpen}
        title={editing ? '编辑角色' : '新增角色'}
        placement="right"
        width={640}
        onClose={() => setModalOpen(false)}
        footer={
          <Space style={{ display: 'flex', justifyContent: 'flex-end' }}>
            <Button onClick={() => setModalOpen(false)}>取消</Button>
            <Button type="primary" onClick={handleSubmit}>确定</Button>
          </Space>
        }
      >
        <Form form={editForm} layout="vertical">
          <Form.Item label="角色名称" name="name" rules={[{ required: true, message: '请输入角色名称' }]}>
            <Input placeholder="请输入角色名称" />
          </Form.Item>
          <Form.Item label="权限字符" name="roleKey" rules={[{ required: true, message: '请输入权限字符' }]}>
            <Input placeholder="例如 operator" />
          </Form.Item>
          <Form.Item label="显示顺序" name="sort">
            <InputNumber min={0} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item label="数据范围" name="dataScope">
            <Select options={dataScopeOptions} />
          </Form.Item>
          <Form.Item label="状态" name="status">
            <Select>
              <Select.Option value={1}>正常</Select.Option>
              <Select.Option value={0}>停用</Select.Option>
            </Select>
          </Form.Item>
          <Form.Item label="菜单权限" name="menuIds">
            <Tree
              checkable
              selectable={false}
              treeData={menuTreeData}
              defaultExpandAll
              checkedKeys={watchedMenuIds || []}
              onCheck={(checked, info) => {
                const keys = Array.isArray(checked) ? checked : checked.checked
                // 半选中的父目录也要一起保存，否则父目录缺失时子菜单挂不到菜单树上
                editForm.setFieldsValue({
                  menuIds: Array.from(new Set([...keys, ...(info.halfCheckedKeys || [])])),
                })
              }}
            />
          </Form.Item>
          <Form.Item label="备注" name="remark">
            <Input.TextArea rows={2} placeholder="请输入备注" />
          </Form.Item>
        </Form>
      </Drawer>

      <Drawer
        open={menuModalOpen}
        title={`菜单权限${menuRole ? ` - ${menuRole.name}` : ''}`}
        placement="right"
        width={520}
        onClose={() => setMenuModalOpen(false)}
        footer={
          <Space style={{ display: 'flex', justifyContent: 'flex-end' }}>
            <Button onClick={() => setMenuModalOpen(false)}>取消</Button>
            <Button type="primary" onClick={handleSaveMenus}>保存</Button>
          </Space>
        }
      >
        <Tree
          checkable
          selectable={false}
          treeData={menuTreeData}
          checkedKeys={checkedKeys}
          expandedKeys={expandedKeys}
          onExpand={keys => setExpandedKeys(keys as string[])}
          onCheck={(checked, info) => {
            setCheckedKeys((Array.isArray(checked) ? checked : checked.checked) as string[])
            setHalfCheckedKeys((info.halfCheckedKeys || []) as string[])
          }}
        />
      </Drawer>
    </Card>
  )
}

export default SysRolePage
