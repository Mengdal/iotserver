import React, { useEffect, useState, useContext, useCallback } from 'react'
import { Card, Tag, Typography, Space, Divider, Button, message } from 'antd'
import { WebSocketProvider, WebSocketContext } from '@/components/WebSocket'
import { OperateStatusMap } from '@/utils/statusMap'
import { cloudService } from '@/api'

import './style.less'

const { useMessage } = message

export interface ServiceMarketProps {

}

export interface ServiceItemType {
  icon: string
  id: string
  is_internal: boolean
  manual: string
  name: string
  description: string
  operate_status: 'installing' | 'default' | 'installed' | 'uninstall'
  version: string
}

const ServiceMarket: React.FC<ServiceMarketProps> = () => {
  const [items, setItems] = useState<ServiceItemType[]>([])
  const [loading, setLoading] = useState(false)
  const [messageApi, contextHolder] = useMessage()

  const key = 'updatable'

  const { ws } = useContext(WebSocketContext)

  const updateItemStatus = useCallback((id: string, status: ServiceItemType['operate_status']) => {
    setItems([...items].map(item => {
      if (item.id === id) {
        item.operate_status = status
      }
      return item
    }))
  }, [items])

  useEffect(() => {
    setLoading(true)
    cloudService<ServiceItemType>()
      .then((resp) => {
        if (resp.success) {
          setItems(resp.result.list)
        }
      })
      .catch((err) => {
        console.log(err)
      })
      .finally(() => {
        setLoading(false)
      })
  }, [])

  return (
    <>
      {contextHolder}
      <div className="my-service p20">
        <Card loading={loading} bordered>
          <div className="responsive-layout">
            {
              items.map((item) => {
                const { color, icon, text } = OperateStatusMap[item.operate_status]
                return (
                  <Card key={item.id} className="card-item" size="small" hoverable>
                    <Tag color={color} icon={icon}>{text}</Tag>
                    <div className="flex-center">
                      <div className="logo" style={{ backgroundImage: `url(${item.icon})` }}></div>
                    </div>
                    <div className="flex justify-center title">
                      <Typography.Title level={5}>{item.name}</Typography.Title>
                    </div>
                    <div className="flex justify-center">
                      <Space split={<Divider type="vertical" />}>
                        <Button
                          type="link"
                          onClick={() => {
                            window.open(item.manual)
                          }}
                        >手册</Button>
                        {(
                          item.operate_status === 'default'
                          || item.operate_status === 'uninstall'
                          || item.operate_status === 'installing'
                        ) && (
                          <Button
                            type="link"
                            disabled={item.operate_status === 'installing'}
                            loading={item.operate_status === 'installing'}
                            onClick={async () => {
                              // loading 用 duration:0（不自动关），成功/失败时用同一个 key 替换掉它；
                              // 每条路径都必须 settle，否则会一直转
                              messageApi.open({ duration: 0, key, type: 'loading', content: '正在下载镜像，请稍候…' })
                              updateItemStatus(item.id, 'installing')

                              try {
                                // ws 未就绪时 send 会 reject，不能用 ws?.send(...).then：
                                // 那样 ws 为空时会静默什么都不做，看着像"点了没反应"
                                if (!ws) throw new Error('WebSocket 未连接，请刷新页面后重试')

                                const resp = await ws.send({
                                  code: 20001,
                                  data: {
                                    id: item.id,
                                    version: item.version,
                                  },
                                })
                                if (resp.data.success) {
                                  messageApi.open({ key, type: 'success', content: resp.data.successMsg || '下载成功' })
                                } else {
                                  messageApi.open({ key, type: 'error', content: resp.data.errorMsg || '下载失败' })
                                }
                                // 后端没回状态时退回 default，避免按钮卡在 loading
                                updateItemStatus(item.id, resp.data.result?.operate_status || 'default')
                              } catch (e: any) {
                                messageApi.open({ key, type: 'error', content: `下载失败：${e?.message || '未知错误'}` })
                                updateItemStatus(item.id, 'default')
                              }
                            }}
                          >下载</Button>
                        )}
                      </Space>

                    </div>
                  </Card>
                )
              })
            }
          </div>
        </Card>
      </div>
    </>
  )
}

export default () => (
  <WebSocketProvider>
    <ServiceMarket></ServiceMarket>
  </WebSocketProvider>
)
