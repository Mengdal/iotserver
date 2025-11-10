package controllers

import (
	"github.com/beego/beego/v2/client/orm"
	"iotServer/services"
	"strconv"
)

type ProjectController struct {
	BaseController
	service services.ProjectService
}

// Detail 根据项目ID查询项目详情
// @Title Get Project Detail
// @Description 根据项目ID查询项目详情
// @Param   Authorization header   string  true    "Bearer YourToken"
// @Param   projectId     query    int     true    "项目ID"
// @Success 200 {object} controllers.SimpleResult
// @router /detail [post]
func (c *ProjectController) Detail() {
	// 验证用户权限
	userId, ok := c.Ctx.Input.GetData("user_id").(int64)
	if !ok {
		c.Error(400, "用户ID不存在")
	}

	projectId, err := strconv.ParseInt(c.GetString("projectId"), 10, 64)
	if err != nil {
		c.Error(400, "Invalid project ID")
	}

	// 权限校验
	if !CheckModelOwnership(orm.NewOrm(), "project", projectId, userId, "user_id") {
		c.Error(400, "无权限访问该项目")
	}

	project, err := c.service.Detail(projectId)
	if err != nil {
		c.Error(400, err.Error())
	}

	c.Success(project)
}

// Create 创建项目
// @Title Create Project
// @Description 创建新项目
// @Param   Authorization header   string  true    "Bearer YourToken"
// @Param   name          query    string  true    "项目名称"
// @Param   description   query    string  false   "项目介绍"
// @Param   address       query    string  false   "项目地址"
// @Param   people        query    string  false   "负责人"
// @Param   mobile        query    string  false   "手机号"
// @Param   personNum     query    string  false   "办公人数"
// @Param   area          query    string  false   "面积"
// @Success 200 {object} controllers.SimpleResult
// @router /create [post]
func (c *ProjectController) Create() {
	// 验证用户权限
	userId, ok := c.Ctx.Input.GetData("user_id").(int64)
	if !ok {
		c.Error(400, "用户ID不存在")
	}

	// 获取参数
	name := c.GetString("name")
	if name == "" {
		c.Error(400, "项目名称不能为空")
	}

	description := c.GetString("description")
	address := c.GetString("address")
	people := c.GetString("people")
	mobile := c.GetString("mobile")
	personNum := c.GetString("personNum")
	area := c.GetString("area")

	// 调用服务创建项目
	projectId, err := c.service.Create(userId, name, description, address, people, mobile, personNum, area)
	if err != nil {
		c.Error(400, "创建项目失败: "+err.Error())
	}

	c.Success(projectId)
}

// Setting 项目设置
// @Title Project Setting
// @Description 项目设置
// @Param   Authorization header   string  true    "Bearer YourToken"
// @Param   projectId     query    int     true        "项目ID"
// @Param   name          query    string  false       "项目名称"
// @Param   description   query    string  false       "项目介绍"
// @Param   address       query    string  false       "项目地址"
// @Param   people        query    string  false       "负责人"
// @Param   mobile        query    string  false       "手机号"
// @Param   personNum     query    string  false       "办公人数"
// @Param   area          query    string  false       "面积"
// @Param   enable        query    bool    false       "是否将scada设为首页"
// @Param   time          query    string  false       "激活时间 2025-05-01"
// @Param   indexType     query    string  false       "首页类型"
// @Success 200 {object} controllers.SimpleResult
// @router /setting [post]
func (c *ProjectController) Setting() {
	// 验证用户权限
	userId, ok := c.Ctx.Input.GetData("user_id").(int64)
	if !ok {
		c.Error(400, "用户ID不存在")
	}

	projectId, err := strconv.ParseInt(c.GetString("projectId"), 10, 64)
	if err != nil {
		c.Error(400, "Invalid project ID")
	}

	// 权限校验
	if !CheckModelOwnership(orm.NewOrm(), "project", projectId, userId, "user_id") {
		c.Error(400, "无权限访问该项目")
	}

	// 获取可选参数
	name := c.GetString("name")
	desc := c.GetString("description")
	addr := c.GetString("address")
	people := c.GetString("people")
	mobile := c.GetString("mobile")
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
	err = c.service.Edit(projectId,
		&name, &desc, &addr, &people, &mobile,
		&personNum, &area, enable, &timeStr, &indexType)

	if err != nil {
		c.Error(400, "Save failed.")
	}

	c.SuccessMsg()
}

// UploadImage 上传项目图片
// @Title Upload Project Image
// @Description 上传项目图片
// @Param   Authorization header   string  true    "Bearer YourToken"
// @Param   projectId     query    int     true        "项目ID"
// @Param   file          formData file    true        "图片文件"
// @Success 200 {object} controllers.SimpleResult
// @router /uploadImage [post]
func (c *ProjectController) UploadImage() {
	// 验证用户权限
	userId, ok := c.Ctx.Input.GetData("user_id").(int64)
	if !ok {
		c.Error(400, "用户ID不存在")
	}

	projectId, err := strconv.ParseInt(c.GetString("projectId"), 10, 64)
	if err != nil {
		c.Error(400, "Invalid project ID")
	}

	// 权限校验
	if !CheckModelOwnership(orm.NewOrm(), "project", projectId, userId, "user_id") {
		c.Error(400, "无权限访问该项目")
	}

	// 获取上传的文件
	_, fileHeader, err := c.GetFile("file")
	if err != nil {
		c.Error(400, "File not found")
	}

	imageURL, err := c.service.UploadImage(projectId, *fileHeader)
	if err != nil {
		c.Error(400, err.Error())
	}

	c.Success(imageURL)
}

// UploadLogo 上传项目logo
// @Title Upload Project Logo
// @Description 上传项目logo
// @Param   Authorization header   string  true    "Bearer YourToken"
// @Param   projectId     query    int     true        "项目ID"
// @Param   logo          formData file    true        "Logo文件"
// @Success 200 {object} controllers.SimpleResult
// @router /uploadLogo [post]
func (c *ProjectController) UploadLogo() {
	// 验证用户权限
	userId, ok := c.Ctx.Input.GetData("user_id").(int64)
	if !ok {
		c.Error(400, "用户ID不存在")
	}

	projectId, err := strconv.ParseInt(c.GetString("projectId"), 10, 64)
	if err != nil {
		c.Error(400, "Invalid project ID")
	}

	// 权限校验
	if !CheckModelOwnership(orm.NewOrm(), "project", projectId, userId, "user_id") {
		c.Error(400, "无权限访问该项目")
	}

	// 获取上传的文件
	_, fileHeader, err := c.GetFile("logo")
	if err != nil {
		c.Error(400, "File not found")
	}

	logoURL, err := c.service.UploadLogo(projectId, *fileHeader)
	if err != nil {
		c.Error(400, err.Error())
	}

	c.Success(logoURL)
}

// UploadIcon 上传项目图标
// @Title Upload Project Icon
// @Description 上传项目图标
// @Param   Authorization header   string  true    "Bearer YourToken"
// @Param   projectId     query    int     true        "项目ID"
// @Param   icon          formData file    true        "图标文件"
// @Success 200 {object} controllers.SimpleResult
// @router /uploadIcon [post]
func (c *ProjectController) UploadIcon() {
	// 验证用户权限
	userId, ok := c.Ctx.Input.GetData("user_id").(int64)
	if !ok {
		c.Error(400, "用户ID不存在")
	}

	projectId, err := strconv.ParseInt(c.GetString("projectId"), 10, 64)
	if err != nil {
		c.Error(400, "Invalid project ID")
	}

	// 权限校验
	if !CheckModelOwnership(orm.NewOrm(), "project", projectId, userId, "user_id") {
		c.Error(400, "无权限访问该项目")
	}

	// 获取上传的文件
	_, fileHeader, err := c.GetFile("icon")
	if err != nil {
		c.Error(400, "File not found")
	}

	iconURL, err := c.service.UploadIcon(projectId, *fileHeader)
	if err != nil {
		c.Error(400, err.Error())
	}

	c.Success(iconURL)
}
