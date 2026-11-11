import { Form, Input, message } from 'antd'
import React, { forwardRef, useCallback, useImperativeHandle, useState } from 'react'
import { addResource, editResource, InfluxDBResource } from '@/api'
import { FormDrawer } from '@/components/FormDrawer'
import { t } from '@/i18n'

const { useForm } = Form

export interface AddInfluxDBModalProps {
  onFinish: () => void
}
export interface AddInfluxDBModalRef {
  open: (record?: InfluxDBResource) => void
}

export const AddInfluxDBModal = forwardRef<AddInfluxDBModalRef, AddInfluxDBModalProps>(({ onFinish }, ref) => {
  const [form] = useForm()
  const [record, setRecord] = useState<InfluxDBResource>()
  const [isModalOpen, setIsModalOpen] = useState<boolean>(false)

  const handleClose = useCallback(() => {
    form.resetFields()
    setIsModalOpen(false)
  }, [form])

  const handleSubmit = () => form.validateFields().then(values => {
    values.type = 'InfluxDB'
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
          label="Addr"
          name={['option', 'addr']}
          rules={[{ required: true, message: t('不能为空') }]}
        >
          <Input placeholder={t('请输入')}></Input>
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
        <Form.Item
          label="Measurement"
          name={['option', 'measurement']}
          rules={[{ required: true, message: t('不能为空') }]}
        >
          <Input placeholder={t('请输入')}></Input>
        </Form.Item>
        <Form.Item
          label="Databasename"
          name={['option', 'databasename']}
          rules={[{ required: true, message: t('不能为空') }]}
        >
          <Input placeholder={t('请输入')}></Input>
        </Form.Item>
        <Form.Item
          label="Tagkey"
          name={['option', 'tagkey']}
          rules={[{ required: true, message: t('不能为空') }]}
        >
          <Input placeholder={t('请输入')}></Input>
        </Form.Item>
        <Form.Item
          label="Tagvalue"
          name={['option', 'tagvalue']}
          rules={[{ required: true, message: t('不能为空') }]}
        >
          <Input placeholder={t('请输入')}></Input>
        </Form.Item>
        {/* <Form.Item
          label="Fields"
          name={['option', 'fields']}
          rules={[{ required: true, message: '不能为空' }]}
        >
          <Input placeholder="请输入"></Input>
        </Form.Item> */}
      </Form>
    </FormDrawer>
  )
})
