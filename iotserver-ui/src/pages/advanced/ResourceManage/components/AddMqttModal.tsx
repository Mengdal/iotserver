import { Form, Input, message, Select } from 'antd'
import React, { forwardRef, useCallback, useImperativeHandle, useState } from 'react'
import { FormDrawer } from '@/components/FormDrawer'
import { addResource, editResource, MqttResource } from '@/api'
import { t } from '@/i18n'

const { useForm } = Form

export interface AddMqttModalProps {
  onFinish: () => void
}
export interface AddMqttModalRef {
  open: (record?: MqttResource) => void
}

const handleChange = (value: string) => {
  console.log(`selected ${value}`)
}

export const AddMqttModal = forwardRef<AddMqttModalRef, AddMqttModalProps>(({ onFinish }, ref) => {
  const [form] = useForm()
  const [record, setRecord] = useState<MqttResource>()
  const [isModalOpen, setIsModalOpen] = useState<boolean>(false)

  console.log('record', record)

  const handleClose = useCallback(() => {
    form.resetFields()
    setIsModalOpen(false)
  }, [form])

  const handleSubmit = () => form.validateFields().then(values => {
    values.type = '消息对队列MQTT'
    const request = record?.id
      ? editResource({ ...values, id: record.id })
      : addResource(values)
    return request.then(resp => {
      if (resp.success) {
        message.success(t(record?.id ? '修改成功' : '添加成功'))
        handleClose()
        onFinish()
      } else {
        message.error(resp.errorMsg)
      }
    })
  })

  useImperativeHandle(ref, () => {
    return {
      open (record) {
        setRecord(record)
        setIsModalOpen(true)
        if (record) {
          form.setFieldsValue(record)
        }
      },
    }
  }, [form])

  return (
    <FormDrawer
      title={record ? t('修改实例') : t('添加实例')}
      open={isModalOpen}
      onClose={handleClose}
      onSubmit={handleSubmit}
    >
      <Form
        form={form}
        layout="vertical"
      >
        <Form.Item
          label={t('实例名称')}
          name="name"
          rules={[
            {
              required: true,
              validator (rule, value, callback) {
                if (!value) {
                  callback(t('请输入实例名称'))
                } else if (/^[a-zA-Z\u4e00-\u9fa5][a-zA-Z0-9\u4e00-\u9fa5_-]{0,31}$/.test(value)) {
                  callback()
                } else {
                  callback(t('1-32位字符，支持中文、英文、数字及特殊字符_-，必须以英文或中文字符开头'))
                }
              },
            },
          ]}
        >
          <Input placeholder={t('1-32位字符，支持中文、英文、数字及特殊字符_-，必须以英文或中文字符开头')}></Input>
        </Form.Item>
        <Form.Item
          label="MQTT broker address"
          name={['option', 'server']}
          rules={[{ required: true, message: t('不能为空') }]}
        >
          <Input placeholder={t('请输入')}></Input>
        </Form.Item>
        <Form.Item
          label="MQTT topic"
          name={['option', 'topic']}
          rules={[{ required: true, message: t('不能为空') }]}
        >
          <Input placeholder={t('请输入')}></Input>
        </Form.Item>
        <Form.Item
          label="MQTT client"
          name={['option', 'clientId']}
        >
          <Input placeholder={t('请输入')}></Input>
        </Form.Item>
        <Form.Item
          label="MQTT protocol version"
          name={['option', 'protocolVersion']}
        >
          <Select
            placeholder={t('请选择')}
            onChange={handleChange}
            options={[
              { value: '3.1', label: '3.1' },
              { value: '3.1.1', label: '3.1.1' },
            ]}
          />
        </Form.Item>
        <Form.Item
          label="QoS"
          name={['option', 'qos']}
        >
          <Select
            placeholder={t('请选择')}
            onChange={handleChange}
            options={[
              { value: 0, label: '0' },
              { value: 1, label: '1' },
              { value: 2, label: '2' },
            ]}
          />
        </Form.Item>
        <Form.Item
          label="Username"
          name={['option', 'username']}
        >
          <Input placeholder={t('请输入')}></Input>
        </Form.Item>
        <Form.Item
          label="Password"
          name={['option', 'password']}
        >
          <Input placeholder={t('请输入')}></Input>
        </Form.Item>
      </Form>
    </FormDrawer>
  )
})
