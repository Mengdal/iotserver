package duckdb

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"

	"iotserver/internal/dtos"
	"iotserver/internal/tools/datadb/aggregate"
)

// 本文件实现 DuckDB 侧的聚合查询：
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

// aggregateProperty 按时间分桶聚合属性历史。
//
// 只跑一次 GROUP BY：把分桶结果全部取回后在内存里分页（桶数已被
// aggregate.MaxBuckets 限制在 2 万以内），避免为了拿总数把聚合跑第二遍。
func (c *Client) aggregateProperty(req dtos.ThingModelPropertyDataRequest, where string, args []interface{}) ([]dtos.ReportData, int, error) {
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

	query := fmt.Sprintf(
		`SELECT %s AS bucket, cast(%s as varchar) AS v FROM %s
		 WHERE %s GROUP BY bucket%s ORDER BY bucket DESC`,
		bucketExpr, expr, tableProperty, where, having)

	rows, err := c.db.Query(query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	buckets := make([]dtos.ReportData, 0, 256)
	for rows.Next() {
		var bucket int64
		var value sql.NullString
		if err = rows.Scan(&bucket, &value); err != nil {
			return buckets, 0, err
		}
		if !value.Valid {
			continue
		}
		buckets = append(buckets, dtos.ReportData{
			Value: restoreAggValue(value.String),
			Time:  bucket,
		})
	}
	if err = rows.Err(); err != nil {
		return buckets, 0, err
	}

	// IsAll（前端「取全部」）：聚合结果不做分页截断，直接返回全部桶。
	// 桶数已被 aggregate.MaxBuckets 限制在 2 万以内，不会失控；
	// 原始数据路径仍保留 maxPageSize 上限（那才是真正可能很大的结果集）。
	if req.IsAll {
		return buckets, len(buckets), nil
	}

	page, pageSize := normalizePage(req.Page, req.PageSize, false)
	pageRows, total := aggregate.Paginate(buckets, page, pageSize)
	return pageRows, total, nil
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
