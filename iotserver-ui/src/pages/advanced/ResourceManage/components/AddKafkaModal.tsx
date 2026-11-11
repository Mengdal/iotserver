import { Form, Input, message, Select } from 'antd'
import React, { forwardRef, useCallback, useImperativeHandle, useState } from 'react'
import { addResource, editResource, KafkaResource } from '@/api'
import { FormDrawer } from '@/components/FormDrawer'
import { t } from '@/i18n'

const { useForm } = Form

export interface AddKafkaModalProps {
  onFinish: () => void
}
export interface AddKafkaModalRef {
  open: (record?: KafkaResource) => void
}

export const AddKafkaModal = forwardRef<AddKafkaModalRef, AddKafkaModalProps>(({ onFinish }, ref) => {
  const [form] = useForm()
  const [record, setRecord] = useState<KafkaResource>()
  const [isModalOpen, setIsModalOpen] = useState<boolean>(false)

  const handleClose = useCallback(() => {
    form.resetFields()
    setIsModalOpen(false)
  }, [form])

  const handleSubmit = () => form.validateFields().then(values => {
    values.type = '消息队列Kafka'
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
          label="Kafka brokers"
          name={['option', 'brokers']}
          rules={[{ required: true, message: t('不能为空') }]}
        >
          <Input placeholder={t('请输入')}></Input>
        </Form.Item>
        <Form.Item
          label="Topic"
          name={['option', 'topic']}
          rules={[{ required: true, message: t('不能为空') }]}
        >
          <Input placeholder={t('请输入')}></Input>
        </Form.Item>
        <Form.Item
          label="SaslAuthType"
          name={['option', 'saslAuthType']}
        >
          <Select
            placeholder={t('请选择')}
            // onChange={handleChange}
            options={[
              { value: 'none', label: 'none' },
              { value: 'plain', label: 'plain' },
              { value: 'scram', label: 'scram' },
            ]}
          />
        </Form.Item>
        <Form.Item
          noStyle
          shouldUpdate={(prevValues, nextValues) => {
            return prevValues?.option?.saslAuthType !== nextValues?.option?.saslAuthType
          }}
        >
          {({ getFieldValue }) => {
            // console.log('shouldUpdate123', getFieldValue(['sub_rule', 'option', 'value_type']))
            return getFieldValue(['option', 'saslAuthType']) !== 'none' && (<><Form.Item
              label="SaslUserName"
              name={['option', 'saslUserName']}
            >
              <Input placeholder={t('请输入')}></Input>
            </Form.Item><Form.Item
              label="SaslPassword"
              name={['option', 'saslPassword']}
            >
              <Input.Password placeholder={t('请输入')} />
            </Form.Item>
            </>)
          }}
        </Form.Item>
      </Form>
    </FormDrawer>
  )
})
