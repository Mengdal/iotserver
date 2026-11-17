import React, { useEffect, useState } from 'react'
import { Tabs, Card } from 'antd'
import { useLocation } from 'react-router-dom'
import AlarmList from './AlarmList'
import AlarmRules from './AlarmRules'
import './style.less'
import { t } from '@/i18n'
// import { ReloadOutlined } from '@ant-design/icons'

export interface AlarmCenterProps {

}

const AlarmCenterList: React.FC<AlarmCenterProps> = () => {
  const location = useLocation()
  const [activeKey, setActiveKey] = useState(() => window.location.hash.slice(1, 2) || '1')

  // 顶栏角标跳到本页时带 ?status=未处理；如果用户正停在「告警规则」tab，
  // 需要切回「告警列表」，否则列表组件未挂载、点击看起来没有反应。
  useEffect(() => {
    if (!new URLSearchParams(location.search).get('status')) return
    setActiveKey('1')
  }, [location.key, location.search])

  return (
    <>
      <Card bodyStyle={{ padding: 0 }}>
        <Tabs
          className="drive-image mt20"
          type="card"
          activeKey={activeKey}
          // tabBarExtraContent={<Button className="ml10"
          //   onClick={() => {
          //     window.location.reload()
          //   }}
          // ><ReloadOutlined />刷新</Button>}
          items={[
            {
              key: '1',
              label: t('告警列表'),
              children: <AlarmList></AlarmList>,
            },
            {
              key: '2',
              label: t('告警规则'),
              children: <AlarmRules></AlarmRules>,
            },
          ]}
          onTabClick={(key) => {
            setActiveKey(key)
            window.location.hash = key
          }}
        />
      </Card>
    </>
  )
}

export default AlarmCenterList
