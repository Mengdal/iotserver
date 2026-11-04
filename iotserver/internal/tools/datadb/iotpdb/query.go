package iotpdb

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"iotserver/internal/dtos"
	"iotserver/internal/models"
	"iotserver/internal/pkg/tenant"
	"iotserver/internal/tools/datadb/aggregate"
	"iotserver/internal/tools/iotp"
)

/* ------------------------------ 属性 ------------------------------ */

// GetDeviceProperty 查询属性历史/最新值（支持按时间分桶聚合）。
func (c *Client) GetDeviceProperty(req dtos.ThingModelPropertyDataRequest, device models.Device) ([]dtos.ReportData, int, error) {
	var response []dtos.ReportData
	if device.Id == "" {
		return response, 0, fmt.Errorf("iotp: deviceId 为空")
	}
	if strings.TrimSpace(req.Code) == "" {
		return response, 0, nil
	}
	c.flushBeforeRead()

	target, err := c.resolve(context.Background(), device.Id)
	if err != nil {
		return response, 0, err
	}
	field := quoteIdent(iotp.FieldName(req.Code))
	start, end := timeWindow(req.Range, req.Last)
	where := fmt.Sprintf("device_id = %d AND time >= %d AND time <= %d AND %s IS NOT NULL",
		target.iotpID, start, end, field)

	if req.AggInterval > 0 {
		return c.aggregateProperty(target, field, req, start, end)
	}

	count, err := c.countRows(target.table, where)
	if err != nil {
		return response, 0, err
	}
	page, pageSize := normalizePage(req.Page, req.PageSize, req.IsAll)
	sql := fmt.Sprintf(`SELECT time, %s AS v FROM %s WHERE %s ORDER BY time DESC LIMIT %d OFFSET %d`,
		field, c.tableRef(target.table), where, pageSize, (page-1)*pageSize)
	rows, err := c.queryOrEmpty(sql)
	if err != nil {
		return response, count, err
	}
	for _, row := range rows {
		response = append(response, dtos.ReportData{
			Value: restoreValue(row["v"]),
			Time:  int64(numberOf(row["time"])),
		})
	}
	return response, count, nil
}

// aggregateProperty 用 SQL 分桶聚合；桶数由应用层 aggregate.Validate 限制。
func (c *Client) aggregateProperty(target deviceTarget, field string, req dtos.ThingModelPropertyDataRequest, start, end int64) ([]dtos.ReportData, int, error) {
	var response []dtos.ReportData
	if err := c.ensureLoaded(context.Background(), target.table); err != nil {
		c.debugf("iotp: 读取表结构失败，按未知类型处理: %v", err)
	}
	method := req.AggMethod
	if method == "" {
		method = aggregate.MethodAvg
	}
	// 文本列无法做数值聚合：直接返回空，行为与 DuckDB 跳过非数值行一致。
	if aggregate.IsNumericMethod(method) && c.schemaType(target.table, strings.Trim(field, `"`)) == iotp.TypeString {
		return response, 0, nil
	}

	intervalMs := int64(req.AggInterval) * 1000
	expr, numericOnly := aggExpr(method, field)
	having := ""
	if numericOnly {
		having = fmt.Sprintf(" HAVING count(%s) > 0", field)
	}
	sql := fmt.Sprintf(`SELECT (time // %d) * %d AS bucket, cast(%s as varchar) AS v FROM %s
		WHERE device_id = %d AND time >= %d AND time <= %d AND %s IS NOT NULL
		GROUP BY bucket%s ORDER BY bucket DESC`,
		intervalMs, intervalMs, expr, c.tableRef(target.table),
		target.iotpID, start, end, field, having)

	rows, err := c.queryOrEmpty(sql)
	if err != nil {
		return response, 0, err
	}
	buckets := make([]dtos.ReportData, 0, len(rows))
	for _, row := range rows {
		raw, ok := row["v"].(string)
		if !ok {
			continue
		}
		buckets = append(buckets, dtos.ReportData{
			Value: restoreAggValue(raw),
			Time:  int64(numberOf(row["bucket"])),
		})
	}
	if req.IsAll {
		return buckets, len(buckets), nil
	}
	page, pageSize := normalizePage(req.Page, req.PageSize, false)
	pageRows, total := aggregate.Paginate(buckets, page, pageSize)
	return pageRows, total, nil
}

func aggExpr(method, field string) (expr string, numericOnly bool) {
	switch method {
	case aggregate.MethodCount:
		return "count(*)", false
	case aggregate.MethodFirst:
		return fmt.Sprintf("arg_min(cast(%s as varchar), time)", field), false
	case aggregate.MethodLast:
		return fmt.Sprintf("arg_max(cast(%s as varchar), time)", field), false
	case aggregate.MethodMax:
		return fmt.Sprintf("max(%s)", field), true
	case aggregate.MethodMin:
		return fmt.Sprintf("min(%s)", field), true
	case aggregate.MethodSum:
		return fmt.Sprintf("sum(%s)", field), true
	default: // avg
		return fmt.Sprintf("avg(%s)", field), true
	}
}

/* ------------------------------ 事件 / 服务 ------------------------------ */

// GetDeviceEvent 查询事件历史。
func (c *Client) GetDeviceEvent(req dtos.ThingModelEventDataRequest, device models.Device, _ models.Product) ([]dtos.EventData, int, error) {
	var response []dtos.EventData
	if device.Id == "" {
		return response, 0, fmt.Errorf("iotp: deviceId 为空")
	}
	c.flushBeforeRead()
	target, err := c.resolve(context.Background(), device.Id)
	if err != nil {
		return response, 0, err
	}
	fields, err := c.payloadFields(target.table, eventFieldPrefix, req.EventCode)
	if err != nil {
		return response, 0, err
	}
	if len(fields) == 0 {
		return response, 0, nil
	}
	start, end := timeWindow(req.Range, req.Last)
	where := fmt.Sprintf("device_id = %d AND time >= %d AND time <= %d", target.iotpID, start, end)
	if len(fields) == 1 {
		where += fmt.Sprintf(" AND %s IS NOT NULL", quoteIdent(fields[0]))
	} else {
		conditions := make([]string, 0, len(fields))
		for _, field := range fields {
			conditions = append(conditions, fmt.Sprintf("%s IS NOT NULL", quoteIdent(field)))
		}
		where += " AND (" + strings.Join(conditions, " OR ") + ")"
	}
	count, err := c.countRows(target.table, where)
	if err != nil {
		return response, 0, err
	}
	page, pageSize := normalizePage(req.Page, req.PageSize, req.IsAll)
	sql := fmt.Sprintf(`SELECT time, %s FROM %s WHERE %s ORDER BY time DESC LIMIT %d OFFSET %d`,
		selectFields(fields), c.tableRef(target.table), where, pageSize, (page-1)*pageSize)
	rows, err := c.queryOrEmpty(sql)
	if err != nil {
		return response, count, err
	}
	for _, row := range rows {
		ts := int64(numberOf(row["time"]))
		for _, field := range fields {
			raw, ok := row[field].(string)
			if !ok || raw == "" {
				continue
			}
			item := dtos.EventData{EventCode: strings.TrimPrefix(field, eventFieldPrefix), EventTime: ts}
			_ = json.Unmarshal([]byte(raw), &item)
			item.EventCode = strings.TrimPrefix(field, eventFieldPrefix)
			item.EventTime = ts
			response = append(response, item)
		}
	}
	return response, count, nil
}

// GetDeviceService 查询服务调用历史。
func (c *Client) GetDeviceService(req dtos.ThingModelServiceDataRequest, device models.Device, _ models.Product) ([]dtos.SaveServiceIssueData, int, error) {
	var response []dtos.SaveServiceIssueData
	if device.Id == "" {
		return response, 0, fmt.Errorf("iotp: deviceId 为空")
	}
	c.flushBeforeRead()
	target, err := c.resolve(context.Background(), device.Id)
	if err != nil {
		return response, 0, err
	}
	fields, err := c.payloadFields(target.table, serviceFieldPrefix, req.Code)
	if err != nil {
		return response, 0, err
	}
	if len(fields) == 0 {
		return response, 0, nil
	}
	start, end := timeWindow(req.Range, req.Last)
	where := fmt.Sprintf("device_id = %d AND time >= %d AND time <= %d", target.iotpID, start, end)
	if len(fields) == 1 {
		where += fmt.Sprintf(" AND %s IS NOT NULL", quoteIdent(fields[0]))
	} else {
		conditions := make([]string, 0, len(fields))
		for _, field := range fields {
			conditions = append(conditions, fmt.Sprintf("%s IS NOT NULL", quoteIdent(field)))
		}
		where += " AND (" + strings.Join(conditions, " OR ") + ")"
	}
	count, err := c.countRows(target.table, where)
	if err != nil {
		return response, 0, err
	}
	page, pageSize := normalizePage(req.Page, req.PageSize, req.IsAll)
	sql := fmt.Sprintf(`SELECT time, %s FROM %s WHERE %s ORDER BY time DESC LIMIT %d OFFSET %d`,
		selectFields(fields), c.tableRef(target.table), where, pageSize, (page-1)*pageSize)
	rows, err := c.queryOrEmpty(sql)
	if err != nil {
		return response, count, err
	}
	for _, row := range rows {
		ts := int64(numberOf(row["time"]))
		for _, field := range fields {
			raw, ok := row[field].(string)
			if !ok || raw == "" {
				continue
			}
			item := dtos.SaveServiceIssueData{Code: strings.TrimPrefix(field, serviceFieldPrefix), Time: ts}
			_ = json.Unmarshal([]byte(raw), &item)
			item.Code = strings.TrimPrefix(field, serviceFieldPrefix)
			item.Time = ts
			response = append(response, item)
		}
	}
	return response, count, nil
}

// payloadFields 返回事件/服务对应的 iotp 字段名；code 为空时扫描表结构里的前缀字段。
func (c *Client) payloadFields(table, prefix, code string) ([]string, error) {
	if strings.TrimSpace(code) != "" {
		return []string{prefix + iotp.FieldName(code)}, nil
	}
	if err := c.ensureLoaded(context.Background(), table); err != nil {
		return nil, err
	}
	c.schemaMu.Lock()
	defer c.schemaMu.Unlock()
	out := make([]string, 0, 8)
	for name := range c.schemas[table] {
		if strings.HasPrefix(name, prefix) {
			out = append(out, name)
		}
	}
	return out, nil
}

/* ------------------------------ 计数 ------------------------------ */

// GetDevicePropertyCount 属性条数。
func (c *Client) GetDevicePropertyCount(req dtos.ThingModelPropertyDataRequest) (int, error) {
	if strings.TrimSpace(req.DeviceId) == "" || strings.TrimSpace(req.Code) == "" {
		return 0, nil
	}
	target, err := c.resolve(context.Background(), req.DeviceId)
	if err != nil {
		return 0, err
	}
	start, end := timeWindow(req.Range, req.Last)
	where := fmt.Sprintf("device_id = %d AND time >= %d AND time <= %d AND %s IS NOT NULL",
		target.iotpID, start, end, quoteIdent(iotp.FieldName(req.Code)))
	return c.countRows(target.table, where)
}

// GetDeviceEventCount 事件条数。
func (c *Client) GetDeviceEventCount(req dtos.ThingModelEventDataRequest) (int, error) {
	if strings.TrimSpace(req.DeviceId) == "" {
		return 0, nil
	}
	target, err := c.resolve(context.Background(), req.DeviceId)
	if err != nil {
		return 0, err
	}
	fields, err := c.payloadFields(target.table, eventFieldPrefix, req.EventCode)
	if err != nil || len(fields) == 0 {
		return 0, err
	}
	start, end := timeWindow(req.Range, req.Last)
	where := fmt.Sprintf("device_id = %d AND time >= %d AND time <= %d", target.iotpID, start, end)
	conditions := make([]string, 0, len(fields))
	for _, field := range fields {
		conditions = append(conditions, fmt.Sprintf("%s IS NOT NULL", quoteIdent(field)))
	}
	where += " AND (" + strings.Join(conditions, " OR ") + ")"
	return c.countRows(target.table, where)
}

// GetDeviceMsgCountByGiveTime 设备在时间窗内的消息数（按 iotp 行计）。
func (c *Client) GetDeviceMsgCountByGiveTime(deviceId string, startTime, endTime int64) (int, error) {
	target, err := c.resolve(context.Background(), deviceId)
	if err != nil {
		return 0, err
	}
	where := fmt.Sprintf("device_id = %d AND time >= %d AND time <= %d", target.iotpID, startTime, endTime)
	return c.countRows(target.table, where)
}

// CountAllMsgByTimeRange 平台/租户在时间窗内的消息总数（遍历库内所有表求和）。
func (c *Client) CountAllMsgByTimeRange(tenantID string, startTime, endTime int64) (int, error) {
	tables, err := c.listTables()
	if err != nil {
		return 0, err
	}
	total := 0
	for _, table := range tables {
		where := fmt.Sprintf("time >= %d AND time <= %d", startTime, endTime)
		if tenantID != "" {
			where += " AND " + quoteIdent("tenant_id") + " = " + quoteString(tenantID)
		}
		count, err := c.countRows(table, where)
		if err != nil {
			c.debugf("iotp: 统计表 %s 消息数失败（跳过）: %v", table, err)
			continue
		}
		total += count
	}
	return total, nil
}

// CountMsgGroupByDevice 按设备分组统计消息数（看板排行榜）。
//
// iotp 的 device_id 是平台设备 ID 的数值映射；这里把结果反向映射回平台设备 ID。
func (c *Client) CountMsgGroupByDevice(tenantID string, startTime, endTime int64) (map[string]int, error) {
	result := make(map[string]int)
	tables, err := c.listTables()
	if err != nil {
		return result, err
	}
	reverse, err := c.deviceIDMap(tenantID)
	if err != nil {
		return result, err
	}
	for _, table := range tables {
		where := fmt.Sprintf("time >= %d AND time <= %d", startTime, endTime)
		if tenantID != "" {
			where += " AND " + quoteIdent("tenant_id") + " = " + quoteString(tenantID)
		}
		sql := fmt.Sprintf(`SELECT device_id, count(*) AS c FROM %s WHERE %s GROUP BY device_id`,
			c.tableRef(table), where)
		rows, err := c.queryOrEmpty(sql)
		if err != nil {
			c.debugf("iotp: 表 %s 按设备统计失败（跳过）: %v", table, err)
			continue
		}
		for _, row := range rows {
			id := int64(numberOf(row["device_id"]))
			platformID, ok := reverse[id]
			if !ok {
				continue
			}
			result[platformID] += int(numberOf(row["c"]))
		}
	}
	return result, nil
}

// deviceIDMap 建立 iotp device_id → 平台设备 ID 的反向表。
func (c *Client) deviceIDMap(tenantID string) (map[int64]string, error) {
	reverse := make(map[int64]string)
	if c.meta == nil {
		return reverse, nil
	}
	ctx := context.Background()
	if tenantID != "" {
		ctx = tenant.With(tenant.Platform(ctx), tenant.Context{TenantID: tenantID})
	} else {
		ctx = tenant.Platform(ctx)
	}
	devices, _, err := c.meta.WithCtx(ctx).DevicesSearch(0, -1, dtos.DeviceSearchQueryRequest{})
	if err != nil {
		return reverse, err
	}
	for _, device := range devices {
		reverse[iotp.DeviceID(device.Id)] = device.Id
	}
	return reverse, nil
}

/* ------------------------------ 查询辅助 ------------------------------ */

// ensureLoaded 保证表结构已从 iotp 拉取（幂等，仅在首次读取时发请求）。
func (c *Client) ensureLoaded(ctx context.Context, table string) error {
	c.schemaMu.Lock()
	loaded := c.loaded[table]
	c.schemaMu.Unlock()
	if loaded {
		return nil
	}
	return c.loadSchema(ctx, table)
}

func (c *Client) flushBeforeRead() {
	if err := c.flush(); err != nil {
		c.warnf("iotp: 查询前写入落盘失败（可能读到旧数据）: %v", err)
	}
}

func (c *Client) queryOrEmpty(sql string) ([]map[string]any, error) {
	ctx, cancel := context.WithTimeout(context.Background(), readTimeout)
	defer cancel()
	result, err := c.api.Query(ctx, sql, "")
	if err != nil {
		var apiErr *iotp.APIError
		// 表/字段还不存在（设备从未写入）时 iotp 返回 UNPROCESSABLE；
		// 读路径按「没有数据」处理，与 DuckDB 空表查询一致。
		if errors.As(err, &apiErr) && apiErr.Code == "UNPROCESSABLE" {
			return nil, nil
		}
		return nil, err
	}
	return result.Rows, nil
}

func (c *Client) countRows(table, where string) (int, error) {
	sql := fmt.Sprintf("SELECT count(*) AS c FROM %s WHERE %s", c.tableRef(table), where)
	rows, err := c.queryOrEmpty(sql)
	if err != nil || len(rows) == 0 {
		return 0, err
	}
	return int(numberOf(rows[0]["c"])), nil
}

func (c *Client) listTables() ([]string, error) {
	c.tablesMu.Lock()
	if time.Since(c.tablesAt) < time.Minute {
		tables := c.tables
		c.tablesMu.Unlock()
		return tables, nil
	}
	c.tablesMu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), readTimeout)
	defer cancel()
	tables, err := c.api.Tables(ctx)
	if err != nil {
		return nil, err
	}
	c.tablesMu.Lock()
	c.tables = tables
	c.tablesAt = time.Now()
	c.tablesMu.Unlock()
	return tables, nil
}

func (c *Client) schemaType(table, field string) string {
	c.schemaMu.Lock()
	defer c.schemaMu.Unlock()
	if schema, ok := c.schemas[table]; ok {
		return schema[field]
	}
	return ""
}

func selectFields(fields []string) string {
	out := make([]string, 0, len(fields))
	for _, field := range fields {
		out = append(out, quoteIdent(field))
	}
	return strings.Join(out, ", ")
}

// timeWindow 计算查询时间窗（毫秒），与 DuckDB 后端保持一致。
func timeWindow(rng []int64, last bool) (start, end int64) {
	if len(rng) == 2 {
		if rng[0] < rng[1] {
			return rng[0], rng[1]
		}
		return rng[1], rng[0]
	}
	now := time.Now()
	if last {
		return now.Add(-30*time.Minute).UnixNano() / int64(time.Millisecond), nowMillis()
	}
	return now.AddDate(-1, 0, 0).UnixNano() / int64(time.Millisecond), nowMillis()
}

// normalizePage 归一化分页参数；isAll 时取单页上限。
func normalizePage(page, pageSize int, isAll bool) (int, int) {
	if isAll {
		return 1, maxPageSize
	}
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = defaultPageSize
	}
	if pageSize > maxPageSize {
		pageSize = maxPageSize
	}
	return page, pageSize
}

// numberOf 把查询结果里的数字（JSON float64 / json.Number）转成 float64。
func numberOf(value any) float64 {
	switch v := value.(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case json.Number:
		f, _ := v.Float64()
		return f
	case string:
		f, _ := strconv.ParseFloat(v, 64)
		return f
	}
	return 0
}

// restoreValue 原始行值：数值保持 float64，其它原样。
func restoreValue(value any) any {
	switch v := value.(type) {
	case json.Number:
		if f, err := v.Float64(); err == nil {
			return f
		}
		return v.String()
	default:
		return value
	}
}

// restoreAggValue 把 cast 成字符串的聚合值还原成数值或字符串。
func restoreAggValue(raw string) interface{} {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return raw
	}
	if value, err := strconv.ParseFloat(trimmed, 64); err == nil {
		return value
	}
	return raw
}

func quoteString(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}
