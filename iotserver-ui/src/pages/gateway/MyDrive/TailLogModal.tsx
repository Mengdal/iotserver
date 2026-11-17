import React, { useCallback, useEffect, useImperativeHandle, useRef, useState, forwardRef } from 'react'
import { Button, Modal, Space } from 'antd'
import {
  DownloadOutlined,
  PauseCircleOutlined,
  PlayCircleOutlined,
  ReloadOutlined,
} from '@ant-design/icons'
import { t } from '@/i18n'
import LogViewer from './LogViewer'
import './TailLogModal.less'

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
 * 按 3 秒轮询日志文件的日志窗口，用于 Mosquitto / eKuiper / iotpGW 等系统组件。
 */
export const TailLogModal = forwardRef<TailLogModalRef>((_, ref) => {
  const [isModalOpen, setIsModalOpen] = useState(false)
  const [title, setTitle] = useState('')
  const [following, setFollowing] = useState(false)
  const [lines, setLines] = useState<string[]>([])

  const targetRef = useRef<TailLogTarget | null>(null)
  const linesRef = useRef<string[]>([])
  const timerRef = useRef<number | null>(null)
  // open() 延迟 60ms 再启动轮询；这个定时器也要能取消，否则关闭快时会在弹窗消失后继续轮询
  const startTimerRef = useRef<number | null>(null)
  const pollingRef = useRef(false)

  const stopPolling = useCallback(() => {
    if (timerRef.current !== null) {
      window.clearInterval(timerRef.current)
      timerRef.current = null
    }
    setFollowing(false)
  }, [])

  const poll = useCallback(async () => {
    const target = targetRef.current
    // 上一次请求还没回来时跳过本轮，避免慢请求返回后覆盖新内容。
    if (!target || pollingRef.current) return
    pollingRef.current = true
    try {
      const result = await target.load()
      // 弹窗切换到另一个组件后，旧请求的结果不能再写进新窗口。
      if (targetRef.current !== target) return
      const next = result.exists
        ? result.lines
        : [`[warn] ${t('日志文件不存在，请检查配置路径')}: ${result.path}`]
      linesRef.current = next
      setLines(next)
    } catch (error) {
      if (targetRef.current !== target) return
      const message = error instanceof Error ? error.message : String(error)
      const next = [`[error] ${message}`]
      linesRef.current = next
      setLines(next)
    } finally {
      pollingRef.current = false
    }
  }, [])

  const clearStartTimer = useCallback(() => {
    if (startTimerRef.current !== null) {
      window.clearTimeout(startTimerRef.current)
      startTimerRef.current = null
    }
  }, [])

  const startPolling = useCallback(() => {
    stopPolling()
    if (!targetRef.current) return
    setFollowing(true)
    poll()
    timerRef.current = window.setInterval(poll, 3000)
  }, [poll, stopPolling])

  useEffect(() => () => {
    if (timerRef.current !== null) window.clearInterval(timerRef.current)
    if (startTimerRef.current !== null) window.clearTimeout(startTimerRef.current)
  }, [])

  useImperativeHandle(ref, () => {
    return {
      open (target: TailLogTarget) {
        clearStartTimer()
        targetRef.current = target
        linesRef.current = []
        setLines([])
        setTitle(target.title)
        setIsModalOpen(true)
        // 等 Modal 挂载后再开始轮询。
        startTimerRef.current = window.setTimeout(() => {
          startTimerRef.current = null
          startPolling()
        }, 60)
      },
    }
  }, [clearStartTimer, startPolling])

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
      className="tail-log-modal"
      width={1120}
      centered
      open={isModalOpen}
      destroyOnClose
      title={title || t('日志')}
      footer={
        <div className="tail-log-footer">
          <Space>
            <Button icon={<ReloadOutlined />} onClick={() => poll()}>{t('刷新')}</Button>
            <Button icon={<DownloadOutlined />} onClick={download}>{t('下载日志')}</Button>
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
          </Space>
        </div>
      }
      onCancel={() => {
        clearStartTimer()
        targetRef.current = null
        stopPolling()
        setIsModalOpen(false)
        setLines([])
        linesRef.current = []
      }}
    >
      <div className="tail-log-body">
        <LogViewer text={lines.join('\n')} />
      </div>
    </Modal>
  )
})

export default TailLogModal
