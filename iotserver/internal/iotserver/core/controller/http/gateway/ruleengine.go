package gateway

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"iotserver/internal/dtos"
	"iotserver/internal/pkg/errort"
	"iotserver/internal/pkg/httphelper"
	"iotserver/internal/pkg/middleware"
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
	// 立即运行等价于调用启动接口，必须同时具备启动权限
	if req.RunImmediately && !middleware.HasPermission(ctl.dic, c, middleware.PermRuleStart) {
		httphelper.RenderFail(c, errort.NewCommonErr(errort.AppPermissionDenied, fmt.Errorf("permission denied")), c.Writer, ctl.lc)
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
	if req.RunImmediately && !middleware.HasPermission(ctl.dic, c, middleware.PermRuleStart) {
		httphelper.RenderFail(c, errort.NewCommonErr(errort.AppPermissionDenied, fmt.Errorf("permission denied")), c.Writer, ctl.lc)
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

// @Tags    高级能力-规则引擎
// @Summary 批量启动规则
// @Produce json
// @Router  /api/v1/rule-engine/batch-start [post]
func (ctl *controller) RuleEngineBatchStart(c *gin.Context) {
	var req dtos.RuleEngineBatchRequest
	if err := c.ShouldBind(&req); err != nil {
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultReqParamsError, err), c.Writer, ctl.lc)
		return
	}
	httphelper.ResultSuccess(ctl.getRuleEngineApp().RuleEngineBatchStart(c, req.Ids), c.Writer, ctl.lc)
}

// @Tags    高级能力-规则引擎
// @Summary 批量停止规则
// @Produce json
// @Router  /api/v1/rule-engine/batch-stop [post]
func (ctl *controller) RuleEngineBatchStop(c *gin.Context) {
	var req dtos.RuleEngineBatchRequest
	if err := c.ShouldBind(&req); err != nil {
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultReqParamsError, err), c.Writer, ctl.lc)
		return
	}
	httphelper.ResultSuccess(ctl.getRuleEngineApp().RuleEngineBatchStop(c, req.Ids), c.Writer, ctl.lc)
}

// @Tags    高级能力-规则引擎
// @Summary SQL 校验
// @Produce json
// @Router  /api/v1/rule-engine/validate [post]
func (ctl *controller) RuleEngineValidate(c *gin.Context) {
	var req dtos.RuleEngineValidateRequest
	if err := c.ShouldBind(&req); err != nil {
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultReqParamsError, err), c.Writer, ctl.lc)
		return
	}
	resp, err := ctl.getRuleEngineApp().RuleEngineValidate(c, req.Sql)
	if err != nil {
		httphelper.RenderFail(c, err, c.Writer, ctl.lc)
		return
	}
	httphelper.ResultSuccess(resp, c.Writer, ctl.lc)
}

// @Tags    高级能力-规则引擎
// @Summary 创建规则试运行
// @Produce json
// @Router  /api/v1/rule-engine/debug [post]
func (ctl *controller) RuleEngineDebugStart(c *gin.Context) {
	var req dtos.RuleEngineDebugRequest
	if err := c.ShouldBind(&req); err != nil {
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultReqParamsError, err), c.Writer, ctl.lc)
		return
	}
	id, err := ctl.getRuleEngineApp().RuleEngineDebugStart(c, req)
	if err != nil {
		httphelper.RenderFail(c, err, c.Writer, ctl.lc)
		return
	}
	httphelper.ResultSuccess(dtos.RuleEngineDebugResponse{Id: id}, c.Writer, ctl.lc)
}

// @Tags    高级能力-规则引擎
// @Summary 规则试运行输出（SSE）
// @Router  /api/v1/rule-engine/debug/{testId}/stream [get]
func (ctl *controller) RuleEngineDebugStream(c *gin.Context) {
	id := c.Param("testId")
	app := ctl.getRuleEngineApp()
	// 打开/启动失败或客户端断开都要删除试运行规则，避免 eKuiper 里残留 test_xxx
	defer func() {
		_ = app.RuleEngineDebugStop(context.Background(), id)
	}()

	// 必须用真正的请求 context：gin.Context 默认 ContextWithFallback=false，
	// 它的 Done() 返回 nil，客户端断开时上游 eKuiper 请求不会被取消。
	body, err := app.RuleEngineDebugStream(c.Request.Context(), id)
	if err != nil {
		httphelper.RenderFail(c, err, c.Writer, ctl.lc)
		return
	}
	defer body.Close()

	// SSE 是长连接，清掉 http.Server 的全局 WriteTimeout（默认 45s），否则输出会被截断
	if rw, ok := httphelper.RawResponseWriter(c.Request.Context()); ok {
		if dlErr := http.NewResponseController(rw).SetWriteDeadline(time.Time{}); dlErr != nil {
			ctl.lc.Warnf("rule debug stream: clear write deadline failed: %v", dlErr)
		}
	} else {
		ctl.lc.Warnf("rule debug stream: raw response writer not found")
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	c.Status(http.StatusOK)
	// 先把 SSE 响应头发出去：真实数据源可能长时间没有数据，
	// 不主动 flush 的话浏览器/代理拿不到响应头，会误以为请求还没开始
	c.Writer.Flush()
	reader := bufio.NewReader(body)
	// eKuiper 的试运行 SSE 一个事件以空行结束；转发事件后执行调试动作，
	// 这样页面上既能看到规则输出，也能看到动作的真实推送结果
	var dataLines []string
	deliverActions := func() bool {
		if len(dataLines) == 0 {
			return true
		}
		payload := strings.Join(dataLines, "\n")
		dataLines = dataLines[:0]
		results := app.RuleEngineDebugDeliver(c.Request.Context(), id, payload)
		for _, result := range results {
			raw, marshalErr := json.Marshal(result)
			if marshalErr != nil {
				continue
			}
			if _, writeErr := fmt.Fprintf(c.Writer, "event: action\ndata: %s\n\n", raw); writeErr != nil {
				return false
			}
			c.Writer.Flush()
		}
		return true
	}
	for {
		line, readErr := reader.ReadString('\n')
		if len(line) > 0 {
			if _, writeErr := c.Writer.WriteString(line); writeErr != nil {
				return
			}
			c.Writer.Flush()
			trimmed := strings.TrimRight(line, "\r\n")
			switch {
			case trimmed == "":
				if !deliverActions() {
					return
				}
			case strings.HasPrefix(trimmed, "data:"):
				value := strings.TrimPrefix(trimmed, "data:")
				// SSE 规范：冒号后最多去掉一个空格
				value = strings.TrimPrefix(value, " ")
				dataLines = append(dataLines, value)
			}
		}
		if readErr != nil {
			deliverActions()
			return
		}
	}
}

// @Tags    高级能力-规则引擎
// @Summary 停止规则试运行
// @Router  /api/v1/rule-engine/debug/{testId} [delete]
func (ctl *controller) RuleEngineDebugStop(c *gin.Context) {
	if err := ctl.getRuleEngineApp().RuleEngineDebugStop(context.Background(), c.Param("testId")); err != nil {
		httphelper.RenderFail(c, err, c.Writer, ctl.lc)
		return
	}
	httphelper.ResultSuccess(nil, c.Writer, ctl.lc)
}

// @Tags    高级能力-规则引擎
// @Summary 导出规则
// @Router  /api/v1/rule-engine/export [get]
func (ctl *controller) RuleEngineExport(c *gin.Context) {
	ids := dtos.ApiParamsStringToArray(c.Query("ids"))
	file, err := ctl.getRuleEngineApp().RuleEngineExport(c, ids)
	if err != nil {
		httphelper.RenderFail(c, err, c.Writer, ctl.lc)
		return
	}
	data, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultSystemError, err), c.Writer, ctl.lc)
		return
	}
	c.Header("Content-Disposition", `attachment; filename="iotserver-rules.json"`)
	c.Data(http.StatusOK, "application/json", data)
}

// @Tags    高级能力-规则引擎
// @Summary 导入规则
// @Produce json
// @Router  /api/v1/rule-engine/import [post]
func (ctl *controller) RuleEngineImport(c *gin.Context) {
	var req dtos.RuleEngineImportRequest
	if err := c.ShouldBind(&req); err != nil {
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultReqParamsError, err), c.Writer, ctl.lc)
		return
	}
	httphelper.ResultSuccess(ctl.getRuleEngineApp().RuleEngineImport(c, req), c.Writer, ctl.lc)
}
