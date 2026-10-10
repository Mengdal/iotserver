import { Card, Col, Row, Typography } from 'antd'
import { AlertOutlined, AppstoreOutlined, CloudOutlined, ClusterOutlined } from '@ant-design/icons'
import React from 'react'
import { PageInfo } from '../types'
import './Overview.less'

const { Text, Title } = Typography

interface OverviewProps {
  pageInfo?: PageInfo
}

interface OverviewTileProps {
  tone: 'product' | 'device' | 'cloud' | 'alert'
  icon: React.ReactNode
  title: string
  count?: number
  tags?: { label: string, value: string }[]
  note?: string
}

const OverviewTile: React.FC<OverviewTileProps> = ({ tone, icon, title, count, tags, note }) => (
  <div className={`overview-tile overview-tile--${tone}`}>
    <div className="overview-tile__head">
      <span className="overview-tile__icon">{icon}</span>
      <span className="overview-tile__title">{title}</span>
    </div>
    <div className="overview-tile__count">{count === undefined ? 0 : count}</div>
    <div className="overview-tile__foot">
      {(tags || []).map(tag => (
        <span className="overview-tile__tag" key={tag.label}>
          {tag.label}
          <b>{tag.value}</b>
        </span>
      ))}
      {note ? <span className="overview-tile__note">{note}</span> : null}
    </div>
  </div>
)

/**
 * 平台概述：参考 demo 的仪表盘样式 —— 卡片标题带副标题，四个指标为带图标/底色的色块，
 * 指标下方用小标签展示子项（本地/第三方、运行中/已停止），不再是原先的分栏下划线列表。
 */
const Overview: React.FC<OverviewProps> = ({ pageInfo }) => (
  <Card
    className="overview"
    title={(
      <div>
        <Title level={4} style={{ marginBottom: 0 }}>平台概述</Title>
        <Text type="secondary" style={{ fontSize: 12, fontWeight: 400 }}>产品、设备、云插件与告警关键指标</Text>
      </div>
    )}
  >
    <Row gutter={[16, 16]}>
      <Col span={24} lg={6}>
        <OverviewTile
          tone="product"
          icon={<AppstoreOutlined />}
          title="产品数"
          count={pageInfo?.product.total}
          tags={[
            { label: '本地', value: `${pageInfo?.product.self === undefined ? 0 : pageInfo?.product.self}个` },
            { label: '第三方', value: `${pageInfo?.product.other === undefined ? 0 : pageInfo?.product.other}个` },
          ]}
        />
      </Col>
      <Col span={24} lg={6}>
        <OverviewTile
          tone="device"
          icon={<ClusterOutlined />}
          title="设备数"
          count={pageInfo?.device.total}
          tags={[
            { label: '本地', value: `${pageInfo?.device.self === undefined ? 0 : pageInfo?.device.self}个` },
            { label: '第三方', value: `${pageInfo?.device.other === undefined ? 0 : pageInfo?.device.other}个` },
          ]}
        />
      </Col>
      <Col span={24} lg={6}>
        <OverviewTile
          tone="cloud"
          icon={<CloudOutlined />}
          title="云插件"
          count={Number(pageInfo?.cloudInstance.count || 0)}
          tags={[
            { label: '运行中', value: `${pageInfo?.cloudInstance.run_count || 0}个` },
            { label: '已停止', value: `${pageInfo?.cloudInstance.stop_count || 0}个` },
          ]}
        />
      </Col>
      <Col span={24} lg={6}>
        <OverviewTile
          tone="alert"
          icon={<AlertOutlined />}
          title="告警数"
          count={pageInfo?.alert.total}
          note="数据统计截止昨日 24 时"
        />
      </Col>
    </Row>
  </Card>
)

export default Overview
