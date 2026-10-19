import React, { forwardRef, useCallback, useImperativeHandle, useState } from 'react'
import { Modal, Table, Tag } from 'antd'
import { thingModelTemplate } from '@/api'
import { t } from '@/i18n'

export interface ThingModelTemplate {
  id: string
  type: string
  name: string
  code: string
  tag: string
  type_spec: string
}

export interface StandardFuncModalRef {
  open: (id: string) => void
}

export interface StandardFuncModalProps {

}

export const StandardFuncModal = forwardRef<StandardFuncModalRef, StandardFuncModalProps>((props, ref) => {
  const [isModalOpen, setIsModalOpen] = useState<boolean>(false)
  const [dataSource, setDataSource] = useState<ThingModelTemplate[]>([])

  useImperativeHandle(ref, () => {
    return {
      open (id) {
        setIsModalOpen(true)
        // 'ECGCard'
        // 'VideoDoorbell'
        thingModelTemplate(id).then(resp => {
          if (resp.success) {
            const { actions, events, properties } = resp.result
            setDataSource(
              [
                ...(properties || []).map(i => ({
                  type: '属性',
                  id: i.id,
                  name: i.name,
                  code: i.code,
                  tag: i.tag,
                  type_spec: i.type_spec.type,
                })),
                ...(events || []).map(i => ({
                  type: '属性',
                  id: i.id,
                  name: i.name,
                  code: i.code,
                  tag: i.tag,
                  type_spec: '-',
                })),
                ...(actions || []).map(i => ({
                  type: '属性',
                  id: i.id,
                  name: i.name,
                  code: i.code,
                  tag: i.tag,
                  type_spec: '-',
                })),
              ],
            )
          }
        })
      },
    }
  }, [])

  const handleClose = useCallback(() => {
    setIsModalOpen(false)
  }, [])

  return (
    <Modal
      title={t('标准功能定义')}
      open={isModalOpen}
      onOk={handleClose}
      onCancel={handleClose}
      okText={t('确定')}
      cancelText={t('关闭')}
      width={700}
    >
      <Table
        rowKey="id"
        dataSource={dataSource}
        columns={[
          {
            title: t('功能类型'),
            key: 'type',
            dataIndex: 'type',
            render (value, record) {
              return (
                <>
                  {value}
                  <Tag color="processing" className="ml10">{record.tag}</Tag>
                </>
              )
            },
          },
          { title: t('功能名称'), key: 'name', dataIndex: 'name' },
          { title: t('标识符'), key: 'code', dataIndex: 'code' },
          { title: t('数据类型'), key: 'type_spec', dataIndex: 'type_spec' },
        ]}
      />

    </Modal>
  )
})

StandardFuncModal.displayName = 'StandardFuncModal'

export default StandardFuncModal
