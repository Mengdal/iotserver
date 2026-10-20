import React from 'react'
import { Card, Col, Row } from 'antd'
import {
  AlertOutlined,
  ApiOutlined,
  ApartmentOutlined,
  CloudServerOutlined,
  DashboardOutlined,
  DatabaseOutlined,
  GlobalOutlined,
  HddOutlined,
  SafetyOutlined,
  SettingOutlined,
  TeamOutlined,
  ThunderboltOutlined,
} from '@ant-design/icons'

import { useBrand } from '@/components/Brand'
import './style.less'
import { t } from '@/i18n'

interface Feature {
  icon: React.ReactNode
  title: string
  description: string
}

const FEATURES: Feature[] = [
  {
    icon: <ApiOutlined />,
    title: t('设备接入'),
    description: t('支持 MQTT、HTTP、TCP 等接入方式，上报属性、事件与服务调用数据。'),
  },
  {
    icon: <CloudServerOutlined />,
    title: t('工业网关'),
    description: t('第三方工业网关 MQTT 接入，支持断点续传补传、配置与命令下发。'),
  },
  {
    icon: <DatabaseOutlined />,
    title: t('物模型'),
    description: t('用属性、事件、服务描述设备能力，支持多种数据类型、取值范围与单位。'),
  },
  {
    icon: <TeamOutlined />,
    title: t('多租户'),
    description: t('一个实例服务多个租户，行级数据隔离，支持配额、到期时间与代管切换。'),
  },
  {
    icon: <AlertOutlined />,
    title: t('告警中心'),
    description: t('按设备数据、事件或状态配置告警规则，通知支持企业微信、钉钉、飞书与 Webhook。'),
  },
  {
    icon: <ThunderboltOutlined />,
    title: t('规则引擎'),
    description: t('基于 eKuiper 的流式规则，统一管理规则创建、启停与运行状态。'),
  },
  {
    icon: <ApartmentOutlined />,
    title: t('场景联动'),
    description: t('设备数据、事件与时间条件触发，自动执行设备控制动作并记录执行日志。'),
  },
  {
    icon: <DashboardOutlined />,
    title: t('可视化看板'),
    description: t('拖拽式看板编辑器，内置指标卡、趋势图、排行榜、表格等组件。'),
  },
  {
    icon: <HddOutlined />,
    title: t('数据存储'),
    description: t('元数据使用 SQLite / MySQL，时序数据默认 DuckDB / DuckLake，也支持多种数据层。'),
  },
  {
    icon: <SettingOutlined />,
    title: t('设备控制'),
    description: t('支持驱动服务属性写入，也支持参考系统网关的属性控制与网关重启。'),
  },
  {
    icon: <SafetyOutlined />,
    title: t('系统管理'),
    description: t('用户、角色、菜单、部门与审查日志，接口级 RBAC 权限控制。'),
  },
  {
    icon: <GlobalOutlined />,
    title: t('中英双语'),
    description: t('管理端、接口文案与告警通知均支持中文与 English 切换。'),
  },
]

const STEPS = [
  { title: t('部署服务'), description: t('使用 Docker Compose 一键启动平台、MQTT Broker 与规则引擎。') },
  { title: t('创建产品'), description: t('按业务定义物模型，或直接从内置行业模板生成产品与告警。') },
  { title: t('接入设备'), description: t('创建设备或工业网关，使用平台分配的 MQTT 凭据连接并上报数据。') },
  { title: t('配置监控'), description: t('设置告警规则、场景联动与可视化看板，持续观察设备运行情况。') },
]

// 系统设置里未改动时的默认系统名称；此时标题只显示品牌名
const DEFAULT_SITE_NAME = 'iotserver 物联网平台'

const Documents: React.FC = () => {
  const { site } = useBrand()
  // 标题直接用用户配置的系统名称
  const siteName = (site.name || '').trim()
  const title = !siteName || siteName === DEFAULT_SITE_NAME ? 'iotserver' : siteName

  return (
    <div className="product-intro">
      <section className="product-hero">
        <h1>{title}</h1>
        <div className="product-hero__desc">
          {t('从设备接入、物模型、工业网关补传，到告警、场景联动与可视化看板，一套服务即可完成部署，适合设备接入、数据采集与运行监控等场景。')}
        </div>
      </section>

      <section className="product-section">
        <h2>{t('核心能力')}</h2>
        <Row gutter={[16, 16]}>
          {FEATURES.map(feature => (
            <Col xs={24} md={12} lg={8} key={feature.title}>
              <Card className="product-feature" size="small">
                <div className="product-feature__icon">{feature.icon}</div>
                <h3>{feature.title}</h3>
                <p>{feature.description}</p>
              </Card>
            </Col>
          ))}
        </Row>
      </section>

      <section className="product-section">
        <h2>{t('快速了解')}</h2>
        <Row gutter={[16, 16]}>
          {STEPS.map((step, index) => (
            <Col xs={24} md={12} lg={6} key={step.title}>
              <Card className="product-step" size="small">
                <div className="product-step__index">{index + 1}</div>
                <h3>{step.title}</h3>
                <p>{step.description}</p>
              </Card>
            </Col>
          ))}
        </Row>
      </section>
    </div>
  )
}

export default Documents
