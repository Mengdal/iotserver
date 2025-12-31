package services

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/beego/beego/v2/client/orm"
	"github.com/beego/beego/v2/core/logs"
	"iotServer/models"
	"iotServer/utils"
	"strconv"
	"strings"
	"time"
)

// ReportService 报表服务
type ReportService struct {
	tdService *TDengineService
}

// NewReportService 创建报表服务实例
func NewReportService() (*ReportService, error) {
	tdService, err := NewTDengineService()
	if err != nil {
		return nil, err
	}

	return &ReportService{
		tdService: tdService,
	}, nil
}

// TimePeriodReport 时间段报表生成
func (r *ReportService) TimePeriodReport(page, size int, start, end string, tenantId int64, projectIds []int64, search string,
	productId int64, propertyIds string, resourceType string, resourceIds string, dateType string, multipleType bool, reportType string) (interface{}, error) {
	// 1. 根据统计类型获取设备
	var Ids []int64
	var err error
	if reportType != "report" {
		Ids, err = utils.GetResourceIds(resourceIds)
		if err != nil {
			return nil, fmt.Errorf("show devices error: %v", err)
		}
	}
	// 2. 根据分页查询设备
	devicePage, err := r.pageByProjectAndProduct(page, size, tenantId, projectIds, search, productId, Ids, resourceType)
	if err != nil {
		return nil, fmt.Errorf("查询设备失败，原因: %v", err)
	}
	deviceList := devicePage.List.(*[]*models.Device)

	// 3. 查询属性信息
	properties, err := r.listByProductAndIds(productId, propertyIds)
	if err != nil {
		return nil, fmt.Errorf("查询属性信息失败: %v", err)
	}

	// 4. 查询超级表标签
	o := orm.NewOrm()
	product := models.Product{Id: productId}
	if err = o.Read(&product); err != nil {
		return nil, fmt.Errorf("查询超级表失败: %v", err)
	}

	// 5. 查询时间段内首末值差值数据 || 查询时间段用量分析
	var result interface{}
	if reportType == "report" {

		fld, err := r.query(product.Key, dateType, start, end, deviceList, properties)
		if err != nil {
			return nil, fmt.Errorf("查询首末值数据失败: %v", err)
		}

		// 5. 处理表头
		labels, err := r.dealLabels(resourceType, properties)
		if err != nil {
			return nil, fmt.Errorf("处理表头失败: %v", err)
		}

		// 6. 装载数据
		array := r.buildReportData(deviceList, properties, fld, resourceType)

		// 7. 处理响应
		result = r.dealResponse(array, labels, devicePage)
	} else if reportType == "analysis" {
		result, err = r.UsageAnalysis(resourceType, product.Key, dateType, start, end, deviceList, properties)
		if err != nil {
			return nil, fmt.Errorf("查询用量数据失败: %v", err)
		}
	} else if reportType == "compareAnalysis" {
		result, err = r.CompareAnalysis(product.Key, dateType, start, end, deviceList, properties)
		if err != nil {
			return nil, fmt.Errorf("查询同比数据失败: %v", err)
		}
	}

	return result, nil
}

// query 查询时间段内首末值差值数据
func (r *ReportService) query(productKey, dateType, start, end string,
	deviceList *[]*models.Device, properties []models.Properties) ([]FirstLastDiff, error) {

	_, startTime, endTime, err := parseTimeRange(dateType, start, end)
	if err != nil {
		return nil, fmt.Errorf(err.Error())
	}

	var results []FirstLastDiff
	if len(*deviceList) == 0 {
		return results, nil
	}

	deviceConditions := ""
	for i, device := range *deviceList {
		if i > 0 {
			deviceConditions += ", "
		}
		deviceConditions += fmt.Sprintf("'%s'", device.Name)
	}

	// 为每个属性查询数据
	for _, property := range properties {
		// 构建IN查询条件

		query := fmt.Sprintf(`
           SELECT
               tbname,
               FIRST(`+"`%s`"+`) as first_value,
               LAST(`+"`%s`"+`) as last_value,
               (LAST(`+"`%s`"+`) - FIRST(`+"`%s`"+`)) as duration_value
           FROM %s.`+"`%s`"+`
           WHERE tbname IN (%s)
           AND ts >= %d AND ts <= %d
           GROUP BY tbname`,
			property.Code, property.Code, property.Code, property.Code,
			DBName, productKey, deviceConditions,
			startTime.UnixMilli(), endTime.UnixMilli())

		rows, err := r.tdService.db.Query(query)
		utils.DebugLog(query)
		if err != nil {
			logs.Warn("查询属性 %s 数据失败: %v", property.Code, err)
			continue
		}
		var typeSpec map[string]interface{}
		var step float64 = 1
		if err = json.Unmarshal([]byte(property.TypeSpec), &typeSpec); err != nil {
			logs.Warn("解析属性规格失败: %v", err)
		} else {
			// specs 是字符串类型，需要再次解析
			specsStr, ok := typeSpec["specs"].(string)
			if ok {
				var specs map[string]string
				if err = json.Unmarshal([]byte(specsStr), &specs); err == nil {
					step, err = strconv.ParseFloat(specs["step"], 64)
					if err != nil {
						logs.Warn("解析step失败: %v", err)
					}
				}
			}
		}

		for rows.Next() {
			var dn string
			var firstValue, lastValue, duration sql.NullFloat64

			if err := rows.Scan(&dn, &firstValue, &lastValue, &duration); err != nil {
				logs.Warn("扫描数据失败: %v", err)
				continue
			}

			fld := FirstLastDiff{
				Dn:  dn,
				Tag: property.Code,
			}

			// 只有当值有效时才设置指针
			if firstValue.Valid {
				formattedValue := formatFloat(firstValue.Float64, step)
				fld.First = &formattedValue
			}
			if lastValue.Valid {
				formattedValue := formatFloat(lastValue.Float64, step)
				fld.Last = &formattedValue
			}
			if duration.Valid {
				formattedValue := formatFloat(duration.Float64, step)
				fld.Duration = &formattedValue
			}

			results = append(results, fld)
		}
		rows.Close()
	}

	return results, nil
}

// UsageAnalysis 用量分析报表
func (r *ReportService) UsageAnalysis(resourceType, productKey, dateType, start, end string,
	deviceList *[]*models.Device, properties []models.Properties) (interface{}, error) {

	deviceConditions := ""
	deviceToLabel := make(map[string]string)  // 核心修改：设备名到组名的映射
	uniqueLabels := make(map[string]struct{}) // 记录有哪些唯一的组，用于后续遍历
	for i, device := range *deviceList {
		if i > 0 {
			deviceConditions += ", "
		}
		deviceConditions += fmt.Sprintf("'%s'", device.Name)
		if resourceType == "Raw" {
			continue
		} else if resourceType == "Group" {
			groupName := device.Group.Name
			deviceToLabel[device.Name] = groupName
			uniqueLabels[groupName] = struct{}{}
		} else if resourceType == "Position" {
			positionName := device.Position.Name
			deviceToLabel[device.Name] = positionName
			uniqueLabels[positionName] = struct{}{}
		}

	}

	_, startTime, endTime, err := parseTimeRange(dateType, start, end)
	if err != nil {
		return nil, err
	}

	// 查询用量数据
	detail := make(map[string][]map[string]interface{})
	summary := make(map[string]float64)

	if len(properties) > 1 {
		return nil, fmt.Errorf("统计出错")
	}
	property := properties[0]
	// 根据属性规格格式化数值
	var typeSpec map[string]interface{}
	var step float64 = 1
	if err = json.Unmarshal([]byte(property.TypeSpec), &typeSpec); err != nil {
		logs.Warn("解析属性规格失败: %v", err)
	} else {
		specsStr, ok := typeSpec["specs"].(string)
		if ok {
			var specs map[string]string
			if err = json.Unmarshal([]byte(specsStr), &specs); err == nil {
				step, err = strconv.ParseFloat(specs["step"], 64)
				if err != nil {
					logs.Warn("解析step失败: %v", err)
				}
			}
		}
	}

	// 使用TDengine的INTERVAL功能查询数据
	query := fmt.Sprintf(`
			SELECT
				tbname,
				_wstart,
				FIRST(`+"`%s`"+`) as first_value,
				LAST(`+"`%s`"+`) as last_value,
				(LAST(`+"`%s`"+`) - FIRST(`+"`%s`"+`)) as duration_value
			FROM %s.`+"`%s`"+`
			WHERE tbname IN (%s)
			AND ts >= %d
			AND ts <= %d
			PARTITION BY tbname
			INTERVAL(%s)
			ORDER BY tbname, _wstart`,
		property.Code, property.Code, property.Code, property.Code,
		DBName, productKey, deviceConditions,
		startTime.UnixNano()/1e6, endTime.UnixNano()/1e6, aggDateType(dateType))

	rows, err := r.tdService.db.Query(query)
	utils.DebugLog(query)
	if err != nil {
		logs.Warn("查询用量数据失败: %v", err)
		return nil, fmt.Errorf("查询用量数据失败: %v", err)
	}
	defer rows.Close()

	// 为每个设备初始化数据
	deviceResults := make(map[string]map[string]float64)
	if resourceType == "Raw" {
		for _, device := range *deviceList {
			deviceResults[device.Name] = make(map[string]float64)
			summary[device.Name] = 0.0
		}
	} else {
		for gName := range uniqueLabels {
			deviceResults[gName] = make(map[string]float64)
			summary[gName] = 0.0
		}
	}

	// 处理查询结果
	for rows.Next() {
		var dn string
		var wstart time.Time
		var firstValue, lastValue, Diff sql.NullFloat64

		if err := rows.Scan(&dn, &wstart, &firstValue, &lastValue, &Diff); err != nil {
			logs.Warn("扫描数据失败: %v", err)
			continue
		}
		wstart = wstart.Local()

		if Diff.Valid {
			formattedValue := formatFloat(Diff.Float64, step)
			periodKey := r.formatPeriodKey(dateType, wstart)

			if resourceType == "Raw" {
				deviceResults[dn][periodKey] = formattedValue
				summary[dn] = formatFloat(summary[dn]+formattedValue, step)
			} else {
				groupName, exists := deviceToLabel[dn]
				if !exists {
					continue // 理论上不应该发生，除非查询出的设备不在列表中
				}
				deviceResults[groupName][periodKey] += formattedValue
				summary[groupName] = formatFloat(summary[groupName]+formattedValue, step)
			}

		}
	}

	// 构建最终的detail数据结构
	periods := r.generatePeriods(dateType, startTime, endTime)
	if resourceType == "Raw" {
		for _, device := range *deviceList {
			deviceData := make([]map[string]interface{}, len(periods))
			for i, period := range periods {
				data := make(map[string]interface{})
				data["first"] = period
				if val, exists := deviceResults[device.Name][period]; exists {
					data["second"] = val
				} else {
					data["second"] = nil
				}
				deviceData[i] = data
			}
			detail[device.Name] = deviceData
		}
	} else {
		for groupName := range uniqueLabels {
			groupData := make([]map[string]interface{}, len(periods))

			for i, period := range periods {
				data := make(map[string]interface{})
				data["first"] = period

				// 检查该组在这个时间点是否有数据
				if val, exists := deviceResults[groupName][period]; exists {
					// 再次 format 确保累加后的精度正确 (可选)
					data["second"] = formatFloat(val, step)
				} else {
					data["second"] = nil
				}
				groupData[i] = data
			}
			detail[groupName] = groupData
		}
	}

	// 构造返回结果
	result := make(map[string]interface{})
	result["detail"] = detail
	result["summary"] = summary

	return result, nil
}

// CompareAnalysis 同比分析报表
func (r *ReportService) CompareAnalysis(productKey, dateType, start, end string,
	deviceList *[]*models.Device, properties []models.Properties) (interface{}, error) {

	deviceConditions := ""
	for i, device := range *deviceList {
		if i > 0 {
			deviceConditions += ", "
		}
		deviceConditions += fmt.Sprintf("'%s'", device.Name)
	}

	baseTime, startTime, endTime, err := parseTimeRange(dateType, start, start)
	if err != nil {
		return nil, err
	}

	if len(properties) > 1 {
		return nil, fmt.Errorf("统计出错")
	}
	property := properties[0]

	// 根据属性规格格式化数值
	var typeSpec map[string]interface{}
	var step float64 = 1
	if err = json.Unmarshal([]byte(property.TypeSpec), &typeSpec); err != nil {
		logs.Warn("解析属性规格失败: %v", err)
	} else {
		specsStr, ok := typeSpec["specs"].(string)
		if ok {
			var specs map[string]string
			if err = json.Unmarshal([]byte(specsStr), &specs); err == nil {
				step, err = strconv.ParseFloat(specs["step"], 64)
				if err != nil {
					logs.Warn("解析step失败: %v", err)
				}
			}
		}
	}

	// 查询用量数据
	summary := make(map[string]float64)
	changePercent := make(map[string]float64)

	// 初始化所有设备的数据
	deviceResults := make(map[string]map[string]float64)
	lastDeviceResults := make(map[string]map[string]float64)

	for _, device := range *deviceList {
		deviceResults[device.Name] = make(map[string]float64)
		lastDeviceResults[device.Name] = make(map[string]float64)
		summary[device.Name] = 0.0
		changePercent[device.Name] = 0.0
	}

	// 查询当前年份数据
	currentQuery := fmt.Sprintf(`
		SELECT
			tbname,
			_wstart,
			FIRST(`+"`%s`"+`) as first_value,
			LAST(`+"`%s`"+`) as last_value,
			(LAST(`+"`%s`"+`) - FIRST(`+"`%s`"+`)) as duration_value
		FROM %s.`+"`%s`"+`
		WHERE tbname IN (%s)
		AND ts >= %d
		AND ts <= %d
		PARTITION BY tbname
		INTERVAL(%s)
		ORDER BY tbname, _wstart`,
		property.Code, property.Code, property.Code, property.Code,
		DBName, productKey, deviceConditions,
		startTime.UnixNano()/1e6, endTime.UnixNano()/1e6, aggDateType(dateType))

	currentRows, err := r.tdService.db.Query(currentQuery)
	if err != nil {
		logs.Warn("查询当前用量数据失败: %v", err)
		return nil, fmt.Errorf("查询当前用量数据失败: %v", err)
	}

	// 处理当前查询结果
	for currentRows.Next() {
		var dn string
		var wstart time.Time
		var firstValue, lastValue, Diff sql.NullFloat64

		if err := currentRows.Scan(&dn, &wstart, &firstValue, &lastValue, &Diff); err != nil {
			logs.Warn("扫描当前数据失败: %v", err)
			continue
		}
		wstart = wstart.Local()

		if Diff.Valid {
			formattedValue := formatFloat(Diff.Float64, step)
			periodKey := r.formatPeriodKey(dateType, wstart)
			deviceResults[dn][periodKey] = formattedValue
			summary[dn] = formatFloat(summary[dn]+formattedValue, step)
		}
	}
	currentRows.Close()

	baseTimeTo, startTimeTo, endTimeTo, err := parseTimeRange(dateType, end, end)
	if err != nil {
		return nil, err
	}

	// 查询上次同期数据
	lastQuery := fmt.Sprintf(`
		SELECT
			tbname,
			_wstart,
			FIRST(`+"`%s`"+`) as first_value,
			LAST(`+"`%s`"+`) as last_value,
			(LAST(`+"`%s`"+`) - FIRST(`+"`%s`"+`)) as duration_value
		FROM %s.`+"`%s`"+`
		WHERE tbname IN (%s)
		AND ts >= %d
		AND ts <= %d
		PARTITION BY tbname
		INTERVAL(%s)
		ORDER BY tbname, _wstart`,
		property.Code, property.Code, property.Code, property.Code,
		DBName, productKey, deviceConditions,
		startTimeTo.UnixNano()/1e6, endTimeTo.UnixNano()/1e6, aggDateType(dateType))

	lastRows, err := r.tdService.db.Query(lastQuery)
	if err != nil {
		logs.Warn("查询去年同期用量数据失败: %v", err)
		return nil, fmt.Errorf("查询去年同期用量数据失败: %v", err)
	}

	// 处理上次同期查询结果
	for lastRows.Next() {
		var dn string
		var wstart time.Time
		var firstValue, lastValue, Diff sql.NullFloat64

		if err := lastRows.Scan(&dn, &wstart, &firstValue, &lastValue, &Diff); err != nil {
			logs.Warn("扫描去年同期数据失败: %v", err)
			continue
		}
		wstart = wstart.Local()

		if Diff.Valid {
			formattedValue := formatFloat(Diff.Float64, step)
			periodKey := r.formatPeriodKey(dateType, wstart)
			lastDeviceResults[dn][periodKey] = formattedValue
			changePercent[dn] = formatFloat(changePercent[dn]+formattedValue, step)
		}
	}
	lastRows.Close()

	// 构建最终的detail数据结构
	periods := r.generatePeriods(dateType, startTime, endTime)

	// 当前数据
	currentDetail := make([]map[string]interface{}, len(periods))
	for i, period := range periods {
		data := make(map[string]interface{})
		data["first"] = period
		// 这里简化处理，实际应该计算所有设备的总和
		totalValue := 0.0
		validCount := 0
		for _, device := range *deviceList {
			if val, exists := deviceResults[device.Name][period]; exists {
				totalValue += val
				validCount++
			}
		}
		if validCount > 0 {
			data["second"] = formatFloat(totalValue, step)
		} else {
			data["second"] = nil
		}
		currentDetail[i] = data
	}

	// 构建对比时间段的数据
	compareDetail := make([]map[string]interface{}, len(periods))
	for i, period := range periods {
		data := make(map[string]interface{})
		data["first"] = period
		totalValue := 0.0
		validCount := 0
		for _, device := range *deviceList {
			if val, exists := lastDeviceResults[device.Name][period]; exists {
				totalValue += val
				validCount++
			}
		}
		if validCount > 0 {
			data["second"] = formatFloat(totalValue, step)
		} else {
			data["second"] = nil
		}
		compareDetail[i] = data
	}

	// 同比变化率数据
	changePercentDetail := make([]map[string]interface{}, len(periods))
	for i, period := range periods {
		data := make(map[string]interface{})
		data["first"] = period

		currentTotal := 0.0
		lastTotal := 0.0
		currentCount := 0
		lastCount := 0

		for _, device := range *deviceList {
			if val, exists := deviceResults[device.Name][period]; exists {
				currentTotal += val
				currentCount++
			}
			if val, exists := lastDeviceResults[device.Name][period]; exists {
				lastTotal += val
				lastCount++
			}
		}

		if currentCount > 0 && lastCount > 0 && lastTotal != 0 {
			percent := ((currentTotal - lastTotal) / lastTotal) * 100
			data["second"] = formatFloat(percent, 0.01) // 百分比保留2位小数
		} else if currentCount > 0 && lastCount == 0 {
			data["second"] = nil // 去年无数据，无法计算同比
		} else {
			data["second"] = nil
		}
		changePercentDetail[i] = data
	}

	// 构建返回的detail结构
	resultDetail := make(map[string]interface{})
	resultDetail[baseTime] = currentDetail
	resultDetail[baseTimeTo] = compareDetail
	resultDetail["changePercent"] = changePercentDetail

	// 计算总同比变化率
	totalCurrent := 0.0
	totalLastYear := 0.0
	for _, sum := range summary {
		totalCurrent += sum
	}
	for _, sum := range changePercent {
		totalLastYear += sum
	}

	resultSummary := make(map[string]interface{})
	resultSummary[baseTime] = formatFloat(totalCurrent, step)
	resultSummary[baseTimeTo] = formatFloat(totalLastYear, step)
	if totalLastYear != 0 {
		percent := ((totalCurrent - totalLastYear) / totalLastYear) * 100
		resultSummary["changePercent"] = fmt.Sprintf("%.2f%%", formatFloat(percent, 0.01))
	} else {
		resultSummary["changePercent"] = "0.00%"
	}

	// 构造返回结果
	result := make(map[string]interface{})
	result["detail"] = resultDetail
	result["summary"] = resultSummary

	return result, nil
}

// formatPeriodKey 格式化周期键
func (r *ReportService) formatPeriodKey(dateType string, t time.Time) string {
	switch dateType {
	case "day":
		return fmt.Sprintf("%02d", t.Hour())
	case "week":
		return fmt.Sprintf("%01d", t.Weekday()+1)
	case "month":
		return fmt.Sprintf("%02d", t.Day())
	case "year":
		return fmt.Sprintf("%02d", int(t.Month()))
	default:
		return fmt.Sprintf("%02d", int(t.Month()))
	}
}

// generatePeriods 生成周期列表
func (r *ReportService) generatePeriods(dateType string, start, end time.Time) []string {
	var periods []string
	switch dateType {
	case "day":
		for i := 0; i < 24; i++ {
			periods = append(periods, fmt.Sprintf("%02d", i))
		}
	case "week": // 添加周报表的周期生成
		periods = append(periods, "1", "2", "3", "4", "5", "6", "7")
	case "month":
		days := end.Day()
		for i := 1; i <= days; i++ {
			periods = append(periods, fmt.Sprintf("%02d", i))
		}
	case "year":
		for i := 1; i <= 12; i++ {
			periods = append(periods, fmt.Sprintf("%02d", i))
		}
	default:
		for i := 1; i <= 12; i++ {
			periods = append(periods, fmt.Sprintf("%02d", i))
		}
	}
	return periods
}

// pageByProjectAndCategory 分页查询设备
func (r *ReportService) pageByProjectAndProduct(page, size int, tenantId int64, projectIds []int64, search string, productId int64, ids []int64, resourceType string) (*utils.PageResult, error) {
	// 这里需要根据您的实际数据库实现查询逻辑
	var devices []*models.Device
	o := orm.NewOrm()
	// 构建查询条件
	query := o.QueryTable(new(models.Device))

	if resourceType == "Position" {
		query = query.RelatedSel("Position").OrderBy("-position_id")
		if ids != nil {
			query = query.Filter("position_id__in", ids)
		}
	}

	if resourceType == "Group" {
		query = query.RelatedSel("Group").OrderBy("-group_id")
		if ids != nil {
			query = query.Filter("group_id__in", ids)
		}
	}

	if resourceType == "Raw" {
		query = query.Filter("id__in", ids)
	}

	if tenantId != 0 {
		query = query.Filter("tenant_id", tenantId)
	}

	if len(projectIds) > 0 {
		query = query.Filter("department_id__in", projectIds)
	}

	// 如果指定了分类ID，则添加分类筛选条件
	if productId > 0 {
		query = query.Filter("product_id", productId)
	}

	// 如果有搜索关键字，则添加搜索条件
	if search != "" {
		// 支持按设备名称或描述搜索
		cond := orm.NewCondition()
		query = query.SetCond(cond.Or("name__icontains", search).Or("description__icontains", search))
	}

	paginate, err := utils.Paginate(query, page, size, &devices)

	if len(devices) == 0 {
		return nil, fmt.Errorf("暂无数据")
	}
	return paginate, err
}

// listByCategoryAndIds 根据分类和ID列表查询属性
func (r *ReportService) listByProductAndIds(productId int64, propertyIds string) ([]models.Properties, error) {
	o := orm.NewOrm()
	var properties []models.Properties

	// 构建基础查询
	query := o.QueryTable(new(models.Properties)).Filter("product_id", productId)

	// 如果提供了属性ID列表，则添加ID筛选条件
	if propertyIds != "" {
		// 解析属性ID列表
		idList := strings.Split(propertyIds, ",")
		var ids []int64
		for _, idStr := range idList {
			if id, err := strconv.ParseInt(strings.TrimSpace(idStr), 10, 64); err == nil {
				ids = append(ids, id)
			}
		}

		// 如果解析出有效的ID，则添加到查询条件中
		if len(ids) > 0 {
			query = query.Filter("id__in", ids)
		}
	}

	// 执行查询
	_, err := query.All(&properties)
	if err != nil {
		return nil, fmt.Errorf("查询属性列表失败: %v", err)
	}

	return properties, nil
}

type FirstLastDiff struct {
	Dn       string   `json:"dn"`
	Tag      string   `json:"tag"`
	First    *float64 `json:"first"`
	Last     *float64 `json:"last"`
	Duration *float64 `json:"duration"`
}

func formatFloat(val float64, step float64) float64 {
	// 计算需要保留的小数位数
	precision := 0
	stepStr := strconv.FormatFloat(step, 'f', -1, 64)
	if dotIndex := strings.Index(stepStr, "."); dotIndex != -1 {
		precision = len(stepStr) - dotIndex - 1
	}

	// 格式化数值
	str := fmt.Sprintf("%.*f", precision, val)
	result, _ := strconv.ParseFloat(str, 64)
	return result
}
func aggDateType(val string) (aggType string) {
	switch val {
	case "interval":
		aggType = ""
	case "day":
		aggType = "1h"
	case "month", "week":
		aggType = "1d"
	case "year":
		aggType = "1n"
	default:
		aggType = ""

	}
	return aggType
}

// parseTimeRange 根据dateType解析时间范围
func parseTimeRange(dateType, start, end string) (baseTimeStr string, startTime, endTime time.Time, err error) {
	loc := time.Local
	switch dateType {
	case "interval": // 时段报表，使用输入的开始结束时间
		baseTimeStr = start
		startTime, err = time.ParseInLocation("2006-01-02 15:04:05", baseTimeStr, loc)
		if err != nil {
			err = fmt.Errorf("开始时间格式错误: %v", err)
			return
		}
		endTime, err = time.ParseInLocation("2006-01-02 15:04:05", end, loc)
		if err != nil {
			err = fmt.Errorf("结束时间格式错误: %v", err)
			return
		}
	case "day": // 日报表，使用 start 日期的 00:00:00 到 23:59:59
		baseTimeStr = start[:10]
		var baseTime time.Time
		baseTime, err = time.ParseInLocation("2006-01-02", baseTimeStr, loc) // 只取日期部分
		if err != nil {
			err = fmt.Errorf("日期格式错误: %v", err)
			return
		}
		startTime = time.Date(baseTime.Year(), baseTime.Month(), baseTime.Day(), 0, 0, 0, 0, loc)
		endTime = time.Date(baseTime.Year(), baseTime.Month(), baseTime.Day(), 23, 59, 59, 0, loc)
	case "week": // 周报表，使用 start 日期所在周的周一 00:00:00 到周日 23:59:59
		baseTimeStr = start[:10]
		var baseTime time.Time
		baseTime, err = time.ParseInLocation("2006-01-02", baseTimeStr, loc) // 只取日期部分
		if err != nil {
			err = fmt.Errorf("日期格式错误: %v", err)
			return
		}
		// 计算该日期所在周的周一
		weekday := int(baseTime.Weekday())
		if weekday == 0 { // 如果是周日，则周一为6天前
			startTime = time.Date(baseTime.Year(), baseTime.Month(), baseTime.Day()-6, 0, 0, 0, 0, loc)
		} else { // 如果是周一到周六，则周一为weekday-1天前
			startTime = time.Date(baseTime.Year(), baseTime.Month(), baseTime.Day()-(weekday-1), 0, 0, 0, 0, loc)
		}
		// 计算该周的周日
		endTime = startTime.AddDate(0, 0, 6) // 周一加6天为周日
		endTime = time.Date(endTime.Year(), endTime.Month(), endTime.Day(), 23, 59, 59, 0, loc)
	case "month": // 月报表，使用 start 月份的第一天到最后一天
		baseTimeStr = start[:7]
		var baseTime time.Time
		baseTime, err = time.ParseInLocation("2006-01", baseTimeStr, loc) // 取年月部分
		if err != nil {
			err = fmt.Errorf("月份格式错误: %v", err)
			return
		}
		startTime = time.Date(baseTime.Year(), baseTime.Month(), 1, 0, 0, 0, 0, loc)
		// 计算月末时间
		nextMonth := startTime.AddDate(0, 1, 0)
		endTime = nextMonth.Add(-time.Second)
	case "year": // 年报表，使用 start 年份的第一天到最后一天
		baseTimeStr = start[:4]
		var baseTime time.Time
		baseTime, err = time.ParseInLocation("2006", baseTimeStr, loc) // 取年份部分
		if err != nil {
			err = fmt.Errorf("年份格式错误: %v", err)
			return
		}
		startTime = time.Date(baseTime.Year(), 1, 1, 0, 0, 0, 0, loc)
		endTime = time.Date(baseTime.Year(), 12, 31, 23, 59, 59, 0, loc)
	default:
		err = fmt.Errorf("不支持的报表类型: %s", dateType)
	}

	return
}

// multiple 计算倍率
func (r *ReportService) multiple(multipleType bool, ratio, pt, ct float64) float64 {
	if multipleType {
		return ratio * pt * ct
	}
	return 1.0
}

// dealLabels 处理表头
func (r *ReportService) dealLabels(labelType string, properties []models.Properties) ([]map[string]string, error) {
	labels := make([]map[string]string, 0)

	if labelType == "Position" {
		labels = append(labels, map[string]string{"prop": "position", "label": "位置"})
	} else if labelType == "Group" {
		labels = append(labels, map[string]string{"prop": "group", "label": "标签"})
	}
	labels = append(labels, map[string]string{"prop": "dn", "label": "设备"})
	if len(properties) > 1 {
		// 多属性表头
		for _, property := range properties {
			label := map[string]string{
				"prop":  property.Code,
				"label": property.Name,
			}
			labels = append(labels, label)
		}
	} else if len(properties) == 1 {
		// 单属性表头
		labels = append(labels, map[string]string{"prop": "tag", "label": "属性名称"})
		labels = append(labels, map[string]string{"prop": "first", "label": "开始读数"})
		labels = append(labels, map[string]string{"prop": "last", "label": "结束读数"})
		labels = append(labels, map[string]string{"prop": "duration", "label": "用量"})
	}

	return labels, nil
}

// dealResponse 处理响应数据
func (r *ReportService) dealResponse(array []map[string]interface{},
	labels []map[string]string, devicePage *utils.PageResult) map[string]interface{} {

	response := make(map[string]interface{})
	response["tableData"] = array
	response["tableLabel"] = labels
	response["pageNum"] = devicePage.PageNumber
	response["pageSize"] = devicePage.PageSize
	response["totalRow"] = devicePage.TotalRow
	response["totalPage"] = devicePage.TotalPage

	return response
}

// buildReportData 构建报表数据，包含分组小计和总计
func (r *ReportService) buildReportData(deviceList *[]*models.Device, properties []models.Properties,
	fld []FirstLastDiff, resourceType string) []map[string]interface{} {

	array := make([]map[string]interface{}, 0)

	// 如果是按位置或标签统计，需要分组处理
	if resourceType == "Position" || resourceType == "Group" {
		// 按分组字段组织数据
		groupedData := make(map[string][]*models.Device)
		groupTotals := make(map[string]float64)
		var grandTotal float64

		// 分组设备
		for _, device := range *deviceList {
			var groupKey string
			if resourceType == "Position" {
				// 检查 Position 是否为空
				if device.Position != nil {
					groupKey = device.Position.Name
				} else {
					groupKey = "未分组" // 默认分组名
				}
			} else { // Group
				// 检查 Group 是否为空
				if device.Group != nil {
					groupKey = device.Group.Name
				} else {
					groupKey = "未分组" // 默认分组名
				}
			}

			groupedData[groupKey] = append(groupedData[groupKey], device)
		}

		// 处理每个分组
		for groupKey, devices := range groupedData {
			// 添加分组内的设备数据
			for _, device := range devices {
				obj := r.buildDeviceData(device, properties, fld, resourceType, groupKey)

				// 累计分组小计和总计
				if len(properties) == 1 && len(*deviceList) > 0 {
					property := properties[0]
					var matchedFld *FirstLastDiff
					for _, diff := range fld {
						if diff.Dn == device.Name && diff.Tag == property.Code {
							matchedFld = &diff
							break
						}
					}

					if matchedFld != nil && matchedFld.Duration != nil {
						groupTotals[groupKey] += *matchedFld.Duration
						grandTotal += *matchedFld.Duration
					}
				}

				array = append(array, obj)
			}

			// 添加分组小计行
			subtotal := make(map[string]interface{})
			if resourceType == "Position" {
				subtotal["position"] = groupKey
			} else {
				subtotal["group"] = groupKey
			}
			subtotal["dn"] = "小计"
			subtotal["dn_en"] = "subTotal"
			if val, exists := groupTotals[groupKey]; exists {
				subtotal["duration"] = formatFloat(val, 0.01)
			}
			array = append(array, subtotal)
		}

		// 添加总计行
		totalRow := make(map[string]interface{})
		if resourceType == "Position" {
			totalRow["position"] = "合计"
			totalRow["position_en"] = "total"
		} else {
			totalRow["group"] = "合计"
			totalRow["group_en"] = "total"
		}
		totalRow["duration"] = formatFloat(grandTotal, 0.01)
		array = append(array, totalRow)
	} else {
		// 原有逻辑：不按分组统计
		for _, device := range *deviceList {
			obj := r.buildDeviceData(device, properties, fld, resourceType, "")
			array = append(array, obj)
		}
	}

	return array
}

// buildDeviceData 构建设备数据
func (r *ReportService) buildDeviceData(device *models.Device, properties []models.Properties,
	fld []FirstLastDiff, resourceType string, groupKey string) map[string]interface{} {

	obj := make(map[string]interface{})

	// 设置分组字段
	if resourceType == "Position" {
		obj["position"] = groupKey
	} else if resourceType == "Group" {
		obj["group"] = groupKey
	}

	if device.Description != "" {
		obj["dn"] = device.Name + "(" + device.Description + ")"
	} else {
		obj["dn"] = device.Name
	}

	if len(properties) > 1 {
		for _, property := range properties {
			var matchedFld *FirstLastDiff
			for _, diff := range fld {
				if diff.Dn == device.Name && diff.Tag == property.Code {
					matchedFld = &diff
					break
				}
			}

			if matchedFld != nil && matchedFld.Duration != nil {
				obj[property.Code] = *matchedFld.Duration
			} else {
				obj[property.Code] = nil
			}
		}
	} else if len(properties) == 1 {
		property := properties[0]
		obj["tag"] = property.Name
		obj["tag_en"] = property.Code

		var matchedFld *FirstLastDiff
		for _, diff := range fld {
			if diff.Dn == device.Name && diff.Tag == property.Code {
				matchedFld = &diff
				break
			}
		}

		if matchedFld != nil {
			if matchedFld.First != nil {
				obj["first"] = *matchedFld.First
			}
			if matchedFld.Last != nil {
				obj["last"] = *matchedFld.Last
			}
			if matchedFld.Duration != nil {
				obj["duration"] = *matchedFld.Duration
			}
		}
	}

	return obj
}

// SinglePointInsert 单点补录数据到TDengine
func (r *ReportService) SinglePointInsert(deviceName, tagCode, timestamp, value string) error {
	// 验证设备是否存在
	o := orm.NewOrm()
	device := models.Device{Name: deviceName}
	err := o.Read(&device, "Name")
	if err != nil {
		return fmt.Errorf("设备 %s 不存在", deviceName)
	}

	// 解析时间戳
	parsedTime, err := parseExcelTime(timestamp)
	if err != nil {
		return fmt.Errorf("时间戳格式错误: %v", err)
	}

	// 构建批量插入请求
	requests := []BatchInsertDataRequest{
		{
			DeviceName:   deviceName,
			PropertyCode: tagCode,
			DataPoints: []DataPointRequest{
				{
					Timestamp: parsedTime,
					Value:     value,
				},
			},
		},
	}

	// 使用现有的批量插入方法
	return r.BatchInsertData(requests)
}
