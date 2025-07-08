package edgeController

import (
	"encoding/json"
	"iotServer/controllers"
	"iotServer/iotp"
	"iotServer/models"
)

type HistoryController struct {
	controllers.BaseController
}

// QueryHistory @Title 历史数据查询
// @Description 根据传入参数查询历史数据
// @Param   body           body    models.HistoryObject  true  "查询条件"
// @Success 200 {object}   controllers.SimpleResult "返回结果"
// @Failure 400 "错误信息"
// @router /queryHistory [post]
func (c *HistoryController) QueryHistory() {
	var objects models.HistoryObject
	rawJSON := c.Ctx.Input.RequestBody
	err := json.Unmarshal(rawJSON, &objects)
	if err != nil {
		c.Error(400, err.Error())
	}
	data, err := iotp.HistoryQuery(objects)
	if err != nil {
		c.Error(400, err.Error())
	}
	c.Success(data)
}

// AggQueryHistory @Title 聚合查询
// @Description 执行聚合查询获取统计类数据
// @Param   body           body    models.AggObject  true  "聚合查询参数"
// @Success 200 {object} controllers.SimpleResult "返回聚合结果"
// @Failure 400 "错误信息"
// @router /aggQueryHistory [post]
func (c *HistoryController) AggQueryHistory() {
	var objects models.AggObject
	rawJSON := c.Ctx.Input.RequestBody
	err := json.Unmarshal(rawJSON, &objects)
	if err != nil {
		c.Error(400, err.Error())
	}
	echartsData, err := iotp.AggQuery(objects)
	if err != nil {
		c.Error(400, err.Error())
	}
	c.Success(echartsData)
}

// DiffQueryHistory @Title 差值查询
// @Description 查询两个时间点之间的差值变化
// @Param   body           body    models.DiffObject  true  "差值查询参数"
// @Success 200 {object} controllers.SimpleResult "返回差值结果"
// @Failure 400 "错误信息"
// @router /diffQueryHistory [post]
func (c *HistoryController) DiffQueryHistory() {
	var objects models.DiffObject
	rawJSON := c.Ctx.Input.RequestBody
	err := json.Unmarshal(rawJSON, &objects)
	if err != nil {
		c.Error(400, err.Error())
	}
	data, err := iotp.DiffQuery(objects)
	if err != nil {
		c.Error(400, err.Error())
	}
	c.Success(data)
}

// BoolQueryHistory @Title 布尔型历史查询
// @Description 查询布尔类型的历史数据变化
// @Param   body           body    models.BoolObject  true  "布尔查询参数"
// @Success 200 {object} controllers.SimpleResult "返回布尔型历史数据"
// @Failure 400  "错误信息"
// @router /boolQueryHistory [post]
func (c *HistoryController) BoolQueryHistory() {
	var objects models.BoolObject
	rawJSON := c.Ctx.Input.RequestBody
	err := json.Unmarshal(rawJSON, &objects)
	if err != nil {
		c.Error(400, err.Error())
	}
	data, err := iotp.BoolQuery(objects)
	if err != nil {
		c.Error(400, err.Error())
	}
	c.Success(data)
}
