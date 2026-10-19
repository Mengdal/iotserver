import React from 'react'
import { BadgeProps, TagProps } from 'antd'
import { SyncOutlined, ClockCircleOutlined, CheckCircleOutlined, CheckCircleTwoTone, MinusCircleFilled } from '@ant-design/icons'
import { ServiceItemType } from '@/pages/cloud/Service'
import { t } from '@/i18n'

export const OperateStatusMap: {
  [key in ServiceItemType['operate_status']]: {
    text: string
    status: BadgeProps['status']
    icon: TagProps['icon']
    color: TagProps['color']
  }
} = {
  installing: { text: t('下载中'), status: 'processing', color: 'processing', icon: <SyncOutlined spin /> },
  default: { text: t('未下载'), status: 'default', color: 'default', icon: <ClockCircleOutlined /> },
  installed: { text: t('已下载'), status: 'success', color: 'processing', icon: <CheckCircleOutlined /> },
  uninstall: { text: t('已卸载'), status: 'warning', color: 'default', icon: <ClockCircleOutlined /> },
}

export const RunStatusMap: {
  [key: string]: {
    text: string
    status: BadgeProps['status']
  }
} = {
  1: { text: t('运行中'), status: 'success' },
  2: { text: t('停止'), status: 'default' },
  3: { text: t('启动中'), status: 'processing' },
  4: { text: t('停止中'), status: 'default' },
  5: { text: t('异常'), status: 'error' },
}

export const RuleStatusMap: {
  [key: string]: {
    text: string
    status: BadgeProps['status']
    icon: TagProps['icon']
  }
} = {
  running: { text: t('启用'), status: 'success', icon: <CheckCircleTwoTone /> },
  stopped: { text: t('禁用'), status: 'default', icon: <MinusCircleFilled /> },
}
