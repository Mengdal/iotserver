import dayjs from 'dayjs'
import React, { forwardRef, useEffect, useImperativeHandle, useMemo, useState } from 'react'
import { useParams, useSearchParams } from 'react-router-dom'
import { Button, DatePicker, Empty, Form, Modal, Segmented, Table } from 'antd'
import { Line } from '@ant-design/plots'
// import { ExclamationCircleOutlined } from '@ant-design/icons'
import { DeviceProperty, getHistoryProperty, ReportDaum } from '@/api/device-thingmodel'
import { useTableRequest } from '@/hooks'
import { t } from '@/i18n'

const { RangePicker } = DatePicker
const { useForm } = Form

// const range = (start: number, end: number) => {
//   const result = []
//   for (let i = start; i < end; i++) {
//     result.push(i)
//   }
//   return result
// }

export type EventValue<DateType> = DateType | null;
export type RangeValue<DateType> = [EventValue<DateType>, EventValue<DateType>] | null;

export interface Tab1ModalRef {
  open: (item: DeviceProperty) => void
}

export interface Tab1ModalProps {

}

export const Tab1Modal = forwardRef<Tab1ModalRef, Tab1ModalProps>((props, ref) => {
  const [searchParams] = useSearchParams()
  const params = useParams<{ id: string }>()
  const [form] = useForm()
  // const [dates, setDates] = useState<RangeValue<Dayjs>>()
  const [record, setRecord] = useState<DeviceProperty>()
  // const [dataSource, setDataSource] = useState<ReportDaum[]>([])
  // const [loading, setLoading] = useState<boolean>(false)
  const [isModalOpen, setIsModalOpen] = useState<boolean>(false)
  // 历史数据展示方式：表格 / 折线图
  const [viewMode, setViewMode] = useState<'table' | 'chart'>('table')
  const { loading, dataSource, pagination, search } = useTableRequest<ReportDaum>({
    onRequest: (param) => {
      // 弹窗未打开 / 还没选中属性时不要发历史查询（原来会请求 code=undefined）
      if (!record?.code) {
        return Promise.resolve({ success: true, result: { list: [], total: 0, page: 1, pageSize: 10 } } as any)
      }
      const [start, end] = form.getFieldValue('range') || []
      return getHistoryProperty(params.id!, {
        ...param,
        code: record?.code!,
        start: start?.valueOf()!,
        end: end?.valueOf()!,
      })
    },
  })

  // 图表按时间正序展示；表格保留接口原来的倒序（dataSource 已由 hook 归一化为数组）
  const chartRows = useMemo(() => [...(dataSource || [])]
    .sort((a, b) => a.time - b.time)
    .map(item => ({ time: dayjs(item.time).format('MM-DD HH:mm:ss'), value: item.value })), [dataSource])

  useImperativeHandle(ref, () => {
    return {
      open (item) {
        setRecord(item)
        setIsModalOpen(true)
        search()
      },
    }
  }, [params])

  // const load = useCallback(() => {
  //   const [start, end] = form.getFieldValue('range') || []
  //   setLoading(true)
  //   getHistoryProperty(params.id!, {
  //     code: record?.code!,
  //     start: start?.valueOf()!,
  //     end: end?.valueOf()!,
  //   }).then(resp => {
  //     if (resp.success) {
  //       setDataSource(resp.result.list || [])
  //     }
  //   }).finally(() => {
  //     setLoading(false)
  //   })
  // }, [record?.code, form, params.id])

  // useEffect(() => {
  //   if (isModalOpen) {
  //     load()
  //   }
  // }, [isModalOpen])

  useEffect(() => {
    const start_at = Number(searchParams.get('start_at'))
    const end_at = Number(searchParams.get('end_at'))
    start_at && end_at && form.setFieldValue('range', [dayjs(start_at), dayjs(end_at)])
  }, [])

  return (
    <Modal
      title={t('功能历史数据')}
      open={isModalOpen}
      footer={null}
      width={1000}
      onCancel={() => {
        setIsModalOpen(false)
      }}
    >
      <Form layout="inline" className="mb5">
        <Form.Item label={t('功能名称')}>{record?.name}</Form.Item>
        <Form.Item label={t('标识符')}>{record?.code}</Form.Item>
        <Form.Item label={t('数据类型')}>{record?.data_type}</Form.Item>
      </Form>
      <Form layout="inline" className="mb10" form={form}>
        <Form.Item name="range" initialValue={[dayjs().add(-1, 'hours'), dayjs()]}>
          <RangePicker
            format="YYYY-MM-DD HH:mm:ss"
            placeholder={[t('开始日期'), t('结束日期')]}
            // onCalendarChange={val => setDates(val)}
            onOpenChange={(open) => {
              if (open) {
                form.setFieldValue('range', [])
              } else {
                // setDates(undefined)
              }
            }}
            showTime={{
              hideDisabledOptions: true,
              defaultValue: [dayjs(dayjs().format('YYYY-MM-DD')), dayjs(dayjs().format('YYYY-MM-DD'))],
            }}
            // disabledDate={(current: Dayjs) => {
            //   if (!dates) return false
            //   const tooLate = dates[0] && current.diff(dates[0], 'days') > 1
            //   const tooEarly = dates[1] && dates[1].diff(current, 'days') > 0
            //   return !!tooEarly || !!tooLate
            // }}
            // disabledTime={(event, type) => {
            //   let hour = 0
            //   let minutes = 0
            //   let seconds = 0
            //   if (dates) {
            //     hour = dayjs(dates[0]).hour()
            //     minutes = dayjs(dates[0]).minute()
            //     seconds = dayjs(dates[0]).second()
            //   }

            //   if (type === 'start' || (dates && type === 'end' && dates[0]?.day() === dayjs(event).day())) {
            //     return {
            //       disabledHours: () => [],
            //       disabledMinutes: () => [],
            //       disabledSeconds: () => [],
            //     }
            //   }

            //   return {
            //     disabledHours: () => range(hour + 1, 24),
            //     disabledMinutes: () => range(minutes + 1, 60),
            //     disabledSeconds: () => range(seconds + 1, 60),
            //   }
            // }}
            presets={[
              { label: t('最近1小时'), value: [dayjs().add(-1, 'hours'), dayjs()] },
              { label: t('最近3小时'), value: [dayjs().add(-3, 'hours'), dayjs()] },
              { label: t('最近24小时'), value: [dayjs().add(-24, 'hours'), dayjs()] },
            ]}
            // renderExtraFooter={() => {
            //   return (
            //     <Text type="warning" style={{ fontSize: 12 }}>
            //       <ExclamationCircleOutlined className="ml20 mr10" />
            //       时间查询跨度应选择二十四小时之内，否则数据加载将消耗很长时间
            //     </Text>
            //   )
            // }}
          />
        </Form.Item>
        <Form.Item>
          <Button type="primary"
            onClick={() => {
              search()
            }}
          >{t('查询')}</Button>
          <Button className="ml10" onClick={() => {
            form.resetFields()
          }}>{t('重置')}</Button>
        </Form.Item>
      </Form>

      <div style={{ display: 'flex', justifyContent: 'flex-end', marginBottom: 8 }}>
        <Segmented
          size="small"
          value={viewMode}
          onChange={value => setViewMode(value as 'table' | 'chart')}
          options={[
            { label: t('表格'), value: 'table' },
            { label: t('图表'), value: 'chart' },
          ]}
        />
      </div>

      {viewMode === 'chart'
        ? (
          chartRows.length === 0
            ? <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={t('暂无数据')} />
            : (
              <div style={{ height: 420 }}>
                <Line
                  data={chartRows}
                  xField="time"
                  yField="value"
                  smooth
                  animation={false}
                  autoFit
                  xAxis={{ label: { autoHide: true, autoRotate: false } }}
                  yAxis={{ label: { formatter: (v: string) => v } }}
                  tooltip={{ showMarkers: true }}
                />
              </div>
            )
        )
        : (
          <Table
            bordered
            size="small"
            scroll={{ y: 400 }}
            loading={loading}
            dataSource={dataSource}
            pagination={pagination}
            rowKey="time"
            columns={[
              {
                title: t('时间'),
                key: 'time',
                dataIndex: 'time',
                width: 500,
                render (value) {
                  return dayjs(value).format('YYYY-MM-DD HH:mm:ss')
                },
              },
              { title: t('原始值'), key: 'value', dataIndex: 'value' },
            ]}
          />
        )}
    </Modal>
  )
})

export default Tab1Modal
