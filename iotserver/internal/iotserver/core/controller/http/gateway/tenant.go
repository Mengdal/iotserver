package gateway

import (
	"github.com/gin-gonic/gin"

	"iotserver/internal/dtos"
	"iotserver/internal/pkg/errort"
	"iotserver/internal/pkg/httphelper"
)

/******************************* 租户管理：平台级 *******************************/

// @Tags    平台管理-租户
// @Summary 租户列表
// @Produce json
// @Success 200 {object} httphelper.CommonResponse
// @Router  /api/v1/tenant [get]
func (ctl *controller) TenantSearch(c *gin.Context) {
	lc := ctl.lc
	var req dtos.TenantSearchQueryRequest
	if err := c.ShouldBind(&req); err != nil {
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultReqParamsError, err), c.Writer, lc)
		return
	}
	list, total, err := ctl.getTenantApp().TenantSearch(c, req)
	if err != nil {
		httphelper.RenderFail(c, err, c.Writer, lc)
		return
	}
	httphelper.ResultSuccess(httphelper.NewPageResult(list, total, req.Page, req.PageSize), c.Writer, lc)
}

// @Tags    平台管理-租户
// @Summary 租户详情
// @Produce json
// @Success 200 {object} httphelper.CommonResponse
// @Router  /api/v1/tenant/{id} [get]
func (ctl *controller) TenantById(c *gin.Context) {
	lc := ctl.lc
	res, err := ctl.getTenantApp().TenantById(c, c.Param("id"))
	if err != nil {
		httphelper.RenderFail(c, err, c.Writer, lc)
		return
	}
	httphelper.ResultSuccess(res, c.Writer, lc)
}

// @Tags    平台管理-租户
// @Summary 新增租户
// @Produce json
// @Success 200 {object} httphelper.CommonResponse
// @Router  /api/v1/tenant [post]
func (ctl *controller) TenantAdd(c *gin.Context) {
	lc := ctl.lc
	var req dtos.TenantAddRequest
	if err := c.ShouldBind(&req); err != nil {
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultReqParamsError, err), c.Writer, lc)
		return
	}
	res, err := ctl.getTenantApp().TenantAdd(c, req)
	if err != nil {
		httphelper.RenderFail(c, err, c.Writer, lc)
		return
	}
	httphelper.ResultSuccess(res, c.Writer, lc)
}

// @Tags    平台管理-租户
// @Summary 修改租户
// @Produce json
// @Success 200 {object} httphelper.CommonResponse
// @Router  /api/v1/tenant [put]
func (ctl *controller) TenantUpdate(c *gin.Context) {
	lc := ctl.lc
	var req dtos.TenantUpdateRequest
	if err := c.ShouldBind(&req); err != nil {
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultReqParamsError, err), c.Writer, lc)
		return
	}
	if err := ctl.getTenantApp().TenantUpdate(c, req); err != nil {
		httphelper.RenderFail(c, err, c.Writer, lc)
		return
	}
	httphelper.ResultSuccess(nil, c.Writer, lc)
}

// @Tags    平台管理-租户
// @Summary 删除租户
// @Produce json
// @Success 200 {object} httphelper.CommonResponse
// @Router  /api/v1/tenant/{id} [delete]
func (ctl *controller) TenantDelete(c *gin.Context) {
	lc := ctl.lc
	if err := ctl.getTenantApp().TenantDelete(c, c.Param("id")); err != nil {
		httphelper.RenderFail(c, err, c.Writer, lc)
		return
	}
	httphelper.ResultSuccess(nil, c.Writer, lc)
}

// @Tags    平台管理-租户
// @Summary 绑定/创建 things 项目
// @Produce json
// @Router  /api/v1/tenant/{id}/things-project [post]
func (ctl *controller) TenantBindThingsProject(c *gin.Context) {
	var req dtos.TenantThingsBindRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultReqParamsError, err), c.Writer, ctl.lc)
		return
	}
	if err := ctl.getTenantApp().TenantBindThingsProject(c, c.Param("id"), req); err != nil {
		httphelper.RenderFail(c, err, c.Writer, ctl.lc)
		return
	}
	httphelper.ResultSuccess(nil, c.Writer, ctl.lc)
}

// @Tags    平台管理-租户
// @Summary 修改 things 项目名称
// @Produce json
// @Router  /api/v1/tenant/{id}/things-project [put]
func (ctl *controller) TenantRenameThingsProject(c *gin.Context) {
	var req dtos.TenantThingsRenameRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultReqParamsError, err), c.Writer, ctl.lc)
		return
	}
	if err := ctl.getTenantApp().TenantRenameThingsProject(c, c.Param("id"), req.Name); err != nil {
		httphelper.RenderFail(c, err, c.Writer, ctl.lc)
		return
	}
	httphelper.ResultSuccess(nil, c.Writer, ctl.lc)
}
