import React from 'react'
import { Button, Collapse, Drawer, Typography } from 'antd'
import { SQL_EXAMPLES } from '../editorUtils'
import { t } from '@/i18n'

interface SqlExamplesDrawerProps {
  open: boolean
  onClose: () => void
  onUse: (sql: string) => void
}

export const SqlExamplesDrawer: React.FC<SqlExamplesDrawerProps> = ({ open, onClose, onUse }) => (
  <Drawer
    title={t('SQL 示例')}
    placement="right"
    width={560}
    open={open}
    onClose={onClose}
    className="rule-sql-examples-drawer"
  >
    <Typography.Paragraph type="secondary">
      {t('选择一个示例快速开始；应用后仍可在 SQL 编辑器里继续修改。保存时平台会自动追加租户隔离条件。')}
    </Typography.Paragraph>
    <Collapse accordion bordered={false}>
      {SQL_EXAMPLES.map(item => (
        <Collapse.Panel header={item.title} key={item.key}>
          <div className="rule-sql-example">
            <Typography.Paragraph type="secondary">{item.description}</Typography.Paragraph>
            <pre className="rule-sql-example-code">{item.sql}</pre>
            <Button type="primary" size="small" onClick={() => onUse(item.sql)}>{t('使用此示例')}</Button>
          </div>
        </Collapse.Panel>
      ))}
    </Collapse>
  </Drawer>
)
