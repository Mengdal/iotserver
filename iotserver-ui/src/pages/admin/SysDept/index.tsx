import React, { useCallback, useEffect, useMemo, useState } from 'react'
import {
  Button,
  Card,
  Col,
  Drawer,
  Form,
  Input,
  InputNumber,
  Popconfirm,
  Radio,
  Row,
  Select,
  Space,
  Table,
  Tag,
  message,
} from 'antd'
import { PlusOutlined, ReloadOutlined, SearchOutlined, MinusSquareOutlined, PlusSquareOutlined } from '@ant-design/icons'
import type { ColumnsType } from 'antd/es/table'
import dayjs from 'dayjs'
import {
  getSysDeptTree,
  addSysDept,
  updateSysDept,
  deleteSysDept,
  SysDept,
} from '@/api'
import { useAuth } from '@/utils/auth'
import { useTreeTable } from '../utils/useTreeTable'
import type { FlatMeta } from '../utils/treeTable'

const { useMessage } = message

/** 组织节点类型：与后端 models.DeptType* 保持一致 */
const DEPT_TYPE_OPTIONS = [
  { value: 1, label: '公司' },
  { value: 2, label: '工厂' },
  { value: 3, label: '车间' },
  { value: 4, label: '产线' },
]

const deptTypeLabel = (value?: number) => DEPT_TYPE_OPTIONS.find(item => item.value === value)?.label || '公司'

type FlatDept = SysDept & FlatMeta

interface ParentOption {
  value: string
  title: string
  children?: ParentOption[]
}

function toParentOptions (list: SysDept[], excludeId?: string): ParentOption[] {
  const result: ParentOption[] = []
  list.forEach(item => {
    if (excludeId && item.id === excludeId) return
    const option: ParentOption = { value: item.id, title: item.name }
    if (item.children && item.children.length > 0) {
      const children = toParentOptions(item.children, excludeId)
      if (children.length > 0) option.children = children
    }
    result.push(option)
  })
  return result
}

const SysDeptPage: React.FC = () => {
  const [messageApi, contextHolder] = useMessage()
  const [searchForm] = Form.useForm()
  const [editForm] = Form.useForm()

  const [loading, setLoading] = useState(false)
  const [treeData, setTreeData] = useState<SysDept[]>([])
  const [filters, setFilters] = useState<{ name?: string, status?: string }>({})
  const [modalOpen, setModalOpen] = useState(false)
  const [editing, setEditing] = useState<SysDept | null>(null)

  const canAdd = useAuth('system:dept:add')
  const canEdit = useAuth('system:dept:edit')
  const canRemove = useAuth('system:dept:remove')

  const loadData = useCallback(async () => {
    try {
      setLoading(true)
      const resp = await getSysDeptTree({ isAll: true })
      if (resp.success) {
        setTreeData(resp.result || [])
      } else {
        messageApi.open({ type: 'error', content: resp.errorMsg })
      }
    } finally {
      setLoading(false)
    }
  }, [messageApi])

  useEffect(() => {
    loadData()
  }, [loadData])

  // 拍平 + 默认全部收起 + 整棵子树折叠
  const flatTable = useTreeTable<SysDept>({
    tree: treeData,
    filtered: Boolean(filters.name || filters.status),
    filterKey: filters,
    getId: item => item.id,
    getParentId: item => item.parentId || '0',
    match: item => {
      const nameHit = !filters.name || item.name.includes(filters.name)
      const statusHit = !filters.status || String(item.status) === filters.status
      return nameHit && statusHit
    },
  })
  const rows = flatTable.rows as unknown as FlatDept[]

  const parentOptions = useMemo(
    () => [{ value: '0', title: '顶级部门' }, ...toParentOptions(treeData, editing?.id)],
    [treeData, editing],
  )

  const openAdd = (parentId?: string) => {
    setEditing(null)
    editForm.resetFields()
    editForm.setFieldsValue({ parentId: parentId || '0', sort: 0, status: 1, deptType: 1 })
    setModalOpen(true)
  }

  const openEdit = (record: SysDept) => {
    setEditing(record)
    editForm.resetFields()
    editForm.setFieldsValue({
      parentId: record.parentId || '0',
      name: record.name,
      deptType: record.deptType || 1,
      sort: record.sort,
      leader: record.leader,
      phone: record.phone,
      email: record.email,
      status: record.status,
    })
    setModalOpen(true)
  }

  const handleSubmit = async () => {
    const values = await editForm.validateFields()
    const resp = editing
      ? await updateSysDept({ ...values, id: editing.id })
      : await addSysDept(values)
    if (resp.success) {
      messageApi.open({ type: 'success', content: editing ? '修改成功' : '新增成功' })
      setModalOpen(false)
      loadData()
    } else {
      messageApi.open({ type: 'error', content: resp.errorMsg })
    }
  }

  const handleDelete = async (id: string) => {
    const resp = await deleteSysDept([id])
    if (resp.success) {
      messageApi.open({ type: 'success', content: '删除成功' })
      loadData()
    } else {
      messageApi.open({ type: 'error', content: resp.errorMsg })
    }
  }

  const columns: ColumnsType<FlatDept> = [
    {
      title: '',
      key: 'fold',
      width: 40,
      render: (_, record) => (record.hasChildren
        ? (
          <Button
            type="text"
            size="small"
            style={{ padding: 0, width: 20, height: 20 }}
            data-node={record.id}
            data-name={record.name}
            aria-label={flatTable.isExpanded(record.id) ? '折叠' : '展开'}
            onClick={() => flatTable.toggleExpand(record)}
          >
            {flatTable.isExpanded(record.id) ? <MinusSquareOutlined /> : <PlusSquareOutlined />}
          </Button>
        )
        : null),
    },
    {
      title: '部门名称',
      dataIndex: 'name',
      width: 260,
      render: (value: string, record) => (
        <span style={{ paddingLeft: record.level * 24 }}>
          {value}
        </span>
      ),
    },
    {
      title: '节点类型',
      dataIndex: 'deptType',
      width: 100,
      render: (value: number) => <Tag color="processing">{deptTypeLabel(value)}</Tag>,
    },
    { title: '负责人', dataIndex: 'leader', width: 140, render: (value: string) => value || '-' },
    { title: '联系电话', dataIndex: 'phone', width: 160, render: (value: string) => value || '-' },
    { title: '邮箱', dataIndex: 'email', width: 200, render: (value: string) => value || '-' },
    { title: '排序', dataIndex: 'sort', width: 80 },
    {
      title: '状态',
      dataIndex: 'status',
      width: 90,
      render: (value: number) => (value === 1 ? <Tag color="success">正常</Tag> : <Tag color="default">停用</Tag>),
    },
    {
      title: '创建时间',
      dataIndex: 'created',
      width: 180,
      render: (value: number) => (value ? dayjs(value).format('YYYY-MM-DD HH:mm:ss') : '-'),
    },
    {
      title: '操作',
      key: 'action',
      width: 200,
      fixed: 'right',
      render: (_, record) => (
        <Space size={4}>
          {canAdd && <Button type="link" size="small" onClick={() => openAdd(record.id)}>新增下级</Button>}
          {canEdit && <Button type="link" size="small" onClick={() => openEdit(record)}>编辑</Button>}
          {canRemove && (
            <Popconfirm title="确认删除该部门？" onConfirm={() => handleDelete(record.id)} okText="确定" cancelText="取消">
              <Button type="link" size="small" danger>删除</Button>
            </Popconfirm>
          )}
        </Space>
      ),
    },
  ]

  return (
    <Card className="admin-page">
      {contextHolder}
      <Form
        form={searchForm}
        layout="inline"
        style={{ marginBottom: 16, rowGap: 8 }}
        onFinish={values => setFilters({ name: values.name, status: values.status })}
      >
        <Form.Item name="name" label="部门名称">
          <Input placeholder="请输入部门名称" allowClear style={{ width: 180 }} />
        </Form.Item>
        <Form.Item name="status" label="状态">
          <Select placeholder="全部" allowClear style={{ width: 120 }}>
            <Select.Option value="1">正常</Select.Option>
            <Select.Option value="0">停用</Select.Option>
          </Select>
        </Form.Item>
        <Form.Item>
          <Space>
            <Button type="primary" icon={<SearchOutlined />} htmlType="submit">查询</Button>
            <Button
              icon={<ReloadOutlined />}
              onClick={() => {
                searchForm.resetFields()
                setFilters({})
              }}
            >
              重置
            </Button>
          </Space>
        </Form.Item>
      </Form>

      <Space style={{ marginBottom: 12 }}>
        {canAdd && <Button type="primary" icon={<PlusOutlined />} onClick={() => openAdd()}>新增部门</Button>}
      </Space>

      <Table<FlatDept>
        rowKey="id"
        size="small"
        bordered
        loading={loading}
        columns={columns}
        dataSource={rows}
        rowClassName={record => `tree-row tree-level-${record.level}`}
        onRow={record => ({ 'data-parent': record.parentKey, 'data-key': record.id } as React.HTMLAttributes<HTMLElement>)}
        pagination={false}
        scroll={{ x: 1200 }}
      />

      <Drawer
        open={modalOpen}
        title={editing ? '编辑部门' : '新增部门'}
        placement="right"
        width={560}
        onClose={() => setModalOpen(false)}
        footer={
          <Space style={{ display: 'flex', justifyContent: 'flex-end' }}>
            <Button onClick={() => setModalOpen(false)}>取消</Button>
            <Button type="primary" onClick={handleSubmit}>确定</Button>
          </Space>
        }
      >
        <Form form={editForm} layout="vertical">
          <Form.Item label="上级部门" name="parentId">
            <Select options={parentOptions} placeholder="请选择上级部门" showSearch optionFilterProp="title" />
          </Form.Item>
          <Form.Item label="部门名称" name="name" rules={[{ required: true, message: '请输入部门名称' }]}>
            <Input placeholder="请输入部门名称" />
          </Form.Item>
          <Form.Item label="节点类型" name="deptType" initialValue={1}>
            <Select options={DEPT_TYPE_OPTIONS} placeholder="请选择节点类型" />
          </Form.Item>
          <Row gutter={12}>
            <Col span={12}>
              <Form.Item label="负责人" name="leader">
                <Input placeholder="请输入负责人" />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item label="联系电话" name="phone">
                <Input placeholder="请输入联系电话" />
              </Form.Item>
            </Col>
          </Row>
          <Row gutter={12}>
            <Col span={12}>
              <Form.Item label="邮箱" name="email">
                <Input placeholder="请输入邮箱" />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item label="显示排序" name="sort">
                <InputNumber min={0} style={{ width: '100%' }} />
              </Form.Item>
            </Col>
          </Row>
          <Form.Item label="部门状态" name="status">
            <Radio.Group>
              <Radio value={1}>正常</Radio>
              <Radio value={0}>停用</Radio>
            </Radio.Group>
          </Form.Item>
        </Form>
      </Drawer>
    </Card>
  )
}

export default SysDeptPage
