import React, { useEffect, useState } from 'react'
import { ConfigProvider } from 'antd'
import antdEnUS from 'antd/locale/en_US'
import antdZhCN from 'antd/locale/zh_CN'
import i18n, { currentLocale } from './index'
import type { Locale } from './index'

/**
 * 语言 → antd 内建文案（分页、日期选择器、空状态、上传等）。
 * 放在最外层：App.tsx 里的 ConfigProvider 只覆盖主题，语言会从这里继承。
 */
export const LocaleProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  // 订阅语言变化：切换语言会整页刷新，这里同时保证「不刷新也能跟着变」
  const [locale, setLocaleState] = useState<Locale>(currentLocale)
  useEffect(() => {
    const onChange = () => setLocaleState(currentLocale())
    i18n.on('languageChanged', onChange)
    return () => i18n.off('languageChanged', onChange)
  }, [])

  return (
    <ConfigProvider locale={locale === 'en-US' ? antdEnUS : antdZhCN}>
      {children}
    </ConfigProvider>
  )
}

export default LocaleProvider
