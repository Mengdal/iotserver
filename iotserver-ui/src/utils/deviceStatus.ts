import { t } from '@/i18n'

/**
 * 设备状态：后端存稳定代码（online/offline/inactive/disabled/unknown），
 * 展示文案在这里按当前语言翻译——切语言时数据库与接口参数都不用动。
 */
export const DEVICE_STATUS_COLORS: Record<string, string> = {
  online: 'green',
  offline: 'gray',
  disabled: 'red',
  inactive: 'orange',
  unknown: 'gray',
}

export function deviceStatusLabel (status?: string | null): string {
  switch (status) {
    case 'online':
      return t('在线')
    case 'offline':
      return t('离线')
    case 'disabled':
      return t('禁用')
    case 'inactive':
      return t('未激活')
    case 'unknown':
      return t('未知')
    default:
      // 兼容历史数据或后端新增的状态：没有映射就原样显示，至少不会丢信息
      return status || t('未知')
  }
}
