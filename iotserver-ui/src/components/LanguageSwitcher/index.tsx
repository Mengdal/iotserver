import React from 'react'
import { Button, Dropdown } from 'antd'
import { GlobalOutlined } from '@ant-design/icons'
import { LOCALES, currentLocale, setLocale } from '@/i18n'
import type { Locale } from '@/i18n'
import { updateUserLang } from '@/api/sysManage'
import { getToken } from '@/utils/auth'

/**
 * 语言切换器：写入 localStorage 后整页刷新（见 @/i18n 的 setLocale）。
 * 侧边栏菜单、状态字典等模块级常量在刷新后才会用新语言重新计算。
 *
 * 已登录时同时把语言偏好存到账号上（`sys_user.lang`），换设备登录也保持一致；
 * 未登录、接口失败都不影响本次切换。
 */
export interface LanguageSwitcherProps {
  /** 文字颜色（登录页等深色背景上可传浅色） */
  color?: string
  /** 是否显示「简体中文 / English」文字，只留图标时传 false */
  showLabel?: boolean
}

const LanguageSwitcher: React.FC<LanguageSwitcherProps> = ({ color, showLabel = true }) => {
  const active = currentLocale()
  const current = LOCALES.find(item => item.value === active)

  const change = async (locale: Locale) => {
    if (locale !== active && getToken()) {
      // 最多等 800ms：接口慢或失败也不能耽误切换
      await Promise.race([
        updateUserLang(locale).catch(() => null),
        new Promise(resolve => setTimeout(resolve, 800)),
      ])
    }
    setLocale(locale)
  }

  return (
    <Dropdown
      placement="bottomRight"
      trigger={['click']}
      menu={{
        selectedKeys: [active],
        items: LOCALES.map(item => ({ key: item.value, label: item.label })),
        onClick: ({ key }) => { change(key as Locale) },
      }}
    >
      <Button type="text" size="small" style={color ? { color } : undefined}>
        <GlobalOutlined />
        {showLabel && <span style={{ marginLeft: 6 }}>{current?.label}</span>}
      </Button>
    </Dropdown>
  )
}

export default LanguageSwitcher
