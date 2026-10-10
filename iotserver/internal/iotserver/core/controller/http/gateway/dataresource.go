package gateway

import (
	"github.com/gin-gonic/gin"

	"iotserver/internal/dtos"
	"iotserver/internal/pkg/errort"
	"iotserver/internal/pkg/httphelper"
)

// @Tags    高级能力-资源管理
// @Summary 资源类型列表（页面页签数据源）
// @Produce json
// @Success 200 {array} string
// @Router  /api/v1/typeresource [get]
func (ctl *controller) TypeResource(c *gin.Context) {
	types := ctl.getResourceApp().TypeResource(c)
	httphelper.ResultSuccess(types, c.Writer, ctl.lc)
}

// @Tags    高级能力-资源管理
// @Summary 资源实例列表
// @Produce json
// @Param   request query dtos.DataResourceSearchRequest true "参数"
// @Success 200 {object} httphelper.CommonResponse
// @Router  /api/v1/dataresource [get]
func (ctl *controller) DataResourceSearch(c *gin.Context) {
	var req dtos.DataResourceSearchRequest
	urlDecodeParam(&req, c.Request, ctl.lc)
	dtos.CorrectionPageParam(&req.BaseSearchConditionQuery)

	list, total, err := ctl.getResourceApp().DataResourceSearch(c, req)
	if err != nil {
		httphelper.RenderFail(c, err, c.Writer, ctl.lc)
		return
	}
	pageResult := httphelper.NewPageResult(list, total, req.Page, req.PageSize)
	httphelper.ResultSuccess(pageResult, c.Writer, ctl.lc)
}

// @Tags    高级能力-资源管理
// @Summary 新增资源实例
// @Produce json
// @Success 200 {object} httphelper.CommonResponse
// @Router  /api/v1/dataresource [post]
func (ctl *controller) DataResourceAdd(c *gin.Context) {
	var req dtos.DataResourceAddRequest
	if err := c.ShouldBind(&req); err != nil {
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultReqParamsError, err), c.Writer, ctl.lc)
		return
	}
	if err := ctl.getResourceApp().DataResourceAdd(c, req); err != nil {
		httphelper.RenderFail(c, err, c.Writer, ctl.lc)
		return
	}
	httphelper.ResultSuccess(nil, c.Writer, ctl.lc)
}

// @Tags    高级能力-资源管理
// @Summary 修改资源实例
// @Produce json
// @Success 200 {object} httphelper.CommonResponse
// @Router  /api/v1/dataresource [put]
func (ctl *controller) DataResourceUpdate(c *gin.Context) {
	var req dtos.DataResourceUpdateRequest
	if err := c.ShouldBind(&req); err != nil {
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultReqParamsError, err), c.Writer, ctl.lc)
		return
	}
	if err := ctl.getResourceApp().DataResourceUpdate(c, req); err != nil {
		httphelper.RenderFail(c, err, c.Writer, ctl.lc)
		return
	}
	httphelper.ResultSuccess(nil, c.Writer, ctl.lc)
}

// @Tags    高级能力-资源管理
// @Summary 删除资源实例
// @Produce json
// @Success 200 {object} httphelper.CommonResponse
// @Router  /api/v1/dataresource/{resourceId} [delete]
func (ctl *controller) DataResourceDelete(c *gin.Context) {
	id := c.Param(UrlDataResourceId)
	if err := ctl.getResourceApp().DataResourceDelete(c, id); err != nil {
		httphelper.RenderFail(c, err, c.Writer, ctl.lc)
		return
	}
	httphelper.ResultSuccess(nil, c.Writer, ctl.lc)
}

// @Tags    高级能力-资源管理
// @Summary 验证资源实例连通性
// @Produce json
// @Success 200 {object} dtos.DataResourceVerifyResponse
// @Router  /api/v1/dataresource/{resourceId}/health [post]
func (ctl *controller) DataResourceVerify(c *gin.Context) {
	id := c.Param(UrlDataResourceId)
	resp, err := ctl.getResourceApp().DataResourceVerify(c, id)
	if err != nil {
		httphelper.RenderFail(c, err, c.Writer, ctl.lc)
		return
	}
	httphelper.ResultSuccess(resp, c.Writer, ctl.lc)
}
