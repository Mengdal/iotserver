package controllers

import (
	"iotServer/models"
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
// @Param   name        	query    string  true         "名称"
// @Param   type        	query    int     false        "类型 0：包含的设备，1：不包含的设备"
// @Param   description 	query    string  false        "描述"
// @Success 200 {object} controllers.SimpleResult
// @router /save [post]
func (c *GroupController) Save() {
	userId, _ := c.Ctx.Input.GetData("user_id").(int64)
	tenantId, err := models.GetUserTenantId(userId)
	if err != nil {
		c.Error(400, err.Error())
	}

	// 解析参数
	idStr := c.GetString("id")
	var id *int64
	if idStr != "" {
		if val, err := strconv.ParseInt(idStr, 10, 64); err == nil {
			id = &val
		}
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
	err = c.service.Save(id, tenantId, name, typeVal, desc)
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

	userId, _ := c.Ctx.Input.GetData("user_id").(int64)
	tenantId, err := models.GetUserTenantId(userId)
	if err != nil {
		c.Error(400, err.Error())
	}

	err = c.service.Delete(tenantId, id)
	if err != nil {
		c.Error(400, err.Error())
	}

	c.SuccessMsg()
}

// GroupList 标签查询
// @Title Get Group List
// @Description 标签查询
// @Param   Authorization header   string  true         "Bearer YourToken"
// @Param   page          query    int     false        "页数"
// @Param   size          query    int     false        "记录数"
// @Success 200 {object} controllers.SimpleResult
// @router /list [post]
func (c *GroupController) GroupList() {

	// 解析参数
	page, _ := c.GetInt("page", 1)
	size, _ := c.GetInt("size", 10)
	userId, _ := c.Ctx.Input.GetData("user_id").(int64)
	tenantId, _ := models.GetUserTenantId(userId)

	// 调用服务
	result, err := c.service.PageByDepartmentId(page, size, tenantId)
	if err != nil {
		c.Error(400, err.Error())
	}

	c.Success(result)
}

// BatchGroup 设备批量绑定标签
// @Title Batch Group
// @Description 设备批量绑定标签
// @Param   Authorization header   string  true        "Bearer YourToken"
// @Param   groupId       query    int     true        "标签id"
// @Param   deviceIds     query    string  true        "设备id列表"
// @Success 200 {object} controllers.SimpleResult
// @router /batchGroup [post]
func (c *GroupController) BatchGroup() {
	groupId, err := strconv.ParseInt(c.GetString("groupId"), 10, 64)
	if err != nil {
		c.Error(400, "Invalid group ID")
	}

	deviceIds := c.GetString("deviceIds")
	if deviceIds == "" {
		c.Error(400, "Device IDs are required")
	}

	userId, _ := c.Ctx.Input.GetData("user_id").(int64)
	tenantId, _ := models.GetUserTenantId(userId)
	// 调用服务
	err = c.service.BatchGroup(tenantId, groupId, deviceIds)
	if err != nil {
		c.Error(400, err.Error())
	}

	c.SuccessMsg()
}

// UnBatchGroup 设备批量解绑标签
// @Title UnBatch Group
// @Description 设备批量解绑标签
// @Param   Authorization header   string  true        "Bearer YourToken"
// @Param   deviceIds     query    string  true        "设备id列表"
// @Success 200 {object} controllers.SimpleResult
// @router /unBatchGroup [post]
func (c *GroupController) UnBatchGroup() {
	deviceIds := c.GetString("deviceIds")
	if deviceIds == "" {
		c.Error(400, "Device IDs are required")
	}

	userId, _ := c.Ctx.Input.GetData("user_id").(int64)
	tenantId, _ := models.GetUserTenantId(userId)

	// 调用服务
	err := c.service.UnBatchGroup(tenantId, deviceIds)
	if err != nil {
		c.Error(400, err.Error())
	}

	c.SuccessMsg()
}
