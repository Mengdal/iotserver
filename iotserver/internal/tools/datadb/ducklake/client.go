// Package ducklake 实现基于 DuckLake（DuckDB + Parquet 湖仓）的数据层。
//
// 配置约定（configuration.toml 的 [Databases.Data.Primary]）：
//
//	Type       = 'ducklake'
//	DataSource = 'manifest/docker/db-data/ducklake/catalog.db'   # 元数据（SQLite catalog）
//	Dsn        = 'manifest/docker/db-data/ducklake/data'          # 数据文件（Parquet）目录
//
// 说明：
//   - 官方 Go 驱动 github.com/duckdb/duckdb-go/v2 依赖 CGO，且要求 Go >= 1.24；
//   - ducklake 扩展不在 DuckDB 内置列表中，需要在可写目录 INSTALL（首次需联网），
//     扩展目录固定为 catalog 同级的 extensions/，便于离线部署时预置；
//   - 时间列统一用毫秒时间戳（BIGINT）保存，避免 TIMESTAMP 的时区/精度歧义，
//     区间查询即整数比较。
package ducklake

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
)

const (
	lakeAlias = "lake"

	tableProperty = "device_property"
	tableEvent    = "device_event"
	tableService  = "device_service"

	// Insert 的 table 参数约定
	eventPrefix   = "event:"
	servicePrefix = "service:"

	defaultPageSize = 10
	maxPageSize     = 1000
)

// Client DuckLake 数据层实现
type Client struct {
	db          *sql.DB
	lc          logger.LoggingClient
	catalogPath string
	dataPath    string

	// 批量写入缓冲（见 batch.go）
	mu            sync.Mutex
	propBuf       []propRow
	eventBuf      []payloadRow
	svcBuf        []payloadRow
	flushRows     int
	flushInterval time.Duration

	// 保留策略
	retentionDays int

	// 后台维护生命周期
	stopCh chan struct{}
	wg     sync.WaitGroup
	closed bool
}

// GetDataDBType 返回数据层类型
func (c *Client) GetDataDBType() constants.DataType {
	return constants.DuckLake
}

// CloseSession 关闭连接。
// 退出前尽力把「内联在元数据里」的小批量数据物化成 Parquet，
// 这样湖仓形态才是完整的（DuckLake 会先把小写入内联进 catalog）。
func (c *Client) CloseSession() {
	if c.db == nil || c.closed {
		return
	}
	c.closed = true
	// 停掉后台维护
	if c.stopCh != nil {
		close(c.stopCh)
		c.wg.Wait()
	}
	// 落盘剩余缓冲，再物化成 Parquet
	c.flushPending()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	if err := c.FlushInlinedData(ctx); err != nil {
		c.lc.Warnf("ducklake: flush inlined data on close failed: %v", err)
	}
	_ = c.db.Close()
}

// FlushInlinedData 把内联数据物化为 Parquet 数据文件（可重复调用，幂等）。
// DuckLake 会先把小批量写入内联进 catalog 元数据，物化后才是标准湖仓形态。
func (c *Client) FlushInlinedData(ctx context.Context) error {
	_, err := c.db.ExecContext(normalizeCtx(ctx),
		fmt.Sprintf("CALL ducklake_flush_inlined_data(%s)", quote(lakeAlias)))
	return err
}

// SnapshotCount 当前快照数（每次写入事务产生一个快照）
func (c *Client) SnapshotCount() (int, error) {
	var n int
	err := c.db.QueryRow(fmt.Sprintf("SELECT count(*) FROM %s.snapshots()", lakeAlias)).Scan(&n)
	return n, err
}

// TableStats 返回某张表的行数与设备数（用于运维观测）
func (c *Client) TableStats(table string) string {
	var rows, devices int
	_ = c.db.QueryRow(fmt.Sprintf("SELECT count(*), count(DISTINCT device_id) FROM %s.%s", lakeAlias, table)).
		Scan(&rows, &devices)
	return fmt.Sprintf("rows=%d devices=%d", rows, devices)
}

// DataFileCount 统计 DATA_PATH 下的 Parquet 数据文件数量（用于观测物化效果）
func (c *Client) DataFileCount() (int, error) {
	n := 0
	err := filepath.Walk(c.dataPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(info.Name(), ".parquet") {
			n++
		}
		return nil
	})
	return n, err
}

/** ***************************** 写入 ***************************** **/

// Insert 写入一行数据。
//
// table 约定：
//   - "<deviceId>"（或带 device_ 前缀）：写入设备属性，data 为 code -> 值
//   - "event:<deviceId>"：写入设备事件，data 为 eventCode -> 事件 JSON
//   - "service:<deviceId>"：写入服务调用，data 为 serviceCode -> 服务 JSON
//
// 每次 Insert 使用同一个毫秒时间戳（与上报批次一致）。
func (c *Client) Insert(ctx context.Context, table string, data map[string]interface{}) (err error) {
	switch {
	case strings.HasPrefix(table, eventPrefix):
		return c.insertEvents(ctx, strings.TrimPrefix(table, eventPrefix), data)
	case strings.HasPrefix(table, servicePrefix):
		return c.insertServices(ctx, strings.TrimPrefix(table, servicePrefix), data)
	default:
		deviceId := strings.TrimPrefix(table, constants.DB_PREFIX)
		return c.insertProperties(ctx, deviceId, data, dtos.ReportData{})
	}
}

// insertProperties 属性入缓冲（攒批后由 flushPending 一次落盘）
func (c *Client) insertProperties(ctx context.Context, deviceId string, data map[string]interface{}, meta dtos.ReportData) error {
	if deviceId == "" {
		return fmt.Errorf("ducklake: deviceId is empty")
	}
	for code, value := range data {
		ts := meta.Time
		if ts == 0 {
			if rd, ok := value.(dtos.ReportData); ok && rd.Time > 0 {
				ts = rd.Time
				value = rd.Value
			} else {
				ts = nowMillis()
			}
		}
		num, text := splitValue(value)
		c.enqueueProperty(propRow{deviceId: deviceId, code: code, ts: ts, num: num, text: text})
	}
	return nil
}

func (c *Client) insertEvents(ctx context.Context, deviceId string, data map[string]interface{}) error {
	if deviceId == "" {
		return fmt.Errorf("ducklake: deviceId is empty")
	}
	ts := nowMillis()
	for code, payload := range data {
		body, eventTime := marshalPayload(payload)
		if eventTime > 0 {
			ts = eventTime
		}
		c.enqueueEvent(payloadRow{deviceId: deviceId, code: code, ts: ts, payload: body})
	}
	return nil
}

func (c *Client) insertServices(ctx context.Context, deviceId string, data map[string]interface{}) error {
	if deviceId == "" {
		return fmt.Errorf("ducklake: deviceId is empty")
	}
	ts := nowMillis()
	for code, payload := range data {
		body, eventTime := marshalPayload(payload)
		if eventTime > 0 {
			ts = eventTime
		}
		c.enqueueService(payloadRow{deviceId: deviceId, code: code, ts: ts, payload: body})
	}
	return nil
}

/** ***************************** 查询 ***************************** **/

// GetDeviceProperty 查询设备属性历史（按时间倒序分页）
func (c *Client) GetDeviceProperty(req dtos.ThingModelPropertyDataRequest, device models.Device) ([]dtos.ReportData, int, error) {
	var response []dtos.ReportData
	if device.Id == "" {
		return response, 0, fmt.Errorf("ducklake: deviceId is nil")
	}
	// 读之前先把缓冲落盘，保证「写入后立刻查询」能读到
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
	query := fmt.Sprintf(`SELECT ts_ms, num_value, text_value FROM %s.%s
		WHERE %s ORDER BY ts_ms DESC LIMIT ? OFFSET ?`, lakeAlias, tableProperty, where)
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
		return response, 0, fmt.Errorf("ducklake: deviceId is nil")
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
	query := fmt.Sprintf(`SELECT event_code, ts_ms, payload FROM %s.%s
		WHERE %s ORDER BY ts_ms DESC LIMIT ? OFFSET ?`, lakeAlias, tableEvent, where)
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
		return response, 0, fmt.Errorf("ducklake: deviceId is nil")
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
	query := fmt.Sprintf(`SELECT code, ts_ms, payload FROM %s.%s
		WHERE %s ORDER BY ts_ms DESC LIMIT ? OFFSET ?`, lakeAlias, tableService, where)
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

/** ***************************** 表结构管理 ***************************** **/

// CreateTable 建表（lake 内按需建属性表，事件/服务表已随连接初始化）
func (c *Client) CreateTable(ctx context.Context, stable, table string) error {
	return c.ensureTables(ctx)
}

// DropTable 删除设备数据（按 device_id 清理，避免误删整表）
func (c *Client) DropTable(ctx context.Context, table string) error {
	deviceId := strings.TrimPrefix(table, constants.DB_PREFIX)
	if deviceId == "" {
		return nil
	}
	for _, t := range []string{tableProperty, tableEvent, tableService} {
		if _, err := c.db.ExecContext(ctx,
			fmt.Sprintf("DELETE FROM %s.%s WHERE device_id = ?", lakeAlias, t), deviceId); err != nil {
			return err
		}
	}
	return nil
}

// CreateStable 产品维度无需单独建表
func (c *Client) CreateStable(ctx context.Context, product models.Product) error {
	return nil
}

// DropStable 产品维度无需单独删表
func (c *Client) DropStable(ctx context.Context, table string) error {
	return nil
}

// AddDatabaseField 属性新增时无需改表结构（code 为行值），仅确保表存在
func (c *Client) AddDatabaseField(ctx context.Context, tableName string, specsType constants.SpecsType, code string, name string) error {
	return c.ensureTables(ctx)
}

// DelDatabaseField 删除某设备某属性的历史数据
func (c *Client) DelDatabaseField(ctx context.Context, tableName, code string) error {
	deviceId := strings.TrimPrefix(tableName, constants.DB_PREFIX)
	if deviceId == "" {
		return nil
	}
	_, err := c.db.ExecContext(ctx,
		fmt.Sprintf("DELETE FROM %s.%s WHERE device_id = ? AND code = ?", lakeAlias, tableProperty),
		deviceId, code)
	return err
}

// ModifyDatabaseField 物模型修改不影响已存数据
func (c *Client) ModifyDatabaseField(ctx context.Context, tableName string, specsType constants.SpecsType, code string, name string) error {
	return nil
}

/** ***************************** 内部工具 ***************************** **/

// ensureTables 建立 lake 内的三张表
func (c *Client) ensureTables(ctx context.Context) error {
	ctx = normalizeCtx(ctx)
	stmts := []string{
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.%s (
			device_id VARCHAR, code VARCHAR, ts_ms BIGINT, num_value DOUBLE, text_value VARCHAR
		)`, lakeAlias, tableProperty),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.%s (
			device_id VARCHAR, event_code VARCHAR, ts_ms BIGINT, payload VARCHAR
		)`, lakeAlias, tableEvent),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.%s (
			device_id VARCHAR, code VARCHAR, ts_ms BIGINT, payload VARCHAR
		)`, lakeAlias, tableService),
	}
	for _, stmt := range stmts {
		if _, err := c.db.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}

func (c *Client) count(table, where string, args []interface{}) (int, error) {
	var n int
	query := fmt.Sprintf("SELECT count(*) FROM %s.%s WHERE %s", lakeAlias, table, where)
	if err := c.db.QueryRow(query, args...).Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}

// NewClient 建立 DuckLake 连接：装/载扩展 → ATTACH 湖 → 建表
func NewClient(config dtos.Configuration, lc logger.LoggingClient) (c interfaces.DataDBClient, errEdgeX error) {
	catalogPath := strings.TrimSpace(config.DataSource)
	if catalogPath == "" {
		return nil, fmt.Errorf("ducklake: DataSource(catalog path) is required")
	}
	dataPath := strings.TrimSpace(config.Dsn)
	baseDir := filepath.Dir(catalogPath)
	if dataPath == "" {
		dataPath = filepath.Join(baseDir, "data")
	}
	extDir := filepath.Join(baseDir, "extensions")

	for _, dir := range []string{baseDir, dataPath, extDir} {
		if err := os.MkdirAll(dir, os.ModePerm); err != nil {
			return nil, fmt.Errorf("ducklake: create dir %s: %w", dir, err)
		}
	}

	db, err := sql.Open("duckdb", "")
	if err != nil {
		return nil, fmt.Errorf("ducklake: open duckdb: %w", err)
	}
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
		catalogPath:   catalogPath,
		dataPath:      dataPath,
		flushRows:     defaultFlushRows,
		flushInterval: defaultFlushInterval,
		retentionDays: retentionDays,
		stopCh:        make(chan struct{}),
	}

	// 扩展目录固定，便于内网离线预置
	if _, err = db.Exec("SET extension_directory = " + quote(extDir)); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ducklake: set extension_directory: %w", err)
	}
	if _, err = db.Exec("INSTALL ducklake"); err != nil {
		// 已安装或离线预置时允许继续，由随后的 LOAD 判定
		lc.Warnf("ducklake: INSTALL ducklake failed (will try LOAD): %v", err)
	}
	if _, err = db.Exec("LOAD ducklake"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ducklake: LOAD ducklake failed (需要可访问的 ducklake 扩展): %w", err)
	}

	// DATA_PATH 会被 ducklake 记进 catalog；同一个湖换个路径字符串访问（容器 vs 宿主机本地）
	// 会被拒绝，这时用 OverrideDataPath 显式覆盖（两种写法必须指向同一个物理目录）
	override := ""
	if config.OverrideDataPath {
		override = ", OVERRIDE_DATA_PATH true"
	}
	attach := fmt.Sprintf("ATTACH %s AS %s (DATA_PATH %s%s)",
		quote("ducklake:"+catalogPath), lakeAlias, quote(dataPath), override)
	if _, err = db.Exec(attach); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ducklake: attach %s: %w", catalogPath, err)
	}
	if err = client.ensureTables(context.Background()); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ducklake: create tables: %w", err)
	}

	// 后台按时间间隔落盘（即使没攒够行数）
	client.startFlusher()
	// 保留策略（retentionDays > 0 时生效）
	client.startMaintenance()

	lc.Infof("ducklake: lake attached, catalog=%s data=%s (batch=%d rows/%s, retention=%d days)",
		catalogPath, dataPath, client.flushRows, client.flushInterval, client.retentionDays)
	return client, nil
}

// quote 生成 SQL 字符串字面量（转义单引号）
func quote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}
