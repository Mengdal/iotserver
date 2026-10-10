import React, { useCallback, useEffect, useMemo, useState } from 'react'
import {
  Button,
  Card,
  Col,
  DatePicker,
  Empty,
  Form,
  Row,
  Select,
  Space,
  Spin,
  Table,
  Typography,
  message,
} from 'antd'
import {
  DeleteOutlined,
  DownloadOutlined,
  PlusOutlined,
  ReloadOutlined,
  SearchOutlined,
} from '@ant-design/icons'
import type { ColumnsType } from 'antd/es/table'
import { Line } from '@ant-design/plots'
import dayjs from 'dayjs'
// eslint-disable-next-line
import { getDevice, getProductList, useGetProductList } from '@/api'
import { DeviceProperty, ReportDaum, getDeviceProperty, getHistoryProperty } from '@/api/device-thingmodel'
import { DevicesType } from '@/api/devices'
import { useAuth } from '@/utils/auth'
import * as P from '@/routes/permissions'

const { RangePicker } = DatePicker
const { Text, Title } = Typography
const { useMessage } = message

/** 单行查询条件：产品 → 设备 → 参数 */
interface QueryRow {
  key: string
  productId?: string
  deviceId?: string
  code?: string
}

/** 表格行：设备 / 属性 / 值 / 时间 */
interface HistoryRow {
  key: string
  deviceName: string
  propertyName: string
  value: number
  time: number
  unit?: string
}

/** 折线图数据点 */
interface ChartPoint {
  time: string
  value: number
  series: string
}

/** 单次查询最多取多少条（用于画图与导出） */
const MAX_POINTS = 1000

/** 聚合时间选项（对齐参考站：不聚合 + 12 档间隔） */
const AGG_INTERVALS = [
  { label: '不聚合', value: 'none', seconds: 0 },
  { label: '30秒', value: '30s', seconds: 30 },
  { label: '1分钟', value: '1m', seconds: 60 },
  { label: '2分钟', value: '2m', seconds: 120 },
  { label: '5分钟', value: '5m', seconds: 300 },
  { label: '10分钟', value: '10m', seconds: 600 },
  { label: '30分钟', value: '30m', seconds: 1800 },
  { label: '1小时', value: '1h', seconds: 3600 },
  { label: '3小时', value: '3h', seconds: 10800 },
  { label: '6小时', value: '6h', seconds: 21600 },
  { label: '12小时', value: '12h', seconds: 43200 },
  { label: '1天', value: '1d', seconds: 86400 },
]

/** 聚合方法：前 4 种对齐参考站，后 3 种用于文本/枚举型属性 */
const AGG_METHODS = [
  { label: '平均数', value: 'avg' },
  { label: '最大值', value: 'max' },
  { label: '最小值', value: 'min' },
  { label: '求和', value: 'sum' },
  { label: '计数', value: 'count' },
  { label: '首值', value: 'first' },
  { label: '末值', value: 'last' },
]

/** 需要数值型属性的聚合方法 */
const NUMERIC_METHODS = ['avg', 'max', 'min', 'sum']
/** 物模型里属于数值类型的数据类型 */
const NUMERIC_DATA_TYPES = ['int', 'float']

let rowSeq = 0
const newRow = (): QueryRow => ({ key: `row-${++rowSeq}` })

const HistoryQueryPage: React.FC = () => {
  const [messageApi, contextHolder] = useMessage()
  const [form] = Form.useForm()

  const [rows, setRows] = useState<QueryRow[]>([newRow()])
  const [deviceOptions, setDeviceOptions] = useState<Record<string, DevicesType[]>>({})
  const [propertyOptions, setPropertyOptions] = useState<Record<string, DeviceProperty[]>>({})

  const [loading, setLoading] = useState(false)
  // 聚合查询：聚合时间 none = 不聚合；聚合方法仅在聚合时有意义
  const [aggTime, setAggTime] = useState('none')
  const [aggMethod, setAggMethod] = useState('avg')
  const [chartData, setChartData] = useState<ChartPoint[]>([])
  const [tableData, setTableData] = useState<HistoryRow[]>([])
  const [searched, setSearched] = useState(false)

  const { data: productList } = useGetProductList({})
  const canQuery = useAuth([P.DeviceQuery])

  const productOptions = useMemo(
    () => (productList || []).map(item => ({ label: item.name, value: item.id })),
    [productList],
  )

  // 设备下拉：按产品过滤，支持关键字远程搜索
  const loadDevices = useCallback(async (productId?: string, keyword?: string) => {
    const cacheKey = `${productId || 'all'}|${keyword || ''}`
    try {
      const resp = await getDevice({ page: 1, pageSize: 1000, product_id: productId, name: keyword })
      if (resp.success) {
        setDeviceOptions(prev => ({ ...prev, [cacheKey]: resp.result.list || [] }))
      }
    } catch (error) {
      // 忽略：下拉为空即可
    }
  }, [])

  // 参数下拉：取设备物模型属性
  const loadProperties = useCallback(async (deviceId: string) => {
    try {
      const resp = await getDeviceProperty(deviceId)
      if (resp.success) {
        setPropertyOptions(prev => ({ ...prev, [deviceId]: resp.result || [] }))
      }
    } catch (error) {
      // 忽略
    }
  }, [])

  const deviceListOf = useCallback((row: QueryRow) => {
    if (deviceOptions[`${row.productId || 'all'}|`]) return deviceOptions[`${row.productId || 'all'}|`]
    const hit = Object.keys(deviceOptions).find(key => key.startsWith(`${row.productId || 'all'}|`))
    return hit ? deviceOptions[hit] : []
  }, [deviceOptions])

  // 首次加载设备列表
  useEffect(() => {
    loadDevices()
  }, [loadDevices])

  // 已选中属性里只要有一个是数值型，就允许数值聚合方法
  const allowedMethods = useMemo(() => {
    const selected = rows.filter(row => row.deviceId && row.code)
    if (selected.length === 0) return AGG_METHODS // 还没选属性，不做限制
    const anyNumeric = selected.some(row => {
      const property = (propertyOptions[row.deviceId!] || []).find(item => item.code === row.code)
      return NUMERIC_DATA_TYPES.indexOf(String(property?.data_type || '').toLowerCase()) >= 0
    })
    return AGG_METHODS.filter(item => !NUMERIC_METHODS.includes(item.value) || anyNumeric)
  }, [rows, propertyOptions])

  // 换属性后若当前聚合方法不再适用，自动回退到 avg（数值）或 count（文本）
  useEffect(() => {
    if (aggTime === 'none') return
    if (allowedMethods.some(item => item.value === aggMethod)) return
    setAggMethod(allowedMethods.some(item => item.value === 'avg') ? 'avg' : 'count')
  }, [allowedMethods, aggTime, aggMethod])

  /** 当前聚合配置（不聚合时为 null） */
  const currentAgg = useMemo(() => {
    const option = AGG_INTERVALS.find(item => item.value === aggTime)
    if (!option || option.seconds <= 0) return null
    return { seconds: option.seconds, method: aggMethod, label: `${option.label} · ${AGG_METHODS.find(m => m.value === aggMethod)?.label || aggMethod}` }
  }, [aggTime, aggMethod])

  const updateRow = (key: string, patch: Partial<QueryRow>) => {
    setRows(prev => prev.map(row => {
      if (row.key !== key) return row
      const next = { ...row, ...patch }
      // 换产品要清掉设备与参数；换设备要清掉参数
      if (patch.productId !== undefined) {
        next.deviceId = undefined
        next.code = undefined
      }
      if (patch.deviceId !== undefined && patch.deviceId !== row.deviceId) {
        next.code = undefined
      }
      return next
    }))
  }

  const handleAddRow = () => setRows(prev => [...prev, newRow()])

  const handleRemoveRow = (key: string) => {
    setRows(prev => (prev.length <= 1 ? prev : prev.filter(row => row.key !== key)))
  }

  const handleReset = () => {
    form.setFieldValue('range', [])
    setAggTime('none')
    setAggMethod('avg')
    setRows([newRow()])
    setChartData([])
    setTableData([])
    setSearched(false)
  }

  const handleSearch = async () => {
    const validRows = rows.filter(row => row.deviceId && row.code)
    if (validRows.length === 0) {
      messageApi.open({ type: 'warning', content: '请至少选择一组「设备 + 参数」' })
      return
    }
    const [start, end] = form.getFieldValue('range') || []
    if (!start || !end) {
      messageApi.open({ type: 'warning', content: '请选择时间范围' })
      return
    }

    try {
      setLoading(true)
      setSearched(true)
      const points: ChartPoint[] = []
      const tableRows: HistoryRow[] = []

      for (const row of validRows) {
        const device = deviceListOf(row).find(item => item.id === row.deviceId)
        const property = (propertyOptions[row.deviceId!] || []).find(item => item.code === row.code)
        const deviceName = device?.name || row.deviceId!
        const propertyName = property?.name ? `${property.name}` : row.code!
        const series = `${deviceName} / ${propertyName}`

        const resp = await getHistoryProperty<{ list: ReportDaum[], total: number }>(row.deviceId!, {
          code: row.code!,
          start: start.valueOf(),
          end: end.valueOf(),
          page: 1,
          pageSize: MAX_POINTS,
          aggInterval: currentAgg?.seconds,
          aggMethod: currentAgg?.method,
        })
        if (!resp.success) {
          messageApi.open({ type: 'error', content: resp.errorMsg || '查询失败' })
          continue
        }
        const list = resp.result?.list || []
        list.forEach(item => {
          points.push({
            time: dayjs(item.time).format('YYYY-MM-DD HH:mm:ss'),
            value: Number(item.value),
            series,
          })
          tableRows.push({
            key: `${row.key}-${item.time}`,
            deviceName,
            propertyName,
            value: item.value,
            time: item.time,
            unit: property?.unit,
          })
        })
      }

      // 折线图按时间正序
      points.sort((a, b) => (a.time < b.time ? -1 : 1))
      // 表格按时间倒序（最新在前）
      tableRows.sort((a, b) => b.time - a.time)

      setChartData(points)
      setTableData(tableRows)
      if (points.length === 0) {
        messageApi.open({ type: 'info', content: '所选时间范围内没有数据' })
      }
    } finally {
      setLoading(false)
    }
  }

  /** 导出 CSV（带 BOM，Excel 直接打开不乱码） */
  const handleExport = () => {
    if (tableData.length === 0) {
      messageApi.open({ type: 'warning', content: '暂无可导出的数据' })
      return
    }
    const header = ['设备名称', '属性名称', '属性值', '单位', '时间']
    const lines = tableData.map(row => [
      row.deviceName,
      row.propertyName,
      String(row.value),
      row.unit || '',
      dayjs(row.time).format('YYYY-MM-DD HH:mm:ss'),
    ].map(field => `"${String(field).replace(/"/g, '""')}"`).join(','))
    const csv = `\ufeff${[header.join(','), ...lines].join('\n')}`
    const blob = new Blob([csv], { type: 'text/csv;charset=utf-8;' })
    const url = URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = url
    link.download = `历史数据_${currentAgg ? `${currentAgg.label.replace(/ · /g, '')}_` : ''}${dayjs().format('YYYYMMDDHHmmss')}.csv`
    link.click()
    URL.revokeObjectURL(url)
    messageApi.open({ type: 'success', content: `已导出 ${tableData.length} 条` })
  }

  const columns: ColumnsType<HistoryRow> = [
    { title: '设备名称', dataIndex: 'deviceName', key: 'deviceName', width: 220, ellipsis: true },
    { title: '属性名称', dataIndex: 'propertyName', key: 'propertyName', width: 180 },
    {
      title: '属性值',
      dataIndex: 'value',
      key: 'value',
      width: 160,
      render: (value: number, record) => {
        const showUnit = record.unit && currentAgg?.method !== 'count'
        return `${value}${showUnit ? ` ${record.unit}` : ''}`
      },
    },
    {
      title: currentAgg ? '时间（桶起始）' : '时间',
      dataIndex: 'time',
      key: 'time',
      width: 220,
      render: (value: number) => dayjs(value).format('YYYY-MM-DD HH:mm:ss'),
    },
  ]

  const chartConfig = {
    xField: 'time',
    yField: 'value',
    seriesField: 'series',
    smooth: true,
    height: 360,
    legend: { position: 'top' as const },
    // 点很多时限制刻度数量并缩短标签，避免 X 轴糊成一片
    xAxis: {
      tickCount: 8,
      label: {
        autoHide: true,
        formatter: (text: string) => (text ? text.slice(5, 16) : ''),
      },
    },
    yAxis: { nice: true },
    tooltip: { shared: true },
  }

  return (
    <Card className="admin-page">
      {contextHolder}
      <Title level={5} style={{ marginBottom: 4 }}>历史查询</Title>
      <Text type="secondary">
        历史查询功能支持单个设备属性查询，同时也支持多设备不同属性查询用作对比，结果以折线图和表格的形式展示，并且支持导出。
      </Text>
      <div style={{ height: 16 }} />

      <Form form={form} layout="vertical">
        {/* 时间范围与操作按钮同一行 */}
        <Space align="end" size={12} wrap style={{ marginBottom: 12 }}>
          <Form.Item
            name="range"
            label="时间范围"
            initialValue={[dayjs().add(-24, 'hours'), dayjs()]}
            rules={[{ required: true, message: '请选择时间范围' }]}
            style={{ marginBottom: 0 }}
          >
            <RangePicker
              format="YYYY-MM-DD HH:mm:ss"
              showTime={{ hideDisabledOptions: true }}
              style={{ width: 420 }}
              presets={[
                { label: '最近1小时', value: [dayjs().add(-1, 'hours'), dayjs()] },
                { label: '最近24小时', value: [dayjs().add(-24, 'hours'), dayjs()] },
                { label: '最近7天', value: [dayjs().add(-7, 'days'), dayjs()] },
                { label: '最近30天', value: [dayjs().add(-30, 'days'), dayjs()] },
              ]}
            />
          </Form.Item>
          <Form.Item label="聚合时间" className="history-agg-time" style={{ marginBottom: 0 }}>
            <Select
              value={aggTime}
              style={{ width: 110 }}
              options={AGG_INTERVALS.map(item => ({ label: item.label, value: item.value }))}
              onChange={(value) => {
                setAggTime(value)
                // 对齐参考站：切回「不聚合」清空方法；切到具体间隔且方法为空则默认「平均数」
                if (value === 'none') {
                  setAggMethod('avg')
                } else if (!aggMethod) {
                  setAggMethod('avg')
                }
              }}
            />
          </Form.Item>
          {aggTime !== 'none' && (
            <Form.Item label="聚合方法" className="history-agg-method" style={{ marginBottom: 0 }}>
              <Select
                value={aggMethod}
                style={{ width: 110 }}
                options={allowedMethods.map(item => ({ label: item.label, value: item.value }))}
                onChange={(value) => setAggMethod(value)}
              />
            </Form.Item>
          )}
          <Space size={8} style={{ marginBottom: 4 }} wrap>
            <Button type="primary" icon={<SearchOutlined />} loading={loading} disabled={!canQuery} onClick={handleSearch}>
              查询
            </Button>
            <Button icon={<ReloadOutlined />} onClick={handleReset}>重置</Button>
            <Button icon={<DownloadOutlined />} disabled={tableData.length === 0} onClick={handleExport}>导出</Button>
            <Text type="secondary" style={{ fontSize: 12 }}>
              单次最多返回 {MAX_POINTS} {currentAgg ? '个聚合桶/组' : '条/组'}
            </Text>
          </Space>
        </Space>

        {rows.map((row, index) => (
          <Row className="history-query-row" gutter={12} key={row.key} align="bottom" style={{ marginBottom: 8 }}>
            <Col span={6}>
              <Form.Item
                label={index === 0 ? '产品' : ''}
                style={{ marginBottom: 8 }}
              >
                <Select
                  placeholder="请选择产品（可搜索）"
                  allowClear
                  showSearch
                  optionFilterProp="label"
                  value={row.productId}
                  options={productOptions}
                  onChange={(value) => {
                    updateRow(row.key, { productId: value })
                    loadDevices(value)
                  }}
                />
              </Form.Item>
            </Col>
            <Col span={6}>
              <Form.Item label={index === 0 ? '设备' : ''} style={{ marginBottom: 8 }}>
                <Select
                  placeholder="请选择设备（可搜索）"
                  allowClear
                  showSearch
                  optionFilterProp="label"
                  value={row.deviceId}
                  options={deviceListOf(row).map(item => ({ label: item.name, value: item.id }))}
                  onSearch={(keyword) => loadDevices(row.productId, keyword)}
                  onChange={(value) => {
                    updateRow(row.key, { deviceId: value })
                    if (value) loadProperties(value)
                  }}
                />
              </Form.Item>
            </Col>
            <Col span={6}>
              <Form.Item label={index === 0 ? '参数' : ''} style={{ marginBottom: 8 }}>
                <Select
                  placeholder={row.deviceId ? '请选择参数' : '请先选择设备'}
                  allowClear
                  disabled={!row.deviceId}
                  value={row.code}
                  options={(propertyOptions[row.deviceId!] || []).map(item => ({
                    label: `${item.name}（${item.code}）`,
                    value: item.code,
                  }))}
                  onChange={(value) => updateRow(row.key, { code: value })}
                />
              </Form.Item>
            </Col>
            <Col span={6}>
              <Form.Item label={index === 0 ? ' ' : ''} style={{ marginBottom: 8 }}>
                <Space>
                  <Button icon={<PlusOutlined />} onClick={handleAddRow}>添加</Button>
                  {rows.length > 1 && (
                    <Button danger icon={<DeleteOutlined />} onClick={() => handleRemoveRow(row.key)}>删除</Button>
                  )}
                </Space>
              </Form.Item>
            </Col>
          </Row>
        ))}

      </Form>

      <div style={{ height: 16 }} />

      <Spin spinning={loading}>
        <Card
          size="small"
          title={currentAgg ? `趋势对比（${currentAgg.label}）` : '趋势对比'}
          bodyStyle={{ paddingTop: 8 }}
        >
          {chartData.length > 0
            ? <Line data={chartData} {...chartConfig} />
            : <Empty description={searched ? '暂无数据' : '请选择设备与参数后查询'} image={Empty.PRESENTED_IMAGE_SIMPLE} />}
        </Card>

        <div style={{ height: 16 }} />

        <Card
          size="small"
          title={`数据明细${currentAgg ? `（${currentAgg.label}）` : ''}${tableData.length > 0 ? `（共 ${tableData.length} 条）` : ''}`}
        >
          <Table<HistoryRow>
            rowKey="key"
            size="small"
            bordered
            columns={columns}
            dataSource={tableData}
            scroll={{ x: 800 }}
            pagination={{
              size: 'small',
              showQuickJumper: true,
              showSizeChanger: true,
              showTotal: total => `共 ${total} 条`,
              defaultPageSize: 10,
            }}
          />
        </Card>
      </Spin>
    </Card>
  )
}

export default HistoryQueryPage
