import React, { useCallback, useEffect, useState } from 'react'
import Overview from './components/Overview'
import DeviceMessages from './components/DeviceMessages'
import DeviceMessageRank from './components/DeviceMessageRank'
import { HomePageInfo } from './types'
import { getHomePageInfo } from '@/api'
import './index.less'
import { Col, Row } from 'antd'

interface DashboardProps {
}

/**
 * 首页仪表盘：参考 demo.winc-link.com/home/dashboard 的排版，只保留三块
 *
 *   第一行  平台概述（整宽）
 *   第二行  设备消息总数（宽） | 设备消息量排名
 *
 * 数据来源都是平台自己的接口（/api/v1/home-page 的 pageInfo 与 msg_gather）；
 * 「设备消息量排名」需要的 topMessage 接口本地后端还没有，先按 demo 给空状态。
 * Shaper 看板保留在「数据看板」整页与「运维监控-系统监控」两处（见 shaper/README.md）。
 */
const Dashboard: React.FC<DashboardProps> = () => {
  const [homePageInfo, setHomePageInfo] = useState<HomePageInfo>()

  const loadHomePageInfo = useCallback(() => {
    getHomePageInfo<HomePageInfo>()
      .then((resp) => {
        if (resp.success) {
          setHomePageInfo({
            ...resp.result,
            msg_gather: (resp.result.msg_gather || []).reverse(),
          })
        }
      }).catch((err: any) => {
        console.log(err)
      })
  }, [])

  useEffect(() => {
    loadHomePageInfo()
  }, [loadHomePageInfo])

  return (<div className="dashboard">
    <Row gutter={[20, 20]}>
      <Col span={24}>
        <Overview pageInfo={homePageInfo?.pageInfo} />
      </Col>
    </Row>
    <Row gutter={[20, 20]} className="mt20">
      <Col span={24} xl={16} xxl={16}>
        <DeviceMessages msgGatherList={homePageInfo?.msg_gather || []} />
      </Col>
      <Col span={24} xl={8} xxl={8}>
        <DeviceMessageRank />
      </Col>
    </Row>
  </div>)
}

export default Dashboard
