package controllers

import (
	"context"
	"encoding/json"
	"github.com/beego/beego/v2/client/orm"
	"iotServer/common"
	"iotServer/models"
	"iotServer/models/constants"
	"iotServer/models/dtos"
	"iotServer/services"
	"iotServer/utils"
	"time"
)

// EngineController 规则引擎资源管理
type EngineController struct {
	BaseController
}

// Sources @Title 获取资源列表
// @Description 分页获取资源列表
// @Param   Authorization  header  string  true  "Bearer YourToken"
// @Param   page           query   int     false "当前页码，默认1"
// @Param   size           query   int     false "每页数量，默认10"
// @Param   type           query   string  false "转发类型"
// @Success 200 {object} controllers.SimpleResult "请求成功"
// @Failure 400 用户ID不存在 或 查询失败
// @router /sources [post]
func (c *EngineController) Sources() {
	page, _ := c.GetInt("page", 1)
	size, _ := c.GetInt("size", 10)
	name := c.GetString("type")

	if !constants.IsSourceType(name) {
		c.Error(400, "不支持的消息类型，目前支持：[\"HTTP推送\",\"消息对队列MQTT\",\"消息队列Kafka\",\"InfluxDB\",\"TDengine\"]")
	}
	o := orm.NewOrm()

	// 查询资源
	var dataSources []*models.DataResource
	qs := o.QueryTable(new(models.DataResource)).
		Filter("type", name)
	paginate, err := utils.Paginate(qs, page, size, &dataSources)
	if err != nil {
		c.Error(400, "查询失败")
	}

	c.Success(paginate)
}

// EditSource @Title 创建/更新资源列表
// @Description Id存在/不存在 创建/更新资源实例
// @Param   Authorization  header  string  true  "Bearer YourToken"
// @Param   body       	   body    dtos.EngineOption  true  "type:资源类型，option:资源结构"
// @Success 200 {object} controllers.SimpleResult "请求成功"
// @Failure 400 创建失败
// @router /editSource [post]
func (c *EngineController) EditSource() {
	var req dtos.EngineOption
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &req); err != nil {
		c.Error(400, "参数解析失败: "+err.Error())
	}

	if !constants.IsSourceType(req.Type) {
		c.Error(400, "不支持的消息类型，目前支持：[\"HTTP推送\",\"消息对队列MQTT\",\"消息队列Kafka\",\"InfluxDB\",\"TDengine\"]")
	}

	option := services.ParseOption(req.Type, req.Option)
	if option == nil {
		c.Error(400, req.Type+"格式错误")
	}

	// 查询资源
	var dataSources models.DataResource
	dataSources.Type = req.Type
	options, _ := json.Marshal(option)
	dataSources.Option = string(options)

	if err := services.ValidateConnection(dataSources.Type, option); err != nil {
		dataSources.Health = string(constants.RuleStop)
	} else {
		dataSources.Health = string(constants.RuleStart)
	}
	o := orm.NewOrm()

	if req.Id == 0 {
		dataSources.BeforeInsert()
		_, err := o.Insert(&dataSources)
		if err != nil {
			c.Error(400, "插入失败")
		}
	} else {
		dataSources.Id = req.Id
		dataSources.BeforeUpdate()
		_, err := o.Update(&dataSources)
		if err != nil {
			c.Error(400, "更新失败")
		}
	}

	c.SuccessMsg()
}

// EditEngine @Title 创建/更新规则引擎
// @Description 创建或更新基础的规则引擎
// @Param   Authorization  header  string  true  "Bearer YourToken"
// @Param   body       	   body    dtos.EngineCreate  true  "转发规则（更新+ID,无法更改名称）"
// @Success 200 {object} controllers.Result
// @Failure 400 "请求出错"
// @router /editEngine [post]
func (c *EngineController) EditEngine() {
	var req dtos.EngineCreate
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &req); err != nil {
		c.Error(400, "参数解析失败: "+err.Error())
	}

	o := orm.NewOrm()
	ruleEngine := models.RuleEngine{Name: req.Name, Description: req.Description}
	ruleEngine.Status = string(constants.RuleStop) //默认关闭
	if req.Id == 0 {
		// 创建告警规则
		ruleEngine.BeforeInsert()
		_, err := o.Insert(&ruleEngine)
		if err != nil {
			c.Error(400, "插入失败:"+err.Error())
		}
	} else {
		ruleEngine.Id = req.Id
		ruleEngine.BeforeUpdate()
		_, err := o.Update(&ruleEngine)
		if err != nil {
			c.Error(400, "更新失败")
		}
	}
	c.SuccessMsg()
}

// Engines @Title 获取转发规则列表
// @Description 分页获取规则转发列表
// @Param   Authorization  header  string  true  "Bearer YourToken"
// @Param   page           query   int     false "当前页码，默认1"
// @Param   size           query   int     false "每页数量，默认10"
// @Param   status         query   string  false "状态查询"
// @Param   name           query   string  false "规则名称"
// @Success 200 {object} controllers.SimpleResult "请求成功"
// @Failure 400 用户ID不存在 或 查询失败
// @router /engines [post]
func (c *EngineController) Engines() {
	page, _ := c.GetInt("page", 1)
	size, _ := c.GetInt("size", 10)
	status := c.GetString("status")
	name := c.GetString("name")

	if status != "" && !constants.IsStatusValid(status) {
		c.Error(400, "不支持的状态")
	}
	o := orm.NewOrm()

	// 查询资源
	var ruleEngines []*models.RuleEngine
	qs := o.QueryTable(new(models.RuleEngine))
	if name != "" {
		qs = qs.Filter("name__icontains", name)
	}
	if status != "" {
		qs = qs.Filter("status", status)
	}
	paginate, err := utils.Paginate(qs, page, size, &ruleEngines)
	if err != nil {
		c.Error(400, "查询失败")
	}

	c.Success(paginate)
}

// EngineConfig @Title 配置规则引擎
// @Description 配置规则引擎并更新到 eKuiper
// @Param   Authorization  header  string  true  "Bearer YourToken"
// @Param   body    	   body    dtos.EngineUpdate  true  "规则引擎信息"
// @Success 200 {object} controllers.Result
// @Failure 400 "请求出错"
// @router /engineConfig [post]
func (c *EngineController) EngineConfig() {
	var req dtos.EngineUpdate
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &req); err != nil {
		c.Error(400, "参数解析失败: "+err.Error())
	}
	// 2. 取出设备及属性类型
	o := orm.NewOrm()
	var ruleEngine models.RuleEngine
	var dataSource models.DataResource
	dataSource.Id = req.DataSourceId

	ruleEngine.Id = req.Id
	ruleEngine.Name = req.Name
	ruleEngine.DataResource = &dataSource
	ruleEngine.Status = req.Status
	ruleEngine.Description = req.Description
	filter := req.Filter
	filter.SQL = "select rule_id()," + req.Filter.SelectName + " from stream "
	if req.Filter.Condition != "" {
		filter.SQL += " where " + req.Filter.Condition
	}
	filterMarshal, err := json.Marshal(filter)
	if err != nil {
		c.Error(400, "系统出错: "+err.Error())
	}
	ruleEngine.Filter = string(filterMarshal)
	ruleEngine.BeforeUpdate()

	update, err := o.Update(&ruleEngine)
	if err != nil {
		c.Error(400, "更新失败: "+err.Error())
	} else if update == 0 {
		c.Error(400, "更新失败: 规则引擎不存在")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// ekuiper 规则创建转发接收
	actions := common.GetRuleAlertEkuiperActions(common.CallBackUrl + "/api/ekuiper/callback3")

	if err := common.Ekuiper.RuleExist(ctx, req.Name+"__Engine"); err == nil {
		err = common.Ekuiper.UpdateRule(ctx, actions, req.Name+"__Engine", filter.SQL)
		if err != nil {
			c.Error(400, "更新规则失败: "+err.Error())
		}
	} else {
		err = common.Ekuiper.CreateRule(ctx, actions, req.Name+"__Engine", filter.SQL)
		if err != nil {
			c.Error(400, "更新规则失败: "+err.Error())
		}
	}

	if ruleEngine.Status == string(constants.RuleStart) {
		err = common.Ekuiper.StartRule(ctx, req.Name+"__Engine")
		if err != nil {
			c.Error(400, "规则启动失败: "+err.Error())
		}
	}

	c.SuccessMsg()
}
