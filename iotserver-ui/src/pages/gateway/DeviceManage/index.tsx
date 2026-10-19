import dayjs from 'dayjs'
import React, { useEffect, useRef, useState } from 'react'
import { Table, Card, Form, Input, Select, Button, Space, message, Typography, Popconfirm, Badge } from 'antd'
import { PlusOutlined, DeleteOutlined, LockOutlined, UnlockOutlined, SyncOutlined, ReloadOutlined } from '@ant-design/icons'
import { getDevice, deleteDevice, deleteDevices, unBindDriver, useGetDeviceServers, useGetDeviceStatusTemplate, useGetIotPlatform, useGetProductList } from '@/api'
import { useTableRequest } from '@/hooks'
import { confirmPlatformViewAdd } from '@/utils/platformViewGuard'
import { AddModal, AddModalRef } from './AddModal'
import { EditModal, EditModalRef } from './EditModal'
import { BatchBindModal, BatchBindModalRef } from './BatchBindModal'
import { DeviceSyncModal, DeviceSyncModalRef } from './DeviceSyncModal'
import { useNavigate, useParams, useSearchParams } from 'react-router-dom'
import { DevicesType } from '@/api/devices'
import { t, tt } from '@/i18n'

const { Text } = Typography
const { useMessage } = message

export interface DevicesManageProps {

}

const DevicesManage: React.FC<DevicesManageProps> = () => {
  const [match] = useSearchParams()
  useEffect(() => {
    match.has('create') && addModalRef.current?.open()
  }, [])
  const navigate = useNavigate()
  const params = useParams<{ id?: string }>()
  console.log(params?.id)

  const addModalRef = useRef<AddModalRef>(null)
  const editModalRef = useRef<EditModalRef>(null)
  const batchBindModalRef = useRef<BatchBindModalRef>(null)
  const deviceSyncModalRef = useRef<DeviceSyncModalRef>(null)

  const [messageApi, contextHolder] = useMessage()
  const [selectedRowKeys, setSelectedRowKeys] = useState<string[]>([])

  const { loading, form, dataSource, pagination, reload, search, reset } = useTableRequest<DevicesType>({
    onRequest: (data) => getDevice({ ...data, product_id: data.product_id || params.id }),
  })

  const { data: deviceServersOptions } = useGetDeviceServers({ platform: '本地' })
  const { data: statusOptions } = useGetDeviceStatusTemplate()
  const { data: iotPlatformOptions } = useGetIotPlatform()
  const { data: productListOptions } = useGetProductList({ isAll: true })

  return (
    <>
      {contextHolder}
      <AddModal
        ref={addModalRef}
        deviceServersOptions={deviceServersOptions}
        onFinish={reload}
      />
      <EditModal
        ref={editModalRef}
        onFinish={reload}
      />
      <DeviceSyncModal
        ref={deviceSyncModalRef}
        options={deviceServersOptions}
        onFinish={() => {
          reload()
        }}
      />
      <BatchBindModal
        ref={batchBindModalRef}
        // options={deviceServersOptions}
        device_ids={selectedRowKeys}
        onFinish={() => {
          setSelectedRowKeys([])
          reload()
        }}
      />
      <Card className="m20" size="small">
        <Form
          layout="inline"
          form={form}
          initialValues={{
            product_id: params?.id || '',
            platform: '',
            drive_instance_id: '',
            status: '',
            name: '',
          }}
        >
          <Form.Item key="product_id" name="product_id">
            <Select
              style={{ width: 180 }}
              placeholder={t('产品范围')}
              options={[{ label: t('产品范围 (全部)'), value: '' }, ...productListOptions.map(i => ({ label: i.name, value: i.id }))]}
            />
          </Form.Item>
          <Form.Item key="platform" name="platform">
            <Select
              style={{ width: 180 }}
              placeholder={t('平台')}
              options={[{ label: t('平台 (全部)'), value: '' }, ...iotPlatformOptions.map(i => ({ label: tt(i), value: i }))]}
            />
          </Form.Item>
          <Form.Item key="drive_instance_id" name="drive_instance_id">
            <Select
              style={{ width: 180 }}
              placeholder={t('驱动')}
              options={[{ label: t('驱动 (全部)'), value: '' }, ...deviceServersOptions.map(i => ({ label: i.name, value: i.id }))]}
            />
          </Form.Item>
          <Form.Item key="status" name="status">
            <Select
              style={{ width: 180 }}
              placeholder={t('状态')}
              options={[{ label: t('状态 (全部)'), value: '' }, ...statusOptions.map(i => ({ label: tt(i), value: i }))]}
            />
          </Form.Item>
          <Form.Item key="name" name="name">
            <Input placeholder={t('设备名称')}></Input>
          </Form.Item>
          <Form.Item>
            <Button type="primary" onClick={search}>{t('查询')}</Button>
          </Form.Item>
          <Form.Item>
            <Button type="default" onClick={reset}>{t('重置')}</Button>
          </Form.Item>
        </Form>
      </Card>
      <Card className="m20" size="small">
        <Space className="mb15 justify-space-between">
          <div>
            <Button
              type="primary"
              onClick={async () => {
                if (!await confirmPlatformViewAdd()) return
                addModalRef.current?.open()
              }}
            ><PlusOutlined />{t('添加设备')}</Button>
          </div>
          <div>
            <Button
              className="ml10"
              disabled={!selectedRowKeys?.length}
              onClick={() => {
                batchBindModalRef.current?.open()
              }}><LockOutlined />{t('批量驱动绑定')}
            </Button>
            <Popconfirm
              title={t('确定解绑当前勾选的设备吗?')}
              okText={t('确定')}
              cancelText={t('取消')}
              disabled={!selectedRowKeys?.length}
              onConfirm={() => {
                unBindDriver({ device_ids: selectedRowKeys }).then(resp => {
                  if (resp.success) {
                    messageApi.open({ type: 'success', content: t('批量解绑成功') })
                    setSelectedRowKeys([])
                    reload()
                  } else {
                    messageApi.open({ type: 'error', content: resp.errorMsg })
                  }
                })
              }}
            >
              <Button className="ml10" disabled={!selectedRowKeys?.length}><UnlockOutlined />{t('批量驱动解绑')}</Button>
            </Popconfirm>
            <Popconfirm
              title={t('确定删除当前勾选的设备吗?')}
              okText={t('确定')}
              cancelText={t('取消')}
              disabled={!selectedRowKeys?.length}
              onConfirm={() => {
                deleteDevices({ device_ids: selectedRowKeys }).then(resp => {
                  if (resp.success) {
                    messageApi.open({ type: 'success', content: t('批量删除成功') })
                    setSelectedRowKeys([])
                    reload()
                  } else {
                    messageApi.open({ type: 'error', content: resp.errorMsg })
                  }
                })
              }}
            >
              <Button className="ml10" disabled={!selectedRowKeys?.length}><DeleteOutlined />{t('批量删除')}</Button>
            </Popconfirm>
            <Button className="ml10"
              onClick={() => {
                deviceSyncModalRef.current?.open()
              }}
            ><SyncOutlined />{t('设备同步')}</Button>
            <Button className="ml10"
              onClick={() => {
                reload()
              }}
            ><ReloadOutlined />{t('刷新')}</Button>
          </div>
        </Space>
        <Table
          loading={loading}
          dataSource={dataSource}
          pagination={pagination}
          bordered
          rowKey="id"
          size="small"
          columns={[
            { title: t('设备ID'), key: 'id', dataIndex: 'id' },
            { title: t('设备名称'), key: 'name', dataIndex: 'name' },
            { title: t('所属产品'), key: 'product_name', dataIndex: 'product_name' },
            {
              title: t('设备状态'),
              key: 'status',
              dataIndex: 'status',
              render (value, record) {
                return (
                  <>
                    <Badge color={{
                      在线: 'green',
                      离线: 'gray',
                      禁用: 'red',
                      未激活: 'orange',
                    }[record.status] || 'gray'} />
                    <Text className="ml5">{tt(value) || t('未知')}</Text>
                  </>
                )
              },
            },
            { title: t('平台'), key: 'platform', dataIndex: 'platform', render: (value: string) => tt(value) },
            { title: t('关联驱动'), key: 'driver_service_name', dataIndex: 'driver_service_name' },
            {
              title: t('创建时间'),
              key: 'created',
              dataIndex: 'created',
              render (value) {
                return dayjs(value).format('YYYY-MM-DD HH:mm:ss')
              },
            },
            {
              title: t('最近在线时间'),
              key: 'last_online_time',
              dataIndex: 'last_online_time',
              render (value) {
                return value === 0 ? '-' : dayjs(value).format('YYYY-MM-DD HH:mm:ss')
              },
            },
            {
              title: t('操作'),
              key: 'action',
              dataIndex: 'action',
              width: 190,
              render (value, record) {
                return (
                  <Space split={<Text type="secondary">/</Text>}>
                    <Typography.Link
                      onClick={() => {
                        navigate(`/gateway/device/detail/${record?.id}`)
                      }}
                    >{t('详情')}</Typography.Link>
                    <Typography.Link
                      onClick={() => {
                        editModalRef.current?.open(record)
                      }}
                    >{t('编辑')}</Typography.Link>
                    <Popconfirm
                      title={t('此操作将永久删除该条记录, 是否继续?')}
                      okText={t('确定')}
                      cancelText={t('取消')}
                      placement="topRight"
                      onConfirm={() => {
                        deleteDevice(record?.id)
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
          rowSelection={{
            type: 'checkbox',
            onChange (selectedRowKeys) {
              setSelectedRowKeys(selectedRowKeys as string[])
            },
          }}
        />
      </Card>
    </>
  )
}

export default DevicesManage
