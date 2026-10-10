import React from 'react'
import { Link, useLocation } from 'react-router-dom'
import { Breadcrumb } from 'antd'
import { lookupMenuName } from '@/routes/menus'
import './style.less'

export interface NormalBreadcrumbProps {
  // eslint-disable-next-line react/no-unused-prop-types
  children?: React.ReactNode
}

const NormalBreadcrumb:React.FC<NormalBreadcrumbProps> = () => {
  const location = useLocation()
  const pathSnippets = location.pathname.split('/').filter(i => i)

  return (
    <div className="normal-breadcrumb-title">
      <Breadcrumb>
        <Breadcrumb.Item key="home">
          <Link to="/">首页</Link>
        </Breadcrumb.Item>
        {
          pathSnippets.map((_, index) => {
            const url = `/${pathSnippets.slice(0, index + 1).join('/')}`

            const name = lookupMenuName(url)
            if (name) {
              return (
                <Breadcrumb.Item key={url}>
                  <Link to={url}>{name}</Link>
                </Breadcrumb.Item>
              )
            }
            return null
          })
        }
      </Breadcrumb>
    </div>
  )
}

export default NormalBreadcrumb
