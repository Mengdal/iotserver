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
  CollectImportDevicePreview,
  CollectOverview,
  CollectOverviewChannel,
  CollectRealtimeValue,
  CollectImportPreview,
  CollectMonitorStatus,
  CollectMonitorValue,
  CollectMonitorResult,
  CollectPoint,
  bindChannelDevices,
  deleteCollectChannel,
  exportAllCollectPoints,
  getCollectChannels,
  getCollectConfig,
  getCollectOverview,
  getCollectRealtime,
  getCollectDevices,
  getCollectLatest,
  getCollectTemplate,
  importAllCollectPoints,
  previewCollectImport,
  saveCollectChannel,
  setCollectDeviceEnabled,
  writeCollectTag,
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

interface CollectConfigPageProps {
  /** 在网关管理的抽屉里使用时，压缩一屏高度 */
  embedded?: boolean
}

const CollectConfigPage: React.FC<CollectConfigPageProps> = ({ embedded = false }) => {
  const [messageApi, contextHolder] = message.useMessage()

  const [treeWidth, setTreeWidth] = useState(() => {
    const saved = Number(localStorage.getItem(TREE_WIDTH_STORAGE_KEY))
    return saved >= MIN_TREE_WIDTH && saved <= MAX_TREE_WIDTH ? saved : DEFAULT_TREE_WIDTH
  })
  const [channels, setChannels] = useState<CollectChannel[]>([])
  const [selectedChannel, setSelectedChannel] = useState<CollectChannel | null>(null)
  const [overviewSelected, setOverviewSelected] = useState(false)
  const [overview, setOverview] = useState<CollectOverview | null>(null)
  const [overviewLoading, setOverviewLoading] = useState(false)
  const [realtimeValues, setRealtimeValues] = useState<CollectRealtimeValue[]>([])
  const [realtimeLoading, setRealtimeLoading] = useState(false)
  const [realtimeChannelId, setRealtimeChannelId] = useState<number | undefined>()
  const [realtimeDeviceId, setRealtimeDeviceId] = useState<number | undefined>()
  const [realtimeKeyword, setRealtimeKeyword] = useState('')
  const [realtimeKeywordInput, setRealtimeKeywordInput] = useState('')
  const [realtimeRefreshSeq, setRealtimeRefreshSeq] = useState(0)
  const [realtimePage, setRealtimePage] = useState(1)
  const [writePoint, setWritePoint] = useState<CollectPoint | null>(null)
  const [writeValue, setWriteValue] = useState('')
  const [writing, setWriting] = useState(false)
  const [collectDevices, setCollectDevices] = useState<CollectDevice[]>([])
  const [deviceLoading, setDeviceLoading] = useState(false)
  const [expandedKeys, setExpandedKeys] = useState<React.Key[]>([])
  const [selectedDevice, setSelectedDevice] = useState<CollectDevice | null>(null)

  const [config, setConfig] = useState<CollectConfig | null>(null)
  const [loading, setLoading] = useState(false)
  const [saving, setSaving] = useState(false)
  const [exporting, setExporting] = useState(false)
  const bulkImportInputRef = useRef<HTMLInputElement | null>(null)
  const [bulkPreviewOpen, setBulkPreviewOpen] = useState(false)
  const [bulkPreview, setBulkPreview] = useState<CollectImportPreview | null>(null)
  const [bulkFile, setBulkFile] = useState<File | null>(null)
  const [bulkImporting, setBulkImporting] = useState(false)
  const [bulkPreviewLoading, setBulkPreviewLoading] = useState(false)
  const [enablingDeviceId, setEnablingDeviceId] = useState<number | null>(null)
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

  const loadDeviceConfig = async (device: CollectDevice, nextMode?: 'auto' | 'edit') => {
    setSelectedDevice(device)
    setLoading(true)
    try {
      const resp = await getCollectConfig(device.iotpId)
      if (resp.success) {
        setConfig(resp.result)
        if (nextMode === 'edit') {
          setViewMode('edit')
        } else if (nextMode === 'auto') {
          setViewMode(resp.result.configured ? 'monitor' : 'edit')
        }
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

  const loadRealtime = useCallback(async () => {
    if (!overviewSelected) return
    setRealtimeLoading(true)
    try {
      const resp = await getCollectRealtime({
        channelId: realtimeChannelId,
        deviceId: realtimeDeviceId,
        keyword: realtimeKeyword,
      })
      if (resp.success) {
        setRealtimeValues(resp.result || [])
      } else {
        messageApi.error(resp.errorMsg)
      }
    } finally {
      setRealtimeLoading(false)
    }
  }, [overviewSelected, realtimeChannelId, realtimeDeviceId, realtimeKeyword, messageApi])

  useEffect(() => {
    if (!overviewSelected) return
    loadRealtime().catch(() => undefined)
    const timer = window.setInterval(() => {
      loadRealtime().catch(() => undefined)
    }, 3000)
    return () => window.clearInterval(timer)
  }, [overviewSelected, loadRealtime, realtimeRefreshSeq])

  const loadOverview = useCallback(async () => {
    setOverviewLoading(true)
    try {
      const resp = await getCollectOverview()
      if (resp.success) {
        setOverview(resp.result)
      } else {
        messageApi.error(resp.errorMsg)
      }
    } finally {
      setOverviewLoading(false)
    }
  }, [messageApi])

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
      setOverviewSelected(false)
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
      setOverviewSelected(false)
      const channel = channels.find(item => item.id === device.channelId) || null
      if (channel) setSelectedChannel(channel)
      loadDeviceConfig(device, 'auto')
    })
  }

  /** 明确的「配置点位」入口：切回编辑模式并加载点表。 */
  const requestDeviceConfig = (device: CollectDevice) => {
    const sameDevice = device.iotpId === selectedDevice?.iotpId && device.channelId === selectedDevice?.channelId
    confirmDiscard(() => {
      setOverviewSelected(false)
      const channel = channels.find(item => item.id === device.channelId) || null
      if (channel) setSelectedChannel(channel)
      setViewMode('edit')
      setMonitor(null)
      if (sameDevice && config) return
      loadDeviceConfig(device, 'edit')
    })
  }

  const handleTreeSelect = (keys: React.Key[]) => {
    const key = String(keys[0] || '')
    if (key === 'root') {
      confirmDiscard(() => {
        setOverviewSelected(true)
        setSelectedChannel(null)
        setSelectedDevice(null)
        setConfig(null)
        setDirty(false)
        setShowErrors(false)
        loadOverview().catch(() => undefined)
      })
      return
    }
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

  const removePoint = (index: number) => {
    setConfig(prev => (prev ? { ...prev, points: prev.points.filter((_, i) => i !== index) } : prev))
    setDirty(true)
  }

  const pointMissing = (point: CollectPoint) => (
    !point.tagId.trim() || !point.address.trim() || !point.dataType.trim()
  )

  const missingCount = config ? config.points.filter(pointMissing).length : 0
  // 数据少时也铺满一屏，避免卡片过矮显得紧凑。
  // 抽屉里已经去掉了页面级面包屑/标题，留白相应减少。
  const pageBodyMinHeight = embedded
    ? 'max(420px, calc(100vh - 170px))'
    : 'max(460px, calc(100vh - 328px))'

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

  const handleBulkExport = async () => {
    setExporting(true)
    try {
      const resp = await exportAllCollectPoints()
      const blob = resp.data as Blob
      const disposition = String(resp.headers?.['content-disposition'] || '')
      const matched = /filename\*?=(?:UTF-8'')?([^;]+)/i.exec(disposition)
      const fileName = matched
        ? decodeURIComponent(matched[1].replace(/"/g, ''))
        : 'CollectPointsAll.xlsx'
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

  const handleBulkFileChange = async (event: React.ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0]
    event.target.value = ''
    if (!file) return
    if (dirty) {
      messageApi.warning(t('有未保存的修改，请先保存后再批量导入'))
      return
    }
    if (!/\.xlsx$/i.test(file.name)) {
      messageApi.error(t('仅支持 .xlsx 文件'))
      return
    }
    setBulkPreviewLoading(true)
    try {
      const resp = await previewCollectImport(file)
      if (!resp.success) {
        messageApi.error(resp.errorMsg)
        return
      }
      setBulkFile(file)
      setBulkPreview(resp.result)
      setBulkPreviewOpen(true)
    } catch (error) {
      messageApi.error(t('导入失败'))
    } finally {
      setBulkPreviewLoading(false)
    }
  }

  const handleBulkApply = async () => {
    if (!bulkFile) return
    setBulkImporting(true)
    try {
      const resp = await importAllCollectPoints(bulkFile)
      if (!resp.success) {
        messageApi.error(resp.errorMsg)
        return
      }
      const result = resp.result
      const failedIds = new Set(result.failed.map(item => item.deviceId))
      const previewDevices = bulkPreview?.devices || []
      const importedOutside = selectedChannel
        ? previewDevices.filter(device => device.channelId !== selectedChannel.id && !failedIds.has(device.deviceId)).length
        : 0
      setBulkPreviewOpen(false)
      setBulkPreview(null)
      setBulkFile(null)
      if (result.failed.length > 0) {
        Modal.warning({
          title: t('部分设备导入失败'),
          width: 520,
          content: (
            <div>
              <p>{t('成功导入 {{devices}} 台设备、{{points}} 个点位', { devices: result.devices, points: result.points })}</p>
              {importedOutside > 0 && (
                <p>{t('其中 {{count}} 台设备不在当前通道', { count: importedOutside })}</p>
              )}
              <ul style={{ paddingLeft: 18, marginBottom: 0 }}>
                {result.failed.map(item => (
                  <li key={`${item.deviceId}-${item.deviceName}`}>
                    {item.deviceName || `#${item.deviceId}`}：{item.message}
                  </li>
                ))}
              </ul>
            </div>
          ),
        })
      } else if (importedOutside > 0) {
        messageApi.success(t('成功导入 {{devices}} 台设备、{{points}} 个点位，其中 {{outside}} 台不在当前通道', {
          devices: result.devices,
          points: result.points,
          outside: importedOutside,
        }))
      } else {
        messageApi.success(t('成功导入 {{devices}} 台设备、{{points}} 个点位', {
          devices: result.devices,
          points: result.points,
        }))
      }
      refreshChannels()
      loadCollectDevices()
      if (selectedDevice) loadDeviceConfig(selectedDevice)
    } catch (error) {
      messageApi.error(t('导入失败'))
    } finally {
      setBulkImporting(false)
    }
  }

  const handleDeviceEnabledChange = async (device: CollectDevice, enabled: boolean) => {
    setEnablingDeviceId(device.iotpId)
    try {
      const resp = await setCollectDeviceEnabled(device.iotpId, enabled)
      if (!resp.success) {
        messageApi.error(resp.errorMsg)
        return
      }
      messageApi.success(enabled ? t('设备已启用') : t('设备已停用'))
      await loadCollectDevices()
      if (selectedDevice?.iotpId === device.iotpId) {
        await loadDeviceConfig(device)
      }
    } catch (error) {
      messageApi.error(t('操作失败'))
    } finally {
      setEnablingDeviceId(null)
    }
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

  const doBindDevices = async () => {
    if (!pickerChannel?.id) return
    const deviceIds = pickerSelected.map(key => String(key))
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

  const handleBindDevices = () => {
    if (!pickerChannel?.id) return
    const deviceIds = pickerSelected.map(key => String(key))
    if (deviceIds.length === 0) {
      messageApi.warning(t('请选择要添加的设备'))
      return
    }
    const moving = pickerSelected
      .map(key => collectDevices.find(device => device.deviceId === String(key)))
      .filter((device): device is CollectDevice => !!device && device.channelId !== pickerChannel.id)
    if (moving.length === 0) {
      doBindDevices().catch(() => undefined)
      return
    }
    Modal.confirm({
      title: t('设备将移动到当前通道'),
      width: 480,
      okText: t('移动并添加'),
      cancelText: t('取消'),
      content: (
        <div>
          <p>{t('以下设备已在其他通道，添加后会移动到 {{channel}}：', { channel: pickerChannel.name })}</p>
          <ul style={{ paddingLeft: 18 }}>
            {moving.map(device => {
              const oldChannel = channels.find(channel => channel.id === device.channelId)?.name || `#${device.channelId}`
              return (
                <li key={device.iotpId}>
                  {device.deviceName}：{oldChannel} → {pickerChannel.name}
                </li>
              )
            })}
          </ul>
          <p style={{ marginBottom: 0 }}>{t('点位配置会保留，采集引擎里的点位会重新创建。')}</p>
        </div>
      ),
      onOk: doBindDevices,
    })
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
      selectable: true,
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

  const selectedKeys = overviewSelected
    ? ['root']
    : selectedDevice
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
      title: t('原始值'),
      key: 'rawValue',
      width: 90,
      render: (_value, point) => {
        const raw = monitorByTag.get(point.tagId)?.rawValue
        if (raw === null || raw === undefined || raw === '') return '-'
        const numeric = Number(raw)
        return Number.isFinite(numeric) ? String(parseFloat(numeric.toFixed(4))) : String(raw)
      },
    },
    {
      title: t('当前值'),
      key: 'value',
      width: 100,
      render: (_value, point) => {
        const item = monitorByTag.get(point.tagId)
        if (!item || item.value === null || item.value === undefined || item.value === '') return '-'
        // 后端已按点位倍率/偏移换算为工程值，这里直接展示
        const numeric = Number(item.value)
        return Number.isFinite(numeric) ? String(parseFloat(numeric.toFixed(4))) : String(item.value)
      },
    },
    {
      title: t('最后设定值'),
      key: 'lastWriteValue',
      width: 110,
      render: (_value, point) => {
        const item = monitorByTag.get(point.tagId)
        if (!item?.lastWriteValue) return '-'
        return (
          <span title={item.lastWriteTime ? dayjs(item.lastWriteTime).format('YYYY-MM-DD HH:mm:ss') : undefined}>
            {item.lastWriteValue}
          </span>
        )
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
          case 'error':
            return <Tag color="red">{t('采集错误')}</Tag>
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
      title: t('操作'),
      key: 'write',
      width: 60,
      fixed: 'right',
      render: (_value, point) => {
        const writable = ['W', 'RW'].includes((point.accessMode || '').toUpperCase())
        return (
          <Button
            type="link"
            size="small"
            style={{ padding: 0 }}
            disabled={!writable}
            title={writable ? undefined : t('点位只读，不能写入')}
            onClick={() => openWritePoint(point)}
          >
            {t('写入')}
          </Button>
        )
      },
    },
  ]

  const channelDevices = selectedChannel
    ? collectDevices.filter(device => device.channelId === selectedChannel.id)
    : []

  const channelDeviceColumns: ColumnsType<CollectDevice> = [
    { title: t('设备名称'), dataIndex: 'deviceName', ellipsis: true },
    { title: t('设备ID'), dataIndex: 'iotpId', width: 130, ellipsis: true },
    { title: t('产品'), dataIndex: 'productName', width: 140, ellipsis: true },
    { title: t('目标表'), dataIndex: 'measurement', width: 140, ellipsis: true },
    {
      title: t('使能'),
      dataIndex: 'enabled',
      width: 80,
      render: (value: boolean, device) => (
        <Switch
          size="small"
          checked={value !== false}
          loading={enablingDeviceId === device.iotpId}
          onChange={next => handleDeviceEnabledChange(device, next)}
        />
      ),
    },
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
          {t('配置')}
        </Button>
      ),
    },
  ]

  const pickerColumns: ColumnsType<DevicesType> = [
    { title: t('设备名称'), dataIndex: 'name', ellipsis: true },
    { title: t('产品'), dataIndex: 'product_name', width: 140, ellipsis: true },
    {
      title: t('目标表'),
      key: 'measurement',
      width: 140,
      ellipsis: true,
      render: (_value, record) => record.product_id || 'default',
    },
    {
      title: t('当前通道'),
      key: 'boundChannel',
      width: 110,
      render: (_value, record) => {
        const binding = collectDevices.find(device => device.deviceId === record.id)
        if (!binding) return <span style={{ color: '#8a949c' }}>{t('未绑定')}</span>
        const name = channels.find(channel => channel.id === binding.channelId)?.name || `#${binding.channelId}`
        return <Tag color={binding.channelId === pickerChannel?.id ? 'default' : 'orange'}>{name}</Tag>
      },
    },
    {
      title: t('状态'),
      dataIndex: 'status',
      width: 80,
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

  const projectMenu: MenuProps = {
    items: [
      { key: 'import', label: t('导入'), icon: <UploadOutlined /> },
      { key: 'export', label: t('导出'), icon: <DownloadOutlined /> },
    ],
    onClick: ({ key }) => {
      if (key === 'import') bulkImportInputRef.current?.click()
      if (key === 'export') {
        handleBulkExport().catch(() => undefined)
      }
    },
  }

  const openWritePoint = (point: CollectPoint) => {
    if (!config) return
    if (!config.enabled) {
      messageApi.warning(t('设备已停用，无法写入'))
      return
    }
    setWritePoint(point)
    setWriteValue('')
  }

  const handleWritePoint = async () => {
    if (!config || !writePoint) return
    setWriting(true)
    try {
      const resp = await writeCollectTag(config.deviceId, writePoint.tagId, writeValue)
      if (!resp.success) {
        messageApi.error(resp.errorMsg)
        return
      }
      messageApi.success(t('写入请求已提交，最终结果以驱动回调为准'))
      setWritePoint(null)
      loadMonitor(true).catch(() => undefined)
    } finally {
      setWriting(false)
    }
  }

  const overviewStats = [
    { label: t('通道数'), value: overview?.channelCount ?? 0 },
    { label: t('设备数'), value: overview?.deviceCount ?? 0 },
    { label: t('点位数'), value: overview?.pointCount ?? 0 },
    { label: t('停用设备'), value: overview?.disabledDeviceCount ?? 0 },
  ]

  const overviewColumns: ColumnsType<CollectOverviewChannel> = [
    { title: t('通道'), dataIndex: 'name', ellipsis: true },
    { title: t('协议类型'), dataIndex: 'channelType', width: 90, render: (value: string) => channelTypeLabel(value) },
    { title: t('协议'), dataIndex: 'protocol', width: 150, ellipsis: true },
    { title: t('设备数'), dataIndex: 'deviceCount', width: 80 },
    { title: t('点位数'), dataIndex: 'pointCount', width: 80 },
    {
      title: t('状态'),
      dataIndex: 'enabled',
      width: 80,
      render: (value: boolean) => (
        <Tag color={value ? 'green' : 'default'}>{value ? t('启用') : t('停用')}</Tag>
      ),
    },
    {
      title: t('操作'),
      width: 90,
      render: (_value, record) => (
        <Button
          type="link"
          size="small"
          style={{ padding: 0 }}
          onClick={() => {
            const channel = channels.find(item => item.id === record.id)
            if (channel) requestChannelSelect(channel)
          }}
        >
          {t('查看通道')}
        </Button>
      ),
    },
  ]

  const realtimeStatusTag = (status: CollectMonitorStatus) => {
    switch (status) {
      case 'normal':
        return <Tag color="green">{t('正常')}</Tag>
      case 'stale':
        return <Tag color="orange">{t('超时')}</Tag>
      case 'error':
        return <Tag color="red">{t('采集错误')}</Tag>
      case 'disabled':
        return <Tag>{t('未使能')}</Tag>
      default:
        return <Tag>{t('无数据')}</Tag>
    }
  }

  const realtimeColumns: ColumnsType<CollectRealtimeValue> = [
    { title: t('通道'), dataIndex: 'channelName', width: 70 },
    { title: t('设备'), dataIndex: 'deviceName', width: 100, ellipsis: true },
    { title: t('标签ID'), dataIndex: 'tagId', width: 160, ellipsis: true },
    {
      title: t('当前值'),
      dataIndex: 'value',
      width: 90,
      render: (value: string) => {
        if (!value) return '-'
        const numeric = Number(value)
        return Number.isFinite(numeric) ? String(parseFloat(numeric.toFixed(4))) : value
      },
    },
    {
      title: t('状态'),
      dataIndex: 'status',
      width: 70,
      render: (status: CollectMonitorStatus) => realtimeStatusTag(status),
    },
    {
      title: t('更新时间'),
      dataIndex: 'time',
      width: 130,
      render: (time: number) => (time ? dayjs(time).format('MM-DD HH:mm:ss') : '-'),
    },
  ]

  const bulkCrossChannelDevices = (bulkPreview?.devices || []).filter(
    device => selectedChannel && device.channelId !== selectedChannel.id,
  )

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
                  {t('配置工程')}
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
                  title={t('刷新通道和设备列表')}
                  onClick={() => {
                    refreshChannels()
                    loadCollectDevices()
                  }}
                />
                <Dropdown menu={projectMenu} trigger={['click']} placement="bottomRight">
                  <Button
                    type="text"
                    size="small"
                    icon={<MoreOutlined />}
                    title={t('更多操作')}
                    loading={bulkPreviewLoading || exporting}
                  />
                </Dropdown>
                <input
                  id="collect-bulk-import"
                  ref={bulkImportInputRef}
                  type="file"
                  accept=".xlsx"
                  style={{ display: 'none' }}
                  onChange={handleBulkFileChange}
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
          {overviewSelected
            ? (
              <Card
                size="small"
                loading={overviewLoading}
                title={t('采集概览')}
                extra={(
                  <Button
                    size="small"
                    icon={<ReloadOutlined />}
                    onClick={() => {
                      loadOverview().catch(() => undefined)
                      loadRealtime().catch(() => undefined)
                    }}
                  >
                    {t('刷新')}
                  </Button>
                )}
                bodyStyle={{ minHeight: pageBodyMinHeight }}
              >
                <Row gutter={12} style={{ marginBottom: 16 }}>
                  {overviewStats.map(item => (
                    <Col span={6} key={item.label}>
                      <div style={{ background: '#f7f9fc', borderRadius: 6, padding: '12px 14px' }}>
                        <div style={{ color: '#8a949c', fontSize: 12 }}>{item.label}</div>
                        <div style={{ fontSize: 24, fontWeight: 600, marginTop: 4 }}>{item.value}</div>
                      </div>
                    </Col>
                  ))}
                </Row>
                <div style={{ marginBottom: 8, fontWeight: 600 }}>{t('通道明细')}</div>
                <Table<CollectOverviewChannel>
                  className="collect-point-table"
                  size="small"
                  bordered
                  rowKey="id"
                  columns={overviewColumns}
                  dataSource={overview?.channels || []}
                  pagination={false}
                  locale={{ emptyText: t('还没有通道') }}
                />
                <div style={{ margin: '24px 0 8px', fontWeight: 600 }}>{t('实时数据')}</div>
                <Space wrap style={{ marginBottom: 12 }}>
                  <Select
                    allowClear
                    placeholder={t('全部通道')}
                    style={{ width: 150 }}
                    value={realtimeChannelId}
                    options={channels.map(channel => ({ value: channel.id, label: channel.name }))}
                    onChange={value => {
                      setRealtimeChannelId(value)
                      setRealtimeDeviceId(undefined)
                      setRealtimePage(1)
                    }}
                  />
                  <Select
                    allowClear
                    placeholder={t('全部设备')}
                    style={{ width: 170 }}
                    value={realtimeDeviceId}
                    options={collectDevices
                      .filter(device => !realtimeChannelId || device.channelId === realtimeChannelId)
                      .map(device => ({ value: device.iotpId, label: device.deviceName }))}
                    onChange={value => {
                      setRealtimeDeviceId(value)
                      setRealtimePage(1)
                    }}
                  />
                  <Input
                    allowClear
                    placeholder={t('设备或标签关键字')}
                    style={{ width: 200 }}
                    value={realtimeKeywordInput}
                    onChange={event => {
                      setRealtimeKeywordInput(event.target.value)
                      setRealtimePage(1)
                    }}
                    onPressEnter={() => {
                      setRealtimeKeyword(realtimeKeywordInput)
                      setRealtimeRefreshSeq(value => value + 1)
                      setRealtimePage(1)
                    }}
                  />
                  <Button onClick={() => {
                    setRealtimeKeyword(realtimeKeywordInput)
                    setRealtimeRefreshSeq(value => value + 1)
                    setRealtimePage(1)
                  }}>
                    {t('查询')}
                  </Button>
                  <span style={{ color: '#8a949c', fontSize: 11 }}>{t('每 3 秒自动刷新')}</span>
                </Space>
                <Table<CollectRealtimeValue>
                  className="collect-point-table"
                  size="small"
                  bordered
                  rowKey={record => `${record.iotpId}:${record.tagId}`}
                  loading={realtimeLoading}
                  columns={realtimeColumns}
                  dataSource={realtimeValues}
                  pagination={{
                    current: realtimePage,
                    pageSize: 10,
                    showSizeChanger: false,
                    showTotal: count => t('共 {{count}} 条', { count }),
                    onChange: page => setRealtimePage(page),
                  }}
                  locale={{ emptyText: t('暂无实时数据') }}
                  scroll={{ x: 640 }}
                />
              </Card>
            )
            : !selectedChannel
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
                      </Space>
                    )}
                    bodyStyle={{ minHeight: pageBodyMinHeight }}
                  >
                    <div style={{ marginBottom: 8, fontWeight: 600 }}>{t('通道详情')}</div>
                    <Descriptions size="small" column={2} bordered style={{ marginBottom: 16 }}>
                      <Descriptions.Item label={t('协议类型')}>
                        {channelTypeLabel(selectedChannel.channelType)}
                      </Descriptions.Item>
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
                    <div style={{ marginBottom: 8, fontWeight: 600 }}>{t('通道设备')}</div>
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
                            title={viewMode === 'monitor' ? t('立即刷新实时值') : t('刷新当前设备点表')}
                            loading={viewMode === 'monitor' ? monitorLoading : false}
                            onClick={() => {
                              if (viewMode === 'monitor') {
                                loadMonitor(true).catch(() => undefined)
                              } else if (selectedDevice) {
                                if (dirty) {
                                  confirmDiscard(() => {
                                    loadDeviceConfig(selectedDevice)
                                  })
                                } else {
                                  loadDeviceConfig(selectedDevice)
                                }
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
                        scroll={{ x: viewMode === 'monitor' ? 900 : 700 }}
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
          {t('已在其他通道的设备会移动到当前通道，点位配置保留；当前通道下的设备不可重复添加')}
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

      <Modal
        title={t('写入点位')}
        open={!!writePoint}
        centered
        width={420}
        okText={t('写入')}
        cancelText={t('取消')}
        confirmLoading={writing}
        onOk={handleWritePoint}
        onCancel={() => {
          if (!writing) setWritePoint(null)
        }}
      >
        {writePoint && (
          <Input
            size="large"
            autoFocus
            value={writeValue}
            onChange={event => setWriteValue(event.target.value)}
            placeholder={t('输入写入值')}
          />
        )}
      </Modal>

      <Modal
        title={t('批量导入点位')}
        open={bulkPreviewOpen}
        width={640}
        okText={t('覆盖导入')}
        cancelText={t('取消')}
        confirmLoading={bulkImporting}
        okButtonProps={{
          disabled: !bulkPreview || bulkPreview.devices.length === 0 || bulkPreview.errors.length > 0,
        }}
        onOk={handleBulkApply}
        onCancel={() => {
          if (bulkImporting) return
          setBulkPreviewOpen(false)
          setBulkPreview(null)
          setBulkFile(null)
        }}
      >
        {bulkPreview && (
          <>
            {bulkPreview.errors.length > 0 && (
              <Alert
                type="error"
                showIcon
                style={{ marginBottom: 12 }}
                message={t('有 {{count}} 条校验错误，修正后重新导入', { count: bulkPreview.errors.length })}
                description={(
                  <div style={{ maxHeight: 140, overflow: 'auto' }}>
                    {bulkPreview.errors.map(item => (
                      <div key={`${item.row}-${item.message}`}>
                        {t('第 {{row}} 行：{{message}}', { row: item.row, message: item.message })}
                      </div>
                    ))}
                  </div>
                )}
              />
            )}
            {bulkCrossChannelDevices.length > 0 && (
              <Alert
                type="warning"
                showIcon
                style={{ marginBottom: 12 }}
                message={t('文件包含其他通道设备，确认后也会一起导入')}
                description={bulkCrossChannelDevices.map(device => (
                  `${device.channelName || `#${device.channelId}`} / ${device.deviceName}`
                )).join('、')}
              />
            )}
            <div style={{ marginBottom: 8 }}>
              {t('共 {{devices}} 台设备、{{points}} 个点位', {
                devices: bulkPreview.devices.length,
                points: bulkPreview.devices.reduce((sum, item) => sum + item.points, 0),
              })}
            </div>
            <Table<CollectImportDevicePreview>
              size="small"
              bordered
              rowKey="deviceId"
              pagination={false}
              dataSource={bulkPreview.devices}
              locale={{ emptyText: t('文件中没有有效点位') }}
              columns={[
                { title: t('通道'), dataIndex: 'channelName', width: 100 },
                { title: t('设备名称'), dataIndex: 'deviceName' },
                { title: t('设备ID'), dataIndex: 'deviceId', width: 130 },
                { title: t('点位数量'), dataIndex: 'points', width: 90 },
              ]}
            />
            <div style={{ color: '#8a949c', fontSize: 11, marginTop: 8 }}>
              {t('导入会覆盖文件中设备的点位，未出现的设备保持不变')}
            </div>
          </>
        )}
      </Modal>
    </Card>
  )
}

export default CollectConfigPage
