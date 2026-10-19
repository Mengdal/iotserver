import React, { useEffect, useState } from 'react'
import { Alert, Card, Spin } from 'antd'
import CanvasBoard from '@/components/CanvasBoard'
import { getVizBoards, VizBoard } from '@/api'
import { t } from '@/i18n'

/**
 * 系统监控：组件由「看板管理」里的「系统监控看板」（画布看板）渲染。
 *
 * 好处：改布局不用发前端版本，在画布编辑器里拖拽/改文本即可。
 */
const SystemMonitor: React.FC = () => {
  const [board, setBoard] = useState<VizBoard | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  useEffect(() => {
    getVizBoards()
      .then((resp) => {
        if (!resp.success) {
          setError(resp.errorMsg || t('看板加载失败'))
          return
        }
        const hit = (resp.result.list || []).find(b => b.name === '系统监控看板')
        if (hit) {
          setBoard(hit)
        } else {
          setError(t('找不到「系统监控看板」，请在「看板管理」里新建/改名'))
        }
      })
      .catch((e: any) => setError(e?.message || t('看板加载失败')))
      .finally(() => setLoading(false))
  }, [])

  if (loading) {
    return <Card className="m20" size="small"><div style={{ padding: 48, textAlign: 'center' }}><Spin /></div></Card>
  }
  if (error) {
    return <Card className="m20" size="small"><Alert type="info" showIcon message={t('系统监控看板未就绪')} description={error} /></Card>
  }

  return (
    <Card className="m20" size="small" title={t('系统监控')}>
      {board ? <CanvasBoard boardId={board.id} /> : null}
    </Card>
  )
}

export default SystemMonitor
