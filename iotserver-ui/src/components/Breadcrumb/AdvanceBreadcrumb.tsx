import React, { useMemo } from 'react'
import { Breadcrumb, Typography } from 'antd'
import { LeftOutlined } from '@ant-design/icons'
import { Link, useLocation, useNavigate } from 'react-router-dom'
import { lookupMenuName } from '@/routes/menus'
import './style.less'

const { Title, Paragraph, Text } = Typography

export interface BreadcrumbItem {
  to: string
  label: string
}

export interface AdvanceBreadcrumbProps {
  title: React.ReactNode
  describe: React.ReactNode
  hasBack?: boolean
  background?: any
  breadcrumb?: BreadcrumbItem[]
  /** 附加类名。Layout 统一渲染的标题条会带 page-banner，用于统一它下方的间距 */
  className?: string
}

const AdvanceBreadcrumb: React.FC<AdvanceBreadcrumbProps> = ({ title, describe, background, breadcrumb, hasBack = false, className }) => {
  const location = useLocation()
  const navigate = useNavigate()

  const backgroundImage = useMemo(() => {
    return `url(${background || require('@/assets/images/head-bg.8b029587.png')})`
  }, [background])

  const defaultBreadcrumb = useMemo<BreadcrumbItem[]>(() => {
    const pathSnippets = location.pathname.split('/').filter(i => i)
    return pathSnippets
      .map((_, index) => {
        const to = `/${pathSnippets.slice(0, index + 1).join('/')}`
        const label = lookupMenuName(to)
        return label ? { to, label } : null
      })
      .filter((item): item is BreadcrumbItem => item !== null)
      // 首层面包屑固定是「首页」，首页自身不再重复一次
      .filter(item => item.label !== '首页')
  }, [location])

  return (
    <div className={`advance-breadcrumb-title${className ? ` ${className}` : ''}`} style={{ backgroundImage }}>
      <Breadcrumb>
        <Breadcrumb.Item key="home">
          <Link to="/">首页</Link>
        </Breadcrumb.Item>
        {(breadcrumb?.length ? breadcrumb : defaultBreadcrumb).map(({ to, label }, index) => (
          <Breadcrumb.Item key={index}>
            <Text
              onClick={() => {
                if (to) {
                  navigate(to)
                }
              }}
            >{label}</Text>
          </Breadcrumb.Item>
        ))}
      </Breadcrumb>
      <div className="flex">
        {hasBack && (
          <div style={{ paddingTop: 22, paddingLeft: 10 }}>
            {/* <span className="square"></span> */}
            <LeftOutlined
              className="back-icon"
              onClick={() => {
                navigate(-1)
              }}
            />
          </div>
        )}
        <div style={{ flex: '1 1 auto' }}>
          <Title className="title" level={4}>{title}</Title>
          <Paragraph className="paragraph">{describe}</Paragraph>
        </div>
      </div>
    </div>
  )
}

export default AdvanceBreadcrumb
