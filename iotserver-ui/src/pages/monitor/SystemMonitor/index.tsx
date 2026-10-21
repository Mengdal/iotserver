import React, { useEffect, useState } from 'react'
import { Empty, Spin } from 'antd'
import CanvasBoard from '@/components/CanvasBoard'
import { getVizBoards, VizBoard } from '@/api'
import { t } from '@/i18n'

/**
 * 系统监控：与首页一致，整页直接渲染「系统监控看板」。
 *
 * 这块看板只属于平台作用域；菜单本身由后端 platform_only 过滤，
 * 平台管理员代管租户时侧边栏也会隐藏（见 Layout），这里不再做额外提示。
 */
const SystemMonitor: React.FC = () => {
  const [board, setBoard] = useState<VizBoard | undefined>()
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  useEffect(() => {
    getVizBoards()
      .then((resp) => {
        if (resp.success) {
          // 按 id 找内置系统看板（名称已统一英文，不能只靠中文名匹配）
          const isSystemBoard = (b: VizBoard) =>
            b.id === 'builtin-ops'
            || b.id.startsWith('builtin-ops-')
            || b.id.startsWith('bd-tenant-monitor')
            || b.name === 'System Monitoring'
            || b.name === '系统监控看板'
          setBoard((resp.result.list || []).find(isSystemBoard))
        } else {
          setError(resp.errorMsg || t('看板加载失败'))
        }
      })
      .catch((e: any) => setError(e?.message || t('看板加载失败')))
      .finally(() => setLoading(false))
  }, [])

  if (loading) {
    return <div style={{ padding: 48, textAlign: 'center' }}><Spin /></div>
  }

  if (!board) {
    return (
      <div style={{ minHeight: 400, padding: '16px 20px 24px' }}>
        <Empty
          image={Empty.PRESENTED_IMAGE_SIMPLE}
          description={error || t('系统监控看板未就绪')}
        />
      </div>
    )
  }

  return (
    <div style={{ minHeight: 400, padding: '16px 20px 24px' }}>
      <CanvasBoard boardId={board.id} />
    </div>
  )
}

export default SystemMonitor
