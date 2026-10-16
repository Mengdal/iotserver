package seed

import (
	"fmt"

	"iotserver/internal/models"
	"iotserver/internal/pkg/permissions"
)

// 系统管理菜单 ID（固定 ID，便于角色授权与版本升级）
const (
	MenuAdminId       = "9000000000000000001"
	MenuAdminUser     = "9000000000000000002"
	MenuAdminRole     = "9000000000000000003"
	MenuAdminMenu     = "9000000000000000004"
	MenuAdminDept     = "9000000000000000005"
	MenuAdminAuditLog = "9000000000000000006"
	MenuAdminConfig   = "9000000000000000007"
)

// 其它模块菜单 ID
const (
	MenuHomeId      = "9000000000000000010"
	MenuDocsId      = "9000000000000000011"
	MenuGatewayId   = "9001000000000000001"
	MenuProduct     = "9001000000000000002"
	MenuDevice      = "9001000000000000003"
	MenuImage       = "9001000000000000004"
	MenuDrive       = "9001000000000000005"
	MenuDataId      = "9003000000000000001"
	MenuDataHistory = "9003000000000000002"
	// MenuVizGroupId 可视化目录（数据看板 / 看板管理 的分组）
	MenuVizGroupId       = "9004000000000000000"
	MenuVizId            = "9004000000000000001"
	MenuVizBoardId       = "9004000000000000002"
	MenuAdvancedId       = "9005000000000000001"
	MenuAdvancedResource = "9005000000000000002"
	MenuAdvancedRule     = "9005000000000000003"
	MenuAdvancedScene    = "9005000000000000004"
	MenuMonitorId        = "9002000000000000001"
	MenuSysMon           = "9002000000000000002"
	MenuAlarm            = "9002000000000000003"
)

// 内置角色
const (
	RoleAdminId  = "8000000000000000001"
	RoleAdminKey = "admin"
)

// 平台级菜单（platform_only=1，不下发给租户用户）
const (
	MenuPlatformTenant = "9000000000000000101"
	// MenuPlatformId 平台管理目录（平台管理员专属功能的分组）
	MenuPlatformId = "9000000000000000100"
)

// 设备接入 - 网关管理菜单
const (
	MenuGatewayManage = "9001000000000000010"
)

// 设备接入 - 方案中心菜单
const (
	MenuSolution = "9001000000000000011"
)

// 设备接入 - 驱动管理目录（把驱动相关菜单收进去）
const (
	MenuDriveGroup = "9001000000000000006"
)

// 内置部门
const (
	DeptRootId = "7000000000000000001"
)

// 会被真实读取的系统配置键
const (
	ConfigKeyPasswordMinLength = "security.passwordMinLength"
	ConfigKeySessionTimeout    = "security.sessionTimeout"
	ConfigKeyRetentionDays     = "data.retentionDays"
	// ConfigKeyOfflineTimeout 设备/网关离线判定超时（分钟）：最后活跃时间早于
	// now - 该值，即由后台扫描置为离线。设备与网关共用同一个值，避免两套语义。
	ConfigKeyOfflineTimeout = "data.offlineTimeout"
)

// 内置管理员
const (
	AdminUserId = "5000000000000000001"
	AdminName   = "admin"
)

type permAction struct {
	Suffix string
	Name   string
}

var crudActions = []permAction{
	{"list", "查询"},
	{"query", "详情"},
	{"add", "新增"},
	{"edit", "修改"},
	{"remove", "删除"},
}

// buttonGroup 每个菜单需要生成的按钮权限
type buttonGroup struct {
	parentId string
	name     string
	prefix   string
	actions  []permAction
}

func withActions(base []permAction, extra ...permAction) []permAction {
	out := make([]permAction, 0, len(base)+len(extra))
	out = append(out, base...)
	out = append(out, extra...)
	return out
}

var buttonGroups = []buttonGroup{
	// 平台级：租户管理，只授予 ROBX 运营账号
	{MenuPlatformTenant, "租户管理", "platform:tenant", []permAction{
		{"add", "新增"}, {"edit", "修改"}, {"remove", "删除"}, {"switch", "切换"}}},
	{MenuAdminUser, "用户管理", "system:user", crudActions},
	{MenuAdminRole, "角色管理", "system:role", crudActions},
	{MenuAdminMenu, "菜单管理", "system:menu", crudActions},
	{MenuAdminDept, "部门管理", "system:dept", crudActions},
	{MenuAdminAuditLog, "审查日志", "system:auditlog", []permAction{{"list", "查询"}, {"remove", "删除"}}},
	{MenuAdminConfig, "系统配置", "system:config", crudActions},
	{MenuProduct, "产品管理", "gateway:product", crudActions},
	{MenuDevice, "设备管理", "gateway:device", withActions(crudActions,
		permAction{"import", "导入"}, permAction{"bind", "绑定"})},
	{MenuImage, "驱动镜像", "gateway:image", []permAction{{"list", "查询"}, {"add", "新增"}, {"edit", "修改"}, {"remove", "删除"}}},
	{MenuDrive, "我的驱动", "gateway:drive", crudActions},
	{MenuGatewayManage, "网关管理", "gateway:gateway", withActions(crudActions,
		permAction{"config", "下发配置"}, permAction{"command", "下发命令"})},
	// 方案中心：list 权限已挂在菜单上，按钮组只补「应用」与物模型条目的增/改/删
	{MenuSolution, "方案中心", "solution", []permAction{
		{"query", "详情"},
		{"apply", "应用"},
		{"tsl:add", "添加物模型"}, {"tsl:edit", "编辑物模型"}, {"tsl:remove", "删除物模型"}}},
	{MenuAlarm, "告警中心", "monitor:alert", withActions(crudActions, permAction{"handle", "处理"})},
	// 资源管理没有独立详情接口（列表即用于编辑回填），因此按钮组不含 query
	{MenuAdvancedResource, "资源管理", "advanced:resource", []permAction{
		{"list", "查询"}, {"add", "新增"}, {"edit", "修改"}, {"remove", "删除"}, {"verify", "验证"}}},
	{MenuAdvancedRule, "规则引擎", "advanced:rule", withActions(crudActions,
		permAction{"start", "启动"}, permAction{"stop", "停止"})},
	{MenuAdvancedScene, "场景联动", "advanced:scene", withActions(crudActions,
		permAction{"start", "启动"}, permAction{"stop", "停止"})},
	// 菜单定制的按钮权限（system:menu:custom:pref / :reset）。
	// 必须追加在最后：按钮菜单 ID 是按声明顺序生成的（9100000000000%05d），
	// 插在中间会让其后所有按钮 ID 整体位移 —— 升级时旧 ID 被当「遗留菜单」清理重建，
	// 租户角色已授权的按钮会全部失效。prefix 拆到 system:menu:custom，
	// 后缀 pref/reset 拼出与 permissions 常量一致的权限码。
	{MenuAdminMenu, "菜单管理", "system:menu:custom", []permAction{
		{"pref", "菜单定制"}, {"reset", "重置菜单定制"}}},
}

// DefaultMenus 返回平台默认菜单树（扁平列表，含目录 / 菜单 / 按钮）
func DefaultMenus() []models.Menu {
	menus := []models.Menu{
		// ===== 首页 =====
		{
			Id: MenuHomeId, ParentId: "0", Name: "首页", Path: "/home/dashboard",
			Icon: "dashboard", MenuType: models.MenuTypeMenu, Perms: permissions.HomeView,
			Sort: 1, Visible: 1, Status: 1, IsFrame: 1,
		},
		// ===== 平台管理（仅 ROBX 运营账号可见）=====
		{
			// 平台管理：平台管理员专属功能的分组（租户管理、系统配置）。
			// 整组标记 platform_only：租户用户侧边栏不出现该目录，也不会出现"空目录"。
			Id: MenuPlatformId, ParentId: "0", Name: "平台管理", Path: "/platform",
			Icon: "safety", MenuType: models.MenuTypeDir,
			Sort: 5, Visible: 1, Status: 1, IsFrame: 1, PlatformOnly: 1,
		},
		{
			Id: MenuPlatformTenant, ParentId: MenuPlatformId, Name: "租户管理", Path: "/platform/tenant",
			Icon: "team", MenuType: models.MenuTypeMenu, Perms: permissions.PlatformTenantList,
			Sort: 1, Visible: 1, Status: 1, IsFrame: 1, PlatformOnly: 1,
		},
		// ===== 设备接入 =====
		{
			Id: MenuGatewayId, ParentId: "0", Name: "设备接入", Path: "/gateway",
			Icon: "global", MenuType: models.MenuTypeDir,
			Sort: 10, Visible: 1, Status: 1, IsFrame: 1,
		},
		{
			Id: MenuProduct, ParentId: MenuGatewayId, Name: "产品管理", Path: "/gateway/product/manage",
			Icon: "appstore", MenuType: models.MenuTypeMenu, Perms: permissions.ProductList,
			Sort: 1, Visible: 1, Status: 1, IsFrame: 1,
		},
		{
			Id: MenuDevice, ParentId: MenuGatewayId, Name: "设备管理", Path: "/gateway/device/manage",
			Icon: "hdd", MenuType: models.MenuTypeMenu, Perms: permissions.DeviceList,
			Sort: 2, Visible: 1, Status: 1, IsFrame: 1,
		},
		{
			Id: MenuImage, ParentId: MenuDriveGroup, Name: "驱动镜像", Path: "/gateway/drive/image",
			Icon: "cloud-server", MenuType: models.MenuTypeMenu, Perms: permissions.ImageList,
			Sort: 1, Visible: 1, Status: 1, IsFrame: 1,
		},
		{
			Id: MenuDrive, ParentId: MenuDriveGroup, Name: "我的驱动", Path: "/gateway/my/drive",
			Icon: "deployment-unit", MenuType: models.MenuTypeMenu, Perms: permissions.DriveList,
			Sort: 2, Visible: 1, Status: 1, IsFrame: 1,
		},
		{
			// 网关管理：第三方工业网关的注册、MQTT 接入信息、配置下发与在线状态
			Id: MenuGatewayManage, ParentId: MenuGatewayId, Name: "网关管理", Path: "/gateway/gateway/manage",
			Icon: "cloud-server", MenuType: models.MenuTypeMenu, Perms: permissions.GatewayList,
			Sort: 3, Visible: 1, Status: 1, IsFrame: 1,
		},
		{
			// 方案中心：行业方案（物模型 + 告警预设 + 看板预设）一键生成产品与告警规则
			Id: MenuSolution, ParentId: MenuGatewayId, Name: "方案中心", Path: "/solution/center",
			Icon: "appstore", MenuType: models.MenuTypeMenu, Perms: permissions.SolutionList,
			Sort: 4, Visible: 1, Status: 1, IsFrame: 1,
		},
		{
			// 驱动管理：与「设备接入」同级的顶层目录（镜像仓库 + 驱动实例）
			Id: MenuDriveGroup, ParentId: "0", Name: "驱动管理", Path: "/drive",
			Icon: "cloud-server", MenuType: models.MenuTypeDir,
			Sort: 12, Visible: 1, Status: 1, IsFrame: 1,
		},
		// ===== 数据中心 =====
		{
			Id: MenuDataId, ParentId: "0", Name: "数据中心", Path: "/data",
			Icon: "database", MenuType: models.MenuTypeDir,
			Sort: 15, Visible: 1, Status: 1, IsFrame: 1,
		},
		{
			Id: MenuDataHistory, ParentId: MenuDataId, Name: "历史查询", Path: "/data/history",
			Icon: "line-chart", MenuType: models.MenuTypeMenu, Perms: permissions.DeviceQuery,
			Sort: 1, Visible: 1, Status: 1, IsFrame: 1,
		},
		// ===== 可视化 =====
		{
			// 可视化：数据看板与看板管理的分组目录（放在「数据中心」之上）
			Id: MenuVizGroupId, ParentId: "0", Name: "可视化", Path: "/viz",
			Icon: "area-chart", MenuType: models.MenuTypeDir,
			Sort: 14, Visible: 1, Status: 1, IsFrame: 1,
		},
		{
			// 数据看板：把「看板管理」里设为整页/默认的那块画布看板渲染出来（只读）
			// 查看 + 取数权限：没有 viz:data:query 的角色能看排版，但取不到设备数据
			Id: MenuVizId, ParentId: MenuVizGroupId, Name: "数据看板", Path: "/viz/dashboard",
			Icon: "bar-chart", MenuType: models.MenuTypeMenu,
			Perms: permissions.VizBoardList + "," + permissions.VizBoardQuery + "," + permissions.VizDataQuery,
			Sort:  1, Visible: 1, Status: 1, IsFrame: 1,
		},
		{
			// 看板管理：新建/编辑/删除画布看板，并指定首页卡片与整页用哪一块
			Id: MenuVizBoardId, ParentId: MenuVizGroupId, Name: "看板管理", Path: "/viz/boards",
			Icon: "setting", MenuType: models.MenuTypeMenu,
			Perms: permissions.VizBoardList + "," + permissions.VizBoardQuery + "," + permissions.VizDataQuery +
				"," + permissions.VizBoardAdd + "," + permissions.VizBoardEdit + "," + permissions.VizBoardRemove,
			Sort: 2, Visible: 1, Status: 1, IsFrame: 1,
		},
		// ===== 高级能力 =====
		{
			Id: MenuAdvancedId, ParentId: "0", Name: "高级能力", Path: "/advanced",
			Icon: "cluster", MenuType: models.MenuTypeDir,
			Sort: 25, Visible: 1, Status: 1, IsFrame: 1,
		},
		{
			Id: MenuAdvancedResource, ParentId: MenuAdvancedId, Name: "资源管理", Path: "/advanced/resource/manage",
			Icon: "api", MenuType: models.MenuTypeMenu, Perms: permissions.AdvancedResourceList,
			Sort: 1, Visible: 1, Status: 1, IsFrame: 1,
		},
		{
			Id: MenuAdvancedRule, ParentId: MenuAdvancedId, Name: "规则引擎", Path: "/advanced/rule/engine",
			Icon: "deployment-unit", MenuType: models.MenuTypeMenu, Perms: permissions.AdvancedRuleList,
			Sort: 2, Visible: 1, Status: 1, IsFrame: 1,
		},
		{
			Id: MenuAdvancedScene, ParentId: MenuAdvancedId, Name: "场景联动", Path: "/advanced/scene/linkage",
			Icon: "apartment", MenuType: models.MenuTypeMenu, Perms: permissions.AdvancedSceneList,
			Sort: 3, Visible: 1, Status: 1, IsFrame: 1,
		},
		// ===== 运维监控 =====
		{
			Id: MenuMonitorId, ParentId: "0", Name: "运维监控", Path: "/monitor",
			Icon: "field-time", MenuType: models.MenuTypeDir,
			Sort: 20, Visible: 1, Status: 1, IsFrame: 1,
		},
		{
			Id: MenuSysMon, ParentId: MenuMonitorId, Name: "系统监控", Path: "/monitor/systemMonitor",
			Icon: "dashboard", MenuType: models.MenuTypeMenu, Perms: permissions.MonitorView,
			Sort: 1, Visible: 1, Status: 1, IsFrame: 1,
		},
		{
			Id: MenuAlarm, ParentId: MenuMonitorId, Name: "告警中心", Path: "/monitor/alarmCenter/list",
			Icon: "alert", MenuType: models.MenuTypeMenu, Perms: permissions.AlertList,
			Sort: 2, Visible: 1, Status: 1, IsFrame: 1,
		},
		// ===== 系统配置 =====
		{
			// 顶级目录用「系统管理」，避免与其下的「系统配置」菜单重名
			Id: MenuAdminId, ParentId: "0", Name: "系统管理", Path: "/admin",
			Icon: "setting", MenuType: models.MenuTypeDir,
			Sort: 90, Visible: 1, Status: 1, IsFrame: 1,
		},
		{
			Id: MenuAdminUser, ParentId: MenuAdminId, Name: "用户管理", Path: "/admin/sysUser",
			Icon: "user", MenuType: models.MenuTypeMenu, Perms: permissions.UserList,
			Sort: 1, Visible: 1, Status: 1, IsFrame: 1,
		},
		{
			Id: MenuAdminRole, ParentId: MenuAdminId, Name: "角色管理", Path: "/admin/sysRole",
			Icon: "team", MenuType: models.MenuTypeMenu, Perms: permissions.RoleList,
			Sort: 2, Visible: 1, Status: 1, IsFrame: 1,
		},
		{
			Id: MenuAdminMenu, ParentId: MenuAdminId, Name: "菜单管理", Path: "/admin/sysMenu",
			Icon: "menu", MenuType: models.MenuTypeMenu, Perms: permissions.MenuList,
			Sort: 3, Visible: 1, Status: 1, IsFrame: 1,
		},
		{
			Id: MenuAdminDept, ParentId: MenuAdminId, Name: "部门管理", Path: "/admin/sysDept",
			Icon: "apartment", MenuType: models.MenuTypeMenu, Perms: permissions.DeptList,
			Sort: 4, Visible: 1, Status: 1, IsFrame: 1,
		},
		{
			Id: MenuAdminAuditLog, ParentId: MenuAdminId, Name: "审查日志", Path: "/admin/sysAuditLog",
			Icon: "file-search", MenuType: models.MenuTypeMenu, Perms: permissions.AuditLogList,
			Sort: 5, Visible: 1, Status: 1, IsFrame: 1,
		},
		{
			// 系统配置：全局唯一一份（sys_config 无 tenant_id），写接口为平台管理员专属，
			// 因此菜单标记为平台级，不下发给租户用户（读接口仍保留给租户页面）
			Id: MenuAdminConfig, ParentId: MenuPlatformId, Name: "系统配置", Path: "/platform/sysSet",
			Icon: "tool", MenuType: models.MenuTypeMenu, Perms: permissions.ConfigList,
			Sort: 2, Visible: 1, Status: 1, IsFrame: 1, PlatformOnly: 1,
		},
		// 个人中心不再建菜单：它此前是 ParentId=0 的顶层菜单，因此会出现在侧边栏。
		// 入口固定在顶栏头像下拉里，页面路由 /common/userProfile 保持不变。
		// ===== 工具与文档 =====
		{
			Id: MenuDocsId, ParentId: "0", Name: "工具与文档", Path: "/documents/documents",
			Icon: "book", MenuType: models.MenuTypeMenu, Perms: permissions.DocsView,
			Sort: 92, Visible: 1, Status: 1, IsFrame: 1,
		},
	}

	// 生成按钮权限
	// 注意：Sort 必须按「同一父级内」重新计数，否则按钮的排序值会大于父菜单，
	// 导致按 sort 排序的树表把子节点排到父菜单前面。
	seq := 0
	for _, group := range buttonGroups {
		for i, action := range group.actions {
			seq++
			menus = append(menus, models.Menu{
				Id:       fmt.Sprintf("9100000000000%05d", seq),
				ParentId: group.parentId,
				Name:     fmt.Sprintf("%s%s", group.name, action.Name),
				MenuType: models.MenuTypeButton,
				Perms:    fmt.Sprintf("%s:%s", group.prefix, action.Suffix),
				Sort:     i + 1,
				Visible:  1,
				Status:   1,
				IsFrame:  1,
			})
		}
	}
	return menus
}

// DefaultDepts 默认部门
func DefaultDepts() []models.Dept {
	return []models.Dept{
		{
			Id:        DeptRootId,
			ParentId:  "0",
			Ancestors: "0",
			Name:      "iotserver",
			Sort:      1,
			Leader:    AdminName,
			Status:    1,
		},
	}
}

// DefaultRoles 默认角色
func DefaultRoles() []models.Role {
	return []models.Role{
		{
			Id:        RoleAdminId,
			Name:      "超级管理员",
			RoleKey:   RoleAdminKey,
			Sort:      1,
			DataScope: models.DataScopeAll,
			Status:    1,
			Remark:    "拥有全部权限",
		},
	}
}

// 系统配置默认项
type ConfigItem struct {
	Key   string
	Name  string
	Value string
	Group string
	Type  string
	Sort  int
}

// DefaultConfigs 默认系统配置
//
// 只保留「真实生效」的配置项（每一项都有代码读取它），
// 避免出现页面上能改、实际不起作用的装饰性配置。
func DefaultConfigs() []ConfigItem {
	return []ConfigItem{
		{"platform.name", "系统名称", "iotserver 物联网平台", "brand", "string", 1},
		{"platform.copyright", "版权信息", "© iotserver", "brand", "string", 2},
		{"platform.logo", "系统 Logo", "", "brand", "image", 3},
		{"platform.favicon", "浏览器标签", "", "brand", "image", 4},
		{"platform.loginBackground", "登录页背景图", "", "brand", "image", 5},
		{"security.sessionTimeout", "会话超时时间(分钟)", "180", "security", "number", 6},
		{"security.passwordMinLength", "密码最小长度", "6", "security", "number", 7},
		{"data.retentionDays", "数据保留天数", "30", "data", "number", 8},
		{"data.offlineTimeout", "离线判定超时(分钟)", "5", "data", "number", 9},
	}
}

// UnusedConfigKeys 历史版本写过、但没有任何代码消费的配置项；
// 启动时清理，避免页面上出现「改了不生效」的配置。
func UnusedConfigKeys() []string {
	return []string{
		"platform.description",
		"security.loginFailLock",
		"security.allowRegister",
		"data.messageRetentionDays",
		"notify.enable",
		"notify.silenceTime",
	}
}
