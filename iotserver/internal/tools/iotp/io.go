package iotp

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
)

// WriteRow 一条待写入的记录（JSON 信封：measurement + tags + fields + timestamp）。
//
// 时间戳单位为毫秒；为 0 时由 iotp 使用服务端当前时间。
type WriteRow struct {
	Measurement string
	// Tags 行协议标签，当前只支持 device_id。
	// 注意：JSON 信封里 device_id 必须是数字，字符串形式会被整条丢弃（"no valid elements"）。
	Tags map[string]any
	// Fields 测点值。数值/布尔/字符串会被 iotp 按严格类型校验。
	Fields map[string]any
	// Timestamp epoch 毫秒。
	Timestamp int64
}

// WriteTableResult 单张表的写入结果。
type WriteTableResult struct {
	Accepted int            `json:"accepted"`
	Rejected int            `json:"rejected"`
	Reasons  map[string]int `json:"reasons"`
}

// WriteResult 批量写入结果（部分接受是正常情况）。
type WriteResult struct {
	AcceptedRows int                         `json:"accepted_rows"`
	RejectedRows int                         `json:"rejected_rows"`
	Tables       map[string]WriteTableResult `json:"tables"`
}

// Rejected 返回被拒绝的行数（方便调用方判断是否需要补 schema 后重试）。
func (r WriteResult) Rejected() int { return r.RejectedRows }

// Reason 返回指定原因的拒绝行数（所有表合计）。
func (r WriteResult) Reason(name string) int {
	total := 0
	for _, table := range r.Tables {
		total += table.Reasons[name]
	}
	return total
}

type writeEnvelope struct {
	Measurement string         `json:"measurement"`
	Tags        map[string]any `json:"tags,omitempty"`
	Fields      map[string]any `json:"fields"`
	Timestamp   int64          `json:"timestamp,omitempty"`
}

// Write 批量写入数据点。rows 为空时直接返回。
//
// iotp 的写入接口是 POST /write?db=<库>，JSON 信封可数组批量；
// 部分接受时返回 200，被拒行数与原因在 WriteResult.Tables 里。
func (c *Client) Write(ctx context.Context, rows []WriteRow) (WriteResult, error) {
	var result WriteResult
	if len(rows) == 0 {
		return result, nil
	}
	envelopes := make([]writeEnvelope, 0, len(rows))
	for _, row := range rows {
		envelopes = append(envelopes, writeEnvelope{
			Measurement: row.Measurement,
			Tags:        row.Tags,
			Fields:      row.Fields,
			Timestamp:   row.Timestamp,
		})
	}
	path := "/write?db=" + url.QueryEscape(c.db)
	if err := c.do(ctx, http.MethodPost, path, envelopes, &result); err != nil {
		return result, err
	}
	return result, nil
}

// QueryResult iotp SQL 查询结果。
//
// rows 是对象数组（列名 → 值）；标签列（device_id、tenant_id 等）也会出现在行里。
type QueryResult struct {
	Rows         []map[string]any `json:"rows"`
	Truncated    bool             `json:"truncated"`
	ReturnedRows int              `json:"returned_rows"`
	ElapsedMs    float64          `json:"elapsed_ms"`
	Message      string           `json:"message"`
}

// Query 执行一条只读 SQL。
//
// mode：all（内存 buffer ∪ parquet）或 history（仅 parquet）；留空使用服务端默认 all。
func (c *Client) Query(ctx context.Context, sql string, mode string) (QueryResult, error) {
	var result QueryResult
	body := map[string]any{"sql": sql}
	if mode != "" {
		body["mode"] = mode
	}
	if err := c.do(ctx, http.MethodPost, "/api/v1/query", body, &result); err != nil {
		return result, err
	}
	return result, nil
}

// Tables 列出库内表名。iotp 不支持 information_schema/SHOW TABLES，只能走元数据接口。
func (c *Client) Tables(ctx context.Context) ([]string, error) {
	var tables []string
	path := "/api/v1/meta/tables?db=" + url.QueryEscape(c.db)
	if err := c.do(ctx, http.MethodGet, path, nil, &tables); err != nil {
		return nil, err
	}
	return tables, nil
}

// TableSchema 单表字段定义。
type TableSchema struct {
	Db          string  `json:"db"`
	Table       string  `json:"table"`
	Fields      []Field `json:"fields"`
	DeviceCount int     `json:"device_count"`
}

// TableFields 读取单表字段定义（用于事件/服务这类需要扫描前缀字段的场景）。
func (c *Client) TableFields(ctx context.Context, table string) ([]Field, error) {
	var schema TableSchema
	path := fmt.Sprintf("/api/v1/meta/tables/%s/%s", url.PathEscape(c.db), url.PathEscape(table))
	if err := c.do(ctx, http.MethodGet, path, nil, &schema); err != nil {
		return nil, err
	}
	return schema.Fields, nil
}
