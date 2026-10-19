import dayjs from 'dayjs'
import React, { useRef, useState } from 'react'
import { Button, Card, Popconfirm, Space, Table, Typography, message, Badge, Form, Input, Tag } from 'antd'
import { PlusOutlined } from '@ant-design/icons'
import { getDeviceLibraries, deleteDeviceLibraries } from '@/api'
import { useTableRequest } from '@/hooks'
import { OperateStatusMap } from '@/utils/statusMap'
import { AddModal, AddModalRef } from './AddModal'
import { UpdateModal } from './UpdateModal'
import { WebSocketProvider } from '@/components/WebSocket'
import { DeviceLibraries } from '@/api/device-libraries'
import { t } from '@/i18n'

const { Text } = Typography
const { useMessage } = message

export interface CustomDriveProps {

}

const CustomDrive: React.FC<CustomDriveProps> = () => {
  const [messageApi, contextHolder] = useMessage()
  const [selectedRowKeys, setSelectedRowKeys] = useState<string[]>([])
  const addModalRef = useRef<AddModalRef>(null)

  const { loading, dataSource, pagination, form, reload, search, reset } = useTableRequest<DeviceLibraries>({
    onRequest: (params) => getDeviceLibraries({ ...params, is_internal: false }),
  })

  return (
    <>
      {contextHolder}
      <AddModal ref={addModalRef} onFinish={reload}></AddModal>
      <Form layout="inline" className="ml20" form={form}>
        <Form.Item key="name" name="name">
          <Input placeholder={t('驱动镜像名称')}></Input>
        </Form.Item>
        <Form.Item>
          <Button type="primary" onClick={search}>{t('查询')}</Button>
        </Form.Item>
        <Form.Item>
          <Button type="default" onClick={reset}>{t('重置')}</Button>
        </Form.Item>
      </Form>
      <Card className="m20" size="small">
        <Space className="mb15 justify-space-between">
          <Button
            type="primary"
            onClick={() => {
              addModalRef.current?.open()
            }}
          ><PlusOutlined />{t('新增')}</Button>
        </Space>
        <Table
          loading={loading}
          dataSource={dataSource}
          pagination={pagination}
          bordered
          rowKey="id"
          size="small"
          columns={[
            { title: t('镜像编号'), key: 'id', dataIndex: 'id' },
            { title: t('镜像名称'), key: 'name', dataIndex: 'name' },
            {
              title: t('类型'),
              key: 'support_versions',
              dataIndex: 'support_versions',
              render (value, { support_versions }) {
                return support_versions[0]?.is_default ? <Tag color="green">{t('官方')}</Tag> : <Tag color="blue">{t('自定义')}</Tag>
              },
            },
            { title: t('版本'), key: 'version', dataIndex: 'version' },
            { title: t('协议'), key: 'protocol', dataIndex: 'protocol' },
            {
              title: t('状态'),
              key: 'operate_status',
              dataIndex: 'operate_status',
              render (value, { operate_status }) {
                const { status, text } = OperateStatusMap[operate_status]
                return (
                  <Badge status={status} text={text} />
                )
              },
            },
            {
              title: t('创建时间'),
              key: 'created',
              dataIndex: 'created',
              width: 170,
              render (value) {
                return dayjs(value).format('YYYY-MM-DD HH:mm:ss')
              },
            },
            {
              title: t('操作'),
              key: 'action',
              dataIndex: 'action',
              width: 170,
              render (value, record) {
                return (
                  <Space split={<Text type="secondary">/</Text>}>
                    <UpdateModal
                      record={record}
                      selectedRowKeys={selectedRowKeys}
                      onFinish={reload}
                      onChange={(id) => {
                        setSelectedRowKeys([id])
                      }}
                    ></UpdateModal>
                    <Typography.Link
                      onClick={() => {
                        addModalRef.current?.open(record)
                      }}
                    >{t('编辑')}</Typography.Link>
                    <Popconfirm
                      title={t('此操作将永久删除该条记录, 是否继续?')}
                      okText={t('确定')}
                      cancelText={t('取消')}
                      placement="topRight"
                      onConfirm={() => {
                        deleteDeviceLibraries(record?.id)
                          .then(resp => {
                            if (resp.success) {
                              messageApi.open({ type: 'success', content: t('删除成功') })
                              reload()
                            } else {
                              messageApi.open({ type: 'error', content: resp.errorMsg })
                            }
                          })
                      }}
                    >
                      <Typography.Link>{t('删除')}</Typography.Link>
                    </Popconfirm>
                  </Space>
                )
              },
            },
          ]}
        />
      </Card>
    </>
  )
}

export default () => (
  <WebSocketProvider>
    <CustomDrive></CustomDrive>
  </WebSocketProvider>
)
