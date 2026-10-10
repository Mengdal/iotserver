import React, { useCallback, useEffect, useState } from 'react'
import {
  Avatar,
  Button,
  Card,
  Col,
  Descriptions,
  Divider,
  Drawer,
  Form,
  Input,
  Row,
  Space,
  Tag,
  Typography,
  message,
} from 'antd'
import { LockOutlined, SaveOutlined, UserOutlined } from '@ant-design/icons'
import { changePassword, getUserProfile, updateUserProfile, SysUserProfile } from '@/api'
import { clearUser, getUser, setUser } from '@/utils/auth'
import { useNavigate } from 'react-router-dom'

const { Title, Text } = Typography
const { useMessage } = message

const UserProfilePage: React.FC = () => {
  const [messageApi, contextHolder] = useMessage()
  const [form] = Form.useForm()
  const [pwdForm] = Form.useForm()
  const navigate = useNavigate()

  const [loading, setLoading] = useState(false)
  const [saving, setSaving] = useState(false)
  const [profile, setProfile] = useState<SysUserProfile | null>(null)
  const [pwdOpen, setPwdOpen] = useState(false)

  const loadData = useCallback(async () => {
    try {
      setLoading(true)
      const resp = await getUserProfile()
      if (resp.success) {
        setProfile(resp.result)
        form.setFieldsValue({
          username: resp.result.user.username,
          nickname: resp.result.user.nickname,
          email: resp.result.user.email,
          phone: resp.result.user.phone,
        })
      } else {
        messageApi.open({ type: 'error', content: resp.errorMsg })
      }
    } finally {
      setLoading(false)
    }
  }, [form, messageApi])

  useEffect(() => {
    loadData()
  }, [loadData])

  const handleSave = async () => {
    try {
      const values = await form.validateFields()
      setSaving(true)
      const resp = await updateUserProfile({
        nickname: values.nickname,
        email: values.email,
        phone: values.phone,
      })
      if (resp.success) {
        messageApi.open({ type: 'success', content: '保存成功' })
        const stored = getUser()
        if (stored?.user) {
          stored.user = { ...stored.user, nickname: values.nickname, email: values.email, phone: values.phone }
          setUser(stored)
        }
        loadData()
      } else {
        messageApi.open({ type: 'error', content: resp.errorMsg })
      }
    } catch (error) {
      // ignore
    } finally {
      setSaving(false)
    }
  }

  const handleChangePassword = async () => {
    const values = await pwdForm.validateFields()
    const resp = await changePassword({
      currentPassword: values.currentPassword,
      newPassword: values.newPassword,
    })
    if (resp.success) {
      messageApi.open({ type: 'success', content: '修改成功，请重新登录' })
      setPwdOpen(false)
      pwdForm.resetFields()
      setTimeout(() => {
        clearUser()
        navigate('/auth/login')
      }, 1000)
    } else {
      messageApi.open({ type: 'error', content: resp.errorMsg })
    }
  }

  const user = profile?.user

  return (
    <Card className="admin-page" loading={loading}>
      {contextHolder}
      <Row gutter={24}>
        <Col xs={24} md={8}>
          <Card bordered={false} style={{ textAlign: 'center' }}>
            <Avatar size={72} icon={<UserOutlined />} />
            <Title level={4} style={{ marginTop: 12, marginBottom: 4 }}>{user?.nickname || user?.username}</Title>
            <Text type="secondary">{user?.username}</Text>
            <div style={{ marginTop: 12 }}>
              {profile?.admin
                ? <Tag color="gold">超级管理员</Tag>
                : (profile?.roles || []).map(role => <Tag color="blue" key={role}>{role}</Tag>)}
            </div>
            <Divider />
            <Space direction="vertical" style={{ width: '100%' }}>
              <Button icon={<LockOutlined />} block onClick={() => setPwdOpen(true)}>修改密码</Button>
            </Space>
          </Card>
        </Col>
        <Col xs={24} md={16}>
          <Card bordered={false} title="基本信息">
            <Form form={form} layout="vertical" style={{ maxWidth: 520 }}>
              <Form.Item label="账号" name="username">
                <Input disabled />
              </Form.Item>
              <Form.Item label="昵称" name="nickname" rules={[{ required: true, message: '请输入昵称' }]}>
                <Input placeholder="请输入昵称" />
              </Form.Item>
              <Form.Item label="邮箱" name="email">
                <Input placeholder="请输入邮箱" />
              </Form.Item>
              <Form.Item label="手机号" name="phone">
                <Input placeholder="请输入手机号" />
              </Form.Item>
              <Form.Item>
                <Button type="primary" icon={<SaveOutlined />} loading={saving} onClick={handleSave}>保存</Button>
              </Form.Item>
            </Form>
          </Card>

          <Card bordered={false} title="账号信息" style={{ marginTop: 16 }}>
            <Descriptions column={2} size="small">
              <Descriptions.Item label="所属部门">{user?.deptName || '-'}</Descriptions.Item>
              <Descriptions.Item label="角色">
                {(user?.roleNames || []).length === 0 ? '-' : (user?.roleNames || []).join('、')}
              </Descriptions.Item>
              <Descriptions.Item label="最后登录">
                {user?.lastLogin ? new Date(user.lastLogin).toLocaleString() : '-'}
              </Descriptions.Item>
              <Descriptions.Item label="账号状态">
                {user?.status === 1 ? <Tag color="success">正常</Tag> : <Tag color="default">停用</Tag>}
              </Descriptions.Item>
            </Descriptions>
          </Card>

          <Card bordered={false} title="权限标识" style={{ marginTop: 16 }}>
            {(profile?.permissions || []).length === 0
              ? <Text type="secondary">暂无权限数据</Text>
              : (
                <Space size={[6, 6]} wrap>
                  {(profile?.permissions || []).map(perm => <Tag key={perm}>{perm}</Tag>)}
                </Space>
              )}
          </Card>
        </Col>
      </Row>

      <Drawer
        open={pwdOpen}
        title="修改密码"
        placement="right"
        width={420}
        onClose={() => setPwdOpen(false)}
        footer={
          <Space style={{ display: 'flex', justifyContent: 'flex-end' }}>
            <Button onClick={() => setPwdOpen(false)}>取消</Button>
            <Button type="primary" onClick={handleChangePassword}>确定</Button>
          </Space>
        }
      >
        <Form form={pwdForm} layout="vertical">
          <Form.Item label="原密码" name="currentPassword" rules={[{ required: true, message: '请输入原密码' }]}>
            <Input.Password placeholder="请输入原密码" />
          </Form.Item>
          <Form.Item
            label="新密码"
            name="newPassword"
            rules={[{ required: true, message: '请输入新密码' }, { min: 6, message: '密码长度不能少于 6 位' }]}
          >
            <Input.Password placeholder="请输入新密码" />
          </Form.Item>
          <Form.Item
            label="确认密码"
            name="confirmPassword"
            dependencies={['newPassword']}
            rules={[
              { required: true, message: '请输入确认密码' },
              ({ getFieldValue }) => ({
                validator (_, value) {
                  if (!value || getFieldValue('newPassword') === value) return Promise.resolve()
                  return Promise.reject(new Error('两次密码输入不一致'))
                },
              }),
            ]}
          >
            <Input.Password placeholder="请再次输入新密码" />
          </Form.Item>
        </Form>
      </Drawer>
    </Card>
  )
}

export default UserProfilePage
