import { Button, Drawer, Form, Input, message, Radio, Select, Space } from 'antd'
import { CloseOutlined } from '@ant-design/icons'
import React, { forwardRef, useCallback, useImperativeHandle, useState } from 'react'
import { addRule } from '@/api'
import { DeviceLibraries } from '@/api/device-libraries'
import { t } from '@/i18n'

const { useForm } = Form
const { TextArea } = Input

export interface AddRuleModalProps {
  onFinish: () => void
}
export interface AddRuleModalRef {
  open: (record?: DeviceLibraries) => void
}

const handleChange = (value: string) => {
  console.log(`selected ${value}`)
}

export const AddRuleModal = forwardRef<AddRuleModalRef, AddRuleModalProps>(({ onFinish }, ref) => {
  const [form] = useForm()
  const [record, setRecord] = useState<DeviceLibraries>()
  const [submitting, setSubmitting] = useState<boolean>(false)
  const [isModalOpen, setIsModalOpen] = useState<boolean>(false)

  // const dockerConfigModalRef = useRef<DockerConfigModalRef>(null)

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
          form.setFieldsValue(record)
        }
      },
    }
  }, [form])

  return (
    <Drawer
      title={record ? t('编辑规则') : t('添加规则')}
      placement="right"
      open={isModalOpen}
      width={550}
      closable={false}
      maskClosable={false}
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
            onClick={() => {
              form.validateFields()
                .then(values => {
                  setSubmitting(true)
                  addRule(values)
                    .then(resp => {
                      if (resp.success) {
                        message.success(t('添加成功'))
                        handleClose()
                        onFinish()
                      } else {
                        message.error(resp.errorMsg)
                      }
                    }).finally(() => {
                      setSubmitting(false)
                    })
                })
            }}
          >{t('确定')}</Button>
          <Button onClick={handleClose}>{t('取消')}</Button>
        </Space>
      }
    >
      <Form form={form} layout="vertical">
        <Form.Item
          label={t('规则名称')}
          name="name"
          rules={[
            {
              required: true,
              validator (rule, value, callback) {
                if (!value) {
                  callback(t('请输入规则名称'))
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
          label={t('告警类型')}
          name="type"
        >
          <Radio checked>{t('设备告警')}</Radio>
        </Form.Item>
        <Form.Item
          label={t('告警级别')}
          name="alert_level"
          rules={[{ required: true, message: t('请选择告警级别') }]}
        >
          <Select
            placeholder={t('请选择')}
            onChange={handleChange}
            options={[
              { value: '紧急', label: t('紧急') },
              { value: '重要', label: t('重要') },
              { value: '次要', label: t('次要') },
              { value: '提示', label: t('提示') },
            ]}
          />
        </Form.Item>
        <Form.Item
          label={t('规则描述')}
          name="description"
        >
          <TextArea
            showCount
            maxLength={100}
            placeholder={t('请输入描述信息')}
            rows={4}></TextArea>
        </Form.Item>
      </Form>
    </Drawer>
  )
})

export default AddRuleModal
