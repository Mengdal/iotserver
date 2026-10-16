/*******************************************************************************
 * Copyright 2017.
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

package middleware

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"iotserver/internal/iotserver/core/container"
	"iotserver/internal/pkg/constants"
	pkgcontainer "iotserver/internal/pkg/container"
	"iotserver/internal/pkg/di"
	"iotserver/internal/pkg/errort"
	"iotserver/internal/pkg/httphelper"
	"iotserver/internal/pkg/logger"
	"iotserver/internal/pkg/permissions"
	"iotserver/internal/pkg/utils"
)

// 权限标识统一来自 internal/pkg/permissions（单一来源），
// 这里保留常量别名，便于路由侧书写。
const (
	PermHomeView = permissions.HomeView

	PermUserList   = permissions.UserList
	PermUserQuery  = permissions.UserQuery
	PermUserAdd    = permissions.UserAdd
	PermUserEdit   = permissions.UserEdit
	PermUserRemove = permissions.UserRemove

	PermRoleList   = permissions.RoleList
	PermRoleQuery  = permissions.RoleQuery
	PermRoleAdd    = permissions.RoleAdd
	PermRoleEdit   = permissions.RoleEdit
	PermRoleRemove = permissions.RoleRemove

	PermMenuList   = permissions.MenuList
	PermMenuQuery  = permissions.MenuQuery
	PermMenuAdd    = permissions.MenuAdd
	PermMenuEdit   = permissions.MenuEdit
	PermMenuRemove = permissions.MenuRemove
	// 租户菜单展示层覆盖（隐藏/改名/图标/排序）与平台重置租户菜单定制
	PermMenuCustomPref  = permissions.MenuCustomPref
	PermMenuCustomReset = permissions.MenuCustomReset

	PermDeptList   = permissions.DeptList
	PermDeptQuery  = permissions.DeptQuery
	PermDeptAdd    = permissions.DeptAdd
	PermDeptEdit   = permissions.DeptEdit
	PermDeptRemove = permissions.DeptRemove

	// 审查日志（登录 + 操作）
	PermAuditLogList   = permissions.AuditLogList
	PermAuditLogRemove = permissions.AuditLogRemove

	// 兼容别名：旧权限码保留一版，下版本删除
	PermLoginLogList   = permissions.LoginLogList
	PermLoginLogRemove = permissions.LoginLogRemove

	PermConfigList   = permissions.ConfigList
	PermConfigQuery  = permissions.ConfigQuery
	PermConfigAdd    = permissions.ConfigAdd
	PermConfigEdit   = permissions.ConfigEdit
	PermConfigRemove = permissions.ConfigRemove

	PermProductList  = permissions.ProductList
	PermProductQuery = permissions.ProductQuery
	PermProductAdd   = permissions.ProductAdd
	PermProductEdit  = permissions.ProductEdit
	PermProductDel   = permissions.ProductDel

	PermDeviceList   = permissions.DeviceList
	PermDeviceQuery  = permissions.DeviceQuery
	PermDeviceAdd    = permissions.DeviceAdd
	PermDeviceEdit   = permissions.DeviceEdit
	PermDeviceDel    = permissions.DeviceDel
	PermDeviceImport = permissions.DeviceImport
	PermDeviceBind   = permissions.DeviceBind

	PermImageList   = permissions.ImageList
	PermImageAdd    = permissions.ImageAdd
	PermImageEdit   = permissions.ImageEdit
	PermImageRemove = permissions.ImageRemove

	PermDriveList   = permissions.DriveList
	PermDriveQuery  = permissions.DriveQuery
	PermDriveAdd    = permissions.DriveAdd
	PermDriveEdit   = permissions.DriveEdit
	PermDriveRemove = permissions.DriveRemove

	PermMonitorView = permissions.MonitorView

	PermAlertList   = permissions.AlertList
	PermAlertQuery  = permissions.AlertQuery
	PermAlertAdd    = permissions.AlertAdd
	PermAlertEdit   = permissions.AlertEdit
	PermAlertRemove = permissions.AlertRemove
	PermAlertHandle = permissions.AlertHandle

	PermDocsView = permissions.DocsView

	// 系统调试
	PermDebugPprof = permissions.SystemDebugPprof

	// 高级能力 - 资源管理
	PermResourceList   = permissions.AdvancedResourceList
	PermResourceAdd    = permissions.AdvancedResourceAdd
	PermResourceEdit   = permissions.AdvancedResourceEdit
	PermResourceRemove = permissions.AdvancedResourceRemove
	PermResourceVerify = permissions.AdvancedResourceVerify

	// 高级能力 - 规则引擎
	PermRuleList   = permissions.AdvancedRuleList
	PermRuleQuery  = permissions.AdvancedRuleQuery
	PermRuleAdd    = permissions.AdvancedRuleAdd
	PermRuleEdit   = permissions.AdvancedRuleEdit
	PermRuleRemove = permissions.AdvancedRuleRemove
	PermRuleStart  = permissions.AdvancedRuleStart
	PermRuleStop   = permissions.AdvancedRuleStop

	// 高级能力 - 场景联动
	PermSceneList   = permissions.AdvancedSceneList
	PermSceneQuery  = permissions.AdvancedSceneQuery
	PermSceneAdd    = permissions.AdvancedSceneAdd
	PermSceneEdit   = permissions.AdvancedSceneEdit
	PermSceneRemove = permissions.AdvancedSceneRemove
	PermSceneStart  = permissions.AdvancedSceneStart
	PermSceneStop   = permissions.AdvancedSceneStop

	// 可视化看板
	PermVizBoardList   = permissions.VizBoardList
	PermVizBoardQuery  = permissions.VizBoardQuery
	PermVizBoardAdd    = permissions.VizBoardAdd
	PermVizBoardEdit   = permissions.VizBoardEdit
	PermVizBoardRemove = permissions.VizBoardRemove
	PermVizDataQuery   = permissions.VizDataQuery

	// 平台管理 - 租户
	PermPlatformTenantList   = permissions.PlatformTenantList
	PermPlatformTenantQuery  = permissions.PlatformTenantQuery
	PermPlatformTenantAdd    = permissions.PlatformTenantAdd
	PermPlatformTenantEdit   = permissions.PlatformTenantEdit
	PermPlatformTenantRemove = permissions.PlatformTenantRemove
	PermPlatformTenantSwitch = permissions.PlatformTenantSwitch

	// 网关（第三方工业网关接入）
	PermGatewayList    = permissions.GatewayList
	PermGatewayQuery   = permissions.GatewayQuery
	PermGatewayAdd     = permissions.GatewayAdd
	PermGatewayEdit    = permissions.GatewayEdit
	PermGatewayRemove  = permissions.GatewayRemove
	PermGatewayConfig  = permissions.GatewayConfig
	PermGatewayCommand = permissions.GatewayCommand

	// 方案（行业模板一键应用）
	PermSolutionList  = permissions.SolutionList
	PermSolutionQuery = permissions.SolutionQuery
	PermSolutionApply = permissions.SolutionApply
	// 方案物模型条目维护
	PermSolutionTslAdd    = permissions.SolutionTslAdd
	PermSolutionTslEdit   = permissions.SolutionTslEdit
	PermSolutionTslRemove = permissions.SolutionTslRemove
)

// PermRequired 生成权限校验中间件
// 需要放在 JWT 鉴权中间件之后，依赖 c.Get(constants.JwtParsedInfo)
func PermRequired(dic *di.Container, perms ...string) gin.HandlerFunc {
	userItf := container.UserItfFrom(dic.Get)
	lc := pkgcontainer.LoggingClientFrom(dic.Get)
	required := make([]string, 0, len(perms))
	for _, p := range perms {
		if p != "" {
			required = append(required, p)
		}
	}
	return func(c *gin.Context) {
		if len(required) == 0 {
			c.Next()
			return
		}
		username, ok := UsernameFromContext(c)
		if !ok {
			renderPermissionDenied(c, lc)
			return
		}

		// 超级管理员放行
		if userItf.IsAdminUsername(username) {
			c.Next()
			return
		}

		owned, err := userItf.PermissionsByUsername(username)
		if err != nil {
			lc.Errorf("load user permissions error %v", err)
			renderPermissionDenied(c, lc)
			return
		}
		for _, need := range required {
			if utils.InStringSlice(need, owned) {
				c.Next()
				return
			}
		}
		renderPermissionDenied(c, lc)
	}
}

// UsernameFromContext 从上下文获取当前登录用户名
func UsernameFromContext(c *gin.Context) (string, bool) {
	value, ok := c.Get(constants.JwtParsedInfo)
	if !ok {
		return "", false
	}
	claims, ok := value.(*CustomClaims)
	if !ok || claims == nil || claims.Username == "" {
		return "", false
	}
	return claims.Username, true
}

func renderPermissionDenied(c *gin.Context, lc logger.LoggingClient) {
	err := errort.NewCommonErr(errort.AppPermissionDenied, fmt.Errorf("permission denied"))
	httphelper.RenderFail(c, err, c.Writer, lc)
	c.Abort()
}
