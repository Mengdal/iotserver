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
	MenuAdminLoginLog = "9000000000000000006"
	MenuAdminConfig   = "9000000000000000007"
	MenuAdminProfile  = "9000000000000000008"
)

// 其它模块菜单 ID
const (
	MenuHomeId           = "9000000000000000010"
	MenuDocsId           = "9000000000000000011"
	MenuGatewayId        = "9001000000000000001"
	MenuProduct          = "9001000000000000002"
	MenuDevice           = "9001000000000000003"
	MenuImage            = "9001000000000000004"
	MenuDrive            = "9001000000000000005"
	MenuDataId           = "9003000000000000001"
	MenuDataHistory      = "9003000000000000002"
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

// 内置部门
const (
	DeptRootId = "7000000000000000001"
)

// 会被真实读取的系统配置键
const (
	ConfigKeyPasswordMinLength = "security.passwordMinLength"
	ConfigKeySessionTimeout    = "security.sessionTimeout"
	ConfigKeyRetentionDays     = "data.retentionDays"
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
	{MenuAdminUser, "用户管理", "system:user", crudActions},
	{MenuAdminRole, "角色管理", "system:role", crudActions},
	{MenuAdminMenu, "菜单管理", "system:menu", crudActions},
	{MenuAdminDept, "部门管理", "system:dept", crudActions},
	{MenuAdminLoginLog, "登入日志", "system:loginlog", []permAction{{"list", "查询"}, {"remove", "删除"}}},
	{MenuAdminConfig, "系统配置", "system:config", crudActions},
	{MenuProduct, "产品管理", "gateway:product", crudActions},
	{MenuDevice, "设备管理", "gateway:device", withActions(crudActions,
		permAction{"import", "导入"}, permAction{"bind", "绑定"})},
	{MenuImage, "驱动镜像", "gateway:image", []permAction{{"list", "查询"}, {"add", "新增"}, {"edit", "修改"}, {"remove", "删除"}}},
	{MenuDrive, "我的驱动", "gateway:drive", crudActions},
	{MenuAlarm, "告警中心", "monitor:alert", withActions(crudActions, permAction{"handle", "处理"})},
	// 资源管理没有独立详情接口（列表即用于编辑回填），因此按钮组不含 query
	{MenuAdvancedResource, "资源管理", "advanced:resource", []permAction{
		{"list", "查询"}, {"add", "新增"}, {"edit", "修改"}, {"remove", "删除"}, {"verify", "验证"}}},
	{MenuAdvancedRule, "规则引擎", "advanced:rule", withActions(crudActions,
		permAction{"start", "启动"}, permAction{"stop", "停止"})},
	{MenuAdvancedScene, "场景联动", "advanced:scene", withActions(crudActions,
		permAction{"start", "启动"}, permAction{"stop", "停止"})},
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
			Id: MenuImage, ParentId: MenuGatewayId, Name: "驱动镜像", Path: "/gateway/drive/image",
			Icon: "cloud-server", MenuType: models.MenuTypeMenu, Perms: permissions.ImageList,
			Sort: 3, Visible: 1, Status: 1, IsFrame: 1,
		},
		{
			Id: MenuDrive, ParentId: MenuGatewayId, Name: "我的驱动", Path: "/gateway/my/drive",
			Icon: "deployment-unit", MenuType: models.MenuTypeMenu, Perms: permissions.DriveList,
			Sort: 4, Visible: 1, Status: 1, IsFrame: 1,
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
			// Shaper 看板：前端用 React SDK 直嵌（不走 iframe），看板内容在 Shaper 里用 SQL 维护
			Id: MenuVizId, ParentId: "0", Name: "数据看板", Path: "/viz/dashboard",
			Icon: "bar-chart", MenuType: models.MenuTypeMenu,
			Sort: 93, Visible: 1, Status: 1, IsFrame: 1,
		},
		{
			// 看板管理：新建/编辑/删除 Shaper 看板，并指定首页卡片与整页用哪一块
			Id: MenuVizBoardId, ParentId: "0", Name: "看板管理", Path: "/viz/boards",
			Icon: "setting", MenuType: models.MenuTypeMenu,
			Sort: 94, Visible: 1, Status: 1, IsFrame: 1,
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
			Id: MenuAdminLoginLog, ParentId: MenuAdminId, Name: "登入日志", Path: "/admin/sysLoginLog",
			Icon: "file-search", MenuType: models.MenuTypeMenu, Perms: permissions.LoginLogList,
			Sort: 5, Visible: 1, Status: 1, IsFrame: 1,
		},
		{
			Id: MenuAdminConfig, ParentId: MenuAdminId, Name: "系统配置", Path: "/admin/sysSet",
			Icon: "tool", MenuType: models.MenuTypeMenu, Perms: permissions.ConfigList,
			Sort: 6, Visible: 1, Status: 1, IsFrame: 1,
		},
		// ===== 个人中心（不在侧边栏，由头像菜单进入）=====
		{
			Id: MenuAdminProfile, ParentId: "0", Name: "个人中心", Path: "/common/userProfile",
			Icon: "profile", MenuType: models.MenuTypeMenu, Perms: permissions.UserQuery,
			Sort: 91, Visible: 1, Status: 1, IsFrame: 1,
		},
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
