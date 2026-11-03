import React, { useCallback, useEffect, useMemo, useState } from 'react'
import { Button, Spin } from 'antd'
import { Gateway } from '@/api'
import { currentLocale, t } from '@/i18n'

/**
 * 配置工程地址：直接复用网关自带的 GCPage。
 * 参数与 edgeWeb 打开配置工程时保持一致（gatewayid=52&userid=1）。
 */
export function configProjectUrl (gateway: Gateway): string {
  const host = gateway.ip || window.location.hostname
  const language = currentLocale() === 'en-US' ? 'en-US' : 'zh-CN'
  const name = encodeURIComponent(gateway.name || gateway.sn || '')
  return `http://${host}:8091/GCPage/things.html?gatewayid=52&gatewayName=${name}&itemid=39&userid=1&language=${language}`
}

export function gatewayWebBase (gateway: Gateway): string {
  const host = gateway.ip || window.location.hostname
  return `http://${host}:8080`
}

export type ConfigLinkStatus = 'checking' | 'editing' | 'switching' | 'error'

interface ConfigProjectLinkProps {
  gateway: Gateway
  reloadKey?: number
  onStatusChange?: (status: ConfigLinkStatus) => void
}

export const ConfigProjectLink: React.FC<ConfigProjectLinkProps> = ({
  gateway,
  reloadKey = 0,
  onStatusChange,
}) => {
  const [status, setStatus] = useState<ConfigLinkStatus>('checking')
  const [retryKey, setRetryKey] = useState(0)
  const url = useMemo(() => configProjectUrl(gateway), [gateway])
  const webBase = useMemo(() => gatewayWebBase(gateway), [gateway])

  const updateStatus = useCallback((next: ConfigLinkStatus) => {
    setStatus(next)
    onStatusChange?.(next)
  }, [onStatusChange])

  // 8091 可达 = 网关处于配置模式（gchttp 在跑）。
  const probeOnce = useCallback(async () => {
    try {
      await fetch(url, { mode: 'no-cors', cache: 'no-store' })
      return true
    } catch (error) {
      return false
    }
  }, [url])

  useEffect(() => {
    let cancelled = false

    const run = async () => {
      // 配置工程只适用于 ioserver 软网关；其他来源绝不能触发 /stopGW。
      if (gateway.source !== 'ioserver') return
      updateStatus('checking')
      if (await probeOnce()) {
        if (!cancelled) updateStatus('editing')
        return
      }

      // 采集模式下 8091 是关的：自动停止采集、启动配置服务，再等它起来。
      updateStatus('switching')
      try {
        await fetch(`${webBase}/stopGW`, { method: 'POST' })
      } catch (error) {
        // 继续轮询，可能网关本来就在配置模式
      }
      for (let i = 0; i < 30; i += 1) {
        await new Promise(resolve => setTimeout(resolve, 1000))
        if (cancelled) return
        if (await probeOnce()) {
          updateStatus('editing')
          return
        }
      }
      if (!cancelled) updateStatus('error')
    }

    run()
    return () => {
      cancelled = true
    }
  }, [probeOnce, webBase, updateStatus, reloadKey, retryKey])

  if (status === 'editing') {
    return (
      <iframe
        key={reloadKey}
        title="gateway-config-project"
        src={url}
        style={{
          width: '100%',
          height: 'calc(100vh - 280px)',
          minHeight: 560,
          border: '1px solid #f0f0f0',
          borderRadius: 6,
          background: '#fff',
        }}
      />
    )
  }

  return (
    <div style={{ minHeight: 320, display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', gap: 12 }}>
      <Spin size="large" tip={status === 'switching' ? t('正在切换到配置模式...') : t('正在加载配置工程...')}>
        <div style={{ width: 240, height: 120 }} />
      </Spin>
      {status === 'error' && (
        <Button size="small" onClick={() => setRetryKey(key => key + 1)}>
          {t('重试')}
        </Button>
      )}
    </div>
  )
}
