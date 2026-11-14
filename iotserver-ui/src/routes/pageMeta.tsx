import type { ReactNode } from 'react'
import { lookupMenuName } from './menus'
import { t } from '@/i18n'

/**
 * 页面标题条（.advance-breadcrumb-title）元数据。
 *
 * 标题条由 Layout 统一渲染，不再由各页面自己拼：
 * - 标题优先取「菜单管理」里的菜单名（后端菜单 path → name），取不到时回落到这里的 title；
 *   因此在菜单管理里改菜单名，侧边栏、面包屑、页面标题会一起变。
 * - describe 是页面专属文案，菜单里没有，只能在这里维护。
 * - 少数详情页的标题/面包屑来自页面数据（设备名、规则名等），标 banner: 'page' 由页面自己渲染。
 */
export type BannerMode = 'layout' | 'page' | 'none'

export interface PageMeta {
  /** 标题。一般留空即可，直接用菜单名；仅给不在菜单里的页面兜底 */
  title?: string
  /** 标题条描述文案 */
  describe?: ReactNode
  /** 标题条渲染方式。默认 layout：Layout 统一渲染；page：页面自己渲染；none：不显示 */
  banner?: BannerMode
  /** 带动态段的路由用：标题取这条菜单路径的菜单名 */
  menuPath?: string
  /**
   * 标题条版式：
   * - default：左侧标题 + 描述，右侧插槽；
   * - centered：不显示标题（菜单/面包屑已有），插槽（如搜索框）与描述在标题条内居中堆叠。
   */
  bannerLayout?: 'default' | 'centered'
}

const pageMetas: Record<string, PageMeta> = {
  // ===== 首页 =====
  '/home/dashboard': {
    // 首页整页渲染「首页看板」，不要标题条（标题仍会体现在浏览器标签上）
    banner: 'none',
    describe: t('展示「看板管理」中设为首页的看板；内置看板汇总产品、设备、网关、告警与消息量趋势，布局和指标可调整。'),
  },
  // ===== 设备接入 =====
  '/gateway/product/manage': {
    describe: t('定义同类设备的产品信息与物模型（属性、事件、服务）；创建设备时需选择所属产品。'),
  },
  '/gateway/device/manage': {
    describe: t('管理平台设备，支持单个创建、批量导入和搜索；设备需先创建并绑定产品才能接入。'),
  },
  '/gateway/gateway/manage': {
    describe: t('注册第三方工业网关，查看 MQTT 接入信息、配置下发与在线状态。'),
  },
  '/gateway/drive/image': {
    describe: t('驱动市场、Docker 镜像和内置系统组件的统一入口；嵌入式部署自动隐藏镜像相关功能。'),
  },
  '/gateway/my/drive': {
    describe: t('管理已安装的驱动实例，查看运行状态、配置参数和实时日志。'),
  },
  // ===== 方案中心 =====
  '/solution/center': {
    // 版式对齐参考站：标题条内只放「居中搜索框 + 下方说明文案」，标题不重复显示（菜单/面包屑已有）
    bannerLayout: 'centered',
    describe: t('提供行业产品模板，可一键创建对应产品。'),
  },
  // ===== 数据中心 =====
  '/data/history': {
    describe: t('按产品、设备和参数检索历史数据，支持多行条件对比，结果以折线图和表格展示，可导出 CSV。'),
  },
  // ===== 运维监控 =====
  '/monitor/systemMonitor': {
    // 与首页一致：整页直接渲染系统监控看板，不要标题条
    banner: 'none',
    describe: t('展示平台自身 CPU、系统负载、内存和磁盘等运行指标，可按时间范围查看趋势。'),
  },
  '/monitor/alarmCenter/list': {
    describe: t('为业务指标配置告警规则；触发后通过告警列表、邮件、短信、WhatsApp、Webhook 等方式通知。'),
  },
  // ===== 系统管理 =====
  '/admin/sysUser': {
    describe: t('维护平台账号，支持新增、编辑、分配部门与角色、重置密码和启停账号。'),
  },
  '/admin/sysRole': {
    describe: t('定义角色的菜单与按钮权限；用户分配角色后，可见菜单和可执行操作由角色决定。'),
  },
  '/admin/sysMenu': {
    describe: t('维护侧边栏的目录、菜单和按钮权限标识；名称、路径、图标、排序与显示状态会同步到侧边栏。'),
  },
  '/admin/sysDept': {
    describe: t('维护组织架构树，用户归属部门后可用于划分数据范围和职责。'),
  },
  '/admin/sysAuditLog': {
    describe: t('记录登录与关键操作：操作人、时间、模块、动作、结果和耗时。'),
  },
  '/platform/sysSet': {
    describe: t('维护平台品牌与安全策略：系统名称、Logo、浏览器标签、登录页背景、会话超时和密码强度等。'),
  },
  // ===== 平台管理（菜单由后端只下发给平台管理员） =====
  '/platform/tenant': {
    describe: t('维护租户、资源配额和到期时间；平台管理员可切换到租户视图查看和代管数据。'),
  },
  // ===== 个人中心 / 工具与文档 / 数据看板 =====
  '/common/userProfile': {
    describe: t('查看和修改当前账号资料，修改登录密码。'),
  },
  '/common/embed': {
    title: t('嵌入页面'),
    banner: 'none',
    describe: t('以 iframe 嵌入菜单指向的页面。'),
  },
  '/documents/documents': {
    // 产品介绍页：整页直接展示 hero + 能力介绍，不要标题条
    banner: 'none',
    describe: t('iotserver 物联网平台的产品介绍、核心能力与快速开始。'),
  },
  '/viz/dashboard': {
    describe: t('实时呈现平台运行数据，表格和图表自动刷新，布局可拖动调整。'),
  },
  '/viz/boards': {
    describe: t('新建、编辑、删除看板，并指定「数据看板」整页使用哪一块看板。'),
  },
  // ===== 不在侧边栏、由首页快捷入口等进入的页面 =====
  '/advanced/rule/engine': {
    title: t('规则引擎'),
    describe: t('对接入平台的设备数据进行过滤、转换，并推送到指定消息目的地；规则需配置消息源、过滤条件和目的地。'),
  },
  '/advanced/resource/manage': {
    title: t('资源管理'),
    describe: t('提供消息通信中间件资源，可作为规则引擎的消息目的地，将数据推送到应用平台。'),
  },
  '/advanced/scene/linkage': {
    title: t('场景联动'),
    describe: t('用设备或时间条件编排自动化逻辑，自定义设备间的联动规则。'),
  },
  '/cloud/service': {
    title: t('插件市场'),
    describe: t('提供云插件，将设备数据上报到各大物联网平台；统一 SDK 屏蔽厂商协议差异，可按需下载使用。'),
  },
  '/cloud/my/service': {
    title: t('我的服务'),
    describe: t('汇总当前账号已开通的云插件服务，查看授权状态、运行状态和调用日志。'),
  },
}

/** 标题/面包屑依赖页面数据的详情页：标题条由页面自己渲染，这里的 title 只用于浏览器标签 */
const pageRenderedRules: Array<{ test: RegExp, title: string, menuPath?: string }> = [
  { test: /^\/gateway\/product\/detail\//, title: t('产品详情'), menuPath: '/gateway/product/manage' },
  { test: /^\/gateway\/device\/detail\//, title: t('设备详情'), menuPath: '/gateway/device/manage' },
  { test: /^\/monitor\/alarmCenter\/detail\//, title: t('规则详情'), menuPath: '/monitor/alarmCenter/list' },
  { test: /^\/advanced\/scene\/log\//, title: t('联动日志'), menuPath: '/advanced/scene/linkage' },
]

/** 带动态段但仍是列表页的路径：按 basePath 取所属菜单的标题条 */
const prefixMetas: Array<{ test: RegExp, basePath: string, meta: PageMeta }> = [
  { test: /^\/gateway\/device\/manage\//, basePath: '/gateway/device/manage', meta: pageMetas['/gateway/device/manage'] },
]

/**
 * 解析当前路径对应的标题条元数据。
 * banner 为 'page' 表示页面自己渲染，'layout' 表示由 Layout 统一渲染。
 */
export function resolvePageMeta (pathname: string): PageMeta {
  const pageRendered = pageRenderedRules.find(rule => rule.test.test(pathname))
  if (pageRendered) {
    return { banner: 'page', title: pageRendered.title }
  }
  const exact = pageMetas[pathname]
  if (exact) return { banner: 'layout', ...exact }
  const prefixed = prefixMetas.find(item => item.test.test(pathname))
  if (prefixed) return { banner: 'layout', menuPath: prefixed.basePath, ...prefixed.meta }
  // 没登记的路径：仍然交给 Layout 渲染（标题取菜单名），避免新增页面漏掉标题条
  return { banner: 'layout' }
}

/**
 * 页面标题：优先取「菜单管理」里的菜单名，取不到再回落到元数据里的兜底标题。
 * 标题条与浏览器标签都用它，保证两处一致。
 */
export function resolvePageTitle (pathname: string): string | undefined {
  const meta = resolvePageMeta(pathname)
  return lookupMenuName(meta.menuPath || pathname) || meta.title
}

/**
 * 当前路径应在侧边栏选中的菜单项。
 * 详情页等带动态段的路由没有一一对应的菜单（如 /gateway/device/manage/:productId、
 * /gateway/product/detail/:id），回落到所属列表菜单，避免跳转后侧边栏没有任何选中项。
 */
export function resolveMenuPath (pathname: string): string | undefined {
  const pageRendered = pageRenderedRules.find(rule => rule.test.test(pathname))
  if (pageRendered?.menuPath) return pageRendered.menuPath
  const prefixed = prefixMetas.find(item => item.test.test(pathname))
  if (prefixed) return prefixed.basePath
  return undefined
}
