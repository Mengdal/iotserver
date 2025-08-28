package controllers

import (
	"encoding/json"
	"github.com/beego/beego/v2/client/orm"
	"iotServer/iotp"
	"iotServer/models"
	"iotServer/models/dtos"
	"iotServer/services"
)

// DeviceController 设备管理控制器
type DeviceController struct {
	BaseController
}

var tagService = iotp.TagService{}

// GetAllDevices @Title 获取所有设备
// @Description 获取系统中所有绑定设备的信息
// @Param   Authorization  header  string  true  "Bearer YourToken"
// @Param   page           query   int     false "当前页码，默认1"
// @Param   size           query   int     false "每页数量，默认10"
// @Success 200 {object} controllers.SimpleResult
// @Failure 400 "请求错误"
// @router /all [post]
func (c *DeviceController) GetAllDevices() {

	page, _ := c.GetInt("page", 1)
	size, _ := c.GetInt("size", 10)

	devices, err := tagService.GetAllDevices(page, size)
	if err != nil {
		c.Error(400, "获取设备列表失败: "+err.Error())
	}

	c.Success(devices)
}

// GetDevicesTree @Title 获取设备树
// @Description 根据传入产品获取点树
// @Param   Authorization  header  string  true  "Bearer YourToken"
// @Param   productId      query   string  true  "产品ID"
// @Success 200 {object} controllers.SimpleResult "返回结果树"
// @Failure 400 "错误信息"
// @router /getTagsTree [post]
func (c *DeviceController) GetDevicesTree() {
	productId := c.GetString("productId")
	data, err := tagService.DevicesTagsTree("productId", productId)
	if err != nil {
		c.Error(400, err.Error())
	}
	c.Success(data)
}

// GetNoBindDevices @Title 查询未绑定设备
// @Description 添加设备时查询未绑定产品的设备
// @Param   Authorization  header  string  true  "Bearer YourToken"
// @Success 200 {object} controllers.SimpleResult
// @Failure 400 "请求错误"
// @router /devices [post]
func (c *DeviceController) GetNoBindDevices() {

	devices, err := tagService.GetNoBindDevices()
	if err != nil {
		c.Error(400, "获取设备列表失败: "+err.Error())
	}

	c.Success(devices)
}

// Update @Title 设备标签
// @Description 给指定设备打上标签信息，如果key值相同则为更新操作
// @Param   Authorization  header  string  true  "Bearer YourToken"
// @Param   body    	   body    dtos.TagAddRequest  true  "更新内容"
// @Success 200 {object} controllers.Result
// @Failure 400 "请求出错"
// @router /update [post]
func (c *DeviceController) Update() {
	var req dtos.TagAddRequest
	if err := json.NewDecoder(c.Ctx.Request.Body).Decode(&req); err != nil {
		c.Error(400, "参数解析失败: "+err.Error())
	}

	if req.DeviceName == "" {
		c.Error(400, "设备ID不能为空")
	}

	if req.TagName == "productId" || req.TagName == "productName" {
		c.Error(400, "内置标签无法使用")
	}

	err := tagService.AddTag(req.DeviceName, req.TagName, req.TagValue)
	if err != nil {
		c.Error(400, "更新设备失败: "+err.Error())
	}

	c.SuccessMsg()
}

// Bind @Title 设备绑定产品
// @Description 给指定设备打上产品信息，如果key值相同则为更新操作
// @Param   Authorization  header  string  true  "Bearer YourToken"
// @Param   body  body  dtos.ProductAddRequest  true  "请求体: {productId: 123, devicesName: [...]}"
// @Success 200 {object} controllers.Result
// @Failure 400 "请求出错"
// @router /bind [post]
func (c *DeviceController) Bind() {
	var req dtos.ProductAddRequest
	if err := json.NewDecoder(c.Ctx.Request.Body).Decode(&req); err != nil {
		c.Error(400, "参数解析失败: "+err.Error())
	}

	// 参数校验
	if req.ProductID <= 0 {
		c.Error(400, "产品ID必须大于0")
	}
	if len(req.DeviceName) == 0 {
		c.Error(400, "设备ID列表不能为空")
	}
	o := orm.NewOrm()
	var product models.Product
	product.Id = req.ProductID
	err := o.Read(&product)
	if err != nil {
		c.Error(400, "产品ID无效")
	}

	// 循环绑定产品ID标签
	err = services.BindDeviceTags(tagService, req.DeviceName, req.ProductID, product.Name)
	if err != nil {
		c.Error(400, "绑定失败: "+err.Error())
	}
	c.Success("批量绑定成功")
}

// Delete @Title 删除设备
// @Description 根据设备ID删除设备，仅支持停止上传数据后删除
// @Param   Authorization  header  string  true  "Bearer YourToken"
// @Param   deviceName     query    string  true  "设备ID"
// @Success 200 {object} controllers.Result
// @Failure 400 "请求出错"
// @router /delete [post]
func (c *DeviceController) Delete() {
	deviceID := c.GetString("deviceName")
	if deviceID == "" {
		c.Error(400, "设备ID不能为空")
	}

	err := tagService.DeleteDevices(deviceID)
	if err != nil {
		c.Error(400, "删除设备失败: "+err.Error())
	}

	c.SuccessMsg()
}
