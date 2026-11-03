import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import {
  Alert,
  AutoComplete,
  Button,
  Card,
  Col,
  Descriptions,
  Dropdown,
  Empty,
  Form,
  Input,
  InputNumber,
  Modal,
  Row,
  Segmented,
  Select,
  Space,
  Spin,
  Switch,
  Table,
  Tag,
  Tooltip,
  Tree,
  Upload,
  message,
} from 'antd'
import {
  ApiOutlined,
  AppstoreOutlined,
  ClusterOutlined,
  DeleteOutlined,
  DownloadOutlined,
  EditOutlined,
  HddOutlined,
  InfoCircleOutlined,
  MoreOutlined,
  PlusOutlined,
  ReloadOutlined,
  SaveOutlined,
  UploadOutlined,
} from '@ant-design/icons'
import type { ColumnsType } from 'antd/es/table'
import type { MenuProps } from 'antd'
import dayjs from 'dayjs'
import { getDevice } from '@/api/devices'
import type { DevicesType } from '@/api/devices'
import {
  CollectChannel,
  CollectConfig,
  CollectDevice,
  CollectMonitorStatus,
  CollectMonitorValue,
  CollectMonitorResult,
  CollectPoint,
  bindChannelDevices,
  deleteCollectChannel,
  exportCollectPoints,
  getCollectChannels,
  getCollectConfig,
  getCollectDevices,
  getCollectLatest,
  getCollectTemplate,
  importCollectPoints,
  saveCollectChannel,
  saveCollectConfig,
  unbindChannelDevice,
} from '@/api/collect'
import { t } from '@/i18n'
import './style.less'

const PROTOCOLS = [
  'ModbusTCPClient',
  'ModbusRTUClient',
  'OPCUAClient',
  'IEC104',
  'S7_1200_Network',
  'S7_1500_Network',
  'BACnetIP',
  'DLT645',
]

const CHANNEL_TYPES = [
  { value: 'network', label: t('网口') },
  { value: 'serialport', label: t('串口') },
]

const DATA_TYPES = [
  'bool',
  'float',
  'double',
  'int16',
  'uint16',
  'int32',
  'uint32',
  'int64',
  'uint64',
  'string',
]

const ACCESS_MODES = [
  { value: 'R', label: 'R' },
  { value: 'W', label: 'W' },
  { value: 'RW', label: 'RW' },
]

const DEFAULT_TREE_WIDTH = 320
const MIN_TREE_WIDTH = 220
const MAX_TREE_WIDTH = 520
const TREE_WIDTH_STORAGE_KEY = 'collect-tree-width'

const channelTypeLabel = (value: string) => CHANNEL_TYPES.find(item => item.value === value)?.label || value || '-'

const channelSummary = (channel: CollectChannel) => {
  const cfg = channel.config || {}
  if (channel.channelType === 'network') {
    return [cfg.ip, cfg.port].filter(Boolean).join(':')
  }
  if (channel.channelType === 'serialport') {
    return [cfg.com, cfg.baudRate].filter(Boolean).join(' / ')
  }
  return ''
}

const CollectConfigPage: React.FC = () => {
  const [messageApi, contextHolder] = message.useMessage()

  const [treeWidth, setTreeWidth] = useState(() => {
    const saved = Number(localStorage.getItem(TREE_WIDTH_STORAGE_KEY))
    return saved >= MIN_TREE_WIDTH && saved <= MAX_TREE_WIDTH ? saved : DEFAULT_TREE_WIDTH
  })
  const [channels, setChannels] = useState<CollectChannel[]>([])
  const [selectedChannel, setSelectedChannel] = useState<CollectChannel | null>(null)
  const [collectDevices, setCollectDevices] = useState<CollectDevice[]>([])
  const [deviceLoading, setDeviceLoading] = useState(false)
  const [expandedKeys, setExpandedKeys] = useState<React.Key[]>([])
  const [selectedDevice, setSelectedDevice] = useState<CollectDevice | null>(null)

  const [config, setConfig] = useState<CollectConfig | null>(null)
  const [loading, setLoading] = useState(false)
  const [saving, setSaving] = useState(false)
  const [exporting, setExporting] = useState(false)
  const [viewMode, setViewMode] = useState<'edit' | 'monitor'>('edit')
  const [monitor, setMonitor] = useState<CollectMonitorResult | null>(null)
  const [monitorLoading, setMonitorLoading] = useState(false)
  const [monitorUpdatedAt, setMonitorUpdatedAt] = useState(0)
  const [dirty, setDirty] = useState(false)
  const [showErrors, setShowErrors] = useState(false)

  const [channelModalOpen, setChannelModalOpen] = useState(false)
  const [editingChannel, setEditingChannel] = useState<CollectChannel | null>(null)
  const [channelSaving, setChannelSaving] = useState(false)
  const [channelForm] = Form.useForm()
  const watchedChannelType = Form.useWatch('channelType', channelForm) || 'network'

  const [pickerOpen, setPickerOpen] = useState(false)
  const [pickerChannel, setPickerChannel] = useState<CollectChannel | null>(null)
  const [pickerDevices, setPickerDevices] = useState<DevicesType[]>([])
  const [pickerTotal, setPickerTotal] = useState(0)
  const [pickerPage, setPickerPage] = useState(1)
  const [pickerPageSize, setPickerPageSize] = useState(10)
  const [pickerName, setPickerName] = useState('')
  const [pickerLoading, setPickerLoading] = useState(false)
  const [pickerSelected, setPickerSelected] = useState<React.Key[]>([])
  const confirmingDiscard = useRef(false)

  const refreshChannels = useCallback(async (selectId?: string) => {
    const resp = await getCollectChannels()
    if (!resp.success) {
      messageApi.error(resp.errorMsg)
      return
    }
    const list = resp.result || []
    setChannels(list)
    setSelectedChannel(prev => {
      const wanted = selectId || prev?.id
      return list.find(channel => channel.id === wanted) || list[0] || null
    })
  }, [messageApi])

  const loadCollectDevices = useCallback(async (): Promise<CollectDevice[] | null> => {
    setDeviceLoading(true)
    try {
      const resp = await getCollectDevices()
      if (resp.success) {
        const list = resp.result || []
        setCollectDevices(list)
        return list
      }
      setCollectDevices([])
      messageApi.error(resp.errorMsg)
      return null
    } finally {
      setDeviceLoading(false)
    }
  }, [messageApi])

  useEffect(() => {
    refreshChannels()
    loadCollectDevices()
  }, [loadCollectDevices, refreshChannels])

  const pickerChannelDevices = useMemo(
    () => collectDevices.filter(device => device.channelId === pickerChannel?.id),
    [collectDevices, pickerChannel?.id],
  )

  // 通道列表变化时默认展开所有通道，保留用户手动展开的节点。
  useEffect(() => {
    const keys: React.Key[] = ['root', ...channels.map(channel => `channel:${channel.id}`)]
    setExpandedKeys(prev => Array.from(new Set([...prev, ...keys])))
  }, [channels])

  const loadDeviceConfig = async (device: CollectDevice) => {
    setSelectedDevice(device)
    setLoading(true)
    try {
      const resp = await getCollectConfig(device.iotpId)
      if (resp.success) {
        setConfig(resp.result)
        setDirty(false)
        setShowErrors(false)
      } else {
        setConfig(null)
        messageApi.error(resp.errorMsg)
      }
    } finally {
      setLoading(false)
    }
  }

  /** 切换设备/通道前，有未保存修改时先确认。 */
  const confirmDiscard = (onOk: () => void) => {
    if (!dirty) {
      onOk()
      return
    }
    if (confirmingDiscard.current) return
    confirmingDiscard.current = true
    Modal.confirm({
      title: t('有未保存的修改'),
      content: t('切换后将丢失当前点表的修改，是否继续？'),
      okText: t('继续'),
      cancelText: t('取消'),
      onOk,
      onCancel: () => {
        confirmingDiscard.current = false
      },
      afterClose: () => {
        confirmingDiscard.current = false
      },
    })
  }

  const requestChannelSelect = (channel: CollectChannel) => {
    if (channel.id === selectedChannel?.id && !selectedDevice) return
    confirmDiscard(() => {
      setSelectedChannel(channel)
      setSelectedDevice(null)
      setConfig(null)
      setDirty(false)
      setShowErrors(false)
    })
  }

  const requestDeviceSelect = (device: CollectDevice) => {
    if (device.iotpId === selectedDevice?.iotpId && device.channelId === selectedDevice?.channelId) return
    confirmDiscard(() => {
      const channel = channels.find(item => item.id === device.channelId) || null
      if (channel) setSelectedChannel(channel)
      loadDeviceConfig(device)
    })
  }

  /** 明确的「配置点位」入口：切回编辑模式并加载点表。 */
  const requestDeviceConfig = (device: CollectDevice) => {
    const sameDevice = device.iotpId === selectedDevice?.iotpId && device.channelId === selectedDevice?.channelId
    confirmDiscard(() => {
      const channel = channels.find(item => item.id === device.channelId) || null
      if (channel) setSelectedChannel(channel)
      setViewMode('edit')
      setMonitor(null)
      if (sameDevice && config) return
      loadDeviceConfig(device)
    })
  }

  const handleTreeSelect = (keys: React.Key[]) => {
    const key = String(keys[0] || '')
    if (key.startsWith('device:')) {
      const [, channelIdText, iotpIdText] = key.split(':')
      const channelId = Number(channelIdText)
      const iotpId = Number(iotpIdText)
      const device = collectDevices.find(item => item.channelId === channelId && item.iotpId === iotpId)
      if (device) requestDeviceSelect(device)
      return
    }
    if (key.startsWith('channel:')) {
      const channelId = Number(key.slice('channel:'.length))
      const channel = channels.find(item => item.id === channelId)
      if (channel) requestChannelSelect(channel)
    }
  }

  const patchPoint = (index: number, patch: Partial<CollectPoint>) => {
    setConfig(prev => (prev
      ? { ...prev, points: prev.points.map((point, i) => (i === index ? { ...point, ...patch } : point)) }
      : prev))
    setDirty(true)
  }

  const addPoint = () => {
    setConfig(prev => (prev
      ? {
        ...prev,
        points: [
          ...prev.points,
          {
            tagId: '',
            address: '',
            dataType: 'int16',
            accessMode: 'R',
            multiplier: 1,
            offset: 0,
            storage: true,
            enabled: true,
            sort: prev.points.length,
          },
        ],
      }
      : prev))
    setDirty(true)
  }

  const removePoint = (index: number) => {
    setConfig(prev => (prev ? { ...prev, points: prev.points.filter((_, i) => i !== index) } : prev))
    setDirty(true)
  }

  const pointMissing = (point: CollectPoint) => (
    !point.tagId.trim() || !point.address.trim() || !point.dataType.trim()
  )

  const missingCount = config ? config.points.filter(pointMissing).length : 0
  // 数据少时也铺满一屏，避免卡片过矮显得紧凑。
  const pageBodyMinHeight = 'max(460px, calc(100vh - 350px))'

  const handleGenerateTemplate = async () => {
    if (!config) return
    const resp = await getCollectTemplate(config.deviceId)
    if (!resp.success) {
      messageApi.error(resp.errorMsg)
      return
    }
    const templates = resp.result || []
    const existing = new Set(config.points.map(point => point.tagId))
    const additions = templates
      .filter(point => !existing.has(point.tagId))
      .map((point, index) => ({
        ...point,
        id: undefined,
        sort: config.points.length + index,
      }))
    if (additions.length === 0) {
      messageApi.info(t('没有可补充的物模型点位'))
      return
    }
    setConfig(prev => (prev ? { ...prev, points: [...prev.points, ...additions] } : prev))
    setDirty(true)
    messageApi.success(t('已补充 {{count}} 个点位', { count: additions.length }))
  }

  const handleExport = async () => {
    if (!config) return
    setExporting(true)
    try {
      const resp = await exportCollectPoints(config.deviceId)
      const blob = resp.data as Blob
      const disposition = String(resp.headers?.['content-disposition'] || '')
      const matched = /filename\*?=(?:UTF-8'')?([^;]+)/i.exec(disposition)
      const fileName = matched
        ? decodeURIComponent(matched[1].replace(/"/g, ''))
        : `${config.deviceName || 'points'}.xlsx`
      const url = URL.createObjectURL(blob)
      const link = document.createElement('a')
      link.href = url
      link.download = fileName
      link.style.display = 'none'
      link.click()
      URL.revokeObjectURL(url)
    } catch (error) {
      messageApi.error(t('导出失败'))
    } finally {
      setExporting(false)
    }
  }

  const handleImport = async (file: File): Promise<boolean> => {
    if (!config) return false
    if (!/\.xlsx$/i.test(file.name)) {
      messageApi.error(t('仅支持 .xlsx 文件'))
      return false
    }
    try {
      const resp = await importCollectPoints(config.deviceId, file)
      if (!resp.success) {
        messageApi.error(resp.errorMsg)
        return false
      }
      const points = (resp.result || []).map((point, index) => ({ ...point, sort: index }))
      if (points.length === 0) {
        messageApi.warning(t('Excel 中没有有效点位'))
        return false
      }
      setConfig(prev => (prev ? { ...prev, points } : prev))
      setDirty(true)
      setShowErrors(false)
      messageApi.success(t('已导入 {{count}} 个点位，保存后生效', { count: points.length }))
    } catch (error) {
      messageApi.error(t('导入失败'))
    }
    return false
  }

  const handleSave = async () => {
    if (!config) return
    if (config.points.some(pointMissing)) {
      setShowErrors(true)
      messageApi.error(t('点表还有未填写的必填项，请补全后再保存'))
      return
    }
    const points = config.points.map((point, index) => ({ ...point, sort: index }))
    setSaving(true)
    try {
      const resp = await saveCollectConfig({
        deviceId: config.deviceId,
        enabled: config.enabled,
        points,
      })
      if (resp.success) {
        messageApi.success(t('保存成功'))
        if (selectedDevice) {
          // 重新读取以拿到新点位 ID。
          await loadDeviceConfig(selectedDevice)
        } else {
          setConfig(prev => (prev ? { ...prev, points } : prev))
          setDirty(false)
          setShowErrors(false)
        }
      } else {
        messageApi.error(resp.errorMsg)
      }
    } finally {
      setSaving(false)
    }
  }

  const loadMonitor = useCallback(async (showLoading = false) => {
    if (!config || viewMode !== 'monitor') return
    if (typeof document !== 'undefined' && document.visibilityState === 'hidden') return
    if (showLoading) setMonitorLoading(true)
    try {
      const resp = await getCollectLatest(config.deviceId)
      if (resp.success) {
        setMonitor(resp.result)
        setMonitorUpdatedAt(Date.now())
      } else {
        setMonitor({ available: false, message: resp.errorMsg, items: [] })
      }
    } catch (error) {
      setMonitor({ available: false, message: t('读取实时数据失败'), items: [] })
    } finally {
      if (showLoading) setMonitorLoading(false)
    }
  }, [config, viewMode])

  useEffect(() => {
    if (viewMode !== 'monitor' || !config) return
    setMonitor(null)
    loadMonitor().catch(() => undefined)
    const timer = window.setInterval(() => {
      loadMonitor().catch(() => undefined)
    }, 3000)
    return () => window.clearInterval(timer)
  }, [viewMode, config, loadMonitor])

  const handleViewModeChange = (value: string | number) => {
    const next = value === 'monitor' ? 'monitor' : 'edit'
    if (next === 'monitor' && dirty) {
      messageApi.warning(t('有未保存的修改，请先保存后再切换到监控'))
      return
    }
    setViewMode(next)
  }

  const openChannelModal = (channel?: CollectChannel) => {
    setEditingChannel(channel || null)
    channelForm.resetFields()
    if (channel) {
      const cfg = channel.config || {}
      channelForm.setFieldsValue({
        name: channel.name,
        channelType: channel.channelType,
        protocol: channel.protocol,
        ip: cfg.ip,
        port: cfg.port,
        com: cfg.com,
        baudRate: cfg.baudRate,
        dataBits: cfg.dataBits,
        stopBits: cfg.stopBits,
        parity: cfg.parity,
        pollIntervalMs: channel.pollIntervalMs,
        timeoutMs: channel.timeoutMs,
        enabled: channel.enabled,
      })
    } else {
      channelForm.setFieldsValue({
        name: '',
        channelType: 'network',
        protocol: 'ModbusTCPClient',
        ip: '',
        port: 502,
        com: 'COM1',
        baudRate: 9600,
        dataBits: 8,
        stopBits: 1,
        parity: 'none',
        pollIntervalMs: 1000,
        timeoutMs: 3000,
        enabled: true,
      })
    }
    setChannelModalOpen(true)
  }

  const handleChannelSubmit = async () => {
    const values = await channelForm.validateFields()
    const channelConfig: Record<string, any> = {}
    if (values.channelType === 'network') {
      channelConfig.ip = values.ip || ''
      channelConfig.port = values.port || 0
    } else if (values.channelType === 'serialport') {
      channelConfig.com = values.com || ''
      channelConfig.baudRate = values.baudRate || 0
      channelConfig.dataBits = values.dataBits || 0
      channelConfig.stopBits = values.stopBits || 0
      channelConfig.parity = values.parity || ''
    }
    setChannelSaving(true)
    try {
      const resp = await saveCollectChannel({
        id: editingChannel?.id,
        name: values.name,
        channelType: values.channelType,
        protocol: values.protocol || '',
        config: channelConfig,
        pollIntervalMs: Number(values.pollIntervalMs) || 0,
        timeoutMs: Number(values.timeoutMs) || 0,
        enabled: !!values.enabled,
      })
      if (resp.success) {
        setChannelModalOpen(false)
        // 保持当前选中的通道，避免保存通道时丢掉正在编辑的设备点表。
        await refreshChannels()
        messageApi.success(t('保存成功'))
      } else {
        messageApi.error(resp.errorMsg)
      }
    } finally {
      setChannelSaving(false)
    }
  }

  const handleChannelDelete = async (channel: CollectChannel) => {
    if (!channel.id) return
    const resp = await deleteCollectChannel(channel.id)
    if (!resp.success) {
      messageApi.error(resp.errorMsg)
      return
    }
    if (selectedChannel?.id === channel.id) {
      setSelectedChannel(null)
      setSelectedDevice(null)
      setConfig(null)
      setDirty(false)
      setShowErrors(false)
    }
    messageApi.success(t('删除成功'))
    await refreshChannels()
    await loadCollectDevices()
  }

  const unbindDeviceNow = async (device: CollectDevice) => {
    const resp = await unbindChannelDevice(device.channelId, device.iotpId)
    if (!resp.success) {
      messageApi.error(resp.errorMsg)
      return
    }
    if (selectedDevice?.iotpId === device.iotpId && selectedDevice?.channelId === device.channelId) {
      setSelectedDevice(null)
      setConfig(null)
      setDirty(false)
      setShowErrors(false)
    }
    messageApi.success(t('移除成功'))
    await loadCollectDevices()
  }

  const confirmRemoveDevice = (device: CollectDevice) => {
    Modal.confirm({
      title: t('移除设备会同时删除采集引擎和平台本地的点位，重新添加会重新生成模板，确认？'),
      okText: t('确定'),
      cancelText: t('取消'),
      okButtonProps: { danger: true },
      onOk: () => unbindDeviceNow(device),
    })
  }

  const loadPickerDevices = useCallback(async (nextPage: number, nextPageSize: number, name: string) => {
    setPickerLoading(true)
    try {
      const resp = await getDevice({ page: nextPage, pageSize: nextPageSize, name })
      if (resp.success) {
        setPickerDevices(resp.result.list || [])
        setPickerTotal(resp.result.total || 0)
        setPickerPage(nextPage)
        setPickerPageSize(nextPageSize)
      } else {
        messageApi.error(resp.errorMsg)
      }
    } finally {
      setPickerLoading(false)
    }
  }, [messageApi])

  const openDevicePicker = (channel?: CollectChannel) => {
    const target = channel || selectedChannel
    if (!target?.id) {
      messageApi.warning(t('请先新增通道'))
      return
    }
    setPickerChannel(target)
    setPickerName('')
    setPickerSelected([])
    setPickerDevices([])
    setPickerTotal(0)
    setPickerPage(1)
    setPickerOpen(true)
    loadPickerDevices(1, pickerPageSize, '')
  }

  const handleBindDevices = async () => {
    if (!pickerChannel?.id) return
    const deviceIds = pickerSelected.map(key => String(key))
    if (deviceIds.length === 0) {
      messageApi.warning(t('请选择要添加的设备'))
      return
    }
    const resp = await bindChannelDevices(pickerChannel.id, deviceIds)
    if (!resp.success) {
      messageApi.error(resp.errorMsg)
      return
    }
    setPickerOpen(false)
    messageApi.success(t('保存成功'))
    const list = await loadCollectDevices()
    if (list && selectedDevice) {
      // 若当前设备被移动到别的通道，同步切换左侧选中项并刷新点表。
      const updated = list.find(item => item.iotpId === selectedDevice.iotpId)
      if (updated) {
        const channel = channels.find(item => item.id === updated.channelId) || null
        if (updated.channelId !== selectedDevice.channelId) {
          if (channel) setSelectedChannel(channel)
          setSelectedDevice(updated)
          await loadDeviceConfig(updated)
        } else {
          setSelectedDevice(updated)
        }
      }
    }
  }

  const openChannelDeleteConfirm = (channel: CollectChannel) => {
    Modal.confirm({
      title: t('确认删除该通道？'),
      content: t('删除会直接从采集引擎移除该通道；通道下还有设备时需先移除设备。'),
      okText: t('确定'),
      cancelText: t('取消'),
      okButtonProps: { danger: true },
      onOk: () => handleChannelDelete(channel),
    })
  }

  const toggleChannelExpand = (channel: CollectChannel) => {
    const key = `channel:${channel.id}`
    setExpandedKeys(prev => (prev.includes(key) ? prev.filter(item => item !== key) : [...prev, key]))
  }

  const channelMenu = (channel: CollectChannel): MenuProps => ({
    items: [
      { key: 'add-device', label: t('添加设备'), icon: <PlusOutlined /> },
      { key: 'edit', label: t('编辑通道'), icon: <EditOutlined /> },
      { type: 'divider' },
      { key: 'delete', label: t('删除通道'), danger: true, icon: <DeleteOutlined /> },
    ],
    onClick: ({ key, domEvent }) => {
      domEvent.stopPropagation()
      if (key === 'add-device') openDevicePicker(channel)
      if (key === 'edit') openChannelModal(channel)
      if (key === 'delete') openChannelDeleteConfirm(channel)
    },
  })

  const deviceMenu = (device: CollectDevice): MenuProps => ({
    items: [
      { key: 'config', label: t('配置点位') },
      { type: 'divider' },
      { key: 'remove', label: t('移除设备'), danger: true },
    ],
    onClick: ({ key, domEvent }) => {
      domEvent.stopPropagation()
      if (key === 'config') requestDeviceConfig(device)
      if (key === 'remove') confirmRemoveDevice(device)
    },
  })

  const rootMenu: MenuProps = {
    items: [{ key: 'add-channel', label: t('新增通道'), icon: <PlusOutlined /> }],
    onClick: ({ domEvent }) => {
      domEvent.stopPropagation()
      openChannelModal()
    },
  }

  const treeData = [
    {
      key: 'root',
      selectable: false,
      title: (
        <Dropdown menu={rootMenu} trigger={['contextMenu']}>
          <span style={{ display: 'flex', alignItems: 'center', gap: 6, width: '100%' }}>
            <ApiOutlined />
            <span>{t('数据采集')}</span>
          </span>
        </Dropdown>
      ),
      children: channels.map(channel => {
        const devices = collectDevices.filter(device => device.channelId === channel.id)
        return {
          key: `channel:${channel.id}`,
          title: (
            <Dropdown menu={channelMenu(channel)} trigger={['contextMenu']}>
              <span
                style={{ display: 'flex', alignItems: 'center', gap: 6, width: '100%' }}
                onDoubleClick={event => {
                  event.stopPropagation()
                  toggleChannelExpand(channel)
                }}
              >
                <ClusterOutlined />
                <span>{channel.name}</span>
                <Tag color="default" style={{ marginInlineEnd: 0 }}>{channelTypeLabel(channel.channelType)}</Tag>
                {channelSummary(channel) && (
                  <span style={{ color: '#8a949c', fontSize: 11 }}>{channelSummary(channel)}</span>
                )}
                <Dropdown menu={channelMenu(channel)} trigger={['click']} placement="bottomRight">
                  <Button
                    type="text"
                    size="small"
                    className="collect-node-more"
                    title={t('更多操作')}
                    icon={<MoreOutlined />}
                    style={{ width: 22, height: 22, minWidth: 22, padding: 0, marginLeft: 'auto' }}
                    onClick={event => event.stopPropagation()}
                  />
                </Dropdown>
              </span>
            </Dropdown>
          ),
          children: devices.map(device => ({
            key: `device:${device.channelId}:${device.iotpId}`,
            isLeaf: true,
            title: (
              <Dropdown menu={deviceMenu(device)} trigger={['contextMenu']}>
                <span
                  style={{ display: 'flex', alignItems: 'center', gap: 6, width: '100%' }}
                  onDoubleClick={event => {
                    event.stopPropagation()
                    requestDeviceConfig(device)
                  }}
                >
                  <HddOutlined />
                  <span>{device.deviceName}</span>
                  <Dropdown menu={deviceMenu(device)} trigger={['click']} placement="bottomRight">
                    <Button
                      type="text"
                      size="small"
                      className="collect-node-more"
                      title={t('更多操作')}
                      icon={<MoreOutlined />}
                      style={{ width: 22, height: 22, minWidth: 22, padding: 0, marginLeft: 'auto' }}
                      onClick={event => event.stopPropagation()}
                    />
                  </Dropdown>
                </span>
              </Dropdown>
            ),
          })),
        }
      }),
    },
  ]

  const selectedKeys = selectedDevice
    ? [`device:${selectedDevice.channelId}:${selectedDevice.iotpId}`]
    : selectedChannel?.id
      ? [`channel:${selectedChannel.id}`]
      : []

  const pointColumns: ColumnsType<CollectPoint> = [
    {
      title: t('标签ID'),
      dataIndex: 'tagId',
      width: 140,
      fixed: 'left',
      render: (value: string, _record, index) => (
        <Input
          size="small"
          value={value}
          placeholder="D1.a"
          status={showErrors && !value.trim() ? 'error' : undefined}
          onChange={event => patchPoint(index, { tagId: event.target.value })}
        />
      ),
    },
    {
      title: t('地址'),
      dataIndex: 'address',
      width: 150,
      render: (value: string, _record, index) => (
        <Input
          size="small"
          value={value}
          placeholder="slave=1;addr=400001"
          status={showErrors && !value.trim() ? 'error' : undefined}
          onChange={event => patchPoint(index, { address: event.target.value })}
        />
      ),
    },
    {
      title: t('数据类型'),
      dataIndex: 'dataType',
      width: 88,
      render: (value: string, _record, index) => (
        <AutoComplete
          size="small"
          style={{ width: '100%' }}
          value={value}
          status={showErrors && !value.trim() ? 'error' : undefined}
          options={DATA_TYPES.map(item => ({ value: item }))}
          onChange={next => patchPoint(index, { dataType: next })}
        />
      ),
    },
    {
      title: t('读写'),
      dataIndex: 'accessMode',
      width: 68,
      render: (value: string, _record, index) => (
        <Select
          size="small"
          style={{ width: '100%' }}
          value={value || 'R'}
          options={ACCESS_MODES}
          onChange={next => patchPoint(index, { accessMode: next })}
        />
      ),
    },
    {
      title: t('倍率/偏移'),
      dataIndex: 'multiplier',
      width: 128,
      render: (value: number, record, index) => (
        <Space size={4}>
          <InputNumber
            size="small"
            value={value}
            style={{ width: 54 }}
            onChange={next => patchPoint(index, { multiplier: Number(next ?? 1) })}
          />
          <span>/</span>
          <InputNumber
            size="small"
            value={record.offset}
            style={{ width: 54 }}
            onChange={next => patchPoint(index, { offset: Number(next ?? 0) })}
          />
        </Space>
      ),
    },
    {
      title: t('存储'),
      dataIndex: 'storage',
      width: 60,
      render: (value: boolean, _record, index) => (
        <Switch size="small" checked={value} onChange={next => patchPoint(index, { storage: next })} />
      ),
    },
    {
      title: t('使能'),
      dataIndex: 'enabled',
      width: 50,
      render: (value: boolean, _record, index) => (
        <Switch size="small" checked={value !== false} onChange={next => patchPoint(index, { enabled: next })} />
      ),
    },
    {
      title: t('操作'),
      width: 56,
      fixed: 'right',
      render: (_value, _record, index) => (
        <Button type="text" size="small" icon={<DeleteOutlined />} onClick={() => removePoint(index)} />
      ),
    },
  ]

  const monitorByTag = useMemo(() => {
    const map = new Map<string, CollectMonitorValue>()
    monitor?.items.forEach(item => map.set(item.tagId, item))
    return map
  }, [monitor])

  const monitorStatusOf = (point: CollectPoint): CollectMonitorStatus => (
    monitorByTag.get(point.tagId)?.status || (point.enabled ? 'no_data' : 'disabled')
  )

  const monitorColumns: ColumnsType<CollectPoint> = [
    { title: t('标签ID'), dataIndex: 'tagId', width: 125, fixed: 'left', ellipsis: true },
    {
      title: t('当前值'),
      key: 'value',
      width: 100,
      render: (_value, point) => {
        const item = monitorByTag.get(point.tagId)
        if (!item || item.value === null || item.value === undefined || item.value === '') return '-'
        if (typeof item.value !== 'number') return String(item.value)
        const engineering = item.value * (point.multiplier || 1) + (point.offset || 0)
        return Number.isFinite(engineering) ? String(parseFloat(engineering.toFixed(4))) : String(item.value)
      },
    },
    {
      title: t('状态'),
      key: 'status',
      width: 70,
      render: (_value, point) => {
        switch (monitorStatusOf(point)) {
          case 'normal':
            return <Tag color="green">{t('正常')}</Tag>
          case 'stale':
            return <Tag color="orange">{t('超时')}</Tag>
          case 'disabled':
            return <Tag>{t('未使能')}</Tag>
          default:
            return <Tag>{t('无数据')}</Tag>
        }
      },
    },
    {
      title: t('更新时间'),
      key: 'time',
      width: 120,
      render: (_value, point) => {
        const time = monitorByTag.get(point.tagId)?.time
        return time ? dayjs(time).format('MM-DD HH:mm:ss') : '-'
      },
    },
    { title: t('地址'), dataIndex: 'address', width: 140, ellipsis: true },
    { title: t('数据类型'), dataIndex: 'dataType', width: 70 },
    {
      title: t('使能'),
      dataIndex: 'enabled',
      width: 50,
      render: (value: boolean) => <Tag>{value ? t('是') : t('否')}</Tag>,
    },
  ]

  const channelDevices = selectedChannel
    ? collectDevices.filter(device => device.channelId === selectedChannel.id)
    : []

  const channelDeviceColumns: ColumnsType<CollectDevice> = [
    { title: t('设备名称'), dataIndex: 'deviceName', ellipsis: true },
    { title: t('产品'), dataIndex: 'productName', width: 140, ellipsis: true },
    {
      title: t('操作'),
      width: 96,
      render: (_value, device) => (
        <Button
          type="link"
          size="small"
          style={{ padding: 0 }}
          onClick={() => requestDeviceConfig(device)}
        >
          {t('配置点位')}
        </Button>
      ),
    },
  ]

  const pickerColumns: ColumnsType<DevicesType> = [
    { title: t('设备名称'), dataIndex: 'name', ellipsis: true },
    { title: t('产品'), dataIndex: 'product_name', width: 160, ellipsis: true },
    {
      title: t('状态'),
      dataIndex: 'status',
      width: 90,
      render: (value: string) => <Tag>{value || '-'}</Tag>,
    },
  ]

  const startTreeResize = (event: React.MouseEvent<HTMLDivElement>) => {
    event.preventDefault()
    const startX = event.clientX
    const startWidth = treeWidth
    let nextWidth = startWidth
    const handleMove = (moveEvent: MouseEvent) => {
      nextWidth = Math.min(MAX_TREE_WIDTH, Math.max(MIN_TREE_WIDTH, startWidth + moveEvent.clientX - startX))
      setTreeWidth(nextWidth)
    }
    const handleUp = () => {
      document.removeEventListener('mousemove', handleMove)
      document.removeEventListener('mouseup', handleUp)
      document.body.style.cursor = ''
      document.body.style.userSelect = ''
      localStorage.setItem(TREE_WIDTH_STORAGE_KEY, String(nextWidth))
    }
    document.addEventListener('mousemove', handleMove)
    document.addEventListener('mouseup', handleUp)
    document.body.style.cursor = 'col-resize'
    document.body.style.userSelect = 'none'
  }

  const resetTreeWidth = () => {
    setTreeWidth(DEFAULT_TREE_WIDTH)
    localStorage.setItem(TREE_WIDTH_STORAGE_KEY, String(DEFAULT_TREE_WIDTH))
  }

  return (
    <Card className="admin-page">
      {contextHolder}
      <Row gutter={0} wrap={false} className="collect-layout">
        <Col flex={`0 0 ${treeWidth}px`}>
          <Card
            size="small"
            title={(
              <Tooltip title={t('右键操作，双击展开或配置')}>
                <span style={{ cursor: 'help' }}>
                  {t('数据采集')}
                  <InfoCircleOutlined style={{ marginLeft: 6, color: '#8a949c', fontSize: 11 }} />
                </span>
              </Tooltip>
            )}
            extra={(
              <Space size={4}>
                <Button
                  type="text"
                  size="small"
                  icon={<ReloadOutlined />}
                  title={t('刷新')}
                  onClick={() => {
                    refreshChannels()
                    loadCollectDevices()
                  }}
                />
                <Button size="small" type="primary" icon={<PlusOutlined />} onClick={() => openChannelModal()}>
                  {t('新增通道')}
                </Button>
              </Space>
            )}
            bodyStyle={{ padding: 10, minHeight: pageBodyMinHeight }}
          >
            <Spin spinning={deviceLoading}>
              {channels.length === 0
                ? <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={t('还没有通道')} />
                : (
                  <Tree
                    className="collect-tree"
                    blockNode
                    treeData={treeData}
                    selectedKeys={selectedKeys}
                    expandedKeys={expandedKeys}
                    onExpand={keys => setExpandedKeys(keys)}
                    onSelect={handleTreeSelect}
                  />
                )}
            </Spin>
          </Card>
        </Col>
        <Col
          flex="0 0 12px"
          className="collect-resizer"
          title={t('拖动调整宽度，双击恢复默认')}
          onMouseDown={startTreeResize}
          onDoubleClick={resetTreeWidth}
        />

        <Col flex="1 1 auto">
          {!selectedChannel
            ? (
              <Card size="small" bodyStyle={{ minHeight: pageBodyMinHeight }}>
                <div style={{ margin: 'auto' }}>
                  <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={t('请选择左侧通道')} />
                </div>
              </Card>
            )
            : !selectedDevice
              ? (
                <Card
                  size="small"
                  title={(
                    <Space>
                      <ClusterOutlined />
                      <span>{selectedChannel.name}</span>
                      <Tag color="default">{channelTypeLabel(selectedChannel.channelType)}</Tag>
                    </Space>
                  )}
                  extra={(
                    <Space>
                      <Button size="small" icon={<PlusOutlined />} onClick={() => openDevicePicker(selectedChannel)}>
                        {t('添加设备')}
                      </Button>
                      <Button size="small" icon={<EditOutlined />} onClick={() => openChannelModal(selectedChannel)}>
                        {t('编辑通道')}
                      </Button>
                    </Space>
                  )}
                  bodyStyle={{ minHeight: pageBodyMinHeight }}
                >
                  <Descriptions size="small" column={2} bordered style={{ marginBottom: 16 }}>
                    <Descriptions.Item label={t('协议')}>{selectedChannel.protocol || '-'}</Descriptions.Item>
                    <Descriptions.Item label={t('连接参数')}>{channelSummary(selectedChannel) || '-'}</Descriptions.Item>
                    <Descriptions.Item label={t('轮询周期(ms)')}>{selectedChannel.pollIntervalMs || '-'}</Descriptions.Item>
                    <Descriptions.Item label={t('超时(ms)')}>{selectedChannel.timeoutMs || '-'}</Descriptions.Item>
                    <Descriptions.Item label={t('状态')}>
                      <Tag color={selectedChannel.enabled ? 'green' : 'default'}>
                        {selectedChannel.enabled ? t('启用') : t('停用')}
                      </Tag>
                    </Descriptions.Item>
                  </Descriptions>
                  <div style={{ marginBottom: 8, fontWeight: 600 }}>{t('通道下设备')}</div>
                  <Table<CollectDevice>
                    className="collect-point-table"
                    size="small"
                    bordered
                    rowKey={device => `${device.channelId}:${device.iotpId}`}
                    columns={channelDeviceColumns}
                    dataSource={channelDevices}
                    pagination={false}
                    locale={{ emptyText: t('该通道下还没有设备') }}
                  />
                </Card>
              )
              : !config
                ? (
                  <Card size="small" loading={loading} bodyStyle={{ minHeight: pageBodyMinHeight }}>
                    <div style={{ margin: 'auto' }}>
                      <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={t('正在加载设备配置')} />
                    </div>
                  </Card>
                )
                : (
                  <Card
                    size="small"
                    loading={loading}
                    title={(
                      <Space>
                        <span>{config.deviceName}</span>
                        {config.productName && (
                          <Tag color="default" icon={<AppstoreOutlined />}>{config.productName}</Tag>
                        )}
                        <Tag color="default" icon={<ClusterOutlined />}>
                          {config.channelName || selectedChannel.name}
                        </Tag>
                        <Segmented
                          size="small"
                          value={viewMode}
                          options={[
                            { label: t('编辑'), value: 'edit' },
                            { label: t('监控'), value: 'monitor' },
                          ]}
                          onChange={handleViewModeChange}
                        />
                        {dirty && <Tag color="orange">{t('未保存')}</Tag>}
                      </Space>
                    )}
                    extra={(
                      <Space>
                        <Button
                          size="small"
                          icon={<ReloadOutlined />}
                          loading={viewMode === 'monitor' ? monitorLoading : false}
                          onClick={() => {
                            if (viewMode === 'monitor') {
                              loadMonitor(true).catch(() => undefined)
                            } else if (selectedDevice) {
                              loadDeviceConfig(selectedDevice)
                            }
                          }}
                        >
                          {t('刷新')}
                        </Button>
                        {viewMode === 'edit' && (
                          <Button size="small" type="primary" icon={<SaveOutlined />} loading={saving} onClick={handleSave}>
                            {t('保存')}
                          </Button>
                        )}
                      </Space>
                    )}
                    bodyStyle={{ minHeight: pageBodyMinHeight }}
                  >
                    <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 8 }}>
                      <div>
                        <span style={{ fontWeight: 600, marginRight: 8 }}>
                          {viewMode === 'monitor' ? t('实时值') : t('点表')}
                        </span>
                        <span style={{ color: '#8a949c', fontSize: 11 }}>
                          {t('共 {{count}} 个点位', { count: config.points.length })}
                          {viewMode === 'edit' && missingCount > 0 && (
                            <span style={{ color: showErrors ? '#c0392b' : '#b26a00' }}>
                              {' · '}{t('{{count}} 个待填写', { count: missingCount })}
                            </span>
                          )}
                          {viewMode === 'monitor' && monitorUpdatedAt > 0 && (
                            <span>{' · '}{t('最后刷新 {{time}}', { time: dayjs(monitorUpdatedAt).format('HH:mm:ss') })}</span>
                          )}
                        </span>
                      </div>
                      {viewMode === 'edit'
                        ? (
                          <Space size={4}>
                            <Button size="small" onClick={handleGenerateTemplate}>
                              {t('从物模型生成')}
                            </Button>
                            <Button size="small" icon={<PlusOutlined />} onClick={addPoint}>
                              {t('新增点位')}
                            </Button>
                            <Upload accept=".xlsx" showUploadList={false} beforeUpload={handleImport}>
                              <Button size="small" icon={<UploadOutlined />}>{t('导入')}</Button>
                            </Upload>
                            <Button
                              size="small"
                              icon={<DownloadOutlined />}
                              loading={exporting}
                              onClick={handleExport}
                            >
                              {t('导出')}
                            </Button>
                          </Space>
                        )
                        : (
                          <span style={{ color: '#8a949c', fontSize: 11 }}>{t('每 3 秒自动刷新')}</span>
                        )}
                    </div>
                    {viewMode === 'monitor' && monitor?.message && (
                      <Alert
                        style={{ marginBottom: 8 }}
                        type={monitor.available ? 'info' : 'warning'}
                        showIcon
                        message={t(monitor.message)}
                      />
                    )}
                    <Table<CollectPoint>
                      className="collect-point-table"
                      rowKey={(_record, index) => String(index ?? 0)}
                      size="small"
                      bordered
                      columns={viewMode === 'monitor' ? monitorColumns : pointColumns}
                      dataSource={config.points}
                      pagination={false}
                      scroll={{ x: viewMode === 'monitor' ? 680 : 760 }}
                      rowClassName={(_record, index) => {
                        if (viewMode !== 'edit' || !showErrors) return ''
                        const point = config.points[index ?? 0]
                        return point && pointMissing(point) ? 'collect-row-error' : ''
                      }}
                      locale={{ emptyText: t('还没有点位') }}
                    />
                  </Card>
                )}
        </Col>
      </Row>

      <Modal
        title={editingChannel ? t('编辑通道') : t('新增通道')}
        open={channelModalOpen}
        onCancel={() => setChannelModalOpen(false)}
        onOk={handleChannelSubmit}
        confirmLoading={channelSaving}
        okText={t('确定')}
        cancelText={t('取消')}
        forceRender
      >
        <Form form={channelForm} layout="vertical">
          <Row gutter={12}>
            <Col span={12}>
              <Form.Item
                name="name"
                label={t('通道名称')}
                rules={[{ required: true, message: t('请输入通道名称') }]}
              >
                <Input placeholder="C1" />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="channelType" label={t('通道类型')}>
                <Select options={CHANNEL_TYPES} />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item
                name="protocol"
                label={t('协议')}
                rules={[{ required: true, message: t('请输入协议') }]}
              >
                <AutoComplete options={PROTOCOLS.map(value => ({ value }))} placeholder="ModbusTCPClient" />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="enabled" label={t('启用')} valuePropName="checked">
                <Switch />
              </Form.Item>
            </Col>

            {watchedChannelType === 'network' && (
              <>
                <Col span={12}>
                  <Form.Item
                    name="ip"
                    label={t('IP 地址')}
                    rules={[{ required: true, message: t('请输入 IP 地址') }]}
                  >
                    <Input placeholder="192.168.1.10" />
                  </Form.Item>
                </Col>
                <Col span={12}>
                  <Form.Item
                    name="port"
                    label={t('端口')}
                    rules={[{ required: true, message: t('请输入端口') }]}
                  >
                    <InputNumber style={{ width: '100%' }} min={1} max={65535} />
                  </Form.Item>
                </Col>
              </>
            )}

            {watchedChannelType === 'serialport' && (
              <>
                <Col span={12}>
                  <Form.Item
                    name="com"
                    label={t('串口')}
                    rules={[{ required: true, message: t('请输入串口') }]}
                  >
                    <Input placeholder="COM1" />
                  </Form.Item>
                </Col>
                <Col span={12}>
                  <Form.Item name="baudRate" label={t('波特率')}>
                    <InputNumber style={{ width: '100%' }} min={0} />
                  </Form.Item>
                </Col>
                <Col span={8}>
                  <Form.Item name="dataBits" label={t('数据位')}>
                    <InputNumber style={{ width: '100%' }} min={1} max={8} />
                  </Form.Item>
                </Col>
                <Col span={8}>
                  <Form.Item name="stopBits" label={t('停止位')}>
                    <InputNumber style={{ width: '100%' }} min={1} max={2} />
                  </Form.Item>
                </Col>
                <Col span={8}>
                  <Form.Item name="parity" label={t('校验位')}>
                    <Input placeholder="none" />
                  </Form.Item>
                </Col>
              </>
            )}

            <Col span={12}>
              <Form.Item name="pollIntervalMs" label={t('轮询周期(ms)')}>
                <InputNumber style={{ width: '100%' }} min={0} />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="timeoutMs" label={t('超时(ms)')}>
                <InputNumber style={{ width: '100%' }} min={0} />
              </Form.Item>
            </Col>
          </Row>
        </Form>
      </Modal>

      <Modal
        title={(
          <Space>
            <span>{t('添加设备')}</span>
            {pickerChannel && <Tag color="blue">{pickerChannel.name}</Tag>}
          </Space>
        )}
        open={pickerOpen}
        onCancel={() => setPickerOpen(false)}
        onOk={handleBindDevices}
        okText={t('确定')}
        cancelText={t('取消')}
        width={720}
      >
        <div style={{ marginBottom: 8, color: '#8a949c', fontSize: 12 }}>
          {t('从系统设备中选择并绑定到当前通道；已在其他通道的设备添加后会移动过来')}
        </div>
        <Space.Compact style={{ width: '100%', marginBottom: 8 }}>
          <Input
            placeholder={t('按设备名称搜索')}
            value={pickerName}
            allowClear
            onChange={event => setPickerName(event.target.value)}
            onPressEnter={() => loadPickerDevices(1, pickerPageSize, pickerName)}
          />
          <Button type="primary" onClick={() => loadPickerDevices(1, pickerPageSize, pickerName)}>
            {t('查询')}
          </Button>
        </Space.Compact>
        <Table<DevicesType>
          rowKey="id"
          size="small"
          loading={pickerLoading}
          columns={pickerColumns}
          dataSource={pickerDevices}
          rowSelection={{
            selectedRowKeys: pickerSelected,
            onChange: keys => setPickerSelected(keys),
            getCheckboxProps: record => ({
              disabled: pickerChannelDevices.some(device => device.deviceId === record.id),
            }),
          }}
          pagination={{
            current: pickerPage,
            pageSize: pickerPageSize,
            total: pickerTotal,
            size: 'small',
            showSizeChanger: true,
            showTotal: count => t('共 {{count}} 条', { count }),
            onChange: (nextPage, nextPageSize) => loadPickerDevices(nextPage, nextPageSize, pickerName),
          }}
        />
      </Modal>
    </Card>
  )
}

export default CollectConfigPage
