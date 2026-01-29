package controllers

import (
	"iotServer/models"
	"iotServer/services"
	"strconv"
)

type PositionController struct {
	BaseController
	service services.PositionService
}

// Create 创建位置
// @Title Create Position
// @Description 创建位置
// @Param   Authorization    header   string  true    "Bearer YourToken"
// @Param   name        	 query    string  true    "位置名称"
// @Param   parentId     	 query    int     false   "父节点ID"
// @Success 200 {object} web.Result
// @router /create [post]
func (c *PositionController) Create() {
	name := c.GetString("name")
	if name == "" {
		c.Error(400, "Name is required")
	}

	parentIdStr := c.GetString("parentId")
	var parentId *int64
	if parentIdStr != "" {
		pid, err := strconv.ParseInt(parentIdStr, 10, 64)
		if err != nil {
			c.Error(400, "Invalid parent ID")
		}
		parentId = &pid
	}
	userId, _ := c.Ctx.Input.GetData("user_id").(int64)
	tenantId, err := models.GetUserTenantId(userId)
	err = c.service.Create(tenantId, name, parentId)
	if err != nil {
		c.Error(400, err.Error())
	} else {
		c.SuccessMsg()
	}
}

// Delete 删除位置
// @Title Delete Position
// @Description 删除位置
// @Param   Authorization   header  string       true        "Bearer YourToken"
// @Param   id     query    int     true        "位置ID"
// @Success 200 {object} web.Result
// @router /delete [post]
func (c *PositionController) Delete() {
	id, err := strconv.ParseInt(c.GetString("id"), 10, 64)
	if err != nil {
		c.Error(400, "Invalid ID")
	}

	userId, _ := c.Ctx.Input.GetData("user_id").(int64)
	tenantId, err := models.GetUserTenantId(userId)
	err = c.service.DeleteAll(tenantId, id)
	if err != nil {
		c.Error(400, err.Error())
	} else {
		c.SuccessMsg()
	}
}

// Edit 编辑位置
// @Title Edit Position
// @Description 编辑位置
// @Param   Authorization header   string  true        "Bearer YourToken"
// @Param   id            query    int     true        "位置ID"
// @Param   name          query    string  true        "位置名称"
// @Param   parentId      query    int     false       "父节点ID"
// @Success 200 {object} web.Result
// @router /edit [post]
func (c *PositionController) Edit() {
	id, err := strconv.ParseInt(c.GetString("id"), 10, 64)
	if err != nil {
		c.Error(400, "Invalid ID")
	}

	name := c.GetString("name")
	if name == "" {
		c.Error(400, "Name is required")
	}

	userId, _ := c.Ctx.Input.GetData("user_id").(int64)
	tenantId, err := models.GetUserTenantId(userId)
	err = c.service.Edit(tenantId, id, name)
	if err != nil {
		c.Error(400, err.Error())
	} else {
		c.SuccessMsg()
	}
}

// List 获取位置树
// @Title Get Position Tree
// @Description 获取位置树
// @Param   Authorization header   string  true        "Bearer YourToken"
// @Success 200 {object} web.Result
// @router /tree [post]
func (c *PositionController) List() {
	userId, _ := c.Ctx.Input.GetData("user_id").(int64)
	tenantId, err := models.GetUserTenantId(userId)
	tree, err := c.service.TreeOnly(tenantId)
	if err != nil {
		c.Error(400, err.Error())
	} else {
		c.Success(tree)
	}
}

// ListArea 获取区域树
// @Title Get Area Tree
// @Description 获取区域树
// @Param   Authorization header   string  true        "Bearer YourToken"
// @Success 200 {object} web.Result
// @router /treeArea [post]
func (c *PositionController) ListArea() {
	//userId, _ := c.Ctx.Input.GetData("user_id").(int64)
	//tenantId, err := models.GetUserTenantId(userId)
	tree, err := c.service.GetAreaTreeWithDepartmentCount()
	if err != nil {
		c.Error(400, err.Error())
	} else {
		c.Success(tree)
	}
}
