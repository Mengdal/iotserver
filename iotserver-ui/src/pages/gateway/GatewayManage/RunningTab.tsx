import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import {
  Alert,
  Button,
  Card,
  Form,
  Input,
  Space,
  Table,
  Tag,
  Typography,
  message,
} from 'antd'
import { ReloadOutlined, SearchOutlined } from '@ant-design/icons'
import type { ColumnsType } from 'antd/es/table'
import dayjs from 'dayjs'
import { Gateway } from '@/api'
import { t } from '@/i18n'

const { Text } = Typography

interface RunningRow {
  id: string
  description?: string
  val?: string
  status?: string
  timestamp?: number
}

interface RunningTabProps {
  gateway: Gateway
}

const tokenKey = (gatewayId: string) => `iotserver.gatewayToken.${gatewayId}`

/** 网关地址：上报过 IP 用网关 IP，否则按平台所在主机（本机 Docker 部署）。 */
function gatewayBase (gateway: Gateway): string {
  const host = gateway.ip || window.location.hostname
  return `http://${host}:8080`
}

function statusTag (status?: string) {
  const value = String(status || '').toLowerCase()
  if (value === 'good' || value === '正常' || value === 'ok') {
    return <Tag color="success">{status}</Tag>
  }
  if (value === 'uncertain' || value === '不确定') {
    return <Tag color="warning">{status}</Tag>
  }
  if (value === 'error' || value === 'bad' || value === '异常' || value === '故障') {
    return <Tag color="error">{status}</Tag>
  }
  return <Tag>{status || '-'}</Tag>
}

export const RunningTab: React.FC<RunningTabProps> = ({ gateway }) => {
  const [messageApi, contextHolder] = message.useMessage()
  const [loginForm] = Form.useForm()
  const [searchForm] = Form.useForm()

  const [token, setToken] = useState('')
  const [loggingIn, setLoggingIn] = useState(false)
  const [loading, setLoading] = useState(false)
  const [rows, setRows] = useState<RunningRow[]>([])
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(30)
  const filtersRef = useRef<{ id?: string, description?: string }>({})

  const baseUrl = useMemo(() => gatewayBase(gateway), [gateway])

  useEffect(() => {
    try {
      setToken(localStorage.getItem(tokenKey(gateway.id)) || '')
    } catch (error) {
      setToken('')
    }
    setRows([])
    setTotal(0)
    setPage(1)
    filtersRef.current = {}
    searchForm.resetFields()
  }, [gateway.id])

  const clearToken = useCallback(() => {
    try {
      localStorage.removeItem(tokenKey(gateway.id))
    } catch (error) {
      // 忽略
    }
    setToken('')
    setRows([])
    setTotal(0)
  }, [gateway.id])

  const load = useCallback(async (
    nextPage = page,
    nextPageSize = pageSize,
    filters: { id?: string, description?: string } = filtersRef.current,
    tk = token,
  ) => {
    if (!tk) return
    setLoading(true)
    try {
      const params = new URLSearchParams({
        page: String(nextPage),
        pagesize: String(nextPageSize),
        token: tk,
      })
      if (filters?.id) params.set('id', filters.id)
      if (filters?.description) params.set('description', filters.description)
      const resp = await fetch(`${baseUrl}/getSystemData?${params.toString()}`)
      const json = await resp.json()
      if (json.code === '401') {
        clearToken()
        messageApi.warning(t('网关登录已失效，请重新登录'))
        return
      }
      if (json.code !== '200') {
        messageApi.error(json.message || t('读取网关数据失败'))
        return
      }
      setRows(Array.isArray(json.tags) ? json.tags : [])
      setTotal(Number(json.nums) || 0)
      setPage(nextPage)
      setPageSize(nextPageSize)
    } catch (error) {
      messageApi.error(t('连接网关失败，请确认浏览器能访问 {{url}}', { url: baseUrl }))
    } finally {
      setLoading(false)
    }
  }, [baseUrl, clearToken, messageApi, page, pageSize, token])

  // token 从 localStorage 恢复后触发首次加载；登录成功后 setToken 也会走到这里。
  useEffect(() => {
    if (token) {
      load(1, pageSize, undefined, token)
    }
  }, [token, gateway.id])

  const handleLogin = async () => {
    const values = await loginForm.validateFields()
    setLoggingIn(true)
    try {
      const body = new URLSearchParams({
        username: String(values.username || '').trim(),
        password: String(values.password || ''),
      })
      const resp = await fetch(`${baseUrl}/login`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
        body: body.toString(),
      })
      const json = await resp.json()
      if (json.code !== '200' || !json.token) {
        messageApi.error(json.message || t('网关登录失败'))
        return
      }
      try {
        localStorage.setItem(tokenKey(gateway.id), json.token)
      } catch (error) {
        // 忽略
      }
      // 设置 token 后由上面的 effect 触发首次加载
      setToken(json.token)
    } catch (error) {
      messageApi.error(t('连接网关失败，请确认浏览器能访问 {{url}}', { url: baseUrl }))
    } finally {
      setLoggingIn(false)
    }
  }

  const onSearch = () => {
    const values = searchForm.getFieldsValue()
    filtersRef.current = { id: values.id, description: values.description }
    load(1, pageSize, filtersRef.current)
  }

  const onReset = () => {
    searchForm.resetFields()
    filtersRef.current = {}
    load(1, pageSize, {})
  }

  const columns: ColumnsType<RunningRow> = [
    {
      title: 'No',
      key: 'index',
      width: 60,
      align: 'center',
      render: (_, __, index) => (page - 1) * pageSize + index + 1,
    },
    { title: 'ID', dataIndex: 'id', width: 260, ellipsis: true },
    { title: t('描述'), dataIndex: 'description', width: 220, ellipsis: true },
    { title: t('值'), dataIndex: 'val', width: 200, ellipsis: true },
    {
      title: t('状态'),
      dataIndex: 'status',
      width: 110,
      align: 'center',
      render: (value: string) => statusTag(value),
    },
    {
      title: t('时间'),
      dataIndex: 'timestamp',
      width: 180,
      align: 'center',
      render: (value: number) => (value ? dayjs(value * 1000).format('YYYY-MM-DD HH:mm:ss') : '-'),
    },
  ]

  if (!token) {
    return (
      <>
        {contextHolder}
        <Alert
          type="info"
          showIcon
          style={{ marginBottom: 12 }}
          message={t('运行状态直接读取网关（{{url}}），需要先登录网关账号。', { url: baseUrl })}
        />
        <Card size="small" title={t('登录网关')} style={{ maxWidth: 420 }}>
          <Form form={loginForm} layout="vertical" onFinish={handleLogin}>
            <Form.Item label={t('用户名')} name="username" rules={[{ required: true, message: t('请输入用户名') }]}>
              <Input autoComplete="username" />
            </Form.Item>
            <Form.Item label={t('密码')} name="password" rules={[{ required: true, message: t('请输入密码') }]}>
              <Input.Password autoComplete="current-password" />
            </Form.Item>
            <Button type="primary" htmlType="submit" loading={loggingIn}>{t('登录')}</Button>
          </Form>
        </Card>
      </>
    )
  }

  return (
    <>
      {contextHolder}
      <Card size="small" style={{ marginBottom: 12 }}>
        <Form form={searchForm} layout="inline" onFinish={onSearch}>
          <Form.Item label="ID" name="id">
            <Input placeholder={t('请输入点位 ID')} allowClear style={{ width: 180 }} />
          </Form.Item>
          <Form.Item label={t('描述')} name="description">
            <Input placeholder={t('请输入描述')} allowClear style={{ width: 180 }} />
          </Form.Item>
          <Form.Item>
            <Space>
              <Button type="primary" icon={<SearchOutlined />} htmlType="submit">{t('查询')}</Button>
              <Button onClick={onReset}>{t('重置')}</Button>
            </Space>
          </Form.Item>
        </Form>
      </Card>

      <Card
        size="small"
        title={t('运行状态')}
        extra={(
          <Space>
            <Text type="secondary" style={{ fontSize: 12 }}>{t('系统点位 {{count}} 个', { count: total })}</Text>
            <Button size="small" icon={<ReloadOutlined />} loading={loading} onClick={() => load(page, pageSize)}>
              {t('刷新')}
            </Button>
          </Space>
        )}
      >
        <Table<RunningRow>
          rowKey={record => `${record.id}-${record.timestamp || ''}`}
          size="small"
          loading={loading}
          columns={columns}
          dataSource={rows}
          scroll={{ x: 1000 }}
          pagination={{
            current: page,
            pageSize,
            total,
            showSizeChanger: true,
            pageSizeOptions: [15, 30, 50, 100],
            showTotal: count => t('共 {{count}} 条', { count }),
            onChange: (nextPage, nextPageSize) => load(nextPage, nextPageSize),
          }}
        />
      </Card>
    </>
  )
}
