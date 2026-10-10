import React, { useEffect, useState } from 'react'
import { Alert, Card, Spin } from 'antd'
import CanvasBoard from '@/components/CanvasBoard'
import { getVizBoards, VizBoard } from '@/api'

/**
 * 数据看板：展示「看板管理」里设为**数据看板整页**的那块看板。
 *
 * 平台看板（kind=canvas）：按保存的布局渲染（CanvasBoard）。
 * 早期用 SQL 建的 Shaper 看板（kind=shaper）不再内嵌，提示去「看板管理」转成平台看板。
 */
const ShaperDashboardPage: React.FC = () => {
  const [board, setBoard] = useState<VizBoard | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  useEffect(() => {
    getVizBoards()
      .then((resp) => {
        if (!resp.success) {
          setError(resp.errorMsg || '看板加载失败')
          return
        }
        const all = resp.result.list || []
        const def = all.find(b => b.isDefault) || all[0]
        if (def) {
          setBoard(def)
        } else {
          setError('还没有看板，去「看板管理」新建一块')
        }
      })
      .catch((e: any) => setError(e?.message || '看板加载失败'))
      .finally(() => setLoading(false))
  }, [])

  if (loading) {
    return (
      <Card className="admin-page">
        <div style={{ padding: 48, textAlign: 'center' }}><Spin /></div>
      </Card>
    )
  }

  if (error) {
    return (
      <Card className="admin-page">
        <Alert type="info" showIcon message="数据看板尚未就绪" description={error} />
      </Card>
    )
  }

  return (
    <Card className="admin-page" title={board ? board.name : '数据看板'}>
      {board && board.kind === 'canvas'
        ? <CanvasBoard boardId={board.id} />
        : (
          <Alert
            type="info"
            showIcon
            message="这是早期用 SQL 建的看板"
            description="请到「看板管理」里对它执行「转为平台看板」，之后就能在这里正常展示。"
          />
        )}
    </Card>
  )
}

export default ShaperDashboardPage
