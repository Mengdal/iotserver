package controllers

import (
	"fmt"
	"github.com/beego/beego/v2/client/orm"
	"iotServer/models"
	"iotServer/models/constants"
	"iotServer/services"
	"iotServer/utils"
	"strconv"
)

type ProductController struct {
	BaseController
}

// Get @Title 获取产品列表
// @Description 分页获取产品列表，支持父子用户权限控制
// @Param   Authorization  header  string  true  "Bearer YourToken"
// @Param   page           query   int     false "当前页码，默认1"
// @Param   size           query   int     false "每页数量，默认10"
// @Param   name           query   string  false "模糊搜索"
// @Success 200 {object} controllers.SimpleResult "请求成功"
// @Failure 400 用户ID不存在 或 查询失败
// @router /get [post]
func (c *ProductController) Get() {
	page, _ := c.GetInt("page", 1)
	size, _ := c.GetInt("size", 10)
	name := c.GetString("name")

	userId, ok := c.Ctx.Input.GetData("user_id").(int64)
	if !ok {
		c.Error(400, "用户ID不存在")
	}

	o := orm.NewOrm()

	// 获取当前用户信息
	var currentUser models.User
	currentUser.Id = userId
	err := o.Read(&currentUser)
	if err != nil {
		c.Error(400, "用户不存在")
	}

	// 如果是父用户（ParentId == nil），获取自己和所有子用户的产品
	var userIds []int64
	if currentUser.ParentId == nil {
		userIds, _ = GetAllSubUserIds(userId)
		userIds = append(userIds, userId) // 包括自己
	} else {
		userIds = []int64{userId} // 子用户只能查看自己的产品
	}

	// 查询产品
	var products []*models.Product
	qs := o.QueryTable(new(models.Product)).
		Filter("user_id__in", userIds)
	// 如果提供了名称参数，则添加模糊搜索条件
	if name != "" {
		qs = qs.Filter("name__icontains", name)
	}
	paginate, err := utils.Paginate(qs, page, size, &products)
	if err != nil {
		c.Error(400, "查询失败")
	}

	c.Success(paginate)
}

// GetAllSubUserIds 递归获取某个用户下的所有子用户ID
func GetAllSubUserIds(userId int64) ([]int64, error) {
	o := orm.NewOrm()
	var users []*models.User
	_, err := o.QueryTable(new(models.User)).Filter("parent_id", userId).All(&users)
	if err != nil {
		return nil, err
	}

	var ids []int64
	for _, user := range users {
		ids = append(ids, user.Id)
		subIds, _ := GetAllSubUserIds(user.Id)
		ids = append(ids, subIds...)
	}
	return ids, nil
}

// Detail @Title 获取产品详情
// @Description 产品物模型
// @Param   Authorization  header  string  true  "Bearer YourToken"
// @Param   productId      query   int     false "产品Id"
// @Success 200 {object} controllers.SimpleResult "请求成功"
// @Failure 400 用户ID不存在 或 查询失败
// @router /detail [post]
func (c *ProductController) Detail() {
	productId, _ := c.GetInt64("productId")

	o := orm.NewOrm()

	// 查询产品
	var product models.Product
	product.Id = productId
	err := o.QueryTable(new(models.Product)).Filter("id", productId).One(&product)

	if err != nil {
		c.Error(400, "查询失败")
	}

	// 查询属性
	o.LoadRelated(&product, "Properties")
	// 查询事件
	o.LoadRelated(&product, "Events")
	// 查询动作
	o.LoadRelated(&product, "Actions")

	c.Success(product)
}

// Create @Title 创建产品
// @Description 创建新产品，仅支持部分字段，自动绑定当前用户
// @Param   Authorization  header  string  true  "Bearer YourToken"
// @Param   name         query   string  true  "产品名称"
// @Param   status       query    bool   true  "是否启用"
// @Param   description  query   string  false "描述"
// @Param   nodeType     query   string  false "类型(默认：网关子设备)"
// @Param   factory      query   string  false "工厂名称"
// @Param   categoryId   query   string  false "内置标准物模型品类"
// @Success 200 {object} controllers.SimpleResult "返回产品ID"
// @Failure 400 参数错误 / 权限不足
// @router /create [post]
func (c *ProductController) Create() {

	id := utils.GenerateID()
	secret := utils.GenerateDeviceSecret(15)

	name := c.GetString("name")
	status, _ := c.GetBool("status")
	description := c.GetString("description")
	nodeType := c.GetString("nodeType", "网关子设备") // 控制器代码
	categoryId, _ := c.GetInt64("categoryId")

	if name == "" {
		c.Error(400, "参数有误")
	}

	userId, ok := c.Ctx.Input.GetData("user_id").(int64)
	if !ok {
		c.Error(400, "用户ID不存在")
	}

	o := orm.NewOrm()

	product := models.Product{
		Id:             id,
		Name:           name,
		DataFormat:     string(constants.Standard),
		Status:         convertStatus(status),
		Description:    description,
		Key:            secret,
		CloudProductId: secret,
		Platform:       string(constants.PlanformLocal),
		Protocol:       string(constants.MQTT),
		NodeType:       nodeType,
	}

	// 获取用户对象
	var user models.User
	user.Id = userId
	product.User = &user

	// Created / Modified
	_ = product.BeforeInsert()

	//用户选择了标准品类
	if categoryId != 0 {
		// 初始化物模型数据
		var properties []*models.Properties
		var events []*models.Events
		var actions []*models.Actions
		//TODO 初步处理标准模型的生成 event及actions未处理
		err := services.ParseThingModelToEntities(categoryId, o, &properties, &events, &actions)
		if err != nil {
			c.Error(400, err.Error())
		}

		// 保存物模型数据
		for _, property := range properties {
			property.Product = &product
			property.BeforeInsert()
			if _, err := o.Insert(property); err != nil {
				c.Error(500, "保存属性失败: "+err.Error())
			}
		}

		for _, event := range events {
			event.Product = &product
			event.BeforeInsert()
			if _, err := o.Insert(event); err != nil {
				c.Error(500, "保存事件失败: "+err.Error())
			}
		}

		for _, action := range actions {
			action.Product = &product
			action.BeforeInsert()
			if _, err := o.Insert(action); err != nil {
				c.Error(500, "保存服务失败: "+err.Error())
			}
		}
	}

	_, err := o.Insert(&product)
	if err != nil {
		c.Error(400, "创建失败: "+err.Error())
	}

	c.Success(product.Id)
}

// Update @Title 更新产品
// @Description 修改已有产品信息（支持部分字段）
// @Param   Authorization    header   string  true        "Bearer YourToken"
// @Param   id          query   int64  true  "产品ID"
// @Param	name	    query	string	false	"产品名称"
// @Param   status      query    bool   true  "是否启用"
// @Param	description	query	string	false	"描述"
// @Success 200 {object} controllers.SimpleResult "操作成功"
// @Failure 400 参数错误 / 无权限
// @router /update [post]
func (c *ProductController) Update() {

	//产品ID
	id, _ := c.GetInt64("id")

	userId, ok := c.Ctx.Input.GetData("user_id").(int64)
	if !ok {
		c.Error(400, "用户ID不存在")
	}

	o := orm.NewOrm()

	var product models.Product
	product.Id = id

	err := o.Read(&product)
	if err != nil {
		c.Error(400, "产品不存在")
	}

	// 权限判断（假设你有 CheckModelOwnership 函数）
	var productIDInt int64
	_, err = fmt.Sscanf(strconv.FormatInt(id, 10), "%d", &productIDInt)
	if err != nil {
		c.Error(400, "产品ID必须是数字")
	}
	fmt.Println("产品ID", productIDInt)

	if !CheckModelOwnership(o, "product", productIDInt, userId, "user_id") {
		c.Error(400, "无权限")
	}

	// 更新字段
	if name := c.GetString("name"); name != "" {
		product.Name = name
	}
	if description := c.GetString("description"); description != "" {
		product.Description = description
	}
	status, _ := c.GetBool("status")
	product.Status = convertStatus(status)

	// 更新数据库
	_ = product.BeforeUpdate()
	_, err = o.Update(&product)
	if err != nil {
		c.Error(400, "更新失败")
	}
	c.SuccessMsg()
}

// Delete @Title 删除产品
// @Description 根据ID删除产品
// @Param   Authorization    header   string  true        "Bearer YourToken"
// @Param	id		query	int64	true	"产品ID"
// @Success 200 {object} controllers.SimpleResult "删除成功"
// @Failure 400 参数错误 / 无权限 / 删除失败
// @router /delete [post]
func (c *ProductController) Delete() {
	id, _ := c.GetInt64("id")

	userId, ok := c.Ctx.Input.GetData("user_id").(int64)
	if !ok {
		c.Error(400, "用户ID不存在")
	}

	o := orm.NewOrm()

	fmt.Println("产品ID", id)
	// 权限判断
	if !CheckModelOwnership(o, "product", id, userId, "user_id") {
		c.Error(400, "无权限")
	}

	// 执行删除
	var product models.Product
	product.Id = id

	_, err := o.Delete(&product)
	if err != nil {
		c.Error(400, "删除失败")
	}

	c.SuccessMsg()
}
