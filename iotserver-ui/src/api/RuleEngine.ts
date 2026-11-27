import { httpRequest, http } from '@/utils/request'
import { TableRequestType, CommonResponse } from '@/types'

export function getRuleEngine<T> (params: TableRequestType & {
  status?: string
  name?: string
  keyword?: string
}) {
  return httpRequest<any, CommonResponse<T>>({
    url: '/api/v1/rule-engine',
    params,
  })
}

export interface RuleEngineActionRef {
  resource_id: string
}

export interface RuleEngineAction {
  resource_id: string
  resource_name: string
  resource_type: string
}

export interface RuleEngineRestartStrategy {
  attempts?: number
  delay?: number
  maxDelay?: number
  multiplier?: number
  jitterFactor?: number
}

export interface RuleEngineOptions {
  debug?: boolean
  isEventTime?: boolean
  lateTolerance?: number
  concurrency?: number
  bufferLength?: number
  sendMetaToSink?: boolean
  sendNilField?: boolean
  sendError?: boolean
  qos?: number
  checkpointInterval?: number
  disableBufferFullDiscard?: boolean
  restartStrategy?: RuleEngineRestartStrategy
  cron?: string
  duration?: string
}

export interface RuleEngineFilter {
  product_id?: string
  message_source?: string
  select_name?: string
  condition?: string
  sql: string
  compiled_sql?: string
}

export interface DataRuleEngine {
  id: string
  name: string
  description: string
  tags: string[]
  actions: RuleEngineAction[]
  options: RuleEngineOptions
  created: number
  modified?: number
  status: string
  resource_type: string
  filter: RuleEngineFilter
  data_resource_id: string
  dataResource?: {
    name: string
    type: string
  }
}

export interface AddRuleEngine {
  id?: string
  name: string
  description?: string
  tags?: string[]
  actions: RuleEngineActionRef[]
  filter: RuleEngineFilter
  options?: RuleEngineOptions
  run_immediately?: boolean
  data_resource_id?: string
  resource_type?: string
}

export function addRuleEngine<T> (data: AddRuleEngine) {
  return httpRequest<any, CommonResponse<T>>({
    url: '/api/v1/rule-engine',
    method: 'post',
    data,
  })
}

export function editRuleEngine<T> (data: AddRuleEngine & { id: string }) {
  return httpRequest<any, CommonResponse<T>>({
    url: '/api/v1/rule-engine',
    method: 'put',
    data,
  })
}

export function getRuleEngineInfo (id: string) {
  return httpRequest<any, CommonResponse<DataRuleEngine>>({
    url: `/api/v1/rule-engine/${id}`,
    method: 'get',
  })
}

export function startRuleEngine (id: string) {
  return httpRequest<any, CommonResponse<any>>({
    url: `/api/v1/rule-engine/${id}/start`,
    method: 'post',
  })
}

export function stopRuleEngine (id: string) {
  return httpRequest<any, CommonResponse<any>>({
    url: `/api/v1/rule-engine/${id}/stop`,
    method: 'post',
  })
}

export function statusRuleEngine (id: string) {
  return httpRequest<any, CommonResponse<any>>({
    url: `/api/v1/rule-engine/${id}/status`,
    method: 'get',
  })
}

export function deleteRuleEngine (id: string) {
  return httpRequest<any, CommonResponse<any>>({
    url: `/api/v1/rule-engine/${id}/delete`,
    method: 'delete',
  })
}

export interface RuleEngineBatchResult {
  success: number
  failed: Array<{ id: string, message: string }>
}

export function batchStartRuleEngine (ids: string[]) {
  return httpRequest<any, CommonResponse<RuleEngineBatchResult>>({
    url: '/api/v1/rule-engine/batch-start',
    method: 'post',
    data: { ids },
  })
}

export function batchStopRuleEngine (ids: string[]) {
  return httpRequest<any, CommonResponse<RuleEngineBatchResult>>({
    url: '/api/v1/rule-engine/batch-stop',
    method: 'post',
    data: { ids },
  })
}

export interface RuleEngineValidateResult {
  valid: boolean
  message: string
  compiled_sql: string
  tenant_id?: string
}

export function validateRuleEngine (sql: string) {
  return httpRequest<any, CommonResponse<RuleEngineValidateResult>>({
    url: '/api/v1/rule-engine/validate',
    method: 'post',
    data: { sql },
  })
}

export interface RuleEngineDebugRequest {
  sql: string
  data: Array<Record<string, unknown>>
  interval?: number
  loop?: boolean
  // 可选：模拟 Sink 的 Data Template，对 SQL 输出再做一次格式化
  data_template?: string
  // false 时使用真实数据源，不发送模拟数据
  mock_enabled?: boolean
  // 调试时按规则当前动作配置真实推送数据
  execute_actions?: boolean
  // 需要执行的动作资源 ID，与规则动作顺序一致
  action_resource_ids?: string[]
}

export interface RuleEngineDebugActionResult {
  ok: boolean
  resource_id: string
  name: string
  type: string
  status_code?: number
  duration_ms: number
  error?: string
}

export function debugRuleEngine (data: RuleEngineDebugRequest) {
  return httpRequest<any, CommonResponse<{ id: string }>>({
    url: '/api/v1/rule-engine/debug',
    method: 'post',
    data,
  })
}

export function stopRuleEngineDebug (id: string) {
  return httpRequest<any, CommonResponse<any>>({
    url: `/api/v1/rule-engine/debug/${id}`,
    method: 'delete',
  })
}

export interface RuleEngineExportAction {
  resource_name: string
  resource_type: string
}

export interface RuleEngineExportRule {
  name: string
  description: string
  tags: string[]
  filter: RuleEngineFilter
  actions: RuleEngineExportAction[]
  options: RuleEngineOptions
}

export interface RuleEngineExportFile {
  version: number
  rules: RuleEngineExportRule[]
}

export interface RuleEngineImportResult {
  imported: number
  failed: Array<{ name: string, message: string }>
}

export async function exportRuleEngine (ids?: string[]) {
  const resp = await http.get('/api/v1/rule-engine/export', {
    params: ids && ids.length > 0 ? { ids: ids.join(',') } : undefined,
    responseType: 'blob',
  })
  const blob = resp.data as Blob
  // 后端业务错误会以 HTTP 200 + JSON 信封返回；不能让用户把它当成导出文件
  const text = await blob.text()
  try {
    const parsed = JSON.parse(text)
    if (parsed && parsed.success === false) {
      throw new Error(parsed.errorMsg || '')
    }
  } catch (error) {
    // 解析失败说明不是错误信封（正常导出文件一定可以解析），按文件返回
    if (!(error instanceof SyntaxError)) throw error
  }
  return blob
}

export function importRuleEngine (rules: RuleEngineExportRule[]) {
  return httpRequest<any, CommonResponse<RuleEngineImportResult>>({
    url: '/api/v1/rule-engine/import',
    method: 'post',
    data: { rules },
  })
}
