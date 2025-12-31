package services

import (
	"fmt"
	"github.com/beego/beego/v2/core/logs"
	"github.com/xuri/excelize/v2"
	"iotServer/models"
	"iotServer/utils"
	"strconv"
	"strings"
	"time"
)

// HistoryDataPoint 历史数据点
type HistoryDataPoint struct {
	Status    string `json:"status"`
	Val       string `json:"val"`
	Timestamp int64  `json:"timestamp"`
}

// HistoryQueryResponse 历史数据查询响应
type HistoryQueryResponse map[string][]HistoryDataPoint

// PropertyWithRealTime 带实时值的属性结构
type PropertyWithRealTime struct {
	Id          int64  `json:"id"`
	AccessMode  string `json:"access_mode"`
	Name        string `json:"name"`
	Code        string `json:"code"`
	Description string `json:"description"`
	Require     bool   `json:"require"`
	TypeSpec    string `json:"type_spec"`
	Tag         string `json:"tag"`
	System      bool   `json:"system"`
	Created     int64  `json:"created"`
	Updated     int64  `json:"updated"`
	Type        string `json:"type"`
	Val         string `json:"val"`
	Status      string `json:"status"`
	Timestamp   int64  `json:"timestamp"`
}

// HistoryQuery 普通历史数据查询
func (r *ReportService) HistoryQuery(req models.HistoryObject) (HistoryQueryResponse, error) {
	// 1. 参数验证
	if len(req.IDs) == 0 {
		return nil, fmt.Errorf("IDs不能为空")
	}
	if req.StartTime == "" || req.EndTime == "" {
		return nil, fmt.Errorf("开始时间和结束时间不能为空")
	}

	// 2. 解析时间范围
	loc := time.Local
	startTime, err := time.ParseInLocation("2006-01-02 15:04:05", req.StartTime, loc)
	if err != nil {
		return nil, fmt.Errorf("开始时间格式错误: %v", err)
	}
	endTime, err := time.ParseInLocation("2006-01-02 15:04:05", req.EndTime, loc)
	if err != nil {
		return nil, fmt.Errorf("结束时间格式错误: %v", err)
	}

	// 3. 解析IDs并按设备分组
	deviceGroups, err := r.parseAndGroupIDs(req.IDs)
	if err != nil {
		return nil, err
	}

	// 4. 设置查询条数
	limit := req.Count
	if limit <= 0 {
		limit = 100
	}
	if limit > 10000 {
		limit = 10000
	}

	// 5. 查询数据
	response := make(HistoryQueryResponse)

	for productKey, deviceProps := range deviceGroups {
		for deviceName, properties := range deviceProps {
			// 构建查询字段
			fields := []string{"`ts`"}
			for _, prop := range properties {
				fields = append(fields, fmt.Sprintf("`%s`", prop))
			}

			// 构建查询SQL
			query := fmt.Sprintf(`
				SELECT %s
				FROM %s.`+"`%s`"+`
				WHERE tbname = '%s'
				AND ts >= %d
				AND ts <= %d
				ORDER BY ts DESC
				LIMIT %d`,
				strings.Join(fields, ", "),
				DBName, productKey,
				deviceName,
				startTime.UnixMilli(),
				endTime.UnixMilli(),
				limit)

			utils.DebugLog(query)

			rows, err := r.tdService.db.Query(query)
			if err != nil {
				logs.Warn("查询设备 %s 数据失败: %v", deviceName, err)
				continue
			}

			// 解析查询结果
			for rows.Next() {
				var ts time.Time
				values := make([]interface{}, len(properties))
				scanArgs := make([]interface{}, len(fields))
				scanArgs[0] = &ts
				for i := range properties {
					scanArgs[i+1] = &values[i]
				}

				if err := rows.Scan(scanArgs...); err != nil {
					logs.Warn("扫描数据失败: %v", err)
					continue
				}

				// 为每个属性创建数据点
				for i, prop := range properties {
					key := fmt.Sprintf("%s.%s", deviceName, prop)

					dataPoint := HistoryDataPoint{
						Status:    "Good",
						Timestamp: ts.UnixMilli(),
					}

					// 处理值
					if values[i] == nil {
						dataPoint.Val = ""
						dataPoint.Status = "Bad"
					} else {
						dataPoint.Val = fmt.Sprintf("%v", values[i])
					}

					response[key] = append(response[key], dataPoint)
				}
			}
			rows.Close()
		}
	}

	// 6. 为没有查询到数据的ID添加空数组
	for _, id := range req.IDs {
		if _, exists := response[id]; !exists {
			response[id] = []HistoryDataPoint{}
		}
	}

	return response, nil
}

// AggregateQuery 聚合历史数据查询
func (r *ReportService) AggregateQuery(req models.AggObject) (HistoryQueryResponse, error) {
	// 1. 参数验证
	if len(req.IDs) == 0 {
		return nil, fmt.Errorf("IDs不能为空")
	}
	if req.StartTime == "" || req.EndTime == "" {
		return nil, fmt.Errorf("开始时间和结束时间不能为空")
	}
	if req.AggType == "" {
		return nil, fmt.Errorf("聚合类型不能为空")
	}

	// 2. 解析时间范围
	loc := time.Local
	startTime, err := time.ParseInLocation("2006-01-02 15:04:05", req.StartTime, loc)
	if err != nil {
		return nil, fmt.Errorf("开始时间格式错误: %v", err)
	}
	endTime, err := time.ParseInLocation("2006-01-02 15:04:05", req.EndTime, loc)
	if err != nil {
		return nil, fmt.Errorf("结束时间格式错误: %v", err)
	}

	// 3. 解析IDs并按设备分组
	deviceGroups, err := r.parseAndGroupIDs(req.IDs)
	if err != nil {
		return nil, err
	}

	// 4. 获取聚合函数
	aggFunc, err := r.getAggregateFunction(req.AggType)
	if err != nil {
		return nil, err
	}

	// 5. 计算聚合周期
	var interval string
	if req.Period > 0 {
		interval = fmt.Sprintf("%ds", req.Period)
	}

	// 6. 查询数据
	response := make(HistoryQueryResponse)
	isMedian := strings.ToLower(req.AggType) == "median"

	for productKey, deviceProps := range deviceGroups {
		for deviceName, properties := range deviceProps {
			// 构建查询字段
			fields := []string{}
			if interval != "" {
				fields = append(fields, "_wstart as ts")
			}

			for _, prop := range properties {
				if isMedian {
					fields = append(fields, fmt.Sprintf("APERCENTILE(`%s`, 50) as `%s`", prop, prop))
				} else {
					fields = append(fields, fmt.Sprintf("%s(`%s`) as `%s`", aggFunc, prop, prop))
				}
			}

			// 构建查询SQL
			var query string
			orderBy := "ASC"
			if req.Desc {
				orderBy = "DESC"
			}

			if interval != "" {
				query = fmt.Sprintf(`
					SELECT %s
					FROM %s.`+"`%s`"+`
					WHERE tbname = '%s'
					AND ts >= %d
					AND ts <= %d
					PARTITION BY tbname
					INTERVAL(%s)
					ORDER BY ts %s`,
					strings.Join(fields, ", "),
					DBName, productKey,
					deviceName,
					startTime.UnixMilli(),
					endTime.UnixMilli(),
					interval,
					orderBy)
			} else {
				query = fmt.Sprintf(`
					SELECT %s
					FROM %s.`+"`%s`"+`
					WHERE tbname = '%s'
					AND ts >= %d
					AND ts <= %d`,
					strings.Join(fields, ", "),
					DBName, productKey,
					deviceName,
					startTime.UnixMilli(),
					endTime.UnixMilli())
			}

			utils.DebugLog(query)

			rows, err := r.tdService.db.Query(query)
			if err != nil {
				logs.Warn("查询设备 %s 聚合数据失败: %v", deviceName, err)
				continue
			}

			// 解析查询结果
			for rows.Next() {
				var ts *time.Time
				values := make([]interface{}, len(properties))

				var scanArgs []interface{}
				if interval != "" {
					scanArgs = make([]interface{}, len(properties)+1)
					scanArgs[0] = &ts
					for i := range properties {
						scanArgs[i+1] = &values[i]
					}
				} else {
					scanArgs = make([]interface{}, len(properties))
					for i := range properties {
						scanArgs[i] = &values[i]
					}
				}

				if err := rows.Scan(scanArgs...); err != nil {
					logs.Warn("扫描聚合数据失败: %v", err)
					continue
				}

				// 获取时间戳
				var timestamp int64
				if ts != nil {
					timestamp = ts.UnixMilli()
				} else {
					timestamp = endTime.UnixMilli()
				}

				// 为每个属性创建数据点
				for i, prop := range properties {
					key := fmt.Sprintf("%s.%s", deviceName, prop)

					dataPoint := HistoryDataPoint{
						Status:    "Good",
						Timestamp: timestamp,
					}

					if values[i] == nil {
						dataPoint.Val = ""
						dataPoint.Status = "Bad"
					} else {
						dataPoint.Val = fmt.Sprintf("%v", values[i])
					}

					response[key] = append(response[key], dataPoint)
				}
			}
			rows.Close()
		}
	}

	// 7. 为没有查询到数据的ID添加空数组
	for _, id := range req.IDs {
		if _, exists := response[id]; !exists {
			response[id] = []HistoryDataPoint{}
		}
	}

	return response, nil
}

// parseAndGroupIDs 解析IDs并按设备分组
func (r *ReportService) parseAndGroupIDs(ids []string) (map[string]map[string][]string, error) {
	// productKey -> deviceName -> []propertyCode
	deviceGroups := make(map[string]map[string][]string)

	for _, id := range ids {
		parts := strings.Split(id, ".")
		if len(parts) != 2 {
			return nil, fmt.Errorf("ID格式错误: %s, 应为 设备名称.属性代码", id)
		}

		deviceName := parts[0]
		propertyCode := parts[1]

		// 查询设备的ProductKey
		productKey, ok := GetDeviceCategoryKeyFromCache(deviceName)
		if !ok {
			return nil, fmt.Errorf("设备 %s 不存在或未找到对应的产品", deviceName)
		}

		if deviceGroups[productKey] == nil {
			deviceGroups[productKey] = make(map[string][]string)
		}
		if deviceGroups[productKey][deviceName] == nil {
			deviceGroups[productKey][deviceName] = []string{}
		}

		// 检查是否已存在该属性，避免重复
		exists := false
		for _, prop := range deviceGroups[productKey][deviceName] {
			if prop == propertyCode {
				exists = true
				break
			}
		}
		if !exists {
			deviceGroups[productKey][deviceName] = append(deviceGroups[productKey][deviceName], propertyCode)
		}
	}

	return deviceGroups, nil
}

// getAggregateFunction 获取聚合函数名称
func (r *ReportService) getAggregateFunction(aggType string) (string, error) {
	switch strings.ToLower(aggType) {
	case "first":
		return "FIRST", nil
	case "last":
		return "LAST", nil
	case "min":
		return "MIN", nil
	case "max":
		return "MAX", nil
	case "sum":
		return "SUM", nil
	case "avg", "average":
		return "AVG", nil
	case "median":
		return "APERCENTILE", nil
	default:
		return "", fmt.Errorf("不支持的聚合类型: %s", aggType)
	}
}

func (r *ReportService) GetRealData(deviceName string, properties []*models.Properties) ([]*PropertyWithRealTime, error) {
	if len(properties) == 0 {
		return nil, fmt.Errorf("属性列表不能为空")
	}

	// 构建查询IDs列表
	var ids []string
	for _, prop := range properties {
		id := fmt.Sprintf("%s.%s", deviceName, prop.Code)
		ids = append(ids, id)
	}

	// 创建历史查询请求（查询最近的数据）
	historyReq := models.HistoryObject{
		IDs:       ids,
		StartTime: time.Now().Add(-24 * time.Hour * 7).Format("2006-01-02 15:04:05"), // 最近24小时
		EndTime:   time.Now().Format("2006-01-02 15:04:05"),
		Count:     1, // 只查询最新的一条
	}

	// 调用历史查询获取实时值
	result, err := r.HistoryQuery(historyReq)
	if err != nil {
		return nil, fmt.Errorf("查询实时值失败: %v", err)
	}

	// 构建带实时值的属性列表
	propertiesWithValue := make([]*PropertyWithRealTime, 0, len(properties))
	for _, prop := range properties {
		propWithValue := &PropertyWithRealTime{
			Id:          prop.Id,
			AccessMode:  prop.AccessMode,
			Name:        prop.Name,
			Code:        prop.Code,
			Description: prop.Description,
			Require:     prop.Require,
			TypeSpec:    prop.TypeSpec,
			Tag:         prop.Tag,
			System:      prop.System,
			Created:     prop.Created,
			Updated:     prop.Updated,
			Type:        prop.Type,
		}

		// 查找对应的实时值
		id := fmt.Sprintf("%s.%s", deviceName, prop.Code)
		if dataPoints, exists := result[id]; exists && len(dataPoints) > 0 {
			latestPoint := dataPoints[0]
			propWithValue.Val = latestPoint.Val
			propWithValue.Status = latestPoint.Status
			propWithValue.Timestamp = latestPoint.Timestamp
		} else {
			// 没有数据时返回默认值
			propWithValue.Val = ""
			propWithValue.Status = "Error"
			propWithValue.Timestamp = time.Now().UnixMilli()
		}

		propertiesWithValue = append(propertiesWithValue, propWithValue)
	}

	return propertiesWithValue, nil
}

// BatchInsertData 批量插入数据到TDengine
func (r *ReportService) BatchInsertData(requests []BatchInsertDataRequest) error {
	if len(requests) == 0 {
		return fmt.Errorf("请求数据不能为空")
	}

	// 对每个设备执行批量插入
	deviceSQLs := make([]string, 0)

	for _, req := range requests {
		if len(req.DataPoints) == 0 {
			continue
		}

		// 构建INSERT语句 - 使用DBName作为数据库名
		sql := fmt.Sprintf("INSERT INTO `%s`.`%s` (`ts`, `%s`) VALUES ", DBName, req.DeviceName, req.PropertyCode)

		valuesList := make([]string, 0, len(req.DataPoints))
		for _, point := range req.DataPoints {
			// 转义值，防止SQL注入
			escapedValue := strings.ReplaceAll(point.Value, "'", "''")
			valuesList = append(valuesList, fmt.Sprintf("('%s', %s)", point.Timestamp, escapedValue))
		}

		sql += strings.Join(valuesList, ", ")
		deviceSQLs = append(deviceSQLs, sql)
	}

	// 执行SQL语句
	for _, sql := range deviceSQLs {
		utils.DebugLog("执行SQL: %s", sql)
		_, err := r.tdService.db.Exec(sql)
		if err != nil {
			logs.Warn("插入数据失败: %v, SQL: %s", err, sql)
			return fmt.Errorf("插入数据失败: %v", err)
		}
	}

	return nil
}

// BatchInsertDataRequest 批量插入数据请求结构
type BatchInsertDataRequest struct {
	DeviceName   string             `json:"deviceName"`
	PropertyCode string             `json:"propertyCode"`
	DataPoints   []DataPointRequest `json:"dataPoints"`
}

// DataPointRequest 数据点请求结构
type DataPointRequest struct {
	Timestamp string `json:"timestamp"` // 格式: "2006-01-02 15:04:05"
	Value     string `json:"value"`
}

// UploadExcelAndInsertDataFromReader 从Excel文件对象批量插入数据到TDengine
func (r *ReportService) UploadExcelAndInsertDataFromReader(f *excelize.File) error {
	// 获取第一个工作表名称
	sheetList := f.GetSheetList()
	if len(sheetList) == 0 {
		return fmt.Errorf("Excel文件中没有工作表")
	}
	sheetName := sheetList[0]

	// 获取所有行
	rows, err := f.GetRows(sheetName)
	if err != nil {
		return fmt.Errorf("读取Excel数据失败: %v", err)
	}

	if len(rows) <= 1 {
		return fmt.Errorf("Excel文件中没有数据行")
	}

	// 解析Excel数据
	var requests []BatchInsertDataRequest

	// 第一行是标题行：设备名称、属性标识、时间戳1、时间戳2...
	if len(rows) < 1 {
		return fmt.Errorf("Excel文件中没有标题行")
	}

	headers := rows[0]
	if len(headers) < 3 { // 至少需要设备名称、属性标识和一个时间戳列
		return fmt.Errorf("Excel文件格式不正确，至少需要设备名称、属性标识和一个时间戳列")
	}

	// 从第二行开始是数据行
	for i := 1; i < len(rows); i++ {
		row := rows[i]
		if len(row) < 3 { // 至少需要设备名称、属性标识和一个值
			continue
		}

		deviceName := strings.TrimSpace(row[0])
		propertyCode := strings.TrimSpace(row[1])

		if deviceName == "" || propertyCode == "" {
			continue
		}

		var dataPoints []DataPointRequest

		// 从第3列开始，每一列对应一个时间戳和值
		// headers[2]是时间戳，row[2]是对应的值
		// headers[3]是时间戳，row[3]是对应的值...
		for j := 2; j < len(row) && j < len(headers); j++ {
			timestampStr := strings.TrimSpace(headers[j])
			value := strings.TrimSpace(row[j])

			if timestampStr == "" || value == "" {
				continue
			}

			// 尝试解析时间格式，支持多种格式
			parsedTime, err := parseExcelTime(timestampStr)
			if err != nil {
				logs.Warn("时间格式错误 %s 在第 %d 行: %v", timestampStr, i+1, err)
				continue
			}

			dataPoints = append(dataPoints, DataPointRequest{
				Timestamp: parsedTime,
				Value:     value,
			})
		}

		if len(dataPoints) > 0 {
			requests = append(requests, BatchInsertDataRequest{
				DeviceName:   deviceName,
				PropertyCode: propertyCode,
				DataPoints:   dataPoints,
			})
		}
	}

	if len(requests) == 0 {
		return fmt.Errorf("Excel文件中没有有效数据")
	}

	// 使用之前实现的批量插入方法
	return r.BatchInsertData(requests)
}

// parseExcelTime 解析Excel中的时间格式，支持多种格式
func parseExcelTime(timeStr string) (string, error) {
	// 常见的时间格式
	formats := []string{
		"2006/01/02",          // Excel默认日期格式
		"2006-01-02",          // 标准日期格式
		"2006/01/02 15:04:05", // Excel日期时间格式
		"2006-01-02 15:04:05", // 标准日期时间格式
		"01/02/2006",          // 美式日期格式
		"01-02-2006",          // 美式日期格式
		"01/02/2006 15:04:05", // 美式日期时间格式
		"01-02-2006 15:04:05", // 美式日期时间格式
		"01-02-06",            // 对应 12-26-25 (月-日-年)
		"01/02/06",            // 对应 12/26/25
		"06-01-02",            // 对应 25-12-26 (年-月-日)
		"02-01-06",            // 对应 26-12-25 (日-月-年)
		"2006/01/02",
		"2006-01-02",
	}

	for _, format := range formats {
		if t, err := time.ParseInLocation(format, timeStr, time.Local); err == nil {
			return strconv.FormatInt(t.UnixMilli(), 10), nil
		}
	}

	// 如果所有格式都不匹配，返回错误
	return "", fmt.Errorf("无法解析时间格式: %s", timeStr)
}
