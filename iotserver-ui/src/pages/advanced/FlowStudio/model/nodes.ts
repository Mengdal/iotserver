import { t } from '@/i18n'
import type { NodeCategory, NodeDef, PortKind } from './types'

/** 节点分类：顺序与画布调色板一致 */
export const CATEGORY_ORDER: NodeCategory[] = ['source', 'transform', 'streaming', 'routing', 'sink']

export const CATEGORY_LABEL: Record<NodeCategory, string> = {
  source: '源',
  transform: '转换',
  streaming: '窗口与聚合',
  routing: '路由',
  sink: '目标',
}

export const CATEGORY_COLOR: Record<NodeCategory, string> = {
  source: '#1677ff',
  transform: '#722ed1',
  streaming: '#13a8a8',
  routing: '#fa8c16',
  sink: '#8c8c8c',
}

export const PORT_KIND_LABEL: Record<PortKind, string> = {
  stream: '事件流',
  collection: '事件集合',
  table: '表',
  any: '任意',
}

const input = (kind: PortKind = 'stream') => [{ id: 'in', label: '输入', kind }]
const output = (kind: PortKind = 'stream') => [{ id: 'out', label: '输出', kind }]

/** 内置节点定义（演示版）。属性类型、端口 kind、编译 nodeType 均按参考实现收敛。 */
export const NODE_DEFS: NodeDef[] = [
  {
    type: 'mqtt-source',
    version: 1,
    displayName: 'MQTT 数据源',
    description: '从 MQTT 主题接入设备事件',
    category: 'source',
    icon: 'MQ',
    accent: CATEGORY_COLOR.source,
    inputs: [],
    outputs: output('stream'),
    subtitleKey: 'topic',
    runtimeKind: 'source',
    nodeType: 'mqtt',
    properties: [
      { key: 'topic', label: '主题', type: 'string', required: true, placeholder: 'eventbus/in', hint: '平台现有规则共用 eventbus/in 主题' },
      { key: 'qos', label: 'QoS', type: 'select', options: [{ label: '0', value: 0 }, { label: '1', value: 1 }, { label: '2', value: 2 }] },
      { key: 'protocolVersion', label: '协议版本', type: 'select', options: [{ label: '3.1.1', value: '3.1.1' }, { label: '5', value: '5' }] },
    ],
  },
  {
    type: 'memory-source',
    version: 1,
    displayName: '内存数据源',
    description: '从 eKuiper 内存主题接入事件',
    category: 'source',
    icon: 'ME',
    accent: CATEGORY_COLOR.source,
    inputs: [],
    outputs: output('stream'),
    subtitleKey: 'topic',
    runtimeKind: 'source',
    nodeType: 'memory',
    properties: [
      { key: 'topic', label: '主题', type: 'string', required: true, placeholder: 'demo' },
    ],
  },
  {
    type: 'filter',
    version: 1,
    displayName: '过滤',
    description: '按表达式过滤事件',
    category: 'transform',
    icon: 'FI',
    accent: CATEGORY_COLOR.transform,
    inputs: input('stream'),
    outputs: output('stream'),
    subtitleKey: 'expression',
    runtimeKind: 'operator',
    nodeType: 'filter',
    properties: [
      { key: 'expression', label: '过滤表达式', type: 'expression', required: true, placeholder: 'temperature > 30', hint: 'eKuiper 表达式，例如 temperature > 30' },
    ],
  },
  {
    type: 'window',
    version: 1,
    displayName: '窗口',
    description: '把事件流切分成滚动窗口',
    category: 'streaming',
    icon: 'WI',
    accent: CATEGORY_COLOR.streaming,
    inputs: input('stream'),
    outputs: output('collection'),
    subtitleKey: 'length',
    runtimeKind: 'operator',
    nodeType: 'window',
    properties: [
      { key: 'length', label: '窗口长度', type: 'number', required: true, min: 1, step: 1, hint: '演示版固定使用滚动窗口' },
      { key: 'timeUnit', label: '时间单位', type: 'select', required: true, options: [{ label: '秒', value: 'SS' }, { label: '分', value: 'MI' }, { label: '时', value: 'HH' }, { label: '天', value: 'DD' }] },
    ],
  },
  {
    type: 'group-by',
    version: 1,
    displayName: '分组',
    description: '按维度对窗口内的事件分组',
    category: 'streaming',
    icon: 'GB',
    accent: CATEGORY_COLOR.streaming,
    inputs: input('collection'),
    outputs: output('collection'),
    subtitleKey: 'keys',
    runtimeKind: 'operator',
    nodeType: 'groupby',
    properties: [
      { key: 'keys', label: '分组字段', type: 'expression', required: true, placeholder: 'deviceId', hint: '多个字段用逗号分隔' },
    ],
  },
  {
    type: 'aggregate',
    version: 1,
    displayName: '聚合',
    description: '对分组后的事件执行聚合函数',
    category: 'streaming',
    icon: 'AG',
    accent: CATEGORY_COLOR.streaming,
    inputs: input('collection'),
    outputs: output('stream'),
    subtitleKey: 'fields',
    runtimeKind: 'operator',
    nodeType: 'aggfunc',
    properties: [
      { key: 'fields', label: '聚合表达式', type: 'expression', required: true, placeholder: 'avg(temperature) AS avgTemp', hint: '单个聚合函数调用' },
    ],
  },
  {
    type: 'switch',
    version: 1,
    displayName: '条件分支',
    description: '按条件把事件分到两个分支',
    category: 'routing',
    icon: 'SW',
    accent: CATEGORY_COLOR.routing,
    inputs: input('stream'),
    outputs: [
      { id: 'branch-1', label: '分支 1', kind: 'stream' },
      { id: 'branch-2', label: '分支 2', kind: 'stream' },
      { id: 'default', label: '默认', kind: 'stream' },
    ],
    subtitleKey: 'cases',
    runtimeKind: 'operator',
    nodeType: 'switch',
    properties: [
      { key: 'cases', label: '分支条件', type: 'expression', required: true, placeholder: 'temperature > 30, temperature > 10', hint: '恰好两条条件，用逗号分隔，对应分支 1 / 分支 2' },
    ],
  },
  {
    type: 'mqtt-sink',
    version: 1,
    displayName: 'MQTT 目标',
    description: '把结果发布到 MQTT 主题',
    category: 'sink',
    icon: 'MQ',
    accent: CATEGORY_COLOR.sink,
    inputs: input('stream'),
    outputs: [],
    subtitleKey: 'topic',
    runtimeKind: 'sink',
    nodeType: 'mqtt',
    properties: [
      { key: 'server', label: 'Broker 地址', type: 'string', required: true, placeholder: 'tcp://mqtt-broker:58090', hint: 'eKuiper MQTT sink 的 broker 地址，平台 compose 默认为 tcp://mqtt-broker:58090' },
      { key: 'topic', label: '主题', type: 'string', required: true, placeholder: 'alarm/high-temp' },
      { key: 'qos', label: 'QoS', type: 'select', options: [{ label: '0', value: 0 }, { label: '1', value: 1 }, { label: '2', value: 2 }] },
    ],
  },
  {
    type: 'rest-sink',
    version: 1,
    displayName: 'HTTP 目标',
    description: '把结果推送到 HTTP 接口',
    category: 'sink',
    icon: 'HT',
    accent: CATEGORY_COLOR.sink,
    inputs: input('stream'),
    outputs: [],
    subtitleKey: 'url',
    runtimeKind: 'sink',
    nodeType: 'rest',
    properties: [
      { key: 'url', label: 'URL', type: 'string', required: true, placeholder: 'http://127.0.0.1:3000/api/v1/ekuiper/alert' },
      { key: 'method', label: '方法', type: 'select', options: [{ label: 'POST', value: 'POST' }, { label: 'PUT', value: 'PUT' }, { label: 'PATCH', value: 'PATCH' }] },
    ],
  },
  {
    type: 'log-sink',
    version: 1,
    displayName: '日志目标',
    description: '把结果写入 eKuiper 日志',
    category: 'sink',
    icon: 'LG',
    accent: CATEGORY_COLOR.sink,
    inputs: input('stream'),
    outputs: [],
    runtimeKind: 'sink',
    nodeType: 'log',
    properties: [],
  },
]

export const NODE_DEF_MAP: Record<string, NodeDef> = NODE_DEFS.reduce((acc, def) => {
  acc[def.type] = def
  return acc
}, {} as Record<string, NodeDef>)

export function getNodeDef (type: string, version?: number): NodeDef | undefined {
  const def = NODE_DEF_MAP[type]
  if (!def) return undefined
  if (version !== undefined && def.version !== version) return undefined
  return def
}

/** 新节点的默认配置：必填字符串留空，数字取 min 或 1，下拉取第一项 */
export function defaultConfig (def: NodeDef): Record<string, any> {
  const config: Record<string, any> = {}
  def.properties.forEach(prop => {
    if (prop.type === 'number') {
      config[prop.key] = prop.min !== undefined ? prop.min : 1
    } else if (prop.type === 'boolean') {
      config[prop.key] = false
    } else if (prop.type === 'select') {
      config[prop.key] = prop.options && prop.options.length ? prop.options[0].value : ''
    } else if (prop.type === 'json') {
      config[prop.key] = {}
    } else {
      config[prop.key] = ''
    }
  })
  return config
}

export function nodeSubtitle (def: NodeDef, config: Record<string, any>): string {
  if (def.type === 'window') {
    const unitLabel: Record<string, string> = { SS: '秒', MI: '分', HH: '时', DD: '天' }
    return t('{{count}} {{unit}} 滚动窗口', {
      count: config.length || '-',
      unit: t(unitLabel[String(config.timeUnit)] || ''),
    })
  }
  if (!def.subtitleKey) return def.description
  const value = config[def.subtitleKey]
  if (value === undefined || value === null || String(value).trim() === '') return def.description
  return String(value)
}
