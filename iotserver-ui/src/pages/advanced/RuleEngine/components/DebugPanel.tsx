import React, { useCallback, useEffect, useRef, useState } from 'react'
import { Button, Input, InputNumber, Modal, Space, Switch, Tooltip, Typography, message } from 'antd'
import {
  ArrowDownOutlined,
  BugOutlined,
  ClearOutlined,
  PauseOutlined,
  PlayCircleOutlined,
  QuestionCircleOutlined,
  StopOutlined,
} from '@ant-design/icons'
import MonacoEditor from 'react-monaco-editor'
import { debugRuleEngine, stopRuleEngineDebug, validateRuleEngine } from '@/api/RuleEngine'
import type { RuleEngineAction, RuleEngineDebugActionResult } from '@/api/RuleEngine'
import { getToken, isPlatformAdmin, getCurrentTenantId } from '@/utils/auth'
import { currentLocale, t } from '@/i18n'
import { parseMockPayload, sampleMessage } from '../editorUtils'

const { TextArea } = Input

interface DebugPanelProps {
  getSql: () => string
  tenantId?: string
  actions: RuleEngineAction[]
}

interface OutputLine {
  time: string
  content: string
  kind?: 'action' | 'action-error'
}

function formatActionResult (raw: string): { text: string, ok: boolean } {
  try {
    const result = JSON.parse(raw) as RuleEngineDebugActionResult
    if (!result || typeof result !== 'object') return { text: raw, ok: true }
    const name = result.name || result.resource_id || ''
    const type = result.type || ''
    if (result.ok) {
      if (result.status_code) {
        return {
          text: t('[应用] {{type}} {{name}} 已推送（HTTP {{status}}，{{ms}}ms）', {
            type, name, status: result.status_code, ms: result.duration_ms,
          }),
          ok: true,
        }
      }
      return {
        text: t('[应用] {{type}} {{name}} 已推送（{{ms}}ms）', { type, name, ms: result.duration_ms }),
        ok: true,
      }
    }
    return {
      text: t('[应用] {{type}} {{name}} 推送失败：{{error}}', { type, name, error: result.error || '' }),
      ok: false,
    }
  } catch (error) {
    return { text: raw, ok: true }
  }
}

export const DebugPanel: React.FC<DebugPanelProps> = ({ getSql, tenantId, actions }) => {
  const [messageApi, contextHolder] = message.useMessage()
  const [mockOpen, setMockOpen] = useState(false)
  const [mockText, setMockText] = useState(() => sampleMessage(tenantId || ''))
  const [draftMockText, setDraftMockText] = useState(mockText)
  // 默认使用真实数据源；需要模拟输入时再手动打开
  const [mockEnabled, setMockEnabled] = useState(false)
  const [draftMockEnabled, setDraftMockEnabled] = useState(false)
  const [interval, setIntervalValue] = useState(1000)
  const [loop, setLoop] = useState(true)
  // 可选：模拟 Sink 的 Data Template，对 SQL 输出做格式化
  const [dataTemplate, setDataTemplate] = useState('')
  const [draftTemplate, setDraftTemplate] = useState('')
  const [templateOpen, setTemplateOpen] = useState(false)
  const [running, setRunning] = useState(false)
  const [starting, setStarting] = useState(false)
  const [autoScroll, setAutoScroll] = useState(true)
  const [output, setOutput] = useState<OutputLine[]>([])
  const testIdRef = useRef('')
  const abortRef = useRef<AbortController | null>(null)
  const outputRef = useRef<HTMLDivElement>(null)

  const append = useCallback((content: string, kind?: OutputLine['kind']) => {
    setOutput(prev => {
      const next = [...prev, { time: new Date().toLocaleTimeString(), content, kind }]
      return next.length > 200 ? next.slice(next.length - 200) : next
    })
  }, [])

  useEffect(() => {
    if (!autoScroll) return
    const el = outputRef.current
    if (el) el.scrollTop = el.scrollHeight
  }, [output, autoScroll])

  const stop = useCallback(async (silent = false) => {
    abortRef.current?.abort()
    abortRef.current = null
    const id = testIdRef.current
    testIdRef.current = ''
    setRunning(false)
    if (id) {
      try {
        await stopRuleEngineDebug(id)
      } catch (error) {
        if (!silent) messageApi.error(t('停止调试失败'))
      }
    }
  }, [messageApi])

  useEffect(() => () => {
    abortRef.current?.abort()
    if (testIdRef.current) {
      stopRuleEngineDebug(testIdRef.current)
    }
  }, [])

  const startStream = useCallback(async (id: string) => {
    const controller = new AbortController()
    abortRef.current = controller
    setRunning(true)
    const headers: Record<string, string> = {
      'x-token': getToken() || '',
      'Accept-Language': currentLocale(),
      Accept: 'text/event-stream',
    }
    if (isPlatformAdmin()) {
      const current = getCurrentTenantId()
      if (current) headers['x-tenant-id'] = current
    }
    try {
      const resp = await fetch(`/api/v1/rule-engine/debug/${encodeURIComponent(id)}/stream`, {
        headers,
        signal: controller.signal,
      })
      // 后端失败时返回的是 HTTP 200 的 JSON 错误信封，不能当成空 SSE
      const contentType = resp.headers.get('content-type') || ''
      if (!resp.ok || !contentType.includes('text/event-stream')) {
        const text = await resp.text()
        let detail = text
        try {
          const parsed = JSON.parse(text)
          detail = parsed.errorMsg || parsed.message || text
        } catch (error) {
          // 非 JSON 响应直接使用原文
        }
        throw new Error(detail || `HTTP ${resp.status}`)
      }
      if (!resp.body) throw new Error(t('浏览器不支持流式响应'))
      const reader = resp.body.getReader()
      const decoder = new TextDecoder()
      let buffer = ''
      const handleEvent = (block: string) => {
        const dataLines: string[] = []
        let eventName = ''
        block.split(/\r?\n/).forEach(line => {
          if (line.startsWith('event:')) {
            eventName = line.slice('event:'.length).trim()
          } else if (line.startsWith('data:')) {
            let value = line.slice('data:'.length)
            // SSE 规范：冒号后最多去掉一个空格
            if (value.startsWith(' ')) value = value.slice(1)
            dataLines.push(value)
          }
        })
        if (dataLines.length === 0) return
        const content = dataLines.join('\n')
        if (eventName === 'action') {
          const result = formatActionResult(content)
          append(result.text, result.ok ? 'action' : 'action-error')
          return
        }
        append(content)
      }
      while (true) {
        const { done, value } = await reader.read()
        if (done) break
        buffer += decoder.decode(value, { stream: true })
        const chunks = buffer.split('\n\n')
        buffer = chunks.pop() || ''
        chunks.forEach(handleEvent)
      }
      if (buffer.trim()) handleEvent(buffer)
    } catch (error) {
      if (!controller.signal.aborted) {
        const msg = error instanceof Error ? error.message : String(error)
        append(`[error] ${msg}`)
        messageApi.error(msg)
        // 服务端失败路径会清理试运行规则；这里再兜底一次，避免网络异常时残留
        stopRuleEngineDebug(id).catch(() => {})
      }
    } finally {
      if (abortRef.current === controller) {
        abortRef.current = null
        setRunning(false)
      }
      if (testIdRef.current === id) testIdRef.current = ''
    }
  }, [append, messageApi])

  const handleStart = async () => {
    const sql = getSql()
    if (!sql.trim()) {
      messageApi.warning(t('请先填写规则 SQL'))
      return
    }
    setStarting(true)
    try {
      // 调试前先预校验 SQL；校验失败不创建试运行规则
      const validation = await validateRuleEngine(sql)
      if (!validation.success || !validation.result?.valid) {
        const msg = validation.result?.message || validation.errorMsg || t('SQL 校验失败')
        messageApi.error(msg)
        append(`[error] ${msg}`)
        return
      }

      let data: Array<Record<string, unknown>> = []
      if (mockEnabled) {
        try {
          data = parseMockPayload(mockText)
        } catch (error) {
          messageApi.error(error instanceof Error ? error.message : t('模拟数据必须是合法 JSON'))
          return
        }
      }

      if (testIdRef.current) await stop(true)
      const resp = await debugRuleEngine({
        sql,
        data,
        interval,
        loop,
        data_template: dataTemplate.trim() ? dataTemplate : undefined,
        mock_enabled: mockEnabled,
        // 调试默认按当前规则的应用真实推送；界面上不再提供开关
        execute_actions: actions.length > 0,
        action_resource_ids: actions.map(action => action.resource_id),
      })
      if (!resp.success || !resp.result?.id) {
        messageApi.error(resp.errorMsg || t('启动调试失败'))
        return
      }
      testIdRef.current = resp.result.id
      setOutput([])
      setAutoScroll(true)
      append(`[${t('已启动调试')}] ${resp.result.id}`)
      startStream(resp.result.id)
    } catch (error) {
      messageApi.error(t('启动调试失败'))
    } finally {
      setStarting(false)
    }
  }

  return (
    <>
      {contextHolder}
      <div className="rule-debug-panel">
        <div className="rule-debug-toolbar">
          <Space wrap>
            <Button
              size="small"
              className={!mockEnabled ? 'rule-debug-source-off' : undefined}
              onClick={() => {
                setDraftMockText(mockText)
                setDraftMockEnabled(mockEnabled)
                setMockOpen(true)
              }}
            >
              {!mockEnabled && <span className="rule-mock-status-dot" />}
              {t('模拟数据源')}
            </Button>
            <Button
              size="small"
              onClick={() => {
                setDraftTemplate(dataTemplate)
                setTemplateOpen(true)
              }}
            >
              {t('数据模版')}
            </Button>
            {running
              ? <Button size="small" danger icon={<StopOutlined />} onClick={() => stop()}>{t('停止调试')}</Button>
              : <Button size="small" type="primary" icon={<PlayCircleOutlined />} loading={starting} onClick={handleStart}>{t('开始调试')}</Button>}
          </Space>
          <Space>
            <Tooltip title={autoScroll ? t('暂停滚动') : t('恢复滚动')}>
              <span className="rule-debug-icon-btn">
                <Button
                  size="small"
                  aria-label={autoScroll ? t('暂停滚动') : t('恢复滚动')}
                  icon={autoScroll ? <PauseOutlined /> : <ArrowDownOutlined />}
                  onClick={() => setAutoScroll(value => !value)}
                  disabled={output.length === 0}
                />
              </span>
            </Tooltip>
            <Tooltip title={t('清空日志')}>
              <span className="rule-debug-icon-btn">
                <Button
                  size="small"
                  aria-label={t('清空日志')}
                  icon={<ClearOutlined />}
                  onClick={() => setOutput([])}
                  disabled={output.length === 0}
                />
              </span>
            </Tooltip>
          </Space>
        </div>
        <div className="rule-debug-output" ref={outputRef}>
          {output.length === 0
            ? <div className="rule-debug-empty"><BugOutlined /> {t('运行调试后，这里会实时显示规则输出')}</div>
            : output.map((line, index) => (
              <div className={`rule-debug-line${line.kind ? ` is-${line.kind}` : ''}`} key={index}>
                <span className="rule-debug-time">{line.time}</span>
                <pre>{line.content}</pre>
              </div>
            ))}
        </div>
      </div>
      <Modal
        title={t('模拟数据源')}
        open={mockOpen}
        onCancel={() => setMockOpen(false)}
        onOk={() => {
          setMockText(draftMockText)
          setMockEnabled(draftMockEnabled)
          setMockOpen(false)
        }}
        okText={t('确定')}
        cancelText={t('取消')}
        width={720}
        destroyOnClose
      >
        <div className="rule-field-label">
          {t('Payload')}
          <Tooltip title={t('填写单个 JSON 对象、JSON 数组，或每行一个 JSON 对象作为 mqtt_stream 的模拟输入；租户规则的数据需要带当前租户的 tenantId。')}>
            <QuestionCircleOutlined className="rule-help-icon" />
          </Tooltip>
        </div>
        {!draftMockEnabled && (
          <Typography.Paragraph type="secondary" className="rule-mock-off-note">
            {t('模拟数据源已关闭，调试将使用真实数据源。')}
          </Typography.Paragraph>
        )}
        <TextArea
          rows={12}
          disabled={!draftMockEnabled}
          value={draftMockText}
          onChange={event => setDraftMockText(event.target.value)}
          spellCheck={false}
          style={{ fontFamily: 'Menlo, Monaco, Consolas, monospace', fontSize: 12 }}
        />
        <div className="rule-mock-options">
          <span className="rule-mock-option">
            <span className="rule-mock-option-label">{t('发送间隔')}：</span>
            <InputNumber
              min={100}
              max={10000}
              step={100}
              disabled={!draftMockEnabled}
              value={interval}
              onChange={value => setIntervalValue(Number(value || 1000))}
              addonAfter="ms"
            />
          </span>
          <span className="rule-mock-option">
            <span className="rule-mock-option-label">{t('循环发送')}：</span>
            <Switch disabled={!draftMockEnabled} checked={loop} onChange={setLoop} />
          </span>
          <span className="rule-mock-option">
            <span className="rule-mock-option-label">{t('启用模拟数据源')}：</span>
            <Switch checked={draftMockEnabled} onChange={setDraftMockEnabled} />
          </span>
        </div>
      </Modal>
      <Modal
        title={t('数据模版')}
        open={templateOpen}
        onCancel={() => setTemplateOpen(false)}
        onOk={() => {
          setDataTemplate(draftTemplate)
          setTemplateOpen(false)
        }}
        okText={t('保存')}
        cancelText={t('取消')}
        width={720}
        destroyOnClose
      >
        <div className="rule-field-label">{t('输入数据模版')}</div>
        <Typography.Paragraph type="secondary">
          {t('可选项，模拟 Sink 中的 Data Template，对 SQL 语句处理过后的输出，进行数据格式修改')}
        </Typography.Paragraph>
        <div className="rule-template-editor">
          <MonacoEditor
            height={280}
            language="plaintext"
            theme="vs"
            value={draftTemplate}
            onChange={value => setDraftTemplate(value)}
            options={{
              minimap: { enabled: false },
              fontSize: 12,
              lineNumbers: 'on',
              wordWrap: 'on',
              scrollBeyondLastLine: false,
              automaticLayout: true,
              tabSize: 2,
              renderLineHighlight: 'line',
              fixedOverflowWidgets: true,
            }}
          />
        </div>
      </Modal>
    </>
  )
}
