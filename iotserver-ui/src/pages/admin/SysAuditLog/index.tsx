import React, { useCallback, useEffect, useState } from 'react'
import {
  Button,
  Card,
  Col,
  DatePicker,
  Descriptions,
  Empty,
  Form,
  Input,
  Modal,
  Popconfirm,
  Row,
  Select,
  Space,
  Table,
  Tag,
  Tooltip,
  message,
} from 'antd'
import { ClearOutlined, ReloadOutlined, SearchOutlined } from '@ant-design/icons'
import type { ColumnsType } from 'antd/es/table'
import dayjs from 'dayjs'
import { getSysAuditLogs, deleteSysAuditLogs, clearSysAuditLogs, SysAuditLog } from '@/api'
import { useAuth } from '@/utils/auth'
import { t, tt } from '@/i18n'

const { useMessage } = message
const { RangePicker } = DatePicker

/**
 * 权限码：permissions.ts 由 tools/gen-permissions 重新生成前，先用字面量，避免依赖生成产物。
 * 旧码 system:loginlog:* 由后端保留一版兼容。
 */
const PERM_AUDIT_LIST = 'system:auditlog:list'
const PERM_AUDIT_REMOVE = 'system:auditlog:remove'

const TYPE_OPTIONS = [
  { label: t('全部'), value: '' },
  { label: t('登录'), value: 'login' },
  { label: t('操作'), value: 'operation' },
]

const STATUS_OPTIONS = [
  { label: t('全部'), value: '' },
  { label: t('成功'), value: 1 },
  { label: t('失败'), value: 0 },
]

const formatTime = (value?: number) => (value ? dayjs(value).format('YYYY-MM-DD HH:mm:ss') : '-')

/** params 是后端存的 JSON 字符串，能解析就格式化，解析失败原样展示 */
const formatParams = (params?: string) => {
  if (!params) return '-'
  try {
    return JSON.stringify(JSON.parse(params), null, 2)
  } catch (error) {
    return params
  }
}

const isLogin = (record: SysAuditLog) => record.type === 'login'

const SysAuditLogPage: React.FC = () => {
  const [messageApi, contextHolder] = useMessage()
  const [form] = Form.useForm()

  const [loading, setLoading] = useState(false)
  const [dataSource, setDataSource] = useState<SysAuditLog[]>([])
  const [pagination, setPagination] = useState({ current: 1, pageSize: 10 })
  const [total, setTotal] = useState(0)
  const [detail, setDetail] = useState<SysAuditLog | null>(null)

  const canList = useAuth(PERM_AUDIT_LIST)
  const canRemove = useAuth(PERM_AUDIT_REMOVE)

  const loadData = useCallback(async (page = 1, pageSize = 10) => {
    try {
      setLoading(true)
      const values = form.getFieldsValue()
      const params: Record<string, any> = { page, pageSize }
      const username = String(values.username || '').trim()
      if (username) params.username = username
      const module = String(values.module || '').trim()
      if (module) params.module = module
      if (values.type) params.type = values.type
      if (values.status === 0 || values.status === 1) params.status = values.status
      if (Array.isArray(values.time) && values.time.length === 2) {
        params.startTime = values.time[0].valueOf()
        params.endTime = values.time[1].valueOf()
      }
      const resp = await getSysAuditLogs(params)
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
  }, [form, messageApi])

  useEffect(() => {
    // 无列表权限时不发请求，直接展示空态
    if (canList) {
      loadData(1, pagination.pageSize)
    }
  }, [canList])

  const search = () => {
    setPagination({ ...pagination, current: 1 })
    loadData(1, pagination.pageSize)
  }

  const reset = () => {
    form.resetFields()
    setPagination({ ...pagination, current: 1 })
    loadData(1, pagination.pageSize)
  }

  const handleDelete = async (id: string) => {
    const resp = await deleteSysAuditLogs([id])
    if (resp.success) {
      messageApi.open({ type: 'success', content: t('删除成功') })
      loadData(pagination.current, pagination.pageSize)
    } else {
      messageApi.open({ type: 'error', content: resp.errorMsg })
    }
  }

  const handleClear = async () => {
    const resp = await clearSysAuditLogs()
    if (resp.success) {
      messageApi.open({ type: 'success', content: t('清空成功') })
      setPagination({ current: 1, pageSize: pagination.pageSize })
      loadData(1, pagination.pageSize)
    } else {
      messageApi.open({ type: 'error', content: resp.errorMsg })
    }
  }

  const renderType = (value: string) => {
    if (value === 'login') return <Tag color="processing">{t('登录')}</Tag>
    if (value === 'operation') return <Tag color="success">{t('操作')}</Tag>
    return <Tag>{value || '-'}</Tag>
  }

  const renderStatus = (record: SysAuditLog) => {
    const ok = record.status === 1
    return (
      <Space size={4}>
        <Tag color={ok ? 'success' : 'error'}>{ok ? t('成功') : t('失败')}</Tag>
        {!ok && (
          <Tooltip title={record.message || ''}>
            <span style={{ color: '#ff4d4f' }}>{record.errorCode ? String(record.errorCode) : t('失败')}</span>
          </Tooltip>
        )}
      </Space>
    )
  }

  const renderDuration = (record: SysAuditLog) => {
    if (isLogin(record) || record.durationMs === undefined || record.durationMs === null) return '-'
    return `${record.durationMs} ms`
  }

  const columns: ColumnsType<SysAuditLog> = [
    { title: t('类型'), dataIndex: 'type', width: 80, render: (value: string) => renderType(value) },
    { title: t('操作人'), dataIndex: 'username', width: 140, render: (value: string) => value || '-' },
    { title: t('模块'), dataIndex: 'module', width: 140, ellipsis: true, render: (value: string) => tt(value) || '-' },
    { title: t('动作'), dataIndex: 'action', width: 160, ellipsis: true, render: (value: string) => tt(value) || '-' },
    { title: t('对象'), dataIndex: 'target', width: 160, ellipsis: true, render: (value: string) => value || '-' },
    { title: t('结果'), dataIndex: 'status', width: 160, render: (_: any, record: SysAuditLog) => renderStatus(record) },
    { title: t('耗时'), dataIndex: 'durationMs', width: 100, render: (_: any, record: SysAuditLog) => renderDuration(record) },
    { title: 'IP', dataIndex: 'ip', width: 140, render: (value: string) => value || '-' },
    {
      title: t('时间'),
      dataIndex: 'created',
      width: 180,
      render: (_: any, record: SysAuditLog) => formatTime(record.created || record.loginTime),
    },
    {
      title: t('操作'),
      key: 'action',
      width: 140,
      fixed: 'right',
      render: (_: any, record: SysAuditLog) => (
        <Space size={4}>
          <Button type="link" size="small" onClick={() => setDetail(record)}>{t('详情')}</Button>
          {canRemove && (
            <Popconfirm
              title={t('此操作将永久删除该记录, 是否继续?')}
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

  return (
    <Card className="admin-page">
      {contextHolder}
      <Form form={form} layout="inline" style={{ marginBottom: 16, rowGap: 8 }} disabled={!canList}>
        <Form.Item name="type" label={t('类型')}>
          <Select options={TYPE_OPTIONS} style={{ width: 120 }} />
        </Form.Item>
        <Form.Item name="username" label={t('操作人')}>
          <Input placeholder={t('请输入操作人')} allowClear style={{ width: 160 }} />
        </Form.Item>
        <Form.Item name="module" label={t('模块')}>
          <Input placeholder={t('请输入模块')} allowClear style={{ width: 160 }} />
        </Form.Item>
        <Form.Item name="status" label={t('状态')}>
          <Select options={STATUS_OPTIONS} style={{ width: 120 }} />
        </Form.Item>
        <Form.Item name="time" label={t('时间范围')}>
          <RangePicker showTime style={{ width: 360 }} />
        </Form.Item>
        <Form.Item>
          <Space>
            <Button type="primary" icon={<SearchOutlined />} disabled={!canList} onClick={search}>
              {t('查询')}
            </Button>
            <Button icon={<ReloadOutlined />} disabled={!canList} onClick={reset}>
              {t('重置')}
            </Button>
          </Space>
        </Form.Item>
      </Form>

      <Row style={{ marginBottom: 12 }}>
        <Col>
          <Space>
            {canRemove && (
              <Popconfirm
                title={t('此操作将永久删除全部审查日志, 是否继续?')}
                onConfirm={handleClear}
                okText={t('确定')}
                cancelText={t('取消')}
                disabled={!canList}
              >
                <Button danger icon={<ClearOutlined />} disabled={!canList}>{t('清空')}</Button>
              </Popconfirm>
            )}
          </Space>
        </Col>
      </Row>

      {canList
        ? (
          <Table<SysAuditLog>
            rowKey="id"
            size="small"
            bordered
            loading={loading}
            columns={columns}
            dataSource={dataSource}
            scroll={{ x: 1460 }}
            locale={{ emptyText: <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={t('暂无审查日志')} /> }}
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
        )
        : (
          <Empty
            image={Empty.PRESENTED_IMAGE_SIMPLE}
            description={t('暂无审查日志查看权限')}
            style={{ padding: '48px 0' }}
          />
        )}

      <Modal
        title={t('审查日志详情')}
        open={!!detail}
        footer={null}
        width={760}
        onCancel={() => setDetail(null)}
      >
        {detail && (
          <Descriptions column={2} size="small" bordered>
            <Descriptions.Item label={t('类型')}>{renderType(detail.type)}</Descriptions.Item>
            <Descriptions.Item label={t('操作人')}>{detail.username || '-'}</Descriptions.Item>
            <Descriptions.Item label={t('模块')}>{tt(detail.module) || '-'}</Descriptions.Item>
            <Descriptions.Item label={t('动作')}>{tt(detail.action) || '-'}</Descriptions.Item>
            <Descriptions.Item label={t('对象')}>{detail.target || '-'}</Descriptions.Item>
            <Descriptions.Item label={t('方法')}>{detail.method || '-'}</Descriptions.Item>
            <Descriptions.Item label={t('路径')} span={2}>{detail.path || '-'}</Descriptions.Item>
            <Descriptions.Item label={t('结果')}>{renderStatus(detail)}</Descriptions.Item>
            <Descriptions.Item label={t('错误码')}>{detail.errorCode ? String(detail.errorCode) : '-'}</Descriptions.Item>
            <Descriptions.Item label={t('耗时')}>{renderDuration(detail)}</Descriptions.Item>
            <Descriptions.Item label="IP">{detail.ip || '-'}</Descriptions.Item>
            <Descriptions.Item label={t('浏览器')}>{detail.browser || '-'}</Descriptions.Item>
            <Descriptions.Item label={t('操作系统')}>{detail.os || '-'}</Descriptions.Item>
            <Descriptions.Item label={t('时间')}>{formatTime(detail.created || detail.loginTime)}</Descriptions.Item>
            <Descriptions.Item label={t('请求参数')} span={2}>
              <pre style={{ maxHeight: 240, overflow: 'auto', margin: 0, whiteSpace: 'pre-wrap' }}>
                {formatParams(detail.params)}
              </pre>
            </Descriptions.Item>
            {detail.status !== 1 && (
              <Descriptions.Item label={t('错误信息')} span={2}>{detail.message || '-'}</Descriptions.Item>
            )}
          </Descriptions>
        )}
      </Modal>
    </Card>
  )
}

export default SysAuditLogPage
