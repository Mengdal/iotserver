import React, { useCallback, useEffect, useImperativeHandle, useRef, useState, forwardRef } from 'react'
import { Button, Modal } from 'antd'
import { PauseCircleOutlined, PlayCircleOutlined } from '@ant-design/icons'
import { Terminal } from 'xterm'
import 'xterm/css/xterm.css'
import { t } from '@/i18n'

export interface TailLogResult {
  lines: string[]
  exists: boolean
  path: string
}

export interface TailLogTarget {
  title: string
  load: () => Promise<TailLogResult>
}

export interface TailLogModalRef {
  open: (target: TailLogTarget) => void
}

/**
 * 按 3 秒轮询日志文件的终端窗口，视觉和交互与协议驱动日志保持一致。
 * 用于 Mosquitto / eKuiper / iotpGW 等系统组件。
 */
export const TailLogModal = forwardRef<TailLogModalRef>((_, ref) => {
  const [isModalOpen, setIsModalOpen] = useState(false)
  const [title, setTitle] = useState('')
  const [following, setFollowing] = useState(false)
  const [lines, setLines] = useState<string[]>([])

  const targetRef = useRef<TailLogTarget | null>(null)
  const linesRef = useRef<string[]>([])
  const prevLinesRef = useRef<string[]>([])
  const timerRef = useRef<number | null>(null)
  const xtermRef = useRef<HTMLDivElement>(null)
  const termRef = useRef<Terminal | null>(null)

  const writeLines = useCallback((next: string[], reset = false) => {
    const term = termRef.current
    if (!term) return
    const prev = prevLinesRef.current
    let start = 0
    if (reset || prev.length === 0) {
      term.reset()
    } else if (next.length >= prev.length) {
      // 日志是追加的：上一次的整段内容应该仍是新内容的前缀。
      let samePrefix = true
      for (let i = 0; i < prev.length; i++) {
        if (next[i] !== prev[i]) {
          samePrefix = false
          break
        }
      }
      if (samePrefix) {
        start = prev.length
      } else {
        // 文件被轮转/清空，或两次轮询之间滚出了窗口，重画。
        term.reset()
      }
    } else {
      term.reset()
    }
    for (let i = start; i < next.length; i++) {
      term.writeln(next[i])
    }
    prevLinesRef.current = next
  }, [])

  const stopPolling = useCallback(() => {
    if (timerRef.current !== null) {
      window.clearInterval(timerRef.current)
      timerRef.current = null
    }
    setFollowing(false)
  }, [])

  const poll = useCallback(async () => {
    const target = targetRef.current
    if (!target) return
    try {
      const result = await target.load()
      const next = result.exists
        ? result.lines
        : [`[warn] ${t('日志文件不存在，请检查配置路径')}: ${result.path}`]
      linesRef.current = next
      setLines(next)
      writeLines(next)
    } catch (error) {
      const message = error instanceof Error ? error.message : String(error)
      const next = [`[error] ${message}`]
      linesRef.current = next
      setLines(next)
      writeLines(next)
    }
  }, [writeLines])

  const startPolling = useCallback(() => {
    stopPolling()
    setFollowing(true)
    poll()
    timerRef.current = window.setInterval(poll, 3000)
  }, [poll, stopPolling])

  useEffect(() => {
    if (!isModalOpen) {
      stopPolling()
      termRef.current?.dispose()
      termRef.current = null
      return
    }
    const timer = window.setTimeout(() => {
      if (!xtermRef.current) return
      const term = new Terminal({
        cols: 150,
        rows: 35,
        fontSize: 13,
      })
      term.open(xtermRef.current)
      termRef.current = term
      writeLines(linesRef.current, true)
    }, 0)
    return () => window.clearTimeout(timer)
  }, [isModalOpen, stopPolling, writeLines])

  useEffect(() => () => {
    if (timerRef.current !== null) window.clearInterval(timerRef.current)
    termRef.current?.dispose()
  }, [])

  useImperativeHandle(ref, () => {
    return {
      open (target: TailLogTarget) {
        targetRef.current = target
        linesRef.current = []
        prevLinesRef.current = []
        setLines([])
        setTitle(target.title)
        setIsModalOpen(true)
        // 等 Modal 和 xterm 挂载后再开始轮询。
        window.setTimeout(startPolling, 60)
      },
    }
  }, [startPolling])

  const download = () => {
    const content = lines.join('\n')
    const a = document.createElement('a')
    a.href = 'data:text/plain;charset=utf-8,' + encodeURIComponent(content)
    a.download = 'component-log.txt'
    document.body.appendChild(a)
    a.click()
    document.body.removeChild(a)
  }

  return (
    <Modal
      title={title || t('日志')}
      width={1250}
      maskClosable={false}
      open={isModalOpen}
      destroyOnClose
      footer={
        <>
          <Button type="default" onClick={download}>{t('下载日志')}</Button>
          <Button
            type="primary"
            icon={following ? <PauseCircleOutlined /> : <PlayCircleOutlined />}
            onClick={() => {
              if (following) {
                stopPolling()
              } else {
                startPolling()
              }
            }}
          >
            {following ? t('停止') : t('开始')}
          </Button>
        </>
      }
      onCancel={() => {
        stopPolling()
        setIsModalOpen(false)
        setLines([])
        linesRef.current = []
      }}
    >
      <div ref={xtermRef}></div>
    </Modal>
  )
})

export default TailLogModal
