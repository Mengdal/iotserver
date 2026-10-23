import React, { useImperativeHandle, forwardRef, useState, useCallback } from 'react'
import { Form, Drawer, Input, Radio, Select, message, Button, Space, Typography, Upload } from 'antd'
import { CloseOutlined } from '@ant-design/icons'
import { addDevice, addDevices, downloadTemplate, useGetProductList } from '@/api'
import { DeviceServers } from '@/api/device-servers'
import { downloadXlsx } from '@/utils/download'
import { UploadChangeParam, UploadFile } from 'antd/es/upload'
import type { UploadProps } from 'antd'
import { getToken } from '@/utils/auth'
import { t } from '@/i18n'

const { useForm } = Form
const { TextArea } = Input
const { Text, Link } = Typography

export interface AddModalProps {
  deviceServersOptions: DeviceServers[]
  onFinish: () => void
}

export interface AddModalRef {
  open: () => void
}

export const AddModal = forwardRef<AddModalRef, AddModalProps>(({ deviceServersOptions, onFinish }, ref) => {
  const [form] = useForm()
  const [submitting, setSubmitting] = useState<boolean>(false)
  const [isModalOpen, setIsModalOpen] = useState<boolean>(false)

  const { data: productListOptions } = useGetProductList({ isAll: true, platform: '本地' })

  useImperativeHandle(ref, () => {
    return {
      open () {
        setIsModalOpen(true)
      },
    }
  }, [form])

  const handleClose = useCallback(() => {
    form.resetFields()
    setIsModalOpen(false)
  }, [form])

  const props: UploadProps = {
    name: 'file',
    accept: '.xlsx',
    action: '/api/v1/device/upload-validated',
    headers: {
      'x-token': getToken(),
    },
  }

  return (
    <Drawer
      title={t('创建设备')}
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
                  if (values.type === '1') {
                    setSubmitting(true)
                    addDevice({
                      name: values.name,
                      product_id: values.product_id,
                      driver_instance_id: values.driver_instance_id,
                      description: values.description,
                    })
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
                  } else {
                    const formdata = new FormData()
                    const fileInfo: UploadChangeParam<UploadFile<any>> = values.file
                    formdata.append('file', fileInfo.file.originFileObj as any)
                    console.log('values', values)

                    setSubmitting(true)
                    addDevices(formdata, {
                      product_id: values.product_id,
                      driver_instance_id: values.driver_instance_id,
                    }).then(resp => {
                      if (resp.success) {
                        message.success(t('批量添加成功'))
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
          label={t('添加设备方式')}
          name="type"
          initialValue={'1'}
          rules={[{ required: true, message: t('请选择添加设备方式') }]}
        >
          <Radio.Group
            optionType="button"
            buttonStyle="solid"
            options={[
              { label: t('单个设备'), value: '1' },
              { label: t('批量添加'), value: '2' },
            ]}
          />
        </Form.Item>

        <Form.Item
          noStyle
          shouldUpdate={(prevValues, nextValues) => {
            return prevValues.type !== nextValues.type
          }}
        >
          {({ getFieldValue }) => {
            return getFieldValue('type') === '1'
              ? (
                <>
                  <Form.Item
                    label={t('设备名称')}
                    name="name"
                    rules={[{ required: true, message: t('请输入设备名称') }]}
                  >
                    <Input placeholder={t('请输入设备名称')}></Input>
                  </Form.Item>
                  <Form.Item
                    label={t('所属产品')}
                    name="product_id"
                    rules={[{ required: true, message: t('请选择所属产品') }]}
                  >
                    <Select
                      placeholder={t('请选择所属产品')}
                      options={productListOptions.map(i => ({ label: i.name, value: i.id }))}
                    ></Select>
                  </Form.Item>
                  <Form.Item
                    label={t('关联驱动')}
                    name="driver_instance_id"
                  >
                    <Select
                      placeholder={t('请选择关联驱动')}
                      options={deviceServersOptions.map(i => ({ label: i.name, value: i.id }))}
                    ></Select>
                  </Form.Item>
                  <Form.Item
                    label={t('设备描述')}
                    name="description"
                  >
                    <TextArea placeholder={t('请输入产品描述')} rows={4}></TextArea>
                  </Form.Item>
                </>
              )
              : (
                <>
                  <Form.Item>
                    <Text type="secondary">{t('批量添加设备可在批次列表中查询相关记录')}</Text>
                    <div>
                      <Text type="secondary" className="mr5">{t('格式.xlsx 最大2M，单次500个设备')}</Text>
                      <Link onClick={() => {
                        message.loading({ content: t('下载中, 请稍后') })
                        downloadTemplate().then(resp => {
                          const fileName = window.decodeURI(resp.headers['content-disposition']?.split('=')[1] || '')
                          downloadXlsx(resp.data, fileName)
                        })
                      }}>{t('模板下载')}</Link>
                    </div>
                  </Form.Item>
                  <Form.Item
                    label={t('上传设备表')}
                    name="file"
                    rules={[{ required: true, message: t('请上传文件') }]}
                  >
                    <Upload
                      // name="file"
                      // accept=".xlsx"
                      // action="/api/v1/device/upload-validated"
                      {...props}
                      onChange={info => {
                        console.log(info)
                        if (info.file.status === 'done') {
                          if (info.file.response?.success) {
                            message.success(t('文件上传成功'))
                          } else {
                            form.resetFields(['file'])
                            message.error(t('{{p1}}, 请重新上传', { p1: info.file.response?.errorMsg || t('系统错误') }))
                          }
                        } else if (info.file.status === 'error') {
                          message.error(t('文件上传失败'))
                        }
                      }}
                    >
                      <Link>{t('上传文件')}</Link>
                    </Upload>
                  </Form.Item>
                  <Form.Item
                    label={t('所属产品')}
                    name="product_id"
                    rules={[{ required: true, message: t('请选择所属产品') }]}
                  >
                    <Select
                      placeholder={t('请选择所属产品')}
                      options={productListOptions.map(i => ({ label: i.name, value: i.id }))}
                    ></Select>
                  </Form.Item>
                  <Form.Item
                    label={t('关联驱动')}
                    name="driver_instance_id"
                  >
                    <Select
                      placeholder={t('请选择关联驱动')}
                      options={deviceServersOptions.map(i => ({ label: i.name, value: i.id }))}
                    ></Select>
                  </Form.Item>
                </>
              )
          }}
        </Form.Item>

      </Form>
    </Drawer>
  )
})

export default AddModal
