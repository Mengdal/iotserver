package gateway

import (
	"github.com/gin-gonic/gin"

	"iotserver/internal/dtos"
	"iotserver/internal/pkg/errort"
	"iotserver/internal/pkg/httphelper"
)

// @Tags    高级能力-场景联动
// @Summary 场景列表
// @Produce json
// @Success 200 {object} httphelper.CommonResponse
// @Router  /api/v1/scene [get]
func (ctl *controller) SceneSearch(c *gin.Context) {
	var req dtos.SceneSearchQueryRequest
	urlDecodeParam(&req, c.Request, ctl.lc)
	dtos.CorrectionPageParam(&req.BaseSearchConditionQuery)

	list, total, err := ctl.getSceneApp().SceneSearch(c, req)
	if err != nil {
		httphelper.RenderFail(c, err, c.Writer, ctl.lc)
		return
	}
	httphelper.ResultSuccess(httphelper.NewPageResult(list, total, req.Page, req.PageSize), c.Writer, ctl.lc)
}

// @Tags    高级能力-场景联动
// @Summary 新建场景
// @Produce json
// @Success 200 {object} httphelper.CommonResponse
// @Router  /api/v1/scene [post]
func (ctl *controller) SceneAdd(c *gin.Context) {
	var req dtos.SceneAddRequest
	if err := c.ShouldBind(&req); err != nil {
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultReqParamsError, err), c.Writer, ctl.lc)
		return
	}
	if err := ctl.getSceneApp().SceneAdd(c, req); err != nil {
		httphelper.RenderFail(c, err, c.Writer, ctl.lc)
		return
	}
	httphelper.ResultSuccess(nil, c.Writer, ctl.lc)
}

// @Tags    高级能力-场景联动
// @Summary 编辑场景（触发条件与执行动作）
// @Produce json
// @Success 200 {object} httphelper.CommonResponse
// @Router  /api/v1/scene [put]
func (ctl *controller) SceneUpdate(c *gin.Context) {
	var req dtos.SceneUpdateRequest
	if err := c.ShouldBind(&req); err != nil {
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultReqParamsError, err), c.Writer, ctl.lc)
		return
	}
	if err := ctl.getSceneApp().SceneUpdate(c, req); err != nil {
		httphelper.RenderFail(c, err, c.Writer, ctl.lc)
		return
	}
	httphelper.ResultSuccess(nil, c.Writer, ctl.lc)
}

// @Tags    高级能力-场景联动
// @Summary 删除场景
// @Produce json
// @Success 200 {object} httphelper.CommonResponse
// @Router  /api/v1/scene/{sceneId} [delete]
func (ctl *controller) SceneDelete(c *gin.Context) {
	if err := ctl.getSceneApp().SceneDelete(c, c.Param(SceneId)); err != nil {
		httphelper.RenderFail(c, err, c.Writer, ctl.lc)
		return
	}
	httphelper.ResultSuccess(nil, c.Writer, ctl.lc)
}

// @Tags    高级能力-场景联动
// @Summary 启动场景
// @Produce json
// @Success 200 {object} httphelper.CommonResponse
// @Router  /api/v1/scene/{sceneId}/start [post]
func (ctl *controller) SceneStart(c *gin.Context) {
	if err := ctl.getSceneApp().SceneStart(c, c.Param(SceneId)); err != nil {
		httphelper.RenderFail(c, err, c.Writer, ctl.lc)
		return
	}
	httphelper.ResultSuccess(nil, c.Writer, ctl.lc)
}

// @Tags    高级能力-场景联动
// @Summary 停止场景
// @Produce json
// @Success 200 {object} httphelper.CommonResponse
// @Router  /api/v1/scene/{sceneId}/stop [post]
func (ctl *controller) SceneStop(c *gin.Context) {
	if err := ctl.getSceneApp().SceneStop(c, c.Param(SceneId)); err != nil {
		httphelper.RenderFail(c, err, c.Writer, ctl.lc)
		return
	}
	httphelper.ResultSuccess(nil, c.Writer, ctl.lc)
}

// @Tags    高级能力-场景联动
// @Summary 场景联动日志
// @Produce json
// @Success 200 {object} httphelper.CommonResponse
// @Router  /api/v1/scene/{sceneId}/log [get]
func (ctl *controller) SceneLog(c *gin.Context) {
	var req dtos.SceneLogSearchQueryRequest
	urlDecodeParam(&req, c.Request, ctl.lc)
	dtos.CorrectionPageParam(&req.BaseSearchConditionQuery)

	list, total, err := ctl.getSceneApp().SceneLog(c, c.Param(SceneId), req)
	if err != nil {
		httphelper.RenderFail(c, err, c.Writer, ctl.lc)
		return
	}
	httphelper.ResultSuccess(httphelper.NewPageResult(list, total, req.Page, req.PageSize), c.Writer, ctl.lc)
}

// @Tags    高级能力-场景联动
// @Summary eKuiper 场景触发回调（由 eKuiper 规则动作回调，免鉴权）
// @Produce json
// @Success 200 {object} httphelper.CommonResponse
// @Router  /api/v1/ekuiper/scene [post]
func (ctl *controller) EkuiperScene(c *gin.Context) {
	if err := checkCallbackToken(c); err != nil {
		httphelper.RenderFail(c, err, c.Writer, ctl.lc)
		return
	}

	req := make(map[string]interface{})
	if err := c.ShouldBind(&req); err != nil {
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultReqParamsError, err), c.Writer, ctl.lc)
		return
	}
	ctl.lc.Infof("ekuiper scene trigger: %v", req)
	if err := ctl.getSceneApp().ExecuteSceneByRule(c, req); err != nil {
		httphelper.RenderFail(c, err, c.Writer, ctl.lc)
		return
	}
	httphelper.ResultSuccess(nil, c.Writer, ctl.lc)
}
