import React from 'react'
import { Form, Select, Typography } from 'antd'
import { Struct } from './Struct'
import { t } from '@/i18n'

const { Text } = Typography

interface EventFormItemProps {
  showView?: boolean
}

export const EventFormItem: React.FC<EventFormItemProps> = (props) => {
  return (
    <>
      <Form.Item
        label={t('事件类型')}
        name="event_type"
        rules={[{ required: true, message: t('请选择事件类型') }]}
      >
        <Select
          placeholder={t('请选择')}
          options={[
            { label: t('信息'), value: 'info' },
            { label: t('告警'), value: 'alert' },
            { label: t('故障'), value: 'error' },
          ]}
        ></Select>
      </Form.Item>
      <Form.Item
        required
        name="structObj"
        rules={[
          {
            validator (rule, value, callback) {
              if (!value || (Array.isArray(value) && !value.length)) {
                callback(t('请至少添加一个参数'))
              } else if (value.length > 50) {
                callback(t('最多支持50个参数'))
              } else {
                callback()
              }
            },
          },
        ]}
        label={
          <>
            <Text className="mr20">{t('输出参数')}</Text>
            <Text type="secondary">{t('最多支持50个参数')}</Text>
          </>
        }
      >
        <Struct showView={props.showView}/>
      </Form.Item>
    </>
  )
}
