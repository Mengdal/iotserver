package gateway

import (
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
