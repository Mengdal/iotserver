import React, { useRef, useState } from 'react'
import {
  Button,
  Card,
  Descriptions,
  Drawer,
  Form,
  Input,
  message,
  Modal,
  Select,
  Space,
  Table,
  Tag,
  Typography,
} from 'antd'
import {
  ApartmentOutlined,
  CloudDownloadOutlined,
  CloudUploadOutlined,
  PauseCircleOutlined,
  PlayCircleOutlined,
  PlusOutlined,
  ReloadOutlined,
  SearchOutlined,
  SyncOutlined,
} from '@ant-design/icons'
import dayjs from 'dayjs'
import { useNavigate } from 'react-router-dom'
import {
  batchStartRuleEngine,
  batchStopRuleEngine,
  deleteRuleEngine,
  exportRuleEngine,
  getRuleEngine,
  getRuleEngineInfo,
  importRuleEngine,
  startRuleEngine,
  statusRuleEngine,
  stopRuleEngine,
} from '@/api/RuleEngine'
import type { DataRuleEngine, RuleEngineExportFile } from '@/api/RuleEngine'
import { useTableRequest } from '@/hooks'
import useRequest from '@/hooks/useRequest'
import { hasPermission } from '@/utils/auth'
import { AdvancedRuleAdd, AdvancedRuleEdit, AdvancedRuleRemove, AdvancedRuleStart, AdvancedRuleStop } from '@/routes/permissions'
import { t } from '@/i18n'
import './RuleEngine.less'

const { Text } = Typography
const { useMessage } = message

const RuleEngine: React.FC = () => {
  const navigate = useNavigate()
  const [messageApi, contextHolder] = useMessage()
  const [selectedKeys, setSelectedKeys] = useState<React.Key[]>([])
  const [detailOpen, setDetailOpen] = useState(false)
  const [detailInfo, setDetailInfo] = useState<DataRuleEngine | null>(null)
  const [detailStatus, setDetailStatus] = useState<any>(null)
  const detailIdRef = useRef('')
  const [batchLoading, setBatchLoading] = useState(false)
  const [importing, setImporting] = useState(false)
  const fileInputRef = useRef<HTMLInputElement>(null)
  const infoApi = useRequest(getRuleEngineInfo, { manual: true })
  const statusApi = useRequest(statusRuleEngine, { manual: true })

  const { loading, dataSource, pagination, form, reload, search, reset } = useTableRequest<DataRuleEngine>({
    onRequest: params => getRuleEngine({ ...params }),
  })

  const canAdd = hasPermission(AdvancedRuleAdd)
  const canEdit = hasPermission(AdvancedRuleEdit)
  const canRemove = hasPermission(AdvancedRuleRemove)
  const canStart = hasPermission(AdvancedRuleStart)
  const canStop = hasPermission(AdvancedRuleStop)

  const selectedIds = selectedKeys.map(String)

  const openCreate = () => navigate('/advanced/rule/engine/new')

  // 编辑不提前停止规则：编辑器保存时再停止/更新/恢复，避免用户只是打开编辑页
  // 就改变了运行状态（侧边栏跳转、刷新等场景无法可靠恢复）。
  const openEdit = (record: DataRuleEngine) => {
    navigate(`/advanced/rule/engine/${record.id}/edit`)
  }

  const toggleRule = async (record: DataRuleEngine) => {
    try {
      const resp = record.status === 'running'
        ? await stopRuleEngine(record.id)
        : await startRuleEngine(record.id)
      if (!resp.success) {
        messageApi.error(resp.errorMsg || t('操作失败'))
        return
      }
      messageApi.success(record.status === 'running' ? t('规则已停止') : t('规则已启动'))
      reload()
    } catch (error) {
      messageApi.error(t('操作失败'))
    }
  }

  const handleBatch = async (start: boolean) => {
    if (selectedIds.length === 0) {
      messageApi.warning(t('请先选择规则'))
      return
    }
    setBatchLoading(true)
    try {
      const resp = start
        ? await batchStartRuleEngine(selectedIds)
        : await batchStopRuleEngine(selectedIds)
      if (!resp.success) {
        messageApi.error(resp.errorMsg || t('操作失败'))
        return
      }
      const result = resp.result
      if (result?.failed?.length) {
        messageApi.warning(t('成功 {{success}} 条，失败 {{failed}} 条', {
          success: result.success,
          failed: result.failed.length,
        }))
      } else {
        messageApi.success(t('成功 {{num}} 条', { num: result?.success || 0 }))
      }
      setSelectedKeys([])
      reload()
    } catch (error) {
      messageApi.error(t('操作失败'))
    } finally {
      setBatchLoading(false)
    }
  }

  const handleExport = async () => {
    try {
      const blob = await exportRuleEngine(selectedIds)
      const url = URL.createObjectURL(blob)
      const link = document.createElement('a')
      link.href = url
      link.download = `iotserver-rules-${dayjs().format('YYYYMMDD-HHmmss')}.json`
      document.body.appendChild(link)
      link.click()
      document.body.removeChild(link)
      // Safari 会异步开始下载，立即 revoke 会导致空文件/下载失败
      window.setTimeout(() => URL.revokeObjectURL(url), 1000)
    } catch (error) {
      messageApi.error(error instanceof Error && error.message ? error.message : t('导出失败'))
    }
  }

  const handleImportFile = async (file: File) => {
    setImporting(true)
    try {
      const text = await file.text()
      const parsed = JSON.parse(text) as RuleEngineExportFile
      if (!parsed || !Array.isArray(parsed.rules)) {
        messageApi.error(t('导入文件格式不正确'))
        return
      }
      const resp = await importRuleEngine(parsed.rules)
      if (!resp.success) {
        messageApi.error(resp.errorMsg || t('导入失败'))
        return
      }
      const result = resp.result
      if (result?.failed?.length) {
        Modal.warning({
          title: t('部分规则导入失败'),
          content: (
            <div style={{ whiteSpace: 'pre-wrap' }}>
              {result.failed.map(item => `${item.name || '-'}: ${item.message}`).join('\n')}
            </div>
          ),
        })
      }
      messageApi.success(t('成功导入 {{num}} 条规则', { num: result?.imported || 0 }))
      reload()
    } catch (error) {
      messageApi.error(t('导入文件格式不正确'))
    } finally {
      setImporting(false)
    }
  }

  const openDetail = async (record: DataRuleEngine) => {
    detailIdRef.current = record.id
    setDetailOpen(true)
    setDetailInfo(null)
    setDetailStatus(null)
    const [info, status] = await Promise.all([
      infoApi.run(record.id),
      statusApi.run(record.id),
    ])
    if (detailIdRef.current !== record.id) return
    setDetailInfo((info as DataRuleEngine) || null)
    setDetailStatus(status || null)
  }

  const closeDetail = () => {
    detailIdRef.current = ''
    setDetailOpen(false)
    setDetailInfo(null)
    setDetailStatus(null)
  }

  const refreshDetail = async () => {
    const id = detailIdRef.current
    if (!id) return
    const status = await statusApi.run(id)
    if (detailIdRef.current === id) setDetailStatus(status || null)
  }

  const handleDelete = (record: DataRuleEngine) => {
    Modal.confirm({
      title: t('您确定要删除该规则吗？'),
      content: record.name,
      okText: t('删除'),
      okButtonProps: { danger: true },
      cancelText: t('取消'),
      onOk: async () => {
        const resp = await deleteRuleEngine(record.id)
        if (!resp.success) {
          messageApi.error(resp.errorMsg)
          return
        }
        messageApi.success(t('删除成功'))
        reload()
      },
    })
  }

  return (
    <>
      {contextHolder}
      <input
        ref={fileInputRef}
        type="file"
        accept=".json,application/json"
        style={{ display: 'none' }}
        onChange={event => {
          const file = event.target.files?.[0]
          if (file) handleImportFile(file)
          event.target.value = ''
        }}
      />
      <Card className="m20 rule-list-card" size="small">
        <div className="rule-list-toolbar">
          <Form form={form} layout="inline" className="rule-list-search">
            <Form.Item name="keyword">
              <Input.Search
                allowClear
                placeholder={t('输入 ID/名称/标签进行搜索')}
                style={{ width: 260 }}
                onSearch={() => search()}
              />
            </Form.Item>
            <Form.Item name="status">
              <Select
                allowClear
                style={{ width: 130 }}
                placeholder={t('状态 (全部)')}
                options={[
                  { label: t('运行中'), value: 'running' },
                  { label: t('已停止'), value: 'stopped' },
                ]}
              />
            </Form.Item>
            <Form.Item>
              <Button type="primary" icon={<SearchOutlined />} onClick={() => search()}>{t('查询')}</Button>
            </Form.Item>
            <Form.Item>
              <Button icon={<ReloadOutlined />} onClick={() => reset()}>{t('重置')}</Button>
            </Form.Item>
          </Form>
          <Space wrap className="rule-list-actions">
            <Button
              icon={<PlayCircleOutlined />}
              loading={batchLoading}
              disabled={selectedIds.length === 0 || !canStart}
              onClick={() => handleBatch(true)}
            >{t('批量启动')}</Button>
            <Button
              icon={<PauseCircleOutlined />}
              loading={batchLoading}
              disabled={selectedIds.length === 0 || !canStop}
              onClick={() => handleBatch(false)}
            >{t('批量停止')}</Button>
            <Button icon={<CloudDownloadOutlined />} onClick={() => handleExport()}>{t('导出规则')}</Button>
            <Button
              icon={<CloudUploadOutlined />}
              loading={importing}
              disabled={!canAdd}
              onClick={() => fileInputRef.current?.click()}
            >{t('导入规则')}</Button>
            <Button icon={<ApartmentOutlined />} onClick={() => navigate('/advanced/rule/flow')}>
              {t('流程编排（演示）')}
            </Button>
            {canAdd && (
              <Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>{t('新建规则')}</Button>
            )}
          </Space>
        </div>

        <Table
          loading={loading}
          dataSource={dataSource}
          pagination={pagination}
          rowKey="id"
          size="small"
          bordered
          scroll={{ x: 'max-content' }}
          rowSelection={{ selectedRowKeys: selectedKeys, onChange: setSelectedKeys }}
          columns={[
            {
              title: 'ID',
              dataIndex: 'id',
              width: 150,
              render: value => <Typography.Text copyable className="rule-id-text">{value}</Typography.Text>,
            },
            {
              title: t('名称'),
              dataIndex: 'name',
              width: 180,
              render: (value, record) => (
                <div className="rule-name-cell">
                  <Typography.Link onClick={() => openDetail(record)}>{value}</Typography.Link>
                  {record.description && <span className="rule-name-desc">{record.description}</span>}
                </div>
              ),
            },
            {
              title: t('状态'),
              dataIndex: 'status',
              width: 100,
              render: status => (
                <Tag color={status === 'running' ? 'green' : 'default'}>
                  {status === 'running' ? t('运行中') : t('已停止')}
                </Tag>
              ),
            },
            {
              title: t('标签'),
              dataIndex: 'tags',
              width: 180,
              render: (tags: string[]) => (
                tags && tags.length > 0
                  ? <Space size={[0, 4]} wrap>{tags.map(tag => <Tag key={tag}>{tag}</Tag>)}</Space>
                  : <Text type="secondary">-</Text>
              ),
            },
            {
              title: t('应用'),
              dataIndex: 'actions',
              width: 240,
              render: (actions: DataRuleEngine['actions']) => (
                actions && actions.length > 0
                  ? (
                    <Space size={[8, 4]} wrap>
                      {actions.map(action => (
                        <Typography.Link
                          key={action.resource_id}
                          onClick={() => navigate(`/advanced/resource/manage?type=${encodeURIComponent(action.resource_type)}`)}
                        >
                          {action.resource_name || action.resource_id}
                        </Typography.Link>
                      ))}
                    </Space>
                  )
                  : <Text type="secondary">-</Text>
              ),
            },
            {
              title: t('创建时间'),
              dataIndex: 'created',
              width: 170,
              render: value => dayjs(value).format('YYYY-MM-DD HH:mm:ss'),
            },
            {
              title: t('操作'),
              key: 'action',
              width: 250,
              render: (_, record) => {
                const canToggle = record.status === 'running' ? canStop : canStart
                return (
                  <Space size={4} split={<Text type="secondary">/</Text>}>
                    <Typography.Link onClick={() => openDetail(record)}>{t('详情')}</Typography.Link>
                    <Typography.Link disabled={!canEdit} onClick={canEdit ? () => openEdit(record) : undefined}>
                      {t('编辑')}
                    </Typography.Link>
                    <Typography.Link
                      disabled={!canToggle}
                      onClick={canToggle ? () => toggleRule(record) : undefined}
                    >
                      {record.status === 'running' ? t('停止') : t('启动')}
                    </Typography.Link>
                    <Typography.Link disabled={!canRemove} onClick={canRemove ? () => handleDelete(record) : undefined}>
                      {t('删除')}
                    </Typography.Link>
                  </Space>
                )
              },
            },
          ]}
        />
      </Card>

      <Drawer
        title={t('规则详情')}
        open={detailOpen}
        width={780}
        onClose={closeDetail}
        destroyOnClose
        extra={<Button icon={<SyncOutlined />} loading={statusApi.loading} onClick={refreshDetail} />}
      >
        {detailInfo && (
          <>
            <Descriptions column={2} size="small" bordered>
              <Descriptions.Item label="ID" span={2}>{detailInfo.id}</Descriptions.Item>
              <Descriptions.Item label={t('名称')}>{detailInfo.name}</Descriptions.Item>
              <Descriptions.Item label={t('状态')}>
                <Tag color={detailInfo.status === 'running' ? 'green' : 'default'}>
                  {detailInfo.status === 'running' ? t('运行中') : t('已停止')}
                </Tag>
              </Descriptions.Item>
              <Descriptions.Item label={t('标签')} span={2}>
                {((detailInfo.tags || []) as string[]).map((tag: string) => <Tag key={tag}>{tag}</Tag>)}
              </Descriptions.Item>
              <Descriptions.Item label={t('应用')} span={2}>
                <Space size={[8, 4]} wrap>
                  {((detailInfo.actions || []) as DataRuleEngine['actions']).map(action => (
                    <Typography.Link
                      key={action.resource_id}
                      onClick={() => navigate(`/advanced/resource/manage?type=${encodeURIComponent(action.resource_type)}`)}
                    >
                      {action.resource_name || action.resource_id}
                    </Typography.Link>
                  ))}
                </Space>
              </Descriptions.Item>
              <Descriptions.Item label={t('创建时间')}>{dayjs(detailInfo.created).format('YYYY-MM-DD HH:mm:ss')}</Descriptions.Item>
              <Descriptions.Item label={t('转发方式')}>{detailInfo.dataResource?.type}</Descriptions.Item>
            </Descriptions>
            <div className="rule-detail-section-title">{t('SQL')}</div>
            <pre className="rule-detail-sql">{detailInfo.filter?.sql}</pre>
            <div className="rule-detail-section-title">{t('运行指标')}</div>
            <pre className="rule-detail-status">{detailStatus ? JSON.stringify(detailStatus, null, 2) : '-'}</pre>
          </>
        )}
      </Drawer>
    </>
  )
}

export default RuleEngine
