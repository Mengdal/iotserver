import type { ReactNode } from 'react'
import { lookupMenuName } from './menus'

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
}

const pageMetas: Record<string, PageMeta> = {
  // ===== 首页 =====
  '/home/dashboard': {
    // 首页是看板式布局，不要标题条（标题仍会体现在浏览器标签上）
    banner: 'none',
    describe: '罗米物联网平台提供设备接入、数据查询、运维监控与告警处理等能力，本页汇总平台的产品、设备、云插件与告警概况，可快速进入常用功能。',
  },
  // ===== 设备接入 =====
  '/gateway/product/manage': {
    describe: '在物联网平台中，某一类具有相同能力或特征的设备的合集被称为一款产品。罗米物联网平台已经打通了市面上主流iot云平台，可以把云平台产品数据同步到本地，第一次使用请点击数据同步按钮进行数据同步。',
  },
  '/gateway/device/manage': {
    describe: '物理设备要连接到平台，需要先在平台创建设备(支持单个或批量导入创建)。设备列表包含自主创建的设备和云平台同步的设备，同时支持灵活的搜索。',
  },
  '/gateway/drive/image': {
    describe: '驱动镜像是用户编写程序最终打包成可下载的docker镜像，它是实体设备与物联网平台通讯的桥梁，所有的设备数据由驱动上报到物联网平台，所有的设备控制指令由物联网平台下发给驱动。',
  },
  '/gateway/my/drive': {
    describe: '我的驱动用于管理已安装到本地的设备驱动实例，支持查看驱动运行状态、配置参数与实时日志，是设备接入平台的运行载体。',
  },
  // ===== 数据中心 =====
  '/data/history': {
    describe: '历史查询支持按产品、设备与参数检索设备上报的历史数据，可添加多行条件做多设备对比，结果以折线图与表格展示，并支持导出 CSV。',
  },
  // ===== 运维监控 =====
  '/monitor/systemMonitor': {
    describe: '系统监控展示平台自身的运行指标，包括 CPU、内存、网络与磁盘等资源使用情况，可按时间范围查看趋势，及时发现服务负载异常。',
  },
  '/monitor/alarmCenter/list': {
    describe: '平台提供监控告警功能，用户可以针对自己关心的业务指标设置告警规则，当监控指标满足条件触发后，支持以告警列表、钉钉机器人、微信机器人等方式通知用户，帮助用户快速处理故障，避免业务损失。',
  },
  // ===== 系统管理 =====
  '/admin/sysUser': {
    describe: '用户管理用于维护平台账号，支持新增与编辑用户、分配部门与角色、重置密码以及启用或停用账号。',
  },
  '/admin/sysRole': {
    describe: '角色管理用于定义角色的菜单与按钮权限，给用户分配角色后，其可见菜单与可执行操作即由角色权限决定。',
  },
  '/admin/sysMenu': {
    describe: '菜单管理维护侧边栏的目录、菜单与按钮权限标识，菜单名称、路径、图标、排序与显示状态都会同步到侧边栏。',
  },
  '/admin/sysDept': {
    describe: '部门管理维护平台的组织结构树，用户可归属到具体部门，用于划分数据范围与职责。',
  },
  '/admin/sysLoginLog': {
    describe: '登入日志记录账号的登录时间、IP、浏览器与登录结果，便于排查异常登录与安全审计。',
  },
  '/admin/sysSet': {
    describe: '系统配置维护平台品牌与安全策略，包括系统名称、Logo、浏览器标签、登录页背景、会话超时时间与密码强度等。',
  },
  // ===== 个人中心 / 工具与文档 / 数据看板 =====
  '/common/userProfile': {
    describe: '个人中心可查看与修改当前账号的资料，并支持修改登录密码。',
  },
  '/documents/documents': {
    describe: '工具与文档汇总平台的使用说明、接口文档与常用工具，帮助快速上手与二次开发。',
  },
  '/viz/dashboard': {
    describe: '实时呈现平台运行数据：表格与图表随数据自动刷新，布局可直接拖动调整。',
  },
  '/viz/boards': {
    describe: '看板管理：新建、编辑、删除看板，并指定「数据看板」整页使用哪一块看板。',
  },
  // ===== 不在侧边栏、由首页快捷入口等进入的页面 =====
  '/advanced/rule/engine': {
    title: '规则引擎',
    describe: '规则引擎提供数据流转能力，可对接入平台的设备数据进行过滤转换，并将数据推送至用户指定的消息目的地。规则引擎流转规则需要配置消息源（推送消息类型）、条件过滤规则及消息目的地（推送方式）。',
  },
  '/advanced/resource/manage': {
    title: '资源管理',
    describe: '平台提供多种消息通信中间件资源，资源可作为规则引擎的消息目的地，通过创建资源快速将数据推送至应用平台。',
  },
  '/advanced/scene/linkage': {
    title: '场景联动',
    describe: '场景联动是一种开发自动化业务逻辑的编程方式，目前支持设备、时间条件触发，您可以自定义设备之间的联动规则，系统执行自定义的业务逻辑，满足多场景联动需求。',
  },
  '/cloud/service': {
    title: '插件市场',
    describe: '罗米物联网平台提供云插件功能，支持把设备数据上报到各大物联网平台中。罗米物联网平台屏蔽了各大厂商协议的差异性，相同的sdk方法可以支持多个物联网平台，用户可按需下载进行使用。',
  },
  '/cloud/my/service': {
    title: '我的服务',
    describe: '我的服务汇总当前账号已开通的云插件服务，可查看授权状态、运行状态与调用日志。',
  },
}

/** 标题/面包屑依赖页面数据的详情页：标题条由页面自己渲染，这里的 title 只用于浏览器标签 */
const pageRenderedRules: Array<{ test: RegExp, title: string }> = [
  { test: /^\/gateway\/product\/detail\//, title: '产品详情' },
  { test: /^\/gateway\/device\/detail\//, title: '设备详情' },
  { test: /^\/monitor\/alarmCenter\/detail\//, title: '规则详情' },
  { test: /^\/advanced\/scene\/log\//, title: '联动日志' },
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

export default pageMetas
