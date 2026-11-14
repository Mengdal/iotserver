import dayjs from 'dayjs'
import React, { useCallback, useContext, useEffect, useRef, useState } from 'react'
import {
  Badge,
  Button,
  Card,
  Dropdown,
  Form,
  Input,
  message,
  Modal,
  Popconfirm,
  Select,
  Space,
  Switch,
  Table,
  Tag,
  Tooltip,
  Typography,
} from 'antd'
import { DownOutlined, PlusOutlined, ReloadOutlined } from '@ant-design/icons'
import type { ColumnsType } from 'antd/es/table'
import { WebSocketProvider, WebSocketContext } from '@/components/WebSocket'
import {
  createPluginInstance,
  deletePluginInstance,
  getDeviceServers,
  getPluginInstanceLogs,
  getPluginInstances,
  getPluginPackages,
  getSystemComponentLogs,
  getSystemComponents,
  PluginInstance,
  PluginPackage,
  SystemComponent,
  restartPluginInstance,
  startPluginInstance,
  stopPluginInstance,
} from '@/api'
import LogModal, { LogModalRef } from './LogModal'
import TailLogModal, { TailLogModalRef } from './TailLogModal'
import { EditModal, EditModalRef } from './EditModal'
import { DeviceServers } from '@/api/device-servers'
import { useAuth } from '@/utils/auth'
import { RunStatusMap } from '@/utils/statusMap'
import { t, tt } from '@/i18n'

const { Text } = Typography

type DriverKind = 'docker' | 'native' | 'system'

interface DriverRow {
  key: string
  kind: DriverKind
  name: string
  version: string
  internal?: boolean
  platform?: string
  status: 'running' | 'stopped' | 'error' | 'unknown'
  pid?: number
  address?: string
  lastError?: string
  updated?: number
  docker?: DeviceServers
  native?: PluginInstance
  system?: SystemComponent
}

interface CreateFormValues {
  libraryId: string
  name?: string
  autoStart?: boolean
}

const NATIVE_STATUS: Record<string, { text: string, status: 'success' | 'default' | 'error' | 'warning' }> = {
  running: { text: t('运行中'), status: 'success' },
  stopped: { text: t('停止'), status: 'default' },
  error: { text: t('异常'), status: 'error' },
  unknown: { text: t('未知'), status: 'warning' },
}

const MyDrive: React.FC = () => {
  const { ws } = useContext(WebSocketContext)
  const logModalRef = useRef<LogModalRef>(null)
  const tailLogModalRef = useRef<TailLogModalRef>(null)
  const editModalRef = useRef<EditModalRef>(null)
  const [form] = Form.useForm()
  const [createForm] = Form.useForm<CreateFormValues>()
  const canAdd = useAuth('gateway:drive:add')
  const canEdit = useAuth('gateway:drive:edit')
  const canRemove = useAuth('gateway:drive:remove')

  const [dockerRows, setDockerRows] = useState<DeviceServers[]>([])
  const [nativeRows, setNativeRows] = useState<PluginInstance[]>([])
  const [systemRows, setSystemRows] = useState<SystemComponent[]>([])
  const [packages, setPackages] = useState<PluginPackage[]>([])
  const [loading, setLoading] = useState(false)
  const [actingId, setActingId] = useState('')
  const [keyword, setKeyword] = useState('')

  const [createOpen, setCreateOpen] = useState(false)
  const [creating, setCreating] = useState(false)

  const load = useCallback(async () => {
    setLoading(true)
    try {
      const values = form.getFieldsValue()
      const [dockerResp, nativeResp, packageResp, systemResp] = await Promise.all([
        getDeviceServers({ page: 1, pageSize: 1000, name: values.name || undefined }),
        getPluginInstances(),
        getPluginPackages(),
        getSystemComponents(),
      ])
      if (dockerResp.success) setDockerRows(dockerResp.result?.list || [])
      if (nativeResp.success) setNativeRows(nativeResp.result || [])
      if (packageResp.success) setPackages(packageResp.result || [])
      if (systemResp.success) setSystemRows(systemResp.result || [])
      setKeyword((values.name || '').trim().toLowerCase())
    } finally {
      setLoading(false)
    }
  }, [form])

  useEffect(() => {
    load()
  }, [load])

  const send = useCallback((data: any) => {
    setActingId(data?.data?.id || '')
    return ws?.send(data).then(resp => {
      if (resp.data.success) {
        message.success(resp.data.successMsg)
        load()
      } else {
        message.error(resp.data.errorMsg)
      }
    }).finally(() => {
      setActingId('')
    })
  }, [ws, load])

  const runNativeAction = async (id: string, action: 'start' | 'stop' | 'restart') => {
    setActingId(id)
    try {
      const fn = action === 'start'
        ? startPluginInstance
        : action === 'stop'
          ? stopPluginInstance
          : restartPluginInstance
      const resp = await fn(id)
      if (resp.success) {
        message.success(t('操作成功'))
        load()
      } else {
        message.error(resp.errorMsg)
      }
    } finally {
      setActingId('')
    }
  }

  const openPluginLogs = (record: PluginInstance) => {
    tailLogModalRef.current?.open({
      title: `${t('日志')} · ${record.name}`,
      load: async () => {
        const resp = await getPluginInstanceLogs(record.id, 300)
        if (!resp.success) throw new Error(resp.errorMsg)
        return { lines: resp.result?.lines || [], exists: true, path: '' }
      },
    })
  }

  const openSystemLogs = (record: SystemComponent) => {
    tailLogModalRef.current?.open({
      title: `${t('日志')} · ${record.name}`,
      load: async () => {
        const resp = await getSystemComponentLogs(record.id, 300)
        if (!resp.success) throw new Error(resp.errorMsg)
        return {
          lines: resp.result?.lines || [],
          exists: resp.result?.exists ?? false,
          path: resp.result?.path || record.logPath || '',
        }
      },
    })
  }

  const handleCreate = async () => {
    const values = await createForm.validateFields()
    setCreating(true)
    try {
      const resp = await createPluginInstance({
        libraryId: values.libraryId,
        name: values.name?.trim() || undefined,
        autoStart: values.autoStart !== false,
      })
      if (resp.success) {
        message.success(t('实例创建成功'))
        setCreateOpen(false)
        load()
      } else {
        message.error(resp.errorMsg)
      }
    } finally {
      setCreating(false)
    }
  }

  const handleNativeDelete = async (id: string) => {
    setActingId(id)
    try {
      const resp = await deletePluginInstance(id)
      if (resp.success) {
        message.success(t('删除成功'))
        load()
      } else {
        message.error(resp.errorMsg)
      }
    } finally {
      setActingId('')
    }
  }

  const rows: DriverRow[] = [
    ...dockerRows.map(record => ({
      key: `docker-${record.id}`,
      kind: 'docker' as const,
      name: record.name,
      version: record.deviceLibrary?.version || '',
      internal: record.deviceLibrary?.is_internal,
      platform: record.platform,
      status: record.runStatus === 1 ? 'running' as const : 'stopped' as const,
      docker: record,
    })),
    ...nativeRows
      .filter(record => !keyword || record.name.toLowerCase().includes(keyword))
      .map(record => ({
        key: `native-${record.id}`,
        kind: 'native' as const,
        name: record.name,
        version: record.version,
        status: (record.status === 'running' ? 'running' : record.status === 'error' ? 'error' : record.status === 'unknown' ? 'unknown' : 'stopped') as DriverRow['status'],
        pid: record.pid,
        address: record.baseAddress,
        lastError: record.lastError,
        updated: record.updated,
        native: record,
      })),
    ...systemRows
      .filter(record => !keyword || record.name.toLowerCase().includes(keyword))
      .map(record => ({
        key: `system-${record.id}`,
        kind: 'system' as const,
        name: record.name,
        version: record.version,
        status: (record.status === 'running' ? 'running' : record.status === 'error' ? 'error' : record.status === 'unknown' ? 'unknown' : 'stopped') as DriverRow['status'],
        address: record.endpoint,
        updated: record.updated,
        system: record,
      })),
  ]

  const columns: ColumnsType<DriverRow> = [
    {
      title: t('驱动名称'),
      dataIndex: 'name',
      width: 180,
    },
    {
      title: t('类型'),
      dataIndex: 'kind',
      width: 110,
      render: (value: DriverKind) => {
        // 与驱动市场的类型标签保持一致：协议驱动 / 驱动组件 / 系统组件，
        // 不再把 Docker 当作类型名称展示。
        const label = value === 'system'
          ? t('系统组件')
          : value === 'native'
            ? t('驱动组件')
            : t('协议驱动')
        return <Tag color="default">{label}</Tag>
      },
    },
    {
      title: t('版本'),
      dataIndex: 'version',
      width: 100,
      render: (value: string) => (value === 'builtin' ? t('内置') : value === 'external' ? t('外部') : (value || '-')),
    },
    {
      title: t('状态'),
      dataIndex: 'status',
      width: 100,
      render: (value: DriverRow['status'], record) => {
        if (record.kind === 'native' || record.kind === 'system') {
          const meta = NATIVE_STATUS[value] || NATIVE_STATUS.stopped
          return <Badge status={meta.status} text={meta.text} />
        }
        const meta = RunStatusMap[record.docker?.runStatus ?? 2]
        return <Badge status={meta?.status} text={meta?.text} />
      },
    },
    {
      title: t('地址 / PID'),
      key: 'address',
      width: 200,
      render: (_, record) => record.kind === 'native' || record.kind === 'system'
        ? <span>{record.address || '-'}{record.pid ? ` · PID ${record.pid}` : ''}</span>
        : <Text type="secondary">{tt(record.platform)}</Text>,
    },
    {
      title: t('最近错误'),
      dataIndex: 'lastError',
      ellipsis: true,
      render: (value: string) => value
        ? <Text type="danger" ellipsis={{ tooltip: value }}>{value}</Text>
        : '-',
    },
    {
      title: t('更新时间'),
      dataIndex: 'updated',
      width: 170,
      render: (value: number) => (value ? dayjs(value).format('YYYY-MM-DD HH:mm:ss') : '-'),
    },
    {
      title: t('操作'),
      key: 'action',
      width: 330,
      fixed: 'right',
      render: (_, record) => {
        if (record.kind === 'native') {
          const item = record.native!
          return (
            <Space size={4} wrap={false} style={{ whiteSpace: 'nowrap' }}>
              {canEdit && item.status !== 'running' && (
                <Button type="link" size="small" loading={actingId === item.id} onClick={() => runNativeAction(item.id, 'start')}>
                  {t('启动')}
                </Button>
              )}
              {canEdit && item.status === 'running' && (
                <Button type="link" size="small" loading={actingId === item.id} onClick={() => runNativeAction(item.id, 'stop')}>
                  {t('停止')}
                </Button>
              )}
              {canEdit && (
                <Button type="link" size="small" loading={actingId === item.id} onClick={() => runNativeAction(item.id, 'restart')}>
                  {t('重启')}
                </Button>
              )}
              <Button type="link" size="small" onClick={() => openPluginLogs(item)}>{t('日志')}</Button>
              {canRemove && (
                <Popconfirm
                  title={t('确认删除该插件实例？')}
                  okText={t('确定')}
                  cancelText={t('取消')}
                  onConfirm={() => handleNativeDelete(item.id)}
                >
                  <Button type="link" size="small" danger loading={actingId === item.id}>{t('删除')}</Button>
                </Popconfirm>
              )}
            </Space>
          )
        }
        if (record.kind === 'system') {
          const item = record.system!
          if (!item.logConfigured) {
            return (
              <Tooltip title={t('外部服务，未配置本机日志文件；可在 [ComponentLogs] 中配置路径')}>
                <span>
                  <Button type="link" size="small" disabled>{t('日志')}</Button>
                </span>
              </Tooltip>
            )
          }
          return (
            <Button type="link" size="small" onClick={() => openSystemLogs(item)}>
              {t('日志')}
            </Button>
          )
        }

        const docker = record.docker!
        return (
          <Space split={<Text type="secondary">/</Text>}>
            <Dropdown
              trigger={['click']}
              menu={{
                items: [
                  { label: t('启动'), key: 'start' },
                  { label: t('停止'), key: 'stop' },
                  { label: t('编辑'), key: 'edit' },
                ],
                onClick ({ key }) {
                  if (['start', 'stop'].includes(key)) {
                    send({
                      code: 10002,
                      data: {
                        id: docker.id,
                        run_status: { start: 1, stop: 2 }[key],
                      },
                    })
                  }
                  if (key === 'edit') {
                    editModalRef.current?.open(docker)
                  }
                },
              }}
            >
              <Typography.Link>
                {t('操作')}
                <DownOutlined className="ml5" style={{ fontSize: 9 }} />
              </Typography.Link>
            </Dropdown>
            <Typography.Link onClick={() => logModalRef.current?.open(docker.id)}>{t('日志')}</Typography.Link>
            <Popconfirm
              title={t('此操作将永久删除关联的产品以及设备, 是否继续?')}
              okText={t('确定')}
              cancelText={t('取消')}
              placement="topRight"
              onConfirm={() => send({ code: 10003, data: { id: docker.id } })}
            >
              <Typography.Link>{t('删除')}</Typography.Link>
            </Popconfirm>
          </Space>
        )
      },
    },
  ]

  return (
    <>
      <LogModal ref={logModalRef} />
      <TailLogModal ref={tailLogModalRef} />
      <EditModal ref={editModalRef} onSuccess={load} />
      <Card className="m20" size="small">
        <Form layout="inline" form={form}>
          <Form.Item key="name" name="name">
            <Input placeholder={t('驱动名称')} />
          </Form.Item>
          <Form.Item>
            <Button type="primary" onClick={load}>{t('查询')}</Button>
          </Form.Item>
          <Form.Item>
            <Button type="default" onClick={() => { form.resetFields(); load() }}>{t('重置')}</Button>
          </Form.Item>
        </Form>
      </Card>
      <Card
        className="m20"
        title={t('驱动实例')}
        extra={(
          <Space>
            <Button icon={<ReloadOutlined />} onClick={load} loading={loading}>{t('刷新')}</Button>
            {canAdd && (
              <Button
                type="primary"
                icon={<PlusOutlined />}
                disabled={packages.length === 0}
                onClick={() => {
                  createForm.resetFields()
                  createForm.setFieldsValue({ autoStart: true })
                  setCreateOpen(true)
                }}
              >
                {t('创建原生实例')}
              </Button>
            )}
          </Space>
        )}
      >
        <Table<DriverRow>
          rowKey="key"
          size="small"
          bordered
          loading={loading}
          columns={columns}
          dataSource={rows}
          pagination={{
            size: 'small',
            showSizeChanger: true,
            showTotal: total => t('共 {{count}} 条', { count: total }),
            defaultPageSize: 10,
          }}
          scroll={{ x: 1400 }}
        />
      </Card>

      <Modal
        open={createOpen}
        title={t('创建插件实例')}
        okText={t('确定')}
        cancelText={t('取消')}
        confirmLoading={creating}
        destroyOnClose
        onOk={handleCreate}
        onCancel={() => setCreateOpen(false)}
      >
        <Form form={createForm} layout="vertical" initialValues={{ autoStart: true }}>
          <Form.Item label={t('插件包')} name="libraryId" rules={[{ required: true, message: t('请选择插件包') }]}>
            <Select
              placeholder={t('请选择插件包')}
              options={packages.map(item => ({
                value: item.id,
                label: `${item.name}${item.version ? ` v${item.version}` : ''}`,
              }))}
            />
          </Form.Item>
          <Form.Item label={t('实例名称')} name="name" tooltip={t('留空时使用插件名称')}>
            <Input placeholder={t('留空时使用插件名称')} />
          </Form.Item>
          <Form.Item label={t('创建后自动启动')} name="autoStart" valuePropName="checked">
            <Switch />
          </Form.Item>
        </Form>
      </Modal>

    </>
  )
}

export default () => (
  <WebSocketProvider>
    <MyDrive />
  </WebSocketProvider>
)
