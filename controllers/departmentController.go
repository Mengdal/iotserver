package controllers

import (
	"encoding/json"
	"iotServer/models"
	"iotServer/models/dtos"
	"iotServer/services"
	"strconv"
)

// DepartmentController 部门管理控制器
type DepartmentController struct {
	BaseController
	service services.DepartmentService
}

// CreateDepartment @Title 创建部门
// @Description 创建新部门
// @Param Authorization header string true "Bearer YourToken"
// @Param body body dtos.CreateDepartmentRequest true "创建部门请求参数"
// @Success 200 {object} SimpleResult "创建成功"
// @Failure 400 {object} SimpleResult "参数错误或创建失败"
// @router /create [post]
func (c *DepartmentController) CreateDepartment() {
	var req dtos.CreateDepartmentRequest
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &req); err != nil {
		c.Error(400, "参数解析失败: "+err.Error())
	}

	if !models.IsValidLevelType(req.LevelType) {
		c.Error(400, "部门类型非法")
	}
	if req.Name == "" {
		c.Error(400, "部门名称不能为空")
	}

	DepartmentID, err := c.service.CreateDepartment(req.Name, req.Leader, req.Phone, req.Email, req.Status, req.Remark, req.ParentID, req.Sort, req.LevelType)
	if err != nil {
		c.Error(400, "创建部门失败: "+err.Error())
	}
	if len(req.DeviceIDs) > 0 && req.LevelType == "PROJECT" {
		if err := c.service.AssignDevicesToDepartment(DepartmentID, req.DeviceIDs); err != nil {
			c.Error(400, "分配设备失败: "+err.Error())
		}
	}

	c.SuccessMsg()
}

// UpdateDepartment @Title 更新部门
// @Description 更新部门信息
// @Param Authorization header string true "Bearer YourToken"
// @Param body body dtos.UpdateDepartmentRequest true "更新部门请求参数"
// @Success 200 {object} SimpleResult "更新成功"
// @Failure 400 {object} SimpleResult "参数错误或更新失败"
// @router /update [post]
func (c *DepartmentController) UpdateDepartment() {
	var req dtos.UpdateDepartmentRequest
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &req); err != nil {
		c.Error(400, "参数解析失败: "+err.Error())
	}

	if req.ID == 0 {
		c.Error(400, "部门ID不能为空")
	}

	if req.Name == "" {
		c.Error(400, "部门名称不能为空")
	}
	userId, _ := c.Ctx.Input.GetData("user_id").(int64)
	tenantId, _ := models.GetUserTenantId(userId)
	err := c.service.UpdateDepartment(tenantId, req.ID, req.Name, req.Leader, req.Phone, req.Email, req.Status, req.Remark, req.ParentID, req.Sort, req.DeviceIds)
	if err != nil {
		c.Error(400, "更新部门失败: "+err.Error())
	}
	c.SuccessMsg()
}

// DeleteDepartment @Title 删除部门
// @Description 删除指定部门
// @Param Authorization header string true "Bearer YourToken"
// @Param id query int64 true "部门ID"
// @Success 200 {object} SimpleResult "删除成功"
// @Failure 400 {object} SimpleResult "参数错误或删除失败"
// @router /delete [post]
func (c *DepartmentController) DeleteDepartment() {
	id, err := c.GetInt64("id")
	if err != nil || id == 0 {
		c.Error(400, "部门ID参数错误")
	}
	userId, _ := c.Ctx.Input.GetData("user_id").(int64)
	tenantId, _ := models.GetUserTenantId(userId)
	err = c.service.DeleteDepartment(tenantId, id)
	if err != nil {
		c.Error(400, "删除部门失败: "+err.Error())
	}

	c.SuccessMsg()
}

// GetDepartmentTree @Title 获取部门树
// @Description 获取部门树形结构（支持模糊搜索）
// @Param Authorization header string true "Bearer YourToken"
// @Param keyword query string false "部门名称（模糊搜索）"
// @Success 200 {object} SimpleResult "查询成功"
// @Failure 400 {object} SimpleResult "查询失败"
// @router /tree [post]
func (c *DepartmentController) GetDepartmentTree() {
	userId, _ := c.Ctx.Input.GetData("user_id").(int64)
	tenantId, _ := models.GetUserTenantId(userId)
	keyword := c.GetString("keyword")
	tree, err := c.service.GetDepartmentTree(tenantId, keyword)
	if err != nil {
		c.Error(400, "查询部门树失败: "+err.Error())
	}

	c.Success(tree)
}

// NoDepartmentDevices @Title 获取未绑定部门设备列表
// @Description 获取直接属于该部门的设备，不输入departmentId则直接获取未分配部门的设备
// @Param Authorization header string true "Bearer YourToken"
// @Param departmentId query int64 false "部门ID"
// @Param page query int false "页码，默认1"
// @Param size query int false "每页数量，默认10"
// @Success 200 {object} SimpleResult "查询成功"
// @Failure 400 {object} SimpleResult "参数错误或查询失败"
// @router /noDepartmentDevices [post]
func (c *DepartmentController) NoDepartmentDevices() {
	page, _ := c.GetInt("page", 1)
	size, _ := c.GetInt("size", 10)
	departmentID, err := c.GetInt64("departmentId")
	userId, _ := c.Ctx.Input.GetData("user_id").(int64)
	tenantId, _ := models.GetUserTenantId(userId)

	result, err := c.service.GetNoDepartmentDevices(tenantId, departmentID, page, size)
	if err != nil {
		c.Error(400, "查询设备失败: "+err.Error())
	}

	c.Success(result)
}

// Detail 根据部门ID查询部门详情
// @Description TENANT查询租户，DEPARTMENT普通查询，PROJECT查询设备 （不包括子部门，配合/tree获取部门树使用）
// @Param   Authorization header   string  true    "Bearer YourToken"
// @Param   departmentId  query    int     true    "部门ID"
// @Param page query int false "页码，默认1"
// @Param size query int false "每页数量，默认10"
// @Success 200 {object} controllers.SimpleResult
// @router /detail [post]
func (c *DepartmentController) Detail() {
	page, _ := c.GetInt("page", 1)
	size, _ := c.GetInt("size", 10)
	userId, _ := c.Ctx.Input.GetData("user_id").(int64)
	tenantId, _ := models.GetUserTenantId(userId)

	departmentId, err := strconv.ParseInt(c.GetString("departmentId"), 10, 64)
	if err != nil {
		c.Error(400, "Invalid TENANT ID")
	}
	service := services.TenantService{}
	project, err := service.Detail(tenantId, departmentId, page, size)
	if err != nil {
		c.Error(400, err.Error())
	}

	c.Success(project)
}

/*// GetDepartmentDevices @Title 获取部门设备列表
// @Description 获取部门及其子部门的所有设备列表
// @Param Authorization header string true "Bearer YourToken"
// @Param departmentId query int64 true "部门ID"
// @Param page query int false "页码，默认1"
// @Param size query int false "每页数量，默认10"
// @Success 200 {object} SimpleResult "查询成功"
// @Failure 400 {object} SimpleResult "参数错误或查询失败"
// @router /devices [get]
func (c *DepartmentController) GetDepartmentDevices() {
	departmentID, err := c.GetInt64("departmentId")
	if err != nil || departmentID == 0 {
		c.Error(400, "部门ID参数错误")
	}

	page, _ := c.GetInt("page", 1)
	size, _ := c.GetInt("size", 10)

	result, err := c.service.GetDepartmentDevices(departmentID, page, size)
	if err != nil {
		c.Error(400, "查询设备失败: "+err.Error())
	}

	c.Success(result)
}

// GetDepartmentDeviceTree @Title 获取部门设备树
// @Description 获取部门及其子部门的设备树形结构
// @Param Authorization header string true "Bearer YourToken"
// @Param departmentId query int64 true "部门ID"
// @Success 200 {object} SimpleResult "查询成功"
// @Failure 400 {object} SimpleResult "参数错误或查询失败"
// @router /deviceTree [get]
func (c *DepartmentController) GetDepartmentDeviceTree() {
	departmentID, err := c.GetInt64("departmentId")
	if err != nil || departmentID == 0 {
		c.Error(400, "部门ID参数错误")
	}

	result, err := c.service.GetDepartmentDeviceTree(departmentID)
	if err != nil {
		c.Error(400, "查询设备树失败: "+err.Error())
	}

	c.Success(result)
}*/
