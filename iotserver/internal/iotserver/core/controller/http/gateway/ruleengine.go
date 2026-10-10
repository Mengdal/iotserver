package gateway

import (
	"github.com/gin-gonic/gin"

	"iotserver/internal/dtos"
	"iotserver/internal/pkg/errort"
	"iotserver/internal/pkg/httphelper"
)

// @Tags    高级能力-规则引擎
// @Summary 规则列表
// @Produce json
// @Param   request query dtos.RuleEngineSearchQueryRequest true "参数"
// @Success 200 {object} httphelper.CommonResponse
// @Router  /api/v1/rule-engine [get]
func (ctl *controller) RuleEngineSearch(c *gin.Context) {
	var req dtos.RuleEngineSearchQueryRequest
	urlDecodeParam(&req, c.Request, ctl.lc)
	dtos.CorrectionPageParam(&req.BaseSearchConditionQuery)

	list, total, err := ctl.getRuleEngineApp().RuleEngineSearch(c, req)
	if err != nil {
		httphelper.RenderFail(c, err, c.Writer, ctl.lc)
		return
	}
	httphelper.ResultSuccess(httphelper.NewPageResult(list, total, req.Page, req.PageSize), c.Writer, ctl.lc)
}

// @Tags    高级能力-规则引擎
// @Summary 规则详情
// @Produce json
// @Success 200 {object} dtos.RuleEngineSearchQueryResponse
// @Router  /api/v1/rule-engine/{ruleEngineId} [get]
func (ctl *controller) RuleEngineById(c *gin.Context) {
	detail, err := ctl.getRuleEngineApp().RuleEngineById(c, c.Param(RuleEngineId))
	if err != nil {
		httphelper.RenderFail(c, err, c.Writer, ctl.lc)
		return
	}
	httphelper.ResultSuccess(detail, c.Writer, ctl.lc)
}

// @Tags    高级能力-规则引擎
// @Summary 规则运行状态（eKuiper 指标）
// @Produce json
// @Success 200 {object} httphelper.CommonResponse
// @Router  /api/v1/rule-engine/{ruleEngineId}/status [get]
func (ctl *controller) RuleEngineStatus(c *gin.Context) {
	stats, err := ctl.getRuleEngineApp().RuleEngineStatus(c, c.Param(RuleEngineId))
	if err != nil {
		httphelper.RenderFail(c, err, c.Writer, ctl.lc)
		return
	}
	httphelper.ResultSuccess(stats, c.Writer, ctl.lc)
}

// @Tags    高级能力-规则引擎
// @Summary 添加规则
// @Produce json
// @Success 200 {object} httphelper.CommonResponse
// @Router  /api/v1/rule-engine [post]
func (ctl *controller) RuleEngineAdd(c *gin.Context) {
	var req dtos.RuleEngineAddRequest
	if err := c.ShouldBind(&req); err != nil {
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultReqParamsError, err), c.Writer, ctl.lc)
		return
	}
	if err := ctl.getRuleEngineApp().RuleEngineAdd(c, req); err != nil {
		httphelper.RenderFail(c, err, c.Writer, ctl.lc)
		return
	}
	httphelper.ResultSuccess(nil, c.Writer, ctl.lc)
}

// @Tags    高级能力-规则引擎
// @Summary 编辑规则
// @Produce json
// @Success 200 {object} httphelper.CommonResponse
// @Router  /api/v1/rule-engine [put]
func (ctl *controller) RuleEngineUpdate(c *gin.Context) {
	var req dtos.RuleEngineUpdateRequest
	if err := c.ShouldBind(&req); err != nil {
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultReqParamsError, err), c.Writer, ctl.lc)
		return
	}
	if err := ctl.getRuleEngineApp().RuleEngineUpdate(c, req); err != nil {
		httphelper.RenderFail(c, err, c.Writer, ctl.lc)
		return
	}
	httphelper.ResultSuccess(nil, c.Writer, ctl.lc)
}

// @Tags    高级能力-规则引擎
// @Summary 删除规则
// @Produce json
// @Success 200 {object} httphelper.CommonResponse
// @Router  /api/v1/rule-engine/{ruleEngineId}/delete [delete]
func (ctl *controller) RuleEngineDelete(c *gin.Context) {
	if err := ctl.getRuleEngineApp().RuleEngineDelete(c, c.Param(RuleEngineId)); err != nil {
		httphelper.RenderFail(c, err, c.Writer, ctl.lc)
		return
	}
	httphelper.ResultSuccess(nil, c.Writer, ctl.lc)
}

// @Tags    高级能力-规则引擎
// @Summary 启动规则
// @Produce json
// @Success 200 {object} httphelper.CommonResponse
// @Router  /api/v1/rule-engine/{ruleEngineId}/start [post]
func (ctl *controller) RuleEngineStart(c *gin.Context) {
	if err := ctl.getRuleEngineApp().RuleEngineStart(c, c.Param(RuleEngineId)); err != nil {
		httphelper.RenderFail(c, err, c.Writer, ctl.lc)
		return
	}
	httphelper.ResultSuccess(nil, c.Writer, ctl.lc)
}

// @Tags    高级能力-规则引擎
// @Summary 停止规则
// @Produce json
// @Success 200 {object} httphelper.CommonResponse
// @Router  /api/v1/rule-engine/{ruleEngineId}/stop [post]
func (ctl *controller) RuleEngineStop(c *gin.Context) {
	if err := ctl.getRuleEngineApp().RuleEngineStop(c, c.Param(RuleEngineId)); err != nil {
		httphelper.RenderFail(c, err, c.Writer, ctl.lc)
		return
	}
	httphelper.ResultSuccess(nil, c.Writer, ctl.lc)
}
