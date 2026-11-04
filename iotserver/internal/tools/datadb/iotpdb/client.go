// Package iotpdb 把外部 iotp 时序库（HTTP API）适配成平台的 DataDBClient。
//
// 数据模型差异：平台按「设备 + 测点」写长表，iotp 是「一个产品一张宽表、
// 一行一个时间戳、一个测点一列」。本包负责宽窄转换、schema/设备实例的惰性补齐，
// 以及把平台查询翻译成 iotp 的标准 SQL（POST /api/v1/query）。
package iotpdb

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	interfaces "iotserver/internal/iotserver/core/interface"
	"iotserver/internal/models"
	"iotserver/internal/pkg/constants"
	"iotserver/internal/pkg/logger"
	"iotserver/internal/pkg/tenant"
	"iotserver/internal/tools/iotp"
)

const (
	defaultFlushRows     = 500
	defaultFlushInterval = 500 * time.Millisecond
	defaultMaxPending    = 20000
	defaultDeviceTTL     = 10 * time.Minute
	writeTimeout         = 15 * time.Second
	readTimeout          = 20 * time.Second

	defaultPageSize = 10
	maxPageSize     = 1000

	eventFieldPrefix   = "evt_"
	serviceFieldPrefix = "act_"
)

// errBufferFull 缓冲区满（iotp 不可用且积压过多）时拒绝写入，避免内存无界增长。
var errBufferFull = errors.New("iotp 写入缓冲区已满")

type pendingRow struct {
	table    string
	deviceID int64
	ts       int64
	fields   map[string]any
}

type cachedDevice struct {
	at         time.Time
	table      string
	iotpID     int64
	tenantID   string
	name       string
	tags       map[string]string
	registered bool
}

type deviceTarget struct {
	table    string
	iotpID   int64
	tenantID string
	name     string
	tags     map[string]string
}

// Client iotp 数据层适配器。
type Client struct {
	api  *iotp.Client
	meta interfaces.DBClient
	lc   logger.LoggingClient
	db   string

	flushRows     int
	flushInterval time.Duration
	maxPending    int

	mu       sync.Mutex
	pending  []pendingRow
	stopping bool
	stopCh   chan struct{}
	stopOnce sync.Once

	devMu   sync.Mutex
	devices map[string]cachedDevice

	schemaMu sync.Mutex
	schemas  map[string]map[string]string // table -> field -> iotp 类型
	ensured  map[string]bool              // table -> 基础 schema 已确保
	loaded   map[string]bool              // table -> 已从 iotp 拉取过完整字段

	tablesMu sync.Mutex
	tablesAt time.Time
	tables   []string
}

// New 构造适配器并启动后台 flush。
//
// meta 用于把平台设备 ID 解析为「产品表 + 设备实例标签」；api 已完成认证配置。
func New(api *iotp.Client, meta interfaces.DBClient, lc logger.LoggingClient) *Client {
	c := newClient(api, meta, lc, true)
	return c
}

// newClient 构造适配器；startLoop=false 供单测关闭后台定时 flush。
func newClient(api *iotp.Client, meta interfaces.DBClient, lc logger.LoggingClient, startLoop bool) *Client {
	c := &Client{
		api:           api,
		meta:          meta,
		lc:            lc,
		db:            api.Database(),
		flushRows:     defaultFlushRows,
		flushInterval: defaultFlushInterval,
		maxPending:    defaultMaxPending,
		stopCh:        make(chan struct{}),
		devices:       make(map[string]cachedDevice),
		schemas:       make(map[string]map[string]string),
		ensured:       make(map[string]bool),
		loaded:        make(map[string]bool),
	}
	if startLoop {
		go c.loop()
	}
	return c
}

// GetDataDBType 返回后端标识。
func (c *Client) GetDataDBType() constants.DataType { return constants.Iotp }

// CloseSession 停止后台 flush 并把缓冲区写完。
func (c *Client) CloseSession() {
	c.stopOnce.Do(func() {
		c.mu.Lock()
		c.stopping = true
		c.mu.Unlock()
		close(c.stopCh)
	})
	if err := c.flush(); err != nil {
		c.warnf("iotp: 关闭前落盘失败: %v", err)
	}
}

func (c *Client) loop() {
	ticker := time.NewTicker(c.flushInterval)
	defer ticker.Stop()
	for {
		select {
		case <-c.stopCh:
			return
		case <-ticker.C:
			if err := c.flush(); err != nil {
				c.warnf("iotp: 定时写入失败: %v", err)
			}
		}
	}
}

/* ------------------------------ DDL ------------------------------ */

// CreateTable 设备创建时调用：stable 是产品 ID，table 是设备 ID。
func (c *Client) CreateTable(ctx context.Context, stable, table string) error {
	if strings.TrimSpace(stable) == "" {
		return nil
	}
	return c.ensureBase(ctx, iotp.TableName(stable))
}

// DropTable 设备删除：iotp 没有按设备删数据的接口，保留历史。
func (c *Client) DropTable(_ context.Context, table string) error {
	c.debugf("iotp: 数据层不支持按设备删除，保留历史（device=%s）", table)
	return nil
}

// CreateStable 产品发布时确保表与基础标签字段存在。
func (c *Client) CreateStable(ctx context.Context, product models.Product) error {
	if strings.TrimSpace(product.Id) == "" {
		return nil
	}
	return c.ensureBase(ctx, iotp.TableName(product.Id))
}

// DropStable 产品删除：iotp 表与历史保留。
func (c *Client) DropStable(_ context.Context, table string) error {
	c.debugf("iotp: 数据层不支持删除产品表，保留历史（product=%s）", table)
	return nil
}

// AddDatabaseField 物模型属性新增：确保表并补字段。
func (c *Client) AddDatabaseField(ctx context.Context, tableName string, specsType constants.SpecsType, code string, name string) error {
	if strings.TrimSpace(tableName) == "" || strings.TrimSpace(code) == "" {
		return nil
	}
	// 事件/服务调用时不带类型（specsType 为空）；它们的字段是 evt_/act_ 前缀，
	// 由 iotpsync 负责，避免这里再建一个无前缀的同名列。
	if specsType == "" {
		return nil
	}
	table := iotp.TableName(tableName)
	if err := c.ensureBase(ctx, table); err != nil {
		return err
	}
	return c.addField(ctx, table, iotp.Field{Name: iotp.FieldName(code), Type: iotp.FieldType(specsType)})
}

// DelDatabaseField 物模型属性删除：iotp 没有删数据接口，保留历史列。
func (c *Client) DelDatabaseField(_ context.Context, tableName, code string) error {
	c.debugf("iotp: 数据层不支持删除字段数据，保留历史（product=%s code=%s）", tableName, code)
	return nil
}

// ModifyDatabaseField 物模型属性修改：尝试补字段（类型变更在有数据时会被 iotp 拒绝）。
func (c *Client) ModifyDatabaseField(ctx context.Context, tableName string, specsType constants.SpecsType, code string, name string) error {
	if err := c.AddDatabaseField(ctx, tableName, specsType, code, name); err != nil {
		c.warnf("iotp: 字段 %s 类型变更失败（有数据时 iotp 会拒绝）: %v", code, err)
	}
	return nil
}

/* ------------------------------ 内部工具 ------------------------------ */

func (c *Client) ensureBase(ctx context.Context, table string) error {
	c.schemaMu.Lock()
	done := c.ensured[table]
	c.schemaMu.Unlock()
	if done {
		return nil
	}
	if err := c.api.EnsureDatabase(ctx); err != nil {
		return err
	}
	base := iotp.BaseFields()
	if err := c.api.EnsureTable(ctx, table, base); err != nil {
		return err
	}
	c.schemaMu.Lock()
	if c.schemas[table] == nil {
		c.schemas[table] = make(map[string]string, len(base))
	}
	for _, field := range base {
		c.schemas[table][field.Name] = field.Type
	}
	c.ensured[table] = true
	c.schemaMu.Unlock()
	return nil
}

func (c *Client) addField(ctx context.Context, table string, field iotp.Field) error {
	err := c.api.AddField(ctx, table, field)
	if err != nil && !iotp.IsConflict(err) {
		return err
	}
	return nil
}

// resolve 把平台设备 ID 解析成 iotp 的表、设备实例 ID 与标签（带缓存）。
func (c *Client) resolve(ctx context.Context, deviceID string) (deviceTarget, error) {
	c.devMu.Lock()
	cached, ok := c.devices[deviceID]
	c.devMu.Unlock()
	if ok && time.Since(cached.at) < defaultDeviceTTL {
		return deviceTarget{table: cached.table, iotpID: cached.iotpID, tenantID: cached.tenantID, name: cached.name, tags: cached.tags}, nil
	}

	if c.meta == nil {
		return deviceTarget{}, fmt.Errorf("iotp: 缺少元数据库客户端，无法解析设备 %s", deviceID)
	}
	device, err := c.meta.WithCtx(tenant.Platform(ctx)).DeviceById(deviceID)
	if err != nil {
		return deviceTarget{}, err
	}
	table := iotp.DefaultTable
	if strings.TrimSpace(device.ProductId) != "" {
		table = iotp.TableName(device.ProductId)
	}
	key := strings.TrimSpace(device.DeviceKey)
	if key == "" {
		key = strings.TrimSpace(device.DeviceSn)
	}
	if key == "" {
		key = device.Id
	}
	target := deviceTarget{
		table:    table,
		iotpID:   iotp.DeviceID(device.Id),
		tenantID: device.TenantId,
		name:     device.Name,
		tags: map[string]string{
			"tenant_id":  device.TenantId,
			"product_id": device.ProductId,
			"device_sn":  device.DeviceSn,
			"device_key": key,
			"dept_id":    device.DeptId,
		},
	}
	c.devMu.Lock()
	entry := cachedDevice{at: time.Now(), table: table, iotpID: target.iotpID, tenantID: target.tenantID, name: target.name, tags: target.tags}
	if old, exists := c.devices[deviceID]; exists {
		entry.registered = old.registered
	}
	c.devices[deviceID] = entry
	c.devMu.Unlock()
	return target, nil
}

// markRegistered 记录设备实例已注册，避免每包都调元数据接口。
func (c *Client) markRegistered(deviceID string) {
	c.devMu.Lock()
	entry, ok := c.devices[deviceID]
	if ok {
		entry.registered = true
		c.devices[deviceID] = entry
	}
	c.devMu.Unlock()
}

func (c *Client) isRegistered(deviceID string) bool {
	c.devMu.Lock()
	defer c.devMu.Unlock()
	entry, ok := c.devices[deviceID]
	return ok && entry.registered
}

func (c *Client) cachedByIotpID(id int64) (cachedDevice, bool) {
	c.devMu.Lock()
	defer c.devMu.Unlock()
	for _, entry := range c.devices {
		if entry.iotpID == id {
			return entry, true
		}
	}
	return cachedDevice{}, false
}

// tableRef 生成 SQL 表引用 "db"."table"。
func (c *Client) tableRef(table string) string {
	return quoteIdent(c.db) + "." + quoteIdent(table)
}

func quoteIdent(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

func nowMillis() int64 {
	return time.Now().UnixNano() / int64(time.Millisecond)
}

func (c *Client) infof(format string, args ...interface{}) {
	if c.lc != nil {
		c.lc.Infof(format, args...)
	}
}

func (c *Client) warnf(format string, args ...interface{}) {
	if c.lc != nil {
		c.lc.Warnf(format, args...)
	}
}

func (c *Client) debugf(format string, args ...interface{}) {
	if c.lc != nil {
		c.lc.Debugf(format, args...)
	}
}

// 保证编译期满足接口（可选能力由编译期断言单独检查）。
var _ interfaces.DataDBClient = (*Client)(nil)
