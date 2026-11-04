import React from 'react'
import { Result } from 'antd'
import { useSearchParams } from 'react-router-dom'
import { t } from '@/i18n'

/**
 * 链接菜单的「嵌入」打开方式：在当前内容区用 iframe 展示目标页面。
 *
 * 只接受来自菜单 key 的 src/title；目标站点若通过 X-Frame-Options / CSP
 * 禁止被嵌入，浏览器会显示空白，这是目标站点侧的策略，页面无法绕过。
 */
const EmbedPage: React.FC = () => {
  const [params] = useSearchParams()
  const src = params.get('src') || ''
  const title = params.get('title') || t('嵌入页面')

  if (!src) {
    return <Result status="warning" title={t('缺少嵌入地址')} />
  }

  return (
    <div style={{ height: '100%' }}>
      <iframe
        title={title}
        src={src}
        style={{ width: '100%', height: '100%', border: 0, display: 'block' }}
        referrerPolicy="no-referrer"
      />
    </div>
  )
}

export default EmbedPage
