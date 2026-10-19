import i18n from 'i18next'
import dayjs from 'dayjs'
import 'dayjs/locale/zh-cn'
import enUS from './locales/en-US.json'

/**
 * 国际化（i18n）约定
 *
 * 1. 词条 key 直接使用「中文原文」，例如 t('设备名称')。
 *    这样老代码迁移时不需要先发明一套 key，未翻译的语言也能自动回落到中文原文，
 *    后端下发的菜单名（本身就是中文）也能直接 t(菜单名) 翻译。
 * 2. 因此中文不需要资源文件（回落到 key 即为中文），英文资源在 locales/en-US.json。
 *    中文文案需要改动时，记得同步改 en-US.json 里对应的 key。
 * 3. 带变量的文案统一用 {{name}} 占位：t('共 {{count}} 条', { count: 5 })。
 *    count 会触发 i18next 的复数处理，英文资源可写 `xxx_one` / `xxx_other` 两条。
 * 4. 组件外（模块级常量、工具函数、api 层）直接用导出的 t()；
 *    切换语言走 setLocale()，它会整页刷新，保证模块级常量也重新取到新语言。
 *
 * 为什么不用 react-i18next：它的类型声明会 `declare module 'react'` 出一个 JSX 命名空间，
 * 在 TS 4.8 下会顶掉 @types/react 的全局 JSX，导致全项目 JSX 类型失效（TS7026）。
 * 这里只需要 i18next 本身，语言切换靠整页刷新 + LocaleProvider 订阅语言事件即可。
 */
export type Locale = 'zh-CN' | 'en-US'

export const LOCALE_STORAGE_KEY = '$locale$'

export const DEFAULT_LOCALE: Locale = 'zh-CN'

export const LOCALES: ReadonlyArray<{ value: Locale, label: string }> = [
  { value: 'zh-CN', label: '简体中文' },
  { value: 'en-US', label: 'English' },
]

export function isLocale (value: unknown): value is Locale {
  return value === 'zh-CN' || value === 'en-US'
}

/** 跟随浏览器语言：zh* → 中文，en* → 英文，其它回落到默认语言 */
function browserLocale (): Locale {
  const candidates = [
    ...(Array.isArray(navigator.languages) ? navigator.languages : []),
    navigator.language,
  ]
  for (const lang of candidates) {
    if (!lang) continue
    if (/^zh\b|^zh-/i.test(lang)) return 'zh-CN'
    if (/^en\b|^en-/i.test(lang)) return 'en-US'
  }
  return DEFAULT_LOCALE
}

/** 把后端 / 浏览器给的语言值归一化：zh* → zh-CN，en* → en-US，其它返回 null */
export function normalizeLocale (raw?: string | null): Locale | null {
  if (!raw) return null
  if (/^zh/i.test(raw)) return 'zh-CN'
  if (/^en/i.test(raw)) return 'en-US'
  return null
}

/** 当前语言：localStorage 优先（用户显式选过），其次跟随浏览器 */
export function getLocale (): Locale {
  try {
    const stored = localStorage.getItem(LOCALE_STORAGE_KEY)
    if (isLocale(stored)) return stored
  } catch (error) {
    // 无痕模式等禁用 localStorage 的场景：忽略，继续跟随浏览器
  }
  return browserLocale()
}

/** i18n 实例里的当前语言（可能是 zh-CN / en-US 的别名，统一归一化） */
export function currentLocale (): Locale {
  return i18n.language && /^en/i.test(i18n.language) ? 'en-US' : 'zh-CN'
}

/** antd / dayjs 认的语言标识 */
export function dayjsLocale (locale: Locale = currentLocale()): string {
  return locale === 'en-US' ? 'en' : 'zh-cn'
}

/** 把语言同步到 document / dayjs（antd 由 LocaleProvider 负责） */
function applyRuntimeLocale (locale: Locale) {
  if (typeof document !== 'undefined') {
    document.documentElement.setAttribute('lang', locale)
  }
  // 日期格式化跟随语言；'en' 是 dayjs 内置语言，不需要额外引入
  dayjs.locale(dayjsLocale(locale))
}

i18n.init({
  lng: getLocale(),
  fallbackLng: DEFAULT_LOCALE,
  supportedLngs: ['zh-CN', 'en-US'],
  // 中文没有资源文件，缺失即回落到 key（中文原文）
  resources: { 'en-US': { translation: enUS } },
  // key 是中文原文，里面可能出现 . 和 :，不能当层级/命名空间分隔符拆开
  keySeparator: false,
  nsSeparator: false,
  interpolation: { escapeValue: false },
  returnNull: false,
})

applyRuntimeLocale(currentLocale())

/** 切换语言：写入 localStorage 并整页刷新（模块级常量随刷新一起换语言） */
export function setLocale (locale: Locale) {
  try {
    localStorage.setItem(LOCALE_STORAGE_KEY, locale)
  } catch (error) {
    // 写不进去也照样切，只是刷新后回到浏览器语言
  }
  if (currentLocale() === locale) {
    applyRuntimeLocale(locale)
    return
  }
  i18n.changeLanguage(locale).finally(() => {
    applyRuntimeLocale(locale)
    window.location.reload()
  })
}

/** 翻译函数：组件内外通用。取不到英文时回落到中文原文 */
export function t (key: string, params?: Record<string, unknown>): string {
  return i18n.t(key, params) as string
}

/**
 * 翻译「来自数据的展示文案」：后端下发的菜单名、看板里配置的组件标题/标签/单位等。
 * 词条里有就翻，没有就原样显示——所以用户自己起的名字不会被改坏。
 * 带 {{ }} 的值直接原样返回，避免被当成插值模板处理。
 */
export function tt (value?: string | null): string {
  if (!value) return ''
  if (value.includes('{{') || value.includes('}}')) return value
  return i18n.exists(value) ? t(value) : value
}

/** 按当前语言取一个「中英文二选一」的值（用于后端返回的字段只有中文时的兜底） */
export function pick (zh: string, en: string): string {
  return currentLocale() === 'en-US' ? en : zh
}

export default i18n
