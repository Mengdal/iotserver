import React, { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Button, Empty, Spin } from 'antd'
import { getVizBoards, VizBoard } from '@/api'
import CanvasBoard from '@/components/CanvasBoard'
import './index.less'

/**
 * 首页：整页渲染「首页看板」（看板管理里设为首页看板的那块；新装由内置种子自带，
 * 见后端 pkg/seed/boards.go）。
 *
 * 首页不再维护第二套手写 UI：布局、取数、自动刷新都由看板编辑器与画布渲染负责，
 * 改版式不用发前端版本。没有首页看板时（例如被删光）给引导去「看板管理」。
 */
const Dashboard: React.FC = () => {
  const navigate = useNavigate()
  const [board, setBoard] = useState<VizBoard | undefined>()
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  useEffect(() => {
    getVizBoards()
      .then((resp) => {
        if (resp.success) setBoard((resp.result.list || []).find(b => b.isHome))
        else setError(resp.errorMsg || '看板加载失败')
      })
      .catch((e: any) => setError(e?.message || '看板加载失败'))
      .finally(() => setLoading(false))
  }, [])

  if (loading) {
    return <div style={{ padding: 48, textAlign: 'center' }}><Spin /></div>
  }

  if (!board) {
    return (
      <div className="dashboard">
        <Empty
          image={Empty.PRESENTED_IMAGE_SIMPLE}
          description={error || '还没有首页看板：去「看板管理」新建一块，并设为首页看板'}
        >
          <Button type="primary" onClick={() => navigate('/viz/boards')}>去看板管理</Button>
        </Empty>
      </div>
    )
  }

  return (
    <div className="dashboard">
      <CanvasBoard boardId={board.id} />
    </div>
  )
}

export default Dashboard
