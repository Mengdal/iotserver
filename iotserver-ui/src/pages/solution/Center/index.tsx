import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import { useNavigate } from 'react-router-dom'
import {
  Button,
  Card,
  Empty,
  Form,
  Input,
  InputNumber,
  Modal,
  Popconfirm,
  Radio,
  Select,
  Space,
  Table,
  Tabs,
  Tag,
  Typography,
  message,
} from 'antd'
import {
  MinusCircleOutlined,
  PlusOutlined,
  SearchOutlined,
} from '@ant-design/icons'
import type { ColumnsType } from 'antd/es/table'
import {
  addSolutionThingModelItem,
  deleteSolutionThingModelItem,
  editSolutionThingModelItem,
  getSolutions,
  Solution,
  SolutionThingModelItem,
  SolutionThingModelItemType,
} from '@/api'
import { DetailModal, DetailModalRef } from '@/pages/gateway/ProductManage/DeatilModal'
import {
  BaseThingModelType,
  ProductInfo,
  ProductThingModelType,
  TypeSpec,
} from '@/pages/gateway/ProductManage/types'
import { useAuth } from '@/utils/auth'
import * as P from '@/routes/permissions'
import { t, tt } from '@/i18n'
import { localized } from '@/utils/localized'

const { useMessage } = message
const { Text } = Typography

/** 权限码：后端权限常量尚未生成时先用字面量，避免依赖生成器 */
const PERM_TSL_ADD = 'solution:tsl:add'
const PERM_TSL_EDIT = 'solution:tsl:edit'
const PERM_TSL_REMOVE = 'solution:tsl:remove'

/** 「自定义」页签：租户自己的方案（builtIn === false） */
const CUSTOM_TAB = '自定义'

/** 功能类型筛选值：'' = 全部 */
type FuncTypeFilter = '' | 'property' | 'event' | 'action'
/** 合并行类型：属性 property / 事件 event / 服务 action */
type RowType = 'property' | 'event' | 'action'

interface DataSpecs {
  min?: string | number
  max?: string | number
  step?: string | number
  unit?: string
  unitName?: string
  length?: string | number
}

interface DataSpecItem {
  name?: string
  value?: string | number
}

interface ThingModelProperty {
  name?: string
  nameEn?: string
  identifier?: string
  description?: string
  descriptionEn?: string
  dataType?: string
  /** R 只上报 / RW 可下发可上报 / W 只下发 */
  rwFlag?: string
  dataSpecs?: DataSpecs | null
  dataSpecsList?: DataSpecItem[] | null
}

interface ThingModelEvent {
  eventName?: string
  eventNameEn?: string
  identifier?: string
  description?: string
  descriptionEn?: string
  /** ALERT 告警 / INFO 信息 / ERROR 错误 */
  eventType?: string
  outputData?: unknown[]
}

interface ThingModelService {
  serviceName?: string
  serviceNameEn?: string
  identifier?: string
  description?: string
  descriptionEn?: string
  /** ASYNC 异步 / SYNC 同步 */
  callType?: string
  inputParams?: unknown[]
  outputParams?: unknown[]
}

interface ThingModel {
  properties?: ThingModelProperty[]
  events?: ThingModelEvent[]
  services?: ThingModelService[]
}

/** 主表格一行的合并结构：属性 + 事件 + 服务 */
interface ThingRow {
  id: string
  type: RowType
  name: string
  identifier: string
  dataType: string
  rwFlag: string
  eventType: string
  callType: string
  /** 数据定义的多行文本，空数组表示展示 '-' */
  definition: string[]
  /** 原始功能项，供「查看」「编辑」使用 */
  raw: Record<string, any>
}

interface ThingItemFormValues {
  type: RowType
  name: string
  nameEn?: string
  identifier: string
  description?: string
  descriptionEn?: string
  dataType?: string
  rwFlag?: string
  min?: number
  max?: number
  step?: number
  unit?: string
  unitName?: string
  length?: number
  enumList?: DataSpecItem[]
  eventType?: string
  callType?: string
}

/**
 * JSON 解析：字符串按 JSON.parse，已是对象/数组则原样返回。
 * 任何异常都返回 null，由调用方展示兜底文案，绝不让整页报错。
 */
const parseJson = <T extends unknown>(raw?: string | object | null): T | null => {
  if (raw === null || raw === undefined || raw === '') return null
  if (typeof raw === 'object') return raw as T
  try {
    const parsed = JSON.parse(raw as string)
    return parsed === null || parsed === undefined ? null : (parsed as T)
  } catch (error) {
    return null
  }
}

const asArray = <T extends unknown>(value?: T[] | null): T[] => (Array.isArray(value) ? value : [])

const isEmptyValue = (value: unknown) => value === null || value === undefined || value === ''

/** 功能类型标签：属性类型(绿) / 事件类型(火山) / 服务类型(蓝) */
const TYPE_META: Record<RowType, { text: string, color: string }> = {
  property: { text: t('属性类型'), color: 'green' },
  event: { text: t('事件类型'), color: 'volcano' },
  action: { text: t('服务类型'), color: 'blue' },
}

/** 属性数据类型：后端英文枚举 → 中文 */
const DATA_TYPE_TEXT: Record<string, string> = {
  TEXT: t('字符串'),
  FLOAT: t('浮点型'),
  INT: t('整数型'),
  BOOL: t('布尔型'),
  ENUM: t('枚举型'),
  DATE: t('时间型'),
}

/** 表单可选数据类型（提交英文枚举） */
const DATA_TYPE_OPTIONS = [
  { value: 'TEXT', label: t('字符串') },
  { value: 'FLOAT', label: t('浮点型') },
  { value: 'INT', label: t('整数型') },
  { value: 'BOOL', label: t('布尔型') },
  { value: 'ENUM', label: t('枚举型') },
]

const EVENT_TYPE: Record<string, string> = {
  ALERT: t('告警'),
  INFO: t('信息'),
  ERROR: t('错误'),
}

const CALL_TYPE: Record<string, string> = {
  ASYNC: t('异步'),
  SYNC: t('同步'),
}

const NUMERIC_TYPE = ['INT', 'FLOAT', 'DOUBLE']
const ENUM_TYPE = ['BOOL', 'ENUM']

/** 属性「数据定义」：数值型三行 / 枚举项 / 文本长度，缺哪行不显示 */
const propertyDefinition = (property: ThingModelProperty): string[] => {
  const dataType = (property.dataType || '').toUpperCase()
  if (NUMERIC_TYPE.includes(dataType)) {
    const specs = property.dataSpecs
    if (!specs) return []
    const lines: string[] = []
    if (!isEmptyValue(specs.min) || !isEmptyValue(specs.max)) {
      lines.push(t('取值范围: {{p1}}-{{p2}}', { p1: isEmptyValue(specs.min) ? '' : specs.min, p2: isEmptyValue(specs.max) ? '' : specs.max }))
    }
    if (!isEmptyValue(specs.step)) lines.push(t('步长: {{step}}', { step: specs.step }))
    if (!isEmptyValue(specs.unit)) {
      const unit = tt(specs.unit)
      const unitName = isEmptyValue(specs.unitName) ? '' : tt(specs.unitName)
      lines.push(t('单位: {{unit}}{{p2}}', { unit, p2: unitName && unitName !== unit ? ` (${unitName})` : '' }))
    }
    return lines
  }
  if (ENUM_TYPE.includes(dataType)) {
    return asArray(property.dataSpecsList)
      .filter(item => item && (!isEmptyValue(item.name) || !isEmptyValue(item.value)))
      .map(item => `${isEmptyValue(item.name) ? '' : tt(item.name)}=${isEmptyValue(item.value) ? '' : item.value}`)
  }
  if (dataType === 'TEXT') {
    const length = property.dataSpecs?.length
    return isEmptyValue(length) ? [] : [t('数据长度: {{count}}', { count: length })]
  }
  return []
}

/** 把物模型合并成一张表：属性 → 事件 → 服务 */
const buildThingRows = (thingModel: ThingModel | null): ThingRow[] => {
  if (!thingModel) return []
  const rows: ThingRow[] = []
  asArray(thingModel.properties).forEach((item, index) => {
    rows.push({
      id: `property-${item.identifier || index}`,
      type: 'property',
      name: localized(item.name, item.nameEn) || '-',
      identifier: item.identifier || '-',
      dataType: item.dataType || '',
      rwFlag: item.rwFlag || '',
      eventType: '',
      callType: '',
      definition: propertyDefinition(item),
      raw: item as Record<string, any>,
    })
  })
  asArray(thingModel.events).forEach((item, index) => {
    rows.push({
      id: `event-${item.identifier || index}`,
      type: 'event',
      name: localized(item.eventName, item.eventNameEn) || '-',
      identifier: item.identifier || '-',
      dataType: '',
      rwFlag: '',
      eventType: item.eventType || '',
      callType: '',
      definition: [t('输出参数 {{count}} 个', { count: asArray(item.outputData).length })],
      raw: item as Record<string, any>,
    })
  })
  asArray(thingModel.services).forEach((item, index) => {
    rows.push({
      id: `action-${item.identifier || index}`,
      type: 'action',
      name: localized(item.serviceName, item.serviceNameEn) || '-',
      identifier: item.identifier || '-',
      dataType: '',
      rwFlag: '',
      eventType: '',
      callType: item.callType || '',
      definition: [t('入参 {{count}} 个', { count: asArray(item.inputParams).length })],
      raw: item as Record<string, any>,
    })
  })
  return rows
}

/** 方案物模型 dataType（大写）→ 产品物模型 type_spec.type（小写枚举） */
const TYPE_SPEC_KEY: Record<string, TypeSpec> = {
  INT: 'int',
  FLOAT: 'float',
  TEXT: 'text',
  BOOL: 'bool',
  ENUM: 'enum',
  DATE: 'date',
}

/**
 * 「查看」用的产品物模型结构：只填产品抽屉会读取的字段，
 * 其余字段（created / product_id 等）抽屉不读，故不构造。
 */
interface ThingModelViewRecord {
  id: string
  type: RowType
  name: string
  code: string
  description: string
  access_mode?: string
  type_spec?: { type: TypeSpec, specs: string }
  event_type?: string
  call_type?: string
  input_params?: unknown[]
  output_params?: unknown[]
}

/** 属性数据定义 → 产品抽屉能识别的 type_spec.specs（JSON 字符串） */
const buildSpecsJson = (raw: Record<string, any>, specType: TypeSpec): string => {
  if (specType === 'int' || specType === 'float') return JSON.stringify(raw.dataSpecs || {})
  if (specType === 'text') {
    const length = raw.dataSpecs?.length
    return JSON.stringify(isEmptyValue(length) ? {} : { length })
  }
  if (specType === 'bool' || specType === 'enum') {
    const specs: Record<string, string> = {}
    asArray<DataSpecItem>(raw.dataSpecsList).forEach(item => {
      specs[String(item?.value ?? '')] = String(item?.name ?? '')
    })
    return JSON.stringify(specs)
  }
  return '{}'
}

/** 方案行 → 产品物模型抽屉的查看入参（有 record 时该抽屉只读） */
const buildViewRecord = (row: ThingRow): ProductThingModelType => {
  const raw = row.raw || {}
  const base = {
    id: row.id,
    name: row.name === '-' ? '' : row.name,
    code: row.identifier === '-' ? '' : row.identifier,
    description: localized(raw.description, raw.descriptionEn),
  }
  if (row.type === 'property') {
    const specType = TYPE_SPEC_KEY[(raw.dataType || '').toUpperCase()] || 'text'
    const record: ThingModelViewRecord = {
      ...base,
      type: 'property',
      access_mode: raw.rwFlag || 'R',
      type_spec: { type: specType, specs: buildSpecsJson(raw, specType) },
    }
    return record as unknown as ProductThingModelType
  }
  if (row.type === 'event') {
    const record: ThingModelViewRecord = {
      ...base,
      type: 'event',
      event_type: raw.eventType || '',
      output_params: asArray(raw.outputData),
    }
    return record as unknown as ProductThingModelType
  }
  const record: ThingModelViewRecord = {
    ...base,
    type: 'action',
    call_type: raw.callType || '',
    input_params: asArray(raw.inputParams),
    // 内置模板里服务出参写的是 outParams，这里兼容两种写法
    output_params: asArray(raw.outParams || raw.outputParams),
  }
  return record as unknown as ProductThingModelType
}

/**
 * 产品物模型抽屉提交的 payload → 方案物模型接口的 { type, item }。
 * payload 的类型相关字段来自 helper.ts：属性在 property、事件在 event、服务在 action。
 */
const buildSolutionThingModelItem = (
  payload: BaseThingModelType,
): { type: SolutionThingModelItemType, item: SolutionThingModelItem } | null => {
  const type = payload.thing_model_type as SolutionThingModelItemType
  const description = payload.description || ''
  const descriptionEn = payload.description_en || ''

  if (type === 'property') {
    const property = (payload.property || {}) as Record<string, any>
    const specs = property.specs && typeof property.specs === 'object' ? property.specs : {}
    const dataType = String(property.type || '').toUpperCase()
    const item: SolutionThingModelItem = {
      name: payload.name,
      nameEn: payload.name_en || '',
      identifier: payload.code,
      dataType,
      rwFlag: property.access_model,
      dataSpecs: null,
      dataSpecsList: null,
      description,
      descriptionEn,
    }
    if (dataType === 'INT' || dataType === 'FLOAT') {
      item.dataSpecs = {
        min: specs.min,
        max: specs.max,
        step: specs.step,
        unit: specs.unit,
        unitName: specs.unitName,
      }
    } else if (dataType === 'BOOL' || dataType === 'ENUM') {
      item.dataSpecsList = Object.entries(specs as Record<string, unknown>).map(([value, name]) => ({
        name: String(name ?? ''),
        value,
      }))
    } else if (dataType === 'TEXT') {
      // 字符串类型把"数据长度"落进 dataSpecs（与内置模板一致，否则产品侧拿不到长度）
      item.dataSpecs = { length: (specs as Record<string, unknown>).length }
    }
    return { type, item }
  }

  if (type === 'event') {
    const event = (payload.event || {}) as Record<string, any>
    return {
      type,
      item: {
        eventName: payload.name,
        eventNameEn: payload.name_en || '',
        identifier: payload.code,
        eventType: String(event.event_type || '').toUpperCase(),
        // helper.ts 产出的字段名是 output_param，这里兼容两种写法
        outputData: event.output_params || event.output_param || [],
        description,
        descriptionEn,
      },
    }
  }

  if (type === 'action') {
    const action = (payload.action || {}) as Record<string, any>
    return {
      type,
      item: {
        serviceName: payload.name,
        serviceNameEn: payload.name_en || '',
        identifier: payload.code,
        callType: String(action.call_type || '').toUpperCase(),
        inputParams: action.input_params || action.input_param || [],
        // 规范字段名是 outParams（dtos.ThingModelTemplateServices.OutParams），写成 outputParams 会在"用方案建产品"时被丢弃
        outParams: action.output_params || action.output_param || [],
        description,
      },
    }
  }

  return null
}

const SolutionCenterPage: React.FC = () => {
  const [messageApi, contextHolder] = useMessage()
  const [itemForm] = Form.useForm<ThingItemFormValues>()
  const navigate = useNavigate()
  const detailModalRef = useRef<DetailModalRef>(null)

  const [loading, setLoading] = useState(false)
  /** Layout banner 上的插槽（搜索框要压在 banner 上） */
  const [bannerSlot, setBannerSlot] = useState<HTMLElement | null>(null)
  /** 全量方案：页签、左侧列表、当前方案都从这里派生 */
  const [solutions, setSolutions] = useState<Solution[]>([])
  /** 顶部 banner 搜索：仅筛选左侧方案列表 */
  const [searchText, setSearchText] = useState('')
  /** 当前分类页签（行业名，或「自定义」） */
  const [activeTab, setActiveTab] = useState('')
  /** 左侧选中的方案 key */
  const [selectedKey, setSelectedKey] = useState('')

  // 右侧表格
  const [funcType, setFuncType] = useState<FuncTypeFilter>('')
  const [keyword, setKeyword] = useState('')
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(10)

  // 编辑功能项（新增与查看复用产品管理的抽屉）
  const [formOpen, setFormOpen] = useState(false)
  const [formType, setFormType] = useState<RowType>('property')
  const [editingIdentifier, setEditingIdentifier] = useState('')
  const [formSubmitting, setFormSubmitting] = useState(false)

  const canList = useAuth(P.SolutionList)
  const canApply = useAuth(P.SolutionApply)
  const canTslAdd = useAuth(PERM_TSL_ADD)
  const canTslEdit = useAuth(PERM_TSL_EDIT)
  const canTslRemove = useAuth(PERM_TSL_REMOVE)

  /** 重拉全量方案：新增/编辑/删除后调用，表格与左侧列表一起刷新 */
  const loadSolutions = useCallback(async () => {
    try {
      setLoading(true)
      const resp = await getSolutions<Solution[]>()
      if (resp.success) {
        setSolutions(resp.result || [])
      } else {
        messageApi.open({ type: 'error', content: resp.errorMsg || t('方案列表加载失败') })
      }
    } catch (error) {
      messageApi.open({ type: 'error', content: t('方案列表加载失败，请稍后重试') })
    } finally {
      setLoading(false)
    }
  }, [messageApi])

  useEffect(() => {
    if (!canList) return
    loadSolutions()
  }, [])

  // 搜索框挂到 Layout 顶部 banner 的插槽上（banner 由 Layout 渲染，需等挂载后再取节点）
  useEffect(() => {
    setBannerSlot(document.getElementById('page-banner-extra'))
  }, [])

  /** 分类页签：行业去重排序 + 末尾「自定义」 */
  const industryTabs = useMemo(() => {
    const set = new Set<string>()
    solutions.forEach(item => set.add(item.industry || t('未分类')))
    const list = Array.from(set).sort((a, b) => a.localeCompare(b, 'zh-Hans-CN'))
    return [...list, CUSTOM_TAB]
  }, [solutions])

  /** 默认选中第一个行业页签；页签消失时回落到第一个 */
  useEffect(() => {
    if (!solutions.length) {
      setActiveTab('')
      return
    }
    if (!industryTabs.includes(activeTab)) setActiveTab(industryTabs[0])
  }, [industryTabs, activeTab, solutions.length])

  /** 当前页签下的方案（不含搜索） */
  const tabSolutions = useMemo(() => {
    if (activeTab === CUSTOM_TAB) return solutions.filter(item => !item.builtIn)
    return solutions.filter(item => (item.industry || t('未分类')) === activeTab)
  }, [solutions, activeTab])

  /** 左侧列表：有搜索词时按方案名称或行业匹配全量，否则用当前页签全量 */
  const listSolutions = useMemo(() => {
    const text = searchText.trim().toLowerCase()
    const base = text
      ? solutions.filter(item => (item.name || '').toLowerCase().includes(text)
        || (item.industry || '').toLowerCase().includes(text))
      : tabSolutions
    return [...base].sort((a, b) => (a.name || a.key).localeCompare(b.name || b.key, 'zh-Hans-CN'))
  }, [solutions, tabSolutions, searchText])

  /**
   * 选中方案不在当前列表里（首次加载 / 切页签 / 搜索）时，
   * 自动选中新列表的第一项，保证右侧表格始终有数据。
   */
  useEffect(() => {
    if (listSolutions.some(item => item.key === selectedKey)) return
    setSelectedKey(listSolutions[0]?.key || '')
  }, [listSolutions, selectedKey])

  /** 筛选条件或选中方案变化时回到第一页 */
  useEffect(() => {
    setPage(1)
  }, [activeTab, searchText, funcType, keyword, selectedKey])

  const currentSolution = useMemo(
    () => solutions.find(item => item.key === selectedKey) || null,
    [solutions, selectedKey],
  )

  /** 复用的产品物模型抽屉：方案物模型提交走 onSubmit，产品 id 只用于 payload 兜底 */
  const detailProductInfo = useMemo(
    () => (currentSolution ? ({ id: currentSolution.key } as ProductInfo) : undefined),
    [currentSolution],
  )

  /** 物模型解析：失败与为空都按空表处理，页面不报错 */
  const thingModel = useMemo<ThingModel | null>(
    () => (currentSolution ? parseJson<ThingModel>(currentSolution.thingModelJson) : null),
    [currentSolution],
  )
  const thingRows = useMemo(() => buildThingRows(thingModel), [thingModel])

  const filteredRows = useMemo(() => {
    const text = keyword.trim().toLowerCase()
    return thingRows.filter(row => {
      if (funcType && row.type !== funcType) return false
      if (!text) return true
      return row.name.toLowerCase().includes(text) || row.identifier.toLowerCase().includes(text)
    })
  }, [thingRows, funcType, keyword])

  // ===== 物模型功能项：查看 / 新增 / 编辑 / 删除 =====

  /**
   * 「创建产品」：跳到产品管理并预选当前方案的品类（sol_<key> 即后端品类 id/category_key）。
   * 「自定义」页签下没有选中方案时也要能创建：此时只打开创建抽屉，不预选品类。
   */
  const goCreateProduct = () => {
    navigate('/gateway/product/manage', {
      state: {
        openAdd: true,
        ...(currentSolution
          ? {
            categoryTemplateId: 'sol_' + currentSolution.key,
            categoryName: localized(currentSolution.name, currentSolution.nameEn),
          }
          : {}),
      },
    })
  }

  /** 「添加物模型」：复用产品管理的抽屉，提交时落到方案物模型接口 */
  const handleThingModelSubmit = async (
    payload: BaseThingModelType,
  ): Promise<{ success: boolean, errorMsg?: string }> => {
    if (!currentSolution) return { success: false, errorMsg: t('请先选择左侧方案') }
    const built = buildSolutionThingModelItem(payload)
    if (!built) return { success: false, errorMsg: t('不支持的功能类型') }
    try {
      const resp = await addSolutionThingModelItem(currentSolution.key, {
        type: built.type,
        item: built.item,
      })
      if (resp.success) {
        // 成功提示由复用的抽屉统一给出，这里只负责刷新表格
        loadSolutions()
      }
      return { success: resp.success, errorMsg: resp.errorMsg }
    } catch (error) {
      return { success: false, errorMsg: t('物模型添加失败，请稍后重试') }
    }
  }

  const openView = (row: ThingRow) => {
    detailModalRef.current?.open(buildViewRecord(row))
  }

  const openEdit = (row: ThingRow) => {
    const raw = row.raw || {}
    const name = row.type === 'event'
      ? (raw.eventName || '')
      : row.type === 'action' ? (raw.serviceName || '') : (raw.name || '')
    setFormType(row.type)
    setEditingIdentifier(row.identifier === '-' ? '' : row.identifier)
    itemForm.resetFields()
    itemForm.setFieldsValue({
      type: row.type,
      name,
      nameEn: raw.nameEn || raw.eventNameEn || raw.serviceNameEn || '',
      identifier: row.identifier === '-' ? '' : row.identifier,
      description: raw.description || '',
      descriptionEn: raw.descriptionEn || '',
      dataType: raw.dataType || undefined,
      rwFlag: raw.rwFlag || 'R',
      min: raw.dataSpecs?.min,
      max: raw.dataSpecs?.max,
      step: raw.dataSpecs?.step,
      unit: raw.dataSpecs?.unit,
      unitName: raw.dataSpecs?.unitName,
      length: raw.dataSpecs?.length,
      enumList: asArray<DataSpecItem>(raw.dataSpecsList).length
        ? asArray<DataSpecItem>(raw.dataSpecsList).map(entry => ({ name: entry?.name, value: entry?.value }))
        : [{ name: '', value: '' }],
      eventType: raw.eventType || 'INFO',
      callType: raw.callType || 'ASYNC',
    })
    setFormOpen(true)
  }

  /** 表单值 → 接口 item：字段名随类型变化，dataSpecs / dataSpecsList 按类型组装 */
  const buildItemPayload = (values: ThingItemFormValues): Record<string, any> => {
    const item: Record<string, any> = {
      identifier: (values.identifier || '').trim(),
      description: values.description || '',
      descriptionEn: values.descriptionEn || '',
    }
    if (formType === 'property') {
      item.name = (values.name || '').trim()
      item.nameEn = (values.nameEn || '').trim()
      item.dataType = values.dataType
      item.rwFlag = values.rwFlag
      const dataType = (values.dataType || '').toUpperCase()
      if (NUMERIC_TYPE.includes(dataType)) {
        const dataSpecs: DataSpecs = {}
        if (values.min !== undefined && values.min !== null) dataSpecs.min = values.min
        if (values.max !== undefined && values.max !== null) dataSpecs.max = values.max
        if (values.step !== undefined && values.step !== null) dataSpecs.step = values.step
        if (!isEmptyValue(values.unit)) dataSpecs.unit = values.unit
        if (!isEmptyValue(values.unitName)) dataSpecs.unitName = values.unitName
        item.dataSpecs = dataSpecs
      } else if (dataType === 'TEXT') {
        if (values.length !== undefined && values.length !== null) {
          item.dataSpecs = { length: values.length }
        }
      } else if (ENUM_TYPE.includes(dataType)) {
        item.dataSpecsList = asArray(values.enumList)
          .filter(entry => entry && (!isEmptyValue(entry.name) || !isEmptyValue(entry.value)))
          .map(entry => ({ name: entry.name ?? '', value: entry.value ?? '' }))
      }
    } else if (formType === 'event') {
      item.eventName = (values.name || '').trim()
      item.eventNameEn = (values.nameEn || '').trim()
      item.eventType = values.eventType
    } else {
      item.serviceName = (values.name || '').trim()
      item.serviceNameEn = (values.nameEn || '').trim()
      item.callType = values.callType
    }
    return item
  }

  const handleItemSubmit = async () => {
    if (!currentSolution) return
    let values: ThingItemFormValues
    try {
      values = await itemForm.validateFields()
    } catch (error) {
      return
    }
    const item = buildItemPayload(values)
    setFormSubmitting(true)
    try {
      // 编辑保持自有表单：产品那个抽屉有 record 时是只读的，不能复用
      const resp = await editSolutionThingModelItem(currentSolution.key, {
        type: formType,
        identifier: editingIdentifier,
        item,
      })
      if (resp.success) {
        messageApi.open({ type: 'success', content: t('编辑成功') })
        await loadSolutions()
        setFormOpen(false)
      } else {
        messageApi.open({ type: 'error', content: resp.errorMsg || t('保存失败，请稍后重试') })
      }
    } catch (error) {
      messageApi.open({ type: 'error', content: t('保存失败，请稍后重试') })
    } finally {
      setFormSubmitting(false)
    }
  }

  const handleDelete = async (row: ThingRow) => {
    if (!currentSolution) return
    try {
      const resp = await deleteSolutionThingModelItem(currentSolution.key, {
        type: row.type,
        identifier: row.identifier,
      })
      if (resp.success) {
        messageApi.open({ type: 'success', content: t('删除成功') })
        await loadSolutions()
      } else {
        messageApi.open({ type: 'error', content: resp.errorMsg || t('删除失败，请稍后重试') })
      }
    } catch (error) {
      messageApi.open({ type: 'error', content: t('删除失败，请稍后重试') })
    }
  }

  // ===== 表格列 =====

  const columns: ColumnsType<ThingRow> = [
    {
      title: t('功能类型'),
      dataIndex: 'type',
      width: 110,
      render: (value: RowType) => <Tag color={TYPE_META[value].color}>{TYPE_META[value].text}</Tag>,
    },
    { title: t('功能名称'), dataIndex: 'name', width: 180, ellipsis: true },
    { title: t('标识符'), dataIndex: 'identifier', width: 180, ellipsis: true },
    {
      title: t('数据类型'),
      key: 'dataType',
      width: 110,
      render: (_, record) => {
        if (record.type === 'property') {
          return DATA_TYPE_TEXT[(record.dataType || '').toUpperCase()] || record.dataType || '-'
        }
        if (record.type === 'event') return EVENT_TYPE[record.eventType] || record.eventType || '-'
        return CALL_TYPE[record.callType] || record.callType || '-'
      },
    },
    {
      title: t('读写类型'),
      key: 'rwFlag',
      width: 100,
      render: (_, record) => (record.type === 'property' ? (record.rwFlag || '-') : '-'),
    },
    {
      title: t('数据定义'),
      key: 'definition',
      render: (_, record) => (record.definition.length
        ? (
          <>
            {record.definition.map((line, index) => <div key={index}>{line}</div>)}
          </>
        )
        : '-'),
    },
    {
      title: t('操作'),
      key: 'action',
      width: 160,
      fixed: 'right',
      render: (_, record) => (
        <Space split={<Text type="secondary">/</Text>}>
          <Typography.Link onClick={() => openView(record)}>{t('查看')}</Typography.Link>
          {canTslEdit && <Typography.Link onClick={() => openEdit(record)}>{t('编辑')}</Typography.Link>}
          {canTslRemove && (
            <Popconfirm
              title={t('此操作将永久删除该记录, 是否继续?')}
              okText={t('删除')}
              cancelText={t('取消')}
              placement="topRight"
              onConfirm={() => handleDelete(record)}
            >
              <Typography.Link>{t('删除')}</Typography.Link>
            </Popconfirm>
          )}
        </Space>
      ),
    },
  ]

  /** 无列表权限时不请求、不渲染主体 */
  if (!canList) {
    return (
      <Card className="admin-page">
        {contextHolder}
        <Empty description={t('无方案列表查看权限')} />
      </Card>
    )
  }

  const tableEmpty = currentSolution
    ? (thingModel ? <Empty description={t('该方案暂无物模型预设')} /> : <Empty description={t('物模型预设无法解析')} />)
    : <Empty description={t('请先选择左侧方案')} />

  /** 编辑弹层表单：按功能类型切换额外字段 */
  const renderItemForm = () => (
    <Form form={itemForm} layout="vertical">
      <Form.Item label={t('功能类型')} name="type" rules={[{ required: true, message: t('请选择功能类型') }]}>
        <Radio.Group
          disabled
          onChange={event => setFormType(event.target.value as RowType)}
        >
          <Radio value="property">{t('属性')}</Radio>
          <Radio value="event">{t('事件')}</Radio>
          <Radio value="action">{t('服务')}</Radio>
        </Radio.Group>
      </Form.Item>
      <Form.Item
        label={t('功能名称')}
        name="name"
        rules={[{ required: true, message: t('请输入功能名称') }]}
      >
        <Input placeholder={t('请输入功能名称')} maxLength={64} />
      </Form.Item>
      <Form.Item label={t('英文名称')} name="nameEn">
        <Input placeholder={t('English name')} maxLength={64} />
      </Form.Item>
      <Form.Item
        label={t('标识符')}
        name="identifier"
        rules={[{ required: true, message: t('请输入标识符') }]}
        tooltip={t('标识符不可修改')}
      >
        <Input placeholder={t('请输入标识符')} maxLength={64} disabled />
      </Form.Item>
      <Form.Item label={t('描述')} name="description">
        <Input.TextArea rows={2} placeholder={t('请输入描述')} maxLength={200} />
      </Form.Item>
      <Form.Item label={t('英文描述')} name="descriptionEn">
        <Input.TextArea rows={2} placeholder={t('English description')} maxLength={200} />
      </Form.Item>

      {formType === 'property' && (
        <>
          <Form.Item
            label={t('数据类型')}
            name="dataType"
            rules={[{ required: true, message: t('请选择数据类型') }]}
          >
            <Select options={DATA_TYPE_OPTIONS} placeholder={t('请选择数据类型')} />
          </Form.Item>
          <Form.Item label={t('读写类型')} name="rwFlag" rules={[{ required: true, message: t('请选择读写类型') }]}>
            <Select
              options={[
                { value: 'R', label: 'R' },
                { value: 'RW', label: 'RW' },
                { value: 'W', label: 'W' },
              ]}
            />
          </Form.Item>
          <Form.Item noStyle shouldUpdate={(prev, next) => prev.dataType !== next.dataType}>
            {({ getFieldValue }) => {
              const dataType = String(getFieldValue('dataType') || '').toUpperCase()
              if (NUMERIC_TYPE.includes(dataType)) {
                return (
                  <>
                    <Space style={{ display: 'flex' }} align="start">
                      <Form.Item label={t('最小值')} name="min">
                        <InputNumber style={{ width: 150 }} placeholder={t('最小值')} />
                      </Form.Item>
                      <Form.Item label={t('最大值')} name="max">
                        <InputNumber style={{ width: 150 }} placeholder={t('最大值')} />
                      </Form.Item>
                      <Form.Item label={t('步长')} name="step">
                        <InputNumber style={{ width: 150 }} placeholder={t('步长')} />
                      </Form.Item>
                    </Space>
                    <Space style={{ display: 'flex' }} align="start">
                      <Form.Item label={t('单位')} name="unit">
                        <Input style={{ width: 150 }} placeholder={t('如 %')} />
                      </Form.Item>
                      <Form.Item label={t('单位名称')} name="unitName">
                        <Input style={{ width: 150 }} placeholder={t('如 百分比')} />
                      </Form.Item>
                    </Space>
                  </>
                )
              }
              if (ENUM_TYPE.includes(dataType)) {
                return (
                  <Form.List name="enumList">
                    {(fields, { add, remove }) => (
                      <>
                        {fields.map(({ key, name, ...restField }) => (
                          <Space key={key} align="baseline" style={{ display: 'flex', marginBottom: 4 }}>
                            <Form.Item
                              {...restField}
                              name={[name, 'name']}
                              rules={[{ required: true, message: t('请输入枚举名称') }]}
                            >
                              <Input style={{ width: 170 }} placeholder={t('枚举名称，如 开')} />
                            </Form.Item>
                            <Form.Item
                              {...restField}
                              name={[name, 'value']}
                              rules={[{ required: true, message: t('请输入枚举值') }]}
                            >
                              <Input style={{ width: 170 }} placeholder={t('枚举值，如 1')} />
                            </Form.Item>
                            <MinusCircleOutlined onClick={() => remove(name)} />
                          </Space>
                        ))}
                        <Button type="dashed" block icon={<PlusOutlined />} onClick={() => add()}>
                          {t('添加枚举项')}
                        </Button>
                      </>
                    )}
                  </Form.List>
                )
              }
              if (dataType === 'TEXT') {
                return (
                  <Form.Item label={t('数据长度')} name="length">
                    <InputNumber min={1} precision={0} style={{ width: 200 }} placeholder={t('如 1024')} />
                  </Form.Item>
                )
              }
              return null
            }}
          </Form.Item>
        </>
      )}

      {formType === 'event' && (
        <Form.Item
          label={t('事件类型')}
          name="eventType"
          rules={[{ required: true, message: t('请选择事件类型') }]}
        >
          <Select
            options={[
              { value: 'ALERT', label: t('告警') },
              { value: 'INFO', label: t('信息') },
              { value: 'ERROR', label: t('错误') },
            ]}
          />
        </Form.Item>
      )}

      {formType === 'action' && (
        <Form.Item
          label={t('调用方式')}
          name="callType"
          rules={[{ required: true, message: t('请选择调用方式') }]}
        >
          <Select
            options={[
              { value: 'ASYNC', label: t('异步') },
              { value: 'SYNC', label: t('同步') },
            ]}
          />
        </Form.Item>
      )}
    </Form>
  )

  return (
    <Card className="admin-page" bodyStyle={{ padding: 16 }}>
      {contextHolder}

      {/* 查看 / 新增物模型：复用产品管理的抽屉（有 record 时只读） */}
      <DetailModal
        ref={detailModalRef}
        productInfo={detailProductInfo}
        onSubmit={handleThingModelSubmit}
      />

      {/* 搜索框：投到 Layout 顶部 banner 的插槽上（压在 banner 上，与参考站一致） */}
      {bannerSlot
        && createPortal(
          <Input
            allowClear
            value={searchText}
            onChange={event => setSearchText(event.target.value)}
            placeholder={t('搜索分类...')}
            prefix={<SearchOutlined style={{ color: 'rgba(0,0,0,0.35)' }} />}
            style={{ width: 460, maxWidth: '100%', height: 40, borderRadius: 20 }}
          />,
          bannerSlot,
        )}

      {/* ===== 2) 分类 Tabs：行业去重 + 自定义 ===== */}
      {solutions.length > 0 && (
        <Tabs
          activeKey={activeTab}
          onChange={setActiveTab}
          items={industryTabs.map(key => ({ key, label: tt(key) }))}
        />
      )}

      {/* ===== 3) 左方案列表 + 4/5) 右工具栏与表格 ===== */}
      <div style={{ display: 'flex', gap: 16, alignItems: 'flex-start' }}>
        <div
          style={{
            flex: '0 0 260px',
            width: 260,
            maxHeight: 560,
            overflowY: 'auto',
            padding: 8,
            border: '1px solid #f0f0f0',
            borderRadius: 8,
          }}
        >
          {listSolutions.length === 0
            ? <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={t('暂无方案')} />
            : listSolutions.map(item => {
              const selected = item.key === selectedKey
              return (
                <div
                  key={item.key}
                  onClick={() => setSelectedKey(item.key)}
                  style={{
                    display: 'flex',
                    alignItems: 'center',
                    gap: 8,
                    marginBottom: 4,
                    padding: '8px 10px',
                    borderRadius: 6,
                    cursor: 'pointer',
                    border: `1px solid ${selected ? '#1677ff' : 'transparent'}`,
                    background: selected ? '#e6f4ff' : undefined,
                    color: selected ? '#1677ff' : undefined,
                  }}
                >
                  <span style={{ overflow: 'hidden', whiteSpace: 'nowrap', textOverflow: 'ellipsis' }}>
                    {localized(item.name, item.nameEn) || item.key}
                  </span>
                </div>
              )
            })}
        </div>

        <div style={{ flex: 1, minWidth: 0 }}>
          {/* 工具栏：左筛选 右操作 */}
          <div
            style={{
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'space-between',
              flexWrap: 'wrap',
              gap: 8,
              marginBottom: 12,
            }}
          >
            <Space wrap>
              <Select<FuncTypeFilter>
                value={funcType}
                onChange={setFuncType}
                style={{ minWidth: 180 }}
                options={[
                  { value: '', label: t('功能类型 (全部)') },
                  { value: 'property', label: t('属性') },
                  { value: 'event', label: t('事件') },
                  { value: 'action', label: t('服务') },
                ]}
              />
              <Input
                allowClear
                value={keyword}
                onChange={event => setKeyword(event.target.value)}
                placeholder={t('请输入名称搜索')}
                style={{ width: 220 }}
              />
            </Space>
            <Space wrap>
              {canTslAdd && (
                <Button
                  type="primary"
                  icon={<PlusOutlined />}
                  disabled={!currentSolution}
                  onClick={() => detailModalRef.current?.open()}
                >
                  {t('添加物模型')}
                </Button>
              )}
              <Button
                icon={<PlusOutlined />}
                style={{ background: '#faad14', borderColor: '#faad14', color: '#fff' }}
                disabled={!canApply}
                onClick={goCreateProduct}
              >
                {t('创建产品')}
              </Button>
            </Space>
          </div>

          <Table<ThingRow>
            rowKey={record => `${record.type}${record.identifier}`}
            size="small"
            bordered
            loading={loading}
            columns={columns}
            dataSource={filteredRows}
            scroll={{ x: 'max-content' }}
            pagination={{
              current: page,
              pageSize,
              showSizeChanger: true,
              pageSizeOptions: ['10', '20', '50'],
              showQuickJumper: true,
              showTotal: total => t('共 {{count}} 条', { count: total }),
              onChange: (nextPage, nextPageSize) => {
                setPage(nextPage)
                setPageSize(nextPageSize)
              },
            }}
            locale={{ emptyText: tableEmpty }}
          />
        </div>
      </div>

      {/* ===== 6) 编辑物模型：产品抽屉有 record 时只读，故编辑保留自有表单 ===== */}
      <Modal
        open={formOpen}
        title={t('编辑物模型')}
        width={640}
        okText={t('保存')}
        cancelText={t('取消')}
        maskClosable={false}
        confirmLoading={formSubmitting}
        onOk={handleItemSubmit}
        onCancel={() => setFormOpen(false)}
      >
        {renderItemForm()}
      </Modal>
    </Card>
  )
}

export default SolutionCenterPage
