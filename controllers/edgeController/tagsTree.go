package edgeController

import (
	"iotServer/controllers"
	"iotServer/iotp"
)

type TagController struct {
	controllers.BaseController
}

// GetTagsTree @Title 获取设备树
// @Description 根据传入设备获取点树
// @Success 200 {object} controllers.SimpleResult "返回结果树"
// @Failure 400 "错误信息"
// @router /getTagsTree [post]
func (c *TagController) GetTagsTree() {
	data, err := iotp.GetTagsTree()
	if err != nil {
		c.Error(400, err.Error())
	}
	c.Success(data)
}
