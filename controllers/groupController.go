package controllers

import (
	"github.com/beego/beego/v2/client/orm"
	"iotServer/services"
	"strconv"
)

type GroupController struct {
	BaseController
	service services.GroupService
}

// Save 保存标签
// @Title Save Group
// @Description 保存标签
// @Param   Authorization   header   string  true    "Bearer YourToken"
// @Param   id          	query    int     false        "标签ID,(新增/修改)"
// @Param   projectId   	query    int     true         "项目ID"
// @Param   name        	query    string  true         "名称"
// @Param   type        	query    int     false        "类型 0：包含的设备，1：不包含的设备"
// @Param   description 	query    string  false        "描述"
// @Success 200 {object} controllers.SimpleResult
// @router /save [post]
func (c *GroupController) Save() {
	// 验证用户权限
	userId, ok := c.Ctx.Input.GetData("user_id").(int64)
	if !ok {
		c.Error(400, "用户ID不存在")
	}

	// 解析参数
	idStr := c.GetString("id")
	var id *int64
	if idStr != "" {
		if val, err := strconv.ParseInt(idStr, 10, 64); err == nil {
			id = &val
		}
	}

	projectId, err := strconv.ParseInt(c.GetString("projectId"), 10, 64)
	if err != nil {
		c.Error(400, "Invalid project ID")
	}

	// 权限校验
	if !CheckModelOwnership(orm.NewOrm(), "project", projectId, userId, "user_id") {
		c.Error(400, "无权限访问该项目")
	}

	name := c.GetString("name")
	if name == "" {
		c.Error(400, "Name is required")
	}

	typeStr := c.GetString("type")
	var typeVal *int8
	if typeStr != "" {
		if val, err := strconv.ParseInt(typeStr, 10, 8); err == nil {
			val8 := int8(val)
			typeVal = &val8
		}
	}

	description := c.GetString("description")
	var desc *string
	if description != "" {
		desc = &description
	}

	// 调用服务
	err = c.service.Save(id, projectId, name, typeVal, desc)
	if err != nil {
		c.Error(400, err.Error())
	}

	c.SuccessMsg()
}

// Delete 删除标签
// @Title Delete Group
// @Description 删除标签
// @Param   Authorization   header   string  true    "Bearer YourToken"
// @Param   id     query    int     true        "标签ID"
// @Success 200 {object} controllers.SimpleResult
// @router /delete [post]
func (c *GroupController) Delete() {
	id, err := strconv.ParseInt(c.GetString("id"), 10, 64)
	if err != nil {
		c.Error(400, "Invalid ID")
	}

	err = c.service.Delete(id)
	if err != nil {
		c.Error(400, err.Error())
	}

	c.SuccessMsg()
}

// GroupList 标签查询
// @Title Get Group List
// @Description 标签查询
// @Param   Authorization header   string  true         "Bearer YourToken"
// @Param   projectId     query    int     false        "项目ID"
// @Param   page          query    int     false        "页数"
// @Param   size          query    int     false        "记录数"
// @Success 200 {object} controllers.SimpleResult
// @router /list [post]
func (c *GroupController) GroupList() {
	// 验证用户权限
	userId, ok := c.Ctx.Input.GetData("user_id").(int64)
	if !ok {
		c.Error(400, "用户ID不存在")
	}

	// 解析参数
	page, _ := c.GetInt("page", 1)
	size, _ := c.GetInt("size", 10)

	projectIdStr := c.GetString("projectId")
	var projectId *int64
	if projectIdStr != "" {
		if val, err := strconv.ParseInt(projectIdStr, 10, 64); err == nil {
			projectId = &val

			// 权限校验
			if !CheckModelOwnership(orm.NewOrm(), "project", *projectId, userId, "user_id") {
				c.Error(400, "无权限访问该项目")
				return
			}
		}
	}

	// 调用服务
	result, err := c.service.PageByProjectId(page, size, projectId)
	if err != nil {
		c.Error(400, err.Error())
	}

	c.Success(result)
}

// UnGroupList 未绑定标签的设备
// @Title Get UnGroup List
// @Description 未绑定标签的设备
// @Param   Authorization header   string  true    	   "Bearer YourToken"
// @Param   projectId     query    int     true        "项目ID"
// @Success 200 {object} controllers.SimpleResult
// @router /unGroupList [post]
func (c *GroupController) UnGroupList() {
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

	// 调用服务
	devices, err := c.service.UnGroupList(projectId)
	if err != nil {
		c.Error(400, err.Error())
	}

	c.Success(devices)
}

// BatchGroup 设备批量绑定标签
// @Title Batch Group
// @Description 设备批量绑定标签
// @Param   Authorization header   string  true        "Bearer YourToken"
// @Param   projectId     query    int     true        "项目ID"
// @Param   groupId       query    int     true        "标签id"
// @Param   deviceIds     query    string  true        "设备id列表"
// @Success 200 {object} controllers.SimpleResult
// @router /batchGroup [post]
func (c *GroupController) BatchGroup() {
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

	groupId, err := strconv.ParseInt(c.GetString("groupId"), 10, 64)
	if err != nil {
		c.Error(400, "Invalid group ID")
	}

	deviceIds := c.GetString("deviceIds")
	if deviceIds == "" {
		c.Error(400, "Device IDs are required")
	}

	// 调用服务
	err = c.service.BatchGroup(projectId, groupId, deviceIds)
	if err != nil {
		c.Error(400, err.Error())
	}

	c.SuccessMsg()
}

// UnBatchGroup 设备批量解绑标签
// @Title UnBatch Group
// @Description 设备批量解绑标签
// @Param   Authorization header   string  true        "Bearer YourToken"
// @Param   projectId     query    int     true        "项目ID"
// @Param   deviceIds     query    string  true        "设备id列表"
// @Success 200 {object} controllers.SimpleResult
// @router /unBatchGroup [post]
func (c *GroupController) UnBatchGroup() {
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

	deviceIds := c.GetString("deviceIds")
	if deviceIds == "" {
		c.Error(400, "Device IDs are required")
	}

	// 调用服务
	err = c.service.UnBatchGroup(projectId, deviceIds)
	if err != nil {
		c.Error(400, err.Error())
	}

	c.SuccessMsg()
}
