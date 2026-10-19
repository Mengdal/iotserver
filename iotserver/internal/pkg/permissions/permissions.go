// Package permissions 定义平台全部接口权限标识（单一来源）。
//
// 约定：
//   - 每个后台接口都必须在路由上用 middleware.Perm(...) 声明所需权限；
//   - 前端与数据库菜单都由本文件生成（见 tools/gen-permissions），
//     避免三处各写一份导致不一致；
//   - 拥有内置角色 admin 的账号直通全部接口。
package permissions

// 系统管理 - 用户
const (
	UserList   = "system:user:list"
	UserQuery  = "system:user:query"
	UserAdd    = "system:user:add"
	UserEdit   = "system:user:edit"
	UserRemove = "system:user:remove"
)

// 系统管理 - 角色
const (
	RoleList   = "system:role:list"
	RoleQuery  = "system:role:query"
	RoleAdd    = "system:role:add"
	RoleEdit   = "system:role:edit"
	RoleRemove = "system:role:remove"
)

// 系统管理 - 菜单
const (
	MenuList   = "system:menu:list"
	MenuQuery  = "system:menu:query"
	MenuAdd    = "system:menu:add"
	MenuEdit   = "system:menu:edit"
	MenuRemove = "system:menu:remove"
	// MenuCustomPref 写租户菜单展示层覆盖（隐藏/改名/图标/排序，tenant_menu_pref）
	MenuCustomPref = "system:menu:custom:pref"
	// MenuCustomReset 平台重置某个租户的菜单定制（清空自建菜单 + 覆盖记录）
	MenuCustomReset = "system:menu:custom:reset"
)

// 系统管理 - 部门
const (
	DeptList   = "system:dept:list"
	DeptQuery  = "system:dept:query"
	DeptAdd    = "system:dept:add"
	DeptEdit   = "system:dept:edit"
	DeptRemove = "system:dept:remove"
)

// 系统管理 - 审查日志（登录 + 操作，单表 sys_audit_log）
const (
	AuditLogList   = "system:auditlog:list"
	AuditLogRemove = "system:auditlog:remove"
)

// 系统管理 - 登入日志（已并入审查日志）：兼容别名，保留一版，下版本删除
const (
	LoginLogList   = "system:loginlog:list"
	LoginLogRemove = "system:loginlog:remove"
)

// 系统管理 - 系统配置
const (
	ConfigList   = "system:config:list"
	ConfigQuery  = "system:config:query"
	ConfigAdd    = "system:config:add"
	ConfigEdit   = "system:config:edit"
	ConfigRemove = "system:config:remove"
)

// 首页
const (
	HomeView = "home:view"
)

// 设备接入 - 产品管理
const (
	ProductList  = "gateway:product:list"
	ProductQuery = "gateway:product:query"
	ProductAdd   = "gateway:product:add"
	ProductEdit  = "gateway:product:edit"
	ProductDel   = "gateway:product:remove"
)

// 设备接入 - 设备管理
const (
	DeviceList   = "gateway:device:list"
	DeviceQuery  = "gateway:device:query"
	DeviceAdd    = "gateway:device:add"
	DeviceEdit   = "gateway:device:edit"
	DeviceDel    = "gateway:device:remove"
	DeviceImport = "gateway:device:import"
	DeviceBind   = "gateway:device:bind"
)

// 设备接入 - 驱动镜像
const (
	ImageList   = "gateway:image:list"
	ImageAdd    = "gateway:image:add"
	ImageEdit   = "gateway:image:edit"
	ImageRemove = "gateway:image:remove"
)

// 设备接入 - 我的驱动
const (
	DriveList   = "gateway:drive:list"
	DriveQuery  = "gateway:drive:query"
	DriveAdd    = "gateway:drive:add"
	DriveEdit   = "gateway:drive:edit"
	DriveRemove = "gateway:drive:remove"
)

// 运维监控 - 系统监控
const (
	MonitorView = "monitor:system:view"
)

// 运维监控 - 告警中心
const (
	AlertList   = "monitor:alert:list"
	AlertQuery  = "monitor:alert:query"
	AlertAdd    = "monitor:alert:add"
	AlertEdit   = "monitor:alert:edit"
	AlertRemove = "monitor:alert:remove"
	AlertHandle = "monitor:alert:handle"
)

// 工具与文档
const (
	DocsView = "docs:view"
)

// 命名权限，用于导出给前端（保持与常量名一致，便于对照）
type NamedPerm struct {
	Name  string
	Value string
}

// 高级能力 - 资源管理
const (
	AdvancedResourceList   = "advanced:resource:list"
	AdvancedResourceAdd    = "advanced:resource:add"
	AdvancedResourceEdit   = "advanced:resource:edit"
	AdvancedResourceRemove = "advanced:resource:remove"
	AdvancedResourceVerify = "advanced:resource:verify"
)

// 高级能力 - 规则引擎
const (
	AdvancedRuleList   = "advanced:rule:list"
	AdvancedRuleQuery  = "advanced:rule:query"
	AdvancedRuleAdd    = "advanced:rule:add"
	AdvancedRuleEdit   = "advanced:rule:edit"
	AdvancedRuleRemove = "advanced:rule:remove"
	AdvancedRuleStart  = "advanced:rule:start"
	AdvancedRuleStop   = "advanced:rule:stop"
)

// 高级能力 - 场景联动
const (
	AdvancedSceneList   = "advanced:scene:list"
	AdvancedSceneQuery  = "advanced:scene:query"
	AdvancedSceneAdd    = "advanced:scene:add"
	AdvancedSceneEdit   = "advanced:scene:edit"
	AdvancedSceneRemove = "advanced:scene:remove"
	AdvancedSceneStart  = "advanced:scene:start"
	AdvancedSceneStop   = "advanced:scene:stop"
)

// 系统调试
const (
	SystemDebugPprof = "system:debug:pprof"
)

// 可视化看板
//
// 查看类（list/query/data）给「数据看板」页用；管理类（add/edit/remove）给「看板管理」页用。
// 看板上的数据类组件要查设备属性，所以 data 单独一个权限，便于只让部分角色看数据。
const (
	VizBoardList   = "viz:board:list"
	VizBoardQuery  = "viz:board:query"
	VizBoardAdd    = "viz:board:add"
	VizBoardEdit   = "viz:board:edit"
	VizBoardRemove = "viz:board:remove"
	VizDataQuery   = "viz:data:query"
)

// 租户管理：**平台级**权限，只授予平台运营账号。
// 租户用户即使拥有同名前缀的菜单也不会生效（菜单带 platform_only，不下发给租户）。
const (
	PlatformTenantList   = "platform:tenant:list"
	PlatformTenantQuery  = "platform:tenant:query"
	PlatformTenantAdd    = "platform:tenant:add"
	PlatformTenantEdit   = "platform:tenant:edit"
	PlatformTenantRemove = "platform:tenant:remove"
	PlatformTenantSwitch = "platform:tenant:switch"
)

// 网关（第三方工业网关接入）：设备接入下的子模块
const (
	GatewayList    = "gateway:gateway:list"
	GatewayQuery   = "gateway:gateway:query"
	GatewayAdd     = "gateway:gateway:add"
	GatewayEdit    = "gateway:gateway:edit"
	GatewayRemove  = "gateway:gateway:remove"
	GatewayConfig  = "gateway:gateway:config"
	GatewayCommand = "gateway:gateway:command"
)

// 方案（行业模板一键应用）：设备接入下的子模块
const (
	SolutionList  = "solution:list"
	SolutionQuery = "solution:query"
	SolutionApply = "solution:apply"
	// 方案物模型（TSL）条目维护：内置方案只读，首次修改会克隆出当前租户的副本
	SolutionTslAdd    = "solution:tsl:add"
	SolutionTslEdit   = "solution:tsl:edit"
	SolutionTslRemove = "solution:tsl:remove"
)

// NamedGroups 按模块分组的全部权限，顺序稳定
func NamedGroups() [][]NamedPerm {
	return [][]NamedPerm{
		{{"PlatformTenantList", PlatformTenantList}, {"PlatformTenantQuery", PlatformTenantQuery},
			{"PlatformTenantAdd", PlatformTenantAdd}, {"PlatformTenantEdit", PlatformTenantEdit},
			{"PlatformTenantRemove", PlatformTenantRemove}, {"PlatformTenantSwitch", PlatformTenantSwitch}},
		{{"UserList", UserList}, {"UserQuery", UserQuery}, {"UserAdd", UserAdd}, {"UserEdit", UserEdit}, {"UserRemove", UserRemove}},
		{{"GatewayList", GatewayList}, {"GatewayQuery", GatewayQuery}, {"GatewayAdd", GatewayAdd},
			{"GatewayEdit", GatewayEdit}, {"GatewayRemove", GatewayRemove},
			{"GatewayConfig", GatewayConfig}, {"GatewayCommand", GatewayCommand}},
		{{"SolutionList", SolutionList}, {"SolutionQuery", SolutionQuery},
			{"SolutionApply", SolutionApply},
			{"SolutionTslAdd", SolutionTslAdd}, {"SolutionTslEdit", SolutionTslEdit},
			{"SolutionTslRemove", SolutionTslRemove}},
		{{"RoleList", RoleList}, {"RoleQuery", RoleQuery}, {"RoleAdd", RoleAdd}, {"RoleEdit", RoleEdit}, {"RoleRemove", RoleRemove}},
		{{"MenuList", MenuList}, {"MenuQuery", MenuQuery}, {"MenuAdd", MenuAdd}, {"MenuEdit", MenuEdit}, {"MenuRemove", MenuRemove},
			{"MenuCustomPref", MenuCustomPref}, {"MenuCustomReset", MenuCustomReset}},
		{{"DeptList", DeptList}, {"DeptQuery", DeptQuery}, {"DeptAdd", DeptAdd}, {"DeptEdit", DeptEdit}, {"DeptRemove", DeptRemove}},
		{{"AuditLogList", AuditLogList}, {"AuditLogRemove", AuditLogRemove}},
		// 兼容别名（旧登入日志权限码，保留一版）
		{{"LoginLogList", LoginLogList}, {"LoginLogRemove", LoginLogRemove}},
		{{"ConfigList", ConfigList}, {"ConfigQuery", ConfigQuery}, {"ConfigAdd", ConfigAdd}, {"ConfigEdit", ConfigEdit}, {"ConfigRemove", ConfigRemove}},
		{{"HomeView", HomeView}},
		{{"VizBoardList", VizBoardList}, {"VizBoardQuery", VizBoardQuery}, {"VizBoardAdd", VizBoardAdd},
			{"VizBoardEdit", VizBoardEdit}, {"VizBoardRemove", VizBoardRemove}, {"VizDataQuery", VizDataQuery}},
		{{"ProductList", ProductList}, {"ProductQuery", ProductQuery}, {"ProductAdd", ProductAdd}, {"ProductEdit", ProductEdit}, {"ProductDel", ProductDel}},
		{{"DeviceList", DeviceList}, {"DeviceQuery", DeviceQuery}, {"DeviceAdd", DeviceAdd}, {"DeviceEdit", DeviceEdit}, {"DeviceDel", DeviceDel}, {"DeviceImport", DeviceImport}, {"DeviceBind", DeviceBind}},
		{{"ImageList", ImageList}, {"ImageAdd", ImageAdd}, {"ImageEdit", ImageEdit}, {"ImageRemove", ImageRemove}},
		{{"DriveList", DriveList}, {"DriveQuery", DriveQuery}, {"DriveAdd", DriveAdd}, {"DriveEdit", DriveEdit}, {"DriveRemove", DriveRemove}},
		{{"MonitorView", MonitorView}},
		{{"AlertList", AlertList}, {"AlertQuery", AlertQuery}, {"AlertAdd", AlertAdd}, {"AlertEdit", AlertEdit}, {"AlertRemove", AlertRemove}, {"AlertHandle", AlertHandle}},
		{{"DocsView", DocsView}},
		{{"AdvancedResourceList", AdvancedResourceList}, {"AdvancedResourceAdd", AdvancedResourceAdd},
			{"AdvancedResourceEdit", AdvancedResourceEdit}, {"AdvancedResourceRemove", AdvancedResourceRemove},
			{"AdvancedResourceVerify", AdvancedResourceVerify}},
		{{"AdvancedRuleList", AdvancedRuleList}, {"AdvancedRuleQuery", AdvancedRuleQuery},
			{"AdvancedRuleAdd", AdvancedRuleAdd}, {"AdvancedRuleEdit", AdvancedRuleEdit},
			{"AdvancedRuleRemove", AdvancedRuleRemove}, {"AdvancedRuleStart", AdvancedRuleStart},
			{"AdvancedRuleStop", AdvancedRuleStop}},
		{{"SystemDebugPprof", SystemDebugPprof}},
		{{"AdvancedSceneList", AdvancedSceneList}, {"AdvancedSceneQuery", AdvancedSceneQuery},
			{"AdvancedSceneAdd", AdvancedSceneAdd}, {"AdvancedSceneEdit", AdvancedSceneEdit},
			{"AdvancedSceneRemove", AdvancedSceneRemove}, {"AdvancedSceneStart", AdvancedSceneStart},
			{"AdvancedSceneStop", AdvancedSceneStop}},
	}
}

// All 返回全部权限标识（顺序稳定）
func All() []string {
	out := make([]string, 0, 64)
	for _, group := range NamedGroups() {
		for _, item := range group {
			out = append(out, item.Value)
		}
	}
	return out
}
