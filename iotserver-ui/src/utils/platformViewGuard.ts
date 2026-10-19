import { Modal } from 'antd'
import { getCurrentTenantId, isPlatformAdmin } from '@/utils/auth'
import { t } from '@/i18n'

/** 是否处于「平台视图」：平台管理员且未切换到任何租户（右上角「当前租户」= 全部租户） */
export const isPlatformView = (): boolean => isPlatformAdmin() && !getCurrentTenantId()

/**
 * 平台视图下新增的数据会落到「平台级」，不属于任何租户，租户登录后看不到。
 * 平台视图时先弹确认提示；返回 true 表示可以继续打开新增弹层（非平台视图时直接返回 true，不打扰用户）。
 */
export const confirmPlatformViewAdd = (): Promise<boolean> => {
  if (!isPlatformView()) return Promise.resolve(true)
  return new Promise<boolean>(resolve => {
    Modal.confirm({
      title: t('当前是平台视图'),
      content: t('这里创建的数据不属于任何租户，租户登录后看不到。如需归到某个租户，请先在右上角「当前租户」切换到该租户再创建。'),
      okText: t('知道了'),
      cancelText: t('取消'),
      onOk: () => resolve(true),
      onCancel: () => resolve(false),
    })
  })
}
