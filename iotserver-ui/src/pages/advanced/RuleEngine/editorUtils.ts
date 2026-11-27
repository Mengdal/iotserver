import type { RuleEngineOptions } from '@/api/RuleEngine'
import { t } from '@/i18n'

export const MQTT_STREAM = 'mqtt_stream'

export interface SqlExample {
  key: string
  title: string
  description: string
  sql: string
}

/** SQL 示例：均基于平台统一的 mqtt_stream，保存时后端会自动追加租户过滤条件 */
export const SQL_EXAMPLES: SqlExample[] = [
  {
    key: 'all',
    title: t('查询全部数据'),
    description: t('读取消息总线中的全部设备数据'),
    sql: 'SELECT *\nFROM mqtt_stream',
  },
  {
    key: 'fields',
    title: t('字段筛选'),
    description: t('只保留需要上报的字段'),
    sql: 'SELECT deviceId, temperature, humidity\nFROM mqtt_stream',
  },
  {
    key: 'condition',
    title: t('条件过滤'),
    description: t('按设备和阈值筛选数据'),
    sql: 'SELECT *\nFROM mqtt_stream\nWHERE deviceId = \'dev-001\' AND temperature > 30',
  },
  {
    key: 'calc',
    title: t('简单数学计算'),
    description: t('在 SQL 中直接做单位换算或派生字段'),
    sql: 'SELECT deviceId, temperature * 1.8 + 32 AS temperature_f\nFROM mqtt_stream',
  },
  {
    key: 'window',
    title: t('时间窗口聚合'),
    description: t('按 10 秒滚动窗口计算设备平均温度'),
    sql: 'SELECT deviceId, avg(temperature) AS avg_temperature, window_end() AS window_end\nFROM mqtt_stream\nGROUP BY TUMBLINGWINDOW(ss, 10), deviceId',
  },
  {
    key: 'function',
    title: t('使用内置函数'),
    description: t('使用字符串、数学等内置函数加工字段'),
    sql: 'SELECT deviceId, upper(status) AS status, abs(temperature) AS temperature\nFROM mqtt_stream',
  },
]

/** 新建规则的默认规则选项（与 eKuiper 默认值对齐） */
export const DEFAULT_RULE_OPTIONS: RuleEngineOptions = {
  concurrency: 1,
  bufferLength: 1024,
  qos: 0,
  checkpointInterval: 300000,
  debug: false,
  sendMetaToSink: false,
  sendError: true,
  sendNilField: false,
  isEventTime: false,
  lateTolerance: 0,
  disableBufferFullDiscard: false,
  restartStrategy: { attempts: 0 },
}

/**
 * 试运行示例输入数据：按平台当前 eventbus/in 的实际消息结构给出。
 * 租户规则需要带当前租户的 tenantId；平台视图的消息没有该字段。
 */
export function sampleMessage (tenantId: string): string {
  const payload: Record<string, unknown> = {
    deviceId: '29365864',
    deviceName: 'C1',
    productId: '67126069',
    productName: 'test',
    messageType: 'PROPERTY_REPORT',
    data: {
      discharge_pressure: 100.815079,
    },
  }
  if (tenantId) payload.tenantId = tenantId
  return JSON.stringify(payload, null, 2)
}

function isRecord (value: unknown): value is Record<string, unknown> {
  return !!value && typeof value === 'object' && !Array.isArray(value)
}

/**
 * 解析模拟数据源 payload。与参考平台一致，支持：
 *   - 单个 JSON 对象（多行/单行均可）
 *   - JSON 对象数组
 *   - 每行一个 JSON 对象（NDJSON）
 */
export function parseMockPayload (text: string): Array<Record<string, unknown>> {
  const input = text.trim()
  if (!input) throw new Error(t('请填写模拟数据'))

  try {
    const parsed = JSON.parse(input)
    if (Array.isArray(parsed)) {
      if (parsed.length === 0) throw new Error(t('模拟数据不能为空'))
      if (!parsed.every(isRecord)) throw new Error(t('模拟数据必须是 JSON 对象或对象数组'))
      return parsed
    }
    if (isRecord(parsed)) return [parsed]
    throw new Error(t('模拟数据必须是 JSON 对象或对象数组'))
  } catch (error) {
    // 整体不是合法 JSON 时，按「每行一条 JSON 对象」再解析
    if (error instanceof SyntaxError) {
      const lines = input.split(/\r?\n/).map(line => line.trim()).filter(Boolean)
      if (lines.length > 0) {
        const records: Array<Record<string, unknown>> = []
        for (const line of lines) {
          let item: unknown
          try {
            item = JSON.parse(line)
          } catch {
            throw new Error(t('模拟数据必须是合法 JSON'))
          }
          if (!isRecord(item)) throw new Error(t('模拟数据必须是 JSON 对象或对象数组'))
          records.push(item)
        }
        return records
      }
    }
    throw error instanceof Error ? error : new Error(t('模拟数据必须是合法 JSON'))
  }
}

export function newRuleId (): string {
  return ''
}
