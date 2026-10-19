import { Button, Drawer, Form, Input, InputNumber, message, Select, Space } from 'antd'
import { CloseOutlined } from '@ant-design/icons'
import React, { forwardRef, useCallback, useImperativeHandle, useState } from 'react'
import { addResource, editResource, TDengineResource } from '@/api'
import { t } from '@/i18n'

const { useForm } = Form

export interface AddTDengineModalProps {
  onFinish: () => void
}
export interface AddTDengineModalRef {
  open: (record?: TDengineResource) => void
}

export const AddTDengineModal = forwardRef<AddTDengineModalRef, AddTDengineModalProps>(({ onFinish }, ref) => {
  const [form] = useForm()
  const [record, setRecord] = useState<TDengineResource>()
  const [submitting, setSubmitting] = useState<boolean>(false)
  const [isModalOpen, setIsModalOpen] = useState<boolean>(false)
  const [sTable, setSTable] = useState('')

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
      title={record ? t('修改实例') : t('添加实例')}
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
                  values.type = 'TDengine'
                  console.log('values', values)
                  setSubmitting(true)
                  if (record?.id) {
                    values.id = record.id
                    editResource(values)
                      .then(resp => {
                        if (resp.success) {
                          message.success(t('修改成功'))
                          handleClose()
                          onFinish()
                        } else {
                          message.error(resp.errorMsg)
                        }
                      }).finally(() => {
                        setSubmitting(false)
                      })
                  } else {
                    addResource(values)
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
      <Form
        form={form}
        layout="vertical"
        initialValues={{
          option: {
            host: 'localhost',
            user: 'root',
            password: 'taosdata',
            provideTs: false,
          },
        }}
        onValuesChange={(changedValues) => {
          // console.log('changedValues', changedValues)
          setSTable(changedValues.option?.sTable)
        }}
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
          label="Host"
          name={['option', 'host']}
        >
          <Input placeholder={t('请输入')}></Input>
        </Form.Item>
        <Form.Item
          label="Port"
          name={['option', 'port']}
          rules={[{ required: true, message: t('不能为空') }]}
        >
          <InputNumber placeholder={t('请输入')} style={{ width: '100%' }}/>
        </Form.Item>
        <Form.Item
          label="User"
          name={['option', 'user']}
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
          label="Database"
          name={['option', 'database']}
          rules={[{ required: true, message: t('不能为空') }]}
        >
          <Input placeholder={t('请输入')}></Input>
        </Form.Item>
        <Form.Item
          label="Table"
          name={['option', 'table']}
          rules={[{ required: true, message: t('不能为空') }]}
        >
          <Input placeholder={t('请输入')}></Input>
        </Form.Item>
        <Form.Item
          label="Fields"
          name={['option', 'fields']}
        >
          <Input placeholder={t('请输入')}></Input>
        </Form.Item>
        <Form.Item
          label="ProvideTs"
          name={['option', 'provideTs']}
        >
          <Select
            placeholder={t('请选择')}
            options={[
              { value: true, label: 'true' },
              { value: false, label: 'false' },
            ]}
          />
        </Form.Item>
        <Form.Item
          label="TsFieldName"
          name={['option', 'tsFieldName']}
          rules={[{ required: true, message: t('不能为空') }]}
        >
          <Input placeholder={t('请输入')}></Input>
        </Form.Item>
        <Form.Item
          label="STable"
          name={['option', 'sTable']}
        >
          <Input placeholder={t('请输入')}></Input>
        </Form.Item>
        <Form.Item
          label="TagFields"
          name={['option', 'tagFields']}
          rules={[{ required: !!sTable, message: t('不能为空') }]}
        >
          <Input placeholder={t('请输入')}></Input>
        </Form.Item>
        <Form.Item
          label="TableDataField"
          name={['option', 'tableDataField']}
        >
          <Input placeholder={t('请输入')}></Input>
        </Form.Item>
      </Form>
    </Drawer>
  )
})

export default AddTDengineModal
