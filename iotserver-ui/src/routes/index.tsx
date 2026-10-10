import React from 'react'
import { Navigate, createBrowserRouter, useParams } from 'react-router-dom'

import App from '@/App'
import ServiceMarket from '@/pages/cloud/Service'
import MyService from '@/pages/cloud/MyService'
import ProductManage from '@/pages/gateway/ProductManage'
import ProductDetail from '@/pages/gateway/ProductManage/Detail'
import DeviceManage from '@/pages/gateway/DeviceManage'
import DeviceDetail from '@/pages/gateway/DeviceManage/Detail'
import MyDrive from '@/pages/gateway/MyDrive'
import DriveImage from '@/pages/gateway/DriveImage'
import Dashboard from '@/pages/home/Dashboard'
import AlarmCenter from '@/pages/monitor/AlarmCenter'
import SystemMonitor from '@/pages/monitor/SystemMonitor'
import RuleDetail from '@/pages/monitor/AlarmCenter/Detail'
import Login from '@/pages/auth/login/index'
import ResourceManage from '@/pages/advanced/ResourceManage'
import RuleEngine from '@/pages/advanced/RuleEngine'
import SceneLinkage from '@/pages/advanced/SceneLinkage'
import LinkageLog from '@/pages/advanced/SceneLinkage/LinkageLog'
import Documents from '@/pages/documents/Documents'
import SysUser from '@/pages/admin/SysUser'
import SysRole from '@/pages/admin/SysRole'
import SysMenu from '@/pages/admin/SysMenu'
import SysDept from '@/pages/admin/SysDept'
import SysLoginLog from '@/pages/admin/SysLoginLog'
import SysSet from '@/pages/admin/SysSet'
import DataHistoryQuery from '@/pages/data/HistoryQuery'
import UserProfile from '@/pages/admin/UserProfile'
import ShaperDashboard from '@/pages/viz/Dashboard'
import BoardManage from '@/pages/viz/BoardManage'

import { Empty } from 'antd'

// 看板编辑器用到 react-grid-layout（体积较大），按需加载
const BoardEdit = React.lazy(() => import('@/pages/viz/BoardEdit'))

/** 旧地址 /viz/boards/:id/canvas → /viz/boards/:id/edit */
const LegacyCanvasRedirect: React.FC = () => {
  const { id = '' } = useParams<{ id: string }>()
  return <Navigate to={`/viz/boards/${id}/edit`} replace />
}

const router = createBrowserRouter([
  {
    path: '/auth/login',
    element: <Login />,
  },
  {
    path: '/',
    element: <App />,
    children: [
      {
        path: '/cloud/service',
        element: <ServiceMarket />,
      },
      {
        path: '/cloud/my/service',
        element: <MyService />,
      },
      {
        path: '/gateway/product/manage',
        element: <ProductManage />,
      },
      {
        path: '/gateway/product/detail/:id',
        element: <ProductDetail />,
      },
      {
        path: '/gateway/device/manage',
        element: <DeviceManage />,
      },
      {
        path: '/gateway/device/manage/:id',
        element: <DeviceManage />,
      },
      {
        path: '/gateway/device/detail/:id',
        element: <DeviceDetail />,
      },
      {
        path: '/gateway/drive/image',
        element: <DriveImage />,
      },
      {
        path: '/gateway/my/drive',
        element: <MyDrive />,
      },
      {
        path: '/home/dashboard',
        element: <Dashboard />,
      },
      {
        path: '/monitor/alarmCenter/list',
        element: <AlarmCenter />,
      },
      {
        path: '/monitor/systemMonitor',
        element: <SystemMonitor />,
      },
      {
        path: '/monitor/alarmCenter/detail/:id',
        element: <RuleDetail />,
      },
      {
        path: '/advanced/resource/manage',
        element: <ResourceManage />,
      },
      {
        path: '/advanced/rule/engine',
        element: <RuleEngine />,
      },
      {
        path: '/advanced/scene/linkage',
        element: <SceneLinkage />,
      },
      {
        path: '/advanced/scene/log/:id',
        element: <LinkageLog />,
      },
      {
        path: '/documents/documents',
        element: <Documents />,
      },
      // ===== 数据中心 =====
      {
        path: '/data/history',
        element: <DataHistoryQuery />,
      },
      // ===== 系统管理 =====
      {
        path: '/admin/sysUser',
        element: <SysUser />,
      },
      {
        path: '/admin/sysRole',
        element: <SysRole />,
      },
      {
        path: '/admin/sysMenu',
        element: <SysMenu />,
      },
      {
        path: '/admin/sysDept',
        element: <SysDept />,
      },
      {
        path: '/admin/sysLoginLog',
        element: <SysLoginLog />,
      },
      {
        path: '/admin/sysSet',
        element: <SysSet />,
      },
      {
        path: '/common/userProfile',
        element: <UserProfile />,
      },
      // ===== 可视化 =====
      {
        path: '/viz/dashboard',
        element: <ShaperDashboard />,
      },
      {
        path: '/viz/boards',
        element: <BoardManage />,
      },
      {
        path: '/viz/boards/:id/edit',
        element: (
          <React.Suspense fallback={<div style={{ padding: 48, textAlign: 'center' }}>加载看板…</div>}>
            <BoardEdit />
          </React.Suspense>
        ),
      },
      // 兼容旧链接：画布编辑器已改为看板编辑器，保留 id 跳过去
      {
        path: '/viz/boards/:id/canvas',
        element: <LegacyCanvasRedirect />,
      },
    ],
  },
  {
    path: '*',
    element: <Empty description={'404 Not Found'} style={{ marginTop: '300px' }}/>,
  },
])

export default router
