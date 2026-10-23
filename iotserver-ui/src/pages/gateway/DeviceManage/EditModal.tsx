import React, { useImperativeHandle, forwardRef, useState, useCallback } from 'react'
import { Form, Drawer, Input, Button, Space, message, Select } from 'antd'
import { CloseOutlined } from '@ant-design/icons'
import { bindDriver, editDevices, unBindDriver, useGetDeviceServers } from '@/api'
import { DevicesType } from '@/api/devices'
import { t } from '@/i18n'

const { useForm } = Form
const { TextArea } = Input

export interface EditModalProps {
  onFinish: () => void
}

export interface EditModalRef {
  open: (record?: DevicesType) => void
}

export const EditModal = forwardRef<EditModalRef, EditModalProps>(({ onFinish }, ref) => {
  const [form] = useForm()
  const [record, setRecord] = useState<DevicesType>()
  const [submitting, setSubmitting] = useState<boolean>(false)
  const [isModalOpen, setIsModalOpen] = useState<boolean>(false)
  const { data: deviceServersOptions } = useGetDeviceServers({})

  const handleClose = useCallback(() => {
    form.resetFields()
    setIsModalOpen(false)
  }, [form])

  useImperativeHandle(ref, () => {
    return {
      open (record) {
        setRecord(record)
        setIsModalOpen(true)
        if (record) {
          form.setFieldsValue({
            ...record,
            // 驱动列表/表单统一用实例 ID；列表里的 driver_service_name 只用于展示名称
            driver_instance_id: record.drive_instance_id,
          })
        }
      },
    }
  }, [form])

  /** 保存驱动变更：先解绑旧驱动，再绑定新驱动（驱动侧依赖这两个接口的回调维护设备映射） */
  const saveDriver = async (device: DevicesType, driverInstanceId: string) => {
    const oldDriver = device.drive_instance_id || ''
    if (driverInstanceId === oldDriver) return true

    if (oldDriver) {
      const resp = await unBindDriver({ device_ids: [device.id] })
      if (!resp.success) {
        message.error(resp.errorMsg)
        return false
      }
    }
    if (driverInstanceId) {
      const resp = await bindDriver({ device_ids: [device.id], driver_instance_id: driverInstanceId })
      if (!resp.success) {
        // 绑定失败时尽量恢复到旧驱动，避免设备停在“已解绑”状态
        if (oldDriver) {
          await bindDriver({ device_ids: [device.id], driver_instance_id: oldDriver })
        }
        message.error(resp.errorMsg)
        return false
      }
    }
    return true
  }

  const handleSubmit = () => {
    form.validateFields().then(async values => {
      if (!record) return
      setSubmitting(true)
      try {
        if (!await saveDriver(record, values.driver_instance_id || '')) return
        const resp = await editDevices({
          id: record.id,
          description: values.description || '',
        })
        if (resp.success) {
          message.success(t('编辑成功'))
          handleClose()
          onFinish()
        } else {
          message.error(resp.errorMsg)
        }
      } finally {
        setSubmitting(false)
      }
    })
  }

  return (
    <Drawer
      title={t('编辑设备')}
      placement="right"
      open={isModalOpen}
      width={550}
      closable={false}
      headerStyle={{ height: 51 }}
      destroyOnClose
      extra={<CloseOutlined onClick={handleClose} />}
      onClose={handleClose}
      footer={
        <Space>
          <Button
            type="primary"
            loading={submitting}
            disabled={submitting}
            onClick={handleSubmit}
          >{t('确定')}</Button>
          <Button onClick={handleClose}>{t('取消')}</Button>
        </Space>
      }
    >
      <Form form={form} layout="vertical">
        <Form.Item
          label={t('设备名称')}
          name="name"
        >
          <Input placeholder={t('请输入设备名称')} disabled></Input>
        </Form.Item>
        <Form.Item
          label={t('设备状态')}
          name="status"
        >
          <Input placeholder={t('设备状态')} disabled></Input>
        </Form.Item>
        <Form.Item
          label={t('平台')}
          name="platform"
        >
          <Input placeholder={t('请输入协议')} disabled></Input>
        </Form.Item>
        <Form.Item
          label={t('关联驱动')}
          name="driver_instance_id"
        >
          <Select
            allowClear
            placeholder={t('请选择关联驱动')}
            options={deviceServersOptions.map(i => ({ label: i.name, value: i.id }))}
          ></Select>
        </Form.Item>
        <Form.Item
          label={t('描述')}
          name="description"
        >
          <TextArea placeholder={t('请输入描述')} rows={4}></TextArea>
        </Form.Item>
      </Form>
    </Drawer>
  )
})

export default EditModal
