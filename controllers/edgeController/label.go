package edgeController

import (
	"encoding/json"
	"iotServer/controllers"
	"iotServer/iotp"
	"iotServer/models/dtos"
)

type LabelController struct {
	controllers.BaseController
}

var tagService = iotp.NewTagService()

// ListDevicesByTag @Title 根据标签查询设备
// @Description 获取拥有指定标签值的所有设备列表
// @Param   Authorization  header  string  true  "Bearer YourToken"
// @Param   body    body    dtos.TagQueryRequest  true  "标签查询参数"
// @Success 200 {object} controllers.SimpleResult
// @Failure 400 "错误信息"
// @router /listDevicesByTag [post]
func (c *LabelController) ListDevicesByTag() {
	var req dtos.TagQueryRequest
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &req); err != nil {
		c.Error(400, "参数解析失败: "+err.Error())
	}

	devices, err := tagService.ListDevicesByTag(req.TagName, req.TagValue, nil)
	if err != nil {
		c.Error(400, "查询失败: "+err.Error())
	}

	c.Success(devices)
}

// ListTagsByDevice @Title 查询设备标签
// @Description 获取指定设备的所有标签
// @Param   Authorization  header  string  true  "Bearer YourToken"
// @Param   body    body    dtos.DeviceQueryRequest  true  "设备查询参数"
// @Success 200 {object} controllers.SimpleResult
// @Failure 400 "错误信息"
// @router /listTagsByDevice [post]
func (c *LabelController) ListTagsByDevice() {
	var req dtos.DeviceQueryRequest
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &req); err != nil {
		c.Error(400, "参数解析失败: "+err.Error())
	}

	tags, err := tagService.ListTagsByDevice(req.DeviceName)
	if err != nil {
		c.Error(400, "查询失败: "+err.Error())
	}

	c.Success(tags)
}

// GetTagValue @Title 获取标签值
// @Description 查询设备特定标签的当前值
// @Param   Authorization  header  string  true  "Bearer YourToken"
// @Param   body    body    dtos.TagValueRequest  true  "标签值查询参数"
// @Success 200 {object} controllers.SimpleResult
// @Failure 400 "错误信息"
// @router /getTagValue [post]
func (c *LabelController) GetTagValue() {
	var req dtos.TagValueRequest
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &req); err != nil {
		c.Error(400, "参数解析失败: "+err.Error())
	}

	value, err := tagService.GetTagValue(req.DeviceName, req.TagName)
	if err != nil {
		c.Error(400, "查询失败: "+err.Error())
	}

	c.Success(value)
}

// AddTag @Title 添加标签
// @Description 为设备添加新标签
// @Param   Authorization  header  string  true  "Bearer YourToken"
// @Param   body    body    dtos.TagAddRequest  true  "标签添加参数"
// @Success 200 {object} controllers.SimpleResult
// @Failure 400 "错误信息"
// @router /addTag [post]
func (c *LabelController) AddTag() {
	var req dtos.TagAddRequest
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &req); err != nil {
		c.Error(400, "参数解析失败: "+err.Error())
	}

	if err := tagService.AddTag(req.DeviceName, req.TagName, req.TagValue); err != nil {
		c.Error(400, "添加失败: "+err.Error())
	}

	c.Success(nil)
}

// RemoveTag @Title 删除标签
// @Description 移除设备的指定标签
// @Param   Authorization  header  string  true  "Bearer YourToken"
// @Param   body    body    dtos.TagRemoveRequest  true  "标签删除参数"
// @Success 200 {object} controllers.SimpleResult
// @Failure 400 "错误信息"
// @router /removeTag [post]
func (c *LabelController) RemoveTag() {
	var req dtos.TagRemoveRequest
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &req); err != nil {
		c.Error(400, "参数解析失败: "+err.Error())
	}

	if err := tagService.RemoveTag(req.DeviceName, req.TagName); err != nil {
		c.Error(400, "删除失败: "+err.Error())
	}

	c.Success(nil)
}
