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

package gateway

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"iotserver/internal/dtos"
	"iotserver/internal/models"
	"iotserver/internal/pkg/constants"
	"iotserver/internal/pkg/errort"
	"iotserver/internal/pkg/httphelper"
	"iotserver/internal/pkg/middleware"
)

/******************************* 用户管理 *******************************/

// @Tags    系统管理-用户
// @Summary 用户列表
// @Produce json
// @Success 200 {object} httphelper.CommonResponse
// @Router  /api/v1/sys-user [get]
func (ctl *controller) SysUserSearch(c *gin.Context) {
	lc := ctl.lc
	var req dtos.SysUserSearchRequest
	if err := c.ShouldBind(&req); err != nil {
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultReqParamsError, err), c.Writer, lc)
		return
	}
	list, total, err := ctl.getUserApp().SysUserSearch(c, req)
	if err != nil {
		httphelper.RenderFail(c, err, c.Writer, lc)
		return
	}
	httphelper.ResultSuccess(httphelper.NewPageResult(list, total, req.Page, req.PageSize), c.Writer, lc)
}

// @Tags    系统管理-用户
// @Summary 用户详情
// @Produce json
// @Success 200 {object} httphelper.CommonResponse
// @Router  /api/v1/sys-user/{id} [get]
func (ctl *controller) SysUserById(c *gin.Context) {
	lc := ctl.lc
	res, err := ctl.getUserApp().SysUserById(c, c.Param("id"))
	if err != nil {
		httphelper.RenderFail(c, err, c.Writer, lc)
		return
	}
	httphelper.ResultSuccess(res, c.Writer, lc)
}

// @Tags    系统管理-用户
// @Summary 新增用户
// @Produce json
// @Success 200 {object} httphelper.CommonResponse
// @Router  /api/v1/sys-user [post]
func (ctl *controller) SysUserAdd(c *gin.Context) {
	lc := ctl.lc
	var req dtos.SysUserAddRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultReqParamsError, err), c.Writer, lc)
		return
	}
	if req.Status == 0 {
		req.Status = 1
	}
	if err := ctl.getUserApp().SysUserAdd(c, req); err != nil {
		httphelper.RenderFail(c, err, c.Writer, lc)
		return
	}
	httphelper.ResultSuccess(nil, c.Writer, lc)
}

// @Tags    系统管理-用户
// @Summary 编辑用户
// @Produce json
// @Success 200 {object} httphelper.CommonResponse
// @Router  /api/v1/sys-user [put]
func (ctl *controller) SysUserUpdate(c *gin.Context) {
	lc := ctl.lc
	var req dtos.SysUserUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultReqParamsError, err), c.Writer, lc)
		return
	}
	if err := ctl.getUserApp().SysUserUpdate(c, req); err != nil {
		httphelper.RenderFail(c, err, c.Writer, lc)
		return
	}
	httphelper.ResultSuccess(nil, c.Writer, lc)
}

// @Tags    系统管理-用户
// @Summary 删除用户
// @Produce json
// @Success 200 {object} httphelper.CommonResponse
// @Router  /api/v1/sys-user [delete]
func (ctl *controller) SysUserDelete(c *gin.Context) {
	lc := ctl.lc
	var req dtos.SysUserIdsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultReqParamsError, err), c.Writer, lc)
		return
	}
	if err := ctl.getUserApp().SysUserDelete(c, req.Ids); err != nil {
		httphelper.RenderFail(c, err, c.Writer, lc)
		return
	}
	httphelper.ResultSuccess(nil, c.Writer, lc)
}

// @Tags    系统管理-用户
// @Summary 重置密码
// @Produce json
// @Success 200 {object} httphelper.CommonResponse
// @Router  /api/v1/pwd/reset [put]
func (ctl *controller) SysUserResetPassword(c *gin.Context) {
	lc := ctl.lc
	var req dtos.SysUserResetPwdRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultReqParamsError, err), c.Writer, lc)
		return
	}
	if err := ctl.getUserApp().SysUserResetPassword(c, c.Param("id"), req); err != nil {
		httphelper.RenderFail(c, err, c.Writer, lc)
		return
	}
	httphelper.ResultSuccess(nil, c.Writer, lc)
}

// @Tags    系统管理-用户
// @Summary 启用/停用用户
// @Produce json
// @Success 200 {object} httphelper.CommonResponse
// @Router  /api/v1/sys-user/status/{id} [put]
func (ctl *controller) SysUserChangeStatus(c *gin.Context) {
	lc := ctl.lc
	var req struct {
		Status int `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultReqParamsError, err), c.Writer, lc)
		return
	}
	if err := ctl.getUserApp().SysUserChangeStatus(c, c.Param("id"), req.Status); err != nil {
		httphelper.RenderFail(c, err, c.Writer, lc)
		return
	}
	httphelper.ResultSuccess(nil, c.Writer, lc)
}

// @Tags    站点信息
// @Summary 站点品牌配置（免鉴权，登录页使用）
// @Produce json
// @Success 200 {object} httphelper.CommonResponse
// @Router  /api/v1/site-info [get]
func (ctl *controller) SiteInfo(c *gin.Context) {
	lc := ctl.lc
	res, err := ctl.getUserApp().SiteInfo(c)
	if err != nil {
		httphelper.RenderFail(c, err, c.Writer, lc)
		return
	}
	httphelper.ResultSuccess(res, c.Writer, lc)
}

/******************************* 个人中心 *******************************/

// @Tags    系统管理-个人中心
// @Summary 查询当前用户信息
// @Produce json
// @Success 200 {object} httphelper.CommonResponse
// @Router  /api/v1/user/profile [get]
func (ctl *controller) UserProfile(c *gin.Context) {
	lc := ctl.lc
	username, ok := currentUsername(c)
	if !ok {
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultTokenPermission, fmt.Errorf("token is invalid")), c.Writer, lc)
		return
	}
	res, err := ctl.getUserApp().CurrentUserProfile(c, username)
	if err != nil {
		httphelper.RenderFail(c, err, c.Writer, lc)
		return
	}
	httphelper.ResultSuccess(res, c.Writer, lc)
}

// @Tags    系统管理-个人中心
// @Summary 修改当前用户信息
// @Produce json
// @Success 200 {object} httphelper.CommonResponse
// @Router  /api/v1/user/profile [put]
func (ctl *controller) UserProfileUpdate(c *gin.Context) {
	lc := ctl.lc
	username, ok := currentUsername(c)
	if !ok {
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultTokenPermission, fmt.Errorf("token is invalid")), c.Writer, lc)
		return
	}
	var req dtos.SysUserProfileUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultReqParamsError, err), c.Writer, lc)
		return
	}
	if err := ctl.getUserApp().UpdateCurrentUserProfile(c, username, req); err != nil {
		httphelper.RenderFail(c, err, c.Writer, lc)
		return
	}
	httphelper.ResultSuccess(nil, c.Writer, lc)
}

// @Tags    系统管理-个人中心
// @Summary 切换当前用户语言
// @Produce json
// @Success 200 {object} httphelper.CommonResponse
// @Router  /api/v1/user/lang [put]
func (ctl *controller) UserLangUpdate(c *gin.Context) {
	lc := ctl.lc
	username, ok := currentUsername(c)
	if !ok {
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultTokenPermission, fmt.Errorf("token is invalid")), c.Writer, lc)
		return
	}
	var req dtos.UpdateLangRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultReqParamsError, err), c.Writer, lc)
		return
	}
	if err := ctl.getUserApp().UpdateCurrentUserLang(c, username, req); err != nil {
		httphelper.RenderFail(c, err, c.Writer, lc)
		return
	}
	httphelper.ResultSuccess(nil, c.Writer, lc)
}

// @Tags    系统管理-个人中心
// @Summary 查询当前用户菜单
// @Produce json
// @Success 200 {object} httphelper.CommonResponse
// @Router  /api/v1/user/getinfo [get]
func (ctl *controller) UserGetInfo(c *gin.Context) {
	lc := ctl.lc
	username, ok := currentUsername(c)
	if !ok {
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultTokenPermission, fmt.Errorf("token is invalid")), c.Writer, lc)
		return
	}
	menus, err := ctl.getUserApp().MenuRoutesByUsername(username)
	if err != nil {
		httphelper.RenderFail(c, err, c.Writer, lc)
		return
	}
	perms, err := ctl.getUserApp().PermissionsByUsername(username)
	if err != nil {
		httphelper.RenderFail(c, err, c.Writer, lc)
		return
	}
	httphelper.ResultSuccess(gin.H{
		"menus":       menus,
		"permissions": perms,
		"admin":       ctl.getUserApp().IsAdminUsername(username),
	}, c.Writer, lc)
}

/******************************* 角色管理 *******************************/

// @Tags    系统管理-角色
// @Summary 角色列表
// @Produce json
// @Success 200 {object} httphelper.CommonResponse
// @Router  /api/v1/role [get]
func (ctl *controller) SysRoleSearch(c *gin.Context) {
	lc := ctl.lc
	var req dtos.SysRoleSearchRequest
	if err := c.ShouldBind(&req); err != nil {
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultReqParamsError, err), c.Writer, lc)
		return
	}
	list, total, err := ctl.getUserApp().SysRoleSearch(c, req)
	if err != nil {
		httphelper.RenderFail(c, err, c.Writer, lc)
		return
	}
	httphelper.ResultSuccess(httphelper.NewPageResult(list, total, req.Page, req.PageSize), c.Writer, lc)
}

// @Tags    系统管理-角色
// @Summary 全部角色
// @Produce json
// @Success 200 {object} httphelper.CommonResponse
// @Router  /api/v1/role/all [get]
func (ctl *controller) SysRoleAll(c *gin.Context) {
	lc := ctl.lc
	list, err := ctl.getUserApp().SysRoleAll(c)
	if err != nil {
		httphelper.RenderFail(c, err, c.Writer, lc)
		return
	}
	httphelper.ResultSuccess(list, c.Writer, lc)
}

// @Tags    系统管理-角色
// @Summary 角色详情
// @Produce json
// @Success 200 {object} httphelper.CommonResponse
// @Router  /api/v1/role/{id} [get]
func (ctl *controller) SysRoleById(c *gin.Context) {
	lc := ctl.lc
	res, err := ctl.getUserApp().SysRoleById(c, c.Param("id"))
	if err != nil {
		httphelper.RenderFail(c, err, c.Writer, lc)
		return
	}
	httphelper.ResultSuccess(res, c.Writer, lc)
}

// @Tags    系统管理-角色
// @Summary 新增角色
// @Produce json
// @Success 200 {object} httphelper.CommonResponse
// @Router  /api/v1/role [post]
func (ctl *controller) SysRoleAdd(c *gin.Context) {
	lc := ctl.lc
	var req dtos.SysRoleAddRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultReqParamsError, err), c.Writer, lc)
		return
	}
	if err := ctl.getUserApp().SysRoleAdd(c, req); err != nil {
		httphelper.RenderFail(c, err, c.Writer, lc)
		return
	}
	httphelper.ResultSuccess(nil, c.Writer, lc)
}

// @Tags    系统管理-角色
// @Summary 编辑角色
// @Produce json
// @Success 200 {object} httphelper.CommonResponse
// @Router  /api/v1/role [put]
func (ctl *controller) SysRoleUpdate(c *gin.Context) {
	lc := ctl.lc
	var req dtos.SysRoleUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultReqParamsError, err), c.Writer, lc)
		return
	}
	if err := ctl.getUserApp().SysRoleUpdate(c, req); err != nil {
		httphelper.RenderFail(c, err, c.Writer, lc)
		return
	}
	httphelper.ResultSuccess(nil, c.Writer, lc)
}

// @Tags    系统管理-角色
// @Summary 删除角色
// @Produce json
// @Success 200 {object} httphelper.CommonResponse
// @Router  /api/v1/role [delete]
func (ctl *controller) SysRoleDelete(c *gin.Context) {
	lc := ctl.lc
	var req dtos.SysRoleIdsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultReqParamsError, err), c.Writer, lc)
		return
	}
	if err := ctl.getUserApp().SysRoleDelete(c, req.Ids); err != nil {
		httphelper.RenderFail(c, err, c.Writer, lc)
		return
	}
	httphelper.ResultSuccess(nil, c.Writer, lc)
}

// @Tags    系统管理-角色
// @Summary 查询角色菜单
// @Produce json
// @Success 200 {object} httphelper.CommonResponse
// @Router  /api/v1/menurole [get]
func (ctl *controller) SysRoleMenus(c *gin.Context) {
	lc := ctl.lc
	roleId := c.Query("roleId")
	if roleId == "" {
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultReqParamsError, fmt.Errorf("roleId is required")), c.Writer, lc)
		return
	}
	menuIds, err := ctl.getUserApp().SysRoleMenus(c, roleId)
	if err != nil {
		httphelper.RenderFail(c, err, c.Writer, lc)
		return
	}
	httphelper.ResultSuccess(gin.H{"roleId": roleId, "menuIds": menuIds}, c.Writer, lc)
}

// @Tags    系统管理-角色
// @Summary 角色绑定菜单
// @Produce json
// @Success 200 {object} httphelper.CommonResponse
// @Router  /api/v1/menurole [post]
func (ctl *controller) SysRoleSetMenus(c *gin.Context) {
	lc := ctl.lc
	var req dtos.SysRoleMenuRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultReqParamsError, err), c.Writer, lc)
		return
	}
	if err := ctl.getUserApp().SysRoleSetMenus(c, req); err != nil {
		httphelper.RenderFail(c, err, c.Writer, lc)
		return
	}
	httphelper.ResultSuccess(nil, c.Writer, lc)
}

/******************************* 菜单管理 *******************************/

// @Tags    系统管理-菜单
// @Summary 菜单列表
// @Produce json
// @Success 200 {object} httphelper.CommonResponse
// @Router  /api/v1/menu [get]
func (ctl *controller) SysMenuSearch(c *gin.Context) {
	lc := ctl.lc
	var req dtos.SysMenuSearchRequest
	if err := c.ShouldBind(&req); err != nil {
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultReqParamsError, err), c.Writer, lc)
		return
	}
	if req.IsAll {
		list, err := ctl.getUserApp().SysMenuTree(c, req)
		if err != nil {
			httphelper.RenderFail(c, err, c.Writer, lc)
			return
		}
		httphelper.ResultSuccess(httphelper.NewPageResult(list, uint32(len(list)), req.Page, req.PageSize), c.Writer, lc)
		return
	}
	list, total, err := ctl.getUserApp().SysMenuSearch(c, req)
	if err != nil {
		httphelper.RenderFail(c, err, c.Writer, lc)
		return
	}
	httphelper.ResultSuccess(httphelper.NewPageResult(list, total, req.Page, req.PageSize), c.Writer, lc)
}

// @Tags    系统管理-菜单
// @Summary 菜单树
// @Produce json
// @Success 200 {object} httphelper.CommonResponse
// @Router  /api/v1/menu/tree [get]
func (ctl *controller) SysMenuTree(c *gin.Context) {
	lc := ctl.lc
	var req dtos.SysMenuSearchRequest
	if err := c.ShouldBind(&req); err != nil {
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultReqParamsError, err), c.Writer, lc)
		return
	}
	list, err := ctl.getUserApp().SysMenuTree(c, req)
	if err != nil {
		httphelper.RenderFail(c, err, c.Writer, lc)
		return
	}
	httphelper.ResultSuccess(list, c.Writer, lc)
}

// @Tags    系统管理-菜单
// @Summary 菜单详情
// @Produce json
// @Success 200 {object} httphelper.CommonResponse
// @Router  /api/v1/menu/{id} [get]
func (ctl *controller) SysMenuById(c *gin.Context) {
	lc := ctl.lc
	res, err := ctl.getUserApp().SysMenuById(c, c.Param("id"))
	if err != nil {
		httphelper.RenderFail(c, err, c.Writer, lc)
		return
	}
	httphelper.ResultSuccess(res, c.Writer, lc)
}

// @Tags    系统管理-菜单
// @Summary 新增菜单
// @Produce json
// @Success 200 {object} httphelper.CommonResponse
// @Router  /api/v1/menu [post]
func (ctl *controller) SysMenuAdd(c *gin.Context) {
	lc := ctl.lc
	var req dtos.SysMenuAddRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultReqParamsError, err), c.Writer, lc)
		return
	}
	if err := ctl.getUserApp().SysMenuAdd(c, req); err != nil {
		httphelper.RenderFail(c, err, c.Writer, lc)
		return
	}
	httphelper.ResultSuccess(nil, c.Writer, lc)
}

// @Tags    系统管理-菜单
// @Summary 编辑菜单
// @Produce json
// @Success 200 {object} httphelper.CommonResponse
// @Router  /api/v1/menu [put]
func (ctl *controller) SysMenuUpdate(c *gin.Context) {
	lc := ctl.lc
	var req dtos.SysMenuUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultReqParamsError, err), c.Writer, lc)
		return
	}
	if err := ctl.getUserApp().SysMenuUpdate(c, req); err != nil {
		httphelper.RenderFail(c, err, c.Writer, lc)
		return
	}
	httphelper.ResultSuccess(nil, c.Writer, lc)
}

// @Tags    系统管理-菜单
// @Summary 删除菜单
// @Produce json
// @Success 200 {object} httphelper.CommonResponse
// @Router  /api/v1/menu [delete]
func (ctl *controller) SysMenuDelete(c *gin.Context) {
	lc := ctl.lc
	var req dtos.SysMenuIdsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultReqParamsError, err), c.Writer, lc)
		return
	}
	if err := ctl.getUserApp().SysMenuDelete(c, req.Ids); err != nil {
		httphelper.RenderFail(c, err, c.Writer, lc)
		return
	}
	httphelper.ResultSuccess(nil, c.Writer, lc)
}

// @Tags    系统管理-菜单
// @Summary 当前租户的菜单展示层覆盖（隐藏/改名/图标/排序）
// @Produce json
// @Success 200 {object} httphelper.CommonResponse
// @Router  /api/v1/menu/pref [get]
func (ctl *controller) SysMenuPrefs(c *gin.Context) {
	lc := ctl.lc
	list, err := ctl.getUserApp().SysMenuPrefs(c)
	if err != nil {
		httphelper.RenderFail(c, err, c.Writer, lc)
		return
	}
	httphelper.ResultSuccess(list, c.Writer, lc)
}

// @Tags    系统管理-菜单
// @Summary 保存菜单展示层覆盖（隐藏/改名/排序，不覆盖 path/perms/component）
// @Produce json
// @Success 200 {object} httphelper.CommonResponse
// @Router  /api/v1/menu/pref [put]
func (ctl *controller) SysMenuPrefSave(c *gin.Context) {
	lc := ctl.lc
	var req dtos.SysMenuPrefSaveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultReqParamsError, err), c.Writer, lc)
		return
	}
	if err := ctl.getUserApp().SysMenuPrefSave(c, req); err != nil {
		httphelper.RenderFail(c, err, c.Writer, lc)
		return
	}
	httphelper.ResultSuccess(nil, c.Writer, lc)
}

// @Tags    系统管理-菜单
// @Summary 平台重置某租户的菜单定制（清空自建菜单与覆盖记录）
// @Produce json
// @Success 200 {object} httphelper.CommonResponse
// @Router  /api/v1/menu/tenant-reset [post]
func (ctl *controller) SysMenuTenantReset(c *gin.Context) {
	lc := ctl.lc
	var req dtos.SysMenuPrefResetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultReqParamsError, err), c.Writer, lc)
		return
	}
	if err := ctl.getUserApp().SysMenuTenantReset(c, req.TenantId); err != nil {
		httphelper.RenderFail(c, err, c.Writer, lc)
		return
	}
	httphelper.ResultSuccess(nil, c.Writer, lc)
}

/******************************* 部门管理 *******************************/

// @Tags    系统管理-部门
// @Summary 部门列表
// @Produce json
// @Success 200 {object} httphelper.CommonResponse
// @Router  /api/v1/dept [get]
func (ctl *controller) SysDeptSearch(c *gin.Context) {
	lc := ctl.lc
	var req dtos.SysDeptSearchRequest
	if err := c.ShouldBind(&req); err != nil {
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultReqParamsError, err), c.Writer, lc)
		return
	}
	list, total, err := ctl.getUserApp().SysDeptSearch(c, req)
	if err != nil {
		httphelper.RenderFail(c, err, c.Writer, lc)
		return
	}
	httphelper.ResultSuccess(httphelper.NewPageResult(list, total, req.Page, req.PageSize), c.Writer, lc)
}

// @Tags    系统管理-部门
// @Summary 部门树
// @Produce json
// @Success 200 {object} httphelper.CommonResponse
// @Router  /api/v1/dept-tree [get]
func (ctl *controller) SysDeptTree(c *gin.Context) {
	lc := ctl.lc
	var req dtos.SysDeptSearchRequest
	if err := c.ShouldBind(&req); err != nil {
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultReqParamsError, err), c.Writer, lc)
		return
	}
	list, err := ctl.getUserApp().SysDeptTree(c, req)
	if err != nil {
		httphelper.RenderFail(c, err, c.Writer, lc)
		return
	}
	httphelper.ResultSuccess(list, c.Writer, lc)
}

// @Tags    系统管理-部门
// @Summary 部门详情
// @Produce json
// @Success 200 {object} httphelper.CommonResponse
// @Router  /api/v1/dept/{id} [get]
func (ctl *controller) SysDeptById(c *gin.Context) {
	lc := ctl.lc
	res, err := ctl.getUserApp().SysDeptById(c, c.Param("id"))
	if err != nil {
		httphelper.RenderFail(c, err, c.Writer, lc)
		return
	}
	httphelper.ResultSuccess(res, c.Writer, lc)
}

// @Tags    系统管理-部门
// @Summary 新增部门
// @Produce json
// @Success 200 {object} httphelper.CommonResponse
// @Router  /api/v1/dept [post]
func (ctl *controller) SysDeptAdd(c *gin.Context) {
	lc := ctl.lc
	var req dtos.SysDeptAddRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultReqParamsError, err), c.Writer, lc)
		return
	}
	if err := ctl.getUserApp().SysDeptAdd(c, req); err != nil {
		httphelper.RenderFail(c, err, c.Writer, lc)
		return
	}
	httphelper.ResultSuccess(nil, c.Writer, lc)
}

// @Tags    系统管理-部门
// @Summary 编辑部门
// @Produce json
// @Success 200 {object} httphelper.CommonResponse
// @Router  /api/v1/dept [put]
func (ctl *controller) SysDeptUpdate(c *gin.Context) {
	lc := ctl.lc
	var req dtos.SysDeptUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultReqParamsError, err), c.Writer, lc)
		return
	}
	if err := ctl.getUserApp().SysDeptUpdate(c, req); err != nil {
		httphelper.RenderFail(c, err, c.Writer, lc)
		return
	}
	httphelper.ResultSuccess(nil, c.Writer, lc)
}

// @Tags    系统管理-部门
// @Summary 删除部门
// @Produce json
// @Success 200 {object} httphelper.CommonResponse
// @Router  /api/v1/dept [delete]
func (ctl *controller) SysDeptDelete(c *gin.Context) {
	lc := ctl.lc
	var req dtos.SysDeptIdsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultReqParamsError, err), c.Writer, lc)
		return
	}
	if err := ctl.getUserApp().SysDeptDelete(c, req.Ids); err != nil {
		httphelper.RenderFail(c, err, c.Writer, lc)
		return
	}
	httphelper.ResultSuccess(nil, c.Writer, lc)
}

/******************************* 审查日志（登录 + 操作） *******************************/

// @Tags    系统管理-审查日志
// @Summary 审查日志列表
// @Produce json
// @Success 200 {object} httphelper.CommonResponse
// @Router  /api/v1/sys-audit-log [get]
func (ctl *controller) SysAuditLogSearch(c *gin.Context) {
	ctl.auditLogSearch(c, "")
}

// @Tags    系统管理-审查日志
// @Summary 删除审查日志
// @Produce json
// @Success 200 {object} httphelper.CommonResponse
// @Router  /api/v1/sys-audit-log [delete]
func (ctl *controller) SysAuditLogDelete(c *gin.Context) {
	ctl.auditLogDelete(c)
}

// @Tags    系统管理-审查日志
// @Summary 清空审查日志
// @Produce json
// @Success 200 {object} httphelper.CommonResponse
// @Router  /api/v1/sys-audit-log/clear [delete]
func (ctl *controller) SysAuditLogClear(c *gin.Context) {
	ctl.auditLogClear(c)
}

/******* 兼容别名：旧 /sys-login-log 路由，行为等价 type=login（下版本删除） *******/

func (ctl *controller) SysLoginLogSearch(c *gin.Context) {
	ctl.auditLogSearch(c, models.AuditTypeLogin)
}

func (ctl *controller) SysLoginLogDelete(c *gin.Context) {
	ctl.auditLogDelete(c)
}

func (ctl *controller) SysLoginLogClear(c *gin.Context) {
	ctl.auditLogClear(c)
}

// auditLogSearch forceType 非空时强制按该类型过滤（旧接口只查登录记录）
func (ctl *controller) auditLogSearch(c *gin.Context, forceType string) {
	lc := ctl.lc
	var req dtos.SysAuditLogSearchRequest
	if err := c.ShouldBind(&req); err != nil {
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultReqParamsError, err), c.Writer, lc)
		return
	}
	if forceType != "" {
		req.Type = forceType
	}
	list, total, err := ctl.getUserApp().SysAuditLogSearch(c, req)
	if err != nil {
		httphelper.RenderFail(c, err, c.Writer, lc)
		return
	}
	httphelper.ResultSuccess(httphelper.NewPageResult(list, total, req.Page, req.PageSize), c.Writer, lc)
}

func (ctl *controller) auditLogDelete(c *gin.Context) {
	lc := ctl.lc
	var req dtos.SysAuditLogIdsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultReqParamsError, err), c.Writer, lc)
		return
	}
	if err := ctl.getUserApp().SysAuditLogDelete(c, req.Ids); err != nil {
		httphelper.RenderFail(c, err, c.Writer, lc)
		return
	}
	httphelper.ResultSuccess(nil, c.Writer, lc)
}

func (ctl *controller) auditLogClear(c *gin.Context) {
	lc := ctl.lc
	if err := ctl.getUserApp().SysAuditLogClear(c); err != nil {
		httphelper.RenderFail(c, err, c.Writer, lc)
		return
	}
	httphelper.ResultSuccess(nil, c.Writer, lc)
}

/******************************* 系统配置 *******************************/

// @Tags    系统管理-系统配置
// @Summary 配置列表
// @Produce json
// @Success 200 {object} httphelper.CommonResponse
// @Router  /api/v1/system-setup [get]
func (ctl *controller) SysConfigSearch(c *gin.Context) {
	lc := ctl.lc
	var req dtos.SysConfigSearchRequest
	if err := c.ShouldBind(&req); err != nil {
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultReqParamsError, err), c.Writer, lc)
		return
	}
	list, total, err := ctl.getUserApp().SysConfigSearch(c, req)
	if err != nil {
		httphelper.RenderFail(c, err, c.Writer, lc)
		return
	}
	httphelper.ResultSuccess(httphelper.NewPageResult(list, total, req.Page, req.PageSize), c.Writer, lc)
}

// @Tags    系统管理-系统配置
// @Summary 配置详情
// @Produce json
// @Success 200 {object} httphelper.CommonResponse
// @Router  /api/v1/system-setup/{id} [get]
func (ctl *controller) SysConfigById(c *gin.Context) {
	lc := ctl.lc
	res, err := ctl.getUserApp().SysConfigById(c, c.Param("id"))
	if err != nil {
		httphelper.RenderFail(c, err, c.Writer, lc)
		return
	}
	httphelper.ResultSuccess(res, c.Writer, lc)
}

// @Tags    系统管理-系统配置
// @Summary 新增配置
// @Produce json
// @Success 200 {object} httphelper.CommonResponse
// @Router  /api/v1/system-setup [post]
func (ctl *controller) SysConfigAdd(c *gin.Context) {
	lc := ctl.lc
	var req dtos.SysConfigAddRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultReqParamsError, err), c.Writer, lc)
		return
	}
	if err := ctl.getUserApp().SysConfigAdd(c, req); err != nil {
		httphelper.RenderFail(c, err, c.Writer, lc)
		return
	}
	httphelper.ResultSuccess(nil, c.Writer, lc)
}

// @Tags    系统管理-系统配置
// @Summary 编辑配置
// @Produce json
// @Success 200 {object} httphelper.CommonResponse
// @Router  /api/v1/system-setup [put]
func (ctl *controller) SysConfigUpdate(c *gin.Context) {
	lc := ctl.lc
	var req dtos.SysConfigUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultReqParamsError, err), c.Writer, lc)
		return
	}
	if err := ctl.getUserApp().SysConfigUpdate(c, req); err != nil {
		httphelper.RenderFail(c, err, c.Writer, lc)
		return
	}
	httphelper.ResultSuccess(nil, c.Writer, lc)
}

// @Tags    系统管理-系统配置
// @Summary 删除配置
// @Produce json
// @Success 200 {object} httphelper.CommonResponse
// @Router  /api/v1/system-setup [delete]
func (ctl *controller) SysConfigDelete(c *gin.Context) {
	lc := ctl.lc
	var req dtos.SysConfigIdsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultReqParamsError, err), c.Writer, lc)
		return
	}
	if err := ctl.getUserApp().SysConfigDelete(c, req.Ids); err != nil {
		httphelper.RenderFail(c, err, c.Writer, lc)
		return
	}
	httphelper.ResultSuccess(nil, c.Writer, lc)
}

// @Tags    系统管理-系统配置
// @Summary 批量保存配置
// @Produce json
// @Success 200 {object} httphelper.CommonResponse
// @Router  /api/v1/system-setup/save [post]
func (ctl *controller) SysSetupSave(c *gin.Context) {
	lc := ctl.lc
	var req dtos.SysSetupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultReqParamsError, err), c.Writer, lc)
		return
	}
	if err := ctl.getUserApp().SysSetupSave(c, req); err != nil {
		httphelper.RenderFail(c, err, c.Writer, lc)
		return
	}
	httphelper.ResultSuccess(nil, c.Writer, lc)
}

/******************************* 工具 *******************************/

func currentUsername(c *gin.Context) (string, bool) {
	value, ok := c.Get(constants.JwtParsedInfo)
	if !ok {
		return "", false
	}
	claims, ok := value.(*middleware.CustomClaims)
	if !ok || claims == nil || claims.Username == "" {
		return "", false
	}
	return claims.Username, true
}
