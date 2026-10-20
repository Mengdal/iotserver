import React, { useCallback, useEffect, useState } from 'react'
import {
  Alert,
  Button,
  Card,
  Col,
  Divider,
  Empty,
  Form,
  Input,
  Modal,
  Spin,
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
  MinusCircleOutlined,
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
  sendGatewayCommand,
  edgeGatewayRestart,
  edgeGatewayControl,
  getEdgeGatewayDevices,
  EdgeDeviceGroup,
  Gateway,
  GatewayMqttInfo,
  GatewayCommandType,
} from '@/api'
import { useAuth } from '@/utils/auth'
import { confirmPlatformViewAdd } from '@/utils/platformViewGuard'
import { t } from '@/i18n'

const { useMessage } = message

interface ControlFormValues {
  items?: Array<{ dn?: string, identifier?: string, value?: string }>
}

/** 参考系统标签树 → 设备分组（dn + 测点） */
function parseEdgeDevices (raw: any): EdgeDeviceGroup[] {
  const groups = new Map<string, EdgeDeviceGroup>()
  const ensure = (dn: string, label?: string) => {
    let group = groups.get(dn)
    if (!group) {
      group = { dn, label: label || dn, tags: [] }
      groups.set(dn, group)
    } else if (label && group.label === group.dn) {
      group.label = label
    }
    return group
  }
  const walk = (node: any, inheritedDevice?: string) => {
    if (!node || typeof node !== 'object') return
    const name = String(node.name || '')
    const id = String(node.id || '')
    let device = inheritedDevice
    if (node.deviceCode) {
      device = String(node.deviceCode)
      ensure(device, name)
    }
    if (id.includes('.')) {
      const [prefix, code] = id.split('.')
      device = device || prefix
      ensure(device)
      const group = ensure(device)
      group.tags.push({ identifier: code, label: node.description || name || code })
    }
    if (Array.isArray(node.children)) {
      const childDevice = node.deviceCode
        ? String(node.deviceCode)
        : (inheritedDevice || (id ? undefined : name) || device)
      node.children.forEach((child: any) => walk(child, childDevice))
    }
  }
  const data = Array.isArray(raw?.data) ? raw.data : (Array.isArray(raw) ? raw : [])
  data.forEach((node: any) => walk(node))
  return Array.from(groups.values()).filter(group => group.tags.length > 0)
}

/** 参考系统网关：配置只能存平台，下发不到真实网关 */
function isEdgeGateway (gateway?: Gateway | null): boolean {
  if (!gateway) return false
  return gateway.protocol === 'edge' || /things|iotddc/i.test(`${gateway.vendor || ''} ${gateway.model || ''}`)
}

const { Text } = Typography

/** 网关状态：0离线 1在线 2从未上线 */
const STATUS_META: Record<number, { text: string, color: string }> = {
  1: { text: t('在线'), color: 'success' },
  0: { text: t('离线'), color: 'default' },
  2: { text: t('从未上线'), color: 'warning' },
}

/** 命令中文名，仅用于提示 */
const COMMAND_LABEL: Record<GatewayCommandType, string> = {
  restart: t('重启网关'),
  sync: t('同步设备'),
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

  // 设备控制（参考系统 /edge 网关）
  const [controlOpen, setControlOpen] = useState(false)
  const [controlTarget, setControlTarget] = useState<Gateway | null>(null)
  const [controlForm] = Form.useForm<ControlFormValues>()
  const [controlDevices, setControlDevices] = useState<EdgeDeviceGroup[]>([])
  const [controlLoading, setControlLoading] = useState(false)
  const [controlSubmitting, setControlSubmitting] = useState(false)
  const [controlResults, setControlResults] = useState<Array<{ ok: boolean, text: string }>>([])
  // 同步设备结果
  const [syncOpen, setSyncOpen] = useState(false)
  const [syncDevices, setSyncDevices] = useState<EdgeDeviceGroup[]>([])

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

  // ===== 设备控制（参考系统 /edge 网关）=====

  const loadControlDevices = async (gatewayId: string) => {
    setControlLoading(true)
    try {
      const resp = await getEdgeGatewayDevices(gatewayId)
      if (resp.success) {
        setControlDevices(parseEdgeDevices(resp.result))
      } else {
        messageApi.open({ type: 'error', content: resp.errorMsg })
      }
    } finally {
      setControlLoading(false)
    }
  }

  const openControl = (record: Gateway) => {
    setControlTarget(record)
    setControlResults([])
    setControlDevices([])
    controlForm.resetFields()
    controlForm.setFieldsValue({ items: [{ value: '' }] })
    setControlOpen(true)
    loadControlDevices(record.id)
  }

  const handleControlSubmit = async () => {
    if (!controlTarget) return
    let values: ControlFormValues
    try {
      values = await controlForm.validateFields()
    } catch (error) {
      return
    }
    const rows = (values.items || []).filter(row => row.dn && row.identifier)
    if (rows.length === 0) {
      messageApi.open({ type: 'warning', content: t('请至少填写一项控制') })
      return
    }
    setControlSubmitting(true)
    const results: Array<{ ok: boolean, text: string }> = []
    try {
      for (const row of rows) {
        const resp = await edgeGatewayControl(controlTarget.id, row.dn!, row.identifier!, row.value || '')
        if (resp.success) {
          const ok = resp.result?.status === true
          results.push({
            ok,
            text: `${row.dn}.${row.identifier}: ${ok ? t('成功') : (resp.result?.message || t('失败'))}`,
          })
        } else {
          results.push({ ok: false, text: `${row.dn}.${row.identifier}: ${resp.errorMsg}` })
        }
      }
      setControlResults(results)
      if (results.every(item => item.ok)) {
        messageApi.open({ type: 'success', content: t('控制指令已全部下发成功') })
      }
    } finally {
      setControlSubmitting(false)
    }
  }

  // ===== 下发命令 =====

  const openCommand = (record: Gateway) => {
    setCommandTarget(record)
    commandForm.resetFields()
    commandForm.setFieldsValue({ command: 'restart' })
    setCommandOpen(true)
  }

  const handleCommandSubmit = async () => {
    if (!commandTarget) return
    const values = await commandForm.validateFields()
    setCommandSubmitting(true)
    try {
      if (values.command === 'sync') {
        if (!isEdgeGateway(commandTarget)) {
          // 自有协议网关仍走原来的 sync 命令
          const resp = await sendGatewayCommand(commandTarget.id, 'sync')
          if (resp.success) {
            messageApi.open({ type: 'success', content: t('已下发「{{p1}}」命令', { p1: COMMAND_LABEL.sync }) })
            setCommandOpen(false)
          } else {
            messageApi.open({ type: 'error', content: resp.errorMsg })
          }
          return
        }
        const resp = await getEdgeGatewayDevices(commandTarget.id)
        if (resp.success) {
          const devices = parseEdgeDevices(resp.result)
          setSyncDevices(devices)
          setSyncOpen(true)
          setCommandOpen(false)
          messageApi.open({ type: 'success', content: t('已同步 {{count}} 台设备', { count: devices.length }) })
        } else {
          messageApi.open({ type: 'error', content: resp.errorMsg })
        }
        return
      }
      // 重启：参考系统走 /edge/restart，自有协议走 iot/down/{sn}/cmd
      const resp = isEdgeGateway(commandTarget)
        ? await edgeGatewayRestart(commandTarget.id)
        : await sendGatewayCommand(commandTarget.id, 'restart')
      if (resp.success) {
        messageApi.open({ type: 'success', content: t('已下发「{{p1}}」命令', { p1: COMMAND_LABEL.restart }) })
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
      width: 340,
      fixed: 'right',
      render: (_, record) => (
        <Space size={4} wrap={false} style={{ whiteSpace: 'nowrap' }}>
          {canEdit && <Button type="link" size="small" onClick={() => openEdit(record)}>{t('编辑')}</Button>}
          {canConfig && isEdgeGateway(record) && <Button type="link" size="small" onClick={() => openControl(record)}>{t('设备控制')}</Button>}
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

      {/* ===== 设备控制（参考系统 /edge）===== */}
      <Modal
        open={controlOpen}
        title={t('设备控制{{p1}}', { p1: controlTarget ? `：${controlTarget.name}` : '' })}
        width={860}
        okText={t('下发')}
        cancelText={t('取消')}
        maskClosable={false}
        confirmLoading={controlSubmitting}
        onOk={handleControlSubmit}
        onCancel={() => setControlOpen(false)}
      >
        <Alert
          type="info"
          showIcon
          style={{ marginBottom: 12 }}
          message={t('选择设备和测点后填写目标值；平台会把控制指令发给网关，并以网关回执为准。')}
        />
        {controlLoading
          ? <div style={{ padding: 32, textAlign: 'center' }}><Spin /></div>
          : controlDevices.length === 0
            ? <Alert type="warning" showIcon message={t('没有从网关拉到设备列表，请先在「下发命令」里执行一次「同步设备」')} />
            : (
              <Form form={controlForm} layout="vertical">
                <Form.List name="items">
                  {(fields, { add, remove }) => (
                    <>
                      {fields.map(({ key, name, ...rest }) => (
                        <Space key={key} align="baseline" style={{ display: 'flex', marginBottom: 4, flexWrap: 'nowrap' }}>
                          <Form.Item {...rest} name={[name, 'dn']} rules={[{ required: true, message: t('请选择设备') }]}>
                            <Select
                              style={{ width: 240 }}
                              showSearch
                              optionFilterProp="label"
                              placeholder={t('设备')}
                              options={controlDevices.map(group => ({ value: group.dn, label: `${group.label} (${group.dn})` }))}
                            />
                          </Form.Item>
                          <Form.Item noStyle shouldUpdate={(prev, cur) => prev.items?.[name]?.dn !== cur.items?.[name]?.dn}>
                            {({ getFieldValue }) => {
                              const dn = getFieldValue(['items', name, 'dn'])
                              const tags = controlDevices.find(group => group.dn === dn)?.tags || []
                              return (
                                <Form.Item {...rest} name={[name, 'identifier']} rules={[{ required: true, message: t('请选择测点') }]}>
                                  <Select
                                    style={{ width: 240 }}
                                    showSearch
                                    optionFilterProp="label"
                                    placeholder={t('测点')}
                                    options={tags.map(tag => ({ value: tag.identifier, label: `${tag.label} (${tag.identifier})` }))}
                                  />
                                </Form.Item>
                              )
                            }}
                          </Form.Item>
                          <Form.Item {...rest} name={[name, 'value']} rules={[{ required: true, message: t('请输入值') }]}>
                            <Input style={{ width: 180 }} placeholder={t('目标值')} />
                          </Form.Item>
                          <MinusCircleOutlined onClick={() => remove(name)} style={{ color: '#999' }} />
                        </Space>
                      ))}
                      <Button type="dashed" block icon={<PlusOutlined />} onClick={() => add({ value: '' })}>
                        {t('添加控制项')}
                      </Button>
                    </>
                  )}
                </Form.List>
              </Form>
            )}
        {controlResults.length > 0 && (
          <div style={{ marginTop: 12 }}>
            <Divider plain style={{ margin: '4px 0 8px' }}>{t('执行结果')}</Divider>
            {controlResults.map((item, index) => (
              <div key={index} style={{ color: item.ok ? '#389e0d' : '#cf1322', fontSize: 13 }}>
                {item.ok ? '✓ ' : '✗ '}{item.text}
              </div>
            ))}
          </div>
        )}
      </Modal>

      {/* ===== 同步设备结果 ===== */}
      <Modal
        open={syncOpen}
        title={t('已同步的设备')}
        width={760}
        footer={null}
        onCancel={() => setSyncOpen(false)}
      >
        <Alert
          type="info"
          showIcon
          style={{ marginBottom: 12 }}
          message={t('设备清单来自网关标签树；平台会继续按该 SN 订阅上报，新设备上报后会自动注册。')}
        />
        {syncDevices.length === 0
          ? <Empty description={t('网关没有返回设备')} />
          : syncDevices.map(group => (
            <Card key={group.dn} size="small" title={`${group.label} (${group.dn})`} style={{ marginBottom: 8 }}>
              <Space wrap>
                {group.tags.map(tag => (
                  <Tag key={tag.identifier}>{tag.label} ({tag.identifier})</Tag>
                ))}
              </Space>
            </Card>
          ))}
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
        <Form form={commandForm} layout="vertical" initialValues={{ command: 'restart' }}>
          <Form.Item
            label={t('命令')}
            name="command"
            rules={[{ required: true, message: t('请选择命令') }]}
          >
            <Select
              options={[
                { label: t('重启网关'), value: 'restart' },
                { label: t('同步设备（拉取网关设备与测点）'), value: 'sync' },
              ]}
            />
          </Form.Item>
          <Alert
            type="info"
            showIcon
            message={t('参考系统网关：重启走 /edge/restart，同步设备会拉取该 SN 下的设备/测点树。')}
          />
        </Form>
      </Modal>
    </Card>
  )
}

export default GatewayManagePage
