import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { Button, Input, Select, Tooltip, Typography } from 'antd'
import { DeleteOutlined, PlusOutlined } from '@ant-design/icons'
import { debounce } from 'lodash'
import { getDevice } from '@/api/devices'
import { getDeviceProperty, DeviceProperty } from '@/api/device-thingmodel'
import { getDeviceDetail } from '@/api/device'
import type { VizSeriesRef } from '@/api'
import { SettingRow, SETTINGS_ITEM_LABEL_WIDTH } from './SettingsLayout'

const { Text } = Typography

export interface SeriesEditorProps {
  value: VizSeriesRef[]
  onChange: (next: VizSeriesRef[]) => void
  /** 最多几条（后端限制 20） */
  max?: number
  /** 只允许一条：仪表盘、数值卡这类"单值"组件用 */
  single?: boolean
  disabled?: boolean
}

interface Option { label: string; value: string }

/**
 * 数据序列编辑器：设备 + 属性（+ 可选图例名）。
 *
 * 所有数据类组件共用这一个编辑器 —— 与 OpenRemote 里"图表/仪表盘都选 AttributeRef"等价，
 * 只是把 assetId+attributeName 换成了 deviceId + 属性 code。
 */
const SeriesEditor: React.FC<SeriesEditorProps> = ({ value, onChange, max = 20, single, disabled }) => {
  const series = value || []
  const [devices, setDevices] = useState<Option[]>([])
  const [props, setProps] = useState<Record<string, DeviceProperty[]>>({})
  const loadingProps = useRef<Record<string, boolean>>({})
  // 下拉加载态：不显示的话，拉取期间的下拉是空的，看起来像"没有数据"
  const [devicesLoading, setDevicesLoading] = useState(true)
  const [propsLoading, setPropsLoading] = useState<Record<string, boolean>>({})

  const mergeDevices = useCallback((list: Option[]) => {
    setDevices((prev) => {
      const map = new Map(prev.map(o => [o.value, o]))
      list.forEach(o => map.set(o.value, o))
      return Array.from(map.values())
    })
  }, [])

  const fetchDevices = useCallback((keyword: string) => {
    setDevicesLoading(true)
    getDevice({ page: 1, pageSize: 50, name: keyword || undefined })
      .then((resp) => {
        if (!resp.success) return
        const list = (resp.result && resp.result.list) || []
        mergeDevices(list.map(d => ({ label: d.name, value: d.id })))
      })
      .catch(() => { /* 拉不到就让用户手动搜 */ })
      .finally(() => setDevicesLoading(false))
  }, [mergeDevices])

  // 首次进来先铺一批设备，避免下拉是空的
  useEffect(() => { fetchDevices('') }, [fetchDevices])

  // 已选设备可能不在前 50 个里：补上名字，否则下拉只显示一串 id
  useEffect(() => {
    const missing = series
      .map(s => s.deviceId)
      .filter(id => id && !devices.some(d => d.value === id))
    if (missing.length === 0) return
    missing.forEach((id) => {
      getDeviceDetail(id)
        .then((resp: any) => {
          const name = resp && resp.success && resp.result ? resp.result.name : ''
          if (name) mergeDevices([{ label: name, value: id }])
        })
        .catch(() => { /* 设备可能已被删除，忽略 */ })
    })
  }, [series, devices, mergeDevices])

  const searchDevices = useMemo(() => debounce((kw: string) => fetchDevices(kw), 400), [fetchDevices])
  useEffect(() => () => searchDevices.cancel(), [searchDevices])

  const loadProps = useCallback((deviceId: string) => {
    if (!deviceId || props[deviceId] || loadingProps.current[deviceId]) return
    loadingProps.current[deviceId] = true
    setPropsLoading(prev => ({ ...prev, [deviceId]: true }))
    getDeviceProperty(deviceId)
      .then((resp) => {
        if (!resp.success) return
        setProps(prev => ({ ...prev, [deviceId]: resp.result || [] }))
      })
      .catch(() => { /* 设备没上报过属性时为空 */ })
      .finally(() => {
        loadingProps.current[deviceId] = false
        setPropsLoading(prev => ({ ...prev, [deviceId]: false }))
      })
  }, [props])

  // 已有的序列进来就把属性列表拉好（编辑旧看板时不用挨个点一遍）
  useEffect(() => {
    series.forEach((s) => { if (s.deviceId) loadProps(s.deviceId) })
  }, [series, loadProps])

  const patch = (index: number, next: Partial<VizSeriesRef>) => {
    onChange(series.map((s, i) => (i === index ? { ...s, ...next } : s)))
  }

  const remove = (index: number) => {
    onChange(series.filter((_, i) => i !== index))
  }

  const add = () => {
    const next = series.concat([{ deviceId: '', code: '' }])
    onChange(single ? [next[next.length - 1]] : next)
  }

  const onDeviceChange = (index: number, deviceId: string) => {
    // 换设备要把属性清掉，否则会留下不属于该设备的 code
    patch(index, { deviceId, code: '' })
    loadProps(deviceId)
  }

  return (
    <div className="viz-settings__items">
      {series.length === 0
        ? <Text type="secondary" style={{ fontSize: 12 }}>还没有绑定数据，点下面「添加数据」。</Text>
        : null}

      {series.map((s, index) => {
        const options = (props[s.deviceId] || []).map(p => ({
          label: p.name ? `${p.name}（${p.code}）${p.unit ? ` · ${p.unit}` : ''}` : `${p.code}${p.unit ? ` · ${p.unit}` : ''}`,
          value: p.code,
        }))
        return (
          <div key={index} className="viz-settings__item">
            <SettingRow label="设备" labelWidth={SETTINGS_ITEM_LABEL_WIDTH}>
              <Select
                style={{ width: '100%' }}
                placeholder="选择设备"
                showSearch
                filterOption={false}
                value={s.deviceId || undefined}
                options={devices}
                loading={devicesLoading}
                disabled={disabled}
                onSearch={searchDevices}
                onChange={(v: string) => onDeviceChange(index, v)}
              />
            </SettingRow>
            <SettingRow label="属性" labelWidth={SETTINGS_ITEM_LABEL_WIDTH}>
              <Select
                style={{ width: '100%' }}
                placeholder={s.deviceId ? '选择属性' : '先选设备'}
                showSearch
                optionFilterProp="label"
                value={s.code || undefined}
                options={options}
                loading={!!s.deviceId && !!propsLoading[s.deviceId]}
                disabled={disabled || !s.deviceId}
                onChange={(v: string) => patch(index, { code: v })}
              />
            </SettingRow>
            <SettingRow label="图例" labelWidth={SETTINGS_ITEM_LABEL_WIDTH}>
              <Input
                style={{ flex: 1, minWidth: 0 }}
                placeholder="默认「设备名·属性名」"
                value={s.name || ''}
                disabled={disabled}
                onChange={e => patch(index, { name: e.target.value })}
              />
              <Tooltip title="删除这条数据">
                <Button
                  danger
                  type="text"
                  icon={<DeleteOutlined />}
                  disabled={disabled}
                  onClick={() => remove(index)}
                />
              </Tooltip>
            </SettingRow>
          </div>
        )
      })}

      <div className="viz-settings__add-row">
        <Button
          type="dashed"
          icon={<PlusOutlined />}
          onClick={add}
          disabled={disabled || series.length >= max || (single && series.length >= 1)}
        >
          添加数据
        </Button>
        {series.length > 0
          ? <Text type="secondary" style={{ fontSize: 12 }}>已绑定 {series.length} 条（上限 {max}）</Text>
          : null}
      </div>
    </div>
  )
}

export default SeriesEditor
