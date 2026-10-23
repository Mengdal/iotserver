import React from 'react'
import { tt } from '@/i18n'
import { Tabs, Card } from 'antd'
import HttpPush from './HttpPush1'
import MessageMQTT from './MessageMQTT'
import MessageKafka from './MessageKafka'
import InfluxDB from './InfluxDB'
import { getTypeResource } from '@/api'
import useRequest from '@/hooks/useRequest'
// import Redis from './Redis'

export interface ResourceManageProps {

}

enum typeEnum {
  HTTP推送 = 'HTTP推送',
  消息对队列MQTT = '消息对队列MQTT',
  消息队列Kafka = '消息队列Kafka',
  InfluxDB = 'InfluxDB',
}

const typeComponent = {
  [typeEnum.HTTP推送]: <HttpPush/>,
  [typeEnum.消息对队列MQTT]: <MessageMQTT/>,
  [typeEnum.消息队列Kafka]: <MessageKafka/>,
  [typeEnum.InfluxDB]: <InfluxDB/>,
}

const ResourceManage: React.FC<ResourceManageProps> = () => {
  const typeResource = useRequest(getTypeResource)
  // console.log('typeResource', typeResource)
  return (
    <>
      <Card bodyStyle={{ padding: 0 }}>
        <Tabs
          className="drive-image mt20"
          type="card"
          items={(typeResource.data || []).map((label: typeEnum, key: number) => ({
            key,
            // 类型来自后端（HTTP推送 / 消息队列Kafka 等），词条里有就翻
            label: tt(label),
            children: typeComponent[label],
          }))}
        />
      </Card>
    </>
  )
}

export default ResourceManage
