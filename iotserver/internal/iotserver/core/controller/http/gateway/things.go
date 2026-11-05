package gateway

import (
	"github.com/gin-gonic/gin"

	"iotserver/internal/iotserver/core/container"
	"iotserver/internal/pkg/httphelper"
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
