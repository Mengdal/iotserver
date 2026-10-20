import React, { useEffect, useRef, useState } from 'react'
import { Select, SelectProps } from 'antd'
import { CategoryModal, CategoryModalRef } from './CategoryModal'
import { localized } from '@/utils/localized'

/** 外部预设的品类项（如从方案中心跳转创建产品时带来的方案品类） */
export interface CategoryPresetOption {
  id: string
  category_name: string
  category_name_en?: string
  category_key?: string
}

export interface CategorySelectProps<ValueType = any> extends Omit<SelectProps<ValueType | ValueType[]>, 'options' | 'children'> {
  onCategoryKeyChange?: ((category_key: string) => void) | undefined
  /** 预设品类：变化时同步到选择器，避免外部预填后展示空白 */
  presetOption?: CategoryPresetOption
}

export const CategorySelect: React.FC<CategorySelectProps> = (props) => {
  const categoryModalRef = useRef<CategoryModalRef>(null)
  const [innerValue, setInnerValue] = useState<string>()
  const [options, setOptions] = useState<SelectProps['options']>()

  // presetOption 只给本组件用，不能再透传给 Select
  const { presetOption, onCategoryKeyChange, ...selectProps } = props
  /** 已应用过的预设 id：用户手动改选品类后不再被同一预设覆盖 */
  const appliedPresetRef = useRef<string>()

  useEffect(() => {
    if (!presetOption?.id) {
      appliedPresetRef.current = undefined
      return
    }
    if (appliedPresetRef.current === presetOption.id) return
    appliedPresetRef.current = presetOption.id
    setInnerValue(presetOption.id)
    setOptions([{ label: localized(presetOption.category_name, presetOption.category_name_en) || presetOption.id, value: presetOption.id }])
    onCategoryKeyChange?.(presetOption.category_key || presetOption.id)
  }, [presetOption, onCategoryKeyChange])

  return (
    <>
      <CategoryModal
        ref={categoryModalRef}
        onChange={({ id, category_key, category_name, category_name_en }) => {
          setInnerValue(id)
          setOptions([{ label: localized(category_name, category_name_en), value: id }])
          selectProps.onChange?.(id, [])
          onCategoryKeyChange?.(category_key)
        }}
      />
      <Select
        {...selectProps}
        open={false}
        options={options}
        value={innerValue}
        onFocus={() => {
          categoryModalRef.current?.open()
        }}
      />
    </>
  )
}

export default CategorySelect
