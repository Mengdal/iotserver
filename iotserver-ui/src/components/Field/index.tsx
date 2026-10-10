import React from 'react'
import './index.less'

/**
 * 表单行：标签固定宽度左对齐、控件右对齐。
 * 看板的组件配置、文本块配置都用它，保证各行的标签与控件对齐。
 */
export const Field: React.FC<{ label: string, hint?: string, children: React.ReactNode }> = ({ label, hint, children }) => (
  <div className="viz-field">
    <div className="viz-field__label">
      {label}
      {hint ? <span className="viz-field__hint">{hint}</span> : null}
    </div>
    <div className="viz-field__control">{children}</div>
  </div>
)

export default Field
