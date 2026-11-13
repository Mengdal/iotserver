package gateway

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"iotserver/internal/pkg/httphelper"
)

// @Tags    驱动管理
// @Summary 内置系统组件列表
// @Produce json
// @Success 200 {object} httphelper.CommonResponse
// @Router  /api/v1/system-components [get]
func (ctl *controller) SystemComponents(c *gin.Context) {
	list, err := ctl.getDriverLibApp().SystemComponents(c)
	if err != nil {
		httphelper.RenderFail(c, err, c.Writer, ctl.lc)
		return
	}
	httphelper.ResultSuccess(list, c.Writer, ctl.lc)
}

// @Tags    驱动管理
// @Summary 内置系统组件日志
// @Produce json
// @Param   id   path  string true  "组件 ID"
// @Param   tail query int    false "尾部行数，默认 200，最大 1000"
// @Success 200 {object} httphelper.CommonResponse
// @Router  /api/v1/system-components/{id}/logs [get]
func (ctl *controller) SystemComponentLogs(c *gin.Context) {
	tail, _ := strconv.Atoi(c.DefaultQuery("tail", "200"))
	result, err := ctl.getDriverLibApp().ComponentLogs(c, c.Param("id"), tail)
	if err != nil {
		httphelper.RenderFail(c, err, c.Writer, ctl.lc)
		return
	}
	httphelper.ResultSuccess(result, c.Writer, ctl.lc)
}
