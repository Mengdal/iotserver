import React from 'react'
import { Select, Space, SelectProps } from 'antd'
import {
  AlertOutlined,
  ApartmentOutlined,
  ApiOutlined,
  AppstoreOutlined,
  AreaChartOutlined,
  AuditOutlined,
  BarChartOutlined,
  BellOutlined,
  BookOutlined,
  BugOutlined,
  BuildOutlined,
  CalendarOutlined,
  CloudOutlined,
  CloudServerOutlined,
  ClusterOutlined,
  CodeOutlined,
  ContainerOutlined,
  DashboardOutlined,
  DatabaseOutlined,
  DeploymentUnitOutlined,
  DesktopOutlined,
  DownloadOutlined,
  EnvironmentOutlined,
  ExperimentOutlined,
  FieldTimeOutlined,
  FileOutlined,
  FileSearchOutlined,
  FileTextOutlined,
  FolderOutlined,
  FundOutlined,
  GlobalOutlined,
  HddOutlined,
  HistoryOutlined,
  HomeOutlined,
  IdcardOutlined,
  KeyOutlined,
  LaptopOutlined,
  LineChartOutlined,
  LinkOutlined,
  LockOutlined,
  MailOutlined,
  MenuOutlined,
  MessageOutlined,
  MobileOutlined,
  MonitorOutlined,
  NotificationOutlined,
  PieChartOutlined,
  PictureOutlined,
  ProfileOutlined,
  ProjectOutlined,
  ReadOutlined,
  RobotOutlined,
  SafetyOutlined,
  SaveOutlined,
  SendOutlined,
  SettingOutlined,
  ShareAltOutlined,
  ShopOutlined,
  ShoppingOutlined,
  SlidersOutlined,
  SolutionOutlined,
  SoundOutlined,
  SyncOutlined,
  TableOutlined,
  TagOutlined,
  TagsOutlined,
  TeamOutlined,
  ThunderboltOutlined,
  ToolOutlined,
  UploadOutlined,
  UsbOutlined,
  UserOutlined,
  WifiOutlined,
} from '@ant-design/icons'

interface IconGroup {
  title: string
  icons: Array<[string, React.ReactNode]>
}

/**
 * 内置菜单图标集。
 * key 为 kebab-case 图标名，与后端菜单表 icon 字段（sysmanage 种子数据）保持一致。
 */
const ICON_GROUPS: IconGroup[] = [
  {
    title: '导航与布局',
    icons: [
      ['home', <HomeOutlined />],
      ['dashboard', <DashboardOutlined />],
      ['menu', <MenuOutlined />],
      ['appstore', <AppstoreOutlined />],
      ['global', <GlobalOutlined />],
      ['link', <LinkOutlined />],
      ['send', <SendOutlined />],
      ['project', <ProjectOutlined />],
      ['apartment', <ApartmentOutlined />],
      ['table', <TableOutlined />],
      ['container', <ContainerOutlined />],
    ],
  },
  {
    title: '系统与权限',
    icons: [
      ['setting', <SettingOutlined />],
      ['tool', <ToolOutlined />],
      ['user', <UserOutlined />],
      ['team', <TeamOutlined />],
      ['profile', <ProfileOutlined />],
      ['idcard', <IdcardOutlined />],
      ['solution', <SolutionOutlined />],
      ['safety', <SafetyOutlined />],
      ['lock', <LockOutlined />],
      ['key', <KeyOutlined />],
      ['audit', <AuditOutlined />],
      ['sliders', <SlidersOutlined />],
      ['build', <BuildOutlined />],
      ['bug', <BugOutlined />],
      ['experiment', <ExperimentOutlined />],
      ['robot', <RobotOutlined />],
    ],
  },
  {
    title: '数据与监控',
    icons: [
      ['database', <DatabaseOutlined />],
      ['hdd', <HddOutlined />],
      ['save', <SaveOutlined />],
      ['cloud', <CloudOutlined />],
      ['cloud-server', <CloudServerOutlined />],
      ['cluster', <ClusterOutlined />],
      ['deployment-unit', <DeploymentUnitOutlined />],
      ['api', <ApiOutlined />],
      ['sync', <SyncOutlined />],
      ['history', <HistoryOutlined />],
      ['field-time', <FieldTimeOutlined />],
      ['monitor', <MonitorOutlined />],
      ['bar-chart', <BarChartOutlined />],
      ['line-chart', <LineChartOutlined />],
      ['pie-chart', <PieChartOutlined />],
      ['area-chart', <AreaChartOutlined />],
      ['fund', <FundOutlined />],
      ['thunderbolt', <ThunderboltOutlined />],
      ['alert', <AlertOutlined />],
      ['bell', <BellOutlined />],
      ['notification', <NotificationOutlined />],
      ['sound', <SoundOutlined />],
      ['message', <MessageOutlined />],
      ['mail', <MailOutlined />],
    ],
  },
  {
    title: '设备与文档',
    icons: [
      ['desktop', <DesktopOutlined />],
      ['laptop', <LaptopOutlined />],
      ['mobile', <MobileOutlined />],
      ['wifi', <WifiOutlined />],
      ['usb', <UsbOutlined />],
      ['picture', <PictureOutlined />],
      ['folder', <FolderOutlined />],
      ['file', <FileOutlined />],
      ['file-text', <FileTextOutlined />],
      ['file-search', <FileSearchOutlined />],
      ['read', <ReadOutlined />],
      ['book', <BookOutlined />],
      ['code', <CodeOutlined />],
      ['tag', <TagOutlined />],
      ['tags', <TagsOutlined />],
      ['shop', <ShopOutlined />],
      ['shopping', <ShoppingOutlined />],
      ['calendar', <CalendarOutlined />],
      ['environment', <EnvironmentOutlined />],
      ['upload', <UploadOutlined />],
      ['download', <DownloadOutlined />],
      ['share-alt', <ShareAltOutlined />],
    ],
  },
]

/** 图标名 → 图标元素 */
export const menuIconMap: Record<string, React.ReactNode> = ICON_GROUPS.reduce(
  (acc, group) => {
    group.icons.forEach(([name, icon]) => { acc[name] = icon })
    return acc
  },
  {} as Record<string, React.ReactNode>,
)

export const menuIconOptions: SelectProps['options'] = ICON_GROUPS.map(group => ({
  label: group.title,
  options: group.icons.map(([name]) => ({
    value: name,
    label: (
      <Space size={8}>
        {menuIconMap[name]}
        <span>{name}</span>
      </Space>
    ),
  })),
}))

/** 兜底渲染：图标名不在内置集合中时返回 null，由调用方决定如何展示原始文本 */
export function renderMenuIcon (name?: string): React.ReactNode {
  if (!name) return null
  return menuIconMap[name] || null
}

export interface IconSelectProps {
  value?: string
  onChange?: (value?: string) => void
  placeholder?: string
  disabled?: boolean
}

/** 菜单图标内置选择器：下拉里直接预览图标，避免手写图标名 */
const IconSelect: React.FC<IconSelectProps> = ({
  value,
  onChange,
  placeholder = '请选择图标',
  disabled,
}) => (
  <Select
    allowClear
    showSearch
    value={value}
    disabled={disabled}
    placeholder={placeholder}
    onChange={onChange}
    options={menuIconOptions}
    style={{ width: '100%' }}
    filterOption={(input, option) => String((option as { value?: string })?.value || '')
      .toLowerCase()
      .includes(input.toLowerCase())}
  />
)

export default IconSelect
