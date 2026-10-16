/**
 * 权限标识常量 —— 本文件由 `cd iotserver && go run ./tools/gen-permissions` 自动生成，请勿手改。
 * 唯一来源: iotserver/internal/pkg/permissions/permissions.go
 */

export const PlatformTenantList = 'platform:tenant:list'
export const PlatformTenantQuery = 'platform:tenant:query'
export const PlatformTenantAdd = 'platform:tenant:add'
export const PlatformTenantEdit = 'platform:tenant:edit'
export const PlatformTenantRemove = 'platform:tenant:remove'
export const PlatformTenantSwitch = 'platform:tenant:switch'

export const UserList = 'system:user:list'
export const UserQuery = 'system:user:query'
export const UserAdd = 'system:user:add'
export const UserEdit = 'system:user:edit'
export const UserRemove = 'system:user:remove'

export const GatewayList = 'gateway:gateway:list'
export const GatewayQuery = 'gateway:gateway:query'
export const GatewayAdd = 'gateway:gateway:add'
export const GatewayEdit = 'gateway:gateway:edit'
export const GatewayRemove = 'gateway:gateway:remove'
export const GatewayConfig = 'gateway:gateway:config'
export const GatewayCommand = 'gateway:gateway:command'

export const SolutionList = 'solution:list'
export const SolutionQuery = 'solution:query'
export const SolutionApply = 'solution:apply'
export const SolutionTslAdd = 'solution:tsl:add'
export const SolutionTslEdit = 'solution:tsl:edit'
export const SolutionTslRemove = 'solution:tsl:remove'

export const RoleList = 'system:role:list'
export const RoleQuery = 'system:role:query'
export const RoleAdd = 'system:role:add'
export const RoleEdit = 'system:role:edit'
export const RoleRemove = 'system:role:remove'

export const MenuList = 'system:menu:list'
export const MenuQuery = 'system:menu:query'
export const MenuAdd = 'system:menu:add'
export const MenuEdit = 'system:menu:edit'
export const MenuRemove = 'system:menu:remove'
export const MenuCustomPref = 'system:menu:custom:pref'
export const MenuCustomReset = 'system:menu:custom:reset'

export const DeptList = 'system:dept:list'
export const DeptQuery = 'system:dept:query'
export const DeptAdd = 'system:dept:add'
export const DeptEdit = 'system:dept:edit'
export const DeptRemove = 'system:dept:remove'

export const AuditLogList = 'system:auditlog:list'
export const AuditLogRemove = 'system:auditlog:remove'

export const LoginLogList = 'system:loginlog:list'
export const LoginLogRemove = 'system:loginlog:remove'

export const ConfigList = 'system:config:list'
export const ConfigQuery = 'system:config:query'
export const ConfigAdd = 'system:config:add'
export const ConfigEdit = 'system:config:edit'
export const ConfigRemove = 'system:config:remove'

export const HomeView = 'home:view'

export const VizBoardList = 'viz:board:list'
export const VizBoardQuery = 'viz:board:query'
export const VizBoardAdd = 'viz:board:add'
export const VizBoardEdit = 'viz:board:edit'
export const VizBoardRemove = 'viz:board:remove'
export const VizDataQuery = 'viz:data:query'

export const ProductList = 'gateway:product:list'
export const ProductQuery = 'gateway:product:query'
export const ProductAdd = 'gateway:product:add'
export const ProductEdit = 'gateway:product:edit'
export const ProductDel = 'gateway:product:remove'

export const DeviceList = 'gateway:device:list'
export const DeviceQuery = 'gateway:device:query'
export const DeviceAdd = 'gateway:device:add'
export const DeviceEdit = 'gateway:device:edit'
export const DeviceDel = 'gateway:device:remove'
export const DeviceImport = 'gateway:device:import'
export const DeviceBind = 'gateway:device:bind'

export const ImageList = 'gateway:image:list'
export const ImageAdd = 'gateway:image:add'
export const ImageEdit = 'gateway:image:edit'
export const ImageRemove = 'gateway:image:remove'

export const DriveList = 'gateway:drive:list'
export const DriveQuery = 'gateway:drive:query'
export const DriveAdd = 'gateway:drive:add'
export const DriveEdit = 'gateway:drive:edit'
export const DriveRemove = 'gateway:drive:remove'

export const MonitorView = 'monitor:system:view'

export const AlertList = 'monitor:alert:list'
export const AlertQuery = 'monitor:alert:query'
export const AlertAdd = 'monitor:alert:add'
export const AlertEdit = 'monitor:alert:edit'
export const AlertRemove = 'monitor:alert:remove'
export const AlertHandle = 'monitor:alert:handle'

export const DocsView = 'docs:view'

export const AdvancedResourceList = 'advanced:resource:list'
export const AdvancedResourceAdd = 'advanced:resource:add'
export const AdvancedResourceEdit = 'advanced:resource:edit'
export const AdvancedResourceRemove = 'advanced:resource:remove'
export const AdvancedResourceVerify = 'advanced:resource:verify'

export const AdvancedRuleList = 'advanced:rule:list'
export const AdvancedRuleQuery = 'advanced:rule:query'
export const AdvancedRuleAdd = 'advanced:rule:add'
export const AdvancedRuleEdit = 'advanced:rule:edit'
export const AdvancedRuleRemove = 'advanced:rule:remove'
export const AdvancedRuleStart = 'advanced:rule:start'
export const AdvancedRuleStop = 'advanced:rule:stop'

export const SystemDebugPprof = 'system:debug:pprof'

export const AdvancedSceneList = 'advanced:scene:list'
export const AdvancedSceneQuery = 'advanced:scene:query'
export const AdvancedSceneAdd = 'advanced:scene:add'
export const AdvancedSceneEdit = 'advanced:scene:edit'
export const AdvancedSceneRemove = 'advanced:scene:remove'
export const AdvancedSceneStart = 'advanced:scene:start'
export const AdvancedSceneStop = 'advanced:scene:stop'

/** 全部权限标识（与后端 /api/v1/user/getinfo 返回的子集一致） */
export const PERMISSIONS: string[] = [
  PlatformTenantList, PlatformTenantQuery, PlatformTenantAdd, PlatformTenantEdit, PlatformTenantRemove, PlatformTenantSwitch,
  UserList, UserQuery, UserAdd, UserEdit, UserRemove,
  GatewayList, GatewayQuery, GatewayAdd, GatewayEdit, GatewayRemove, GatewayConfig, GatewayCommand,
  SolutionList, SolutionQuery, SolutionApply, SolutionTslAdd, SolutionTslEdit, SolutionTslRemove,
  RoleList, RoleQuery, RoleAdd, RoleEdit, RoleRemove,
  MenuList, MenuQuery, MenuAdd, MenuEdit, MenuRemove, MenuCustomPref, MenuCustomReset,
  DeptList, DeptQuery, DeptAdd, DeptEdit, DeptRemove,
  AuditLogList, AuditLogRemove,
  LoginLogList, LoginLogRemove,
  ConfigList, ConfigQuery, ConfigAdd, ConfigEdit, ConfigRemove,
  HomeView,
  VizBoardList, VizBoardQuery, VizBoardAdd, VizBoardEdit, VizBoardRemove, VizDataQuery,
  ProductList, ProductQuery, ProductAdd, ProductEdit, ProductDel,
  DeviceList, DeviceQuery, DeviceAdd, DeviceEdit, DeviceDel, DeviceImport, DeviceBind,
  ImageList, ImageAdd, ImageEdit, ImageRemove,
  DriveList, DriveQuery, DriveAdd, DriveEdit, DriveRemove,
  MonitorView,
  AlertList, AlertQuery, AlertAdd, AlertEdit, AlertRemove, AlertHandle,
  DocsView,
  AdvancedResourceList, AdvancedResourceAdd, AdvancedResourceEdit, AdvancedResourceRemove, AdvancedResourceVerify,
  AdvancedRuleList, AdvancedRuleQuery, AdvancedRuleAdd, AdvancedRuleEdit, AdvancedRuleRemove, AdvancedRuleStart, AdvancedRuleStop,
  SystemDebugPprof,
  AdvancedSceneList, AdvancedSceneQuery, AdvancedSceneAdd, AdvancedSceneEdit, AdvancedSceneRemove, AdvancedSceneStart, AdvancedSceneStop,
]
