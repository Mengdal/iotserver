package iotpdb

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"

	"iotserver/internal/dtos"
	"iotserver/internal/tools/iotp"
)

const (
	kindProperty = iota
	kindEvent
	kindService
)

// Insert 写入一个设备的属性/事件/服务数据。
//
// table 约定与 DuckDB 后端一致：`<deviceId>` 属性、`event:<deviceId>` 事件、
// `service:<deviceId>` 服务；适配器负责选表、补 schema、注册设备并批量写 iotp。
func (c *Client) Insert(ctx context.Context, table string, data map[string]interface{}) error {
	if len(data) == 0 {
		return nil
	}
	kind, deviceID := splitInsertTable(table)
	if deviceID == "" {
		return fmt.Errorf("iotp: 非法表名 %q", table)
	}
	target, err := c.resolve(ctx, deviceID)
	if err != nil {
		return err
	}
	if err = c.ensureBase(ctx, target.table); err != nil {
		return err
	}
	if err = c.ensureDevice(ctx, deviceID, target); err != nil {
		return err
	}

	var rows []pendingRow
	switch kind {
	case kindEvent, kindService:
		prefix := eventFieldPrefix
		if kind == kindService {
			prefix = serviceFieldPrefix
		}
		fields := make(map[string]any, len(data))
		ts := int64(0)
		for code, v := range data {
			raw := valueToString(v)
			fields[prefix+iotp.FieldName(code)] = raw
			if t := payloadTime(raw); t > 0 {
				ts = t
			}
		}
		if ts <= 0 {
			ts = nowMillis()
		}
		coerced, cErr := c.ensureSchema(ctx, target.table, fields)
		if cErr != nil {
			return cErr
		}
		rows = append(rows, pendingRow{table: target.table, deviceID: target.iotpID, ts: ts, fields: coerced})
	default:
		for code, v := range data {
			value, ts := unwrapValue(v)
			if ts <= 0 {
				ts = nowMillis()
			}
			field := iotp.FieldName(code)
			coerced, cErr := c.ensureSchema(ctx, target.table, map[string]any{field: value})
			if cErr != nil {
				return cErr
			}
			rows = append(rows, pendingRow{table: target.table, deviceID: target.iotpID, ts: ts, fields: coerced})
		}
	}
	return c.enqueue(rows)
}

func splitInsertTable(table string) (kind int, deviceID string) {
	switch {
	case strings.HasPrefix(table, "event:"):
		return kindEvent, strings.TrimPrefix(table, "event:")
	case strings.HasPrefix(table, "service:"):
		return kindService, strings.TrimPrefix(table, "service:")
	default:
		return kindProperty, table
	}
}

// ensureDevice 注册设备实例（写数据前必须存在，否则 iotp 会按 unknown_device 拒绝）。
func (c *Client) ensureDevice(ctx context.Context, deviceID string, target deviceTarget) error {
	if c.isRegistered(deviceID) {
		return nil
	}
	err := c.api.RegisterDevice(ctx, target.table, iotp.Device{
		DeviceID: target.iotpID,
		Name:     target.name,
		Tags:     target.tags,
	})
	if err != nil {
		return err
	}
	c.markRegistered(deviceID)
	return nil
}

// ensureSchema 把本次要写的字段对齐到 iotp 表结构，并把值规整成列类型。
//
// 表字段可能来自两类来源：物模型（iotpsync 已同步）或首包动态推断。
func (c *Client) ensureSchema(ctx context.Context, table string, values map[string]any) (map[string]any, error) {
	c.schemaMu.Lock()
	loaded := c.loaded[table]
	c.schemaMu.Unlock()
	if !loaded {
		if err := c.loadSchema(ctx, table); err != nil {
			return nil, err
		}
	}

	c.schemaMu.Lock()
	defer c.schemaMu.Unlock()
	schema := c.schemas[table]
	if schema == nil {
		schema = make(map[string]string, len(values))
		c.schemas[table] = schema
	}
	out := make(map[string]any, len(values))
	for name, value := range values {
		typ, ok := schema[name]
		if !ok {
			typ = iotp.InferType(value)
			if err := c.api.AddField(ctx, table, iotp.Field{Name: name, Type: typ}); err != nil && !iotp.IsConflict(err) {
				return nil, err
			}
			schema[name] = typ
		}
		coerced, ok := coerceValue(value, typ)
		if !ok {
			c.warnf("iotp: 字段 %s 的值类型与列类型 %s 不匹配，跳过该测点", name, typ)
			continue
		}
		out[name] = coerced
	}
	return out, nil
}

// loadSchema 从 iotp 读取表结构；表不存在时创建基础 schema。
func (c *Client) loadSchema(ctx context.Context, table string) error {
	fields, err := c.api.TableFields(ctx, table)
	if err != nil {
		if iotp.IsNotFound(err) {
			return c.ensureBase(ctx, table)
		}
		return err
	}
	c.schemaMu.Lock()
	schema := make(map[string]string, len(fields))
	for _, field := range fields {
		schema[field.Name] = field.Type
	}
	c.schemas[table] = schema
	c.ensured[table] = true
	c.loaded[table] = true
	c.schemaMu.Unlock()
	return nil
}

/* ------------------------------ 缓冲与批量写 ------------------------------ */

func (c *Client) enqueue(rows []pendingRow) error {
	if len(rows) == 0 {
		return nil
	}
	c.mu.Lock()
	if len(c.pending)+len(rows) > c.maxPending {
		c.mu.Unlock()
		return errBufferFull
	}
	c.pending = append(c.pending, rows...)
	shouldFlush := len(c.pending) >= c.flushRows
	c.mu.Unlock()
	if shouldFlush {
		return c.flush()
	}
	return nil
}

// writeGroup 一行 iotp 宽表记录及其来源的缓冲行（重试时按组回填，避免重复写入已成功的行）。
type writeGroup struct {
	row     iotp.WriteRow
	pending []pendingRow
}

// flush 把缓冲区按 (表, 设备, 时间戳) 合并后写 iotp；失败时只把未成功的组放回缓冲区。
func (c *Client) flush() error {
	c.mu.Lock()
	rows := c.pending
	c.pending = nil
	c.mu.Unlock()
	if len(rows) == 0 {
		return nil
	}

	groups := groupRows(rows)
	ctx, cancel := context.WithTimeout(context.Background(), writeTimeout)
	defer cancel()

	const chunkSize = 500
	for start := 0; start < len(groups); start += chunkSize {
		end := min(start+chunkSize, len(groups))
		chunk := groups[start:end]
		res, err := c.api.Write(ctx, writeRowsOf(chunk))
		if err != nil {
			c.requeue(pendingOf(groups[start:]))
			return err
		}
		if res.RejectedRows == 0 {
			continue
		}

		c.warnf("iotp: %d 行被拒绝，原因 %v", res.RejectedRows, res.Tables)
		retry := rejectedGroups(chunk, res, c.db)
		if len(retry) == 0 {
			// 响应没有按表给出原因时，保守地重试整个分片。
			retry = chunk
		}
		c.remediate(ctx, writeRowsOf(retry))
		retryRes, retryErr := c.api.Write(ctx, writeRowsOf(retry))
		if retryErr != nil {
			c.requeue(pendingOf(retry))
			c.requeue(pendingOf(groups[end:]))
			return retryErr
		}
		if retryRes.RejectedRows > 0 {
			c.warnf("iotp: 补全 schema 后仍有 %d 行被拒绝: %v", retryRes.RejectedRows, retryRes.Tables)
		}
	}
	return nil
}

// groupRows 合并同一 (表, 设备, 时间戳) 的测点成一行宽表记录。
func groupRows(rows []pendingRow) []writeGroup {
	type key struct {
		table    string
		deviceID int64
		ts       int64
	}
	order := make([]key, 0, len(rows))
	merged := make(map[key]*writeGroup, len(rows))
	for _, row := range rows {
		k := key{table: row.table, deviceID: row.deviceID, ts: row.ts}
		item, ok := merged[k]
		if !ok {
			item = &writeGroup{row: iotp.WriteRow{
				Measurement: row.table,
				Tags:        map[string]string{"device_id": strconv.FormatInt(row.deviceID, 10)},
				Fields:      make(map[string]any, len(row.fields)),
				Timestamp:   row.ts,
			}}
			merged[k] = item
			order = append(order, k)
		}
		for name, value := range row.fields {
			item.row.Fields[name] = value
		}
		item.pending = append(item.pending, row)
	}
	out := make([]writeGroup, 0, len(order))
	for _, k := range order {
		out = append(out, *merged[k])
	}
	return out
}

func writeRowsOf(groups []writeGroup) []iotp.WriteRow {
	out := make([]iotp.WriteRow, len(groups))
	for i := range groups {
		out[i] = groups[i].row
	}
	return out
}

func pendingOf(groups []writeGroup) []pendingRow {
	var out []pendingRow
	for _, group := range groups {
		out = append(out, group.pending...)
	}
	return out
}

// rejectedGroups 只挑出被 iotp 按表拒绝的组；响应 key 形如 "db.table"。
func rejectedGroups(groups []writeGroup, res iotp.WriteResult, db string) []writeGroup {
	var out []writeGroup
	for _, group := range groups {
		for key, table := range res.Tables {
			if table.Rejected == 0 {
				continue
			}
			if key == group.row.Measurement || key == db+"."+group.row.Measurement ||
				strings.HasSuffix(key, "."+group.row.Measurement) {
				out = append(out, group)
				break
			}
		}
	}
	return out
}

func (c *Client) requeue(rows []pendingRow) {
	c.mu.Lock()
	defer c.mu.Unlock()
	room := c.maxPending - len(c.pending)
	if room <= 0 {
		c.warnf("iotp: 缓冲区已满，丢弃 %d 行", len(rows))
		return
	}
	if len(rows) > room {
		rows = rows[len(rows)-room:]
	}
	c.pending = append(rows, c.pending...)
}

// remediate 被拒绝后按行内容补表/字段/设备，随后由调用方重试一次。
func (c *Client) remediate(ctx context.Context, rows []iotp.WriteRow) {
	for _, row := range rows {
		c.schemaMu.Lock()
		delete(c.schemas, row.Measurement)
		delete(c.ensured, row.Measurement)
		delete(c.loaded, row.Measurement)
		c.schemaMu.Unlock()

		_ = c.api.EnsureDatabase(ctx)
		if err := c.api.EnsureTable(ctx, row.Measurement, iotp.BaseFields()); err != nil {
			c.warnf("iotp: 补表 %s 失败: %v", row.Measurement, err)
		}
		for name, value := range row.Fields {
			_ = c.addField(ctx, row.Measurement, iotp.Field{Name: name, Type: iotp.InferType(value)})
		}
		id, err := strconv.ParseInt(row.Tags["device_id"], 10, 64)
		if err != nil {
			continue
		}
		if device, ok := c.cachedByIotpID(id); ok {
			_ = c.api.RegisterDevice(ctx, row.Measurement, iotp.Device{DeviceID: id, Name: device.name, Tags: device.tags})
		}
	}
}

/* ------------------------------ 值处理 ------------------------------ */

func unwrapValue(value any) (any, int64) {
	switch report := value.(type) {
	case dtos.ReportData:
		return report.Value, report.Time
	case *dtos.ReportData:
		if report != nil {
			return report.Value, report.Time
		}
	}
	return value, 0
}

func valueToString(value any) string {
	if s, ok := value.(string); ok {
		return s
	}
	if value == nil {
		return ""
	}
	if raw, err := json.Marshal(value); err == nil {
		return string(raw)
	}
	return fmt.Sprint(value)
}

// payloadTime 从事件/服务的 JSON 载荷里取时间（秒自动转毫秒）。
func payloadTime(raw string) int64 {
	if strings.TrimSpace(raw) == "" {
		return 0
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return 0
	}
	for _, key := range []string{"time", "eventTime", "event_time", "reportTime", "report_time"} {
		if value, ok := payload[key]; ok {
			if ts, ok := toInt64(value); ok && ts > 0 {
				if ts < 1_000_000_000_000 {
					ts *= 1000
				}
				return ts
			}
		}
	}
	return 0
}

// coerceValue 把 Go 值规整成 iotp 列类型。
//
// iotp 对 JSON 字面量做严格类型校验：整数→I64、带小数点的数→F64。
// 因此 F64 列必须保留小数点（json.Number("23.0")），否则会被当成整数拒绝。
func coerceValue(value any, typ string) (any, bool) {
	switch typ {
	case iotp.TypeI64:
		n, ok := toInt64(value)
		return n, ok
	case iotp.TypeF64:
		f, ok := toFloat64(value)
		if !ok || math.IsNaN(f) || math.IsInf(f, 0) {
			return nil, false
		}
		text := strconv.FormatFloat(f, 'f', -1, 64)
		if !strings.ContainsAny(text, ".eE") {
			text += ".0"
		}
		return json.Number(text), true
	case iotp.TypeBool:
		b, ok := toBool(value)
		return b, ok
	default:
		return valueToString(value), true
	}
}

func toInt64(value any) (int64, bool) {
	switch v := value.(type) {
	case int:
		return int64(v), true
	case int8:
		return int64(v), true
	case int16:
		return int64(v), true
	case int32:
		return int64(v), true
	case int64:
		return v, true
	case uint:
		return int64(v), true
	case uint8:
		return int64(v), true
	case uint16:
		return int64(v), true
	case uint32:
		return int64(v), true
	case uint64:
		return int64(v), true
	case float32:
		if math.Trunc(float64(v)) == float64(v) {
			return int64(v), true
		}
	case float64:
		if math.Trunc(v) == v && !math.IsInf(v, 0) {
			return int64(v), true
		}
	case json.Number:
		if n, err := v.Int64(); err == nil {
			return n, true
		}
		if f, err := v.Float64(); err == nil && math.Trunc(f) == f {
			return int64(f), true
		}
	case string:
		if n, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64); err == nil {
			return n, true
		}
		if f, err := strconv.ParseFloat(strings.TrimSpace(v), 64); err == nil && math.Trunc(f) == f {
			return int64(f), true
		}
	case bool:
		if v {
			return 1, true
		}
		return 0, true
	}
	return 0, false
}

func toFloat64(value any) (float64, bool) {
	switch v := value.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int8:
		return float64(v), true
	case int16:
		return float64(v), true
	case int32:
		return float64(v), true
	case int64:
		return float64(v), true
	case uint:
		return float64(v), true
	case uint8:
		return float64(v), true
	case uint16:
		return float64(v), true
	case uint32:
		return float64(v), true
	case uint64:
		return float64(v), true
	case json.Number:
		f, err := v.Float64()
		return f, err == nil
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
		return f, err == nil
	case bool:
		if v {
			return 1, true
		}
		return 0, true
	}
	return 0, false
}

func toBool(value any) (bool, bool) {
	switch v := value.(type) {
	case bool:
		return v, true
	case string:
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "true", "1", "on", "yes":
			return true, true
		case "false", "0", "off", "no", "":
			return false, true
		}
	case int:
		return v != 0, true
	case int64:
		return v != 0, true
	case float64:
		return v != 0, true
	case json.Number:
		f, err := v.Float64()
		return f != 0, err == nil
	}
	return false, false
}
