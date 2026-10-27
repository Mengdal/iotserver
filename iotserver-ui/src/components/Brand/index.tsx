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

/** 根据地址推断 favicon 的 MIME；无法判断时不写 type，交给浏览器嗅探 */
function faviconMime (href: string): string {
  if (href.startsWith('data:image/svg')) return 'image/svg+xml'
  if (href.startsWith('data:image/png')) return 'image/png'
  if (href.startsWith('data:image/jpeg') || href.startsWith('data:image/jpg')) return 'image/jpeg'
  if (href.startsWith('data:image/x-icon') || href.startsWith('data:image/vnd.microsoft.icon')) return 'image/x-icon'
  if (/\.svg(\?|$)/i.test(href)) return 'image/svg+xml'
  if (/\.png(\?|$)/i.test(href)) return 'image/png'
  if (/\.ico(\?|$)/i.test(href)) return 'image/x-icon'
  return ''
}

/** 把任意图片转成 64×64 PNG（浏览器对 JPEG favicon 支持不一致） */
function toPngDataUrl (src: string, size = 64): Promise<string> {
  return new Promise(resolve => {
    const img = new Image()
    img.onload = () => {
      const canvas = document.createElement('canvas')
      canvas.width = size
      canvas.height = size
      const ctx = canvas.getContext('2d')
      if (!ctx) {
        resolve(src)
        return
      }
      ctx.drawImage(img, 0, 0, size, size)
      try {
        resolve(canvas.toDataURL('image/png'))
      } catch (error) {
        resolve(src)
      }
    }
    img.onerror = () => resolve(src)
    img.src = src
  })
}

let faviconSeq = 0

/** 把 data URL 或图片地址设置为浏览器标签图标（始终有值） */
async function applyFavicon (href: string) {
  const seq = ++faviconSeq
  let src = href
  // 历史上传的 favicon 可能是 JPEG：先转成 PNG 再设置
  if (src.startsWith('data:image/jpeg') || src.startsWith('data:image/jpg')) {
    src = await toPngDataUrl(src)
    if (seq !== faviconSeq) return
  }
  // index.html 自带一条默认 favicon；不清掉的话多条 rel=icon 可能被浏览器选错
  document.querySelectorAll<HTMLLinkElement>('link[rel~="icon"]').forEach(link => {
    if (!link.hasAttribute('data-brand-favicon')) link.remove()
  })
  let link = document.querySelector<HTMLLinkElement>('link[data-brand-favicon]')
  if (!link) {
    link = document.createElement('link')
    link.rel = 'icon'
    link.setAttribute('data-brand-favicon', '1')
    document.head.appendChild(link)
  }
  const mime = faviconMime(src)
  if (mime) link.type = mime
  else link.removeAttribute('type')
  link.href = src
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
    applyFavicon(assets.favicon).catch(() => { /* 图标加载失败不影响页面 */ })
    document.documentElement.setAttribute('data-site-name', site.name || '')
  }, [assets.favicon, site.name])

  // 登录页等「未进入主框架」的页面，标题统一为「登录 - 系统名称」
  useEffect(() => {
    const name = site.name || 'iotserver'
    const loginTitle = t('登录')
    // index.html 里的初始标题是中文的「登录」，英文环境下也要认出「还没设置过标题」
    if (!document.title || document.title === '登录' || document.title === loginTitle) {
      document.title = t('登录 - {{name}}', { name })
    }
  }, [site.name])

  const value = useMemo(() => ({ site, assets, reload: load }), [site, assets, load])

  return <BrandContext.Provider value={value}>{children}</BrandContext.Provider>
}

export const useBrand = () => useContext(BrandContext)
export default BrandContext
