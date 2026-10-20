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
import { t, tt } from '@/i18n'

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
      messageApi.open({ type: 'success', content: editing ? t('修改成功') : t('新增成功') })
      setModalOpen(false)
      loadData()
    } else {
      messageApi.open({ type: 'error', content: resp.errorMsg })
    }
  }

  const handleDelete = async (ids: string[]) => {
    const resp = await deleteSysUser(ids)
    if (resp.success) {
      messageApi.open({ type: 'success', content: t('删除成功') })
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
      messageApi.open({ type: 'success', content: t('密码重置成功') })
      setPwdOpen(false)
      pwdForm.resetFields()
    } else {
      messageApi.open({ type: 'error', content: resp.errorMsg })
    }
  }

  const handleStatusChange = async (record: SysUser, checked: boolean) => {
    const resp = await changeSysUserStatus(record.id, checked ? 1 : 0)
    if (resp.success) {
      messageApi.open({ type: 'success', content: checked ? t('已启用') : t('已停用') })
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
    { title: t('账号'), dataIndex: 'username', width: 140 },
    { title: t('昵称'), dataIndex: 'nickname', width: 140 },
    {
      title: t('部门'),
      dataIndex: 'deptName',
      width: 140,
      render: (value: string, record) => value || deptNameMap[record.deptId] || '-',
    },
    { title: t('手机号'), dataIndex: 'phone', width: 140, render: (value: string) => value || '-' },
    { title: t('邮箱'), dataIndex: 'email', width: 180, render: (value: string) => value || '-' },
    {
      title: t('角色'),
      dataIndex: 'roleNames',
      width: 200,
      render: (value: string[]) => (
        <Space size={[4, 4]} wrap>
          {(value || []).length === 0 ? '-' : value.map(name => <Tag key={name} color="blue">{tt(name)}</Tag>)}
        </Space>
      ),
    },
    {
      title: t('状态'),
      dataIndex: 'status',
      width: 120,
      render: (value: number, record) => (
        <Switch
          checked={value === 1}
          disabled={!canEdit || record.username === 'admin'}
          checkedChildren={t('启用')}
          unCheckedChildren={t('停用')}
          onChange={checked => handleStatusChange(record, checked)}
        />
      ),
    },
    {
      title: t('创建时间'),
      dataIndex: 'created',
      width: 180,
      render: (value: number) => (value ? dayjs(value).format('YYYY-MM-DD HH:mm:ss') : '-'),
    },
    {
      title: t('操作'),
      key: 'action',
      width: 240,
      fixed: 'right',
      render: (_, record) => (
        <Space size={4} wrap={false} style={{ whiteSpace: 'nowrap' }}>
          {canEdit && <Button type="link" size="small" onClick={() => openEdit(record)}>{t('编辑')}</Button>}
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
              {t('重置密码')}
            </Button>
          )}
          {canRemove && record.username !== 'admin' && (
            <Popconfirm title={t('确认删除该用户？')} onConfirm={() => handleDelete([record.id])} okText={t('确定')} cancelText={t('取消')}>
              <Button type="link" size="small" danger>{t('删除')}</Button>
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
        <Form.Item name="username" label={t('账号')}>
          <Input placeholder={t('请输入账号')} allowClear style={{ width: 160 }} />
        </Form.Item>
        <Form.Item name="nickname" label={t('昵称')}>
          <Input placeholder={t('请输入昵称')} allowClear style={{ width: 160 }} />
        </Form.Item>
        <Form.Item name="phone" label={t('手机号')}>
          <Input placeholder={t('请输入手机号')} allowClear style={{ width: 160 }} />
        </Form.Item>
        <Form.Item name="status" label={t('状态')}>
          <Select placeholder={t('全部')} allowClear style={{ width: 120 }}>
            <Select.Option value="1">{t('正常')}</Select.Option>
            <Select.Option value="0">{t('停用')}</Select.Option>
          </Select>
        </Form.Item>
        <Form.Item name="deptId" label={t('部门')}>
          <TreeSelect
            placeholder={t('请选择部门')}
            allowClear
            style={{ width: 180 }}
            treeData={deptTree}
            treeDefaultExpandAll
          />
        </Form.Item>
        <Form.Item>
          <Space>
            <Button type="primary" icon={<SearchOutlined />} onClick={handleSearch}>{t('查询')}</Button>
            <Button icon={<ReloadOutlined />} onClick={handleReset}>{t('重置')}</Button>
          </Space>
        </Form.Item>
      </Form>

      <Row justify="space-between" style={{ marginBottom: 12 }}>
        <Col>
          <Space>
            {canAdd && <Button type="primary" icon={<PlusOutlined />} onClick={openAdd}>{t('新增用户')}</Button>}
            {canRemove && (
              <Popconfirm
                title={t('确认删除选中的 {{count}} 个用户？', { count: selectedRowKeys.length })}
                onConfirm={() => handleDelete(selectedRowKeys as string[])}
                okText={t('确定')}
                cancelText={t('取消')}
                disabled={selectedRowKeys.length === 0}
              >
                <Button danger disabled={selectedRowKeys.length === 0}>{t('批量删除')}</Button>
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
          showTotal: total => t('共 {{count}} 条', { count: total }),
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
        title={editing ? t('编辑用户') : t('新增用户')}
        placement="right"
        width={560}
        onClose={() => setModalOpen(false)}
        footer={
          <Space style={{ display: 'flex', justifyContent: 'flex-end' }}>
            <Button onClick={() => setModalOpen(false)}>{t('取消')}</Button>
            <Button type="primary" onClick={handleSubmit}>{t('确定')}</Button>
          </Space>
        }
      >
        <Form form={editForm} layout="vertical">
          {!editing && (
            <Form.Item label={t('账号')} name="username" rules={[{ required: true, message: t('请输入账号') }]}>
              <Input placeholder={t('请输入账号')} />
            </Form.Item>
          )}
          {!editing && (
            <Form.Item
              label={t('密码')}
              name="password"
              rules={[{ required: true, message: t('请输入密码') }, { min: 6, message: t('密码长度不能少于 6 位') }]}
            >
              <Input.Password placeholder={t('请输入密码')} />
            </Form.Item>
          )}
          <Form.Item label={t('昵称')} name="nickname" rules={[{ required: true, message: t('请输入昵称') }]}>
            <Input placeholder={t('请输入昵称')} />
          </Form.Item>
          <Row gutter={12}>
            <Col span={12}>
              <Form.Item label={t('手机号')} name="phone">
                <Input placeholder={t('请输入手机号')} />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item label={t('邮箱')} name="email">
                <Input placeholder={t('请输入邮箱')} />
              </Form.Item>
            </Col>
          </Row>
          <Form.Item label={t('部门')} name="deptId">
            <TreeSelect placeholder={t('请选择部门')} allowClear treeData={deptTree} treeDefaultExpandAll />
          </Form.Item>
          <Form.Item label={t('角色')} name="roleIds">
            <Select mode="multiple" placeholder={t('请选择角色')} allowClear>
              {roles.map(role => (
                <Select.Option key={role.id} value={role.id}>{tt(role.name)}</Select.Option>
              ))}
            </Select>
          </Form.Item>
          <Form.Item label={t('状态')} name="status" initialValue={1}>
            <Select>
              <Select.Option value={1}>{t('正常')}</Select.Option>
              <Select.Option value={0}>{t('停用')}</Select.Option>
            </Select>
          </Form.Item>
          <Form.Item label={t('备注')} name="remark">
            <Input.TextArea rows={2} placeholder={t('请输入备注')} />
          </Form.Item>
        </Form>
      </Drawer>

      <Drawer
        open={pwdOpen}
        title={t('重置密码{{p1}}', { p1: pwdUser ? ` - ${pwdUser.username}` : '' })}
        placement="right"
        width={420}
        onClose={() => setPwdOpen(false)}
        footer={
          <Space style={{ display: 'flex', justifyContent: 'flex-end' }}>
            <Button onClick={() => setPwdOpen(false)}>{t('取消')}</Button>
            <Button type="primary" onClick={handleResetPwd}>{t('确定')}</Button>
          </Space>
        }
      >
        <Form form={pwdForm} layout="vertical">
          <Form.Item
            label={t('新密码')}
            name="password"
            rules={[{ required: true, message: t('请输入新密码') }, { min: 6, message: t('密码长度不能少于 6 位') }]}
          >
            <Input.Password placeholder={t('请输入新密码')} />
          </Form.Item>
        </Form>
      </Drawer>
    </Card>
  )
}

export default SysUserPage
