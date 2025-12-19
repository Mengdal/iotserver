package controllers

import (
	"iotServer/models"
	"iotServer/services"
	"strconv"
)

type TenantController struct {
	BaseController
	service services.TenantService
}

// Setting 租户设置
// @Title Tenant Setting
// @Description 租户设置
// @Param   Authorization header   string  true    "Bearer YourToken"
// @Param   departmentId  query    int     true        "租户ID"
// @Param   name          query    string  false       "租户名称"
// @Param   address       query    string  false       "租户地址"
// @Param   personNum     query    string  false       "办公人数"
// @Param   area          query    string  false       "面积"
// @Param   enable        query    bool    false       "true 项目制;false 集团制"
// @Param   time          query    string  false       "激活时间 2025-05-01"
// @Param   indexType     query    string  false       "首页类型"
// @Success 200 {object} controllers.SimpleResult
// @router /setting [post]
func (c *TenantController) Setting() {
	departmentId, err := strconv.ParseInt(c.GetString("departmentId"), 10, 64)
	if err != nil {
		c.Error(400, "Invalid department ID")
	}
	userId, _ := c.Ctx.Input.GetData("user_id").(int64)
	tenantId, _ := models.GetUserTenantId(userId)
	if departmentId != tenantId {
		c.Error(400, "无操作权限")
	}
	// 获取可选参数
	name := c.GetString("name")
	addr := c.GetString("address")
	personNum := c.GetString("personNum")
	area := c.GetString("area")
	indexType := c.GetString("indexType")
	timeStr := c.GetString("time")

	var enable *bool
	if enableStr := c.GetString("enable"); enableStr != "" {
		if val, err := strconv.ParseBool(enableStr); err == nil {
			enable = &val
		}
	}

	// 调用服务
	err = c.service.Edit(tenantId,
		&name, &addr, &personNum, &area, enable, &timeStr, &indexType)

	if err != nil {
		c.Error(400, "Save failed.")
	}

	c.SuccessMsg()
}

// UploadImage 上传租户图片
// @Title Upload Tenant Image
// @Description 上传租户图片
// @Param   Authorization header   string  true    "Bearer YourToken"
// @Param   departmentId  query    int     true    "租户ID"
// @Param   file          formData file    true    "图片文件"
// @Success 200 {object} controllers.SimpleResult
// @router /uploadImage [post]
func (c *TenantController) UploadImage() {
	userId, _ := c.Ctx.Input.GetData("user_id").(int64)
	tenantId, _ := models.GetUserTenantId(userId)
	departmentId, err := strconv.ParseInt(c.GetString("departmentId"), 10, 64)
	if err != nil {
		c.Error(400, "Invalid departmentId ID")
	}
	if tenantId != departmentId {
		c.Error(400, "无操作权限")
	}
	// 获取上传的文件
	_, fileHeader, err := c.GetFile("file")
	if err != nil {
		c.Error(400, "File not found")
	}

	imageURL, err := c.service.UploadImage(tenantId, *fileHeader)
	if err != nil {
		c.Error(400, err.Error())
	}

	c.Success(imageURL)
}

// UploadLogo 上传租户logo
// @Title Upload Tenant Logo
// @Description 上传租户logo
// @Param   Authorization header   string  true    "Bearer YourToken"
// @Param   departmentId  query    int     true    "租户ID"
// @Param   logo          formData file    true    "Logo文件"
// @Success 200 {object} controllers.SimpleResult
// @router /uploadLogo [post]
func (c *TenantController) UploadLogo() {
	userId, _ := c.Ctx.Input.GetData("user_id").(int64)
	tenantId, _ := models.GetUserTenantId(userId)
	departmentId, err := strconv.ParseInt(c.GetString("departmentId"), 10, 64)
	if err != nil {
		c.Error(400, "Invalid departmentId ID")
	}
	if tenantId != departmentId {
		c.Error(400, "无操作权限")
	}
	// 获取上传的文件
	_, fileHeader, err := c.GetFile("logo")
	if err != nil {
		c.Error(400, "File not found")
	}

	logoURL, err := c.service.UploadLogo(tenantId, *fileHeader)
	if err != nil {
		c.Error(400, err.Error())
	}

	c.Success(logoURL)
}

// UploadIcon 上传租户图标
// @Title Upload Tenant Icon
// @Description 上传租户图标
// @Param   Authorization header   string  true    "Bearer YourToken"
// @Param   departmentId  query    int     true    "租户ID"
// @Param   icon          formData file    true    "图标文件"
// @Success 200 {object} controllers.SimpleResult
// @router /uploadIcon [post]
func (c *TenantController) UploadIcon() {
	userId, _ := c.Ctx.Input.GetData("user_id").(int64)
	tenantId, _ := models.GetUserTenantId(userId)
	departmentId, err := strconv.ParseInt(c.GetString("departmentId"), 10, 64)
	if err != nil {
		c.Error(400, "Invalid departmentId ID")
	}
	if tenantId != departmentId {
		c.Error(400, "无操作权限")
	}
	// 获取上传的文件
	_, fileHeader, err := c.GetFile("icon")
	if err != nil {
		c.Error(400, "File not found")
	}

	iconURL, err := c.service.UploadIcon(tenantId, *fileHeader)
	if err != nil {
		c.Error(400, err.Error())
	}

	c.Success(iconURL)
}
