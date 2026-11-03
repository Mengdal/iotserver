import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useParams, useSearchParams } from 'react-router-dom'
import { Button, Card, Empty, Modal, Space, Spin, Tabs, Typography, message } from 'antd'
import { DownloadOutlined, FullscreenOutlined, ReloadOutlined, UploadOutlined } from '@ant-design/icons'
import { AdvanceBreadcrumb } from '@/components/Breadcrumb'
import { Gateway, getGatewayInfo } from '@/api'
import { t } from '@/i18n'
import { ConfigProjectLink, configProjectUrl, gatewayWebBase } from './ConfigProjectLink'
import type { ConfigLinkStatus } from './ConfigProjectLink'
import { RunningTab } from './RunningTab'

const { Text } = Typography

const GatewayDetail: React.FC = () => {
  const { id = '' } = useParams<{ id: string }>()
  const [searchParams] = useSearchParams()
  const [activeKey, setActiveKey] = useState(searchParams.get('tab') || 'collect')
  const [gateway, setGateway] = useState<Gateway>()
  const [loading, setLoading] = useState(false)
  const isIoserverGateway = gateway?.source === 'ioserver'

  const [messageApi, contextHolder] = message.useMessage()
  const [configReloadKey, setConfigReloadKey] = useState(0)
  const [configBusy, setConfigBusy] = useState(false)
  const [configStatus, setConfigStatus] = useState<ConfigLinkStatus>('checking')
  const [fullscreenOpen, setFullscreenOpen] = useState(false)
  const fileRef = useRef<HTMLInputElement>(null)

  const load = useCallback(async () => {
    setLoading(true)
    try {
      const resp = await getGatewayInfo(id)
      if (resp.success) setGateway(resp.result)
    } finally {
      setLoading(false)
    }
  }, [id])

  useEffect(() => {
    load()
  }, [load])

  const configUrl = useMemo(() => (gateway ? configProjectUrl(gateway) : ''), [gateway])
  const webBase = useMemo(() => (gateway ? gatewayWebBase(gateway) : ''), [gateway])
  const canOperate = configStatus === 'editing'

  // 导入工程：POST /updateDB（multipart，字段 UploadFile）
  const handleImport = async (event: React.ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0]
    event.target.value = ''
    if (!file) return
    setConfigBusy(true)
    try {
      const form = new FormData()
      form.append('UploadFile', file)
      const resp = await fetch(`${webBase}/updateDB`, { method: 'POST', body: form })
      const json = await resp.json()
      if (String(json.code) === '200') {
        messageApi.success(t('导入成功'))
        setConfigReloadKey(key => key + 1)
      } else {
        messageApi.error(json.message || t('导入失败'))
      }
    } catch (error) {
      messageApi.error(t('连接网关失败，请确认浏览器能访问 {{url}}', { url: webBase }))
    } finally {
      setConfigBusy(false)
    }
  }

  // 导出工程：GET /getConfigDB
  const handleExport = async () => {
    setConfigBusy(true)
    try {
      const resp = await fetch(`${webBase}/getConfigDB`)
      if (!resp.ok) {
        messageApi.error(t('导出失败'))
        return
      }
      const blob = await resp.blob()
      const link = document.createElement('a')
      link.href = URL.createObjectURL(blob)
      link.download = `${gateway?.name || gateway?.sn || 'config'}.db`
      link.click()
      URL.revokeObjectURL(link.href)
    } catch (error) {
      messageApi.error(t('连接网关失败，请确认浏览器能访问 {{url}}', { url: webBase }))
    } finally {
      setConfigBusy(false)
    }
  }

  // 更新工程并重启：POST /copyDB（网关侧替换 lm.db 并重启采集）
  const handleUpdate = async () => {
    setConfigBusy(true)
    try {
      const resp = await fetch(`${webBase}/copyDB`, { method: 'POST' })
      const json = await resp.json()
      if (String(json.code) === '200') {
        messageApi.success(t('更新工程成功，网关正在重启'))
        // 更新后网关切回采集模式，配置工程不可用；回到运行状态页。
        setTimeout(() => setActiveKey('running'), 3000)
      } else {
        messageApi.error(json.message || t('更新工程失败'))
      }
    } catch (error) {
      messageApi.error(t('连接网关失败，请确认浏览器能访问 {{url}}', { url: webBase }))
    } finally {
      setConfigBusy(false)
    }
  }

  const configActions = (
    <Space wrap>
      <input
        ref={fileRef}
        type="file"
        accept=".db"
        style={{ display: 'none' }}
        onChange={handleImport}
      />
      <Button size="small" icon={<UploadOutlined />} disabled={!canOperate} loading={configBusy} onClick={() => fileRef.current?.click()}>
        {t('导入')}
      </Button>
      <Button size="small" icon={<DownloadOutlined />} disabled={!canOperate} loading={configBusy} onClick={handleExport}>
        {t('导出')}
      </Button>
      <Button size="small" icon={<ReloadOutlined />} disabled={!canOperate} loading={configBusy} onClick={handleUpdate}>
        {t('更新工程并重启')}
      </Button>
      <Button size="small" icon={<FullscreenOutlined />} disabled={!canOperate} onClick={() => setFullscreenOpen(true)}>
        {t('全屏')}
      </Button>
    </Space>
  )

  return (
    <>
      {contextHolder}
      <AdvanceBreadcrumb
        hasBack
        title={<Text style={{ fontSize: 17 }}>{gateway?.name || t('网关详情')}</Text>}
        describe={t('网关详情提供采集配置、接入信息与运行状态；采集配置按 ioserver 的通道 → 设备 → 点位组织。')}
        breadcrumb={[
          { label: t('网关管理'), to: '/gateway/gateway/manage' },
          { label: t('网关详情 [ {{p1}} ]', { p1: gateway?.name || '' }), to: '' },
        ]}
      />

      <Card>
        <Tabs
          activeKey={activeKey}
          onChange={setActiveKey}
          tabBarExtraContent={isIoserverGateway && activeKey === 'collect' ? configActions : null}
          items={[
            {
              key: 'collect',
              label: t('配置工程'),
              children: gateway
                ? (isIoserverGateway
                  ? <ConfigProjectLink gateway={gateway} reloadKey={configReloadKey} onStatusChange={setConfigStatus} />
                  : <Empty description={t('仅 ioserver 网关支持配置工程')} />)
                : <Spin spinning={loading}><div style={{ height: 240 }} /></Spin>,
            },
            {
              key: 'running',
              label: t('运行状态'),
              children: gateway
                ? (isIoserverGateway
                  ? <RunningTab gateway={gateway} />
                  : <Empty description={t('仅 ioserver 网关支持运行状态')} />)
                : <Spin spinning={loading}><div style={{ height: 240 }} /></Spin>,
            },
          ]}
        />
      </Card>

      <Modal
        open={fullscreenOpen}
        title={t('配置工程')}
        footer={null}
        width="100vw"
        style={{ top: 0, maxWidth: '100vw', paddingBottom: 0 }}
        bodyStyle={{ height: 'calc(100vh - 55px)', padding: 0 }}
        onCancel={() => setFullscreenOpen(false)}
        destroyOnClose
      >
        <iframe
          title="gateway-config-project-fullscreen"
          src={configUrl}
          style={{ width: '100%', height: '100%', border: 0 }}
        />
      </Modal>
    </>
  )
}

export default GatewayDetail
