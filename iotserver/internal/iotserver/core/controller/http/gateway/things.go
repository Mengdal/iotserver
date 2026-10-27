package gateway

import (
	"github.com/gin-gonic/gin"

	"iotserver/internal/iotserver/core/container"
	"iotserver/internal/pkg/errort"
	"iotserver/internal/pkg/httphelper"
	"iotserver/internal/pkg/tenant"
)

/******************************* things 平台配置与对账 *******************************/

// @Tags    系统管理-things
// @Summary 测试 things 连接
// @Produce json
// @Router  /api/v1/things/test [post]
func (ctl *controller) ThingsTestConnection(c *gin.Context) {
	res, err := container.ThingsItfFrom(ctl.dic.Get).TestConnection(c)
	if err != nil {
		httphelper.RenderFail(c, err, c.Writer, ctl.lc)
		return
	}
	httphelper.ResultSuccess(res, c.Writer, ctl.lc)
}

// @Tags    系统管理-things
// @Summary things 项目列表
// @Produce json
// @Router  /api/v1/things/projects [get]
func (ctl *controller) ThingsProjects(c *gin.Context) {
	list, err := container.ThingsItfFrom(ctl.dic.Get).ListProjects(c, c.Query("keyword"))
	if err != nil {
		httphelper.RenderFail(c, err, c.Writer, ctl.lc)
		return
	}
	httphelper.ResultSuccess(list, c.Writer, ctl.lc)
}

// @Tags    系统管理-things
// @Summary 全量对账
// @Produce json
// @Router  /api/v1/things/reconcile [post]
func (ctl *controller) ThingsReconcile(c *gin.Context) {
	res, err := container.ThingsItfFrom(ctl.dic.Get).ReconcileAll(c)
	if err != nil {
		httphelper.RenderFail(c, err, c.Writer, ctl.lc)
		return
	}
	httphelper.ResultSuccess(res, c.Writer, ctl.lc)
}

// @Tags    设备接入-网关
// @Summary 对当前租户执行 things 网关对账
// @Produce json
// @Router  /api/v1/gateway/things-reconcile [post]
func (ctl *controller) GatewayThingsReconcile(c *gin.Context) {
	tenantID := tenant.FromContext(c).TenantID
	if tenantID == "" {
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultReqParamsError,
			errort.NewMessageErr("请先选择租户", nil)), c.Writer, ctl.lc)
		return
	}
	res, err := container.ThingsItfFrom(ctl.dic.Get).ReconcileTenant(c, tenantID)
	if err != nil {
		httphelper.RenderFail(c, err, c.Writer, ctl.lc)
		return
	}
	httphelper.ResultSuccess(res, c.Writer, ctl.lc)
}
