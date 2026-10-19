/*******************************************************************************
 * Copyright 2017 Dell Inc.
 * Copyright (c) 2019 Intel Corporation
 *
 * Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software distributed under the License
 * is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express
 * or implied. See the License for the specific language governing permissions and limitations under
 * the License.
 *******************************************************************************/
package route

import (
	"iotserver/internal/iotserver/core/controller/http/gateway"
	"iotserver/internal/iotserver/core/controller/http/websocket"
	"iotserver/internal/pkg/di"
	"iotserver/internal/pkg/middleware"
	"iotserver/internal/tools/jwt"

	"github.com/gin-gonic/gin"

	"github.com/swaggo/files"
	gs "github.com/swaggo/gin-swagger"
)

func RegisterGateway(engine *gin.Engine, dic *di.Container) {
	ctl := gateway.New(dic)
	v1 := engine.Group("/api/v1")
	v1.GET("/swagger/*any", gs.WrapHandler(swaggerFiles.Handler))

	v1.POST("auth/login", ctl.Login)
	v1.GET("auth/initInfo", ctl.InitInfo)
	v1.POST("auth/init-password", ctl.InitPassword)
	// 站点品牌信息：登录页需要，免鉴权（只暴露名称/logo/favicon/背景图/版权等展示类字段）
	v1.GET("site-info", ctl.SiteInfo)

	v1.POST("ekuiper/alert", ctl.EkuiperAlert)
	v1.POST("ekuiper/scene", ctl.EkuiperScene)
	v1.GET("ws/", websocket.NewServer(dic).Handle)

	v1Auth := v1.Group("", jwt.JWTAuth(false), middleware.TenantContext(), middleware.TenantExpireCheck(dic),
		// 审查日志（操作日志）：只记非 GET 写操作，异步落库，不阻塞业务
		middleware.AuditLog(dic))
	v1Auth.PUT("auth/password", ctl.UpdatePassword)
	/*******可视化看板（画布）*******/
	{
		// 看板管理（列表/新建/编辑/删除/设为首页或默认）
		v1Auth.GET("viz/boards", ctl.Perm(middleware.PermVizBoardList), ctl.VizBoards)
		v1Auth.POST("viz/boards", ctl.Perm(middleware.PermVizBoardAdd), ctl.VizBoardCreate)
		v1Auth.GET("viz/boards/:boardId", ctl.Perm(middleware.PermVizBoardQuery), ctl.VizBoardDetail)
		v1Auth.PUT("viz/boards/:boardId", ctl.Perm(middleware.PermVizBoardEdit), ctl.VizBoardUpdate)
		v1Auth.DELETE("viz/boards/:boardId", ctl.Perm(middleware.PermVizBoardRemove), ctl.VizBoardDelete)
		v1Auth.POST("viz/boards/:boardId/active", ctl.Perm(middleware.PermVizBoardEdit), ctl.VizBoardActive)
		// 看板取数：画布上的数据类组件（数值/仪表盘/折线/表格）批量查询设备属性与平台/租户统计指标
		v1Auth.POST("viz/device-query", ctl.Perm(middleware.PermVizDataQuery), ctl.VizDeviceQuery)
		// 可绑定的指标清单（前端下拉用；权限与取数一致）
		v1Auth.GET("viz/metrics", ctl.Perm(middleware.PermVizDataQuery), ctl.VizMetrics)
		// 设备消息量排行（首页看板「设备消息量排名」组件）
		v1Auth.GET("viz/device-message-rank", ctl.Perm(middleware.PermVizDataQuery), ctl.VizDeviceMessageRank)
	}
	/******* 平台管理（租户/租户）：平台管理员专属 *******/
	{
		platform := v1Auth.Group("", middleware.PlatformOnly())
		platform.GET("tenant", ctl.Perm(middleware.PermPlatformTenantList), ctl.TenantSearch)
		platform.GET("tenant/:id", ctl.Perm(middleware.PermPlatformTenantQuery), ctl.TenantById)
		platform.POST("tenant", ctl.Perm(middleware.PermPlatformTenantAdd), ctl.TenantAdd)
		platform.PUT("tenant", ctl.Perm(middleware.PermPlatformTenantEdit), ctl.TenantUpdate)
		platform.DELETE("tenant/:id", ctl.Perm(middleware.PermPlatformTenantRemove), ctl.TenantDelete)

		// 系统配置是全局唯一一份（sys_config 没有 tenant_id），写接口只允许平台管理员：
		// 租户管理员天然拥有 system:config:* 权限码，若只校验权限码即可改写全平台配置。
		// 读接口（GET）保留在 v1Auth，租户侧页面/内部逻辑仍需读取配置。
		platform.POST("system-setup", ctl.Perm(middleware.PermConfigAdd), ctl.SysConfigAdd)
		platform.PUT("system-setup", ctl.Perm(middleware.PermConfigEdit), ctl.SysConfigUpdate)
		platform.DELETE("system-setup", ctl.Perm(middleware.PermConfigRemove), ctl.SysConfigDelete)
		platform.POST("system-setup/save", ctl.Perm(middleware.PermConfigEdit), ctl.SysSetupSave)

		// 在线备份：导出 Parquet + schema.sql 一致快照，不需停服（详见 controller 的 SystemBackup）
		platform.POST("system/backup", ctl.Perm(middleware.PermConfigEdit), ctl.SystemBackup)
	}
	/******* 网关管理（第三方工业网关 MQTT 接入）*******/
	{
		v1Auth.GET("gateways", ctl.Perm(middleware.PermGatewayList), ctl.GatewaySearch)
		v1Auth.GET("gateway/:id", ctl.Perm(middleware.PermGatewayQuery), ctl.GatewayById)
		v1Auth.POST("gateway", ctl.Perm(middleware.PermGatewayAdd), ctl.GatewayAdd)
		v1Auth.PUT("gateway", ctl.Perm(middleware.PermGatewayEdit), ctl.GatewayUpdate)
		v1Auth.DELETE("gateway/:id", ctl.Perm(middleware.PermGatewayRemove), ctl.GatewayDelete)
		v1Auth.PUT("gateway/config", ctl.Perm(middleware.PermGatewayConfig), ctl.GatewayPushConfig)
		v1Auth.POST("gateway/command", ctl.Perm(middleware.PermGatewayCommand), ctl.GatewayCommand)
	}
	/******* 设备接入 - 方案（行业模板一键应用）*******/
	{
		v1Auth.GET("solutions", ctl.Perm(middleware.PermSolutionList), ctl.SolutionSearch)
		v1Auth.GET("solution/:key", ctl.Perm(middleware.PermSolutionQuery), ctl.SolutionByKey)
		v1Auth.POST("solution/:key/apply", ctl.Perm(middleware.PermSolutionApply), ctl.SolutionApply)
		// 方案物模型条目维护：内置方案只读，租户首次修改会自动生成租户副本
		v1Auth.POST("solution/:key/thing-model/item", ctl.Perm(middleware.PermSolutionTslAdd), ctl.SolutionThingModelItemAdd)
		v1Auth.PUT("solution/:key/thing-model/item", ctl.Perm(middleware.PermSolutionTslEdit), ctl.SolutionThingModelItemUpdate)
		v1Auth.DELETE("solution/:key/thing-model/item", ctl.Perm(middleware.PermSolutionTslRemove), ctl.SolutionThingModelItemDelete)
	}
	/*******首页 *******/
	{
		v1Auth.GET("home-page", ctl.HomePage)
	}
	{
		/******* 运维监控 *******/
		v1Auth.GET("/metrics/system", ctl.Perm(middleware.PermMonitorView), ctl.SystemMetricsHandler)
	}

	/******* 镜像仓库管理 *******/
	{
		v1Auth.POST("docker-configs", ctl.Perm(middleware.PermImageAdd), ctl.DockerConfigAdd)
		v1Auth.GET("docker-configs", ctl.Perm(middleware.PermImageList), ctl.DockerConfigsSearch)
		v1Auth.PUT("docker-configs/:dockerConfigId", ctl.Perm(middleware.PermImageEdit), ctl.DockerConfigUpdate)
		v1Auth.DELETE("docker-configs/:dockerConfigId", ctl.Perm(middleware.PermImageRemove), ctl.DockerConfigDelete)
	}

	/*******驱动管理 *******/
	{
		v1Auth.POST("device-libraries", ctl.Perm(middleware.PermDriveAdd), ctl.DeviceLibraryAdd)
		v1Auth.GET("device-libraries", ctl.Perm(middleware.PermDriveList), ctl.DeviceLibrariesSearch)
		v1Auth.DELETE("device-libraries/:deviceLibraryId", ctl.Perm(middleware.PermDriveRemove), ctl.DeviceLibraryDelete)
		v1Auth.PUT("device-libraries/:deviceLibraryId", ctl.Perm(middleware.PermDriveEdit), ctl.DeviceLibraryUpdate)
		v1Auth.GET("driver-classify", ctl.Perm(middleware.PermDriveList), ctl.Perm(middleware.PermImageList), ctl.DeviceClassify)

	}
	/*******驱动实例 *******/
	{
		v1Auth.GET("device-servers", ctl.Perm(middleware.PermDriveList), ctl.DeviceServicesSearch)
		v1Auth.PUT("device-server/:deviceServiceId", ctl.Perm(middleware.PermDriveEdit), ctl.DeviceServiceUpdate)
	}

	/*******驱动市场分类 *******/
	{
		v1Auth.GET("device-classify", ctl.Perm(middleware.PermDriveList), ctl.Perm(middleware.PermImageList), ctl.DeviceClassify)
	}

	/*******产品管理 *******/
	{
		v1Auth.GET("products", ctl.Perm(middleware.PermProductList), ctl.ProductsSearch)
		v1Auth.GET("product/:productId", ctl.Perm(middleware.PermProductQuery), ctl.ProductById)
		v1Auth.POST("product", ctl.Perm(middleware.PermProductAdd), ctl.ProductAdd)
		v1Auth.POST("product-release/:productId", ctl.Perm(middleware.PermProductEdit), ctl.ProductRelease)
		v1Auth.POST("product-unrelease/:productId", ctl.Perm(middleware.PermProductEdit), ctl.ProductUnRelease)
		v1Auth.DELETE("product/:productId", ctl.Perm(middleware.PermProductDel), ctl.ProductDelete)
		v1Auth.GET("iot-platform", ctl.IotPlatform)
	}
	/*******产品物模型管理 *******/
	{
		v1Auth.GET("thingmodel/system", ctl.Perm(middleware.PermProductQuery), ctl.Perm(middleware.PermDeviceQuery), ctl.SystemThingModelSearch)
		v1Auth.POST("thingmodel", ctl.Perm(middleware.PermProductEdit), ctl.ThingModelAdd)
		v1Auth.PUT("thingmodel", ctl.Perm(middleware.PermProductEdit), ctl.ThingModelUpdate)
		v1Auth.DELETE("thingmodel", ctl.Perm(middleware.PermProductEdit), ctl.ThingModelDelete)
		v1Auth.GET("thingmodel/unit", ctl.Perm(middleware.PermProductQuery), ctl.Perm(middleware.PermDeviceQuery), ctl.ThingModelUnit)

	}

	/*******设备管理 *******/
	{
		v1Auth.POST("device", ctl.Perm(middleware.PermDeviceAdd), ctl.DeviceByAdd)
		v1Auth.GET("devices", ctl.Perm(middleware.PermDeviceList), ctl.DevicesSearch)
		v1Auth.GET("device/:deviceId", ctl.Perm(middleware.PermDeviceQuery), ctl.DeviceById)
		v1Auth.DELETE("device/:deviceId", ctl.Perm(middleware.PermDeviceDel), ctl.DeviceDelete)
		v1Auth.DELETE("devices", ctl.Perm(middleware.PermDeviceDel), ctl.DevicesDelete)
		v1Auth.PUT("device/:deviceId", ctl.Perm(middleware.PermDeviceEdit), ctl.DeviceUpdate)
		v1Auth.GET("device-mqtt/:deviceId", ctl.Perm(middleware.PermDeviceQuery), ctl.DeviceMqttInfoById)
		v1Auth.POST("device-mqtt", ctl.Perm(middleware.PermDeviceEdit), ctl.AddMqttAuth)
		v1Auth.GET("device/:deviceId/thing-model/property", ctl.Perm(middleware.PermDeviceQuery), ctl.DeviceThingModelPropertyDataSearch)
		v1Auth.GET("device/:deviceId/thing-model/history-property", ctl.Perm(middleware.PermDeviceQuery), ctl.DeviceThingModelHistoryPropertyDataSearch)
		v1Auth.GET("device/:deviceId/thing-model/event", ctl.Perm(middleware.PermDeviceQuery), ctl.DeviceThingModelEventDataSearch)
		v1Auth.GET("device/:deviceId/thing-model/service", ctl.Perm(middleware.PermDeviceQuery), ctl.DeviceThingModelServiceDataSearch)
		v1Auth.GET("device/status-template", ctl.Perm(middleware.PermDeviceList), ctl.DeviceStatusTemplate)

		// ===== 高级能力：资源管理 =====
		v1Auth.GET("typeresource", ctl.Perm(middleware.PermResourceList), ctl.TypeResource)
		v1Auth.GET("dataresource", ctl.Perm(middleware.PermResourceList), ctl.DataResourceSearch)
		v1Auth.POST("dataresource", ctl.Perm(middleware.PermResourceAdd), ctl.DataResourceAdd)
		v1Auth.PUT("dataresource", ctl.Perm(middleware.PermResourceEdit), ctl.DataResourceUpdate)
		v1Auth.DELETE("dataresource/:dataResourceId", ctl.Perm(middleware.PermResourceRemove), ctl.DataResourceDelete)
		v1Auth.POST("dataresource/:dataResourceId/health", ctl.Perm(middleware.PermResourceVerify), ctl.DataResourceVerify)

		// ===== 高级能力：规则引擎 =====
		v1Auth.GET("rule-engine", ctl.Perm(middleware.PermRuleList), ctl.RuleEngineSearch)
		v1Auth.POST("rule-engine", ctl.Perm(middleware.PermRuleAdd), ctl.RuleEngineAdd)
		v1Auth.PUT("rule-engine", ctl.Perm(middleware.PermRuleEdit), ctl.RuleEngineUpdate)
		v1Auth.GET("rule-engine/:ruleEngineId", ctl.Perm(middleware.PermRuleQuery), ctl.RuleEngineById)
		v1Auth.GET("rule-engine/:ruleEngineId/status", ctl.Perm(middleware.PermRuleQuery), ctl.RuleEngineStatus)
		v1Auth.DELETE("rule-engine/:ruleEngineId/delete", ctl.Perm(middleware.PermRuleRemove), ctl.RuleEngineDelete)
		v1Auth.POST("rule-engine/:ruleEngineId/start", ctl.Perm(middleware.PermRuleStart), ctl.RuleEngineStart)
		v1Auth.POST("rule-engine/:ruleEngineId/stop", ctl.Perm(middleware.PermRuleStop), ctl.RuleEngineStop)

		// ===== 高级能力：场景联动 =====
		v1Auth.GET("scene", ctl.Perm(middleware.PermSceneList), ctl.SceneSearch)
		v1Auth.POST("scene", ctl.Perm(middleware.PermSceneAdd), ctl.SceneAdd)
		v1Auth.PUT("scene", ctl.Perm(middleware.PermSceneEdit), ctl.SceneUpdate)
		v1Auth.DELETE("scene/:sceneId", ctl.Perm(middleware.PermSceneRemove), ctl.SceneDelete)
		v1Auth.GET("scene/:sceneId/log", ctl.Perm(middleware.PermSceneQuery), ctl.SceneLog)
		v1Auth.POST("scene/:sceneId/start", ctl.Perm(middleware.PermSceneStart), ctl.SceneStart)
		v1Auth.POST("scene/:sceneId/stop", ctl.Perm(middleware.PermSceneStop), ctl.SceneStop)
		v1Auth.GET("devices/import-template", ctl.Perm(middleware.PermDeviceImport), ctl.DeviceImportTemplateDownload)
		v1Auth.POST("devices/import", ctl.Perm(middleware.PermDeviceImport), ctl.DevicesImport)
		v1Auth.POST("device/upload-validated", ctl.Perm(middleware.PermDeviceImport), ctl.UploadValidated)
		v1Auth.PUT("devices/bind-driver", ctl.Perm(middleware.PermDeviceBind), ctl.DevicesBindDriver)
		v1Auth.PUT("devices/unbind-driver", ctl.Perm(middleware.PermDeviceBind), ctl.DevicesUnBindDriver)
		v1Auth.PUT("devices/bind-product", ctl.Perm(middleware.PermDeviceBind), ctl.DevicesBindByProductId)

	}
	/*******品类、物模型同步接口 *******/
	{
		v1Auth.GET("category-template", ctl.Perm(middleware.PermProductQuery), ctl.Perm(middleware.PermDeviceQuery), ctl.CategoryTemplateSearch)
		v1Auth.GET("thingmodel-template", ctl.Perm(middleware.PermProductQuery), ctl.Perm(middleware.PermDeviceQuery), ctl.ThingModelTemplateSearch)
		v1Auth.GET("thingmodel-template/:categoryKey", ctl.Perm(middleware.PermProductQuery), ctl.Perm(middleware.PermDeviceQuery), ctl.ThingModelTemplateByCategoryKey)
	}

	/*******告警中心接口 *******/
	{
		v1Auth.POST("alert-rule", ctl.Perm(middleware.PermAlertAdd), ctl.AlertRuleAdd)
		v1Auth.PUT("alert-rule/:ruleId", ctl.Perm(middleware.PermAlertEdit), ctl.AlertRuleUpdate)
		v1Auth.PUT("rule-field", ctl.Perm(middleware.PermAlertEdit), ctl.AlertRuleUpdateField)
		v1Auth.GET("alert-rule/:ruleId", ctl.Perm(middleware.PermAlertQuery), ctl.AlertRuleById)
		v1Auth.GET("alert-rule", ctl.Perm(middleware.PermAlertList), ctl.AlertRuleSearch)
		v1Auth.DELETE("alert-rule/:ruleId", ctl.Perm(middleware.PermAlertRemove), ctl.AlertRuleDelete)
		v1Auth.POST("alert-rule/:ruleId/start", ctl.Perm(middleware.PermAlertEdit), ctl.AlertRuleStart)
		v1Auth.POST("alert-rule/:ruleId/stop", ctl.Perm(middleware.PermAlertEdit), ctl.AlertRuleStop)
		v1Auth.POST("alert-rule/:ruleId/restart", ctl.Perm(middleware.PermAlertEdit), ctl.AlertRuleRestart)
		v1Auth.GET("alert-list", ctl.Perm(middleware.PermAlertList), ctl.AlertSearch)
		v1Auth.GET("alert-plate", ctl.Perm(middleware.PermAlertList), ctl.AlertPlate)
		v1Auth.PUT("alert-ignore/:ruleId", ctl.Perm(middleware.PermAlertHandle), ctl.AlertIgnore)
		v1Auth.POST("alert-treated", ctl.Perm(middleware.PermAlertHandle), ctl.AlertTreated)

	}

	/*******文档中心（sdk） *******/
	{

		v1Auth.GET("language-sdk", ctl.Perm(middleware.PermDocsView), ctl.LanguageSdkSearch)
	}

	/******* 系统管理 ********/
	{
		// 个人中心
		v1Auth.GET("user/profile", ctl.Perm(middleware.PermUserQuery), ctl.UserProfile)
		v1Auth.PUT("user/profile", ctl.UserProfileUpdate)
		// 语言偏好：切换语言时前端调它，换设备登录也能保持一致
		v1Auth.PUT("user/lang", ctl.UserLangUpdate)
		v1Auth.GET("user/getinfo", ctl.UserGetInfo)

		// 用户管理
		v1Auth.GET("sys-user", ctl.Perm(middleware.PermUserList), ctl.SysUserSearch)
		v1Auth.GET("sys-user/:id", ctl.Perm(middleware.PermUserQuery), ctl.SysUserById)
		v1Auth.POST("sys-user", ctl.Perm(middleware.PermUserAdd), ctl.SysUserAdd)
		v1Auth.PUT("sys-user", ctl.Perm(middleware.PermUserEdit), ctl.SysUserUpdate)
		v1Auth.DELETE("sys-user", ctl.Perm(middleware.PermUserRemove), ctl.SysUserDelete)
		v1Auth.PUT("sys-user/status/:id", ctl.Perm(middleware.PermUserEdit), ctl.SysUserChangeStatus)
		v1Auth.PUT("pwd/reset/:id", ctl.Perm(middleware.PermUserEdit), ctl.SysUserResetPassword)

		// 角色管理
		v1Auth.GET("role", ctl.Perm(middleware.PermRoleList), ctl.SysRoleSearch)
		v1Auth.GET("role/all", ctl.Perm(middleware.PermRoleList, middleware.PermUserList), ctl.SysRoleAll)
		v1Auth.GET("role/:id", ctl.Perm(middleware.PermRoleQuery), ctl.SysRoleById)
		v1Auth.POST("role", ctl.Perm(middleware.PermRoleAdd), ctl.SysRoleAdd)
		v1Auth.PUT("role", ctl.Perm(middleware.PermRoleEdit), ctl.SysRoleUpdate)
		v1Auth.DELETE("role", ctl.Perm(middleware.PermRoleRemove), ctl.SysRoleDelete)
		v1Auth.GET("menurole", ctl.Perm(middleware.PermRoleQuery), ctl.SysRoleMenus)
		v1Auth.POST("menurole", ctl.Perm(middleware.PermRoleEdit), ctl.SysRoleSetMenus)

		// 菜单管理
		v1Auth.GET("menu", ctl.Perm(middleware.PermMenuList), ctl.SysMenuSearch)
		v1Auth.GET("menu/tree", ctl.Perm(middleware.PermMenuList), ctl.SysMenuTree)
		v1Auth.GET("menu/:id", ctl.Perm(middleware.PermMenuQuery), ctl.SysMenuById)
		v1Auth.POST("menu", ctl.Perm(middleware.PermMenuAdd), ctl.SysMenuAdd)
		v1Auth.PUT("menu", ctl.Perm(middleware.PermMenuEdit), ctl.SysMenuUpdate)
		v1Auth.DELETE("menu", ctl.Perm(middleware.PermMenuRemove), ctl.SysMenuDelete)
		// 租户展示层覆盖（隐藏/改名/图标/排序）与平台重置租户菜单定制
		v1Auth.GET("menu/pref", ctl.Perm(middleware.PermMenuList), ctl.SysMenuPrefs)
		v1Auth.PUT("menu/pref", ctl.Perm(middleware.PermMenuCustomPref), ctl.SysMenuPrefSave)
		v1Auth.POST("menu/tenant-reset", ctl.Perm(middleware.PermMenuCustomReset), ctl.SysMenuTenantReset)

		// 部门管理
		v1Auth.GET("dept", ctl.Perm(middleware.PermDeptList), ctl.SysDeptSearch)
		v1Auth.GET("dept-tree", ctl.Perm(middleware.PermDeptList), ctl.SysDeptTree)
		v1Auth.GET("dept/:id", ctl.Perm(middleware.PermDeptQuery), ctl.SysDeptById)
		v1Auth.POST("dept", ctl.Perm(middleware.PermDeptAdd), ctl.SysDeptAdd)
		v1Auth.PUT("dept", ctl.Perm(middleware.PermDeptEdit), ctl.SysDeptUpdate)
		v1Auth.DELETE("dept", ctl.Perm(middleware.PermDeptRemove), ctl.SysDeptDelete)

		// 审查日志（登录 + 操作）
		v1Auth.GET("sys-audit-log", ctl.Perm(middleware.PermAuditLogList), ctl.SysAuditLogSearch)
		v1Auth.DELETE("sys-audit-log", ctl.Perm(middleware.PermAuditLogRemove), ctl.SysAuditLogDelete)
		v1Auth.DELETE("sys-audit-log/clear", ctl.Perm(middleware.PermAuditLogRemove), ctl.SysAuditLogClear)

		// 兼容别名：旧「登入日志」接口与权限码保留一版，行为等价 type=login（下版本删除）
		v1Auth.GET("sys-login-log",
			ctl.Perm(middleware.PermAuditLogList, middleware.PermLoginLogList), ctl.SysLoginLogSearch)
		v1Auth.DELETE("sys-login-log",
			ctl.Perm(middleware.PermAuditLogRemove, middleware.PermLoginLogRemove), ctl.SysLoginLogDelete)
		v1Auth.DELETE("sys-login-log/clear",
			ctl.Perm(middleware.PermAuditLogRemove, middleware.PermLoginLogRemove), ctl.SysLoginLogClear)

		// 系统配置：只保留读接口给租户；写接口在「平台管理」组（PlatformOnly）
		v1Auth.GET("system-setup", ctl.Perm(middleware.PermConfigList), ctl.SysConfigSearch)
		v1Auth.GET("system-setup/:id", ctl.Perm(middleware.PermConfigQuery), ctl.SysConfigById)
	}

}
