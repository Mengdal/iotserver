import React, { useEffect, useRef, useState } from 'react'
import { Input, Typography } from 'antd'
import { FileTextOutlined } from '@ant-design/icons'
import type { VizWidgetManifest } from '../types'
import { t, tt } from '@/i18n'

const { Paragraph } = Typography

/**
 * 文本组件：看板上的说明文字。
 *
 * 编辑态是自适应高度的 textarea，失焦/切只读/卸载时提交（本地草稿 + 防抖提交，
 * 避免每敲一个字就把整块看板写脏、触发一次全量重渲染）。
 */
const TextWidget: React.FC<{ ctx: any }> = ({ ctx }) => {
  const text = ctx.text || ''
  const [draft, setDraft] = useState(text)
  const draftRef = useRef(text)
  const committedRef = useRef(text)
  const changeRef = useRef(ctx.onTextChange)

  useEffect(() => { draftRef.current = draft }, [draft])
  useEffect(() => { changeRef.current = ctx.onTextChange }, [ctx.onTextChange])
  useEffect(() => {
    committedRef.current = text
    setDraft(text)
  }, [text])

  const flush = () => {
    if (changeRef.current && draftRef.current !== committedRef.current) {
      changeRef.current(draftRef.current)
    }
  }
  useEffect(() => {
    if (ctx.readOnly) flush()
  }, [ctx.readOnly])
  useEffect(() => () => flush(), [])

  if (ctx.readOnly) {
    return (
      // 内置看板里的说明文字有对应英文词条时跟着语言变；用户自己写的正文原样显示
      <Paragraph style={{ whiteSpace: 'pre-wrap', marginBottom: 0 }}>
        {tt(text) || t('（空文本）')}
      </Paragraph>
    )
  }
  return (
    <Input.TextArea
      value={draft}
      placeholder={t('写点说明，例如：数据来源、口径、更新频率…')}
      autoSize={{ minRows: 3, maxRows: 14 }}
      onChange={e => setDraft(e.target.value)}
      onBlur={flush}
      style={{ border: 'none', padding: 0, boxShadow: 'none', resize: 'none' }}
    />
  )
}

export const textManifest: VizWidgetManifest = {
  type: 'text',
  label: t('文本'),
  icon: <FileTextOutlined />,
  defaultSize: { w: 6, h: 4 },
  defaultConfig: () => ({}),
  render: ({ ctx }) => <TextWidget ctx={ctx} />,
}
