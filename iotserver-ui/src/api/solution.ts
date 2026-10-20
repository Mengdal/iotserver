import { httpRequest } from '@/utils/request'
import { CommonResponse } from '@/types'

// ===== 类型定义 =====

/**
 * 行业方案（整机模板），字段与后端 dtos.SolutionResponse 对齐。
 *
 * 注意：thingModelJson / alarmRulesJson / dashboardJson / metricsJson 都是
 * **JSON 字符串**，前端展示前必须 JSON.parse 并 try/catch 容错。
 */
export interface Solution {
  /** 方案标识，如 injection_molding */
  key: string
  /** 方案名称（中文），如 注塑机 */
  name: string
  /** 方案英文名称 */
  nameEn?: string
  /** 行业，如 橡塑 */
  industry: string
  version: number
  /** 是否内置方案（内置不可删） */
  builtIn: boolean
  /** 图标名（与菜单图标集一致，如 database） */
  icon: string
  description: string
  /** 英文描述 */
  descriptionEn?: string
  /** 物模型 JSON 字符串：{properties,events,services} */
  thingModelJson: string
  /** 告警预设 JSON 字符串：[{name,propertyCode,operator,value,alertLevel,...}] */
  alarmRulesJson: string
  /** 看板预设 JSON 字符串：{name,widgets:[{title,type,propertyCode,...}]} */
  dashboardJson: string
  /** OEE 指标映射 JSON 字符串：{running,output,good,reject,cycleTime} */
  metricsJson: string
}

/** 一键应用方案的请求体，与后端 dtos.SolutionApplyRequest 对齐 */
export interface SolutionApplyRequest {
  /** 生成的产品名称，留空则后端用方案名 */
  productName?: string
  /** 可选：把生成的设备挂到某个组织节点 */
  deptId?: string
  /** >0 时同时创建对应数量的设备（名称 = 产品名-序号） */
  deviceCount?: number
  /** 是否按预设创建告警规则，后端默认 true */
  createAlarmRules?: boolean
}

/** 应用结果摘要，与后端 dtos.SolutionApplyResponse 对齐 */
export interface SolutionApplyResponse {
  productId: string
  productName: string
  deviceIds: string[]
  alarmRuleIds: string[]
  /** 未完成/被降级的步骤说明，逐条展示给用户 */
  skipped: string[]
}

// ===== 方案中心 =====

/** 方案列表：result 直接是数组，不是分页对象 */
export function getSolutions<T = Solution[]> (params?: { industry?: string }) {
  return httpRequest<any, CommonResponse<T>>({
    url: '/api/v1/solutions',
    method: 'get',
    params,
  })
}

/** 一键应用方案：生成产品（并发布）、可选建设备与告警规则 */
export function applySolution<T = SolutionApplyResponse> (key: string, data: SolutionApplyRequest) {
  return httpRequest<any, CommonResponse<T>>({
    url: `/api/v1/solution/${key}/apply`,
    method: 'post',
    data,
  })
}

// ===== 方案物模型功能项（属性 / 事件 / 服务）增删改 =====

/** 物模型功能项类型：property 属性 / event 事件 / action 服务 */
export type SolutionThingModelItemType = 'property' | 'event' | 'action'

/**
 * 功能项内容：字段名随类型而变（属性 name / 事件 eventName / 服务 serviceName），
 * 所以这里用宽松类型，由调用方按类型组装。
 */
export type SolutionThingModelItem = Record<string, any>

/**
 * 新增功能项，result 是更新后的方案对象（与列表项同结构）。
 * 接口尚未就绪时前端只做类型防御，不改变调用契约。
 */
export function addSolutionThingModelItem<T = Solution> (
  key: string,
  data: { type: SolutionThingModelItemType, item: SolutionThingModelItem },
) {
  return httpRequest<any, CommonResponse<T>>({
    url: `/api/v1/solution/${key}/thing-model/item`,
    method: 'post',
    data,
  })
}

/** 编辑功能项：identifier 是被编辑项的**旧**标识符，item 是改后的完整内容 */
export function editSolutionThingModelItem<T = Solution> (
  key: string,
  data: { type: SolutionThingModelItemType, identifier: string, item: SolutionThingModelItem },
) {
  return httpRequest<any, CommonResponse<T>>({
    url: `/api/v1/solution/${key}/thing-model/item`,
    method: 'put',
    data,
  })
}

/** 删除功能项，result 同样是更新后的方案对象 */
export function deleteSolutionThingModelItem<T = Solution> (
  key: string,
  params: { type: SolutionThingModelItemType, identifier: string },
) {
  return httpRequest<any, CommonResponse<T>>({
    url: `/api/v1/solution/${key}/thing-model/item`,
    method: 'delete',
    params,
  })
}
