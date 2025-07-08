package controllers

import (
	beego "github.com/beego/beego/v2/server/web"
)

type BaseController struct {
	beego.Controller
}

// 简单响应结构（不含 data）
type SimpleResult struct {
	Code    int    `json:"code" example:"200"`
	Message string `json:"message" example:"success"`
}

// 通用响应结构（含 data）
type Result struct {
	SimpleResult
	Data interface{} `json:"data,omitempty"`
}

// 成功响应（无 data）
func (c *BaseController) SuccessMsg() {
	c.Data["json"] = Result{
		SimpleResult: SimpleResult{
			Code:    200,
			Message: "success",
		},
	}
	c.ServeJSON()
	c.StopRun()
}

// 成功响应（带 data）
func (c *BaseController) Success(data interface{}) {
	c.Data["json"] = Result{
		SimpleResult: SimpleResult{
			Code:    200,
			Message: "success",
		},
		Data: data,
	}
	c.ServeJSON()
	c.StopRun()
}

// 错误响应（无 data）
func (c *BaseController) Error(code int, message string) {
	c.Ctx.Output.SetStatus(code)
	c.Data["json"] = Result{
		SimpleResult: SimpleResult{
			Code:    code,
			Message: message,
		},
	}
	c.ServeJSON()
	c.StopRun()
}

// 错误响应（带 data）
func (c *BaseController) ErrorDetail(code int, message string, data interface{}) {
	c.Ctx.Output.SetStatus(code)
	c.Data["json"] = Result{
		SimpleResult: SimpleResult{
			Code:    code,
			Message: message,
		},
		Data: data,
	}
	c.ServeJSON()
	c.StopRun()
}
