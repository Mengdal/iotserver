import React, { useCallback, useEffect, useMemo, useState } from 'react'
import {
  Alert,
  Button,
  Card,
  Col,
  Divider,
  Empty,
  Form,
  Input,
  InputNumber,
  Row,
  Space,
  Spin,
  Typography,
  message,
} from 'antd'
import { ReloadOutlined } from '@ant-design/icons'
import { getSysConfigs, saveSysConfigs, SysConfig } from '@/api'
import { isPlatformAdmin, useAuth } from '@/utils/auth'
import { useBrand } from '@/components/Brand'
import ImageUploader from '@/components/ImageUploader'
import { t, tt } from '@/i18n'

const { useMessage } = message
const { Text, Title } = Typography

/** 图片项推荐尺寸提示（对齐参考站文案） */
const SIZE_HINT = {
  logo: t('推荐尺寸 200*200'),
  favicon: t('推荐尺寸：64*64'),
  background: t('推荐尺寸：800*600'),
}

interface BrandField {
  key: string
  label: string
  type: 'text' | 'image'
  placeholder?: string
  hint?: string
  boxWidth?: number
  boxHeight?: number
}

const TEXT_FIELDS: BrandField[] = [
  { key: 'platform.name', label: t('系统名称'), type: 'text', placeholder: t('请输入系统名称') },
  { key: 'platform.copyright', label: 'copyright', type: 'text', placeholder: t('请输入 copyright') },
]

const IMAGE_FIELDS: BrandField[] = [
  { key: 'platform.logo', label: t('系统 logo'), type: 'image', hint: SIZE_HINT.logo, boxWidth: 120, boxHeight: 120 },
  { key: 'platform.favicon', label: t('浏览器标签'), type: 'image', hint: SIZE_HINT.favicon, boxWidth: 120, boxHeight: 120 },
]

const BG_FIELD: BrandField = {
  key: 'platform.loginBackground',
  label: t('背景图'),
  type: 'image',
  hint: SIZE_HINT.background,
  boxWidth: 260,
  boxHeight: 195,
}

const SysSetPage: React.FC = () => {
  const [messageApi, contextHolder] = useMessage()
  const [form] = Form.useForm()
  const brand = useBrand()

  const [loading, setLoading] = useState(false)
  const [saving, setSaving] = useState(false)
  const [items, setItems] = useState<SysConfig[]>([])

  const canEdit = useAuth('system:config:edit')
  // 系统配置为平台级功能：租户用户菜单已屏蔽，此处兜底 URL 直进
  const [platformAdmin] = useState(() => isPlatformAdmin())

  const loadData = useCallback(async () => {
    try {
      setLoading(true)
      const resp = await getSysConfigs({ isAll: true })
      if (resp.success) {
        const list = resp.result.list || []
        setItems(list)
        const values: Record<string, any> = {}
        list.forEach(item => {
          if (item.type === 'bool') {
            values[item.configKey] = item.configValue === 'true'
          } else if (item.type === 'number') {
            values[item.configKey] = item.configValue === '' ? undefined : Number(item.configValue)
          } else {
            values[item.configKey] = item.configValue
          }
        })
        form.setFieldsValue(values)
      } else {
        messageApi.open({ type: 'error', content: resp.errorMsg })
      }
    } finally {
      setLoading(false)
    }
  }, [form, messageApi])

  useEffect(() => {
    if (!platformAdmin) {
      return
    }
    loadData()
  }, [loadData, platformAdmin])

  const itemByKey = useMemo(() => {
    const map = new Map<string, SysConfig>()
    items.forEach(item => map.set(item.configKey, item))
    return map
  }, [items])

  // 品牌区之外的配置（当前只有安全配置）
  const otherGroups = useMemo(() => {
    const brandKeys = new Set([
      ...TEXT_FIELDS.map(f => f.key),
      ...IMAGE_FIELDS.map(f => f.key),
      BG_FIELD.key,
    ])
    return items.filter(item => !brandKeys.has(item.configKey))
  }, [items])

  const handleSave = async () => {
    try {
      const values = await form.validateFields()
      const payload = items.map(item => {
        const value = values[item.configKey]
        let configValue = ''
        if (typeof value === 'boolean') {
          configValue = value ? 'true' : 'false'
        } else if (value === undefined || value === null) {
          configValue = ''
        } else {
          configValue = String(value)
        }
        return { configKey: item.configKey, configValue }
      })
      setSaving(true)
      const resp = await saveSysConfigs(payload)
      if (resp.success) {
        messageApi.open({ type: 'success', content: t('保存成功，即将刷新页面') })
        await loadData()
        // 品牌信息立即生效（名称 / Logo / 浏览器标签 / 背景图），无需手动刷新
        await brand.reload()
      } else {
        messageApi.open({ type: 'error', content: resp.errorMsg })
      }
    } catch (error) {
      // 表单校验未通过
    } finally {
      setSaving(false)
    }
  }

  /** 图片项：label 带「推荐尺寸」提示，右侧为大预览框 */
  const renderImageItem = (field: BrandField) => {
    // 后端配置项自带中文名称（系统 Logo、登录页背景图…），词条里有对应英文时跟着语言变
    const label = tt(itemByKey.get(field.key)?.name) || field.label
    return (
      <Form.Item
        label={(
          <Space direction="vertical" size={0}>
            <span>{label}</span>
            <Text type="secondary" style={{ fontSize: 12, fontWeight: 'normal' }}>{field.hint}</Text>
          </Space>
        )}
        name={field.key}
      >
        <ImageUploader
          width={field.boxWidth || 120}
          height={field.boxHeight || 120}
          disabled={!canEdit}
        />
      </Form.Item>
    )
  }

  /** 平台级配置常驻提示 */
  const platformNotice = (
    <Alert
      type="info"
      showIcon
      message={t('系统配置为平台级（全局唯一），保存后对所有租户生效；如需按租户区分，请使用各业务模块自己的配置。')}
      style={{ marginBottom: 16 }}
    />
  )

  // 租户用户菜单已屏蔽本页，此处仅兜底 URL 直进：不请求数据
  if (!platformAdmin) {
    return (
      <Card className="admin-page">
        {contextHolder}
        {platformNotice}
        <Empty description={t('系统配置为平台级功能，仅平台管理员可访问')} />
      </Card>
    )
  }

  return (
    <Card className="admin-page">
      {contextHolder}
      {platformNotice}
      <Spin spinning={loading}>
        <Title level={5} style={{ marginBottom: 4 }}>{t('系统配置')}</Title>
        <Text type="secondary">
          {t('系统设置页面提供系统基础配置项，支持用户对系统的名称、登录页、LOGO等基础信息进行维护管理。')}
        </Text>
        <Divider />

        <Form form={form} layout="vertical" disabled={!canEdit} onFinish={handleSave}>
          <Row gutter={24}>
            {/* 左列：系统名称 / copyright / 系统logo / 浏览器标签 */}
            <Col span={12}>
              <Form.Item name="id" noStyle>
                <div />
              </Form.Item>
              {TEXT_FIELDS.map(field => (
                <Form.Item
                  key={field.key}
                  label={field.label}
                  name={field.key}
                  rules={[{ required: true, message: t('请输入{{label}}', { label: field.label }) }]}
                >
                  <Input placeholder={t('请输入')} />
                </Form.Item>
              ))}
              <Row gutter={16}>
                {IMAGE_FIELDS.map(field => (
                  <Col span={12} key={field.key}>
                    {renderImageItem(field)}
                  </Col>
                ))}
              </Row>
            </Col>

            {/* 右列：背景图 */}
            <Col span={12}>
              {renderImageItem(BG_FIELD)}
            </Col>
          </Row>

          {otherGroups.length > 0 && (
            <>
              <Divider orientation="left" plain>{t('安全配置')}</Divider>
              <Row gutter={16}>
                {otherGroups.map(item => (
                  <Col span={12} key={item.id}>
                    <Form.Item label={tt(item.name)} name={item.configKey} extra={tt(item.remark) || undefined}>
                      {item.type === 'number'
                        ? <InputNumber style={{ width: '100%' }} />
                        : <Input />}
                    </Form.Item>
                  </Col>
                ))}
              </Row>
            </>
          )}

          <Form.Item style={{ marginTop: 8 }}>
            <Space>
              <Button type="primary" htmlType="submit" loading={saving}>{t('保存')}</Button>
              <Button icon={<ReloadOutlined />} onClick={loadData}>{t('重置')}</Button>
            </Space>
          </Form.Item>
        </Form>
      </Spin>
    </Card>
  )
}

export default SysSetPage
