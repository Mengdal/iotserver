import React, { memo } from 'react'
import { Position } from '@xyflow/react'
import * as XYFlowReact from '@xyflow/react'
import { t } from '@/i18n'
import type { Diagnostic, NodeDef } from '../model/types'
import { CATEGORY_COLOR, CATEGORY_LABEL, nodeSubtitle } from '../model/nodes'

export interface FlowNodeData {
  name: string
  def: NodeDef
  config: Record<string, any>
  diagnostics: Diagnostic[]
  [key: string]: unknown
}

// @xyflow/react 12.12 的 d.ts 同时把 Handle 导出成类型，TS 4.8 会判定为 type-only；
// 运行时它仍是组件，这里从模块对象上取值绕过旧版 TypeScript 的解析问题。
const Handle = (XYFlowReact as any).Handle

function portTop (index: number, total: number): string {
  return `${((index + 1) / (total + 1)) * 100}%`
}

const FlowNodeView: React.FC<any> = ({ data, selected }: { data: FlowNodeData, selected?: boolean }) => {
  const { name, def, config, diagnostics } = data
  const accent = def.accent || CATEGORY_COLOR[def.category]
  const errors = diagnostics.filter(item => item.severity === 'error').length
  const warnings = diagnostics.filter(item => item.severity === 'warning').length
  const multipleOutputs = def.outputs.length > 1

  return (
    <div
      className={`fs-node${selected ? ' is-selected' : ''}`}
      style={{ ['--fs-accent' as any]: accent }}
      data-node-type={def.type}
    >
      <div className="fs-node__accent" />
      <div className="fs-node__head">
        <span className="fs-node__icon">{def.icon}</span>
        <span className="fs-node__title" title={name}>{name}</span>
        {(errors > 0 || warnings > 0) && (
          <span
            className={`fs-node__badge ${errors > 0 ? 'is-error' : 'is-warning'}`}
            title={errors > 0 ? `${errors} 个错误` : `${warnings} 个警告`}
          >
            {errors > 0 ? errors : warnings}
          </span>
        )}
      </div>
      <div className="fs-node__sub" title={nodeSubtitle(def, config)}>{nodeSubtitle(def, config)}</div>
      <div className="fs-node__meta">
        <span className="fs-node__category">{t(CATEGORY_LABEL[def.category])}</span>
        <span className="fs-node__type">{def.type}</span>
      </div>

      {def.inputs.map((port, index) => (
        <React.Fragment key={`in-${port.id}`}>
          <Handle
            id={port.id}
            type="target"
            position={Position.Left}
            className="fs-handle fs-handle--in"
            style={{ top: portTop(index, def.inputs.length) }}
            title={`${t(port.label)} · ${port.kind}`}
          />
          {def.inputs.length > 1 && (
            <span className="fs-node__port-label fs-node__port-label--in" style={{ top: portTop(index, def.inputs.length) }}>
              {t(port.label)}
            </span>
          )}
        </React.Fragment>
      ))}
      {def.outputs.map((port, index) => (
        <React.Fragment key={`out-${port.id}`}>
          <Handle
            id={port.id}
            type="source"
            position={Position.Right}
            className="fs-handle fs-handle--out"
            style={{ top: portTop(index, def.outputs.length) }}
            title={`${t(port.label)} · ${port.kind}`}
          />
          {multipleOutputs && (
            <span className="fs-node__port-label fs-node__port-label--out" style={{ top: portTop(index, def.outputs.length) }}>
              {t(port.label)}
            </span>
          )}
        </React.Fragment>
      ))}
    </div>
  )
}

export default memo(FlowNodeView)
