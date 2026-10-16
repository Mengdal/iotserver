import React, { useCallback, useEffect, useState } from 'react'
import {
  Button,
  Card,
  Col,
  DatePicker,
  Empty,
  Form,
  Input,
  InputNumber,
  Modal,
  Popconfirm,
  Radio,
  Row,
  Select,
  Space,
  Switch,
  Table,
  Tag,
  message,
} from 'antd'
import { PlusOutlined, ReloadOutlined, SearchOutlined } from '@ant-design/icons'
import type { ColumnsType } from 'antd/es/table'
import dayjs from 'dayjs'
import type { Dayjs } from 'dayjs'
import {
  getTenants,
  addTenant,
  updateTenant,
  deleteTenant,
  Tenant,
} from '@/api'
import { useAuth } from '@/utils/auth'

const { useMessage } = message

/** 租户编码规则，与后端 tenantCodePattern 保持一致 */
const CODE_PATTERN = /^[a-z0-9_]{2,32}$/

interface TenantFormValues {
  code?: string
  name: string
  contactName?: string
  contactPhone?: string
  contactEmail?: string
  expireAt?: Dayjs | null
  maxDevices?: number
  maxGateways?: number
  maxUsers?: number
  status?: number
  remark?: string
}

/** 配额：0 表示不限 */
const quotaText = (value: number) => (value > 0 ? String(value) : '不限')

const TenantPage: React.FC = () => {
  const [messageApi, contextHolder] = useMessage()
  const [searchForm] = Form.useForm()
  const [editForm] = Form.useForm<TenantFormValues>()

  const [loading, setLoading] = useState(false)
  const [dataSource, setDataSource] = useState<Tenant[]>([])
  const [pagination, setPagination] = useState({ current: 1, pageSize: 10 })
  const [total, setTotal] = useState(0)

  const [modalOpen, setModalOpen] = useState(false)
  const [editing, setEditing] = useState<Tenant | null>(null)
  const [submitting, setSubmitting] = useState(false)

  const canList = useAuth('platform:tenant:list')
  const canAdd = useAuth('platform:tenant:add')
  const canEdit = useAuth('platform:tenant:edit')
  const canRemove = useAuth('platform:tenant:remove')

  const loadData = useCallback(async (page = pagination.current, pageSize = pagination.pageSize) => {
    try {
      setLoading(true)
      const values = searchForm.getFieldsValue()
      const resp = await getTenants({
        name: values.name || undefined,
        code: values.code || undefined,
        // 后端 status 只按 0/1 过滤，未选择时传 -1 表示不过滤
        status: values.status === undefined || values.status === '' ? -1 : Number(values.status),
        page,
        pageSize,
      })
      if (resp.success) {
        setDataSource(resp.result.list || [])
        setTotal(resp.result.total || 0)
      } else {
        messageApi.open({ type: 'error', content: resp.errorMsg })
      }
    } finally {
      setLoading(false)
    }
  }, [searchForm, pagination.current, pagination.pageSize, messageApi])

  useEffect(() => {
    if (!canList) return
    loadData(1, pagination.pageSize)
  }, [])

  const handleSearch = () => {
    setPagination({ ...pagination, current: 1 })
    loadData(1, pagination.pageSize)
  }

  const handleReset = () => {
    searchForm.resetFields()
    setPagination({ ...pagination, current: 1 })
    loadData(1, pagination.pageSize)
  }

  const openAdd = () => {
    setEditing(null)
    editForm.resetFields()
    setModalOpen(true)
  }

  const openEdit = (record: Tenant) => {
    setEditing(record)
    editForm.resetFields()
    editForm.setFieldsValue({
      code: record.code,
      name: record.name,
      contactName: record.contactName,
      contactPhone: record.contactPhone,
      contactEmail: record.contactEmail,
      expireAt: record.expireAt ? dayjs(record.expireAt) : null,
      maxDevices: record.maxDevices || 0,
      maxGateways: record.maxGateways || 0,
      maxUsers: record.maxUsers || 0,
      status: record.status,
      remark: record.remark,
    })
    setModalOpen(true)
  }

  const handleSubmit = async () => {
    const values = await editForm.validateFields()
    const payload: Record<string, any> = {
      name: (values.name || '').trim(),
      contactName: values.contactName || '',
      contactPhone: values.contactPhone || '',
      contactEmail: values.contactEmail || '',
      // 0 表示永久
      expireAt: values.expireAt ? values.expireAt.valueOf() : 0,
      maxDevices: values.maxDevices || 0,
      maxGateways: values.maxGateways || 0,
      maxUsers: values.maxUsers || 0,
      remark: values.remark || '',
    }
    setSubmitting(true)
    try {
      let resp
      if (editing) {
        // 编码由后端创建后不可修改，更新时不提交 code
        resp = await updateTenant({ ...payload, id: editing.id, status: values.status ?? editing.status })
      } else {
        resp = await addTenant({ ...payload, code: (values.code || '').trim() })
      }
      if (resp.success) {
        messageApi.open({ type: 'success', content: editing ? '修改成功' : '新增成功' })
        setModalOpen(false)
        loadData()
      } else {
        messageApi.open({ type: 'error', content: resp.errorMsg })
      }
    } finally {
      setSubmitting(false)
    }
  }

  const handleDelete = async (id: string) => {
    const resp = await deleteTenant(id)
    if (resp.success) {
      messageApi.open({ type: 'success', content: '删除成功' })
      loadData()
    } else {
      messageApi.open({ type: 'error', content: resp.errorMsg })
    }
  }

  const handleStatusChange = async (record: Tenant, checked: boolean) => {
    // 后端是整行更新，未提交的字段会被清空，所以带上原值只改状态
    const resp = await updateTenant({ ...record, status: checked ? 1 : 0 })
    if (resp.success) {
      messageApi.open({ type: 'success', content: checked ? '已启用' : '已停用' })
      loadData()
    } else {
      messageApi.open({ type: 'error', content: resp.errorMsg })
    }
  }

  const columns: ColumnsType<Tenant> = [
    { title: '租户名称', dataIndex: 'name', width: 200 },
    { title: '编码', dataIndex: 'code', width: 140, render: (value: string) => value || '-' },
    {
      title: '联系人/电话',
      dataIndex: 'contactName',
      width: 180,
      render: (value: string, record) => (
        <span>
          {value || '-'}
          {record.contactPhone ? ` / ${record.contactPhone}` : ''}
        </span>
      ),
    },
    {
      title: '到期时间',
      dataIndex: 'expireAt',
      width: 190,
      render: (value: number) => {
        if (!value) return <Tag color="blue">永久</Tag>
        const expired = value < Date.now()
        return (
          <Space size={4}>
            <span>{dayjs(value).format('YYYY-MM-DD HH:mm:ss')}</span>
            {expired && <Tag color="error">已过期</Tag>}
          </Space>
        )
      },
    },
    {
      title: '配额（设备/网关/用户）',
      key: 'quota',
      width: 200,
      render: (_, record) => (
        <span>
          {quotaText(record.maxDevices)} / {quotaText(record.maxGateways)} / {quotaText(record.maxUsers)}
        </span>
      ),
    },
    {
      title: '状态',
      dataIndex: 'status',
      width: 90,
      render: (value: number, record) => (
        <Switch
          checked={value === 1}
          disabled={!canEdit}
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
      width: 140,
      fixed: 'right',
      render: (_, record) => (
        <Space size={4}>
          {canEdit && <Button type="link" size="small" onClick={() => openEdit(record)}>编辑</Button>}
          {canRemove && (
            <Popconfirm
              title="确认删除该租户？删除后该租户下的业务数据不会级联清除。"
              onConfirm={() => handleDelete(record.id)}
              okText="确定"
              cancelText="取消"
            >
              <Button type="link" size="small" danger>删除</Button>
            </Popconfirm>
          )}
        </Space>
      ),
    },
  ]

  // 无列表权限时不请求、不渲染表格
  if (!canList) {
    return (
      <Card className="admin-page">
        {contextHolder}
        <Empty description="无租户列表查看权限" />
      </Card>
    )
  }

  return (
    <Card className="admin-page">
      {contextHolder}
      <Form form={searchForm} layout="inline" style={{ marginBottom: 16, rowGap: 8 }}>
        <Form.Item name="name" label="租户名称">
          <Input placeholder="请输入租户名称" allowClear style={{ width: 180 }} />
        </Form.Item>
        <Form.Item name="code" label="编码">
          <Input placeholder="请输入编码" allowClear style={{ width: 160 }} />
        </Form.Item>
        <Form.Item name="status" label="状态">
          <Select placeholder="全部" allowClear style={{ width: 120 }}>
            <Select.Option value="1">启用</Select.Option>
            <Select.Option value="0">停用</Select.Option>
          </Select>
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
            {canAdd && <Button type="primary" icon={<PlusOutlined />} onClick={openAdd}>新增租户</Button>}
          </Space>
        </Col>
      </Row>

      <Table<Tenant>
        rowKey="id"
        size="small"
        bordered
        loading={loading}
        columns={columns}
        dataSource={dataSource}
        scroll={{ x: 1400 }}
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

      <Modal
        open={modalOpen}
        title={editing ? '编辑租户' : '新增租户'}
        width={640}
        okText="确定"
        cancelText="取消"
        maskClosable={false}
        confirmLoading={submitting}
        onOk={handleSubmit}
        onCancel={() => setModalOpen(false)}
      >
        <Form
          form={editForm}
          layout="vertical"
          initialValues={{ status: 1, maxDevices: 0, maxGateways: 0, maxUsers: 0 }}
        >
          <Row gutter={12}>
            <Col span={12}>
              <Form.Item
                label="租户名称"
                name="name"
                rules={[{ required: true, message: '请输入租户名称' }]}
              >
                <Input placeholder="请输入租户名称" />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item
                label="编码"
                name="code"
                tooltip="小写字母/数字/下划线，2~32 位；创建后不可修改"
                rules={editing
                  ? []
                  : [
                    { required: true, message: '请输入编码' },
                    { pattern: CODE_PATTERN, message: '只允许小写字母/数字/下划线，2~32 位' },
                  ]}
              >
                {/* 编码由后端创建后固定，编辑时只读展示 */}
                <Input placeholder="如 customer_01" disabled={!!editing} />
              </Form.Item>
            </Col>
          </Row>
          <Row gutter={12}>
            <Col span={8}>
              <Form.Item label="联系人" name="contactName">
                <Input placeholder="请输入联系人" />
              </Form.Item>
            </Col>
            <Col span={8}>
              <Form.Item label="联系电话" name="contactPhone">
                <Input placeholder="请输入联系电话" />
              </Form.Item>
            </Col>
            <Col span={8}>
              <Form.Item label="联系邮箱" name="contactEmail">
                <Input placeholder="请输入联系邮箱" />
              </Form.Item>
            </Col>
          </Row>
          <Row gutter={12}>
            <Col span={12}>
              <Form.Item label="到期时间" name="expireAt" tooltip="留空表示永久有效">
                <DatePicker
                  showTime
                  style={{ width: '100%' }}
                  placeholder="留空表示永久"
                  format="YYYY-MM-DD HH:mm:ss"
                />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item label="状态" name="status">
                <Radio.Group>
                  <Radio value={1}>启用</Radio>
                  <Radio value={0}>停用</Radio>
                </Radio.Group>
              </Form.Item>
            </Col>
          </Row>
          <Row gutter={12}>
            <Col span={8}>
              <Form.Item label="设备配额" name="maxDevices" tooltip="0 表示不限">
                <InputNumber min={0} precision={0} style={{ width: '100%' }} placeholder="0" />
              </Form.Item>
            </Col>
            <Col span={8}>
              <Form.Item label="网关配额" name="maxGateways" tooltip="0 表示不限">
                <InputNumber min={0} precision={0} style={{ width: '100%' }} placeholder="0" />
              </Form.Item>
            </Col>
            <Col span={8}>
              <Form.Item label="用户配额" name="maxUsers" tooltip="0 表示不限">
                <InputNumber min={0} precision={0} style={{ width: '100%' }} placeholder="0" />
              </Form.Item>
            </Col>
          </Row>
          <Form.Item label="备注" name="remark">
            <Input.TextArea rows={2} placeholder="请输入备注" />
          </Form.Item>
        </Form>
      </Modal>
    </Card>
  )
}

export default TenantPage
