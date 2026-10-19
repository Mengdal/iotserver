import React, { createContext, useCallback, useContext, useEffect, useMemo, useState } from 'react'
import { getSiteInfo, SiteInfo } from '@/api'

/** 系统默认素材：未上传时回退到这些内置资源 */
import defaultLogo from '@/assets/images/logo-hex.svg'
import defaultLoginBackground from '@/assets/images/background.png'
import { t } from '@/i18n'
const DEFAULT_LOGO = defaultLogo as string
const DEFAULT_LOGIN_BACKGROUND = defaultLoginBackground as string
const DEFAULT_FAVICON = '/favicon.ico'
// 登录页与顶栏用同一个矢量 Logo（六边形环，任意尺寸都清晰）
const DEFAULT_LOGIN_LOGO = DEFAULT_LOGO

const DEFAULT_SITE: SiteInfo = {
  name: t('iotserver 物联网平台'),
  logo: '',
  favicon: '',
  loginBackground: '',
  copyright: '© iotserver',
  description: t('iotserver 物联网平台'),
}

interface BrandContextValue {
  /** 原始配置（未上传的项为空字符串） */
  site: SiteInfo
  /** 解析后的资源：未上传时回退到系统默认 */
  assets: {
    logo: string
    loginLogo: string
    favicon: string
    loginBackground: string
  }
  /** 重新拉取站点配置（系统设置保存后调用，立即生效） */
  reload: () => Promise<void>
}

const DEFAULT_ASSETS = {
  logo: DEFAULT_LOGO,
  loginLogo: DEFAULT_LOGIN_LOGO,
  favicon: DEFAULT_FAVICON,
  loginBackground: DEFAULT_LOGIN_BACKGROUND,
}

const BrandContext = createContext<BrandContextValue>({
  site: DEFAULT_SITE,
  assets: DEFAULT_ASSETS,
  reload: async () => {},
})

/** 把 data URL 或图片地址设置为浏览器标签图标（始终有值） */
function applyFavicon (href: string) {
  const selector = 'link[data-brand-favicon]'
  let link = document.querySelector<HTMLLinkElement>(selector)
  if (!link) {
    link = document.createElement('link')
    link.rel = 'icon'
    link.setAttribute('data-brand-favicon', '1')
    document.head.appendChild(link)
  }
  link.type = href.startsWith('data:') && href.includes('svg') ? 'image/svg+xml' : 'image/x-icon'
  link.href = href
}

export const BrandProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const [site, setSite] = useState<SiteInfo>(DEFAULT_SITE)

  const load = useCallback(async () => {
    try {
      const resp = await getSiteInfo()
      if (resp.success && resp.result) {
        setSite({ ...DEFAULT_SITE, ...resp.result })
      }
    } catch (error) {
      // 接口异常时保留默认品牌，不阻塞页面
    }
  }, [])

  useEffect(() => {
    load()
  }, [load])

  // 未上传时回退到系统默认素材
  const assets = useMemo(() => ({
    // 上传的 logo 同时用于顶栏与登录页；未上传时各自回退到合适的系统默认素材
    logo: site.logo || DEFAULT_LOGO,
    loginLogo: site.logo || DEFAULT_LOGIN_LOGO,
    favicon: site.favicon || DEFAULT_FAVICON,
    loginBackground: site.loginBackground || DEFAULT_LOGIN_BACKGROUND,
  }), [site.logo, site.favicon, site.loginBackground])

  // 应用浏览器标签文字与图标
  useEffect(() => {
    applyFavicon(assets.favicon)
    document.documentElement.setAttribute('data-site-name', site.name || '')
  }, [assets.favicon, site.name])

  // 登录页等「未进入主框架」的页面，标题统一为「登录 - 系统名称」
  useEffect(() => {
    const name = site.name || 'iotserver'
    const loginTitle = t('登录')
    // index.html 里的初始标题是中文的「登录」，英文环境下也要认出「还没设置过标题」
    if (!document.title || document.title === '登录' || document.title === loginTitle) {
      document.title = t('登录 - {{name}}', { name: name })
    }
  }, [site.name])

  const value = useMemo(() => ({ site, assets, reload: load }), [site, assets, load])

  return <BrandContext.Provider value={value}>{children}</BrandContext.Provider>
}

export const useBrand = () => useContext(BrandContext)
export default BrandContext
