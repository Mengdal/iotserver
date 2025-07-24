package controllers

import (
	"context"
	"encoding/json"
	beego "github.com/beego/beego/v2/server/web"
	"iotServer/common"
	"iotServer/models/dtos"
	"log"
	"time"
)

// EkuiperController Ekuiper规则管理控制器
type EkuiperController struct {
	BaseController
}

var EkuiperServer, _ = beego.AppConfig.String("ekuiperServer")
var ekuiperClient = common.NewEkuiperClient(EkuiperServer)

// CreateRule @Title 创建规则
// @Description 创建一个新的Ekuiper规则
// @Param   Authorization  header  string  true  "Bearer YourToken"
// @Param   body       	   body    dtos.RuleRequest  true  "规则"
// @Success 200 {object} controllers.Result
// @Failure 400 "请求出错"
// @router /create [post]
func (c *EkuiperController) CreateRule() {
	var req dtos.RuleRequest
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &req); err != nil {
		c.Error(400, "参数解析失败: "+err.Error())
	}

	// 验证参数
	if req.RuleID == "" || req.SQL == "" || req.ActionURL == "" {
		c.Error(400, "参数不完整")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 检查规则是否已存在
	if err := ekuiperClient.RuleExist(ctx, req.RuleID); err == nil {
		c.Error(400, "规则已存在")
	}

	// 创建告警动作
	actions := common.GetRuleAlertEkuiperActions(req.ActionURL)

	// 创建规则
	err := ekuiperClient.CreateRule(ctx, actions, req.RuleID, req.SQL)
	if err != nil {
		c.Error(500, "创建规则失败: "+err.Error())
	}

	// 启动规则
	err = ekuiperClient.StartRule(ctx, req.RuleID)
	if err != nil {
		c.Error(500, "启动规则失败: "+err.Error())
	}

	c.Success(map[string]interface{}{
		"ruleId":  req.RuleID,
		"message": "规则创建并启动成功",
	})
}

// UpdateRule @Title 更新规则
// @Description 更新指定的Ekuiper规则（停止 -> 删除 -> 创建）
// @Param   Authorization  header  string  true  "Bearer YourToken"
// @Param   body    		body    dtos.RuleRequest  true  "规则信息"
// @Success 200 {object} controllers.Result
// @Failure 400 "请求出错"
// @router /update [post]
func (c *EkuiperController) UpdateRule() {
	var req dtos.RuleRequest
	if err := json.NewDecoder(c.Ctx.Request.Body).Decode(&req); err != nil {
		c.Error(400, "参数解析失败: "+err.Error())
	}

	if req.RuleID == "" || req.SQL == "" || req.ActionURL == "" {
		c.Error(400, "ruleId、SQL 和 ActionURL 都不能为空")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := ekuiperClient.RuleExist(ctx, req.RuleID); err != nil {
		c.Error(400, "规则不存在")
	}

	// 创建告警动作
	actions := common.GetRuleAlertEkuiperActions(req.ActionURL)

	// 先尝试停止规则
	err := ekuiperClient.StopRule(ctx, req.RuleID)
	if err != nil {
		c.Error(400, "停止规则失败: "+err.Error())
	}

	// 更新规则
	err = ekuiperClient.UpdateRule(ctx, actions, req.RuleID, req.SQL)
	if err != nil {
		c.Error(400, "更新规则失败: "+err.Error())
	}

	// 重新启动规则
	err = ekuiperClient.StartRule(ctx, req.RuleID)
	if err != nil {
		c.Error(400, "启动规则失败: "+err.Error())
	}

	c.Success(map[string]interface{}{
		"ruleId":  req.RuleID,
		"message": "规则更新并重启成功",
	})
}

// GetRule @Title 规则查询
// @Description 获取指定或全部规则的概览信息
// @Param   Authorization  header  string  true  "Bearer YourToken"
// @Param   ruleId     	 query    string  false  "规则ID,不填默认查询全部"
// @Success 200 {object} controllers.SimpleResult
// @Failure 400 "请求出错"
// @router /rule [get]
func (c *EkuiperController) GetRule() {
	ruleID := c.GetString("ruleId")

	if ruleID == "" {
		rules, err := ekuiperClient.GetAllRules(context.Background())
		if err != nil {
			c.Error(400, "查询规则失败: "+err.Error())
		}
		c.Success(rules)
	} else {
		detail, err := ekuiperClient.GetRule(context.Background(), ruleID)
		if err != nil {
			c.Error(400, "查询规则失败: "+err.Error())
		}
		c.Success(detail)
	}

}

// AlertCallback
// @Title 告警回调接口
// @Description 接收Ekuiper规则触发的告警回调，参数为动态JSON
// @Param   body  body  object  true  "动态参数，内容为eKuiper规则SQL select出的所有字段"
// @Success 200 {object} controllers.SimpleResult
// @router /callback [post]
func (c *EkuiperController) AlertCallback() {
	var req map[string]interface{}
	if err := json.NewDecoder(c.Ctx.Request.Body).Decode(&req); err != nil {
		c.Error(400, "参数解析失败: "+err.Error())
	}
	log.Printf("收到告警回调: %+v", req)
	// 你可以通过 req["rule_id"], req["deviceId"] 等方式获取字段

	// 这里可以添加告警处理逻辑，比如：
	// 1. 保存到数据库
	// 2. 发送通知
	// 3. 触发其他业务逻辑
	c.Success(map[string]interface{}{
		"received":  true,
		"timestamp": time.Now().Unix(),
	})
}
