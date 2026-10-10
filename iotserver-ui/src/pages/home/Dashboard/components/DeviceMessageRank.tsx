import { Card, Empty, Typography } from 'antd'
import React from 'react'

const { Title } = Typography

/**
 * 设备消息量排名。
 *
 * 参考 demo（demo.winc-link.com）首页的「设备消息量排名」卡片：它调的是
 * `GET /api/v1/devices/topMessage?number=5`，本地这套后端**还没有这个接口**
 * （本地 msg_gather 只存「按天的消息总量」，没有按设备的维度，见
 *  iotserver/internal/iotserver/core/application/deviceapp/deviceapp.go:DevicesReportMsgGather）。
 * 所以这里先按 demo 的样子给出空状态；等后端补上 topMessage 接口后，
 * 把 <Empty /> 换成排行榜（柱状/列表）即可，卡片结构与标题都不用动。
 */
const DeviceMessageRank: React.FC = () => (
  <Card
    title={<Title level={3}>设备消息量排名</Title>}
    extra="Top 5"
    className="device-message-rank"
  >
    <div className="device-message-rank__empty">
      <Empty
        image={Empty.PRESENTED_IMAGE_SIMPLE}
        description="暂无设备消息排名数据"
      />
    </div>
  </Card>
)

export default DeviceMessageRank
