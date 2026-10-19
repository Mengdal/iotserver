import React from 'react'
import { Alert } from 'antd'
import { AppstoreOutlined, LineChartOutlined } from '@ant-design/icons'
import type { VizWidgetManifest } from '../types'
import { t } from '@/i18n'

/**
 * 早期图表组件（type=chart）
 *
 * 旧版本用外部查询引擎（Shaper）画的图，数据源已下线，这里只保留登记：
 * 打开老看板时给出占位提示，让用户自行删除或重建成新的数据类组件。
 */
export const legacyChartManifest: VizWidgetManifest = {
  type: 'chart',
  label: t('图表（已下线）'),
  icon: <LineChartOutlined />,
  defaultSize: { w: 6, h: 6 },
  defaultConfig: () => ({}),
  render: () => (
    <Alert
      type="info"
      showIcon
      message={t('这个图表组件已下线')}
      description={t('原来的数据源已移除。可以删掉它，改用「数值 / 仪表盘 / 折线图 / 表格」重新绑定设备数据。')}
    />
  ),
}

/**
 * 容器组件：本身不显示数据，只承载一层子网格（最多两层嵌套）。
 * 渲染由看板编辑器负责（要递归挂 GridCanvas），这里只登记元信息。
 */
export const containerManifest: VizWidgetManifest = {
  type: 'container',
  label: t('容器'),
  icon: <AppstoreOutlined />,
  // 容器留大一点，方便往里拖
  defaultSize: { w: 12, h: 10 },
  layoutOnly: true,
  defaultConfig: () => ({}),
  render: () => null,
}
