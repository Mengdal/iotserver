import React, { useEffect, useState } from 'react'
import { Empty, Select, Spin, Typography } from 'antd'
import { TrophyOutlined } from '@ant-design/icons'
import { getDeviceMessageRank } from '@/api'
import type { VizRankResult } from '@/api'
import type { VizWidgetManifest, VizWidgetSettingsProps } from '../types'
import { SettingGroup, SettingRow } from './SettingsLayout'
import { t, tt } from '@/i18n'

const { Text } = Typography

/** 排行榜配置：时间窗 + 显示条数 */
export interface RankConfig {
  /** 统计时间窗：1h / 24h / 7d / 30d */
  timeRange: string
  /** 显示前 N 名 */
  limit: number
}

const TIME_RANGE_OPTIONS = [
  { value: '1h', label: t('最近1小时') },
  { value: '24h', label: t('最近24小时') },
  { value: '7d', label: t('最近7天') },
  { value: '30d', label: t('最近30天') },
]

const LIMIT_OPTIONS = [3, 5, 10].map(n => ({ value: n, label: String(n) }))

/** 名次配色：前三名用奖牌色，其余用次要色 */
const RANK_COLORS = ['#f5a623', '#9aa4b2', '#c47f3e']

const RankWidget: React.FC<{ config: RankConfig }> = ({ config }) => {
  const timeRange = config.timeRange || '7d'
  const limit = config.limit || 5
  const [data, setData] = useState<VizRankResult | null>(null)
  const [loading, setLoading] = useState(false)
  const [failed, setFailed] = useState(false)

  useEffect(() => {
    let alive = true
    setLoading(true)
    setFailed(false)
    getDeviceMessageRank({ timeRange, limit })
      .then(resp => {
        if (!alive) return
        if (resp.success && resp.result) {
          setData(resp.result)
        } else {
          setFailed(true)
        }
      })
      .catch(() => { if (alive) setFailed(true) })
      .finally(() => { if (alive) setLoading(false) })
    return () => { alive = false }
  }, [timeRange, limit])

  if (loading && !data) {
    return <div style={{ height: '100%', display: 'flex', alignItems: 'center', justifyContent: 'center' }}><Spin /></div>
  }
  if (failed) {
    return <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={t('加载失败')} />
  }
  const items = (data && data.items) || []
  if (items.length === 0) {
    return <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={t('暂无设备消息数据')} />
  }

  const total = Math.max(data?.total || 0, 1)
  const max = items[0].count || 1

  return (
    <div style={{ height: '100%', padding: '4px 12px 12px', overflow: 'auto' }}>
      {items.map((item, index) => {
        const percent = Math.round(item.count / total * 1000) / 10
        const width = Math.max(2, Math.round(item.count / max * 100))
        return (
          <div key={item.deviceId} style={{ padding: '6px 0' }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
              <span
                style={{
                  flex: '0 0 auto',
                  width: 18,
                  height: 18,
                  borderRadius: 9,
                  fontSize: 11,
                  lineHeight: '18px',
                  textAlign: 'center',
                  color: '#fff',
                  background: RANK_COLORS[index] || '#c9ced6',
                }}
              >
                {index + 1}
              </span>
              <Text ellipsis style={{ flex: 1, minWidth: 0 }} title={item.deviceName || item.deviceId}>
                {tt(item.deviceName) || item.deviceId}
              </Text>
              <Text strong style={{ flex: '0 0 auto', fontSize: 12 }}>{item.count}</Text>
              <Text type="secondary" style={{ flex: '0 0 auto', fontSize: 11, width: 44, textAlign: 'right' }}>
                {percent}%
              </Text>
            </div>
            <div style={{ marginLeft: 26, marginTop: 4, height: 4, borderRadius: 2, background: '#f0f1f3' }}>
              <div style={{ width: `${width}%`, height: '100%', borderRadius: 2, background: RANK_COLORS[index] || '#c9ced6' }} />
            </div>
          </div>
        )
      })}
    </div>
  )
}

const RankSettings: React.FC<VizWidgetSettingsProps<RankConfig>> = ({ config, onChange }) => (
  <SettingGroup title={t('数据设置')}>
    <SettingRow label={t('时间范围')}>
      <Select
        style={{ width: '100%' }}
        value={config.timeRange || '7d'}
        options={TIME_RANGE_OPTIONS}
        onChange={v => onChange({ timeRange: v })}
      />
    </SettingRow>
    <SettingRow label={t('显示条数')}>
      <Select
        style={{ width: '100%' }}
        value={config.limit || 5}
        options={LIMIT_OPTIONS}
        onChange={v => onChange({ limit: v })}
      />
    </SettingRow>
  </SettingGroup>
)

export const rankManifest: VizWidgetManifest<RankConfig> = {
  type: 'rank',
  label: t('消息量排行'),
  icon: <TrophyOutlined />,
  defaultSize: { w: 4, h: 8 },
  minSize: { w: 3, h: 4 },
  defaultConfig: () => ({ timeRange: '7d', limit: 5 }),
  render: ({ config }) => <RankWidget config={config} />,
  Settings: RankSettings,
}
