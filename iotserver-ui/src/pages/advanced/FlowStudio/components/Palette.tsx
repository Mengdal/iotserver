import React, { useMemo, useState } from 'react'
import { Empty, Input, Tooltip } from 'antd'
import { SearchOutlined } from '@ant-design/icons'
import { t } from '@/i18n'
import { CATEGORY_LABEL, CATEGORY_ORDER, NODE_DEFS } from '../model/nodes'
import type { NodeDef } from '../model/types'

export interface PaletteProps {
  onAdd: (type: string) => void
}

const Palette: React.FC<PaletteProps> = ({ onAdd }) => {
  const [keyword, setKeyword] = useState('')

  const groups = useMemo(() => {
    const query = keyword.trim().toLowerCase()
    return CATEGORY_ORDER.map(category => ({
      category,
      items: NODE_DEFS.filter(def => {
        if (def.category !== category) return false
        if (!query) return true
        return `${def.displayName} ${def.type} ${def.description}`.toLowerCase().includes(query)
      }),
    })).filter(group => group.items.length > 0)
  }, [keyword])

  const handleDragStart = (event: React.DragEvent, def: NodeDef) => {
    event.dataTransfer.setData('application/x-iotserver-flow-node', def.type)
    event.dataTransfer.effectAllowed = 'move'
  }

  return (
    <aside className="fs-palette" aria-label={t('节点面板')}>
      <div className="fs-palette__head">
        <div className="fs-palette__title">{t('节点面板')}</div>
        <Input
          size="small"
          allowClear
          prefix={<SearchOutlined />}
          placeholder={t('搜索节点')}
          value={keyword}
          onChange={event => setKeyword(event.target.value)}
        />
      </div>
      <div className="fs-palette__body">
        {groups.length === 0 && <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={t('没有匹配的节点')} />}
        {groups.map(group => (
          <div className="fs-palette__group" key={group.category}>
            <div className="fs-palette__group-title">{t(CATEGORY_LABEL[group.category])}</div>
            {group.items.map(def => (
              <Tooltip key={def.type} title={def.description} placement="right" mouseEnterDelay={0.4}>
                <div
                  className="fs-palette__item"
                  draggable
                  onDragStart={event => handleDragStart(event, def)}
                  onClick={() => onAdd(def.type)}
                  role="button"
                  tabIndex={0}
                  onKeyDown={event => {
                    if (event.key === 'Enter' || event.key === ' ') onAdd(def.type)
                  }}
                >
                  <span className="fs-palette__icon" style={{ background: def.accent }}>{def.icon}</span>
                  <span className="fs-palette__text">
                    <span className="fs-palette__name">{t(def.displayName)}</span>
                    <span className="fs-palette__desc">{t(def.description)}</span>
                  </span>
                </div>
              </Tooltip>
            ))}
          </div>
        ))}
      </div>
      <div className="fs-palette__foot">{t('点击或拖拽添加节点')}</div>
    </aside>
  )
}

export default Palette
