import React, { useMemo } from 'react'
import { Tabs, Card } from 'antd'
import AlarmList from './AlarmList'
import AlarmRules from './AlarmRules'
import './style.less'
import { t } from '@/i18n'
// import { ReloadOutlined } from '@ant-design/icons'

export interface AlarmCenterProps {

}

const AlarmCenterList: React.FC<AlarmCenterProps> = () => {
  const activeKey = useMemo(() => {
    const hash = window.location.hash
    if (hash) {
      return hash.slice(1, 2)
    }
    return '1'
  }, [])

  return (
    <>
      <Card bodyStyle={{ padding: 0 }}>
        <Tabs
          className="drive-image mt20"
          type="card"
          defaultActiveKey={activeKey}
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
            window.location.hash = key
          }}
        />
      </Card>
    </>
  )
}

export default AlarmCenterList
