import dayjs from 'dayjs'
import React, { useRef, useState } from 'react'
import { Table, Card, Form, Space, message, Typography, Popconfirm, Switch, Input, Button, Select } from 'antd'
import { PlusOutlined } from '@ant-design/icons'
// import { deleteScene, getScene, Scene, SceneData, startScene, stopScene } from '@/api'
import { deleteScene, getScene, Scene, SceneData, startScene, stopScene } from '@/api'
import { useTableRequest } from '@/hooks'
import { AddScene, AddSceneRef } from './components/AddScene'
// import { EditScene, EditSceneRef } from './components/EditScene'
import EditScene1 from './components/EditScene1'
import { useNavigate } from 'react-router-dom'
import useRequest from '@/hooks/useRequest'
import { t } from '@/i18n'

const { Text } = Typography
const { useMessage } = message

export interface SceneLinkageProps {

}

const SceneLinkage: React.FC<SceneLinkageProps> = () => {
  const navigate = useNavigate()

  const [messageApi, contextHolder] = useMessage()
  const addSceneRef = useRef<AddSceneRef>(null)
  // const editSceneRef = useRef<EditSceneRef>(null)
  const [record, setRecord] = useState<SceneData>()

  // 获取表格数据接口
  const { loading, dataSource, pagination, form, reload, search, reset } = useTableRequest<Scene>({
    onRequest: (params) => getScene({ ...params }),
  })

  return (
    <>
      {contextHolder}
      <AddScene ref={addSceneRef} onFinish={reload}></AddScene>
      {/* <EditScene key={record?.id} ref={editSceneRef} onFinish={reload}></EditScene> */}
      <Card className="m20" size="small">
        <Form
          layout="inline"
          form={form}
        >
          <Form.Item key="status" name="status">
            <Select
              style={{ width: 120 }}
              placeholder={t('启用状态')}
              options={[
                { label: t('状态 (全部)'), value: '' },
                { label: t('启用'), value: 'running' },
                { label: t('禁用'), value: 'stopped' },
              ]}
            />
          </Form.Item>
          <Form.Item key="name" name="name" label={t('场景名称')}>
            <Input style={{ width: 220 }} placeholder={t('请输入')} allowClear></Input>
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
              onClick={() => {
                addSceneRef.current?.open()
              }}
            ><PlusOutlined />{t('新建场景')}</Button>
          </div>
        </Space>
        {record && <EditScene1
          record={record}
          onClose={() => setRecord(undefined)}
          onFinish={() => reload()}
        />}
        <Table
          loading={loading}
          dataSource={dataSource}
          pagination={pagination}
          bordered
          rowKey="id"
          size="small"
          columns={[
            { title: t('场景名称'), key: 'name', dataIndex: 'name' },
            { title: t('场景描述'), key: 'description', dataIndex: 'description' },
            {
              title: t('创建时间'),
              key: 'created',
              dataIndex: 'created',
              render (value) {
                return dayjs(value).format('YYYY-MM-DD HH:mm:ss')
              },
            },
            {
              title: t('启用状态'),
              key: 'status',
              dataIndex: 'status',
              render: (status, r) => <StatusSwitch status={status} id={r.id} onSuccess={() => reload()} />,
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
                        setRecord(record)
                        // setTimeout(() => {
                        //   editSceneRef.current?.open(record)
                        // }, 0)
                      }}
                    >{t('编辑场景')}</Typography.Link>
                    <Typography.Link
                      onClick={() => {
                        navigate(`/advanced/scene/log/${record?.id}`)
                      }}
                    >{t('查询日志')}</Typography.Link>
                    <Popconfirm
                      title={t('您确定要删除该条场景吗？')}
                      okText={t('确定')}
                      cancelText={t('取消')}
                      placement="topRight"
                      onConfirm={() => {
                        deleteScene(record?.id)
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

export default SceneLinkage

enum EStatus {
  stopped = 'stopped',
  running = 'running',
}
function StatusSwitch ({ status, id, onSuccess }: { status: EStatus, id: string, onSuccess: () => void }) {
  const startSceneApi = useRequest(startScene, { manual: true })
  const stopSceneApi = useRequest(stopScene, { manual: true })
  console.log('status', status)

  return (<Switch
    checked={status === EStatus.running}
    checkedChildren={t('开')}
    unCheckedChildren={t('关')}
    loading={stopSceneApi.loading}
    onChange={async (bool) => {
      console.log(bool)
      try {
        if (bool) {
          await startSceneApi.run(id)
        } else {
          await stopSceneApi.run(id)
        }
        onSuccess?.()
      } catch (error) {
        console.log(error)
      }
    }}
  />)
}
