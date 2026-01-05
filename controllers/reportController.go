package controllers

import (
	"bytes"
	"fmt"
	"github.com/beego/beego/v2/core/logs"
	"github.com/xuri/excelize/v2"
	"io"
	"iotServer/models"
	"iotServer/services"
	"path/filepath"
	"strings"
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
		if !strings.Contains(err.Error(), "暂无数据") {
			c.Error(400, "生成报表失败: "+err.Error())
		}
	}

	c.Success(result)
}

// ExportTimePeriodReport @Title 报表导出
// @Description 根据时间范围导出设备数据报表到Excel
// @Param   Authorization  header   string  true   "Bearer YourToken"
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
// @Success 200 {file} file "Excel文件"
// @Failure 400 "错误信息"
// @router /export/timePeriod [post]
func (c *ReportController) ExportTimePeriodReport() {
	// 获取查询参数
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
	if err != nil {
		c.Error(500, "创建报表服务失败: "+err.Error())
	}

	// 调用服务导出Excel报表
	excelData, err := reportService.ExportTimePeriodReportToExcel(
		start, end, tenantId, projectIds, search,
		productId, propertyIds, resourceType, resourceIds, dateType, multipleType, reportType)
	if err != nil {
		if !strings.Contains(err.Error(), "暂无数据") {
			c.Error(400, "导出报表失败: "+err.Error())
		}
	}

	// 设置响应头
	c.Ctx.ResponseWriter.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	filename := fmt.Sprintf("report_%s_%s.xlsx", start, end)
	c.Ctx.ResponseWriter.Header().Set("Content-Disposition", "attachment; filename="+filename)

	// 写入Excel数据到响应
	_, err = c.Ctx.ResponseWriter.Write(excelData)
	if err != nil {
		c.Error(500, "写入Excel文件失败: "+err.Error())
	}
}

// HistoryQuery @Title 历史数据普通查询
// @Description 查询设备历史数据，支持自定义查询条数
// @Param   Authorization  header   string                true   "Bearer YourToken"
// @Param   body           body     models.HistoryObject  true   "查询参数"
// @Success 200 {object} services.HistoryQueryResponse "历史数据"
// @Failure 400 "错误信息"
// @router /history/query [post]
func (c *ReportController) HistoryQuery() {
	var req models.HistoryObject

	// 解析请求参数
	if err := c.BindJSON(&req); err != nil {
		c.Error(400, "参数解析失败: "+err.Error())
	}

	// 参数校验
	if len(req.IDs) == 0 {
		c.Error(400, "IDs不能为空")
	}
	if req.StartTime == "" || req.EndTime == "" {
		c.Error(400, "开始时间和结束时间不能为空")
	}

	// 创建报表服务
	reportService, err := services.NewReportService()
	if err != nil {
		c.Error(500, "创建报表服务失败: "+err.Error())
		return
	}

	// 调用服务查询历史数据
	result, err := reportService.HistoryQuery(req)
	if err != nil {
		c.Error(400, "查询历史数据失败: "+err.Error())
	}

	c.Success(result)
}

// AggregateQuery @Title 历史数据聚合查询
// @Description 查询设备历史数据聚合结果，支持多种聚合方式和聚合周期
// @Param   Authorization  header   string            true   "Bearer YourToken"
// @Param   body           body     models.AggObject  true   "查询参数"
// @Success 200 {object} services.HistoryQueryResponse "聚合数据"
// @Failure 400 "错误信息"
// @router /history/aggregate [post]
func (c *ReportController) AggregateQuery() {
	var req models.AggObject

	// 解析请求参数
	if err := c.BindJSON(&req); err != nil {
		c.Error(400, "参数解析失败: "+err.Error())
	}

	// 参数校验
	if len(req.IDs) == 0 {
		c.Error(400, "IDs不能为空")
	}
	if req.StartTime == "" || req.EndTime == "" {
		c.Error(400, "开始时间和结束时间不能为空")
	}
	if req.AggType == "" {
		c.Error(400, "聚合类型不能为空")
	}

	// 验证聚合类型
	validAggTypes := map[string]bool{
		"first": true, "last": true, "min": true, "max": true,
		"sum": true, "avg": true, "average": true, "median": true,
	}
	if !validAggTypes[req.AggType] {
		c.Error(400, "不支持的聚合类型，支持: first/last/min/max/sum/avg/median")
	}

	// 创建报表服务
	reportService, err := services.NewReportService()
	if err != nil {
		c.Error(500, "创建报表服务失败: "+err.Error())
		return
	}

	// 调用服务查询聚合数据
	result, err := reportService.AggregateQuery(req)
	if err != nil {
		c.Error(400, "查询聚合数据失败: "+err.Error())
	}

	c.Success(result)
}

// UploadExcelData @Title 上传Excel数据
// @Description 上传Excel文件并批量插入到TDengine数据库
// @Param   Authorization  header   string  true   "Bearer YourToken"
// @Param   file           formData file    true   "Excel文件"
// @Success 200 {object} map[string]interface{} "上传成功"
// @Failure 400 "错误信息"
// @router /uploadExcel [post]
func (c *ReportController) UploadExcelData() {
	// 获取上传的文件
	file, header, err := c.GetFile("file")
	if err != nil {
		c.Error(400, "获取文件失败: "+err.Error())
	}
	defer file.Close()

	// 验证文件类型
	fileExt := strings.ToLower(filepath.Ext(header.Filename))
	if fileExt != ".xlsx" && fileExt != ".xls" {
		c.Error(400, "文件格式不支持，请上传Excel文件(.xlsx或.xls)")
	}

	// 直接从上传的文件读取内容到内存
	fileBytes, err := io.ReadAll(file)
	if err != nil {
		c.Error(500, "读取文件内容失败: "+err.Error())
	}

	// 创建报表服务
	reportService, err := services.NewReportService()
	if err != nil {
		c.Error(500, "创建报表服务失败: "+err.Error())
	}

	// 直接从内存中的字节创建Excel文件对象
	f, err := excelize.OpenReader(bytes.NewReader(fileBytes))
	if err != nil {
		logs.Warn("打开Excel文件失败: %v", err)
		c.Error(400, "打开Excel文件失败: "+err.Error())
	}
	defer f.Close()

	// 调用服务处理Excel文件并插入数据（传递Excel文件对象而不是文件路径）
	err = reportService.UploadExcelAndInsertDataFromReader(f)
	if err != nil {
		logs.Warn("处理Excel文件失败: %v", err)
		c.Error(400, "处理Excel文件失败: "+err.Error())
	}

	// 返回成功响应
	c.Success(map[string]interface{}{
		"message": "Excel数据上传并插入成功",
	})
}

// DownloadTemplate @Title 下载Excel模板
// @Description 下载用于数据上传的Excel模板
// @Param   Authorization  header   string  true   "Bearer YourToken"
// @Success 200 {object} []byte "Excel模板文件"
// @Failure 400 "错误信息"
// @router /template [get]
func (c *ReportController) DownloadTemplate() {
	// 创建Excel文件
	f := excelize.NewFile()

	// 创建工作表
	sheetName := "Sheet1"
	f.SetSheetName("Sheet1", sheetName)

	// 设置表头 - 设备名称	属性标识	2025/12/26	2025/12/27	2025/12/29 11:11:00
	headers := []string{"设备名称", "属性标识", "2025/12/26", "2025/12/27", "2025/12/29 11:11:00"}
	for i, header := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue(sheetName, cell, header)
	}

	// 添加示例行 - Device97	tag0001	29.0	29.0	29.0
	f.SetCellValue(sheetName, "A2", "Device97")
	f.SetCellValue(sheetName, "B2", "tag0001")
	f.SetCellValue(sheetName, "C2", 29.0)
	f.SetCellValue(sheetName, "D2", 29.0)
	f.SetCellValue(sheetName, "E2", 29.0)

	// 添加示例行 - Device95	tag0001	0.08	0.08	0.09
	f.SetCellValue(sheetName, "A3", "Device95")
	f.SetCellValue(sheetName, "B3", "tag0001")
	f.SetCellValue(sheetName, "C3", 0.08)
	f.SetCellValue(sheetName, "D3", 0.08)
	f.SetCellValue(sheetName, "E3", 0.09)

	// 添加示例行 - Device94	tag0001	21.0	21.0	22.0
	f.SetCellValue(sheetName, "A4", "Device94")
	f.SetCellValue(sheetName, "B4", "tag0001")
	f.SetCellValue(sheetName, "C4", 21.0)
	f.SetCellValue(sheetName, "D4", 21.0)
	f.SetCellValue(sheetName, "E4", 22.0)

	// 设置响应头
	c.Ctx.ResponseWriter.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Ctx.ResponseWriter.Header().Set("Content-Disposition", "attachment; filename=template.xlsx")

	// 写入Excel文件到响应
	err := f.Write(c.Ctx.ResponseWriter)
	if err != nil {
		c.Error(500, "生成模板文件失败: "+err.Error())
	}
}

// SinglePointInsert @Title 单点补录
// @Description 单点补录设备数据到TDengine数据库
// @Param   Authorization  header   string  true   "Bearer YourToken"
// @Param   deviceName     query    string  true   "设备名称"
// @Param   tagCode        query    string  true   "属性标识"
// @Param   timestamp      query    string  true   "时间戳"
// @Param   value          query    string  true   "数值"
// @Success 200 {object} map[string]interface{} "补录成功"
// @Failure 400 "错误信息"
// @router /uploadSingle [post]
func (c *ReportController) SinglePointInsert() {
	// 获取参数
	deviceName := c.GetString("deviceName")
	tagCode := c.GetString("tagCode")
	timestamp := c.GetString("timestamp")
	value := c.GetString("value")

	// 参数校验
	if deviceName == "" {
		c.Error(400, "设备名称不能为空")
	}
	if tagCode == "" {
		c.Error(400, "属性标识不能为空")
	}
	if timestamp == "" {
		c.Error(400, "时间戳不能为空")
	}
	if value == "" {
		c.Error(400, "数值不能为空")
	}

	// 创建报表服务
	reportService, err := services.NewReportService()
	if err != nil {
		c.Error(500, "创建报表服务失败: "+err.Error())
		return
	}

	// 调用服务进行单点补录
	err = reportService.SinglePointInsert(deviceName, tagCode, timestamp, value)
	if err != nil {
		c.Error(400, "单点补录失败: "+err.Error())
	}

	// 返回成功响应
	c.Success(map[string]interface{}{
		"message": "单点补录成功",
	})
}
