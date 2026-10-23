import React, { useImperativeHandle, forwardRef, useState, useCallback, useRef, useEffect } from 'react'
import { Form, Drawer, Input, Radio, Typography, Select, message, Button, Space } from 'antd'
import { CloseOutlined } from '@ant-design/icons'
import { CategorySelect, CategoryPresetOption } from './CategorySelect'
import { addProduct } from '@/api'
import { StandardFuncModal, StandardFuncModalRef } from './StandardFuncModal'
import { t } from '@/i18n'

const { useForm } = Form
const { TextArea } = Input

export interface AddModalProps {
  onFinish: () => void
}

/** 外部跳转（如方案中心）创建产品时的预选品类 */
export interface AddModalPreset {
  categoryTemplateId?: string
  categoryName?: string
}

export interface AddModalRef {
  /** 不传 preset 时行为与原来完全一致 */
  open: (preset?: AddModalPreset) => void
}

export const AddModal = forwardRef<AddModalRef, AddModalProps>(({ onFinish }, ref) => {
  const [form] = useForm()
  const [submitting, setSubmitting] = useState<boolean>(false)
  const [isModalOpen, setIsModalOpen] = useState<boolean>(false)
  /** 预选的标准品类：同时驱动表单值与选择器展示 */
  const [presetOption, setPresetOption] = useState<CategoryPresetOption>()

  const standardFuncModalRef = useRef<StandardFuncModalRef>(null)
  const categoryKeyRef = useRef('')

  useImperativeHandle(ref, () => {
    return {
      open (preset) {
        setPresetOption(preset?.categoryTemplateId
          ? {
            id: preset.categoryTemplateId,
            category_name: preset.categoryName || preset.categoryTemplateId,
          }
          : undefined)
        setIsModalOpen(true)
      },
    }
  }, [form])

  /** 抽屉挂载后再写回表单，保证「标准品类」单选框与品类选择器都被预选上 */
  useEffect(() => {
    if (!isModalOpen || !presetOption) return
    form.setFieldsValue({
      category_template_id: '0',
      category_template_id2: presetOption.id,
    })
  }, [isModalOpen, presetOption, form])

  const handleClose = useCallback(() => {
    form.resetFields()
    setPresetOption(undefined)
    setIsModalOpen(false)
  }, [form])

  return (
    <Drawer
      title={t('创建产品')}
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
                  if ('category_template_id2' in values) {
                    values.category_template_id = values.category_template_id2
                    delete values.category_template_id2
                  }

                  setSubmitting(true)
                  addProduct(values)
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
      <StandardFuncModal ref={standardFuncModalRef} />
      <Form form={form} layout="vertical">
        <Form.Item
          label={t('产品名称')}
          name="name"
          rules={[{ required: true, message: t('请输入产品名称') }]}
        >
          <Input placeholder={t('请输入产品名称')}></Input>
        </Form.Item>
        {/* <Form.Item
          label="云平台"
          name="cloud_platform"
          rules={[{ required: true, message: '请选择云平台' }]}
        >
          <Select
            placeholder="请输入产品名称"
            options={[{ label: '本地', value: '本地' }]}
          ></Select>
        </Form.Item> */}
        <Form.Item
          label={t('所属品类')}
          name="category_template_id"
          extra={t('选择标准品类时, 需从系统生成的品类列表中选择一个品类')}
          rules={[{ required: true, message: t('请选择所属品类') }]}
        >
          <Radio.Group >
            <Radio value="0">{t('标准品类')}</Radio>
            <Radio value="1">{t('自定义品类')}</Radio>
          </Radio.Group>
        </Form.Item>
        <Form.Item
          noStyle
          shouldUpdate={(prevValues, nextValues) => {
            return prevValues.category_template_id !== nextValues.category_template_id
             || prevValues.category_template_id2 !== nextValues.category_template_id2
          }}
        >
          {({ getFieldValue }) => {
            const category_template_id2 = getFieldValue('category_template_id2')
            return getFieldValue('category_template_id') === '0'
              ? (
                <Form.Item
                  label={t('选择标准品类')}
                  name="category_template_id2"
                  rules={[{ required: true, message: t('请选择标准品类') }]}
                  extra={category_template_id2
                    && <div className="mt10">
                      <Typography.Link
                        onClick={() => {
                          standardFuncModalRef.current?.open(categoryKeyRef.current)
                        }}
                      >
                        {t('查看功能')}
                      </Typography.Link>
                    </div>
                  }
                >
                  <CategorySelect
                    presetOption={presetOption}
                    onCategoryKeyChange={(category_key) => {
                      categoryKeyRef.current = category_key
                    }}
                  />
                  {/* <DebounceSelect
                    placeholder="请选择品类"
                    fetchOptions={async (category_name: string) => {
                      try {
                        const resp = await categoryTemplate<CategoryTemplate>({ category_name })
                        return resp.success
                          ? resp.result.list.map(item => {
                            return {
                              label: (
                                <div>
                                  <div>{item.category_name}</div>
                                  <div>{item.scene}</div>
                                </div>
                              ),
                              value: item.id,
                            }
                          })
                          : []
                      } catch (error) {
                        return []
                      }
                    }}
                  /> */}
                </Form.Item>
              )
              : null
          }}
        </Form.Item>
        <Form.Item
          label={t('节点类型')}
          name="node_type"
          rules={[{ required: true, message: t('请选择节点类型') }]}
        >
          <Select
            placeholder={t('请选择')}
            options={[
              { label: t('直连设备'), value: '直连设备' },
              { label: t('网关设备'), value: '网关设备' },
              { label: t('网关子设备'), value: '网关子设备' },
            ]}
          ></Select>
        </Form.Item>
        <Form.Item
          label={t('接入网关协议')}
          name="protocol"
          rules={[{ required: true, message: t('请选择接入网关协议') }]}
        >
          <Select
            placeholder={t('请选择')}
            options={[
              { label: 'MQTT', value: 'MQTT' },
              { label: 'HTTP', value: 'HTTP' },
              { label: t('其他'), value: '其他' },
            ]}
          ></Select>
        </Form.Item>
        <Form.Item
          label={t('数据格式')}
          name="data_format"
          rules={[{ required: true, message: t('请选择数据格式') }]}
        >
          <Select
            placeholder={t('请选择')}
            options={[
              { label: t('标准物模型'), value: '标准物模型' },
              // { label: '二进制/透传', value: '二进制/透传' },
            ]}
          ></Select>
        </Form.Item>
        <Form.Item
          label={t('网络类型')}
          name="net_type"
          rules={[{ required: true, message: t('请选择网络类型') }]}
        >
          <Radio.Group >
            <Radio value="以太网">{t('以太网')}</Radio>
            <Radio value="蜂窝">{t('蜂窝')}</Radio>
            <Radio value="WIFI">WIFI</Radio>
            <Radio value="NB">NB</Radio>
            <Radio value="其他">{t('其他')}</Radio>
          </Radio.Group>
        </Form.Item>
        <Form.Item
          label={t('工厂')}
          name="factory"
        >
          <Input placeholder={t('请输入')}></Input>
        </Form.Item>
        <Form.Item
          label={t('产品描述')}
          name="description"
        >
          <TextArea placeholder={t('请输入产品描述')} rows={4}></TextArea>
        </Form.Item>
      </Form>
    </Drawer>
  )
})

export default AddModal
