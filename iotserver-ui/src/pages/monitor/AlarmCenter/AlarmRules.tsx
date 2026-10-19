/* eslint-disable multiline-ternary */
import dayjs from 'dayjs'
import React, { useRef } from 'react'
import { Button, Card, Popconfirm, Space, Table, Typography, message, Form, Input, Tooltip } from 'antd'
import { PlusOutlined, MinusCircleFilled, CheckCircleFilled, ArrowsAltOutlined } from '@ant-design/icons'
import { getAlertRule, deleteRule, AlertRule, stopRule, startRule } from '@/api'
import { useTableRequest } from '@/hooks'
// import { RuleStatusMap } from '@/utils/statusMap'
import { AddRuleModal, AddRuleModalRef } from './components/AddRuleModal'
import { useNavigate } from 'react-router-dom'
import { t, tt } from '@/i18n'
// import { map } from 'lodash'

const { Text } = Typography
const { useMessage } = message
const { Search } = Input

export interface AlarmRulesProps {

}

const AlarmRules: React.FC<AlarmRulesProps> = () => {
  const navigate = useNavigate()
  const [messageApi, contextHolder] = useMessage()
  // const [selectedRowKeys, setSelectedRowKeys] = useState<string[]>([])
  const addRuleModalRef = useRef<AddRuleModalRef>(null)

  const { loading, dataSource, pagination, form, reload, search } = useTableRequest<AlertRule>({
    onRequest: (params) => getAlertRule({ ...params }),
  })

  return (
    <>
      {contextHolder}
      <AddRuleModal ref={addRuleModalRef} onFinish={reload}></AddRuleModal>
      <Form layout="inline" className="ml20" form={form}>
        <Form.Item key="name" name="name">
          <Search placeholder={t('请输入告警规则名称')} allowClear onSearch={search} />
        </Form.Item>
      </Form>
      <Card className="m20" size="small">
        <Space className="mb15 justify-space-between">
          <Button
            type="primary"
            onClick={() => {
              addRuleModalRef.current?.open()
            }}
          ><PlusOutlined />{t('添加规则')}</Button>
        </Space>
        <Table
          loading={loading}
          dataSource={dataSource}
          pagination={pagination}
          bordered
          rowKey="id"
          size="small"
          columns={[
            { title: t('告警规则名称'), key: 'name', dataIndex: 'name', width: '10em' },
            {
              title: t('告警规则描述'),
              key: 'description',
              dataIndex: 'description',
              width: 200,
              ellipsis: true,
              render (value) {
                return value
                  ? <>
                    <Tooltip placement="bottom" title={value}>
                      <Space><ArrowsAltOutlined style={{ color: '#1890ff' }}/> </Space>
                    </Tooltip>
                    {value}
                  </>
                  : '-'
              },
            },
            { title: t('告警类型'), key: 'alert_type', dataIndex: 'alert_type', width: '6em' },
            { title: t('告警级别'), key: 'alert_level', dataIndex: 'alert_level', width: '6em', render: (value: string) => tt(value) },
            {
              title: t('创建时间'),
              key: 'created',
              dataIndex: 'created',
              width: 130,
              render (value) {
                return dayjs(value).format('YYYY-MM-DD HH:mm:ss')
              },
            },
            {
              title: t('状态'),
              key: 'status',
              dataIndex: 'status',
              width: '6em',
              render: (value) => {
                if (value === 'stopped') return <Space><MinusCircleFilled style={{ color: '#87909d' }} />{t('禁用')}</Space>
                if (value === 'running') return <Space><CheckCircleFilled style={{ color: '#55af70' }} />{t('启用')}</Space>
                return '-'
              },
            },
            {
              title: t('操作'),
              key: 'action',
              dataIndex: 'action',
              width: 130,
              render (value, record) {
                return (
                  <Space split={<Text type="secondary">/</Text>}>
                    {record.status === 'running' ? (
                      <Popconfirm
                        title={t('您确定要禁用此告警规则吗？')}
                        okText={t('确定')}
                        cancelText={t('取消')}
                        placement="topRight"
                        onConfirm={() => {
                          stopRule(record?.id)
                            .then(resp => {
                              if (resp.success) {
                                messageApi.open({ type: 'success', content: t('禁用成功') })
                                reload()
                              } else {
                                messageApi.open({ type: 'error', content: resp.errorMsg })
                              }
                            })
                        }}
                      >
                        <Typography.Link>{t('禁用')}</Typography.Link>
                      </Popconfirm>
                    ) : (
                      <Popconfirm
                        title={t('您确定要启用此告警规则吗？')}
                        okText={t('确定')}
                        cancelText={t('取消')}
                        placement="topRight"
                        onConfirm={() => {
                          startRule(record?.id)
                            .then(resp => {
                              if (resp.success) {
                                messageApi.open({ type: 'success', content: t('启用成功') })
                                reload()
                              } else {
                                messageApi.open({ type: 'error', content: resp.errorMsg })
                              }
                            })
                        }}
                      >
                        <Typography.Link>{t('启用')}</Typography.Link>
                      </Popconfirm>
                    )}
                    <Typography.Link
                      onClick={() => {
                        navigate(`/monitor/alarmCenter/detail/${record?.id}`)
                      }}
                    >{t('规则详情')}</Typography.Link>
                    <Popconfirm
                      title={t('您确定要删除此告警规则吗？')}
                      okText={t('确定')}
                      cancelText={t('取消')}
                      placement="topRight"
                      onConfirm={() => {
                        deleteRule(record?.id)
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

export default AlarmRules
