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
  Spin,
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
  getThingsProjects,
  bindTenantThingsProject,
  renameTenantThingsProject,
  Tenant,
  ThingsProject,
} from '@/api'
import { useAuth } from '@/utils/auth'
import { t } from '@/i18n'

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
  retentionDays?: number
  status?: number
  thingsProjectName?: string
  remark?: string
}

/** 配额：0 表示不限 */
const quotaText = (value: number) => (value > 0 ? String(value) : t('不限'))

/** 数据保留天数：0 = 默认 30 天，<0 = 永久 */
const retentionText = (value?: number) => {
  if (value === undefined || value === null || value === 0) return t('默认（30 天）')
  if (value < 0) return t('永久')
  return t('{{count}} 天', { count: value })
}

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

  // things 项目绑定/改名
  const [thingsOpen, setThingsOpen] = useState(false)
  const [thingsTarget, setThingsTarget] = useState<Tenant | null>(null)
  const [thingsAction, setThingsAction] = useState<'bind' | 'rename'>('bind')
  const [thingsMode, setThingsMode] = useState<'existing' | 'create'>('existing')
  const [thingsProjects, setThingsProjects] = useState<ThingsProject[]>([])
  const [thingsLoading, setThingsLoading] = useState(false)
  const [thingsSubmitting, setThingsSubmitting] = useState(false)
  const [thingsForm] = Form.useForm()

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
      retentionDays: record.retentionDays ?? 0,
      status: record.status,
      thingsProjectName: record.thingsProjectName,
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
      // 0 = 默认 30 天，-1 = 永久
      retentionDays: values.retentionDays ?? 0,
      thingsProjectName: (values.thingsProjectName || '').trim(),
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
        messageApi.open({ type: 'success', content: editing ? t('修改成功') : t('新增成功') })
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
      messageApi.open({ type: 'success', content: t('删除成功') })
      loadData()
    } else {
      messageApi.open({ type: 'error', content: resp.errorMsg })
    }
  }

  const openThingsBind = async (record: Tenant) => {
    setThingsTarget(record)
    setThingsAction('bind')
    setThingsMode('existing')
    thingsForm.resetFields()
    setThingsOpen(true)
    setThingsLoading(true)
    try {
      const resp = await getThingsProjects()
      if (resp.success) {
        setThingsProjects(resp.result || [])
      } else {
        messageApi.open({ type: 'error', content: resp.errorMsg })
      }
    } finally {
      setThingsLoading(false)
    }
  }

  const openThingsRename = (record: Tenant) => {
    setThingsTarget(record)
    setThingsAction('rename')
    thingsForm.setFieldsValue({ name: record.thingsProjectName })
    setThingsOpen(true)
  }

  const handleThingsSubmit = async () => {
    if (!thingsTarget) return
    const values = await thingsForm.validateFields()
    setThingsSubmitting(true)
    try {
      let resp
      if (thingsAction === 'rename') {
        resp = await renameTenantThingsProject(thingsTarget.id, (values.name || '').trim())
      } else if (thingsMode === 'existing') {
        resp = await bindTenantThingsProject(thingsTarget.id, { projectId: Number(values.projectId) })
      } else {
        resp = await bindTenantThingsProject(thingsTarget.id, {
          name: (values.name || '').trim(),
          description: values.description || '',
        })
      }
      if (resp.success) {
        messageApi.open({ type: 'success', content: t('操作成功') })
        setThingsOpen(false)
        loadData()
      } else {
        messageApi.open({ type: 'error', content: resp.errorMsg })
      }
    } finally {
      setThingsSubmitting(false)
    }
  }

  const handleStatusChange = async (record: Tenant, checked: boolean) => {
    // 后端是整行更新，未提交的字段会被清空，所以带上原值只改状态
    const resp = await updateTenant({ ...record, status: checked ? 1 : 0 })
    if (resp.success) {
      messageApi.open({ type: 'success', content: checked ? t('已启用') : t('已停用') })
      loadData()
    } else {
      messageApi.open({ type: 'error', content: resp.errorMsg })
    }
  }

  const columns: ColumnsType<Tenant> = [
    { title: t('租户名称'), dataIndex: 'name', width: 200 },
    { title: t('编码'), dataIndex: 'code', width: 140, render: (value: string) => value || '-' },
    {
      title: t('联系人/电话'),
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
      title: t('到期时间'),
      dataIndex: 'expireAt',
      width: 190,
      render: (value: number) => {
        if (!value) return <Tag color="blue">{t('永久')}</Tag>
        const expired = value < Date.now()
        return (
          <Space size={4}>
            <span>{dayjs(value).format('YYYY-MM-DD HH:mm:ss')}</span>
            {expired && <Tag color="error">{t('已过期')}</Tag>}
          </Space>
        )
      },
    },
    {
      title: t('配额（设备/网关/用户）'),
      key: 'quota',
      width: 200,
      render: (_, record) => (
        <span>
          {quotaText(record.maxDevices)} / {quotaText(record.maxGateways)} / {quotaText(record.maxUsers)}
        </span>
      ),
    },
    {
      title: t('数据保留'),
      dataIndex: 'retentionDays',
      width: 140,
      render: (value: number) => retentionText(value),
    },
    {
      title: t('things 项目'),
      key: 'things',
      width: 210,
      render: (_, record) => {
        if (!record.thingsProjectName && !record.thingsProjectId) {
          return <Tag>{t('未绑定')}</Tag>
        }
        const color = record.thingsBindStatus === 'error' || record.thingsBindStatus === 'orphaned'
          ? 'error'
          : record.thingsBindStatus === 'bound' ? 'success' : 'default'
        return (
          <Space direction="vertical" size={0}>
            <span>{record.thingsProjectName || `#${record.thingsProjectId}`}</span>
            <Tag color={color}>{record.thingsBindStatus || '-'}</Tag>
            {record.thingsLastError && <Tag color="error">{record.thingsLastError}</Tag>}
          </Space>
        )
      },
    },
    {
      title: t('状态'),
      dataIndex: 'status',
      width: 120,
      render: (value: number, record) => (
        <Switch
          checked={value === 1}
          disabled={!canEdit}
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
          {canEdit && !record.thingsProjectId && (
            <Button type="link" size="small" onClick={() => openThingsBind(record)}>{t('绑定项目')}</Button>
          )}
          {canEdit && !!record.thingsProjectId && (
            <Button type="link" size="small" onClick={() => openThingsRename(record)}>{t('项目改名')}</Button>
          )}
          {canRemove && (
            <Popconfirm
              title={t('确认删除该租户？删除后该租户下的业务数据不会级联清除。')}
              onConfirm={() => handleDelete(record.id)}
              okText={t('确定')}
              cancelText={t('取消')}
            >
              <Button type="link" size="small" danger>{t('删除')}</Button>
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
        <Empty description={t('无租户列表查看权限')} />
      </Card>
    )
  }

  return (
    <Card className="admin-page">
      {contextHolder}
      <Form form={searchForm} layout="inline" style={{ marginBottom: 16, rowGap: 8 }}>
        <Form.Item name="name" label={t('租户名称')}>
          <Input placeholder={t('请输入租户名称')} allowClear style={{ width: 180 }} />
        </Form.Item>
        <Form.Item name="code" label={t('编码')}>
          <Input placeholder={t('请输入编码')} allowClear style={{ width: 160 }} />
        </Form.Item>
        <Form.Item name="status" label={t('状态')}>
          <Select placeholder={t('全部')} allowClear style={{ width: 120 }}>
            <Select.Option value="1">{t('启用')}</Select.Option>
            <Select.Option value="0">{t('停用')}</Select.Option>
          </Select>
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
            {canAdd && <Button type="primary" icon={<PlusOutlined />} onClick={openAdd}>{t('新增租户')}</Button>}
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
        scroll={{ x: 1650 }}
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

      {/* 关闭动画结束时 rc-motion 会先把透明度重置为 1 再 display:none，合成层上会闪一帧完整弹窗；
          这里直接关闭动画，取消后立即消失 */}
      <Modal
        open={modalOpen}
        title={editing ? t('编辑租户') : t('新增租户')}
        width={640}
        okText={t('确定')}
        cancelText={t('取消')}
        maskClosable={false}
        transitionName=""
        maskTransitionName=""
        confirmLoading={submitting}
        onOk={handleSubmit}
        onCancel={() => setModalOpen(false)}
      >
        <Form
          form={editForm}
          layout="vertical"
          initialValues={{ status: 1, maxDevices: 0, maxGateways: 0, maxUsers: 0, retentionDays: 0 }}
        >
          <Row gutter={12}>
            <Col span={12}>
              <Form.Item
                label={t('租户名称')}
                name="name"
                rules={[{ required: true, message: t('请输入租户名称') }]}
              >
                <Input placeholder={t('请输入租户名称')} />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item
                label={t('编码')}
                name="code"
                tooltip={t('小写字母/数字/下划线，2~32 位；创建后不可修改')}
                rules={editing
                  ? []
                  : [
                    { required: true, message: t('请输入编码') },
                    { pattern: CODE_PATTERN, message: t('只允许小写字母/数字/下划线，2~32 位') },
                  ]}
              >
                {/* 编码由后端创建后固定，编辑时只读展示 */}
                <Input placeholder={t('如 customer_01')} disabled={!!editing} />
              </Form.Item>
            </Col>
          </Row>
          <Row gutter={12}>
            <Col span={24}>
              <Form.Item
                label={t('things 项目名称')}
                name="thingsProjectName"
                tooltip={t('things 启用时必填，4~30 个字符且不能与已有项目重名；未启用时忽略')}
              >
                <Input placeholder={t('如 Tenant A')} disabled={!!editing} />
              </Form.Item>
            </Col>
          </Row>
          <Row gutter={12}>
            <Col span={8}>
              <Form.Item label={t('联系人')} name="contactName">
                <Input placeholder={t('请输入联系人')} />
              </Form.Item>
            </Col>
            <Col span={8}>
              <Form.Item label={t('联系电话')} name="contactPhone">
                <Input placeholder={t('请输入联系电话')} />
              </Form.Item>
            </Col>
            <Col span={8}>
              <Form.Item label={t('联系邮箱')} name="contactEmail">
                <Input placeholder={t('请输入联系邮箱')} />
              </Form.Item>
            </Col>
          </Row>
          <Row gutter={12}>
            <Col span={12}>
              <Form.Item label={t('到期时间')} name="expireAt" tooltip={t('留空表示永久有效')}>
                <DatePicker
                  showTime
                  style={{ width: '100%' }}
                  placeholder={t('留空表示永久')}
                  format="YYYY-MM-DD HH:mm:ss"
                />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item label={t('状态')} name="status">
                <Radio.Group>
                  <Radio value={1}>{t('启用')}</Radio>
                  <Radio value={0}>{t('停用')}</Radio>
                </Radio.Group>
              </Form.Item>
            </Col>
          </Row>
          <Row gutter={12}>
            <Col span={8}>
              <Form.Item label={t('设备配额')} name="maxDevices" tooltip={t('0 表示不限')}>
                <InputNumber min={0} precision={0} style={{ width: '100%' }} placeholder="0" />
              </Form.Item>
            </Col>
            <Col span={8}>
              <Form.Item label={t('网关配额')} name="maxGateways" tooltip={t('0 表示不限')}>
                <InputNumber min={0} precision={0} style={{ width: '100%' }} placeholder="0" />
              </Form.Item>
            </Col>
            <Col span={8}>
              <Form.Item label={t('用户配额')} name="maxUsers" tooltip={t('0 表示不限')}>
                <InputNumber min={0} precision={0} style={{ width: '100%' }} placeholder="0" />
              </Form.Item>
            </Col>
          </Row>
          <Row gutter={12}>
            <Col span={12}>
              <Form.Item
                label={t('数据保留天数')}
                name="retentionDays"
                tooltip={t('0 表示使用默认（30 天），-1 表示永久保留')}
              >
                <InputNumber min={-1} precision={0} style={{ width: '100%' }} placeholder="0" />
              </Form.Item>
            </Col>
          </Row>
          <Form.Item label={t('备注')} name="remark">
            <Input.TextArea rows={2} placeholder={t('请输入备注')} />
          </Form.Item>
        </Form>
      </Modal>

      <Modal
        open={thingsOpen}
        title={thingsAction === 'rename' ? t('重命名 things 项目') : t('绑定 things 项目')}
        width={520}
        okText={t('确定')}
        cancelText={t('取消')}
        confirmLoading={thingsSubmitting}
        onOk={handleThingsSubmit}
        onCancel={() => setThingsOpen(false)}
      >
        <Spin spinning={thingsLoading}>
          {thingsAction === 'bind' && (
            <Radio.Group
              value={thingsMode}
              onChange={e => setThingsMode(e.target.value)}
              style={{ marginBottom: 16 }}
            >
              <Radio.Button value="existing">{t('绑定已有项目')}</Radio.Button>
              <Radio.Button value="create">{t('创建新项目')}</Radio.Button>
            </Radio.Group>
          )}
          <Form form={thingsForm} layout="vertical">
            {thingsAction === 'rename' && (
              <Form.Item
                label={t('项目名称')}
                name="name"
                rules={[{ required: true, message: t('请输入项目名称') }]}
              >
                <Input placeholder={t('4~30 个字符')} />
              </Form.Item>
            )}
            {thingsAction !== 'rename' && thingsMode === 'existing' && (
              <Form.Item
                label={t('things 项目')}
                name="projectId"
                rules={[{ required: true, message: t('请选择 things 项目') }]}
              >
                <Select
                  showSearch
                  optionFilterProp="label"
                  placeholder={t('请选择 things 项目')}
                  options={thingsProjects.map(p => ({ value: p.id, label: `${p.name} (#${p.id})` }))}
                />
              </Form.Item>
            )}
            {thingsAction !== 'rename' && thingsMode === 'create' && (
              <>
                <Form.Item
                  label={t('项目名称')}
                  name="name"
                  rules={[{ required: true, message: t('请输入项目名称') }]}
                >
                  <Input placeholder={t('4~30 个字符')} />
                </Form.Item>
                <Form.Item label={t('备注')} name="description">
                  <Input.TextArea rows={2} placeholder={t('请输入备注')} />
                </Form.Item>
              </>
            )}
          </Form>
        </Spin>
      </Modal>
    </Card>
  )
}

export default TenantPage
