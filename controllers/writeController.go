package controllers

import "iotServer/services"

type WriteController struct {
	BaseController
}

// Command @Title 控制下发
// @Description 发送控制命令
// @Param Authorization    header   string  true   "Bearer YourToken"
// @Param deviceCode query string false "设备"
// @Param tagCode query string false "tag点"
// @Param val query string true "值"
// @Success 200 {string} string "控制命令发送成功"
// @Failure 500 {object} map[string]interface{} "控制命令发送失败"
// @router /command [post]
func (c *WriteController) Command(deviceCode, tagCode, val string) {
	userId := c.Ctx.Input.GetData("user_id").(int64)
	// 发布控制命令
	if _, err := services.Processor.Deal(deviceCode, tagCode, val, "手动控制", userId); err != nil {
		c.Error(400, "控制命令发送失败,"+err.Error())
	}
}
