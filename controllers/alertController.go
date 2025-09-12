package controllers

import (
	"encoding/json"
	"github.com/beego/beego/v2/client/orm"
	"iotServer/models"
	"iotServer/models/dtos"
	"iotServer/utils"
	"time"
)

type AlertController struct {
	BaseController
}

// GetAlarmRecord @Title 获取Scada告警记录
// @Description 适用于网关返回的告警记录
// @Param body body dtos.AlarmRecordReq true "请求参数"
// @Success 200 {object} controllers.SimpleResult
// @Failure 400 "查询出错"
// @router /getAlarmRecord [post]
func (c *AlertController) GetAlarmRecord() {

	var req dtos.AlarmRecordReq
	if err := c.BindJSON(&req); err != nil {
		c.Error(400, "请求参数解析失败")
	}

	// 参数处理
	if req.Page == 0 {
		req.Page = 1
	}
	if req.Size == 0 {
		req.Size = 10
	}

	o := orm.NewOrm()
	qs := o.QueryTable(new(models.AlertList)).OrderBy("-TriggerTime")

	// 构建查询条件
	if req.Status != "" {
		qs = qs.Filter("Status", req.Status)
	}
	if req.IsSystem != "" {
		var IsSystem bool
		if req.IsSystem == "true" {
			IsSystem = true
		} else {
			IsSystem = false
		}
		qs = qs.Filter("AlertRule__isnull", IsSystem)
	}

	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		// 容错处理：时区文件缺失时 fallback
		loc = time.FixedZone("CST", 8*3600)
	}
	if req.StartTime != "" {
		startTime, err := time.ParseInLocation("2006-01-02 15:04:05", req.StartTime, loc)
		if err != nil {
			c.Error(400, "开始时间格式错误，请使用格式形如: 2000-01-29 15:04:05")
		}
		qs = qs.Filter("TriggerTime__gte", startTime.Unix()*1000)
	}

	if req.EndTime != "" {
		endTime, err := time.ParseInLocation("2006-01-02 15:04:05", req.EndTime, loc)
		if err != nil {
			c.Error(400, "结束时间格式错误，请使用格式形如: 2000-01-29 15:04:05")
		}
		qs = qs.Filter("TriggerTime__lte", endTime.Unix()*1000)
	}

	// 使用Paginate方法
	var alerts []*models.AlertList
	pageResult, err := utils.Paginate(qs, req.Page, req.Size, &alerts)
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
			alertContent["treated_time"] = alert.TreatedTime
			resultList = append(resultList, alertContent)
		}
	}

	// 替换原始列表为处理后的告警内容
	pageResult.List = resultList

	c.Success(pageResult)
}

// Detail @Title 获取告警详情
// @Description 获取单个告警记录详情
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
