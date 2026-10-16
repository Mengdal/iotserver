import React from 'react'
import { theme } from 'antd'
import './settings.less'

/**
 * 看板组件配置面板的排版构件（纯样式，不含任何业务逻辑）。
 *
 * 约定（各组件 Settings 面板统一遵守）：
 *   - 分组 = 一张卡片：卡头（标题 + 可选 extra） + 卡体，组内行距 12（见 settings.less）；
 *   - 普通行：标签左对齐、固定宽度（labelWidth 可覆盖），控件列撑满剩余宽度；
 *     输入类控件一律 width:100%；标签行高与控件高度（32px）对齐；
 *   - 开关这类短项用 inline：标签在左、控件右对齐；同一行多个控件会一起靠右；
 *   - hint 是次要说明（12px 次要色），换行显示在控件下方；只有 hint、没有控件也合法；
 *   - 分组标题用中性命名（如「数据绑定」），方便以后在组内扩展绑定方式。
 */

/** 标签列宽度（px）：全局统一，改这里即可整体换宽度 */
export const SETTINGS_LABEL_WIDTH = 64
/** 数据绑定条目内的标签列宽（px）：比外层窄一档，用宽度表达层级 */
export const SETTINGS_ITEM_LABEL_WIDTH = 40

export interface SettingRowProps {
  /** 左侧标签；不传则整行只有控件 */
  label?: React.ReactNode
  /** 次要说明：小号灰字，换行在控件下方 */
  hint?: React.ReactNode
  /** 短项（开关等）：标签在左、控件右对齐，而不是左右分栏 */
  inline?: boolean
  /** 覆盖标签列宽（嵌套条目用 SETTINGS_ITEM_LABEL_WIDTH） */
  labelWidth?: number
  className?: string
  style?: React.CSSProperties
  children?: React.ReactNode
}

/** 一行一个配置项：左标签（固定宽度）+ 右控件（撑满） */
export const SettingRow: React.FC<SettingRowProps> = ({
  label, hint, inline, labelWidth = SETTINGS_LABEL_WIDTH, className, style, children,
}) => {
  const { token } = theme.useToken()
  const classes = ['viz-settings__row', inline ? 'viz-settings__row--inline' : '', className]
    .filter(Boolean).join(' ')
  const hintNode = hint
    ? <div className="viz-settings__hint" style={{ color: token.colorTextSecondary }}>{hint}</div>
    : null

  if (inline) {
    return (
      <div className={classes} style={style}>
        {label ? <span className="viz-settings__label" style={{ color: token.colorText }}>{label}</span> : null}
        <div className="viz-settings__control--inline">{children}</div>
        {hintNode}
      </div>
    )
  }

  return (
    <div className={classes} style={style}>
      {label
        ? (
          <span
            className="viz-settings__label"
            style={{ flexBasis: labelWidth, color: token.colorText }}
          >
            {label}
          </span>
        )
        : null}
      <div className="viz-settings__control">
        <div className="viz-settings__control-line">{children}</div>
        {hintNode}
      </div>
    </div>
  )
}

export interface SettingGroupProps {
  /** 卡片头标题；不传且没有 extra 时不渲染卡头（仍是一张卡片） */
  title?: React.ReactNode
  /** 卡片头右侧内容（计数、小动作按钮等） */
  extra?: React.ReactNode
  className?: string
  style?: React.CSSProperties
  children?: React.ReactNode
}

/** 分组卡片：卡头（浅底 + 唯一分隔线）+ 卡体（组内行距 12） */
export const SettingGroup: React.FC<SettingGroupProps> = ({ title, extra, className, style, children }) => {
  const classes = ['viz-settings__group', className].filter(Boolean).join(' ')
  return (
    <section className={classes} style={style}>
      {title || extra
        ? (
          <header className="viz-settings__group-head">
            <span className="viz-settings__group-title">{title}</span>
            {extra ? <span className="viz-settings__group-extra">{extra}</span> : null}
          </header>
        )
        : null}
      <div className="viz-settings__group-body">{children}</div>
    </section>
  )
}

export default SettingGroup
