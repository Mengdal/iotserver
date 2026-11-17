import React, { useCallback, useContext, useImperativeHandle, useMemo, useState, forwardRef } from 'react'
import { Button, Modal, Space } from 'antd'
import { DownloadOutlined, PauseCircleOutlined, PlayCircleOutlined } from '@ant-design/icons'
import { WebSocketContext } from '@/components/WebSocket'
import { t } from '@/i18n'
import LogViewer from './LogViewer'
import './TailLogModal.less'

export interface LogModalProps {
  // ws?: WebSocketClass
}

export interface LogModalRef {
  open: (id: string, name?: string) => void
}

/** 最多保留多少段日志：WebSocket 长连接可能持续数小时，不设上限会占满内存 */
const MAX_LOG_CHUNKS = 2000

/**
 * 协议驱动（Docker 实例）日志：通过 WebSocket 实时推送，窗口外观与系统组件日志一致。
 */
export const LogModal = forwardRef<LogModalRef, LogModalProps>((props, ref) => {
  const [id, setId] = useState('')
  const [title, setTitle] = useState('')
  const [data, setData] = useState<string[]>([])
  const [loading, setLoading] = useState(false)
  const [isModalOpen, setIsModalOpen] = useState(false)

  const { ws } = useContext(WebSocketContext)

  const handleStart = useCallback((id: string) => {
    ws?.sendSync({
      code: 10004,
      data: { id, operate: 1 },
      success (resp) {
        if (!resp.data.success) return
        setLoading(true)
        const chunk = resp.data.result
        // 停止订阅的 ack 也会走这个常驻回调，result 为空，不能当成一行日志
        if (typeof chunk !== 'string' || chunk === '') return
        setData((prevValues) => {
          const next = [...prevValues, chunk]
          return next.length > MAX_LOG_CHUNKS ? next.slice(next.length - MAX_LOG_CHUNKS) : next
        })
      },
    })
  }, [ws])

  // WebSocket 每次推送的是一段日志，通常不带结尾换行；直接拼会把两行粘在一起。
  const text = useMemo(() => {
    let out = ''
    for (const chunk of data) {
      if (out && !out.endsWith('\n')) out += '\n'
      out += chunk
    }
    return out
  }, [data])

  const handleStop = useCallback(() => {
    ws?.send({
      code: 10004,
      data: { id, operate: 2 },
    }).then((resp) => {
      if (resp.data.success) {
        setLoading(false)
      }
    })
  }, [id, ws])

  useImperativeHandle(ref, () => {
    return {
      open (id: string, name?: string) {
        // 先清掉上一个订阅留下的内容，避免新窗口开头混入旧驱动的日志
        setData([])
        setLoading(false)
        setId(id)
        setTitle(name ? `${t('日志')} · ${name}` : t('云服务日志'))
        setIsModalOpen(true)
        handleStart(id)
      },
    }
  }, [handleStart])

  const download = () => {
    const a = document.createElement('a')
    a.href = 'data:text/plain;charset=utf-8,' + encodeURIComponent(text)
    a.download = 'driver-log.txt'
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
      title={title || t('云服务日志')}
      footer={
        <div className="tail-log-footer">
          <Space>
            <Button icon={<DownloadOutlined />} onClick={download}>{t('下载日志')}</Button>
            <Button
              type="primary"
              icon={loading ? <PauseCircleOutlined /> : <PlayCircleOutlined />}
              onClick={() => {
                if (loading) {
                  handleStop()
                } else {
                  handleStart(id)
                }
              }}
            >
              {loading ? t('停止') : t('开始')}
            </Button>
          </Space>
        </div>
      }
      onCancel={() => {
        handleStop()
        setData(() => [])
        setIsModalOpen(false)
      }}
    >
      <div className="tail-log-body">
        <LogViewer text={text} />
      </div>
    </Modal>
  )
})

export default LogModal
