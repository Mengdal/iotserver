import React from 'react'
import { Collapse, Form, Input, InputNumber, Select, Switch, Typography } from 'antd'
import { t } from '@/i18n'

const { Panel } = Collapse

export const RuleOptionsPanel: React.FC = () => (
  <Collapse bordered={false} className="rule-options-collapse">
    <Panel header={t('通用')} key="general" forceRender>
      <div className="rule-options-grid">
        <Form.Item label={t('规则描述')} name="description" className="rule-options-span-2">
          <Input maxLength={255} showCount placeholder={t('请输入规则描述')} />
        </Form.Item>
        <Form.Item label={t('线程数')} name={['options', 'concurrency']} tooltip={t('规则运行的并发线程数')}>
          <InputNumber min={1} max={1024} precision={0} style={{ width: '100%' }} />
        </Form.Item>
        <Form.Item label={t('缓存大小')} name={['options', 'bufferLength']} tooltip={t('规则内部缓冲区的消息条数')}>
          <InputNumber min={1} max={10000000} precision={0} style={{ width: '100%' }} />
        </Form.Item>
        <Form.Item label={t('流的 QoS')} name={['options', 'qos']} tooltip={t('规则内部数据流传递的 QoS')}>
          <Select options={[{ value: 0, label: '0' }, { value: 1, label: '1' }, { value: 2, label: '2' }]} />
        </Form.Item>
        <Form.Item label={t('检查点间隔')} name={['options', 'checkpointInterval']} tooltip={t('单位毫秒；0 表示使用 eKuiper 默认值')}>
          <InputNumber min={0} precision={0} style={{ width: '100%' }} />
        </Form.Item>
        <Form.Item label={t('周期性触发策略')} name={['options', 'cron']} tooltip={t('可选，cron 表达式；填写后规则按计划运行')}>
          <Input placeholder="0 0 * * * *" allowClear />
        </Form.Item>
        <Form.Item label={t('运行持续时间')} name={['options', 'duration']} tooltip={t('可选，例如 1h、30m；填写后规则运行指定时长')}>
          <Input placeholder={t('例如 1h')} allowClear />
        </Form.Item>
        <Form.Item label={t('是否开启 DEBUG 日志')} name={['options', 'debug']} valuePropName="checked">
          <Switch checkedChildren={t('是')} unCheckedChildren={t('否')} />
        </Form.Item>
        <Form.Item label={t('是否发送元数据')} name={['options', 'sendMetaToSink']} valuePropName="checked">
          <Switch checkedChildren={t('是')} unCheckedChildren={t('否')} />
        </Form.Item>
        <Form.Item label={t('禁用缓存区满时丢弃数据')} name={['options', 'disableBufferFullDiscard']} valuePropName="checked">
          <Switch checkedChildren={t('是')} unCheckedChildren={t('否')} />
        </Form.Item>
        <Form.Item label={t('是否发送错误')} name={['options', 'sendError']} valuePropName="checked">
          <Switch checkedChildren={t('是')} unCheckedChildren={t('否')} />
        </Form.Item>
        <Form.Item label={t('是否发送空字段')} name={['options', 'sendNilField']} valuePropName="checked">
          <Switch checkedChildren={t('是')} unCheckedChildren={t('否')} />
        </Form.Item>
      </div>
    </Panel>
    <Panel header={t('事件时间')} key="event-time" forceRender>
      <div className="rule-options-grid rule-options-grid-2">
        <Form.Item label={t('是否使用事件时间')} name={['options', 'isEventTime']} valuePropName="checked">
          <Switch checkedChildren={t('是')} unCheckedChildren={t('否')} />
        </Form.Item>
        <Form.Item label={t('事件时间延迟')} name={['options', 'lateTolerance']} tooltip={t('单位毫秒；允许的事件时间迟到范围')}>
          <InputNumber min={0} precision={0} style={{ width: '100%' }} />
        </Form.Item>
      </div>
    </Panel>
    <Panel header={t('规则重试')} key="retry" forceRender>
      <div className="rule-options-grid rule-options-grid-2">
        <Form.Item label={t('最大重试次数')} name={['options', 'restartStrategy', 'attempts']} tooltip={t('规则异常退出后的自动重启次数')}>
          <InputNumber min={0} max={1000} precision={0} style={{ width: '100%' }} />
        </Form.Item>
        <div className="rule-options-note rule-options-span-2">
          <Typography.Text type="secondary">
            {t('当前 eKuiper 2.4.2 仅支持最大重试次数；重试间隔、最大间隔、退避系数由引擎版本决定。')}
          </Typography.Text>
        </div>
      </div>
    </Panel>
  </Collapse>
)
