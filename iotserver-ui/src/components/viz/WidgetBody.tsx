import React from 'react'
import { Alert } from 'antd'
import type { VizWidget } from '@/api'
import { getWidgetManifest } from './registry'
import type { VizWidgetContext } from './types'

/**
 * 按注册表渲染一个组件的**内容**（不含卡片外框，外框由 WidgetCard 负责）。
 *
 * 渲染前把 manifest 的默认配置与存下来的 config 合并一次：
 * 后端保存时已经补过默认值，这里再兜一层，保证前端单独加了配置项时旧看板也能正常渲染。
 */
const WidgetBody: React.FC<{ widget: VizWidget; ctx: VizWidgetContext }> = ({ widget, ctx }) => {
  const manifest = getWidgetManifest(widget.type)
  if (!manifest) {
    return (
      <Alert
        type="warning"
        showIcon
        message={`当前版本不支持组件类型「${widget.type || '未知'}」`}
        description="看板数据仍然保留，升级到最新版本即可正常显示。"
      />
    )
  }
  const config = { ...manifest.defaultConfig(), ...(widget.config || {}) }
  const Renderer = manifest.render
  return <Renderer id={widget.id} config={config} ctx={ctx} />
}

export default WidgetBody
