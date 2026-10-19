import { Login, getInitInfo, initPassword } from '@/api'
import useRequest from '@/hooks/useRequest'
import { clearCurrentTenantId, setUser } from '@/utils/auth'
import { Button, Form, Input, message } from 'antd'
import React, { useCallback } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import './style.less'
import { useBrand } from '@/components/Brand'
import LanguageSwitcher from '@/components/LanguageSwitcher'
import { LOCALE_STORAGE_KEY, currentLocale, normalizeLocale, t } from '@/i18n'

export default function login () {
  const [search] = useSearchParams()
  const navigate = useNavigate()
  const initInfo = useRequest(getInitInfo)
  const { site, assets: brandAssets } = useBrand()

  // console.log(initInfo.data?.isInit)

  console.log(search)
  const loginSuccess = useCallback(() => {
    const path = search.get('path') || '/home/dashboard'
    navigate(path)
  }, [])

  const onFinish = (values: any) => {
    Login(values)
      .then(resp => {
        if (resp.success) {
          // 新会话默认「平台视图」，不沿用上个账号留下的代管租户
          clearCurrentTenantId()
          setUser(resp.result)
          // 语言跟账号走：登录接口会返回该用户保存的语言偏好
          const accountLocale = normalizeLocale(resp.result?.user?.lang)
          if (accountLocale) localStorage.setItem(LOCALE_STORAGE_KEY, accountLocale)
          if (accountLocale && accountLocale !== currentLocale()) {
            // 当前页面已按旧语言渲染（菜单/状态字典是模块级常量），整页进入目标地址让语言立即生效
            window.location.replace(search.get('path') || '/home/dashboard')
            return
          }
          loginSuccess()
        } else {
          message.error(resp.errorMsg)
        }
      })
  }

  const onInitFinish = (values: any) => {
    values.password1 = undefined
    initPassword(values)
      .then(resp => {
        if (resp.success) {
          message.success(t('初始化密码成功'))
          initInfo.reload()
        } else {
          message.error(resp.errorMsg)
        }
      })
  }

  return (<>
    <div className="login">
      {/* 登录页也要能切语言：未登录时选的语言同样写进 localStorage */}
      <div style={{ position: 'fixed', top: 16, right: 24, zIndex: 10 }}>
        <LanguageSwitcher />
      </div>
      <div className="login-content">
        <div
          className={`login-content__left${site.loginBackground ? ' login-content__left--custom' : ''}`}
          style={site.loginBackground
            ? { backgroundImage: `url(${site.loginBackground})` }
            : undefined}
        >
          {/* 未上传 logo 时回退到系统默认 */}
          <img className="login-content__logo" src={brandAssets.loginLogo} alt={site.name} />
          <div className="login-content__leftTitle">
            {site.name || t('iotserver 物联网平台')}
          </div>
        </div>
        {initInfo.data?.isInit
          ? (<div className="login-content__right">
            <div className="login-content__rightTitle">
              {t('密码登录')}
            </div>
            <Form
              name="basic"
              style={{
                margin: '80px 80px',
              }}
              onFinish={onFinish}
              autoComplete="off"
              className="login-form"
            >
              <Form.Item
                name="username"
                rules={[
                  {
                    required: true,
                    message: t('请输入'),
                  },
                ]}
              >
                <Input placeholder={t('账号')} />
              </Form.Item>

              <Form.Item
                name="password"
                style={{ marginTop: '60px' }}
                rules={[
                  {
                    required: true,
                    message: t('请输入'),
                  },
                ]}
              >
                <Input.Password placeholder={t('密码')} />
              </Form.Item>

              <Form.Item>
                <Button
                  style={{
                    backgroundColor: '#232F3E',
                    borderRadius: '40px',
                    color: '#fefeff',
                    width: '80%',
                    height: '50px',
                    marginTop: '60px',
                    marginLeft: '50px',
                  }}
                  htmlType="submit">
                  {t('登录')}
                </Button>
              </Form.Item>
            </Form>
          </div>)
          : (<div className="login-content__right">
            <div className="login-content__rightTitle">
              {t('初始化密码')}
            </div>
            <Form
              name="basic"
              style={{
                margin: '80px 80px',
              }}
              onFinish={onInitFinish}
              autoComplete="off"
              className="login-form"
            >
              <Form.Item
                name="newPassword"
                rules={[
                  {
                    required: true,
                    message: t('请输入'),
                  },
                ]}
              >
                <Input placeholder={t('密码')} />
              </Form.Item>
              <Form.Item
                name="password1"
                style={{ marginTop: '60px' }}
                dependencies={['newPassword']}
                rules={[
                  {
                    required: true,
                    message: t('请输入'),
                  },
                  ({ getFieldValue }) => ({
                    validator (_, value) {
                      if (!value || getFieldValue('newPassword') === value) {
                        return Promise.resolve()
                      }
                      return Promise.reject(new Error(t('两次密码输⼊不⼀致')))
                    },
                  }),
                ]}
              >
                <Input.Password placeholder={t('确认密码')} />
              </Form.Item>

              <Form.Item>
                <Button
                  style={{
                    backgroundColor: '#232F3E',
                    borderRadius: '40px',
                    color: '#fefeff',
                    width: '80%',
                    height: '50px',
                    marginTop: '60px',
                    marginLeft: '50px',
                  }}
                  htmlType="submit">
                  {t('确定')}
                </Button>
              </Form.Item>
            </Form>
          </div>)
        }
      </div>
    </div>
  </>)
}
