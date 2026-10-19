import React, { useMemo } from 'react'
import { Button, Empty, Input, Select, Spin, Tooltip, Typography } from 'antd'
import {
  AlertOutlined, AppstoreOutlined, BellOutlined, ClockCircleOutlined, CloudServerOutlined,
  ClusterOutlined, DatabaseOutlined, DeleteOutlined, FieldNumberOutlined, FundOutlined,
  LineChartOutlined, MinusCircleOutlined, PlusOutlined, ThunderboltOutlined, UserOutlined,
  WifiOutlined,
} from '@ant-design/icons'
import type { VizMetricValue } from '@/api'
import type { VizWidgetManifest, VizWidgetSettingsProps } from '../types'
import { formatValue } from '../format'
import { isPlatformView } from '@/utils/platformViewGuard'
import { isPlatformOnlyMetric } from '../platformScope'
import { MetricsEditor, useVizMetricDefs } from './MetricsEditor'
import { SettingGroup, SettingRow, SETTINGS_ITEM_LABEL_WIDTH } from './SettingsLayout'
import './stats.less'
import { t, tt } from '@/i18n'

const { Text } = Typography

export interface StatsTag {
  /** 子项名称（如「本地」） */
  label: string
  /** 指标 key */
  metric: string
}

export interface StatsTile {
  /** 块标题（如「产品数」）；留空用指标自带的名称 */
  label: string
  /** 主指标 key */
  metric: string
  /** 图标（ICON_OPTIONS 的 key）；留空用数值图标 */
  icon?: string
  /** 色调（TONE_OPTIONS 的 key）；留空按顺序轮换 */
  tone?: string
  /** 子项：名称 + 指标（如「本地」「第三方」） */
  tags?: StatsTag[]
  /** 脚注（如「数据统计截止昨日 24 时」） */
  note?: string
}

export interface StatsConfig {
  tiles: StatsTile[]
}

/** 预置图标：config 里只存 key（字符串），渲染时查表 */
const ICONS: Record<string, React.ReactNode> = {
  appstore: <AppstoreOutlined />,
  devices: <ClusterOutlined />,
  gateway: <CloudServerOutlined />,
  alert: <AlertOutlined />,
  bell: <BellOutlined />,
  number: <FieldNumberOutlined />,
  chart: <LineChartOutlined />,
  user: <UserOutlined />,
  wifi: <WifiOutlined />,
  bolt: <ThunderboltOutlined />,
  db: <DatabaseOutlined />,
  clock: <ClockCircleOutlined />,
}

const ICON_OPTIONS = [
  { label: t('产品'), value: 'appstore' },
  { label: t('设备'), value: 'devices' },
  { label: t('网关'), value: 'gateway' },
  { label: t('告警'), value: 'alert' },
  { label: t('通知'), value: 'bell' },
  { label: t('数值'), value: 'number' },
  { label: t('趋势'), value: 'chart' },
  { label: t('用户'), value: 'user' },
  { label: t('连接'), value: 'wifi' },
  { label: t('能耗'), value: 'bolt' },
  { label: t('数据'), value: 'db' },
  { label: t('时间'), value: 'clock' },
]

/** 预置色调：与 stats.less 里的 --blue/--cyan/... 对应 */
const TONES = ['blue', 'cyan', 'green', 'orange', 'purple', 'red']

const TONE_OPTIONS = [
  { label: t('蓝'), value: 'blue' },
  { label: t('青'), value: 'cyan' },
  { label: t('绿'), value: 'green' },
  { label: t('橙'), value: 'orange' },
  { label: t('紫'), value: 'purple' },
  { label: t('红'), value: 'red' },
]

/** 全部绑定的指标 key（主指标 + 子项），去重 */
export function statsMetricKeys (config: StatsConfig): string[] {
  const keys: string[] = []
  const push = (key?: string) => {
    const v = (key || '').trim()
    if (v && keys.indexOf(v) < 0) keys.push(v)
  }
  ;(config.tiles || []).forEach((tile) => {
    push(tile.metric)
    ;(tile.tags || []).forEach(tag => push(tag.metric))
  })
  return keys
}

/**
 * 统计卡：一张卡里并排多个指标块（首页「平台概述」的可配置版）。
 *
 * 每个块绑一个瞬时指标，可选图标/色调/子项/脚注；子项也是指标
 * （如主指标「产品总数」+ 子项「本地」「第三方」）。
 */
const StatsWidget: React.FC<{ config: StatsConfig; ctx: any }> = ({ config, ctx }) => {
  const defs = useVizMetricDefs()
  const allTiles = (config.tiles || []).filter(t => t)
  // 租户视图：绑了平台专属指标的块（如系统资源的 CPU/内存）整块隐藏，而不是显示 --
  // （指标清单是异步拉的，拉回来之前会短暂显示为 --，随后自动收起）
  const tiles = isPlatformView() ? allTiles : allTiles.filter(t => !isPlatformOnlyMetric(t.metric, defs))
  // 指标取值按 key 索引；取不到（未绑定 / 本视图不可见）显示 --
  const values = useMemo(() => {
    const map = new Map<string, VizMetricValue>()
    ;(((ctx.data && ctx.data.metrics) || []) as VizMetricValue[]).forEach(m => map.set(m.key, m))
    return map
  }, [ctx.data])

  if (tiles.length === 0) {
    return <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={t('还没配置统计块（右上角菜单 → 配置本组件）')} />
  }
  if (ctx.loading && values.size === 0) return <div style={{ textAlign: 'center', paddingTop: 12 }}><Spin /></div>

  // 数值格式：优先用指标自带的小数位（CPU 12.5%、内存 3.2GB），计数类默认 0 位
  const fmt = (v?: VizMetricValue) => formatValue(v && v.value, (v && v.precision) || 0)

  return (
    <div className="stats-widget">
      {tiles.map((tile, index) => {
        const tone = tile.tone || TONES[index % TONES.length]
        const main = tile.metric ? values.get(tile.metric) : undefined
        return (
          <div key={index} className={`stats-widget__tile stats-widget__tile--${tone}`}>
            <div className="stats-widget__head">
              <span className="stats-widget__icon">{ICONS[tile.icon || ''] || <FieldNumberOutlined />}</span>
              <span className="stats-widget__title">{tt(tile.label) || tt(main && main.label) || t('未命名')}</span>
            </div>
            <div className="stats-widget__value">
              {tile.metric ? fmt(main) : '--'}
              {main && main.unit ? <span className="stats-widget__unit">{tt(main.unit)}</span> : null}
            </div>
            <div className="stats-widget__foot">
              {(tile.tags || []).map((tag, tagIndex) => {
                const v = tag.metric ? values.get(tag.metric) : undefined
                return (
                  <span className="stats-widget__tag" key={tagIndex}>
                    {tt(tag.label) || t('子项')}
                    <b>{tag.metric ? `${fmt(v)}${tt(v && v.unit)}` : '--'}</b>
                  </span>
                )
              })}
              {tile.note ? <span className="stats-widget__note">{tt(tile.note)}</span> : null}
            </div>
          </div>
        )
      })}
    </div>
  )
}

const StatsSettings: React.FC<VizWidgetSettingsProps<StatsConfig>> = ({ config, onChange }) => {
  const tiles = config.tiles || []

  const patchTile = (index: number, patch: Partial<StatsTile>) => {
    onChange({ tiles: tiles.map((t, i) => (i === index ? { ...t, ...patch } : t)) })
  }
  const patchTag = (index: number, tagIndex: number, patch: Partial<StatsTag>) => {
    const tags = (tiles[index].tags || []).map((t, i) => (i === tagIndex ? { ...t, ...patch } : t))
    patchTile(index, { tags })
  }

  return (
    <SettingGroup
      title={t('统计块')}
      extra={(
        <Button
          type="link"
          size="small"
          icon={<PlusOutlined />}
          onClick={() => onChange({ tiles: tiles.concat([{ label: '', metric: '', tags: [] }]) })}
        >
          {t('添加')}
        </Button>
      )}
    >
      <SettingRow hint={t('每块显示一个指标；子项可再绑指标（如「本地 / 第三方」）。')} />
      {tiles.length === 0
        ? <Text type="secondary" style={{ fontSize: 12 }}>{t('还没有统计块，点右上角「添加」。')}</Text>
        : null}
      {tiles.map((tile, index) => (
        <div className="viz-settings__item" key={index}>
          <SettingRow label={t('标题')} labelWidth={SETTINGS_ITEM_LABEL_WIDTH}>
            <Input
              style={{ flex: 1, minWidth: 0 }}
              placeholder={t('如：产品数')}
              value={tile.label}
              onChange={e => patchTile(index, { label: e.target.value })}
            />
            <Tooltip title={t('删除这块')}>
              <Button
                danger
                type="text"
                icon={<DeleteOutlined />}
                onClick={() => onChange({ tiles: tiles.filter((_, i) => i !== index) })}
              />
            </Tooltip>
          </SettingRow>
          <SettingRow label={t('指标')} labelWidth={SETTINGS_ITEM_LABEL_WIDTH}>
            <MetricsEditor
              single
              value={tile.metric ? [tile.metric] : []}
              onChange={next => patchTile(index, { metric: next[0] || '' })}
            />
          </SettingRow>
          <SettingRow label={t('样式')} labelWidth={SETTINGS_ITEM_LABEL_WIDTH}>
            <Select
              style={{ flex: 1, minWidth: 0 }}
              allowClear
              placeholder={t('图标')}
              value={tile.icon || undefined}
              options={ICON_OPTIONS}
              onChange={(v: string) => patchTile(index, { icon: v || '' })}
            />
            <Select
              style={{ width: 76 }}
              value={tile.tone || TONES[index % TONES.length]}
              options={TONE_OPTIONS}
              onChange={(v: string) => patchTile(index, { tone: v })}
            />
          </SettingRow>
          {(tile.tags || []).map((tag, tagIndex) => (
            // 第一行给标签、后续行留空位（' ' 保持标签列宽度，控件才能对齐）
            <SettingRow key={tagIndex} label={tagIndex === 0 ? t('子项') : ' '} labelWidth={SETTINGS_ITEM_LABEL_WIDTH}>
              <Input
                style={{ width: 76 }}
                placeholder={t('名称')}
                value={tag.label}
                onChange={e => patchTag(index, tagIndex, { label: e.target.value })}
              />
              <div style={{ flex: 1, minWidth: 0 }}>
                <MetricsEditor
                  single
                  value={tag.metric ? [tag.metric] : []}
                  onChange={next => patchTag(index, tagIndex, { metric: next[0] || '' })}
                />
              </div>
              <Button
                danger
                type="text"
                size="small"
                icon={<MinusCircleOutlined />}
                onClick={() => patchTile(index, { tags: (tile.tags || []).filter((_, i) => i !== tagIndex) })}
              />
            </SettingRow>
          ))}
          <SettingRow label=" " labelWidth={SETTINGS_ITEM_LABEL_WIDTH}>
            <Button
              type="dashed"
              size="small"
              icon={<PlusOutlined />}
              onClick={() => patchTile(index, { tags: (tile.tags || []).concat([{ label: '', metric: '' }]) })}
            >
              {t('添加子项')}
            </Button>
          </SettingRow>
          <SettingRow label={t('脚注')} labelWidth={SETTINGS_ITEM_LABEL_WIDTH}>
            <Input
              style={{ flex: 1, minWidth: 0 }}
              placeholder={t('可选，如：数据统计截止昨日 24 时')}
              value={tile.note || ''}
              onChange={e => patchTile(index, { note: e.target.value })}
            />
          </SettingRow>
        </div>
      ))}
    </SettingGroup>
  )
}

export const statsManifest: VizWidgetManifest<StatsConfig> = {
  type: 'stats',
  label: t('统计卡'),
  icon: <FundOutlined />,
  defaultSize: { w: 12, h: 5 },
  minSize: { w: 3, h: 3 },
  data: true,
  defaultConfig: () => ({ tiles: [] }),
  buildQuery: config => ({ series: [], metrics: statsMetricKeys(config) }),
  render: ({ config, ctx }) => <StatsWidget config={config} ctx={ctx} />,
  Settings: StatsSettings,
}
