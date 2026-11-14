import React, { useEffect, useRef, useState, useContext, useCallback } from 'react'
import { Card, Tag, Typography, Space, Divider, Button, message, Form, Input, Empty, Modal, Popconfirm, Upload } from 'antd'
import { CheckCircleOutlined, ClockCircleOutlined, CloudServerOutlined, UploadOutlined } from '@ant-design/icons'
import { WebSocketProvider, WebSocketContext } from '@/components/WebSocket'
import { OperateStatusMap } from '@/utils/statusMap'
import { deleteSODriver, getDriverMarket, uploadPluginPackage, uploadSODriver } from '@/api'

import './style.less'
import { isEmbeddedDeployment, useAuth } from '@/utils/auth'
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
  category?: string;
  essential?: boolean;
  runtime?: string;
  arch?: string;
  protocol?: string;
}

const CardItem = ({
  params,
  reloadKey,
}: {
  params: { name?: string };
  reloadKey: number;
}) => {
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
    getDriverMarket<DriverMarketType>({ name, category: 'all' })
      .then((resp) => {
        if (resp.success) {
          // 驱动市场按部署形态隔离：embedded 不显示 Docker，server 不显示 iotpGW .so。
          const embedded = isEmbeddedDeployment()
          const list = (resp.result.list || []).filter(item =>
            embedded ? item.runtime !== 'docker' : item.runtime !== 'iotpgw-so',
          )
          // 协议驱动/原生插件在前，内置组件排在后面。
          list.sort((a, b) => Number(a.category === 'system') - Number(b.category === 'system'))
          setItems(list)
        }
      })
      .finally(() => {
        setLoading(false)
      })
  }

  useEffect(() => {
    getList(params.name)
  }, [params.name, reloadKey])

  return (
    <div className="drive-market p20">
      <Card loading={loading} bordered>
        <div className="responsive-layout">
          {items.map((item) => {
            const isSystem = item.category === 'system'
            const isNative = item.runtime === 'native'
            const isSO = item.runtime === 'iotpgw-so'
            // 系统组件/驱动组件/原生 .so 驱动不使用“下载镜像”的状态文案。
            const statusMeta = isSO
              ? (item.operate_status === 'installed'
                ? { color: 'success', icon: <CheckCircleOutlined />, text: t('已安装') }
                : { color: 'default', icon: <ClockCircleOutlined />, text: t('未安装') })
              : (isSystem || isNative ? OperateStatusMap.installed : OperateStatusMap[item.operate_status])
            const { color, icon, text } = statusMeta
            const handleDeleteSO = async (id: string) => {
              const resp = await deleteSODriver(id)
              if (resp.success) {
                messageApi.success(t('卸载成功'))
                getList(params.name)
              } else {
                messageApi.error(resp.errorMsg)
              }
            }
            const renderFooter = () => {
              if (isSO) {
                return (
                  <div className="flex justify-center">
                    <Space split={<Divider type="vertical" />}>
                      {item.manual && (
                        <Button type="link" onClick={() => window.open(item.manual)}>
                          {t('手册')}
                        </Button>
                      )}
                      {item.operate_status === 'installed' && (
                        <Popconfirm
                          title={t('确认卸载该驱动？')}
                          okText={t('确定')}
                          cancelText={t('取消')}
                          onConfirm={() => handleDeleteSO(item.id)}
                        >
                          <Button type="link" danger>{t('卸载')}</Button>
                        </Popconfirm>
                      )}
                    </Space>
                  </div>
                )
              }
              if (isSystem || isNative) {
                return item.manual
                  ? (
                    <div className="flex justify-center">
                      <Button type="link" onClick={() => window.open(item.manual)}>
                        {t('手册')}
                      </Button>
                    </div>
                  )
                  : <div className="flex justify-center" />
              }
              return (
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
              )
            }
            return (
              <Card key={item.id} className="card-item" size="small" hoverable>
                <Space size={4} wrap>
                  <Tag color={color} icon={icon}>
                    {text}
                  </Tag>
                  <Tag color="default">
                    {isSystem ? t('系统组件') : isNative ? t('驱动组件') : isSO ? t('驱动库') : t('协议驱动')}
                  </Tag>
                  {isSystem && item.essential && <Tag color="blue">{t('必需')}</Tag>}
                </Space>
                <div className="flex-center">
                  {item.icon
                    ? <div className="logo" style={{ backgroundImage: `url(${item.icon})` }}></div>
                    : (
                      <div
                        className="logo"
                        style={{
                          background: '#f0f5ff',
                          display: 'flex',
                          alignItems: 'center',
                          justifyContent: 'center',
                        }}
                      >
                        <CloudServerOutlined style={{ fontSize: 28, color: '#1677ff' }} />
                      </div>
                    )}
                </div>
                <div className="flex justify-center title">
                  <Typography.Title level={5}>{item.name}</Typography.Title>
                </div>
                {renderFooter()}
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
  const [messageApi, contextHolder] = useMessage()
  const uploadRef = useRef<HTMLInputElement>(null)
  const [uploading, setUploading] = useState(false)
  const [reloadKey, setReloadKey] = useState(0)
  const canUpload = useAuth('gateway:drive:add')
  const embedded = isEmbeddedDeployment()

  const [soOpen, setSoOpen] = useState(false)
  const [soUploading, setSoUploading] = useState(false)
  const [soFile, setSoFile] = useState<File | null>(null)
  const [soForm] = Form.useForm()

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

  const handleUploadPackage = async (event: React.ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0]
    event.target.value = ''
    if (!file) return
    setUploading(true)
    try {
      const resp = await uploadPluginPackage(file)
      if (resp.success) {
        messageApi.success(t('插件包安装成功'))
        setReloadKey(key => key + 1)
      } else {
        messageApi.error(resp.errorMsg)
      }
    } catch (error: any) {
      messageApi.error(error?.message || t('插件包安装失败'))
    } finally {
      setUploading(false)
    }
  }

  const openSO = () => {
    soForm.resetFields()
    soForm.setFieldsValue({ version: '1.0.0' })
    setSoFile(null)
    setSoOpen(true)
  }

  const handleUploadSO = async () => {
    const values = await soForm.validateFields()
    if (!soFile) {
      messageApi.error(t('请选择 .so 驱动文件'))
      return
    }
    setSoUploading(true)
    try {
      const resp = await uploadSODriver(soFile, values)
      if (resp.success) {
        messageApi.success(t('驱动安装成功'))
        setSoOpen(false)
        setReloadKey(key => key + 1)
      } else {
        messageApi.error(resp.errorMsg)
      }
    } catch (error: any) {
      messageApi.error(error?.message || t('驱动安装失败'))
    } finally {
      setSoUploading(false)
    }
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
        {canUpload && (
          <Form.Item>
            <Button icon={<UploadOutlined />} loading={uploading} onClick={() => uploadRef.current?.click()}>
              {t('安装插件包')}
            </Button>
            <input
              ref={uploadRef}
              type="file"
              accept=".tar.gz,.tgz,application/gzip"
              style={{ display: 'none' }}
              onChange={handleUploadPackage}
            />
          </Form.Item>
        )}
        {canUpload && embedded && (
          <Form.Item>
            <Button icon={<UploadOutlined />} onClick={openSO}>
              {t('安装 .so 驱动')}
            </Button>
          </Form.Item>
        )}
      </Form>
      <CardItem params={params} reloadKey={reloadKey} />

      <Modal
        open={soOpen}
        title={t('安装 .so 驱动')}
        destroyOnClose
        okText={t('确定')}
        cancelText={t('取消')}
        confirmLoading={soUploading}
        onOk={handleUploadSO}
        onCancel={() => setSoOpen(false)}
      >
        <Form form={soForm} layout="vertical" initialValues={{ version: '1.0.0' }}>
          <Form.Item label={t('驱动名称')} name="name" extra={t('留空时使用文件名')}>
            <Input placeholder={t('驱动名称')} />
          </Form.Item>
          <Form.Item label={t('版本')} name="version">
            <Input placeholder="1.0.0" />
          </Form.Item>
          <Form.Item label={t('协议')} name="protocol">
            <Input placeholder={t('如 Modbus TCP')} />
          </Form.Item>
          <Form.Item label={t('架构')} name="arch" extra={t('留空按当前平台架构')}>
            <Input placeholder="arm64" />
          </Form.Item>
          <Form.Item label="ABI" name="abi">
            <Input placeholder={t('留空不校验')} />
          </Form.Item>
          <Form.Item label={t('说明')} name="description">
            <Input.TextArea rows={2} />
          </Form.Item>
          <Form.Item label={t('驱动文件')} required>
            <Upload
              accept=".so"
              maxCount={1}
              beforeUpload={(file) => {
                setSoFile(file as File)
                return false
              }}
              onRemove={() => setSoFile(null)}
            >
              <Button icon={<UploadOutlined />}>{t('选择 .so 文件')}</Button>
            </Upload>
          </Form.Item>
        </Form>
      </Modal>
    </>
  )
}

export default () => (
  <WebSocketProvider>
    <DriveMarket></DriveMarket>
  </WebSocketProvider>
)
