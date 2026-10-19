import React, { useEffect, useMemo, useState } from 'react'
import { Outlet, useLocation, useNavigate } from 'react-router-dom'
import { UserOutlined } from '@ant-design/icons'
import { Layout, Space, Menu, Avatar, Dropdown, Modal, Form, Input, Select, MenuProps, message } from 'antd'
import { buildMenus, buildMenusFromBackend, isExternalLink } from '@/routes/menus'
import { resolveMenuPath, resolvePageMeta, resolvePageTitle } from '@/routes/pageMeta'
import { AdvanceBreadcrumb } from '@/components/Breadcrumb'
import { useBrand } from '@/components/Brand'
import LanguageSwitcher from '@/components/LanguageSwitcher'
import type { BackendMenuItem, SubMenuType } from '@/routes/menus'
import './style.less'
import { clearUser, getCurrentTenantId, getUser, setCurrentTenantId, setUser } from '@/utils/auth'
import { changePassword, getTenants, getUserGetInfo, updateUserProfile, Tenant } from '@/api'
import { t, tt } from '@/i18n'

const { Header, Content, Sider } = Layout
const { useForm } = Form
const { useMessage } = message

export interface LyaoutProps {

}

function getDisplayName (): string {
  const info = getUser()?.user
  return info?.nickname || info?.username || 'admin'
}

const MainLayout: React.FC = () => {
  const [collapsed, setCollapsed] = useState(false)
  const [messageApi, contextHolder] = useMessage()
  const [form] = useForm()
  const [profileForm] = useForm()
  const [isModalOpen, setIsModalOpen] = useState(false)
  const [isProfileOpen, setIsProfileOpen] = useState(false)

  const location = useLocation()
  const navigate = useNavigate()
  const { site, assets: brandAssets } = useBrand()

  // 登录态：从 localStorage 读取，登录态被补齐后通过 bumpAuth 触发刷新
  const [auth, setAuth] = useState<any>(() => getUser())
  const bumpAuth = () => setAuth(getUser())

  const user = auth
  const userInfo = user?.user
  const admin = userInfo?.admin === true || user?.admin === true
  // 平台（ROBX）管理员：显示租户切换器（登录接口返回 platformAdmin）
  const platformAdmin = userInfo?.platformAdmin === true || user?.platformAdmin === true
  const [tenants, setTenants] = useState<Tenant[]>([])
  const [currentTenantId, setCurrentTenantIdState] = useState<string>(() => getCurrentTenantId())
  const permissions: string[] = useMemo(() => {
    const direct = user?.permissions
    if (Array.isArray(direct) && direct.length > 0) return direct
    return Array.isArray(userInfo?.permissions) ? userInfo.permissions : []
  }, [user])

  // 侧边栏数据源：后端菜单（登录接口返回的 menus）。层级/顺序/图标/显示隐藏都以「菜单管理」为准
  const backendMenus = useMemo<BackendMenuItem[]>(() => {
    const direct = user?.menus
    if (Array.isArray(direct) && direct.length > 0) return direct
    const nested = userInfo?.menus
    return Array.isArray(nested) ? nested : []
  }, [user, userInfo])

  // 登录态里是否有 menus 字段：有就以它为准（哪怕是空的，也不该凭空造菜单）
  const hasBackendMenus = Array.isArray(user?.menus) || Array.isArray(userInfo?.menus)

  const siderMenus = useMemo(() => {
    const fromBackend = buildMenusFromBackend(backendMenus)
    if (hasBackendMenus) return fromBackend
    // 老版本登录态/接口异常时没有 menus 字段，先用前端内置菜单兜底，避免侧边栏空白
    return buildMenus({ admin, permissions })
  }, [backendMenus, hasBackendMenus, admin, permissions])

  // 每次进入/刷新页面都同步一次「当前用户的菜单与权限」
  // 这样在【角色管理】里改了某个角色的菜单权限、或在【菜单管理】里改了菜单/权限标识后，
  // 受影响的人只需刷新页面（F5）即可生效，无需退出重新登录。
  // 注意：后端的接口鉴权始终是实时的，这里只是让前端 UI 与后端保持一致。
  useEffect(() => {
    const stored = getUser()
    if (!stored || !stored.token) return
    let alive = true
    getUserGetInfo().then(resp => {
      if (!alive || !resp.success) return
      const { menus, permissions: perms, admin: isAdminUser } = resp.result
      setUser({
        ...stored,
        // menus 缺失时保留原值，避免后端未返回菜单反而把侧边栏清空
        user: { ...(stored.user || {}), ...(Array.isArray(menus) ? { menus } : {}), admin: isAdminUser },
        ...(Array.isArray(menus) ? { menus } : {}),
        permissions: Array.isArray(perms) ? perms : stored.permissions || [],
        roles: stored.roles || [],
      })
      bumpAuth()
    }).catch(() => { /* 忽略：保持既有登录态，下次刷新再试 */ })
    return () => { alive = false }
  }, [])

  // 租户切换器数据：仅平台管理员拉取（后端只会把「租户管理」菜单下发给平台管理员）
  useEffect(() => {
    if (!platformAdmin) {
      setTenants([])
      return
    }
    let alive = true
    // 注意：/api/v1/tenant 返回的是分页对象 { list, total, page, pageSize }，
    // 不能直接塞进数组 state（否则 tenants.map 会抛 “map is not a function” 整页白屏）。
    // 这里同时兼容数组形态，避免后端将来改成直接返回数组时又踩一次。
    getTenants({ isAll: true, status: -1 })
      .then(resp => {
        if (!alive || !resp.success) return
        const raw: unknown = resp.result
        const list: Tenant[] = Array.isArray(raw)
          ? (raw as Tenant[])
          : Array.isArray((raw as { list?: Tenant[] } | null)?.list)
            ? ((raw as { list: Tenant[] }).list)
            : []
        setTenants(list)
      })
      .catch(() => { /* 忽略：切换器拉不到列表不影响主流程 */ })
    return () => { alive = false }
  }, [platformAdmin])

  /** 切换「当前租户」：写入 localStorage 后整页刷新，确保所有页面数据都按新租户重新拉取 */
  const handleTenantChange = (tenantId?: string) => {
    const next = tenantId || ''
    setCurrentTenantId(next)
    setCurrentTenantIdState(next)
    window.location.reload()
  }

  const [sliderMenuOpenKeys, setSliderMenuOpenKeys] = useState<string[]>([])

  // 侧边栏当前选中项：详情页/带过滤参数的列表页（如 /gateway/device/manage/:productId）
  // 没有与完整路径一一对应的菜单，回落到所属菜单，避免跳转后侧边栏没有任何高亮。
  const selectedMenuKey = useMemo(() => {
    const menuPath = resolveMenuPath(location.pathname)
    if (menuPath) return menuPath
    let best = ''
    const walk = (items: any[]) => {
      items.forEach((item) => {
        const key = String(item?.key || '')
        if (key.startsWith('/') && (location.pathname === key || location.pathname.startsWith(key + '/'))) {
          if (key.length > best.length) best = key
        }
        if (Array.isArray(item?.children)) walk(item.children)
      })
    }
    walk(siderMenus as any[])
    return best || location.pathname
  }, [siderMenus, location.pathname])

  useEffect(() => {
    // 内容区自己滚动（见 style.less）：切路由时手动回到顶部，保持原来整页滚动的体验
    const view = document.querySelector('.main-view')
    if (view) view.scrollTop = 0
    const { paths, titles } = siderMenus.reduce((r, menu) => {
      const item = menu as SubMenuType
      const children = (Array.isArray(item.children) ? item.children : []) as SubMenuType[]
      const findItem = children.find(child => child.key === location.pathname)
      if (findItem) {
        r.paths.push(item.key as string)
        r.titles.push(String(findItem!.label || item.label))
        return r
      }
      if (item.key === location.pathname) {
        r.titles.push(String(item.label))
        return r
      }
      // 动态路径（如 /gateway/device/manage/:productId）：只要展开所属菜单，
      // 浏览器标题仍交给下面的 resolvePageTitle 兜底（详情页要显示「设备详情」而不是列表名）
      if (children.some(child => child.key === selectedMenuKey)) {
        r.paths.push(item.key as string)
      }
      return r
    }, { paths: [], titles: [] } as { paths: string[], titles: string[] })

    // 未命中菜单时（详情页等）回退到菜单名/页面元数据里的标题
    if (titles.length === 0) {
      const fallback = resolvePageTitle(location.pathname)
      if (fallback) titles.push(fallback)
    }

    const brandName = site.name || 'iotserver'
    if (titles[titles.length - 1]) {
      const pageTitle = titles[titles.length - 1]
      document.title = pageTitle === brandName ? pageTitle : `${pageTitle} - ${brandName}`
      localStorage.setItem('title', pageTitle)
    } else {
      document.title = brandName
    }
    setSliderMenuOpenKeys(paths)
  }, [location.pathname, siderMenus, selectedMenuKey])

  const handleOk = async () => {
    const values = await form.validateFields()
    values.newPassword1 = undefined
    changePassword(values)
      .then(resp => {
        if (resp.success) {
          messageApi.open({ type: 'success', content: t('修改成功') })
          setIsModalOpen(false)
          form.resetFields()
          setTimeout(() => {
            clearUser()
            navigate('/auth/login')
          }, 1000)
        } else {
          messageApi.open({ type: 'error', content: resp.errorMsg })
        }
      })
  }

  const handleCancel = () => {
    setIsModalOpen(false)
    form.resetFields()
  }

  const showModal = () => {
    setIsModalOpen(true)
  }

  const openProfile = () => {
    profileForm.setFieldsValue({
      username: userInfo?.username,
      nickname: userInfo?.nickname,
      email: userInfo?.email,
      phone: userInfo?.phone,
    })
    setIsProfileOpen(true)
  }

  const handleProfileOk = async () => {
    const values = await profileForm.validateFields()
    updateUserProfile(values).then(resp => {
      if (resp.success) {
        messageApi.open({ type: 'success', content: t('保存成功') })
        setIsProfileOpen(false)
        const stored = getUser()
        if (stored?.user) {
          stored.user = { ...stored.user, ...values }
          localStorage.setItem('$user$', JSON.stringify(stored))
        }
      } else {
        messageApi.open({ type: 'error', content: resp.errorMsg })
      }
    })
  }

  const items: MenuProps['items'] = [
    {
      key: 'profile',
      label: <div onClick={openProfile}>{t('个人中心')}</div>,
    },
    {
      key: 'password',
      label: <div onClick={showModal}>{t('修改密码')}</div>,
    },
    {
      key: 'logout',
      label: (
        <div
          onClick={() => {
            clearUser()
            navigate('/auth/login')
          }}
        >
          {t('退出登录')}
        </div>
      ),
    },
  ]

  // 页面标题条：标题取「菜单管理」里的菜单名（取不到时回落到页面元数据兜底），
  // 描述文案来自 @/routes/pageMeta；详情页等标题依赖页面数据的路由由页面自己渲染。
  const pageMeta = resolvePageMeta(location.pathname)
  const bannerTitle = resolvePageTitle(location.pathname)
  const showBanner = (pageMeta.banner || 'layout') === 'layout' && !!bannerTitle

  // 租户切换器选项：空值 = 平台视图（不带 x-tenant-id，查看全部租户）
  // 兜底再判一次数组：这个 useMemo 一旦抛错会整页白屏（曾因接口返回分页对象踩过）
  const tenantOptions = useMemo(() => [
    { value: '', label: t('全部租户（平台视图）') },
    ...(Array.isArray(tenants) ? tenants : []).map(item => ({
      value: item.id,
      label: `${item.name}（${item.code}）${item.status === 1 ? '' : t(' 已停用')}`,
    })),
  ], [tenants])

  return (
    <Layout className="main-layout">
      {contextHolder}
      <Header className="main-header">
        <Space size={20}>
          <div
            style={{ cursor: 'pointer', display: 'flex', alignItems: 'center', gap: 8 }}
            onClick={() => {
              navigate('/home/dashboard')
            }}>
            <img
              src={brandAssets.logo}
              alt=""
              style={{ height: 26, maxWidth: 120, objectFit: 'contain' }}
            />
            <div style={{ fontSize: '14px', fontWeight: 600 }}>
              {site.name || 'iotserver'}
            </div>
          </div>
          <div></div>
          {platformAdmin && (
            <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
              <span style={{ fontSize: '13px', color: 'rgba(0, 0, 0, 0.65)' }}>{t('当前租户')}</span>
              <Select
                size="small"
                showSearch
                optionFilterProp="label"
                style={{ width: 220 }}
                value={currentTenantId}
                placeholder={t('全部租户（平台视图）')}
                options={tenantOptions}
                onChange={handleTenantChange}
              />
            </div>
          )}
          <LanguageSwitcher />
          <Dropdown
            menu={{ items }}
            placement="bottom"
            arrow={{ pointAtCenter: true }}
            trigger={['click']}
          >
            <Space size={6} style={{ cursor: 'pointer' }}>
              <Avatar icon={<UserOutlined />} size="small" />
              <span style={{ fontSize: '13px' }}>{tt(getDisplayName())}</span>
            </Space>
          </Dropdown>
        </Space>
      </Header>
      <Layout className="main-body" hasSider>
        <Sider
          width={200}
          className="main-sider"
          theme="light"
          collapsible
          collapsed={collapsed}
          onCollapse={value => setCollapsed(value)}
        >
          <Menu
            mode="inline"
            style={{ height: '100%', borderRight: 0 }}
            items={siderMenus}
            openKeys={sliderMenuOpenKeys}
            onOpenChange={openKeys => {
              setSliderMenuOpenKeys(openKeys)
            }}
            selectedKeys={[selectedMenuKey]}
            onClick={({ key }) => {
              const target = String(key)
              // 菜单管理里配置的外链（http/https）新窗口打开
              if (isExternalLink(target)) {
                window.open(target)
                return
              }
              navigate(target)
            }}
          />
        </Sider>
        <Layout className="main-container" style={{ marginLeft: collapsed ? 80 : 200 }}>
          <Content className="main-view">
            {showBanner && (
              <AdvanceBreadcrumb
                className="page-banner"
                title={bannerTitle}
                describe={pageMeta.describe}
                centered={pageMeta.bannerLayout === 'centered'}
                // 页面可通过 createPortal 把内容（如搜索框）投到这个挂载点上
                extra={<div id="page-banner-extra" />}
              />
            )}
            <Outlet></Outlet>
          </Content>
        </Layout>
      </Layout>

      <Modal
        open={isModalOpen}
        onOk={handleOk}
        onCancel={handleCancel}
        okText={t('确定')}
        cancelText={t('取消')}
        maskClosable={false}
        title={t('修改密码')}
      >
        <Form form={form} layout="vertical" style={{ marginTop: '20px' }}>
          <Form.Item
            label={t('原密码')}
            name="currentPassword"
            rules={[{ required: true, message: t('请输入') }]}
          >
            <Input placeholder={t('请输入原密码')} />
          </Form.Item>
          <Form.Item
            label={t('新密码')}
            name="newPassword"
            rules={[{ required: true, message: t('请输入') }, { min: 6, message: t('密码长度不能少于 6 位') }]}
          >
            <Input.Password placeholder={t('请输入新密码')} />
          </Form.Item>
          <Form.Item
            label={t('确认密码')}
            name="newPassword1"
            dependencies={['newPassword']}
            rules={[
              { required: true, message: t('请输入') },
              ({ getFieldValue }) => ({
                validator (_, value) {
                  if (!value || getFieldValue('newPassword') === value) {
                    return Promise.resolve()
                  }
                  return Promise.reject(new Error(t('两次密码输入不一致')))
                },
              }),
            ]}
          >
            <Input.Password placeholder={t('请输入确认密码')} />
          </Form.Item>
        </Form>
      </Modal>

      <Modal
        open={isProfileOpen}
        onOk={handleProfileOk}
        onCancel={() => setIsProfileOpen(false)}
        okText={t('保存')}
        cancelText={t('取消')}
        maskClosable={false}
        title={t('个人资料')}
      >
        <Form form={profileForm} layout="vertical" style={{ marginTop: '20px' }}>
          <Form.Item label={t('账号')} name="username">
            <Input disabled />
          </Form.Item>
          <Form.Item label={t('昵称')} name="nickname" rules={[{ required: true, message: t('请输入昵称') }]}>
            <Input placeholder={t('请输入昵称')} />
          </Form.Item>
          <Form.Item label={t('邮箱')} name="email">
            <Input placeholder={t('请输入邮箱')} />
          </Form.Item>
          <Form.Item label={t('手机号')} name="phone">
            <Input placeholder={t('请输入手机号')} />
          </Form.Item>
        </Form>
      </Modal>
    </Layout>
  )
}

const Lyaout: React.FC<LyaoutProps> = () => {
  const location = useLocation()
  if (location.pathname === '/auth/login') {
    return <Outlet></Outlet>
  }
  return <MainLayout />
}

export default Lyaout
