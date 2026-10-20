import { useCallback, useEffect, useState } from 'react'
import { Form } from 'antd'
import type { FormInstance, PaginationProps } from 'antd'
import type { TableReponseType, TableRequestType, CommonResponse } from '@/types'
import { t } from '@/i18n'

const { useForm } = Form

type PaginationType = Pick<PaginationProps, 'current' | 'pageSize'>

interface PaginationParamsType<T> {
  onRequest(params: TableRequestType & {
    [key: string]: any
  }): Promise<CommonResponse<TableReponseType<T>>>
}

export function useTableRequest<T> ({ onRequest }: PaginationParamsType<T>): {
  form: FormInstance<any>
  loading: boolean
  dataSource: T[]
  pagination: PaginationProps
  reload: () => void
  search: () => void
  reset: () => void
} {
  const [form] = useForm()
  const [total, setTotal] = useState<number>(0)
  const [loading, setLoading] = useState(false)
  const [dataSource, setDataSource] = useState<T[]>([])
  const [pagination, setPagination] = useState<PaginationType>({ current: 1, pageSize: 10 })

  const handleRequest = useCallback(async () => {
    if (typeof onRequest === 'function') {
      try {
        setLoading(true)
        const params = await form.validateFields()
        const { current, pageSize } = pagination
        const { success, result } = await onRequest({ ...params, page: current!, pageSize })
        if (success) {
          // 后端在「无数据」时可能返回 result.list = null（例如没有 code 的历史查询），
          // 统一归一化成空数组，避免调用方 spread/map 时抛错。
          const { list, total } = (result || {}) as any
          const rows = Array.isArray(list) ? list : []
          setDataSource(rows)
          setTotal(Number(total) || 0)
          if (rows.length === 0 && current && current > 1) {
            setPagination({ ...pagination, current: current - 1 })
          }
        }
      } catch (error) {

      } finally {
        setLoading(false)
      }
    }
  }, [pagination, form])

  const search = useCallback(() => {
    setPagination({ ...pagination, current: 1 })
  }, [pagination])

  const reset = useCallback(() => {
    form.resetFields()
    setPagination({ ...pagination, current: 1 })
  }, [pagination, form])

  const reload = useCallback(() => {
    handleRequest()
  }, [handleRequest])

  useEffect(() => {
    handleRequest()
  }, [pagination])

  return {
    form,
    loading,
    dataSource,
    search,
    reset,
    reload,
    pagination: {
      size: 'small',
      showQuickJumper: true,
      showSizeChanger: true,
      showTotal: (total) => t('共 {{count}} 条', { count: total }),
      total,
      current: pagination.current,
      pageSize: pagination.pageSize,
      onChange (current, pageSize) {
        setPagination({ current, pageSize })
      },
      onShowSizeChange (current, pageSize) {
        setPagination({ current, pageSize })
      },
    },
  }
}
