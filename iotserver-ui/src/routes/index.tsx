import React from 'react'
import { Navigate, createBrowserRouter, useParams } from 'react-router-dom'

import App from '@/App'
import ServiceMarket from '@/pages/cloud/Service'
import MyService from '@/pages/cloud/MyService'
import ProductManage from '@/pages/gateway/ProductManage'
import ProductDetail from '@/pages/gateway/ProductManage/Detail'
import DeviceManage from '@/pages/gateway/DeviceManage'
import DeviceDetail from '@/pages/gateway/DeviceManage/Detail'
import GatewayManage from '@/pages/gateway/GatewayManage'
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
import SysAuditLog from '@/pages/admin/SysAuditLog'
import SysSet from '@/pages/admin/SysSet'
import PlatformTenant from '@/pages/platform/Tenant'
import SolutionCenter from '@/pages/solution/Center'
import DataHistoryQuery from '@/pages/data/HistoryQuery'
import UserProfile from '@/pages/admin/UserProfile'
import VizDashboard from '@/pages/viz/Dashboard'
import BoardManage from '@/pages/viz/BoardManage'
import VizView from '@/pages/viz/View'

import { Empty } from 'antd'
import { t } from '@/i18n'

// 看板编辑器用到 gridstack（体积较大），按需加载
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
  // 看板大屏 / 分享页：不套后台外壳，整页只有看板
  {
    path: '/viz/view/:id',
    element: <VizView />,
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
        path: '/gateway/gateway/manage',
        element: <GatewayManage />,
      },
      {
        path: '/gateway/drive/image',
        element: <DriveImage />,
      },
      {
        path: '/gateway/my/drive',
        element: <MyDrive />,
      },
      // ===== 方案中心（按行业预置的整机方案，菜单由后端下发） =====
      {
        path: '/solution/center',
        element: <SolutionCenter />,
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
        path: '/admin/sysAuditLog',
        element: <SysAuditLog />,
      },
      // 旧地址 /admin/sysLoginLog → /admin/sysAuditLog，保证旧书签不 404
      {
        path: '/admin/sysLoginLog',
        element: <Navigate to="/admin/sysAuditLog" replace />,
      },
      {
        path: '/platform/sysSet',
        element: <SysSet />,
      },
      {
        path: '/common/userProfile',
        element: <UserProfile />,
      },
      // ===== 平台管理（平台管理员专属，入口由后端菜单下发） =====
      {
        path: '/platform/tenant',
        element: <PlatformTenant />,
      },
      // ===== 可视化 =====
      {
        path: '/viz/dashboard',
        element: <VizDashboard />,
      },
      {
        path: '/viz/boards',
        element: <BoardManage />,
      },
      {
        path: '/viz/boards/:id/edit',
        element: (
          <React.Suspense fallback={<div style={{ padding: 48, textAlign: 'center' }}>{t('加载看板…')}</div>}>
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
