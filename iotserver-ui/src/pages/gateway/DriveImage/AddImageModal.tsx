import React, { useImperativeHandle, forwardRef, useState, useCallback } from 'react'
import { Form, Drawer, Input, Button, Space, message } from 'antd'
import { CloseOutlined } from '@ant-design/icons'
import { addDockerConfigs, editDockerConfigs } from '@/api'
import { DockerConfigs } from './DockerConfigModal'
import { t } from '@/i18n'

const { useForm } = Form

export interface AddImageModalProps {
  onFinish: () => void
}

export interface AddImageModalRef {
  open: (record?: DockerConfigs) => void
}

export const AddImageModal = forwardRef<AddImageModalRef, AddImageModalProps>(({ onFinish }, ref) => {
  const [form] = useForm()
  const [record, setRecord] = useState<DockerConfigs>()
  const [submitting, setSubmitting] = useState<boolean>(false)
  const [isModalOpen, setIsModalOpen] = useState<boolean>(false)

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

  const handleClose = useCallback(() => {
    form.resetFields()
    setIsModalOpen(false)
  }, [form])

  return (
    <Drawer
      title={record ? t('镜像仓库编辑') : t('新增镜像仓库')}
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
            onClick={() => {
              form.validateFields()
                .then(values => {
                  setSubmitting(true)

                  if (record?.id) {
                    editDockerConfigs(record.id, { id: record.id, ...values })
                      .then(resp => {
                        if (resp.success) {
                          message.success(t('编辑成功'))
                          handleClose()
                          onFinish()
                        } else {
                          message.error(resp.errorMsg)
                        }
                      }).finally(() => {
                        setSubmitting(false)
                      })
                  } else {
                    addDockerConfigs(values)
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
                  }
                })
            }}
          >{t('确定')}</Button>
          <Button onClick={handleClose}>{t('取消')}</Button>
        </Space>
      }
    >
      <Form form={form} layout="vertical">
        <Form.Item
          label={t('镜像地址')}
          name="address"
          rules={[{ required: true, message: t('请输入镜像地址') }]}
        >
          <Input placeholder={t('请输入镜像地址')}></Input>
        </Form.Item>
        <Form.Item
          label={t('账号')}
          name="account"
        >
          <Input placeholder={t('请输入账号')}></Input>
        </Form.Item>
        <Form.Item
          label={t('密码')}
          name="password"
        >
          <Input.Password
            placeholder={t('请输入密码')}
          ></Input.Password>
        </Form.Item>
      </Form>
    </Drawer>
  )
})

export default AddImageModal
