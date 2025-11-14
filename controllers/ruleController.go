package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/beego/beego/v2/client/orm"
	"iotServer/common"
	"iotServer/models"
	"iotServer/models/constants"
	"iotServer/models/dtos"
	"iotServer/utils"
	"strings"
	"time"
)

// RuleController 告警规则管理
type RuleController struct {
	BaseController
}

// Edit @Title 创建/更新告警规则
// @Description 创建或更新基础的告警规则
// @Param   Authorization  header  string  true  "Bearer YourToken"
// @Param   body       	   body    dtos.RuleCreate  true  "规则（更新+ID,无法更改名称）"
// @Success 200 {object} controllers.Result
// @Failure 400 "请求出错"
// @router /edit [post]
func (c *RuleController) Edit() {
	var req dtos.RuleCreate
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &req); err != nil {
		c.Error(400, "参数解析失败: "+err.Error())
	}
	if !constants.IsValidAlertLevel(req.AlertLevel) {
		c.Error(400, "参数不合法")
	}
	o := orm.NewOrm()
	alertRule := models.AlertRule{Name: req.Name, AlertLevel: req.AlertLevel, Description: req.Description}
	alertRule.Status = string(constants.RuleStop)                 //默认关闭
	alertRule.Condition = string(constants.WorkerConditionAnyone) //任意条件触发
	if req.Id == 0 {
		alertRule.AlertType = string(constants.DeviceAlertType)
		// 创建告警规则
		alertRule.BeforeInsert()
		_, err := o.Insert(&alertRule)
		if err != nil {
			c.Error(400, "插入失败:"+err.Error())
		}
	} else {
		alertRule.Id = req.Id
		alertRule.BeforeUpdate()
		_, err := o.Update(&alertRule)
		if err != nil {
			c.Error(400, "更新失败")
		}
	}
	c.SuccessMsg()
}

// Update @Title 配置告警规则
// @Description 配置告警规则并更新到 eKuiper
// @Param   Authorization  header  string  true  "Bearer YourToken"
// @Param   body    		body    dtos.RuleUpdateRequest  true  "规则信息"
// @Success 200 {object} controllers.Result
// @Failure 400 "请求出错"
// @router /update [post]
func (c *RuleController) Update() {
	// 1. 初步解析请求参数
	var req dtos.RuleUpdateRequest
	var typeStyle string
	if err := json.NewDecoder(c.Ctx.Request.Body).Decode(&req); err != nil {
		c.Error(400, "参数解析失败: "+err.Error())
	}

	// 2. 取出设备及属性类型
	o := orm.NewOrm()
	ids := req.SubRule[0].DeviceId

	if req.SubRule[0].Trigger == string(constants.DeviceDataTrigger) {
		code := req.SubRule[0].Option["code"]
		productId := req.SubRule[0].ProductId
		var property models.Properties
		err := o.QueryTable(new(models.Properties)).Filter("code", code).Filter("product_id", productId).One(&property)
		if err != nil {
			c.Error(400, "属性类型不存在")
		}

		var specs map[string]string
		err = json.Unmarshal([]byte(property.TypeSpec), &specs)
		if err != nil {
			c.Error(400, "属性类型有误")
		}
		typeStyle = specs["type"] // int float text ..
	}

	// 3. 校验参数结构
	err := dtos.ValidateRuleUpdateRequest(&req, typeStyle)
	if err != nil {
		c.Error(400, "参数有误："+err.Error())
	}

	// 4. 构建 SQL 语句
	sql := req.BuildEkuiperSql(ids, typeStyle)
	if sql == "" {
		c.Error(400, "SQL 生成失败")
	}

	// 5. 构建上下文
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if common.CallBackUrl == "" && common.LocalHost == "" {
		c.Error(400, "无法获取本机IP,请手动配置")
	}
	if common.LocalHost != "" {
		common.CallBackUrl = "http://" + common.LocalHost + ":" + common.Port
	}
	actions := common.GetRuleAlertEkuiperActions(common.CallBackUrl + "/api/ekuiper/callback")

	// 6. 检查规则是否已存在,存在则更新，不存在则新建
	var alertRule models.AlertRule
	if err = o.QueryTable(new(models.AlertRule)).Filter("name", req.Name).One(&alertRule); err != nil {
		c.Error(400, "规则不存在，请创建后配置")
	}

	if err := common.Ekuiper.RuleExist(ctx, req.Name); err == nil {
		err = common.Ekuiper.UpdateRule(ctx, actions, req.Name, sql)
		if err != nil {
			c.Error(400, "更新规则失败: "+err.Error())
		}
	} else {
		err = common.Ekuiper.CreateRule(ctx, actions, req.Name, sql)
		if err != nil {
			c.Error(400, "更新规则失败: "+err.Error())
		}
		alertRule.Condition = string(constants.WorkerConditionAnyone)
		alertRule.Status = string(constants.RuleStart)
	}

	// 7 . 全部操作完成写盘保存
	alertRule.SilenceTime = req.SilenceTime
	idsJson, _ := json.Marshal(ids)
	subRuleJson, _ := json.Marshal(req.SubRule)
	subNotifyJson, _ := json.Marshal(req.Notify)
	alertRule.DeviceId = string(idsJson)
	alertRule.SubRule = string(subRuleJson)
	alertRule.Notify = string(subNotifyJson)
	//更新规则后，规则需要启动后restart
	if alertRule.Status == string(constants.RuleStart) {
		err = common.Ekuiper.StartRule(ctx, req.Name)
		err = common.Ekuiper.RestartRule(ctx, req.Name)
		if err != nil {
			c.Error(400, "规则启动失败"+err.Error())
			alertRule.Status = string(constants.RuleStop)
		}
	}

	if _, err = o.Update(&alertRule); err != nil {
		c.Error(400, "更新规则出错"+err.Error())
	}

	// 6. 返回成功响应
	c.Success(map[string]interface{}{
		"ruleId":  req.Name,
		"message": "规则更新成功",
	})
}

// GetRuleStatus @Title 获取规则状态详情
// @Description 获取指定规则或全部规则的状态信息
// @Param   Authorization  header  string  true  "Bearer YourToken"
// @Param   ruleId     	   query    string  false  "规则ID,不填默认查询全部"
// @Success 200 {object} controllers.SimpleResult
// @Failure 400 "请求出错"
// @router /status [get]
func (c *RuleController) GetRuleStatus() {
	var stats map[string]interface{}
	var err error
	ruleID := c.GetString("ruleId")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if ruleID == "" || ruleID == "undefined" || ruleID == " " {
		stats, err = common.Ekuiper.GetAllRuleStats(ctx)
		if err != nil {
			c.Error(400, "查询所有规则失败: "+err.Error())
		}

	} else {
		// 检查规则是否存在
		err = common.Ekuiper.RuleExist(ctx, ruleID)
		if err != nil {
			c.Error(400, "检查规则存在性失败: "+err.Error())
		}

		// 获取规则状态
		stats, err = common.Ekuiper.GetRuleStats(ctx, ruleID)
		if err != nil {
			c.Error(400, "获取规则状态失败: "+err.Error())
		}
	}
	c.Success(stats)
}

// OperateRule @Title 启动/停止/重启/删除规则
// @Description 操作指定的Ekuiper规则
// @Param   Authorization  header  string  true  "Bearer YourToken"
// @Param   body     body  dtos.OperateRuleReq true  "操作内容（start、stop、restart、delete）, 例如 {\"action\":\"start\"}"
// @Success 200 {object} controllers.Result
// @Failure 400 "请求出错"
// @router /operate [post]
func (c *RuleController) OperateRule() {
	var req dtos.OperateRuleReq
	if err := json.NewDecoder(c.Ctx.Request.Body).Decode(&req); err != nil {
		c.Error(400, "参数解析失败: "+err.Error())
	}

	if req.RuleID == "" || req.Action == "" {
		c.Error(400, "ruleId 和 action 参数不能为空")
	}

	var rule models.AlertRule
	rule.Name = req.RuleID
	o := orm.NewOrm()
	if err := o.Read(&rule, "Name"); err != nil {
		c.Error(400, "规则查询失败")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var err error
	var message string
	flag := false

	switch req.Action {
	case "start":
		err = common.Ekuiper.StartRule(ctx, req.RuleID)
		message = "规则已启动"
		rule.Status = string(constants.RuleStart)
	case "stop":
		err = common.Ekuiper.StopRule(ctx, req.RuleID)
		message = "规则已停止"
		rule.Status = string(constants.RuleStop)
	case "delete":
		err = common.Ekuiper.DeleteRule(ctx, req.RuleID)
		message = "规则已删除"
		flag = true
	case "restart":
		err = common.Ekuiper.RestartRule(ctx, req.RuleID)
		message = "规则已重启"
		rule.Status = string(constants.RuleStart)
	default:
		c.Error(400, "无效的操作类型，仅支持 start、stop、restart、delete")
	}

	if err != nil && !strings.Contains(err.Error(), "not found") {
		c.Error(400, fmt.Sprintf("%s失败: %v", message, err))
	}

	// 更新数据库规则状态
	if flag {
		if _, err = o.Delete(&rule); err != nil {
			c.Error(400, "删除规则失败")
		}
	} else {
		rule.BeforeUpdate()
		if _, err = o.Update(&rule); err != nil {
			c.Error(400, "更新规则失败")
		}
	}

	c.Success(map[string]interface{}{
		"ruleId":  req.RuleID,
		"message": message,
	})
}

// List @Title 查询告警规则列表
// @Description 分页告警规则
// @Param   Authorization  header  string  true  "Bearer YourToken"
// @Param	name		   query   string  false "单位名称"
// @Param	status		   query   string  false "状态"
// @Param   page           query   int     false "当前页码，默认1"
// @Param   size           query   int     false "每页数量，默认10"
// @Success 200 {object} controllers.Result
// @Failure 400 "请求出错"
// @router /list [post]
func (c *RuleController) List() {
	page, _ := c.GetInt("page", 1)
	size, _ := c.GetInt("size", 10)
	name := c.GetString("name")
	status := c.GetString("status")

	var engines []*models.AlertRule
	o := orm.NewOrm()

	qs := o.QueryTable(new(models.AlertRule))
	if name != "" {
		qs = qs.Filter("name__icontains", name)
	}
	if status != "" {
		qs = qs.Filter("status", status)
	}
	paginate, err := utils.Paginate(qs, page, size, &engines)
	if err != nil {
		c.Error(400, "查询失败")
	}

	c.Success(paginate)
}
