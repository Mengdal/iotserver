import { CloseOutlined } from '@ant-design/icons'
import { Button, Drawer, Space } from 'antd'
import React, { useState } from 'react'
import { t } from '@/i18n'

export interface FormDrawerProps {
  open: boolean
  title?: React.ReactNode
  width?: number
  onClose: () => void
  /** 点击「确定」触发；返回 Promise 时组件自动显示 loading。 */
  onSubmit: () => void | Promise<void>
  submitting?: boolean
  okText?: string
  cancelText?: string
  destroyOnClose?: boolean
  children: React.ReactNode
}

/** 右侧抽屉表单的统一外壳：标题、关闭按钮、确定/取消和提交 loading。 */
export const FormDrawer: React.FC<FormDrawerProps> = ({
  open,
  title,
  width = 550,
  onClose,
  onSubmit,
  submitting = false,
  okText,
  cancelText,
  destroyOnClose = true,
  children,
}) => {
  const [innerSubmitting, setInnerSubmitting] = useState(false)
  const loading = submitting || innerSubmitting

  const handleSubmit = () => {
    const result = onSubmit()
    if (!result || typeof (result as Promise<void>).then !== 'function') {
      return
    }
    setInnerSubmitting(true)
    Promise.resolve(result)
      .finally(() => setInnerSubmitting(false))
      .catch(() => undefined)
  }

  return (
    <Drawer
      title={title}
      placement="right"
      open={open}
      width={width}
      closable={false}
      headerStyle={{ height: 51 }}
      destroyOnClose={destroyOnClose}
      extra={<CloseOutlined onClick={onClose} />}
      onClose={onClose}
      footer={
        <Space>
          <Button type="primary" loading={loading} disabled={loading} onClick={handleSubmit}>
            {okText ?? t('确定')}
          </Button>
          <Button onClick={onClose}>{cancelText ?? t('取消')}</Button>
        </Space>
      }
    >
      {children}
    </Drawer>
  )
}
