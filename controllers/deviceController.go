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
	service services.DevicesService
}

var tagService = iotp.TagService{}

// GetAllDevices @Title 获取所有设备
// @Description 获取系统中所有绑定设备的信息
// @Param   Authorization  header  string  true  "Bearer YourToken"
// @Param   page           query   int     false "当前页码，默认1"
// @Param   size           query   int     false "每页数量，默认10"
// @Param   productId      query   int64   false "产品ID"
// @Param   projectId      query   int64   false "项目ID"
// @Param   positionId     query   int64   false "位置ID"
// @Param   status         query   string  false "设备状态"
// @Param   name           query   string  false "设备名称(模糊查询)
// @Success 200 {object} controllers.SimpleResult
// @Failure 400 "请求错误"
// @router /all [post]
func (c *DeviceController) GetAllDevices() {
	projectId, _ := c.GetInt64("projectId")
	userId, _ := c.Ctx.Input.GetData("user_id").(int64)
	projectIds, err := models.GetUserProjectIds(userId, projectId)
	tenantId, err := models.GetUserIsRoot(userId)
	isTenant := true
	if err != nil {
		isTenant = false
	}

	page, _ := c.GetInt("page", 1)
	size, _ := c.GetInt("size", 10)
	productId, _ := c.GetInt64("productId")
	positionId, _ := c.GetInt64("positionId")
	status := c.GetString("status")
	name := c.GetString("name")

	devices, err := c.service.GetAllDevices(page, size, tenantId, projectIds, productId, positionId, status, name, isTenant)
	if err != nil {
		c.Error(400, "获取设备列表失败: "+err.Error())
	}

	c.Success(devices)
}

// GetTagsTree @Title 获取设备点树
// @Description 根据传入产品获取点树
// @Param   Authorization  header  string  true  "Bearer YourToken"
// @Param   productId      query   string  true  "产品ID"
// @Param   projectId      query   int64   false "项目ID"
// @Success 200 {object} controllers.SimpleResult "返回结果树"
// @Failure 400 "错误信息"
// @router /getTagsTree [post]
func (c *DeviceController) GetTagsTree() {
	productId := c.GetString("productId")
	projectId, _ := c.GetInt64("projectId")
	userId, _ := c.Ctx.Input.GetData("user_id").(int64)
	projectIds, err := models.GetUserProjectIds(userId, projectId)
	if err != nil {
		c.Error(400, err.Error())
	}
	data, err := tagService.DevicesTagsTree2(projectIds, "product_id", productId)
	if err != nil {
		c.Error(400, err.Error())
	}
	c.Success(data)
}

// GetDevicesTree @Title 获取设备树
// @Description 根据传入产品设备树
// @Param   Authorization  header  string  true  "Bearer YourToken"
// @Param   projectId      query   int64   false "项目ID"
// @Success 200 {object} controllers.SimpleResult "返回结果树"
// @Failure 400 "错误信息"
// @router /getDevicesTree [post]
func (c *DeviceController) GetDevicesTree() {
	projectId, _ := c.GetInt64("projectId")
	userId, _ := c.Ctx.Input.GetData("user_id").(int64)
	projectIds, err := models.GetUserProjectIds(userId, projectId)
	data, err := c.service.GetDevicesTree(projectIds)
	if err != nil {
		c.Error(400, err.Error())
	}
	c.Success(data)
}

// GetNoBindDevices @Title 查询未绑定设备(iotEdgeDB)
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

// Update @Title 设备标签(iotEdgeDB)
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
	product := models.Product{Id: req.ProductID}
	if err := o.Read(&product); err != nil {
		c.Error(400, "产品ID无效")
	}
	tenantId := product.Department.Id
	// 循环绑定产品ID标签
	err := services.BindDeviceTags(tagService, tenantId, req.DeviceName, req.ProductID, product.Name, product.Key, product.CategoryId, req.Tags)
	if err != nil {
		c.Error(400, "绑定失败: "+err.Error())
	}
	// 加载超级表缓存
	go services.LoadAllDeviceCategoryKeys()
	c.SuccessMsg()
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

	//err := tagService.DeleteDevices(deviceID)
	userId, _ := c.Ctx.Input.GetData("user_id").(int64)
	tenantId, _ := models.GetUserTenantId(userId)
	err := c.service.DeleteDevice(deviceID, tenantId)
	if err != nil {
		c.Error(400, "删除设备失败: "+err.Error())
	}

	c.SuccessMsg()
}
