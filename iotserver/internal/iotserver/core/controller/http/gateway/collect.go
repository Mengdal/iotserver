package gateway

import (
	"github.com/gin-gonic/gin"

	"iotserver/internal/dtos"
	"iotserver/internal/pkg/errort"
	"iotserver/internal/pkg/httphelper"
)

/******************************* 设备级采集配置 *******************************/

// @Tags    设备接入-采集配置
// @Summary 读取设备点表（无点位时按物模型生成模板）
// @Produce json
// @Success 200 {object} httphelper.CommonResponse
// @Router  /api/v1/collect/config [get]
func (ctl *controller) CollectConfigGet(c *gin.Context) {
	deviceId := c.Query("deviceId")
	if deviceId == "" {
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultIdEmpty, errort.NewMessageErr("设备 ID 不能为空", nil)), c.Writer, ctl.lc)
		return
	}
	res, err := ctl.getCollectApp().CollectConfigGet(c, deviceId)
	if err != nil {
		httphelper.RenderFail(c, err, c.Writer, ctl.lc)
		return
	}
	httphelper.ResultSuccess(res, c.Writer, ctl.lc)
}

// @Tags    设备接入-采集配置
// @Summary 按设备产品物模型生成点表模板
// @Produce json
// @Success 200 {object} httphelper.CommonResponse
// @Router  /api/v1/collect/config/template [get]
func (ctl *controller) CollectConfigTemplate(c *gin.Context) {
	deviceId := c.Query("deviceId")
	if deviceId == "" {
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultIdEmpty, errort.NewMessageErr("设备 ID 不能为空", nil)), c.Writer, ctl.lc)
		return
	}
	res, err := ctl.getCollectApp().CollectConfigTemplate(c, deviceId)
	if err != nil {
		httphelper.RenderFail(c, err, c.Writer, ctl.lc)
		return
	}
	httphelper.ResultSuccess(res, c.Writer, ctl.lc)
}

// @Tags    设备接入-采集配置
// @Summary 导出设备点表 Excel
// @Produce application/octet-stream
// @Success 200 {file} binary
// @Router  /api/v1/collect/config/export [get]
func (ctl *controller) CollectConfigExport(c *gin.Context) {
	deviceId := c.Query("deviceId")
	if deviceId == "" {
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultIdEmpty, errort.NewMessageErr("设备 ID 不能为空", nil)), c.Writer, ctl.lc)
		return
	}
	file, err := ctl.getCollectApp().CollectConfigExport(c, deviceId)
	if err != nil {
		httphelper.RenderFail(c, err, c.Writer, ctl.lc)
		return
	}
	defer file.Excel.Close()
	data, err := file.Excel.WriteToBuffer()
	if err != nil {
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultReadExcelErrorCode, err), c.Writer, ctl.lc)
		return
	}
	httphelper.ResultExcelData(c, file.FileName, data)
}

// @Tags    设备接入-采集配置
// @Summary 解析导入的点表 Excel（返回点位，需再保存）
// @Accept  multipart/form-data
// @Produce json
// @Success 200 {object} httphelper.CommonResponse
// @Router  /api/v1/collect/config/import [post]
func (ctl *controller) CollectConfigImport(c *gin.Context) {
	deviceId := c.PostForm("deviceId")
	if deviceId == "" {
		deviceId = c.Query("deviceId")
	}
	if deviceId == "" {
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultIdEmpty, errort.NewMessageErr("设备 ID 不能为空", nil)), c.Writer, ctl.lc)
		return
	}
	header, err := c.FormFile("file")
	if err != nil {
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultUploadFileErrorCode, err), c.Writer, ctl.lc)
		return
	}
	reader, err := header.Open()
	if err != nil {
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultUploadFileErrorCode, err), c.Writer, ctl.lc)
		return
	}
	defer reader.Close()
	file, err := dtos.NewImportFile(reader)
	if err != nil {
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultUploadFileErrorCode, err), c.Writer, ctl.lc)
		return
	}
	defer file.Excel.Close()
	res, err := ctl.getCollectApp().CollectConfigImport(c, deviceId, file)
	if err != nil {
		httphelper.RenderFail(c, err, c.Writer, ctl.lc)
		return
	}
	httphelper.ResultSuccess(res, c.Writer, ctl.lc)
}

// @Tags    设备接入-采集配置
// @Summary 读取设备点位实时值
// @Produce json
// @Success 200 {object} httphelper.CommonResponse
// @Router  /api/v1/collect/config/latest [get]
func (ctl *controller) CollectConfigLatest(c *gin.Context) {
	deviceId := c.Query("deviceId")
	if deviceId == "" {
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultIdEmpty, errort.NewMessageErr("设备 ID 不能为空", nil)), c.Writer, ctl.lc)
		return
	}
	res, err := ctl.getCollectApp().CollectConfigLatest(c, deviceId)
	if err != nil {
		httphelper.RenderFail(c, err, c.Writer, ctl.lc)
		return
	}
	httphelper.ResultSuccess(res, c.Writer, ctl.lc)
}

// @Tags    设备接入-采集配置
// @Summary 保存设备点表
// @Accept   json
// @Produce json
// @Success 200 {object} httphelper.CommonResponse
// @Router  /api/v1/collect/config [put]
func (ctl *controller) CollectConfigSave(c *gin.Context) {
	var req dtos.CollectConfigSaveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultReqParamsError, err), c.Writer, ctl.lc)
		return
	}
	if err := ctl.getCollectApp().CollectConfigSave(c, req); err != nil {
		httphelper.RenderFail(c, err, c.Writer, ctl.lc)
		return
	}
	httphelper.ResultSuccess(nil, c.Writer, ctl.lc)
}

// @Tags    设备接入-采集配置
// @Summary 采集通道列表
// @Produce json
// @Success 200 {object} httphelper.CommonResponse
// @Router  /api/v1/collect/channels [get]
func (ctl *controller) CollectChannels(c *gin.Context) {
	res, err := ctl.getCollectApp().CollectChannels(c)
	if err != nil {
		httphelper.RenderFail(c, err, c.Writer, ctl.lc)
		return
	}
	httphelper.ResultSuccess(res, c.Writer, ctl.lc)
}

// @Tags    设备接入-采集配置
// @Summary 新增/修改采集通道
// @Accept   json
// @Produce json
// @Success 200 {object} httphelper.CommonResponse
// @Router  /api/v1/collect/channels [put]
func (ctl *controller) CollectChannelSave(c *gin.Context) {
	var req dtos.CollectChannelSaveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultReqParamsError, err), c.Writer, ctl.lc)
		return
	}
	res, err := ctl.getCollectApp().CollectChannelSave(c, req)
	if err != nil {
		httphelper.RenderFail(c, err, c.Writer, ctl.lc)
		return
	}
	httphelper.ResultSuccess(res, c.Writer, ctl.lc)
}

// @Tags    设备接入-采集配置
// @Summary 删除采集通道
// @Produce json
// @Success 200 {object} httphelper.CommonResponse
// @Router  /api/v1/collect/channels/{id} [delete]
func (ctl *controller) CollectChannelDelete(c *gin.Context) {
	if err := ctl.getCollectApp().CollectChannelDelete(c, c.Param("id")); err != nil {
		httphelper.RenderFail(c, err, c.Writer, ctl.lc)
		return
	}
	httphelper.ResultSuccess(nil, c.Writer, ctl.lc)
}

// @Tags    设备接入-采集配置
// @Summary 所有 IOTP 设备绑定（左侧树）
// @Produce json
// @Success 200 {object} httphelper.CommonResponse
// @Router  /api/v1/collect/devices [get]
func (ctl *controller) CollectDevices(c *gin.Context) {
	res, err := ctl.getCollectApp().CollectDevices(c)
	if err != nil {
		httphelper.RenderFail(c, err, c.Writer, ctl.lc)
		return
	}
	httphelper.ResultSuccess(res, c.Writer, ctl.lc)
}

// @Tags    设备接入-采集配置
// @Summary 通道下的 IOTP 设备绑定列表
// @Produce json
// @Success 200 {object} httphelper.CommonResponse
// @Router  /api/v1/collect/channels/{id}/devices [get]
func (ctl *controller) CollectChannelDevices(c *gin.Context) {
	res, err := ctl.getCollectApp().CollectChannelDevices(c, c.Param("id"))
	if err != nil {
		httphelper.RenderFail(c, err, c.Writer, ctl.lc)
		return
	}
	httphelper.ResultSuccess(res, c.Writer, ctl.lc)
}

// @Tags    设备接入-采集配置
// @Summary 把系统设备绑定到通道
// @Accept   json
// @Produce json
// @Success 200 {object} httphelper.CommonResponse
// @Router  /api/v1/collect/channels/{id}/devices [post]
func (ctl *controller) CollectChannelBindDevices(c *gin.Context) {
	var req dtos.CollectChannelBindRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultReqParamsError, err), c.Writer, ctl.lc)
		return
	}
	if err := ctl.getCollectApp().CollectChannelBindDevices(c, c.Param("id"), req.DeviceIds); err != nil {
		httphelper.RenderFail(c, err, c.Writer, ctl.lc)
		return
	}
	httphelper.ResultSuccess(nil, c.Writer, ctl.lc)
}

// @Tags    设备接入-采集配置
// @Summary 解除系统设备与通道的绑定
// @Produce json
// @Success 200 {object} httphelper.CommonResponse
// @Router  /api/v1/collect/channels/{id}/devices/{deviceId} [delete]
func (ctl *controller) CollectChannelUnbindDevice(c *gin.Context) {
	if err := ctl.getCollectApp().CollectChannelUnbindDevice(c, c.Param("id"), c.Param("deviceId")); err != nil {
		httphelper.RenderFail(c, err, c.Writer, ctl.lc)
		return
	}
	httphelper.ResultSuccess(nil, c.Writer, ctl.lc)
}
