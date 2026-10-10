package ducklake

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"

	"iotserver/internal/dtos"
	"iotserver/internal/tools/datadb/aggregate"
)

// 本文件实现 DuckLake 侧的聚合查询：
// 用 SQL 做时间分桶（(ts_ms // interval) * interval，按 epoch 对齐）并在库内聚合，
// 只把聚合结果传回应用层；分页作用于「桶」而不是原始行。
//
// 所有聚合值统一 cast 成字符串再在 Go 侧还原类型：
// 能解析为数字的按 float64 返回，否则按字符串返回（保证 text/enum 属性的首值/末值不失真）。

// aggExpr 返回聚合表达式；numericOnly 表示该表达式只使用数值列
func aggExpr(method string) (expr string, numericOnly bool) {
	switch method {
	case aggregate.MethodCount:
		return "count(*)", false
	case aggregate.MethodFirst:
		return "arg_min(coalesce(cast(num_value as varchar), text_value), ts_ms)", false
	case aggregate.MethodLast:
		return "arg_max(coalesce(cast(num_value as varchar), text_value), ts_ms)", false
	case aggregate.MethodMax:
		return "max(num_value)", true
	case aggregate.MethodMin:
		return "min(num_value)", true
	case aggregate.MethodSum:
		return "sum(num_value)", true
	default: // avg
		return "avg(num_value)", true
	}
}

// aggregateProperty 按时间分桶聚合属性历史
func (c *Client) aggregateProperty(req dtos.ThingModelPropertyDataRequest, where string, args []interface{}) ([]dtos.ReportData, int, error) {
	var response []dtos.ReportData

	method := req.AggMethod
	if method == "" {
		method = aggregate.MethodAvg
	}
	intervalMs := int64(req.AggInterval) * 1000
	bucketExpr := fmt.Sprintf("(ts_ms // %d) * %d", intervalMs, intervalMs)
	expr, numericOnly := aggExpr(method)

	having := ""
	if numericOnly {
		// 桶内没有任何数值行时该桶不输出
		having = " HAVING count(num_value) > 0"
	}

	// 桶总数
	countSQL := fmt.Sprintf(
		`SELECT cast(count(*) as BIGINT) FROM (SELECT %s AS b FROM %s.%s WHERE %s GROUP BY b%s)`,
		bucketExpr, lakeAlias, tableProperty, where, having)
	var total int
	if err := c.db.QueryRow(countSQL, args...).Scan(&total); err != nil {
		return response, 0, err
	}
	if total == 0 {
		return response, 0, nil
	}

	page, pageSize := normalizePage(req.Page, req.PageSize, req.IsAll)
	query := fmt.Sprintf(
		`SELECT %s AS bucket, cast(%s as varchar) AS v FROM %s.%s
		 WHERE %s GROUP BY bucket%s ORDER BY bucket DESC LIMIT ? OFFSET ?`,
		bucketExpr, expr, lakeAlias, tableProperty, where, having)
	args = append(args, pageSize, (page-1)*pageSize)

	rows, err := c.db.Query(query, args...)
	if err != nil {
		return response, total, err
	}
	defer rows.Close()

	for rows.Next() {
		var bucket int64
		var value sql.NullString
		if err = rows.Scan(&bucket, &value); err != nil {
			return response, total, err
		}
		if !value.Valid {
			continue
		}
		response = append(response, dtos.ReportData{
			Value: restoreAggValue(value.String),
			Time:  bucket,
		})
	}
	return response, total, rows.Err()
}

// restoreAggValue 把 cast 成字符串的聚合值还原成数值或字符串
func restoreAggValue(raw string) interface{} {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return raw
	}
	if v, err := strconv.ParseFloat(trimmed, 64); err == nil {
		return v
	}
	return raw
}
