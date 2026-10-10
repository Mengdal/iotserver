import React, { useCallback, useEffect, useMemo, useState } from 'react'
import {
  Button,
  Card,
  Col,
  Drawer,
  Form,
  Input,
  Popconfirm,
  Row,
  Select,
  Space,
  Switch,
  Table,
  Tag,
  TreeSelect,
  message,
} from 'antd'
import { PlusOutlined, ReloadOutlined, SearchOutlined } from '@ant-design/icons'
import type { ColumnsType } from 'antd/es/table'
import dayjs from 'dayjs'
import {
  getSysUsers,
  addSysUser,
  updateSysUser,
  deleteSysUser,
  resetSysUserPassword,
  changeSysUserStatus,
  getAllSysRoles,
  getSysDeptTree,
  SysUser,
  SysRole,
  SysDept,
} from '@/api'
import { useAuth } from '@/utils/auth'

const { useMessage } = message

interface DeptTreeNode {
  value: string
  title: string
  children?: DeptTreeNode[]
}

function toDeptTreeData (list: SysDept[]): DeptTreeNode[] {
  return list.map(item => {
    const node: DeptTreeNode = { value: item.id, title: item.name }
    if (item.children && item.children.length > 0) {
      node.children = toDeptTreeData(item.children)
    }
    return node
  })
}

const SysUserPage: React.FC = () => {
  const [messageApi, contextHolder] = useMessage()
  const [form] = Form.useForm()
  const [editForm] = Form.useForm()
  const [pwdForm] = Form.useForm()

  const [loading, setLoading] = useState(false)
  const [dataSource, setDataSource] = useState<SysUser[]>([])
  const [pagination, setPagination] = useState({ current: 1, pageSize: 10 })
  const [total, setTotal] = useState(0)

  const [roles, setRoles] = useState<SysRole[]>([])
  const [deptTree, setDeptTree] = useState<DeptTreeNode[]>([])
  const [deptTreeData, setDeptTreeData] = useState<SysDept[]>([])

  const [modalOpen, setModalOpen] = useState(false)
  const [editing, setEditing] = useState<SysUser | null>(null)
  const [pwdOpen, setPwdOpen] = useState(false)
  const [pwdUser, setPwdUser] = useState<SysUser | null>(null)
  const [selectedRowKeys, setSelectedRowKeys] = useState<React.Key[]>([])

  const canAdd = useAuth('system:user:add')
  const canEdit = useAuth('system:user:edit')
  const canRemove = useAuth('system:user:remove')

  const loadData = useCallback(async (page = pagination.current, pageSize = pagination.pageSize) => {
    try {
      setLoading(true)
      const values = await form.validateFields()
      const resp = await getSysUsers({ ...values, page, pageSize })
      if (resp.success) {
        setDataSource(resp.result.list || [])
        setTotal(resp.result.total || 0)
      } else {
        messageApi.open({ type: 'error', content: resp.errorMsg })
      }
    } catch (error) {
      // 表单校验失败忽略
    } finally {
      setLoading(false)
    }
  }, [form, pagination.current, pagination.pageSize])

  const loadOptions = useCallback(async () => {
    const [roleResp, deptResp] = await Promise.all([getAllSysRoles(), getSysDeptTree({ isAll: true })])
    if (roleResp.success) setRoles(roleResp.result || [])
    if (deptResp.success) {
      setDeptTreeData(deptResp.result || [])
      setDeptTree(toDeptTreeData(deptResp.result || []))
    }
  }, [])

  useEffect(() => {
    loadData(1, pagination.pageSize)
    loadOptions()
  }, [])

  const handleSearch = () => {
    setPagination({ ...pagination, current: 1 })
    loadData(1, pagination.pageSize)
  }

  const handleReset = () => {
    form.resetFields()
    setPagination({ ...pagination, current: 1 })
    loadData(1, pagination.pageSize)
  }

  const openAdd = () => {
    setEditing(null)
    editForm.resetFields()
    editForm.setFieldsValue({ status: 1, roleIds: [] })
    setModalOpen(true)
  }

  const openEdit = async (record: SysUser) => {
    setEditing(record)
    editForm.resetFields()
    editForm.setFieldsValue({
      nickname: record.nickname,
      email: record.email,
      phone: record.phone,
      deptId: record.deptId || undefined,
      status: record.status,
      remark: record.remark,
      roleIds: record.roleIds || [],
    })
    setModalOpen(true)
  }

  const handleSubmit = async () => {
    const values = await editForm.validateFields()
    const resp = editing
      ? await updateSysUser({ ...values, id: editing.id })
      : await addSysUser(values)
    if (resp.success) {
      messageApi.open({ type: 'success', content: editing ? '修改成功' : '新增成功' })
      setModalOpen(false)
      loadData()
    } else {
      messageApi.open({ type: 'error', content: resp.errorMsg })
    }
  }

  const handleDelete = async (ids: string[]) => {
    const resp = await deleteSysUser(ids)
    if (resp.success) {
      messageApi.open({ type: 'success', content: '删除成功' })
      setSelectedRowKeys([])
      loadData()
    } else {
      messageApi.open({ type: 'error', content: resp.errorMsg })
    }
  }

  const handleResetPwd = async () => {
    const values = await pwdForm.validateFields()
    if (!pwdUser) return
    const resp = await resetSysUserPassword(pwdUser.id, values.password)
    if (resp.success) {
      messageApi.open({ type: 'success', content: '密码重置成功' })
      setPwdOpen(false)
      pwdForm.resetFields()
    } else {
      messageApi.open({ type: 'error', content: resp.errorMsg })
    }
  }

  const handleStatusChange = async (record: SysUser, checked: boolean) => {
    const resp = await changeSysUserStatus(record.id, checked ? 1 : 0)
    if (resp.success) {
      messageApi.open({ type: 'success', content: checked ? '已启用' : '已停用' })
      loadData()
    } else {
      messageApi.open({ type: 'error', content: resp.errorMsg })
    }
  }

  const deptNameMap = useMemo(() => {
    const map: Record<string, string> = {}
    const loop = (list: SysDept[]) => {
      list.forEach(item => {
        map[item.id] = item.name
        if (item.children) loop(item.children)
      })
    }
    loop(deptTreeData)
    return map
  }, [deptTreeData])

  const columns: ColumnsType<SysUser> = [
    { title: '账号', dataIndex: 'username', width: 140 },
    { title: '昵称', dataIndex: 'nickname', width: 140 },
    {
      title: '部门',
      dataIndex: 'deptName',
      width: 140,
      render: (value: string, record) => value || deptNameMap[record.deptId] || '-',
    },
    { title: '手机号', dataIndex: 'phone', width: 140, render: (value: string) => value || '-' },
    { title: '邮箱', dataIndex: 'email', width: 180, render: (value: string) => value || '-' },
    {
      title: '角色',
      dataIndex: 'roleNames',
      width: 200,
      render: (value: string[]) => (
        <Space size={[4, 4]} wrap>
          {(value || []).length === 0 ? '-' : value.map(name => <Tag key={name} color="blue">{name}</Tag>)}
        </Space>
      ),
    },
    {
      title: '状态',
      dataIndex: 'status',
      width: 90,
      render: (value: number, record) => (
        <Switch
          checked={value === 1}
          disabled={!canEdit || record.username === 'admin'}
          checkedChildren="启用"
          unCheckedChildren="停用"
          onChange={checked => handleStatusChange(record, checked)}
        />
      ),
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
      width: 220,
      fixed: 'right',
      render: (_, record) => (
        <Space size={4}>
          {canEdit && <Button type="link" size="small" onClick={() => openEdit(record)}>编辑</Button>}
          {canEdit && (
            <Button
              type="link"
              size="small"
              onClick={() => {
                setPwdUser(record)
                pwdForm.resetFields()
                setPwdOpen(true)
              }}
            >
              重置密码
            </Button>
          )}
          {canRemove && record.username !== 'admin' && (
            <Popconfirm title="确认删除该用户？" onConfirm={() => handleDelete([record.id])} okText="确定" cancelText="取消">
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
        <Form.Item name="username" label="账号">
          <Input placeholder="请输入账号" allowClear style={{ width: 160 }} />
        </Form.Item>
        <Form.Item name="nickname" label="昵称">
          <Input placeholder="请输入昵称" allowClear style={{ width: 160 }} />
        </Form.Item>
        <Form.Item name="phone" label="手机号">
          <Input placeholder="请输入手机号" allowClear style={{ width: 160 }} />
        </Form.Item>
        <Form.Item name="status" label="状态">
          <Select placeholder="全部" allowClear style={{ width: 120 }}>
            <Select.Option value="1">正常</Select.Option>
            <Select.Option value="0">停用</Select.Option>
          </Select>
        </Form.Item>
        <Form.Item name="deptId" label="部门">
          <TreeSelect
            placeholder="请选择部门"
            allowClear
            style={{ width: 180 }}
            treeData={deptTree}
            treeDefaultExpandAll
          />
        </Form.Item>
        <Form.Item>
          <Space>
            <Button type="primary" icon={<SearchOutlined />} onClick={handleSearch}>查询</Button>
            <Button icon={<ReloadOutlined />} onClick={handleReset}>重置</Button>
          </Space>
        </Form.Item>
      </Form>

      <Row justify="space-between" style={{ marginBottom: 12 }}>
        <Col>
          <Space>
            {canAdd && <Button type="primary" icon={<PlusOutlined />} onClick={openAdd}>新增用户</Button>}
            {canRemove && (
              <Popconfirm
                title={`确认删除选中的 ${selectedRowKeys.length} 个用户？`}
                onConfirm={() => handleDelete(selectedRowKeys as string[])}
                okText="确定"
                cancelText="取消"
                disabled={selectedRowKeys.length === 0}
              >
                <Button danger disabled={selectedRowKeys.length === 0}>批量删除</Button>
              </Popconfirm>
            )}
          </Space>
        </Col>
      </Row>

      <Table<SysUser>
        rowKey="id"
        size="small"
        bordered
        loading={loading}
        columns={columns}
        dataSource={dataSource}
        scroll={{ x: 1280 }}
        rowSelection={{
          selectedRowKeys,
          onChange: keys => setSelectedRowKeys(keys),
          getCheckboxProps: record => ({ disabled: record.username === 'admin' }),
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
        title={editing ? '编辑用户' : '新增用户'}
        placement="right"
        width={560}
        onClose={() => setModalOpen(false)}
        footer={
          <Space style={{ display: 'flex', justifyContent: 'flex-end' }}>
            <Button onClick={() => setModalOpen(false)}>取消</Button>
            <Button type="primary" onClick={handleSubmit}>确定</Button>
          </Space>
        }
      >
        <Form form={editForm} layout="vertical">
          {!editing && (
            <Form.Item label="账号" name="username" rules={[{ required: true, message: '请输入账号' }]}>
              <Input placeholder="请输入账号" />
            </Form.Item>
          )}
          {!editing && (
            <Form.Item
              label="密码"
              name="password"
              rules={[{ required: true, message: '请输入密码' }, { min: 6, message: '密码长度不能少于 6 位' }]}
            >
              <Input.Password placeholder="请输入密码" />
            </Form.Item>
          )}
          <Form.Item label="昵称" name="nickname" rules={[{ required: true, message: '请输入昵称' }]}>
            <Input placeholder="请输入昵称" />
          </Form.Item>
          <Row gutter={12}>
            <Col span={12}>
              <Form.Item label="手机号" name="phone">
                <Input placeholder="请输入手机号" />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item label="邮箱" name="email">
                <Input placeholder="请输入邮箱" />
              </Form.Item>
            </Col>
          </Row>
          <Form.Item label="部门" name="deptId">
            <TreeSelect placeholder="请选择部门" allowClear treeData={deptTree} treeDefaultExpandAll />
          </Form.Item>
          <Form.Item label="角色" name="roleIds">
            <Select mode="multiple" placeholder="请选择角色" allowClear>
              {roles.map(role => (
                <Select.Option key={role.id} value={role.id}>{role.name}</Select.Option>
              ))}
            </Select>
          </Form.Item>
          <Form.Item label="状态" name="status" initialValue={1}>
            <Select>
              <Select.Option value={1}>正常</Select.Option>
              <Select.Option value={0}>停用</Select.Option>
            </Select>
          </Form.Item>
          <Form.Item label="备注" name="remark">
            <Input.TextArea rows={2} placeholder="请输入备注" />
          </Form.Item>
        </Form>
      </Drawer>

      <Drawer
        open={pwdOpen}
        title={`重置密码${pwdUser ? ` - ${pwdUser.username}` : ''}`}
        placement="right"
        width={420}
        onClose={() => setPwdOpen(false)}
        footer={
          <Space style={{ display: 'flex', justifyContent: 'flex-end' }}>
            <Button onClick={() => setPwdOpen(false)}>取消</Button>
            <Button type="primary" onClick={handleResetPwd}>确定</Button>
          </Space>
        }
      >
        <Form form={pwdForm} layout="vertical">
          <Form.Item
            label="新密码"
            name="password"
            rules={[{ required: true, message: '请输入新密码' }, { min: 6, message: '密码长度不能少于 6 位' }]}
          >
            <Input.Password placeholder="请输入新密码" />
          </Form.Item>
        </Form>
      </Drawer>
    </Card>
  )
}

export default SysUserPage
