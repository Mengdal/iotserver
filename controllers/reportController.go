package controllers

import (
	"iotServer/models"
	"iotServer/services"
)

// ReportController 综合报表
type ReportController struct {
	BaseController
}

// TimePeriodReport @Title 时间段报表
// @Description 根据时间范围生成设备数据报表
// @Param   Authorization  header   string  true   "Bearer YourToken"
// @Param   page           query    int     false  "页码，默认1"
// @Param   size           query    int     false  "每页大小，默认10"
// @Param   start          query    string  true   "开始时间，格式: 2006-01-02 15:04:05"
// @Param   end            query    string  true   "结束时间，格式: 2006-01-02 15:04:05"
// @Param   projectId      query    int64   false  "项目ID"
// @Param   search         query    string  false  "搜索关键字"
// @Param   productId      query    int64   true   "产品ID"
// @Param   propertyIds    query    string  false  "属性ID列表，逗号分隔"
// @Param   resourceType   query    string  false  "资源类型（产品设备树Product、位置树Position、标签树Group、原始查询Raw）"
// @Param   resourceIds    query    string  false  "资源ID列表，逗号分隔（对应ProductId、PositionIds、GroupIds、DeviceIds）"
// @Param   type           query    string  true   "日期类型"
// @Param   reportType     query    string  true   "报表类型(report 时段报表 analysis 用量分析 compareAnalysis 同比分析)"
// @Param   multipleType   query    bool    false  "是否启用倍率计算"
// @Success 200 {object} utils.PageResult "报表数据"
// @Failure 400 "错误信息"
// @router /timePeriod [post]
func (c *ReportController) TimePeriodReport() {
	// 获取查询参数
	page, _ := c.GetInt("page", 1)
	size, _ := c.GetInt("size", 10)
	start := c.GetString("start")
	end := c.GetString("end")
	projectId, _ := c.GetInt64("projectId", 0)
	search := c.GetString("search")
	productId, _ := c.GetInt64("productId", 0)
	propertyIds := c.GetString("propertyIds")
	dateType := c.GetString("type")
	reportType := c.GetString("reportType")
	resourceType := c.GetString("resourceType")
	resourceIds := c.GetString("resourceIds")
	multipleType, _ := c.GetBool("multipleType", false)

	// 参数校验
	if start == "" || end == "" {
		c.Error(400, "开始时间和结束时间不能为空")
	}
	userId, _ := c.Ctx.Input.GetData("user_id").(int64)
	tenantId, _ := models.GetUserTenantId(userId)
	var projectIds []int64
	var err error
	if projectId != 0 {
		projectIds, err = models.GetUserProjectIds(userId, projectId)
		if err != nil {
			c.Error(400, err.Error())
		}
	}

	reportService, err := services.NewReportService()
	// 调用服务获取报表数据
	result, err := reportService.TimePeriodReport(
		page, size, start, end, tenantId, projectIds, search,
		productId, propertyIds, resourceType, resourceIds, dateType, multipleType, reportType)

	if err != nil {
		c.Error(400, "生成报表失败: "+err.Error())
	}

	c.Success(result)
}
