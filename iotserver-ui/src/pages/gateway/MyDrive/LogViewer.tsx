import React, { useCallback, useLayoutEffect, useRef } from 'react'
import './LogViewer.less'

// 去掉 ANSI 颜色/光标控制码（协议驱动日志会带），只保留可读文本。
const ANSI_ESCAPE = /\x1b\[[0-9;?]*[ -/]*[@-~]/g

interface LogViewerProps {
  /** 原始日志文本；由调用方决定按行拼还是直接拼接流式片段。 */
  text: string
}

/**
 * 纯文本日志窗口：滚动区域就是内容本身，滚到底时最后一行贴着底部，
 * 不会出现 xterm 那种固定行数留下的空白行。
 * 用户往上翻看历史时暂停自动跟随，滚回底部后恢复。
 */
const LogViewer: React.FC<LogViewerProps> = ({ text }) => {
  const preRef = useRef<HTMLPreElement>(null)
  const followRef = useRef(true)

  const handleScroll = useCallback(() => {
    const el = preRef.current
    if (!el) return
    followRef.current = el.scrollTop + el.clientHeight >= el.scrollHeight - 4
  }, [])

  const rendered = text.replace(ANSI_ESCAPE, '')

  useLayoutEffect(() => {
    const el = preRef.current
    if (el && followRef.current) {
      el.scrollTop = el.scrollHeight
    }
  }, [rendered])

  return (
    <pre ref={preRef} className="log-viewer" onScroll={handleScroll}>{rendered}</pre>
  )
}

export default LogViewer
