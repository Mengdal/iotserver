package gateway

import (
	"strings"

	"github.com/gin-gonic/gin"

	"iotserver/internal/dtos"
	"iotserver/internal/iotserver/core/container"
	"iotserver/internal/models"
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
// @Summary 在 things 中重命名网关
// @Produce json
// @Router  /api/v1/gateway/things-rename [post]
func (ctl *controller) GatewayRenameThings(c *gin.Context) {
	var req dtos.GatewayThingsRenameRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultReqParamsError, err), c.Writer, ctl.lc)
		return
	}
	if err := ctl.getGatewayApp().GatewayRenameThings(c, req); err != nil {
		httphelper.RenderFail(c, err, c.Writer, ctl.lc)
		return
	}
	httphelper.ResultSuccess(nil, c.Writer, ctl.lc)
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

/******************************* 参考系统（/edge）网关控制 *******************************/

type gatewayEdgeRestartRequest struct {
	Id string `json:"id" binding:"required"`
}

type gatewayEdgeControlRequest struct {
	Id         string `json:"id" binding:"required"`
	Dn         string `json:"dn" binding:"required"`
	Identifier string `json:"identifier" binding:"required"`
	Value      string `json:"value"`
}

// isEdgeGateway 判断是否为参考系统网关。历史数据里 protocol 可能仍是默认值，
// 因此同时按厂商/型号兜底识别。
func isEdgeGateway(g dtos.GatewayResponse) bool {
	if strings.EqualFold(g.Protocol, models.GatewayProtocolEdge) {
		return true
	}
	s := strings.ToLower(g.Vendor + " " + g.Model)
	return strings.Contains(s, "things") || strings.Contains(s, "iotddc")
}

func edgePk(g dtos.GatewayResponse) string {
	if strings.TrimSpace(g.Pk) != "" {
		return g.Pk
	}
	return g.Sn
}

func (ctl *controller) loadEdgeGateway(c *gin.Context, id string) (dtos.GatewayResponse, bool) {
	g, err := ctl.getGatewayApp().GatewayById(c, id)
	if err != nil {
		httphelper.RenderFail(c, err, c.Writer, ctl.lc)
		return g, false
	}
	if !isEdgeGateway(g) {
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultReqParamsError,
			errort.NewMessageErr("该网关不是参考系统 /edge 网关，不支持设备控制", nil)), c.Writer, ctl.lc)
		return g, false
	}
	return g, true
}

// @Tags    设备接入-网关
// @Summary 重启参考系统网关
// @Produce json
// @Router  /api/v1/gateway/edge/restart [post]
func (ctl *controller) GatewayEdgeRestart(c *gin.Context) {
	var req gatewayEdgeRestartRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultReqParamsError, err), c.Writer, ctl.lc)
		return
	}
	g, ok := ctl.loadEdgeGateway(c, req.Id)
	if !ok {
		return
	}
	if err := container.EdgeCtlFrom(ctl.dic.Get).Restart(c.Request.Context(), edgePk(g), g.Sn); err != nil {
		httphelper.RenderFail(c, err, c.Writer, ctl.lc)
		return
	}
	httphelper.ResultSuccess(nil, c.Writer, ctl.lc)
}

// @Tags    设备接入-网关
// @Summary 写参考系统网关下设备属性
// @Produce json
// @Router  /api/v1/gateway/edge/control [post]
func (ctl *controller) GatewayEdgeControl(c *gin.Context) {
	var req gatewayEdgeControlRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultReqParamsError, err), c.Writer, ctl.lc)
		return
	}
	g, ok := ctl.loadEdgeGateway(c, req.Id)
	if !ok {
		return
	}
	status, msg, err := container.EdgeCtlFrom(ctl.dic.Get).
		SetProperty(c.Request.Context(), edgePk(g), g.Sn, req.Dn, req.Identifier, req.Value)
	if err != nil {
		httphelper.RenderFail(c, err, c.Writer, ctl.lc)
		return
	}
	httphelper.ResultSuccess(map[string]interface{}{"status": status, "message": msg}, c.Writer, ctl.lc)
}

// @Tags    设备接入-网关
// @Summary 拉取参考系统网关设备/测点树
// @Produce json
// @Router  /api/v1/gateway/edge/devices [get]
func (ctl *controller) GatewayEdgeDevices(c *gin.Context) {
	id := c.Query("id")
	if strings.TrimSpace(id) == "" {
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultReqParamsError,
			errort.NewMessageErr("缺少网关 ID", nil)), c.Writer, ctl.lc)
		return
	}
	g, ok := ctl.loadEdgeGateway(c, id)
	if !ok {
		return
	}
	tree, err := container.EdgeCtlFrom(ctl.dic.Get).TagTree(c.Request.Context(), edgePk(g), g.Sn)
	if err != nil {
		httphelper.RenderFail(c, err, c.Writer, ctl.lc)
		return
	}
	httphelper.ResultSuccess(tree, c.Writer, ctl.lc)
}
