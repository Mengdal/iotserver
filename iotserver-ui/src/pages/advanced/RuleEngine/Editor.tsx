import React, { useCallback, useEffect, useMemo, useState } from 'react'
import {
  Button,
  Card,
  Col,
  Form,
  Input,
  message,
  Row,
  Select,
  Space,
  Spin,
  Switch,
  Tabs,
  Tag,
} from 'antd'
import { SaveOutlined } from '@ant-design/icons'
import MonacoEditor from 'react-monaco-editor'
import { useNavigate, useParams } from 'react-router-dom'
import {
  addRuleEngine,
  editRuleEngine,
  getRuleEngineInfo,
  startRuleEngine,
  stopRuleEngine,
} from '@/api/RuleEngine'
import type { AddRuleEngine, DataRuleEngine, RuleEngineAction, RuleEngineOptions } from '@/api/RuleEngine'
import { getCurrentTenantId, getUserInfo, hasPermission, isPlatformAdmin } from '@/utils/auth'
import { AdvancedRuleStart } from '@/routes/permissions'
import { t } from '@/i18n'
import { ActionPanel } from './components/ActionPanel'
import { DebugPanel } from './components/DebugPanel'
import { RuleOptionsPanel } from './components/RuleOptionsPanel'
import { SqlExamplesDrawer } from './components/SqlExamplesDrawer'
import { DEFAULT_RULE_OPTIONS, MQTT_STREAM } from './editorUtils'
import './RuleEditor.less'

const DEFAULT_SQL = `SELECT *\nFROM ${MQTT_STREAM}`
const NAME_PATTERN = /^[a-zA-Z一-龥][a-zA-Z0-9一-龥_-]{0,31}$/
const ID_PATTERN = /^[A-Za-z0-9_-]{1,64}$/
// 与后端静态路由重名的 ID 会被路由优先匹配，不允许创建
const RESERVED_RULE_IDS = ['export', 'debug', 'batch-start', 'batch-stop', 'validate', 'import']

function mergeOptions (options?: RuleEngineOptions): RuleEngineOptions {
  return {
    ...DEFAULT_RULE_OPTIONS,
    ...(options || {}),
    restartStrategy: {
      ...DEFAULT_RULE_OPTIONS.restartStrategy,
      ...(options?.restartStrategy || {}),
    },
  }
}

function cleanOptions (options?: RuleEngineOptions): RuleEngineOptions {
  const next: RuleEngineOptions = { ...(options || {}) }
  if (!next.cron?.trim()) delete next.cron
  if (!next.duration?.trim()) delete next.duration
  if (next.restartStrategy) {
    const strategy = { ...next.restartStrategy }
    if (!strategy.attempts) strategy.attempts = 0
    next.restartStrategy = strategy
  }
  return next
}

const RuleEditor: React.FC = () => {
  const [form] = Form.useForm()
  const navigate = useNavigate()
  const { id } = useParams<{ id?: string }>()
  const isEdit = !!id
  const [messageApi, contextHolder] = message.useMessage()
  const [loading, setLoading] = useState(false)
  const [saving, setSaving] = useState(false)
  const [status, setStatus] = useState('')
  const [actions, setActions] = useState<RuleEngineAction[]>([])
  const [examplesOpen, setExamplesOpen] = useState(false)
  const canStart = hasPermission(AdvancedRuleStart)
  const [runImmediately, setRunImmediately] = useState(!isEdit && canStart)

  const tenantId = useMemo(() => {
    if (isPlatformAdmin()) return getCurrentTenantId()
    return getUserInfo()?.tenantId || ''
  }, [])

  useEffect(() => {
    if (!isEdit) {
      form.setFieldsValue({
        id: '',
        name: '',
        tags: [],
        description: '',
        filter: { sql: DEFAULT_SQL },
        options: mergeOptions(),
      })
      setActions([])
      setStatus('')
      setRunImmediately(canStart)
      return
    }
    let cancelled = false
    setLoading(true)
    getRuleEngineInfo(id as string)
      .then(resp => {
        if (cancelled || !resp.success || !resp.result) {
          if (!cancelled) messageApi.error(resp.errorMsg || t('规则加载失败'))
          return
        }
        const rule: DataRuleEngine = resp.result
        setActions(rule.actions || [])
        setStatus(rule.status)
        setRunImmediately(false)
        form.setFieldsValue({
          id: rule.id,
          name: rule.name,
          tags: rule.tags || [],
          description: rule.description,
          filter: { sql: rule.filter?.sql || DEFAULT_SQL },
          options: mergeOptions(rule.options),
        })
      })
      .catch(() => {
        if (!cancelled) messageApi.error(t('规则加载失败'))
      })
      .finally(() => {
        if (!cancelled) setLoading(false)
      })
    return () => { cancelled = true }
  }, [canStart, form, id, isEdit, messageApi])

  const goBack = useCallback(() => {
    navigate('/advanced/rule/engine')
  }, [navigate])

  const restartRule = async () => {
    if (!id) return
    try {
      const resp = await startRuleEngine(id)
      if (resp.success) {
        setStatus('running')
      } else {
        messageApi.error(resp.errorMsg || t('恢复规则运行失败'))
      }
    } catch (error) {
      messageApi.error(t('恢复规则运行失败'))
    }
  }

  const handleSave = async () => {
    let values: any
    try {
      values = await form.validateFields()
    } catch (error) {
      return
    }
    if (actions.length === 0) {
      messageApi.error(t('请至少添加一个应用'))
      return
    }
    const payload: AddRuleEngine = {
      id: values.id || undefined,
      name: values.name,
      description: values.description || '',
      tags: values.tags || [],
      actions: actions.map(action => ({ resource_id: action.resource_id })),
      filter: { sql: values.filter?.sql || '' },
      options: cleanOptions(values.options),
      run_immediately: runImmediately,
    }
    // 运行中的规则不允许直接更新：保存时先停止，成功后用新定义恢复运行。
    // 不提前在打开编辑页时停止，避免用户只是看看/跳转就把规则留在停止状态。
    const wasRunning = isEdit && status === 'running' && canStart
    let stopped = false
    setSaving(true)
    try {
      if (wasRunning && id) {
        const stopResp = await stopRuleEngine(id)
        if (!stopResp.success) {
          messageApi.error(stopResp.errorMsg || t('停止规则失败'))
          return
        }
        stopped = true
        setStatus('stopped')
      }
      const resp = isEdit
        ? await editRuleEngine({ ...payload, id: id as string, run_immediately: wasRunning || runImmediately })
        : await addRuleEngine(payload)
      if (!resp.success) {
        if (stopped) await restartRule()
        messageApi.error(resp.errorMsg || t('保存失败'))
        return
      }
      messageApi.success(isEdit ? t('修改成功') : t('添加成功'))
      goBack()
    } catch (error) {
      if (stopped) await restartRule()
      messageApi.error(t('保存失败'))
    } finally {
      setSaving(false)
    }
  }

  const running = status === 'running'

  return (
    <div className="rule-editor-page">
      {contextHolder}
      <div className="rule-editor-header">
        <div className="rule-editor-heading">
          <div className="rule-editor-title">
            {isEdit ? t('编辑规则') : t('新建规则')}
            {isEdit && <Tag color={running ? 'green' : 'default'}>{running ? t('运行中') : t('已停止')}</Tag>}
          </div>
        </div>
      </div>

      <Spin spinning={loading} className="rule-editor-spin">
        <Form form={form} layout="vertical" className="rule-editor-form">
          <Row gutter={16} className="rule-editor-top">
            <Col xs={24} xl={16}>
              <Card size="small" className="rule-editor-card">
                <Row gutter={16}>
                  <Col xs={24} md={8}>
                    <Form.Item
                      label={t('规则 ID')}
                      name="id"
                      rules={isEdit
                        ? []
                        : [
                          { required: true, message: t('请输入规则 ID') },
                          {
                            validator: (_, value) => {
                              if (!value || !ID_PATTERN.test(value)) {
                                return Promise.reject(new Error(t('1-64 位字母、数字、下划线或中划线')))
                              }
                              if (RESERVED_RULE_IDS.includes(String(value).toLowerCase())) {
                                return Promise.reject(new Error(t('规则 ID 不能使用保留字 export、debug、batch-start、batch-stop、validate、import')))
                              }
                              return Promise.resolve()
                            },
                          },
                        ]}
                    >
                      <Input placeholder={t('例如 rule_001')} disabled={isEdit} maxLength={64} />
                    </Form.Item>
                  </Col>
                  <Col xs={24} md={10}>
                    <Form.Item
                      label={t('名称')}
                      name="name"
                      rules={[
                        { required: true, message: t('请输入规则名称') },
                        {
                          validator: (_, value) => {
                            if (!value || NAME_PATTERN.test(value)) return Promise.resolve()
                            return Promise.reject(new Error(t('1-32位字符，支持中文、英文、数字及特殊字符_-，必须以英文或中文字符开头')))
                          },
                        },
                      ]}
                    >
                      <Input placeholder={t('请输入规则名称')} maxLength={32} />
                    </Form.Item>
                  </Col>
                  <Col xs={24} md={6}>
                    <Form.Item label={t('立即运行')}>
                      <Switch
                        checked={(running || runImmediately) && canStart}
                        onChange={setRunImmediately}
                        checkedChildren={t('是')}
                        unCheckedChildren={t('否')}
                        disabled={running || !canStart}
                      />
                    </Form.Item>
                  </Col>
                </Row>
                <Form.Item label={t('规则标签')} name="tags">
                  <Select
                    mode="tags"
                    tokenSeparators={[',']}
                    placeholder={t('输入标签后回车，例如 温控、告警')}
                    maxTagTextLength={32}
                  />
                </Form.Item>

                <div className="rule-sql-header">
                  <span className="rule-sql-title">{t('SQL 编辑器')}</span>
                  <Button type="link" size="small" onClick={() => setExamplesOpen(true)}>{t('SQL 示例')}</Button>
                </div>
                <Form.Item
                  name={['filter', 'sql']}
                  className="rule-sql-form-item"
                  rules={[{ required: true, message: t('请填写规则 SQL') }]}
                >
                  <MonacoEditor
                    height={200}
                    language="sql"
                    theme="vs"
                    options={{
                      minimap: { enabled: false },
                      fontSize: 13,
                      lineNumbers: 'on',
                      wordWrap: 'on',
                      scrollBeyondLastLine: false,
                      automaticLayout: true,
                      tabSize: 2,
                      renderLineHighlight: 'line',
                      fixedOverflowWidgets: true,
                      // 编辑器内容不足一屏时，滚轮继续交给页面，避免光标在 SQL 里时页面滑不动
                      scrollbar: { alwaysConsumeMouseWheel: false, verticalScrollbarSize: 8, horizontalScrollbarSize: 8 },
                    }}
                  />
                </Form.Item>
              </Card>
            </Col>
            <Col xs={24} xl={8}>
              <Card size="small" className="rule-editor-card rule-action-card-wrapper">
                <ActionPanel
                  actions={actions}
                  onChange={setActions}
                  disabled={saving || running}
                  onUseSql={(sql) => form.setFieldsValue({ filter: { sql } })}
                />
              </Card>
            </Col>
          </Row>

          <Card size="small" className="rule-editor-card rule-editor-bottom">
            <Tabs
              className="rule-editor-tabs"
              items={[
                {
                  key: 'debug',
                  label: t('调试规则'),
                  children: (
                    <DebugPanel
                      getSql={() => String(form.getFieldValue(['filter', 'sql']) || '')}
                      tenantId={tenantId}
                      actions={actions}
                    />
                  ),
                },
                {
                  key: 'options',
                  label: t('规则选项'),
                  children: <RuleOptionsPanel />,
                  forceRender: true,
                },
              ]}
            />
          </Card>
        </Form>
      </Spin>

      <div className="rule-editor-footer">
        <Space>
          <Button type="primary" icon={<SaveOutlined />} loading={saving} onClick={handleSave}>
            {t('保存规则')}
          </Button>
          <Button onClick={goBack}>{t('取消')}</Button>
        </Space>
      </div>

      <SqlExamplesDrawer
        open={examplesOpen}
        onClose={() => setExamplesOpen(false)}
        onUse={(sql) => {
          form.setFieldsValue({ filter: { sql } })
          setExamplesOpen(false)
        }}
      />
    </div>
  )
}

export default RuleEditor
