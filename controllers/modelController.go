package controllers

import (
	"encoding/json"
	"github.com/beego/beego/v2/client/orm"
	"iotServer/models"
	"iotServer/models/constants"
	"iotServer/models/dtos"
	"iotServer/utils"
	"strconv"
	"strings"
)

type ModelController struct {
	BaseController
}

// Get @Title 获取模型列表
// @Description 查询当前产品下的模型详情
// @Param   Authorization  header  string  true  "Bearer YourToken"
// @Param   id          query   int64  true  "产品ID"
// @Param   page           query   int     false "当前页码，默认1"
// @Param   size           query   int     false "每页数量，默认10"
// @Success 200 {object} controllers.SimpleResult "请求成功"
// @Failure 400 用户ID不存在 或 查询失败
// @router /get [post]
func (c *ModelController) Get() {
	page, _ := c.GetInt("page", 1)
	size, _ := c.GetInt("size", 10)
	id, _ := c.GetInt64("id")

	o := orm.NewOrm()

	// 查询产品
	var products []*models.Product
	qs := o.QueryTable(new(models.Product)).Filter("id", id)
	paginate, err := utils.Paginate(qs, page, size, &products)
	if err != nil {
		c.Error(400, "查询失败")
	}
	// 加载产品的物模型数据
	for _, p := range products {
		_, _ = o.LoadRelated(p, "Properties")
		_, _ = o.LoadRelated(p, "Events")
		_, _ = o.LoadRelated(p, "Actions")
	}
	c.Success(paginate)
}

// Delete @Title 删除模型
// @Description 具有权限校验
// @Param   Authorization  header    string  true  "Bearer YourToken"
// @Param   thingModelType query     string  true  "物模型类型:参考ThingModelType"
// @Param   id                       query   int64  true  "模型ID"
// @Success 200 {object} controllers.SimpleResult "请求成功"
// @Failure 400 用户ID不存在 或 查询失败
// @router /delete [post]
func (c *ModelController) Delete() {
	id, _ := c.GetInt64("id")
	thingModelType := c.GetString("thingModelType")
	userId, _ := c.Ctx.Input.GetData("user_id").(int64)
	o := orm.NewOrm()

	var productId int64
	var model interface{}
	switch thingModelType {
	case string(constants.ModelTypeEvent):
		model = &models.Events{Id: id}
		_ = o.Raw("SELECT product_id FROM events WHERE id = ?", id).QueryRow(&productId)
	case string(constants.ModelTypeAction):
		model = &models.Actions{Id: id}
		_ = o.Raw("SELECT product_id FROM actions WHERE id = ?", id).QueryRow(&productId)
	case string(constants.ModelTypeProperty):
		model = &models.Properties{Id: id}
		err := o.Raw("SELECT product_id FROM properties WHERE id = ?", id).QueryRow(&productId)
		if err != nil {
			c.Error(400, "未知的模型类型"+err.Error())
		}
	default:
		c.Error(400, "未知的模型类型")
	}

	if ownership := CheckModelOwnership(o, "product", productId, userId, ""); !ownership {
		c.Error(400, "权限不足")
	}

	_, err := o.Delete(model, "id")
	if err != nil {
		c.Error(400, "删除失败")
	}
	c.SuccessMsg()
}

// Create @Title 添加物模型
// @Description 创建物模型：支持属性、事件、服务
// @Param   Authorization  header  string  true  "Bearer YourToken"
// @Param   body  body  dtos.ThingModelDTO  true  "物模型参数"
// @Success 200 {object} controllers.SimpleResult "请求成功"
// @Failure 400 参数错误 / 权限不足
// @router /create [post]
func (c *ModelController) Create() {
	var req dtos.ThingModelDTO
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &req); err != nil {
		c.Error(400, "请求参数格式错误"+err.Error())
	}

	o := orm.NewOrm()

	// 查询产品
	var product models.Product
	product.Id = req.ProductId
	if err := o.Read(&product); err != nil {
		c.Error(400, "产品不存在")
	}

	if product.Status == int(constants.ProductRelease) {
		c.Error(400, "产品已发布，禁止修改物模型")
	}

	switch req.ThingModelType {
	case constants.ModelTypeProperty:
		if req.Property == nil {
			c.Error(400, "属性内容不能为空")
		}
		var property models.Properties

		property.Product = &product
		property.Name = req.Name
		property.Code = req.Code
		property.Description = req.Description
		property.Tag = string(req.Tag)
		property.Type = req.Type

		property.AccessMode = string(req.Property.AccessModel)
		property.Require = req.Property.Require
		datatype := req.Property.DataType
		specs := req.Property.TypeSpec
		specsBytes, _ := json.Marshal(specs)
		typeSpec := map[string]interface{}{
			"type":  datatype,
			"specs": string(specsBytes),
		}
		marshal, err := json.Marshal(typeSpec)
		if err != nil {
			c.Error(500, "参数有误"+err.Error())
		}
		property.TypeSpec = string(marshal)

		if req.Id == 0 {
			property.Id = 0
			_ = property.BeforeInsert()
			if _, err := o.Insert(&property); err != nil {
				c.Error(500, "保存属性失败: "+err.Error())
			}
		} else {
			property.Id = req.Id
			_ = property.BeforeUpdate()
			if _, err := o.Update(&property, "name", "code", "description", "tag", "access_mode", "type", "type_spec", "updated"); err != nil {
				c.Error(500, "保存属性失败: "+err.Error())
			}
		}

		c.Success(property.Id)

	case constants.ModelTypeEvent:
		if req.Event == nil {
			c.Error(400, "事件内容不能为空")
		}
		var event models.Events

		event.Product = &product
		event.Name = req.Name
		event.Code = req.Code
		event.Description = req.Description
		event.Tag = string(req.Tag)
		event.EventType = string(req.Event.EventType)
		event.Type = req.Type

		var outputParams []models.InputOutput
		// 插入输出参数
		for _, param := range req.Event.OutPutParam {
			// 构建 TypeSpec 格式
			typeSpec := map[string]interface{}{
				"type":  param.DataType,
				"specs": param.TypeSpec,
			}
			marshal, err := json.Marshal(typeSpec)
			if err != nil {
				c.Error(500, "参数有误"+err.Error())
			}

			output := models.InputOutput{
				Code:     param.Code,
				Name:     param.Name,
				TypeSpec: string(marshal),
			}
			outputParams = append(outputParams, output)
		}
		marshal, err := json.Marshal(outputParams)
		if err != nil {
			c.Error(500, "输入参数序列化失败: "+err.Error())
		}
		event.OutputParams = string(marshal)

		if req.Id == 0 {
			event.Id = 0
			_ = event.BeforeInsert()
			if _, err := o.Insert(&event); err != nil {
				c.Error(500, "保存事件失败: "+err.Error())
			}
		} else {
			event.Id = req.Id
			_ = event.BeforeUpdate()
			if _, err := o.Update(&event, "name", "code", "description", "tag", "event_type", "type", "output_params", "updated"); err != nil {
				c.Error(500, "保存事件失败: "+err.Error())
			}
		}

		c.Success(event)

	case constants.ModelTypeAction:
		if req.Action == nil {
			c.Error(400, "服务内容不能为空")
		}
		var action models.Actions

		action.Id = 0
		action.Product = &product
		action.Name = req.Name
		action.Code = req.Code
		action.Description = req.Description
		action.Tag = string(req.Tag)
		action.CallType = string(req.Action.CallType)
		action.Type = req.Type

		// --- 构建输入参数 ---
		var inputParams []models.InputOutput
		for _, param := range req.Action.InPutParam {
			typeSpec := map[string]interface{}{
				"type":  param.DataType,
				"specs": param.TypeSpec,
			}
			marshal, err := json.Marshal(typeSpec)
			if err != nil {
				c.Error(500, "输入参数格式错误: "+err.Error())
			}
			inputParams = append(inputParams, models.InputOutput{
				Code:     param.Code,
				Name:     param.Name,
				TypeSpec: string(marshal),
			})
		}
		input, err := json.Marshal(inputParams)
		if err != nil {
			c.Error(500, "输入参数序列化失败: "+err.Error())
		}
		action.InputParams = string(input)

		// --- 构建输出参数 ---
		var outputParams []models.InputOutput
		for _, param := range req.Action.OutPutParam {
			typeSpec := map[string]interface{}{
				"type":  param.DataType,
				"specs": param.TypeSpec,
			}
			marshal, err := json.Marshal(typeSpec)
			if err != nil {
				c.Error(500, "输出参数格式错误: "+err.Error())
			}
			outputParams = append(outputParams, models.InputOutput{
				Code:     param.Code,
				Name:     param.Name,
				TypeSpec: string(marshal),
			})
		}
		output, err := json.Marshal(outputParams)
		if err != nil {
			c.Error(500, "输出参数序列化失败: "+err.Error())
		}
		action.OutputParams = string(output)

		if req.Id == 0 {
			action.Id = 0
			_ = action.BeforeInsert()
			if _, err := o.Insert(&action); err != nil {
				c.Error(500, "保存服务失败: "+err.Error())
			}
		} else {
			action.Id = req.Id
			_ = action.BeforeUpdate()
			if _, err := o.Update(&action, "name", "code", "description", "tag", "call_type", "type", "input_params", "output_params", "updated"); err != nil {
				c.Error(500, "保存服务失败: "+err.Error())
			}
		}
		c.Success(action)

	default:
		c.Error(400, "无效的物模型类型")
	}
}

// Template @Title 获取模型模板列表
// @Description 查询内置模型，支持详细查询
// @Param   Authorization  header  string  true  "Bearer YourToken"
// @Param   id             query   int64   false  "非必填，模型产品ID用于查看功能定义"
// @Param   name           query   string  false  "支持模糊搜索，仅支持全部查询时"
// @Param   page           query   int     false "当前页码，默认1"
// @Param   size           query   int     false "每页数量，默认10"
// @Success 200 {object} controllers.SimpleResult "请求成功"
// @Failure 400 用户ID不存在 或 查询失败
// @router /template [post]
func (c *ModelController) Template() {
	page, _ := c.GetInt("page", 1)
	size, _ := c.GetInt("size", 10)
	id, _ := c.GetInt64("id")
	name := c.GetString("name")

	o := orm.NewOrm()
	qs := o.QueryTable(new(models.Category))
	// 查询产品
	if id == 0 {
		var category []*models.Category
		if name != "" {
			qs = qs.Filter("categoryName__icontains", name)
		}
		paginate, err := utils.Paginate(qs, page, size, &category)
		if err != nil {
			c.Error(400, "查询失败")
		}
		c.Success(paginate)
	} else {
		var category models.Category
		if err := qs.Filter("id", id).One(&category); err != nil {
			c.Error(400, "查询失败")
		}
		var thingModel models.ThingModel
		thingModel.CategoryKey = category.CategoryKey
		if err := o.Read(&thingModel, "CategoryKey"); err != nil {
			c.Error(400, "查询失败,"+err.Error())
		}

		// 解析ThingModelJSON，提取properties、events、actions
		var thingModelData map[string]interface{}
		var properties, events, actions interface{}

		if thingModel.ThingModelJson != "" {
			if err := json.Unmarshal([]byte(thingModel.ThingModelJson), &thingModelData); err == nil {
				if props, exists := thingModelData["properties"]; exists {
					properties = props
				}
				if evts, exists := thingModelData["events"]; exists {
					events = evts
				}
				if acts, exists := thingModelData["services"]; exists {
					actions = acts
				}
			}
		}
		// 构建响应数据，格式与hummingbird保持一致
		response := dtos.ThingModelTemplateResponse{
			Id:             strconv.FormatInt(thingModel.Id, 10),
			CategoryName:   thingModel.CategoryName,
			CategoryKey:    thingModel.CategoryKey,
			ThingModelJSON: "",
			Properties:     properties,
			Events:         events,
			Actions:        actions,
		}
		c.Success(response)
	}
}

// TypeList @Title 查询模型标签列表
// @Description 查询模型标签
// @Param   Authorization  header  string  true  "Bearer YourToken"
// @Param   productId      query   int64   true  "必填产品ID"
// @Param   name           query   string  false "支持模糊搜索"
// @Success 200 {object} controllers.SimpleResult "请求成功"
// @Failure 400 查询失败
// @router /typeList [post]
func (c *ModelController) TypeList() {
	productId, _ := c.GetInt64("productId")
	name := c.GetString("name")
	o := orm.NewOrm()

	// 定义一个轻量 struct 仅映射 Type 字段
	type OnlyType struct {
		Type string `orm:"column(type)"`
	}

	// 用 map 去重
	typeMap := make(map[string]struct{})

	// --- 查询 Properties 的 Type ---
	var propResults []OnlyType
	_, err := o.QueryTable(new(models.Properties)).
		Filter("Product__Id", productId).
		Filter("Type__isnull", false).
		Exclude("Type", "").
		All(&propResults, "Type")
	if err != nil {
		c.Error(400, "查询 Properties 失败: "+err.Error())
		return
	}
	for _, r := range propResults {
		typeMap[r.Type] = struct{}{}
	}

	// --- 查询 Events 的 Type ---
	var eventResults []OnlyType
	_, err = o.QueryTable(new(models.Events)).
		Filter("Product__Id", productId).
		Filter("Type__isnull", false).
		Exclude("Type", "").
		All(&eventResults, "Type")
	if err != nil {
		c.Error(400, "查询 Events 失败: "+err.Error())
		return
	}
	for _, r := range eventResults {
		typeMap[r.Type] = struct{}{}
	}

	// --- 查询 Actions 的 Type ---
	var actionResults []OnlyType
	_, err = o.QueryTable(new(models.Actions)).
		Filter("Product__Id", productId).
		Filter("Type__isnull", false).
		Exclude("Type", "").
		All(&actionResults, "Type")
	if err != nil {
		c.Error(400, "查询 Actions 失败: "+err.Error())
		return
	}
	for _, r := range actionResults {
		typeMap[r.Type] = struct{}{}
	}

	// 转换为切片
	var uniqueTypes []string
	for t := range typeMap {
		// 如果传了 name 参数，执行模糊匹配
		if name == "" || strings.Contains(strings.ToLower(t), strings.ToLower(name)) {
			uniqueTypes = append(uniqueTypes, t)
		}
	}

	c.Success(uniqueTypes)
}
