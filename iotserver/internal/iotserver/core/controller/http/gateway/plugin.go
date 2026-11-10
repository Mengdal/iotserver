package gateway

import (
	"fmt"
	"strconv"

	"github.com/gin-gonic/gin"

	"iotserver/internal/dtos"
	"iotserver/internal/pkg/errort"
	"iotserver/internal/pkg/httphelper"
)

const maxPluginPackageSize = 200 << 20 // 200MB

// @Tags    驱动管理-原生插件
// @Summary 上传并安装插件包
// @Accept  multipart/form-data
// @Produce json
// @Param   file formData file true "插件包（tar.gz）"
// @Success 200  {object} httphelper.CommonResponse
// @Router  /api/v1/plugins/packages/upload [post]
func (ctl *controller) PluginPackageUpload(c *gin.Context) {
	header, err := c.FormFile("file")
	if err != nil {
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultReqParamsError, err), c.Writer, ctl.lc)
		return
	}
	if header.Size > maxPluginPackageSize {
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultReqParamsError,
			fmt.Errorf("插件包不能超过 %d MB", maxPluginPackageSize>>20)), c.Writer, ctl.lc)
		return
	}
	file, err := header.Open()
	if err != nil {
		httphelper.RenderFail(c, err, c.Writer, ctl.lc)
		return
	}
	defer file.Close()
	result, err := ctl.getPluginApp().UploadPackage(c, header.Filename, file)
	if err != nil {
		httphelper.RenderFail(c, err, c.Writer, ctl.lc)
		return
	}
	httphelper.ResultSuccess(result, c.Writer, ctl.lc)
}

// @Tags    驱动管理-原生插件
// @Summary 原生插件包列表
// @Produce json
// @Success 200 {object} httphelper.CommonResponse
// @Router  /api/v1/plugins/packages [get]
func (ctl *controller) PluginPackages(c *gin.Context) {
	list, err := ctl.getPluginApp().ListPackages(c)
	if err != nil {
		httphelper.RenderFail(c, err, c.Writer, ctl.lc)
		return
	}
	httphelper.ResultSuccess(list, c.Writer, ctl.lc)
}

// @Tags    驱动管理-原生插件
// @Summary 删除插件包
// @Produce json
// @Param   id path string true "插件包 ID"
// @Success 200 {object} httphelper.CommonResponse
// @Router  /api/v1/plugins/packages/{id} [delete]
func (ctl *controller) PluginPackageDelete(c *gin.Context) {
	if err := ctl.getPluginApp().DeletePackage(c, c.Param("id")); err != nil {
		httphelper.RenderFail(c, err, c.Writer, ctl.lc)
		return
	}
	httphelper.ResultSuccess(nil, c.Writer, ctl.lc)
}

// @Tags    驱动管理-原生插件
// @Summary 原生插件实例列表
// @Produce json
// @Success 200 {object} httphelper.CommonResponse
// @Router  /api/v1/plugins/instances [get]
func (ctl *controller) PluginInstances(c *gin.Context) {
	list, err := ctl.getPluginApp().ListInstances(c)
	if err != nil {
		httphelper.RenderFail(c, err, c.Writer, ctl.lc)
		return
	}
	httphelper.ResultSuccess(list, c.Writer, ctl.lc)
}

// @Tags    驱动管理-原生插件
// @Summary 创建插件实例
// @Produce json
// @Param   request body dtos.PluginInstanceCreateRequest true "创建参数"
// @Success 200 {object} httphelper.CommonResponse
// @Router  /api/v1/plugins/instances [post]
func (ctl *controller) PluginInstanceCreate(c *gin.Context) {
	var req dtos.PluginInstanceCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultReqParamsError, err), c.Writer, ctl.lc)
		return
	}
	result, err := ctl.getPluginApp().CreateInstance(c, req)
	if err != nil {
		httphelper.RenderFail(c, err, c.Writer, ctl.lc)
		return
	}
	httphelper.ResultSuccess(result, c.Writer, ctl.lc)
}

// @Tags    驱动管理-原生插件
// @Summary 启动/停止/重启插件实例
// @Produce json
// @Param   id path string true "实例 ID"
// @Success 200 {object} httphelper.CommonResponse
// @Router  /api/v1/plugins/instances/{id}/start [post]
func (ctl *controller) PluginInstanceStart(c *gin.Context) {
	if err := ctl.getPluginApp().StartInstance(c, c.Param("id")); err != nil {
		httphelper.RenderFail(c, err, c.Writer, ctl.lc)
		return
	}
	httphelper.ResultSuccess(nil, c.Writer, ctl.lc)
}

func (ctl *controller) PluginInstanceStop(c *gin.Context) {
	if err := ctl.getPluginApp().StopInstance(c, c.Param("id")); err != nil {
		httphelper.RenderFail(c, err, c.Writer, ctl.lc)
		return
	}
	httphelper.ResultSuccess(nil, c.Writer, ctl.lc)
}

func (ctl *controller) PluginInstanceRestart(c *gin.Context) {
	if err := ctl.getPluginApp().RestartInstance(c, c.Param("id")); err != nil {
		httphelper.RenderFail(c, err, c.Writer, ctl.lc)
		return
	}
	httphelper.ResultSuccess(nil, c.Writer, ctl.lc)
}

// @Tags    驱动管理-原生插件
// @Summary 插件实例日志
// @Produce json
// @Param   id   path  string true  "实例 ID"
// @Param   tail query int    false "返回行数"
// @Success 200 {object} httphelper.CommonResponse
// @Router  /api/v1/plugins/instances/{id}/logs [get]
func (ctl *controller) PluginInstanceLogs(c *gin.Context) {
	tail, _ := strconv.Atoi(c.DefaultQuery("tail", "200"))
	result, err := ctl.getPluginApp().InstanceLogs(c, c.Param("id"), tail)
	if err != nil {
		httphelper.RenderFail(c, err, c.Writer, ctl.lc)
		return
	}
	httphelper.ResultSuccess(result, c.Writer, ctl.lc)
}

// @Tags    驱动管理-原生插件
// @Summary 删除插件实例
// @Produce json
// @Param   id path string true "实例 ID"
// @Success 200 {object} httphelper.CommonResponse
// @Router  /api/v1/plugins/instances/{id} [delete]
func (ctl *controller) PluginInstanceDelete(c *gin.Context) {
	if err := ctl.getPluginApp().DeleteInstance(c, c.Param("id")); err != nil {
		httphelper.RenderFail(c, err, c.Writer, ctl.lc)
		return
	}
	httphelper.ResultSuccess(nil, c.Writer, ctl.lc)
}
