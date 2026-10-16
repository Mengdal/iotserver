package gateway

import (
	"github.com/gin-gonic/gin"

	"iotserver/internal/dtos"
	"iotserver/internal/pkg/errort"
	"iotserver/internal/pkg/httphelper"
)

/******************************* 网关管理（第三方工业网关接入） *******************************/

// @Tags    设备接入-网关
// @Summary 网关列表
// @Produce json
// @Success 200 {object} httphelper.CommonResponse
// @Router  /api/v1/gateways [get]
func (ctl *controller) GatewaySearch(c *gin.Context) {
	lc := ctl.lc
	var req dtos.GatewaySearchQueryRequest
	if err := c.ShouldBind(&req); err != nil {
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultReqParamsError, err), c.Writer, lc)
		return
	}
	list, total, err := ctl.getGatewayApp().GatewaySearch(c, req)
	if err != nil {
		httphelper.RenderFail(c, err, c.Writer, lc)
		return
	}
	httphelper.ResultSuccess(httphelper.NewPageResult(list, total, req.Page, req.PageSize), c.Writer, lc)
}

// @Tags    设备接入-网关
// @Summary 网关详情
// @Produce json
// @Success 200 {object} httphelper.CommonResponse
// @Router  /api/v1/gateway/{id} [get]
func (ctl *controller) GatewayById(c *gin.Context) {
	lc := ctl.lc
	res, err := ctl.getGatewayApp().GatewayById(c, c.Param("id"))
	if err != nil {
		httphelper.RenderFail(c, err, c.Writer, lc)
		return
	}
	httphelper.ResultSuccess(res, c.Writer, lc)
}

// @Tags    设备接入-网关
// @Summary 新增网关（返回一次性 MQTT 接入信息）
// @Produce json
// @Success 200 {object} httphelper.CommonResponse
// @Router  /api/v1/gateway [post]
func (ctl *controller) GatewayAdd(c *gin.Context) {
	lc := ctl.lc
	var req dtos.GatewayAddRequest
	if err := c.ShouldBind(&req); err != nil {
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultReqParamsError, err), c.Writer, lc)
		return
	}
	res, err := ctl.getGatewayApp().GatewayAdd(c, req)
	if err != nil {
		httphelper.RenderFail(c, err, c.Writer, lc)
		return
	}
	httphelper.ResultSuccess(res, c.Writer, lc)
}

// @Tags    设备接入-网关
// @Summary 修改网关
// @Produce json
// @Success 200 {object} httphelper.CommonResponse
// @Router  /api/v1/gateway [put]
func (ctl *controller) GatewayUpdate(c *gin.Context) {
	lc := ctl.lc
	var req dtos.GatewayUpdateRequest
	if err := c.ShouldBind(&req); err != nil {
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultReqParamsError, err), c.Writer, lc)
		return
	}
	if err := ctl.getGatewayApp().GatewayUpdate(c, req); err != nil {
		httphelper.RenderFail(c, err, c.Writer, lc)
		return
	}
	httphelper.ResultSuccess(nil, c.Writer, lc)
}

// @Tags    设备接入-网关
// @Summary 删除网关
// @Produce json
// @Success 200 {object} httphelper.CommonResponse
// @Router  /api/v1/gateway/{id} [delete]
func (ctl *controller) GatewayDelete(c *gin.Context) {
	lc := ctl.lc
	if err := ctl.getGatewayApp().GatewayDelete(c, c.Param("id")); err != nil {
		httphelper.RenderFail(c, err, c.Writer, lc)
		return
	}
	httphelper.ResultSuccess(nil, c.Writer, lc)
}

// @Tags    设备接入-网关
// @Summary 下发网关配置
// @Produce json
// @Success 200 {object} httphelper.CommonResponse
// @Router  /api/v1/gateway/config [put]
func (ctl *controller) GatewayPushConfig(c *gin.Context) {
	lc := ctl.lc
	var req dtos.GatewayConfigRequest
	if err := c.ShouldBind(&req); err != nil {
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultReqParamsError, err), c.Writer, lc)
		return
	}
	version, err := ctl.getGatewayApp().GatewayPushConfig(c, req)
	if err != nil {
		httphelper.RenderFail(c, err, c.Writer, lc)
		return
	}
	httphelper.ResultSuccess(map[string]interface{}{"configVersion": version}, c.Writer, lc)
}

// @Tags    设备接入-网关
// @Summary 下发网关命令（restart/ota/sync）
// @Produce json
// @Success 200 {object} httphelper.CommonResponse
// @Router  /api/v1/gateway/command [post]
func (ctl *controller) GatewayCommand(c *gin.Context) {
	lc := ctl.lc
	var req dtos.GatewayCommandRequest
	if err := c.ShouldBind(&req); err != nil {
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultReqParamsError, err), c.Writer, lc)
		return
	}
	if err := ctl.getGatewayApp().GatewayCommand(c, req); err != nil {
		httphelper.RenderFail(c, err, c.Writer, lc)
		return
	}
	httphelper.ResultSuccess(nil, c.Writer, lc)
}
