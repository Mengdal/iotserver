package duckdb

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	_ "github.com/duckdb/duckdb-go/v2"

	"iotserver/internal/dtos"
	interfaces "iotserver/internal/iotserver/core/interface"
	"iotserver/internal/models"
	"iotserver/internal/pkg/constants"
	"iotserver/internal/pkg/logger"
	"iotserver/internal/pkg/tenant"
)

// platformTenantID 平台作用域写入使用的租户标识（空串，与 pkg/tenant 的约定一致）
const platformTenantID = ""

// 编译期断言：DuckDB 数据层必须实现这三个可选能力。
//   - MsgCounter：一条 SQL 统计消息总数，否则持久化层退回 N+1；
//   - DataPurger：按租户清理，否则应用层保留策略无法按租户生效；
//   - DatabaseExporter：在线导出快照，否则只能停服拷贝文件。
var (
	_ interfaces.MsgCounter       = (*Client)(nil)
	_ interfaces.DataPurger       = (*Client)(nil)
	_ interfaces.DatabaseExporter = (*Client)(nil)
)

// Client DuckDB 数据层实现
type Client struct {
	db   *sql.DB
	lc   logger.LoggingClient
	path string

	// 批量写入缓冲（见 batch.go）
	mu            sync.Mutex
	propBuf       []propRow
	eventBuf      []payloadRow
	svcBuf        []payloadRow
	flushRows     int
	flushInterval time.Duration
	// failStreak / nextFlushAt 落盘失败后的退避状态（见 batch.go 的 flush）
	failStreak  int
	nextFlushAt time.Time
	// maxBufferRows 缓冲区行数上限（默认 defaultMaxBufferRows）
	maxBufferRows int
	// writeMu 串行化写事务。DuckDB 是单写者引擎，官方语义下并发写事务
	// 冲突时会直接报错。实测当前驱动在本项目规模下并未出现冲突（32 写者 ×
	// 16000 行、零失败），但写入仍串行化：避免把「冲突即报错」当成常态来重试，
	// 也让批量落盘的顺序可预期。
	writeMu sync.Mutex

	// 保留策略
	retentionDays int

	// 后台维护生命周期
	stopCh chan struct{}
	wg     sync.WaitGroup
	closed bool
}

// GetDataDBType 返回数据层类型
func (c *Client) GetDataDBType() constants.DataType {
	return constants.DuckDB
}

// RetentionDays 当前保留天数（0 = 永久保留）
func (c *Client) RetentionDays() int { return c.retentionDays }

// DatabaseSize 返回数据库文件大小（字节）
func (c *Client) DatabaseSize() (int64, error) {
	info, err := os.Stat(c.path)
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}

// TableStats 返回某张表的行数、设备数与租户数（用于运维观测）
func (c *Client) TableStats(table string) string {
	var rows, devices, tenants int
	_ = c.db.QueryRow(fmt.Sprintf(
		"SELECT count(*), count(DISTINCT device_id), count(DISTINCT tenant_id) FROM %s", table)).
		Scan(&rows, &devices, &tenants)
	return fmt.Sprintf("rows=%d devices=%d tenants=%d", rows, devices, tenants)
}

// CloseSession 落盘缓冲、做一次 CHECKPOINT 并关闭连接
func (c *Client) CloseSession() {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return
	}
	c.closed = true
	c.mu.Unlock()

	// 停掉后台维护
	if c.stopCh != nil {
		close(c.stopCh)
		c.wg.Wait()
	}
	// 落盘剩余缓冲
	c.flushPending()
	// 把 WAL 合并进主文件，保证下次打开数据完整
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	if _, err := c.db.ExecContext(ctx, "CHECKPOINT"); err != nil {
		c.lc.Warnf("duckdb: checkpoint on close failed: %v", err)
	}
	_ = c.db.Close()
}

/** ***************************** 写入 ***************************** **/

// Insert 写入一批数据。
//
// table 约定：
//   - "<deviceId>"（或带 device_ 前缀）：写入设备属性，data 为 code -> 值
//   - "event:<deviceId>"：写入设备事件，data 为 eventCode -> 事件 JSON
//   - "service:<deviceId>"：写入服务调用，data 为 serviceCode -> 服务 JSON
//
// 租户从 ctx 读取（pkg/tenant）；平台作用域写入 tenant_id 为空串。
func (c *Client) Insert(ctx context.Context, table string, data map[string]interface{}) (err error) {
	tenantID := tenantIDOf(ctx)
	switch {
	case strings.HasPrefix(table, eventPrefix):
		return c.insertEvents(strings.TrimPrefix(table, eventPrefix), tenantID, data)
	case strings.HasPrefix(table, servicePrefix):
		return c.insertServices(strings.TrimPrefix(table, servicePrefix), tenantID, data)
	default:
		deviceID := strings.TrimPrefix(table, constants.DB_PREFIX)
		return c.insertProperties(deviceID, tenantID, data)
	}
}

// tenantIDOf 从上下文取租户标识；平台作用域或无租户上下文时返回空串
func tenantIDOf(ctx context.Context) string {
	tc := tenant.FromContext(ctx)
	if tc.Platform {
		return platformTenantID
	}
	return tc.TenantID
}

// insertProperties 属性入缓冲（攒批后由 flushPending 一次落盘）
func (c *Client) insertProperties(deviceID, tenantID string, data map[string]interface{}) error {
	if deviceID == "" {
		return fmt.Errorf("duckdb: deviceId is empty")
	}
	for code, value := range data {
		// 每个测点可以带自己的采集时间（断点续传补传历史数据）
		ts := nowMillis()
		if rd, ok := value.(dtos.ReportData); ok {
			if rd.Time > 0 {
				ts = rd.Time
			}
			value = rd.Value
		}
		num, text := splitValue(value)
		if err := c.enqueueProperty(propRow{
			tenantID: tenantID, deviceID: deviceID, code: code, ts: ts, num: num, text: text,
		}); err != nil {
			return err
		}
	}
	return nil
}

// insertEvents 事件入缓冲。每条事件用自己的时间，避免互相串号。
func (c *Client) insertEvents(deviceID, tenantID string, data map[string]interface{}) error {
	if deviceID == "" {
		return fmt.Errorf("duckdb: deviceId is empty")
	}
	for code, payload := range data {
		body, eventTime := marshalPayload(payload)
		ts := eventTime
		if ts <= 0 {
			ts = nowMillis()
		}
		if err := c.enqueueEvent(payloadRow{
			tenantID: tenantID, deviceID: deviceID, code: code, ts: ts, payload: body,
		}); err != nil {
			return err
		}
	}
	return nil
}

// insertServices 服务调用入缓冲
func (c *Client) insertServices(deviceID, tenantID string, data map[string]interface{}) error {
	if deviceID == "" {
		return fmt.Errorf("duckdb: deviceId is empty")
	}
	for code, payload := range data {
		body, eventTime := marshalPayload(payload)
		ts := eventTime
		if ts <= 0 {
			ts = nowMillis()
		}
		if err := c.enqueueService(payloadRow{
			tenantID: tenantID, deviceID: deviceID, code: code, ts: ts, payload: body,
		}); err != nil {
			return err
		}
	}
	return nil
}

/** ***************************** 查询 ***************************** **/

// GetDeviceProperty 查询设备属性历史
func (c *Client) GetDeviceProperty(req dtos.ThingModelPropertyDataRequest, device models.Device) ([]dtos.ReportData, int, error) {
	var response []dtos.ReportData
	if device.Id == "" {
		return response, 0, fmt.Errorf("duckdb: deviceId is nil")
	}
	// 保证「写入后立刻查询」能读到缓冲里的数据
	c.flushPending()

	start, end := timeWindow(req.Range, req.Last)
	where := "device_id = ? AND ts_ms >= ? AND ts_ms <= ?"
	args := []interface{}{device.Id, start, end}
	if req.Code != "" {
		where += " AND code = ?"
		args = append(args, req.Code)
	}

	// 聚合查询：交给 SQL 分桶聚合（分页作用于桶）
	if req.AggInterval > 0 {
		return c.aggregateProperty(req, where, args)
	}

	count, err := c.count(tableProperty, where, args)
	if err != nil {
		return response, 0, err
	}

	page, pageSize := normalizePage(req.Page, req.PageSize, req.IsAll)
	query := fmt.Sprintf(`SELECT ts_ms, num_value, text_value FROM %s
		WHERE %s ORDER BY ts_ms DESC LIMIT ? OFFSET ?`, tableProperty, where)
	args = append(args, pageSize, (page-1)*pageSize)

	rows, err := c.db.Query(query, args...)
	if err != nil {
		return response, count, err
	}
	defer rows.Close()

	for rows.Next() {
		var ts int64
		var num sql.NullFloat64
		var text sql.NullString
		if err = rows.Scan(&ts, &num, &text); err != nil {
			return response, count, err
		}
		item := dtos.ReportData{Time: ts}
		if num.Valid {
			item.Value = num.Float64
		} else if text.Valid {
			item.Value = text.String
		}
		response = append(response, item)
	}
	return response, count, rows.Err()
}

// GetDeviceEvent 查询设备事件历史
func (c *Client) GetDeviceEvent(req dtos.ThingModelEventDataRequest, device models.Device, product models.Product) ([]dtos.EventData, int, error) {
	var response []dtos.EventData
	if device.Id == "" {
		return response, 0, fmt.Errorf("duckdb: deviceId is nil")
	}
	c.flushPending()

	start, end := timeWindow(req.Range, req.Last)
	where := "device_id = ? AND ts_ms >= ? AND ts_ms <= ?"
	args := []interface{}{device.Id, start, end}
	if req.EventCode != "" {
		where += " AND event_code = ?"
		args = append(args, req.EventCode)
	}

	count, err := c.count(tableEvent, where, args)
	if err != nil {
		return response, 0, err
	}

	page, pageSize := normalizePage(req.Page, req.PageSize, req.IsAll)
	query := fmt.Sprintf(`SELECT event_code, ts_ms, payload FROM %s
		WHERE %s ORDER BY ts_ms DESC LIMIT ? OFFSET ?`, tableEvent, where)
	args = append(args, pageSize, (page-1)*pageSize)

	rows, err := c.db.Query(query, args...)
	if err != nil {
		return response, count, err
	}
	defer rows.Close()

	for rows.Next() {
		var code, payload string
		var ts int64
		if err = rows.Scan(&code, &ts, &payload); err != nil {
			return response, count, err
		}
		item := dtos.EventData{EventCode: code, EventTime: ts}
		unmarshalInto(payload, &item)
		item.EventCode = code
		item.EventTime = ts
		response = append(response, item)
	}
	return response, count, rows.Err()
}

// GetDeviceService 查询服务调用历史
func (c *Client) GetDeviceService(req dtos.ThingModelServiceDataRequest, device models.Device, product models.Product) ([]dtos.SaveServiceIssueData, int, error) {
	var response []dtos.SaveServiceIssueData
	if device.Id == "" {
		return response, 0, fmt.Errorf("duckdb: deviceId is nil")
	}
	c.flushPending()

	start, end := timeWindow(req.Range, req.Last)
	where := "device_id = ? AND ts_ms >= ? AND ts_ms <= ?"
	args := []interface{}{device.Id, start, end}
	if req.Code != "" {
		where += " AND code = ?"
		args = append(args, req.Code)
	}

	count, err := c.count(tableService, where, args)
	if err != nil {
		return response, 0, err
	}

	page, pageSize := normalizePage(req.Page, req.PageSize, req.IsAll)
	query := fmt.Sprintf(`SELECT code, ts_ms, payload FROM %s
		WHERE %s ORDER BY ts_ms DESC LIMIT ? OFFSET ?`, tableService, where)
	args = append(args, pageSize, (page-1)*pageSize)

	rows, err := c.db.Query(query, args...)
	if err != nil {
		return response, count, err
	}
	defer rows.Close()

	for rows.Next() {
		var code, payload string
		var ts int64
		if err = rows.Scan(&code, &ts, &payload); err != nil {
			return response, count, err
		}
		item := dtos.SaveServiceIssueData{Code: code, Time: ts}
		unmarshalInto(payload, &item)
		item.Code = code
		item.Time = ts
		response = append(response, item)
	}
	return response, count, rows.Err()
}

// GetDevicePropertyCount 属性条数
func (c *Client) GetDevicePropertyCount(req dtos.ThingModelPropertyDataRequest) (int, error) {
	// 与查询方法保持一致：先落盘，否则会漏计缓冲区里尚未写入的数据
	c.flushPending()
	start, end := timeWindow(req.Range, false)
	where := "device_id = ? AND ts_ms >= ? AND ts_ms <= ?"
	args := []interface{}{req.DeviceId, start, end}
	if req.Code != "" {
		where += " AND code = ?"
		args = append(args, req.Code)
	}
	return c.count(tableProperty, where, args)
}

// GetDeviceEventCount 事件条数
func (c *Client) GetDeviceEventCount(req dtos.ThingModelEventDataRequest) (int, error) {
	c.flushPending()
	start, end := timeWindow(req.Range, false)
	where := "device_id = ? AND ts_ms >= ? AND ts_ms <= ?"
	args := []interface{}{req.DeviceId, start, end}
	if req.EventCode != "" {
		where += " AND event_code = ?"
		args = append(args, req.EventCode)
	}
	return c.count(tableEvent, where, args)
}

// GetDeviceMsgCountByGiveTime 指定时间段内的设备消息数（属性 + 事件 + 服务）
func (c *Client) GetDeviceMsgCountByGiveTime(deviceId string, startTime, endTime int64) (int, error) {
	c.flushPending()
	total := 0
	for _, table := range []string{tableProperty, tableEvent, tableService} {
		n, err := c.count(table, "device_id = ? AND ts_ms >= ? AND ts_ms <= ?",
			[]interface{}{deviceId, startTime, endTime})
		if err != nil {
			return total, err
		}
		total += n
	}
	return total, nil
}

// CountMsgGroupByDevice 按设备分组统计时间窗内的消息数（属性 + 事件 + 服务）。
// 看板排行榜用：一次 GROUP BY 拿全量设备，避免逐设备统计。
func (c *Client) CountMsgGroupByDevice(tenantID string, startTime, endTime int64) (map[string]int, error) {
	c.flushPending()
	out := make(map[string]int)
	for _, table := range []string{tableProperty, tableEvent, tableService} {
		where := "ts_ms >= ? AND ts_ms <= ?"
		args := []interface{}{startTime, endTime}
		if tenantID != "" {
			where += " AND tenant_id = ?"
			args = append(args, tenantID)
		}
		rows, err := c.db.Query("SELECT device_id, count(*) FROM "+table+" WHERE "+where+" GROUP BY device_id", args...)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var deviceID string
			var n int
			if err := rows.Scan(&deviceID, &n); err != nil {
				rows.Close()
				return nil, err
			}
			out[deviceID] += n
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return nil, err
		}
		rows.Close()
	}
	return out, nil
}

// CountAllMsgByTimeRange 全平台指定时间段内的消息总数（一条 SQL，避免按设备 N+1 计数）
func (c *Client) CountAllMsgByTimeRange(tenantID string, startTime, endTime int64) (int, error) {
	c.flushPending()
	total := 0
	for _, table := range []string{tableProperty, tableEvent, tableService} {
		where := "ts_ms >= ? AND ts_ms <= ?"
		args := []interface{}{startTime, endTime}
		if tenantID != "" {
			where += " AND tenant_id = ?"
			args = append(args, tenantID)
		}
		n, err := c.count(table, where, args)
		if err != nil {
			return total, err
		}
		total += n
	}
	return total, nil
}

// count 统计满足条件的行数
func (c *Client) count(table, where string, args []interface{}) (int, error) {
	var n int
	query := fmt.Sprintf("SELECT count(*) FROM %s WHERE %s", table, where)
	if err := c.db.QueryRow(query, args...).Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}

/** ***************************** 表结构管理 ***************************** **/

// CreateTable 确保表存在（属性表为宽表，按需建表）
func (c *Client) CreateTable(ctx context.Context, stable, table string) error {
	return c.ensureTables(ctx)
}

// DropTable 删除设备数据（按 device_id 清理，避免误删整表）
func (c *Client) DropTable(ctx context.Context, table string) error {
	deviceID := strings.TrimPrefix(table, constants.DB_PREFIX)
	if deviceID == "" {
		return nil
	}
	// 必须先落盘：否则该设备还在缓冲里的数据会在删除之后被写回，删除等于无效
	c.flushPending()
	for _, t := range []string{tableProperty, tableEvent, tableService} {
		if _, err := c.db.ExecContext(normalizeCtx(ctx),
			fmt.Sprintf("DELETE FROM %s WHERE device_id = ?", t), deviceID); err != nil {
			return err
		}
	}
	return nil
}

// CreateStable 产品维度无需单独建表
func (c *Client) CreateStable(ctx context.Context, product models.Product) error { return nil }

// DropStable 产品维度无需单独删表。
//
// 这里是**有意为空**，不是漏实现：删除产品前应用层已强制校验该产品下没有子设备
// （productapp 的 ProductMustDeleteDevice），而时序数据按设备写入、子设备被删除时
// 已经通过 DropTable 清理过，所以走到这一步没有残留数据可清。
//
// 依赖的不变式：设备删除必须**先清时序数据、后删元数据**，否则会留下无法归属的
// 孤儿行（只能等保留期过期）。该顺序由 deviceapp.DeleteDeviceById 保证。
func (c *Client) DropStable(ctx context.Context, table string) error { return nil }

// AddDatabaseField 属性新增无需改表结构（code 为行值），仅确保表存在
func (c *Client) AddDatabaseField(ctx context.Context, tableName string, specsType constants.SpecsType, code string, name string) error {
	return c.ensureTables(ctx)
}

// DelDatabaseField 删除某设备某属性的历史数据
func (c *Client) DelDatabaseField(ctx context.Context, tableName, code string) error {
	deviceID := strings.TrimPrefix(tableName, constants.DB_PREFIX)
	if deviceID == "" {
		return nil
	}
	// 同 DropTable：先落盘，避免缓冲里的数据在删除后被写回
	c.flushPending()
	_, err := c.db.ExecContext(normalizeCtx(ctx),
		fmt.Sprintf("DELETE FROM %s WHERE device_id = ? AND code = ?", tableProperty), deviceID, code)
	return err
}

// ModifyDatabaseField 物模型修改不影响已存数据
func (c *Client) ModifyDatabaseField(ctx context.Context, tableName string, specsType constants.SpecsType, code string, name string) error {
	return nil
}

// ensureTables 建立三张表（幂等）
func (c *Client) ensureTables(ctx context.Context) error {
	ctx = normalizeCtx(ctx)
	stmts := []string{
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
			tenant_id VARCHAR, device_id VARCHAR, code VARCHAR,
			ts_ms BIGINT, num_value DOUBLE, text_value VARCHAR
		)`, tableProperty),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
			tenant_id VARCHAR, device_id VARCHAR, event_code VARCHAR,
			ts_ms BIGINT, payload VARCHAR
		)`, tableEvent),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
			tenant_id VARCHAR, device_id VARCHAR, code VARCHAR,
			ts_ms BIGINT, payload VARCHAR
		)`, tableService),
	}
	for _, stmt := range stmts {
		if _, err := c.db.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}

// Optimize 建可选索引，供运维在写入低峰期手动调用。
//
// DuckDB 主要依靠 row group zone map 裁剪，索引对区间扫描的收益有限（实测 1M 行量级
// 无明显差异），但设备点查较多时可以有帮助；代价是拖慢写入，所以不默认开启。
func (c *Client) Optimize(ctx context.Context) error {
	ctx = normalizeCtx(ctx)
	stmts := []string{
		fmt.Sprintf("CREATE INDEX IF NOT EXISTS idx_%s_device ON %s (device_id, code, ts_ms)", tableProperty, tableProperty),
		fmt.Sprintf("CREATE INDEX IF NOT EXISTS idx_%s_device ON %s (device_id, event_code, ts_ms)", tableEvent, tableEvent),
		fmt.Sprintf("CREATE INDEX IF NOT EXISTS idx_%s_device ON %s (device_id, code, ts_ms)", tableService, tableService),
	}
	for _, stmt := range stmts {
		if _, err := c.db.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}

/** ***************************** 在线备份 ***************************** **/

// ExportDatabase 把当前库导出为**一致的在线快照**：目录下会生成各表的 Parquet
// 数据文件与 schema.sql（用 DuckDB 官方 EXPORT DATABASE，无需停服）。
//
// 为什么不直接拷贝 .db 文件：数据库文件被本进程独占，运行中直接 copy 会拿到
// WAL 未合并、甚至写了一半的状态；本方法先 CHECKPOINT 落盘，再由 DuckDB 自己
// 保证导出视图一致。
func (c *Client) ExportDatabase(ctx context.Context, dir string) error {
	if strings.TrimSpace(dir) == "" {
		return fmt.Errorf("duckdb: 导出目录不能为空")
	}
	ctx = normalizeCtx(ctx)
	// 先把缓冲写下去，否则最近几秒的数据不在导出快照里
	c.flushPending()
	// 把 WAL 合并进主文件，保证导出读到的是完整状态
	if _, err := c.db.ExecContext(ctx, "CHECKPOINT"); err != nil {
		return fmt.Errorf("duckdb: checkpoint 失败: %w", err)
	}
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		return fmt.Errorf("duckdb: 创建导出目录 %s 失败: %w", dir, err)
	}
	if _, err := c.db.ExecContext(ctx,
		fmt.Sprintf("EXPORT DATABASE %s (FORMAT PARQUET)", quote(dir))); err != nil {
		return fmt.Errorf("duckdb: 导出数据库到 %s 失败: %w", dir, err)
	}
	return nil
}

/** ***************************** 构造 ***************************** **/

// NewClient 打开（必要时创建）DuckDB 数据库文件并建表
func NewClient(config dtos.Configuration, lc logger.LoggingClient) (c interfaces.DataDBClient, errEdgeX error) {
	dbPath := strings.TrimSpace(config.DataSource)
	if dbPath == "" {
		return nil, fmt.Errorf("duckdb: DataSource(database file path) is required")
	}
	if err := os.MkdirAll(filepath.Dir(dbPath), os.ModePerm); err != nil {
		return nil, fmt.Errorf("duckdb: create dir for %s: %w", dbPath, err)
	}

	db, err := sql.Open("duckdb", dbPath)
	if err != nil {
		return nil, fmt.Errorf("duckdb: open %s: %w", dbPath, err)
	}
	// DuckDB 是进程内 OLAP 引擎，连接数不宜放开；显式设上限避免高并发下连接失控
	db.SetMaxOpenConns(defaultMaxOpenConns)
	db.SetMaxIdleConns(defaultMaxOpenConns)
	db.SetConnMaxLifetime(0)

	// 保留天数语义：>0 按该天数；0 用默认值；<0 表示永久保留（关闭清理）
	retentionDays := config.RetentionDays
	switch {
	case retentionDays < 0:
		retentionDays = 0
	case retentionDays == 0:
		retentionDays = defaultRetentionDays
	}

	client := &Client{
		db:            db,
		lc:            lc,
		path:          dbPath,
		flushRows:     defaultFlushRows,
		flushInterval: defaultFlushInterval,
		maxBufferRows: defaultMaxBufferRows,
		retentionDays: retentionDays,
		stopCh:        make(chan struct{}),
	}

	if err = client.ensureTables(context.Background()); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("duckdb: create tables: %w", err)
	}

	client.startFlusher()

	size, _ := client.DatabaseSize()
	// retention 提示：实际清理由应用层按租户执行（见 application/retention.go）
	lc.Infof("duckdb: opened %s (size=%d bytes, batch=%d rows/%s, 全局保留期提示=%d 天)",
		dbPath, size, client.flushRows, client.flushInterval, client.retentionDays)
	return client, nil
}
