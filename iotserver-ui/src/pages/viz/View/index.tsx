import React, { useEffect, useState } from 'react'
import { useParams } from 'react-router-dom'
import { Alert, Spin } from 'antd'
import { getVizBoard, VizBoardDetail } from '@/api'
import CanvasBoard from '@/components/CanvasBoard'
import { t } from '@/i18n'

/**
 * 看板只读展示页（大屏 / 分享链接）
 *
 * 挂在后台外壳**之外**（/viz/view/:id），整页只有看板本身：
 *   - 按看板保存的布局渲染，窄屏自动折列（GridCanvas 的 responsive）；
 *   - 按看板的「自动刷新间隔」轮询取数；
 *   - 没有任何编辑入口，权限仍由后端校验（未登录/无权限时取数会失败并提示）。
 */
const VizView: React.FC = () => {
  const { id = '' } = useParams<{ id: string }>()
  const [board, setBoard] = useState<VizBoardDetail | undefined>()
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  useEffect(() => {
    if (!id) {
      setError(t('缺少看板 id'))
      setLoading(false)
      return
    }
    setLoading(true)
    getVizBoard(id)
      .then((resp) => {
        if (resp.success) setBoard(resp.result)
        else setError(resp.errorMsg || t('看板加载失败'))
      })
      .catch((e: any) => setError(e?.message || t('看板加载失败')))
      .finally(() => setLoading(false))
  }, [id])

  if (loading) {
    return (
      <div style={{ height: '100vh', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
        <Spin size="large" />
      </div>
    )
  }
  if (error) {
    return <Alert type="error" showIcon style={{ margin: 24 }} message={t('看板加载失败')} description={error} />
  }
  return (
    <div style={{ minHeight: '100vh', padding: 12, background: '#f5f6f8', boxSizing: 'border-box' }}>
      {board ? <CanvasBoard board={board} /> : null}
    </div>
  )
}

export default VizView
