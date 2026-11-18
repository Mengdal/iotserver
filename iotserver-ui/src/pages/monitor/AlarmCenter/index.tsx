import React, { useEffect } from 'react'
import { Tabs, Card } from 'antd'
import { useLocation, useNavigate } from 'react-router-dom'
import AlarmList from './AlarmList'
import AlarmRules from './AlarmRules'
import './style.less'
import { t } from '@/i18n'
// import { ReloadOutlined } from '@ant-design/icons'

export interface AlarmCenterProps {

}

const TAB_LIST = 'list'
const TAB_RULES = 'rules'

const AlarmCenterList: React.FC<AlarmCenterProps> = () => {
  const location = useLocation()
  const navigate = useNavigate()
  const hash = location.hash.slice(1)
  // 页签直接由路由 hash 推导，不再维护第二份 state：
  // 顶栏角标跳转（清空 hash）会自动回到列表，点页签也不会和 effect 打架。
  const activeKey = hash === TAB_RULES || hash === '2' ? TAB_RULES : TAB_LIST

  // 旧链接的 #1/#2 归一化为语义化 hash；replace 避免留下额外历史记录。
  useEffect(() => {
    if (hash !== '1' && hash !== '2') return
    navigate({
      pathname: location.pathname,
      search: location.search,
      hash: hash === '2' ? TAB_RULES : TAB_LIST,
    }, { replace: true })
  }, [hash, location.pathname, location.search, navigate])

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
              key: TAB_LIST,
              label: t('告警列表'),
              children: <AlarmList></AlarmList>,
            },
            {
              key: TAB_RULES,
              label: t('告警规则'),
              children: <AlarmRules></AlarmRules>,
            },
          ]}
          onTabClick={(key) => {
            if (key === activeKey) return
            navigate({
              pathname: location.pathname,
              search: location.search,
              hash: key,
            })
          }}
        />
      </Card>
    </>
  )
}

export default AlarmCenterList
