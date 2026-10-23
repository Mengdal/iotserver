import dayjs from 'dayjs'
import React, { useEffect, useRef } from 'react'
import { useLocation, useNavigate, useSearchParams } from 'react-router-dom'
import { Table, Card, Form, Input, Select, Button, Space, message, Typography, Popconfirm } from 'antd'
import { PlusOutlined, ReloadOutlined, CheckCircleFilled, MinusCircleFilled } from '@ant-design/icons'
import { getProductList, deleteProduct, useGetIotPlatform } from '@/api'
import { useTableRequest } from '@/hooks'
import { AddModal, AddModalRef } from './AddModal'
import { confirmPlatformViewAdd } from '@/utils/platformViewGuard'
import { ProductType } from '@/api/product'
import { t, tt } from '@/i18n'

const { Text } = Typography
const { useMessage } = message

export interface ProductManageProps {

}

// const NodeTypeMap: {[key: string]: string} = {
//   0: '未知类型',
//   1: '网关',
//   2: '直连设备',
//   3: '网管子设备',
// }

const ProductManage: React.FC<ProductManageProps> = () => {
  const [match] = useSearchParams()
  useEffect(() => {
    match.has('create') && addModalRef.current?.open()
  }, [])

  const navigate = useNavigate()
  const location = useLocation()

  const addModalRef = useRef<AddModalRef>(null)

  // 从方案中心跳转过来时自动打开创建产品抽屉并预选方案品类；
  // 打开后立刻清掉 state，避免刷新或后退时重复弹出。
  useEffect(() => {
    const state = location.state as {
      openAdd?: boolean
      categoryTemplateId?: string
      categoryName?: string
    } | null
    if (!state?.openAdd) return
    addModalRef.current?.open({
      categoryTemplateId: state.categoryTemplateId,
      categoryName: state.categoryName,
    })
    navigate(location.pathname, { replace: true, state: null })
  }, [])

  const [messageApi, contextHolder] = useMessage()

  const { loading, form, dataSource, pagination, reload, search, reset } = useTableRequest<ProductType>({
    onRequest: getProductList,
  })

  const { data: iotPlatformOptions } = useGetIotPlatform()

  return (
    <>
      {contextHolder}
      <AddModal ref={addModalRef} onFinish={search} />
      <Card className="m20" size="small">
        <Form layout="inline" form={form}>
          <Form.Item key="platform" name="platform" initialValue={''}>
            <Select
              placeholder={t('平台')}
              style={{ minWidth: 180 }}
              options={[{ label: t('全部'), value: '' }, ...iotPlatformOptions.map(i => ({ label: tt(i), value: i }))]}
            />
          </Form.Item>
          <Form.Item key="name" name="name">
            <Input placeholder={t('产品名称')}></Input>
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
          <Button
            type="primary"
            onClick={async () => {
              if (!await confirmPlatformViewAdd()) return
              addModalRef.current?.open()
            }}
          ><PlusOutlined />{t('创建产品')}</Button>
          <div className="flex-center">
            <Button
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
            { title: t('产品ID'), key: 'id', dataIndex: 'id' },
            { title: t('产品名称'), key: 'name', dataIndex: 'name' },
            {
              title: t('产品编号'),
              key: 'product_id',
              dataIndex: 'product_id',
              render (value) {
                return value ? <Text copyable={{ tooltips: [t('复制'), t('复制成功')] }}>{value}</Text> : null
              },
            },
            { title: t('类型'), key: 'node_type', dataIndex: 'node_type', render: (value: string) => tt(value) },
            { title: t('云平台'), key: 'platform', dataIndex: 'platform', render: (value: string) => tt(value) },
            {
              title: t('创建时间'),
              key: 'created_at',
              dataIndex: 'created_at',
              render (value) {
                return dayjs(value).format('YYYY-MM-DD HH:mm:ss')
              },
            },
            {
              title: t('状态'),
              key: 'status',
              dataIndex: 'status',
              render: (value) => {
                if (value === '未发布') return <Space><MinusCircleFilled style={{ color: '#87909d' }} />{t('未发布')}</Space>
                if (value === '已发布') return <Space><CheckCircleFilled style={{ color: '#55af70' }} />{t('已发布')}</Space>
                return '-'
              },
              // render (value, record) {
              //   return (
              //     <>
              //       {value && <Badge color={({
              //         已发布: 'green',
              //         未发布: 'gray',
              //       })[record.status]} />}
              //       <Text className="ml5">{value || '-'}</Text>
              //     </>
              //   )
              // },
            },
            {
              title: t('操作'),
              key: 'action',
              dataIndex: 'action',
              render (value, record) {
                return (
                  <Space split={<Text type="secondary">/</Text>}>
                    <Typography.Link
                      onClick={() => {
                        navigate(`/gateway/product/detail/${record?.id}`)
                      }}
                    >{t('详情')}</Typography.Link>
                    <Typography.Link
                      onClick={() => {
                        navigate(`/gateway/device/manage/${record?.id}`)
                      }}
                    >{t('管理设备')}</Typography.Link>
                    <Popconfirm
                      title={t('此操作将永久删除该条记录, 是否继续?')}
                      okText={t('确定')}
                      cancelText={t('取消')}
                      placement="topRight"
                      onConfirm={() => {
                        deleteProduct(record?.id)
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

export default ProductManage
