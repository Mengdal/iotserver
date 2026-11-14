package gateway

import (
	"fmt"

	"github.com/gin-gonic/gin"

	"iotserver/internal/dtos"
	"iotserver/internal/pkg/errort"
	"iotserver/internal/pkg/httphelper"
)

const maxSODriverUploadSize = 100 << 20 // 100MB

// @Tags    驱动管理
// @Summary 上传并安装 iotpGW .so 驱动
// @Accept  multipart/form-data
// @Produce json
// @Param   file formData file true "驱动文件（.so）"
// @Success 200 {object} httphelper.CommonResponse
// @Router  /api/v1/drivers/so [post]
func (ctl *controller) UploadSODriver(c *gin.Context) {
	header, err := c.FormFile("file")
	if err != nil {
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultReqParamsError, err), c.Writer, ctl.lc)
		return
	}
	if header.Size > maxSODriverUploadSize {
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultReqParamsError,
			fmt.Errorf("驱动文件不能超过 %d MB", maxSODriverUploadSize>>20)), c.Writer, ctl.lc)
		return
	}
	file, err := header.Open()
	if err != nil {
		httphelper.RenderFail(c, err, c.Writer, ctl.lc)
		return
	}
	defer file.Close()

	req := dtos.SODriverUploadRequest{
		Name:        c.PostForm("name"),
		Version:     c.PostForm("version"),
		Protocol:    c.PostForm("protocol"),
		Arch:        c.PostForm("arch"),
		ABI:         c.PostForm("abi"),
		Description: c.PostForm("description"),
		FileName:    header.Filename,
		SHA256:      c.PostForm("sha256"),
	}
	result, err := ctl.getDriverLibApp().UploadSODriver(c, req, file)
	if err != nil {
		httphelper.RenderFail(c, err, c.Writer, ctl.lc)
		return
	}
	httphelper.ResultSuccess(result, c.Writer, ctl.lc)
}

// @Tags    驱动管理
// @Summary iotpGW .so 驱动列表
// @Produce json
// @Success 200 {object} httphelper.CommonResponse
// @Router  /api/v1/drivers/so [get]
func (ctl *controller) ListSODrivers(c *gin.Context) {
	result, err := ctl.getDriverLibApp().ListSODrivers(c)
	if err != nil {
		httphelper.RenderFail(c, err, c.Writer, ctl.lc)
		return
	}
	httphelper.ResultSuccess(result, c.Writer, ctl.lc)
}

// @Tags    驱动管理
// @Summary 卸载 iotpGW .so 驱动
// @Produce json
// @Param   id path string true "驱动 ID"
// @Success 200 {object} httphelper.CommonResponse
// @Router  /api/v1/drivers/so/{id} [delete]
func (ctl *controller) DeleteSODriver(c *gin.Context) {
	if err := ctl.getDriverLibApp().DeleteSODriver(c, c.Param("id")); err != nil {
		httphelper.RenderFail(c, err, c.Writer, ctl.lc)
		return
	}
	httphelper.ResultSuccess(nil, c.Writer, ctl.lc)
}
