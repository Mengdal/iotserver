import React, { useEffect, useState, useContext, useCallback } from 'react'
import { Card, Tag, Typography, Space, Divider, Button, message, Tabs, Form, Input, Empty } from 'antd'
import { WebSocketProvider, WebSocketContext } from '@/components/WebSocket'
import { OperateStatusMap } from '@/utils/statusMap'
import { getDriverMarket } from '@/api'

import './style.less'
import { getDriveClassify } from '@/api/device-libraries'
import { t } from '@/i18n'

const { useMessage } = message

export interface DriveMarketProps {}

export interface DriverMarketType {
  icon: string;
  id: string;
  is_internal: boolean;
  manual: string;
  name: string;
  description: string;
  operate_status: 'installing' | 'default' | 'installed' | 'uninstall';
  version: string;
  classify_id: string;
}

const CardItem = ({ id, params }: { id: string; params: { name?: string } }) => {
  const [items, setItems] = useState<DriverMarketType[]>([])
  const [loading, setLoading] = useState(false)
  const [messageApi] = useMessage()

  const key = 'updatable'

  const { ws } = useContext(WebSocketContext)

  const updateItemStatus = useCallback(
    (id: string, status: DriverMarketType['operate_status']) => {
      setItems(
        [...items].map((item) => {
          if (item.id === id) {
            item.operate_status = status
          }
          return item
        }),
      )
    },
    [items],
  )

  const getList = (name?: string) => {
    setLoading(true)
    getDriverMarket<DriverMarketType>({ name, classify_id: id })
      .then((resp) => {
        if (resp.success) {
          setItems(resp.result.list)
        }
      })
      .finally(() => {
        setLoading(false)
      })
  }

  useEffect(() => {
    getList(params.name)
  }, [params.name])

  return (
    <div className="drive-market p20">
      <Card loading={loading} bordered>
        <div className="responsive-layout">
          {items.map((item) => {
            const { color, icon, text } = OperateStatusMap[item.operate_status]
            return (
              <Card key={item.id} className="card-item" size="small" hoverable>
                <Tag color={color} icon={icon}>
                  {text}
                </Tag>
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
                    >
                      {t('手册')}
                    </Button>
                    {(item.operate_status === 'default' || item.operate_status === 'uninstall' || item.operate_status === 'installing') && (
                      <Button
                        type="link"
                        loading={item.operate_status === 'installing'}
                        onClick={async () => {
                          // loading 用 duration:0（不自动关），成功/失败时用同一个 key 替换掉它；
                          // 所以下面每条路径都必须 settle，否则会一直转
                          messageApi.open({ duration: 0, key, type: 'loading', content: t('正在下载镜像，请稍候…') })
                          updateItemStatus(item.id, 'installing')

                          try {
                            // ws 未就绪时 send 会 reject（见 utils/web-socket.ts），
                            // 这里不能用 ws?.send(...).then，那样 ws 为空会静默什么都不做
                            if (!ws) throw new Error(t('WebSocket 未连接，请刷新页面后重试'))

                            const resp = await ws.send({
                              code: 10001,
                              data: { id: item.id, version: item.version },
                            })

                            if (resp.data.success) {
                              messageApi.open({ key, type: 'success', content: resp.data.successMsg || t('下载成功') })
                            } else {
                              messageApi.open({ key, type: 'error', content: resp.data.errorMsg || t('下载失败') })
                            }
                            // 后端没回状态时退回 default，避免按钮卡在 loading
                            updateItemStatus(item.id, resp.data.result?.operate_status || 'default')
                          } catch (e: any) {
                            // 把真实原因显示出来（原来是静默失败，只能翻服务端日志）
                            messageApi.open({ key, type: 'error', content: t('下载失败：{{p1}}', { p1: e?.message || t('未知错误') }) })
                            updateItemStatus(item.id, 'default')
                          }
                        }}
                      >
                        {t('下载')}
                      </Button>
                    )}
                  </Space>
                </div>
              </Card>
            )
          })}
        </div>

        {!items?.length && <Empty description={t('暂无数据')}></Empty>}
      </Card>
    </div>
  )
}

const DriveMarket: React.FC<DriveMarketProps> = () => {
  const [, contextHolder] = useMessage()

  const [tabList, setTabList] = useState<{ id: string; name: string }[]>([])

  const [activeKey, setActiveKey] = useState<string | undefined>('')

  const getDriveList = async () => {
    try {
      const { result } = await getDriveClassify()
      const list = (result?.list || []) as { id: string; name: string }[]
      setTabList(list)
      setActiveKey(list[0]?.id)
    } catch {}
  }

  useEffect(() => {
    getDriveList()
  }, [])
  const [form] = Form.useForm()

  const [params, setParams] = useState<{ name?: string }>({})

  const search = () => {
    const values = form.getFieldsValue()
    setParams({ ...values })
  }

  const reset = () => {
    form.resetFields()
    search()
  }

  return (
    <>
      {contextHolder}
      <Form layout="inline" className="ml20" form={form}>
        <Form.Item key="name" name="name">
          <Input placeholder={t('名称')}></Input>
        </Form.Item>
        <Form.Item>
          <Button type="primary" onClick={search}>
            {t('查询')}
          </Button>
        </Form.Item>
        <Form.Item>
          <Button type="default" onClick={reset}>
            {t('重置')}
          </Button>
        </Form.Item>
      </Form>
      <Tabs
        activeKey={activeKey}
        onChange={(e) => {
          setActiveKey(e)
        }}
        destroyInactiveTabPane={true}
        items={tabList?.map((item) => ({ key: item.id, label: item.name, children: <CardItem id={item.id} params={params}></CardItem> })) || []}
      ></Tabs>
    </>
  )
}

export default () => (
  <WebSocketProvider>
    <DriveMarket></DriveMarket>
  </WebSocketProvider>
)
