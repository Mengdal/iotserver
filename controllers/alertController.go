package controllers

import (
	"encoding/json"
	"github.com/beego/beego/v2/client/orm"
	"iotServer/models"
	"iotServer/utils"
	"time"
)

type AlertController struct {
	BaseController
}

// GetAlarmRecord @Title 获取Scada告警记录
// @Description 适用于网关返回的告警记录
// @Param Authorization header string true "Bearer YourToken"
// @Param page query int false "页码，1"
// @Param size query int false "每页数量，10"
// @Param status query string false "状态筛选 (未处理/已处理/忽略)"
// @Param startTime query string false "开始时间(格式: 2006-01-02 15:04:05)"
// @Param endTime query string false "结束时间(格式: 2006-01-02 15:04:05)"
// @Success 200 {object} controllers.SimpleResult
// @Failure 400 "查询出错"
// @router /getAlarmRecord [post]
func (c *AlertController) GetAlarmRecord() {
	page, _ := c.GetInt("page", 1)
	size, _ := c.GetInt("size", 10)
	status := c.GetString("status")
	startTimeStr := c.GetString("startTime")
	endTimeStr := c.GetString("endTime")

	o := orm.NewOrm()
	qs := o.QueryTable(new(models.AlertList)).OrderBy("-TriggerTime")

	// 构建查询条件
	if status != "" {
		qs = qs.Filter("Status", status)
	}

	loc, _ := time.LoadLocation("Asia/Shanghai")
	if startTimeStr != "" {
		startTime, err := time.ParseInLocation("2006-01-02 15:04:05", startTimeStr, loc)
		if err != nil {
			c.Error(400, "开始时间格式错误，请使用: 2000-01-29 15:04:05")
			return
		}
		qs = qs.Filter("TriggerTime__gte", startTime.Unix()*1000)
	}

	if endTimeStr != "" {
		endTime, err := time.ParseInLocation("2006-01-02 15:04:05", endTimeStr, loc)
		if err != nil {
			c.Error(400, "结束时间格式错误，请使用: 2000-01-29 15:04:05")
			return
		}
		qs = qs.Filter("TriggerTime__lte", endTime.Unix()*1000)
	}

	// 使用Paginate方法
	var alerts []*models.AlertList
	pageResult, err := utils.Paginate(qs, page, size, &alerts)
	if err != nil {
		c.Error(400, "查询失败")
	}

	// 构建返回数据
	resultList := make([]map[string]interface{}, 0)
	for _, alert := range alerts {
		var alertContent map[string]interface{}
		if err := json.Unmarshal([]byte(alert.AlertResult), &alertContent); err == nil {
			alertContent["id"] = alert.Id
			alertContent["handler"] = alert.Status
			resultList = append(resultList, alertContent)
		}
	}

	// 替换原始列表为处理后的告警内容
	pageResult.List = resultList

	c.Success(pageResult)
}

// Detail @Title 获取告警详情
// @Description 获取单个告警记录详情
// @Param Authorization header string true "Bearer YourToken"
// @Param id query int64 true "告警记录ID"
// @Success 200 {object} controllers.SimpleResult
// @Failure 404 {object} "告警记录不存在"
// @router /detail [post]
func (c *AlertController) Detail() {
	id, _ := c.GetInt64("id")
	if id == 0 {
		c.Error(400, "id参数不能为空")
	}

	o := orm.NewOrm()
	alert := models.AlertList{Id: id}
	if err := o.Read(&alert); err != nil {
		c.Error(404, "告警记录不存在")
	}

	c.Success(alert)
}

// UpdateStatus @Title 处理告警
// @Description 更新告警处理状态
// @Param Authorization header string true "Bearer YourToken"
// @Param id query int64 true "告警记录ID"
// @Param status query string true "处理(未处理/已处理/忽略)"
// @Success 200 {object} controllers.SimpleResult
// @Failure 400 "请求出错"
// @router /updateStatus [post]
func (c *AlertController) UpdateStatus() {
	id, _ := c.GetInt64("id")
	status := c.GetString("status")
	if id == 0 {
		c.Error(400, "id参数不能为空")
	}

	o := orm.NewOrm()
	alert := models.AlertList{Id: id}
	if err := o.Read(&alert); err != nil {
		c.Error(404, "告警记录不存在")
	}

	alert.Status = status
	alert.TreatedTime = time.Now().Unix()

	if _, err := o.Update(&alert); err != nil {
		c.Error(500, "更新失败")
	}

	c.SuccessMsg()
}

// Delete @Title 删除告警记录
// @Description 删除单个告警记录
// @Param Authorization header string true "Bearer YourToken"
// @Param id query int64 true "告警记录ID"
// @Success 200 {object} controllers.SimpleResult
// @Failure 404 "请求失败"
// @router /delete [post]
func (c *AlertController) Delete() {
	id, _ := c.GetInt64("id")
	if id == 0 {
		c.Error(400, "id参数不能为空")
	}

	o := orm.NewOrm()
	if _, err := o.Delete(&models.AlertList{Id: id}); err != nil {
		c.Error(500, "删除失败")
	}

	c.SuccessMsg()
}
