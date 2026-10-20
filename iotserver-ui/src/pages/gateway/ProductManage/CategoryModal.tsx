import React, { useImperativeHandle, forwardRef, useState, useCallback, useRef } from 'react'
import { Button, Drawer, Form, Input, Space, Table, Typography } from 'antd'
import { CloseOutlined } from '@ant-design/icons'
import { categoryTemplate } from '@/api'
import { useTableRequest } from '@/hooks'
import { CategoryTemplate } from './types'
import { StandardFuncModal, StandardFuncModalRef } from './StandardFuncModal'
import { t } from '@/i18n'
import { localized } from '@/utils/localized'

const { Text } = Typography

export interface CategoryModalRef {
  open: () => void
}

export interface CategoryModalProps {
  onChange?: (record: CategoryTemplate) => void
}

export const CategoryModal = forwardRef<CategoryModalRef, CategoryModalProps>(({ onChange }, ref) => {
  const standardFuncModalRef = useRef<StandardFuncModalRef>(null)

  const [isModalOpen, setIsModalOpen] = useState<boolean>(false)

  const { loading, dataSource, pagination, form, reset, search } = useTableRequest<CategoryTemplate>({
    onRequest: categoryTemplate,
  })

  useImperativeHandle(ref, () => {
    return {
      open () {
        setIsModalOpen(true)
      },
    }
  }, [])

  const handleClose = useCallback(() => {
    setIsModalOpen(false)
  }, [])

  return (
    <Drawer
      title={t('选择品类')}
      placement="right"
      open={isModalOpen}
      width={550}
      closable={false}
      maskClosable={false}
      destroyOnClose={false}
      headerStyle={{ height: 51 }}
      onClose={handleClose}
      extra={<CloseOutlined onClick={handleClose} />}
      footer={
        <Space>
          <Button onClick={handleClose}>{t('取消')}</Button>
        </Space>
      }
    >
      <StandardFuncModal
        ref={standardFuncModalRef}
      />
      <Form layout="inline" form={form} className="mb20">
        <Form.Item key="category_name" name="category_name">
          <Input placeholder={t('输入品类名称')}></Input>
        </Form.Item>
        <Form.Item>
          <Button type="primary" onClick={search}>{t('查询')}</Button>
        </Form.Item>
        <Form.Item>
          <Button type="default" onClick={reset}>{t('重置')}</Button>
        </Form.Item>
      </Form>
      <Table
        loading={loading}
        dataSource={dataSource}
        pagination={{ ...pagination, showSizeChanger: false, showQuickJumper: false }}
        bordered
        rowKey="id"
        size="small"
        columns={[
          { title: t('品类名称'), key: 'category_name', dataIndex: 'category_name', render: (_value: string, record: CategoryTemplate) => localized(record.category_name, record.category_name_en) },
          { title: t('所属场景'), key: 'scene', dataIndex: 'scene' },
          {
            title: t('操作'),
            key: 'action',
            dataIndex: 'action',
            render (value, record) {
              return (
                <Space split={<Text type="secondary">/</Text>}>
                  <Typography.Link
                    onClick={() => {
                      onChange?.(record)
                      handleClose()
                    }}
                  >{t('选择')}</Typography.Link>
                  <Typography.Link
                    onClick={() => {
                      standardFuncModalRef.current?.open(record.category_key)
                    }}
                  >{t('查看标准物模型')}</Typography.Link>
                </Space>
              )
            },
          },
        ]}
      />
    </Drawer>
  )
})
