import React, { useCallback, useEffect, useState } from 'react'
import {
  Button,
  Card,
  Col,
  DatePicker,
  Form,
  Input,
  Popconfirm,
  Row,
  Space,
  Table,
  Tag,
  message,
} from 'antd'
import { DeleteOutlined, ReloadOutlined, SearchOutlined } from '@ant-design/icons'
import type { ColumnsType } from 'antd/es/table'
import dayjs from 'dayjs'
import { getSysLoginLogs, deleteSysLoginLog, clearSysLoginLog, SysLoginLog } from '@/api'
import { useAuth } from '@/utils/auth'

const { useMessage } = message
const { RangePicker } = DatePicker

const SysLoginLogPage: React.FC = () => {
  const [messageApi, contextHolder] = useMessage()
  const [form] = Form.useForm()

  const [loading, setLoading] = useState(false)
  const [dataSource, setDataSource] = useState<SysLoginLog[]>([])
  const [pagination, setPagination] = useState({ current: 1, pageSize: 10 })
  const [total, setTotal] = useState(0)
  const [selectedRowKeys, setSelectedRowKeys] = useState<React.Key[]>([])

  const canRemove = useAuth('system:loginlog:remove')

  const loadData = useCallback(async (page = pagination.current, pageSize = pagination.pageSize) => {
    try {
      setLoading(true)
      const values = await form.validateFields()
      const params: Record<string, any> = {
        username: values.username,
        ip: values.ip,
        status: values.status,
        page,
        pageSize,
      }
      if (Array.isArray(values.time) && values.time.length === 2) {
        params.startTime = values.time[0].valueOf()
        params.endTime = values.time[1].valueOf()
      }
      const resp = await getSysLoginLogs(params)
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

  useEffect(() => {
    loadData(1, pagination.pageSize)
  }, [])

  const handleDelete = async (ids: string[]) => {
    const resp = await deleteSysLoginLog(ids)
    if (resp.success) {
      messageApi.open({ type: 'success', content: '删除成功' })
      setSelectedRowKeys([])
      loadData()
    } else {
      messageApi.open({ type: 'error', content: resp.errorMsg })
    }
  }

  const handleClear = async () => {
    const resp = await clearSysLoginLog()
    if (resp.success) {
      messageApi.open({ type: 'success', content: '清空成功' })
      setSelectedRowKeys([])
      loadData()
    } else {
      messageApi.open({ type: 'error', content: resp.errorMsg })
    }
  }

  const columns: ColumnsType<SysLoginLog> = [
    { title: '登录账号', dataIndex: 'username', width: 160 },
    { title: '登录IP', dataIndex: 'ip', width: 160, render: (value: string) => value || '-' },
    { title: '浏览器', dataIndex: 'browser', width: 140, render: (value: string) => value || '-' },
    { title: '操作系统', dataIndex: 'os', width: 140, render: (value: string) => value || '-' },
    {
      title: '状态',
      dataIndex: 'status',
      width: 100,
      render: (value: number) => (value === 1 ? <Tag color="success">成功</Tag> : <Tag color="error">失败</Tag>),
    },
    { title: '提示消息', dataIndex: 'message', ellipsis: true, render: (value: string) => value || '-' },
    {
      title: '登录时间',
      dataIndex: 'loginTime',
      width: 190,
      render: (value: number) => (value ? dayjs(value).format('YYYY-MM-DD HH:mm:ss') : '-'),
    },
  ]

  return (
    <Card className="admin-page">
      {contextHolder}
      <Form form={form} layout="inline" style={{ marginBottom: 16, rowGap: 8 }}>
        <Form.Item name="username" label="登录账号">
          <Input placeholder="请输入登录账号" allowClear style={{ width: 160 }} />
        </Form.Item>
        <Form.Item name="ip" label="登录IP">
          <Input placeholder="请输入登录IP" allowClear style={{ width: 160 }} />
        </Form.Item>
        <Form.Item name="status" label="状态">
          <Input placeholder="1 成功 / 0 失败" allowClear style={{ width: 140 }} />
        </Form.Item>
        <Form.Item name="time" label="登录时间">
          <RangePicker showTime style={{ width: 360 }} />
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

      <Row style={{ marginBottom: 12 }}>
        <Col>
          <Space>
            {canRemove && (
              <Popconfirm
                title={`确认删除选中的 ${selectedRowKeys.length} 条日志？`}
                onConfirm={() => handleDelete(selectedRowKeys as string[])}
                okText="确定"
                cancelText="取消"
                disabled={selectedRowKeys.length === 0}
              >
                <Button danger icon={<DeleteOutlined />} disabled={selectedRowKeys.length === 0}>批量删除</Button>
              </Popconfirm>
            )}
            {canRemove && (
              <Popconfirm title="确认清空全部登入日志？" onConfirm={handleClear} okText="确定" cancelText="取消">
                <Button danger>清空日志</Button>
              </Popconfirm>
            )}
          </Space>
        </Col>
      </Row>

      <Table<SysLoginLog>
        rowKey="id"
        size="small"
        bordered
        loading={loading}
        columns={columns}
        dataSource={dataSource}
        scroll={{ x: 1100 }}
        rowSelection={{ selectedRowKeys, onChange: keys => setSelectedRowKeys(keys) }}
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
    </Card>
  )
}

export default SysLoginLogPage
