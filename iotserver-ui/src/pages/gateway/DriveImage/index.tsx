import React from 'react'
import { Tabs, Card } from 'antd'
import CustomDrive from './CustomDrive'
import DriveMarket from './DriveMarket'
import './style.less'
import { t } from '@/i18n'

export interface DriveImageProps {

}

export interface ProductType {
  id: string
  product_id: string
  name: string
  node_type: string
  platform: string
  created_at: number
  category_name: string
}

// const NodeTypeMap: {[key: string]: string} = {
//   0: '未知类型',
//   1: '网关',
//   2: '直连设备',
//   3: '网管子设备',
// }

const DriveImage: React.FC<DriveImageProps> = () => {
  return (
    <>
      <Card bodyStyle={{ padding: 0 }}>
        <Tabs
          className="drive-image mt20"
          type="card"
          items={[
            {
              key: '1',
              label: t('自定义'),
              children: <CustomDrive></CustomDrive>,
            },
            {
              key: '2',
              label: t('驱动市场'),
              children: <DriveMarket></DriveMarket>,
            },
          ]}
        />
      </Card>
    </>
  )
}

export default DriveImage
