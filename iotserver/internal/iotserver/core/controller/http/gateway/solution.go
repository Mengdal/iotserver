package gateway

import (
	"github.com/gin-gonic/gin"

	"iotserver/internal/dtos"
	"iotserver/internal/pkg/errort"
	"iotserver/internal/pkg/httphelper"
)

/******************************* 设备接入 - 方案（行业模板一键应用）*******************************/

// @Tags    设备接入-方案
// @Summary 方案列表（内置 + 本租户自定义）
// @Produce json
// @Param   request query    dtos.SolutionSearchRequest false "参数"
// @Success 200     {array}  dtos.SolutionResponse
// @Router  /api/v1/solutions [get]
func (ctl *controller) SolutionSearch(c *gin.Context) {
	lc := ctl.lc
	var req dtos.SolutionSearchRequest
	if err := c.ShouldBind(&req); err != nil {
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultReqParamsError, err), c.Writer, lc)
		return
	}
	list, err := ctl.getSolutionApp().SolutionSearch(c, req)
	if err != nil {
		httphelper.RenderFail(c, err, c.Writer, lc)
		return
	}
	httphelper.ResultSuccess(list, c.Writer, lc)
}

// @Tags    设备接入-方案
// @Summary 方案详情（含物模型/告警/看板预设）
// @Produce json
// @Success 200 {object} httphelper.CommonResponse
// @Router  /api/v1/solution/{key} [get]
func (ctl *controller) SolutionByKey(c *gin.Context) {
	lc := ctl.lc
	res, err := ctl.getSolutionApp().SolutionByKey(c, c.Param(UrlParamSolutionKey))
	if err != nil {
		httphelper.RenderFail(c, err, c.Writer, lc)
		return
	}
	httphelper.ResultSuccess(res, c.Writer, lc)
}

// @Tags    设备接入-方案
// @Summary 新增方案物模型条目（属性/事件/服务）
// @Accept  json
// @Produce json
// @Param   key     path     string                            true "模板 key"
// @Param   request body     dtos.SolutionThingModelItemRequest true "参数"
// @Success 200     {object} dtos.SolutionResponse
// @Router  /api/v1/solution/{key}/thing-model/item [post]
func (ctl *controller) SolutionThingModelItemAdd(c *gin.Context) {
	lc := ctl.lc
	var req dtos.SolutionThingModelItemRequest
	if err := c.ShouldBind(&req); err != nil {
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultReqParamsError, err), c.Writer, lc)
		return
	}
	res, err := ctl.getSolutionApp().SolutionThingModelItemAdd(c, c.Param(UrlParamSolutionKey), req)
	if err != nil {
		httphelper.RenderFail(c, err, c.Writer, lc)
		return
	}
	httphelper.ResultSuccess(res, c.Writer, lc)
}

// @Tags    设备接入-方案
// @Summary 编辑方案物模型条目（按旧 identifier 定位）
// @Accept  json
// @Produce json
// @Param   key     path     string                             true "模板 key"
// @Param   request body     dtos.SolutionThingModelItemRequest true "参数"
// @Success 200     {object} dtos.SolutionResponse
// @Router  /api/v1/solution/{key}/thing-model/item [put]
func (ctl *controller) SolutionThingModelItemUpdate(c *gin.Context) {
	lc := ctl.lc
	var req dtos.SolutionThingModelItemRequest
	if err := c.ShouldBind(&req); err != nil {
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultReqParamsError, err), c.Writer, lc)
		return
	}
	res, err := ctl.getSolutionApp().SolutionThingModelItemUpdate(c, c.Param(UrlParamSolutionKey), req)
	if err != nil {
		httphelper.RenderFail(c, err, c.Writer, lc)
		return
	}
	httphelper.ResultSuccess(res, c.Writer, lc)
}

// @Tags    设备接入-方案
// @Summary 删除方案物模型条目
// @Produce json
// @Param   key        path  string true "模板 key"
// @Param   type       query string true "类型 property|event|action"
// @Param   identifier query string true "条目标识符"
// @Success 200        {object} dtos.SolutionResponse
// @Router  /api/v1/solution/{key}/thing-model/item [delete]
func (ctl *controller) SolutionThingModelItemDelete(c *gin.Context) {
	lc := ctl.lc
	var req dtos.SolutionThingModelItemDeleteRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultReqParamsError, err), c.Writer, lc)
		return
	}
	res, err := ctl.getSolutionApp().SolutionThingModelItemDelete(c, c.Param(UrlParamSolutionKey), req)
	if err != nil {
		httphelper.RenderFail(c, err, c.Writer, lc)
		return
	}
	httphelper.ResultSuccess(res, c.Writer, lc)
}

// @Tags    设备接入-方案
// @Summary 一键应用方案（生成产品 + 发布 + 告警规则）
// @Produce json
// @Param   key     path     string                            true "模板 key"
// @Param   request body     dtos.SolutionApplyRequest  true "参数"
// @Success 200     {object} dtos.SolutionApplyResponse
// @Router  /api/v1/solution/{key}/apply [post]
func (ctl *controller) SolutionApply(c *gin.Context) {
	lc := ctl.lc
	var req dtos.SolutionApplyRequest
	if err := c.ShouldBind(&req); err != nil {
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultReqParamsError, err), c.Writer, lc)
		return
	}
	res, err := ctl.getSolutionApp().SolutionApply(c, c.Param(UrlParamSolutionKey), req)
	if err != nil {
		httphelper.RenderFail(c, err, c.Writer, lc)
		return
	}
	httphelper.ResultSuccess(res, c.Writer, lc)
}
