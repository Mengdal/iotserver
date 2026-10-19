import React, { useEffect, useRef, useState } from 'react'
import { Button, Table } from 'antd'
import { PlusOutlined } from '@ant-design/icons'
import { StructForm, StructFormRef } from './StructForm'
import { t } from '@/i18n'

export interface StructTableData {
  id: string
  code: string
  name: string
  struct_type: string
}

export interface StructProps {
  showView?: boolean
  disabled?: boolean
  value?: StructTableData[]
  onChange?: (value: StructTableData[]) => void
}

export const Struct: React.FC<StructProps> = ({ showView, disabled, value = [], onChange }) => {
  const isTouched = useRef<boolean>(false)
  const structFormRef = useRef<StructFormRef>(null)
  const [dataSource, setDataSource] = useState<StructTableData[]>(value)

  useEffect(() => {
    if (isTouched.current) {
      onChange?.(dataSource)
    } else {
      isTouched.current = true
    }
  }, [dataSource])
  console.log('dataSource', dataSource)

  return (
    <>
      <StructForm
        ref={structFormRef}
        onAdd={(data) => {
          const oldDataIndex = dataSource.findIndex(i => i.id === data.id)
          if (oldDataIndex > -1) {
            const newDataSource = [...dataSource]
            newDataSource.splice(oldDataIndex, 1, data)
            setDataSource(newDataSource)
          } else {
            setDataSource([...dataSource, data])
          }
        }}
      />
      {
        !!dataSource.length && (
          <Table
            size="small"
            className="mb20"
            rowKey="id"
            dataSource={dataSource}
            pagination={false}
            columns={[
              { dataIndex: 'name', title: t('名称') },
              { dataIndex: 'code', title: t('标识符') },
              { dataIndex: 'struct_type', title: t('数据类型') },
              {
                dataIndex: 'action',
                title: t('操作'),
                width: 140,
                render (value, record) {
                  return (
                    <>
                      {showView
                        && <span
                          style={{ color: '#1677ff', cursor: 'pointer' }}
                          className="ant-btn ant-btn-link p0 mr10"
                          onClick={() => {
                            structFormRef.current?.open(record, true)
                          }}
                        >{t('查看')}</span>
                      }
                      <Button
                        type="link"
                        className="p0"
                        disabled={disabled}
                        onClick={() => {
                          structFormRef.current?.open(record)
                        }}
                      >{t('编辑')}</Button>
                      <Button
                        type="link"
                        className="p0 ml10"
                        disabled={disabled}
                        onClick={() => {
                          setDataSource(dataSource.filter(item => item.id !== record.id))
                        }}
                      >{t('删除')}</Button>
                    </>
                  )
                },
              },
            ]}
          ></Table>
        )
      }
      <Button
        type="link"
        icon={<PlusOutlined />}
        style={{ padding: 0, marginTop: -50 }}
        onClick={() => {
          structFormRef.current?.open()
        }}
      >{t('添加参数')}</Button>
    </>
  )
}

export default Struct
