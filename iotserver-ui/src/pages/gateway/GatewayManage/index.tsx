import React, { useCallback, useEffect, useState } from 'react'
import {
  Alert,
  Button,
  Card,
  Col,
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
  Typography,
  message,
} from 'antd'
import {
  CopyOutlined,
  EyeInvisibleOutlined,
  EyeTwoTone,
  PlusOutlined,
  ReloadOutlined,
  SearchOutlined,
} from '@ant-design/icons'
import type { ColumnsType } from 'antd/es/table'
import dayjs from 'dayjs'
import {
  getGateways,
  addGateway,
  updateGateway,
  deleteGateway,
  pushGatewayConfig,
  sendGatewayCommand,
  Gateway,
  GatewayMqttInfo,
  GatewayCommandType,
} from '@/api'
import { useAuth } from '@/utils/auth'
import { confirmPlatformViewAdd } from '@/utils/platformViewGuard'
import { t } from '@/i18n'

const { useMessage } = message
const { Text } = Typography

/** 网关状态：0离线 1在线 2从未上线 */
const STATUS_META: Record<number, { text: string, color: string }> = {
  1: { text: t('在线'), color: 'success' },
  0: { text: t('离线'), color: 'default' },
  2: { text: t('从未上线'), color: 'warning' },
}

/** 命令中文名，仅用于提示 */
const COMMAND_LABEL: Record<GatewayCommandType, string> = {
  restart: '重启网关',
  ota: '固件升级（OTA）',
  sync: '同步配置/设备',
}

const formatTime = (value?: number) => (value ? dayjs(value).format('YYYY-MM-DD HH:mm:ss') : '-')

/** 复制文本：优先 Clipboard API，非安全上下文回落到临时 textarea */
const copyText = async (text: string): Promise<boolean> => {
  try {
    if (navigator.clipboard && window.isSecureContext) {
      await navigator.clipboard.writeText(text)
      return true
    }
  } catch (error) {
    // 忽略，走下面的兜底方案
  }
  try {
    const el = document.createElement('textarea')
    el.value = text
    el.style.position = 'fixed'
    el.style.opacity = '0'
    document.body.appendChild(el)
    el.select()
    const ok = document.execCommand('copy')
    document.body.removeChild(el)
    return ok
  } catch (error) {
    return false
  }
}

interface GatewayFormValues {
  name: string
  sn?: string
  vendor?: string
  model?: string
  remark?: string
}

interface CommandFormValues {
  command: GatewayCommandType
  payload?: string
}

const GatewayManagePage: React.FC = () => {
  const [messageApi, contextHolder] = useMessage()
  const [searchForm] = Form.useForm()
  const [editForm] = Form.useForm<GatewayFormValues>()
  const [commandForm] = Form.useForm<CommandFormValues>()

  const [loading, setLoading] = useState(false)
  const [dataSource, setDataSource] = useState<Gateway[]>([])
  const [pagination, setPagination] = useState({ current: 1, pageSize: 10 })
  const [total, setTotal] = useState(0)

  const [modalOpen, setModalOpen] = useState(false)
  const [editing, setEditing] = useState<Gateway | null>(null)
  const [submitting, setSubmitting] = useState(false)

  // 新增成功后的一次性接入信息
  const [mqttOpen, setMqttOpen] = useState(false)
  const [mqttInfo, setMqttInfo] = useState<GatewayMqttInfo | null>(null)
  const [mqttGatewayName, setMqttGatewayName] = useState('')
  const [passwordVisible, setPasswordVisible] = useState(false)

  // 下发配置
  const [configOpen, setConfigOpen] = useState(false)
  const [configTarget, setConfigTarget] = useState<Gateway | null>(null)
  const [configText, setConfigText] = useState('')
  const [configError, setConfigError] = useState('')
  const [configSubmitting, setConfigSubmitting] = useState(false)

  // 下发命令
  const [commandOpen, setCommandOpen] = useState(false)
  const [commandTarget, setCommandTarget] = useState<Gateway | null>(null)
  const [commandSubmitting, setCommandSubmitting] = useState(false)

  const canList = useAuth('gateway:gateway:list')
  const canAdd = useAuth('gateway:gateway:add')
  const canEdit = useAuth('gateway:gateway:edit')
  const canRemove = useAuth('gateway:gateway:remove')
  const canConfig = useAuth('gateway:gateway:config')
  const canCommand = useAuth('gateway:gateway:command')

  const loadData = useCallback(async (page = pagination.current, pageSize = pagination.pageSize) => {
    try {
      setLoading(true)
      const values = searchForm.getFieldsValue()
      const resp = await getGateways({
        name: values.name || undefined,
        sn: values.sn || undefined,
        // 未选择状态时显式传 -1（后端约定：非 0/1/2 即不过滤）。
        // 不能"不下传"：后端收到缺省值 0 会当成"只看离线"，从未上线(2)的网关会看不到。
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

  const handleCopy = async (text: string, label: string) => {
    const ok = await copyText(text || '')
    if (ok) {
      messageApi.open({ type: 'success', content: t('{{label}}已复制', { label: label }) })
    } else {
      messageApi.open({ type: 'error', content: t('复制失败，请手动选中复制') })
    }
  }

  // ===== 新增 / 编辑 =====

  const openAdd = async () => {
    if (!await confirmPlatformViewAdd()) return
    setEditing(null)
    editForm.resetFields()
    setModalOpen(true)
  }

  const openEdit = (record: Gateway) => {
    setEditing(record)
    editForm.resetFields()
    editForm.setFieldsValue({
      sn: record.sn,
      name: record.name,
      vendor: record.vendor,
      model: record.model,
      remark: record.remark,
    })
    setModalOpen(true)
  }

  const handleSubmit = async () => {
    const values = await editForm.validateFields()
    const payload: Record<string, any> = {
      name: (values.name || '').trim(),
      vendor: values.vendor || '',
      model: values.model || '',
      remark: values.remark || '',
    }
    setSubmitting(true)
    try {
      if (editing) {
        // SN 创建后不可修改，更新时不提交 sn
        const resp = await updateGateway({ ...payload, id: editing.id, deptId: editing.deptId })
        if (resp.success) {
          messageApi.open({ type: 'success', content: t('修改成功') })
          setModalOpen(false)
          loadData()
        } else {
          messageApi.open({ type: 'error', content: resp.errorMsg })
        }
      } else {
        const resp = await addGateway({ ...payload, sn: (values.sn || '').trim() })
        if (resp.success) {
          messageApi.open({ type: 'success', content: t('新增成功') })
          setModalOpen(false)
          loadData()
          // 接入信息（含密码）只在这次响应里返回，必须立刻提示用户保存
          if (resp.result?.mqtt) {
            setMqttInfo(resp.result.mqtt)
            setMqttGatewayName(resp.result.name || payload.name)
            setPasswordVisible(false)
            setMqttOpen(true)
          }
        } else {
          messageApi.open({ type: 'error', content: resp.errorMsg })
        }
      }
    } finally {
      setSubmitting(false)
    }
  }

  const handleDelete = async (id: string) => {
    const resp = await deleteGateway(id)
    if (resp.success) {
      messageApi.open({ type: 'success', content: t('删除成功') })
      loadData()
    } else {
      messageApi.open({ type: 'error', content: resp.errorMsg })
    }
  }

  // ===== 下发配置 =====

  const openConfig = (record: Gateway) => {
    setConfigTarget(record)
    setConfigError('')
    let text = record.config || ''
    if (text) {
      // 能解析就格式化，便于编辑；解析失败按原样展示，提交时再报错
      try {
        text = JSON.stringify(JSON.parse(text), null, 2)
      } catch (error) {
        text = record.config || ''
      }
    }
    setConfigText(text)
    setConfigOpen(true)
  }

  const handleConfigSubmit = async () => {
    if (!configTarget) return
    const text = configText.trim()
    if (!text) {
      setConfigError(t('配置内容不能为空'))
      return
    }
    try {
      JSON.parse(text)
    } catch (error) {
      setConfigError(t('不是合法的 JSON：{{message}}', { message: (error as Error).message }))
      return
    }
    setConfigError('')
    setConfigSubmitting(true)
    try {
      const resp = await pushGatewayConfig(configTarget.id, text)
      if (resp.success) {
        messageApi.open({
          type: 'success',
          content: t('配置已下发，新版本号：{{p1}}', { p1: resp.result?.configVersion ?? '-' }),
        })
        setConfigOpen(false)
        loadData()
      } else {
        messageApi.open({ type: 'error', content: resp.errorMsg })
      }
    } finally {
      setConfigSubmitting(false)
    }
  }

  // ===== 下发命令 =====

  const openCommand = (record: Gateway) => {
    setCommandTarget(record)
    commandForm.resetFields()
    commandForm.setFieldsValue({ command: 'restart', payload: '' })
    setCommandOpen(true)
  }

  const handleCommandSubmit = async () => {
    if (!commandTarget) return
    const values = await commandForm.validateFields()
    const payload = (values.payload || '').trim()
    if (values.command === 'ota' && payload) {
      try {
        JSON.parse(payload)
      } catch (error) {
        messageApi.open({ type: 'error', content: t('payload 不是合法的 JSON：{{message}}', { message: (error as Error).message }) })
        return
      }
    }
    setCommandSubmitting(true)
    try {
      const resp = await sendGatewayCommand(commandTarget.id, values.command, payload || undefined)
      if (resp.success) {
        messageApi.open({ type: 'success', content: t('已下发「{{p1}}」命令', { p1: COMMAND_LABEL[values.command] }) })
        setCommandOpen(false)
      } else {
        messageApi.open({ type: 'error', content: resp.errorMsg })
      }
    } finally {
      setCommandSubmitting(false)
    }
  }

  const columns: ColumnsType<Gateway> = [
    { title: t('网关名称'), dataIndex: 'name', width: 180, render: (value: string) => value || '-' },
    { title: 'SN', dataIndex: 'sn', width: 170, render: (value: string) => <Text copyable={{ text: value }}>{value || '-'}</Text> },
    {
      title: t('厂商/型号'),
      key: 'vendorModel',
      width: 180,
      render: (_, record) => {
        const text = [record.vendor, record.model].filter(Boolean).join(' / ')
        return text || '-'
      },
    },
    {
      title: t('所属'),
      key: 'dept',
      width: 150,
      render: (_, record) => record.deptName || record.deptId || '-',
    },
    {
      title: t('状态'),
      dataIndex: 'status',
      width: 100,
      render: (value: number) => {
        const meta = STATUS_META[value] || { text: t('未知'), color: 'default' }
        return <Tag color={meta.color}>{meta.text}</Tag>
      },
    },
    {
      title: t('最近上报时间'),
      dataIndex: 'lastReportAt',
      width: 180,
      render: (value: number) => formatTime(value),
    },
    { title: t('设备数'), dataIndex: 'deviceCount', width: 90, render: (value: number) => value ?? 0 },
    {
      title: t('配置版本'),
      dataIndex: 'configVersion',
      width: 100,
      render: (value: number) => (value ? `v${value}` : '-'),
    },
    {
      title: t('操作'),
      key: 'action',
      width: 250,
      fixed: 'right',
      render: (_, record) => (
        <Space size={4} wrap>
          {canEdit && <Button type="link" size="small" onClick={() => openEdit(record)}>{t('编辑')}</Button>}
          {canConfig && <Button type="link" size="small" onClick={() => openConfig(record)}>{t('下发配置')}</Button>}
          {canCommand && <Button type="link" size="small" onClick={() => openCommand(record)}>{t('下发命令')}</Button>}
          {canRemove && (
            <Popconfirm
              title={t('确认删除该网关？删除后该网关的设备绑定关系不会级联清除。')}
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
        <Empty description={t('无网关列表查看权限')} />
      </Card>
    )
  }

  return (
    <Card className="admin-page">
      {contextHolder}
      <Form form={searchForm} layout="inline" style={{ marginBottom: 16, rowGap: 8 }}>
        <Form.Item name="name" label={t('网关名称')}>
          <Input placeholder={t('请输入网关名称')} allowClear style={{ width: 180 }} />
        </Form.Item>
        <Form.Item name="sn" label="SN">
          <Input placeholder={t('请输入 SN')} allowClear style={{ width: 180 }} />
        </Form.Item>
        <Form.Item name="status" label={t('状态')}>
          <Select placeholder={t('全部')} allowClear style={{ width: 140 }}>
            <Select.Option value={1}>{t('在线')}</Select.Option>
            <Select.Option value={0}>{t('离线')}</Select.Option>
            <Select.Option value={2}>{t('从未上线')}</Select.Option>
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
            {canAdd && <Button type="primary" icon={<PlusOutlined />} onClick={openAdd}>{t('新增网关')}</Button>}
          </Space>
        </Col>
      </Row>

      <Table<Gateway>
        rowKey="id"
        size="small"
        bordered
        loading={loading}
        columns={columns}
        dataSource={dataSource}
        scroll={{ x: 1500 }}
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

      {/* ===== 新增 / 编辑 ===== */}
      <Modal
        open={modalOpen}
        title={editing ? t('编辑网关') : t('新增网关')}
        width={640}
        okText={t('确定')}
        cancelText={t('取消')}
        maskClosable={false}
        confirmLoading={submitting}
        onOk={handleSubmit}
        onCancel={() => setModalOpen(false)}
      >
        <Form form={editForm} layout="vertical">
          <Row gutter={12}>
            <Col span={12}>
              <Form.Item
                label={t('网关名称')}
                name="name"
                rules={[{ required: true, message: t('请输入网关名称') }]}
              >
                <Input placeholder={t('请输入网关名称')} />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item
                label="SN"
                name="sn"
                tooltip={t('网关唯一序列号，同时作为 MQTT 客户端 ID，创建后不可修改')}
                rules={editing ? [] : [{ required: true, message: t('请输入 SN') }]}
              >
                <Input placeholder={t('请输入网关 SN')} disabled={!!editing} />
              </Form.Item>
            </Col>
          </Row>
          <Row gutter={12}>
            <Col span={12}>
              <Form.Item label={t('厂商')} name="vendor">
                <Input placeholder={t('如 华为 / 有人 / 研华')} />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item label={t('型号')} name="model">
                <Input placeholder={t('请输入型号')} />
              </Form.Item>
            </Col>
          </Row>
          <Form.Item label={t('备注')} name="remark">
            <Input.TextArea rows={2} placeholder={t('请输入备注')} />
          </Form.Item>
        </Form>
      </Modal>

      {/* ===== 接入信息（仅新增后展示一次） ===== */}
      <Modal
        open={mqttOpen}
        title={t('接入信息{{p1}}', { p1: mqttGatewayName ? `：${mqttGatewayName}` : '' })}
        width={720}
        okText={t('我已保存')}
        maskClosable={false}
        onOk={() => setMqttOpen(false)}
        onCancel={() => setMqttOpen(false)}
        footer={[
          <Button key="ok" type="primary" onClick={() => setMqttOpen(false)}>{t('我已保存')}</Button>,
        ]}
      >
        <Alert
          type="warning"
          showIcon
          style={{ marginBottom: 16 }}
          message={t('密钥仅显示一次，请立即保存到网关')}
          description={t('关闭本窗口后无法再次查看该密码，如已遗失需在网关侧重新生成密钥。')}
        />
        {mqttInfo && (
          <>
            {([
              { label: t('接入地址（broker）'), value: mqttInfo.broker },
              { label: t('客户端 ID（clientId）'), value: mqttInfo.clientId },
              { label: t('用户名（username）'), value: mqttInfo.username },
            ] as Array<{ label: string, value: string }>).map(item => (
              <Row key={item.label} gutter={8} align="middle" style={{ marginBottom: 8 }}>
                <Col span={7}><Text type="secondary">{item.label}</Text></Col>
                <Col span={14}><Text copyable={{ text: item.value || '' }}>{item.value || '-'}</Text></Col>
                <Col span={3}>
                  <Button size="small" icon={<CopyOutlined />} onClick={() => handleCopy(item.value, item.label)}>{t('复制')}</Button>
                </Col>
              </Row>
            ))}
            <Row gutter={8} align="middle" style={{ marginBottom: 8 }}>
              <Col span={7}><Text type="secondary">{t('密码（password）')}</Text></Col>
              <Col span={14}>
                <Text code>
                  {passwordVisible ? (mqttInfo.password || '-') : '••••••••'}
                </Text>
              </Col>
              <Col span={3}>
                <Space size={4}>
                  <Button
                    size="small"
                    type="text"
                    icon={passwordVisible ? <EyeTwoTone /> : <EyeInvisibleOutlined />}
                    onClick={() => setPasswordVisible(v => !v)}
                  />
                  <Button size="small" icon={<CopyOutlined />} onClick={() => handleCopy(mqttInfo.password, t('密码'))}>{t('复制')}</Button>
                </Space>
              </Col>
            </Row>
            {([
              { label: t('上行 Topic'), value: mqttInfo.topicUp, hint: t('网关上报数据 / 批量数据 / 状态 / 离线') },
              { label: t('下行 Topic'), value: mqttInfo.topicDown, hint: t('平台下发配置 / 命令') },
            ] as Array<{ label: string, value: string, hint: string }>).map(item => (
              <Row key={item.label} gutter={8} align="middle" style={{ marginBottom: 8 }}>
                <Col span={7}>
                  <Text type="secondary">{item.label}</Text>
                </Col>
                <Col span={14}>
                  <Text code copyable={{ text: item.value || '' }}>{item.value || '-'}</Text>
                  <div><Text type="secondary" style={{ fontSize: 12 }}>{item.hint}</Text></div>
                </Col>
                <Col span={3}>
                  <Button size="small" icon={<CopyOutlined />} onClick={() => handleCopy(item.value, item.label)}>{t('复制')}</Button>
                </Col>
              </Row>
            ))}
          </>
        )}
      </Modal>

      {/* ===== 下发配置 ===== */}
      <Modal
        open={configOpen}
        title={t('下发配置{{p1}}', { p1: configTarget ? `：${configTarget.name}` : '' })}
        width={720}
        okText={t('下发')}
        cancelText={t('取消')}
        maskClosable={false}
        confirmLoading={configSubmitting}
        onOk={handleConfigSubmit}
        onCancel={() => setConfigOpen(false)}
      >
        <Alert
          type="info"
          showIcon
          style={{ marginBottom: 12 }}
          message={t('当前配置版本：{{p1}}，下发成功后版本号 +1', { p1: configTarget?.configVersion ? `v${configTarget.configVersion}` : t('无') })}
        />
        <Input.TextArea
          rows={12}
          value={configText}
          onChange={e => {
            setConfigText(e.target.value)
            if (configError) setConfigError('')
          }}
          placeholder={t('请输入 JSON 配置，例如 {"interval":10}')}
          style={{ fontFamily: 'monospace' }}
        />
        {configError && <Text type="danger">{configError}</Text>}
      </Modal>

      {/* ===== 下发命令 ===== */}
      <Modal
        open={commandOpen}
        title={t('下发命令{{p1}}', { p1: commandTarget ? `：${commandTarget.name}` : '' })}
        width={560}
        okText={t('下发')}
        cancelText={t('取消')}
        maskClosable={false}
        confirmLoading={commandSubmitting}
        onOk={handleCommandSubmit}
        onCancel={() => setCommandOpen(false)}
      >
        <Form form={commandForm} layout="vertical" initialValues={{ command: 'restart', payload: '' }}>
          <Form.Item
            label={t('命令')}
            name="command"
            rules={[{ required: true, message: t('请选择命令') }]}
          >
            <Select
              options={[
                { label: t('重启网关（restart）'), value: 'restart' },
                { label: t('固件升级（ota）'), value: 'ota' },
                { label: t('同步配置/设备（sync）'), value: 'sync' },
              ]}
            />
          </Form.Item>
          <Form.Item noStyle shouldUpdate={(prev, cur) => prev.command !== cur.command}>
            {({ getFieldValue }) => {
              // 只有 OTA 需要额外填 payload（可空），其余命令不带参数
              if (getFieldValue('command') !== 'ota') return null
              return (
                <Form.Item
                  label={t('payload（JSON，可空）')}
                  name="payload"
                  tooltip={t('如固件下载地址等参数；留空则不带参数下发')}
                >
                  <Input.TextArea rows={5} placeholder={t('可空，例如 {"url":"http://example.com/fw.bin","version":"1.2.0"}')} />
                </Form.Item>
              )
            }}
          </Form.Item>
        </Form>
      </Modal>
    </Card>
  )
}

export default GatewayManagePage
