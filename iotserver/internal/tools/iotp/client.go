// Package iotp 封装 iotp 时序库的元数据 API。
//
// 覆盖平台需要的「建库 / 建表 / 字段 / 设备」写接口，以及设备最新值的实时查询。
// 文档见 iotp 控制台的「API 文档」页（/api/v1/meta/...、/api/v1/realtime/...）。
package iotp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	TypeI64    = "I64"
	TypeF64    = "F64"
	TypeBool   = "Bool"
	TypeString = "String"

	// DerivedDeviceIDStart iotp 为聚合（rollup）派生设备保留的 ID 区间起点（2^40）。
	// 源设备（真实写入的设备实例）ID 必须小于该值，否则写入会被 400 拒绝。
	DerivedDeviceIDStart = int64(1) << 40
)

// Field 表字段。iotp 只有 I64 / F64 / Bool / String 四种类型。
type Field struct {
	Name  string `json:"name"`
	Type  string `json:"type"`
	IsTag bool   `json:"is_tag"`
	Attrs any    `json:"attrs,omitempty"`
}

// Device 设备实例。device_id 必须是正整数；tags 在注册时写入，之后写入数据只带 device_id。
type Device struct {
	DeviceID int64             `json:"device_id,omitempty"`
	Name     string            `json:"name,omitempty"`
	Attrs    map[string]any    `json:"attrs,omitempty"`
	Tags     map[string]string `json:"tags,omitempty"`
}

// RealtimeField 设备某个测量字段的最新值。
type RealtimeField struct {
	Name  string `json:"name"`
	Type  string `json:"type"`
	Value any    `json:"value"`
	Time  int64  `json:"time"`
}

// RealtimeDevice 设备实时值；从未上报时 Fields 为空、UpdatedAt 为 nil。
type RealtimeDevice struct {
	Id        int64           `json:"id"`
	Name      string          `json:"name"`
	Fields    []RealtimeField `json:"fields"`
	UpdatedAt *int64          `json:"updated_at"`
}

// APIError iotp 返回的非 2xx 错误。
type APIError struct {
	Status  int
	Code    string
	Message string
}

func (e *APIError) Error() string {
	if e.Code == "" {
		return fmt.Sprintf("iotp http %d: %s", e.Status, e.Message)
	}
	return fmt.Sprintf("iotp http %d (%s): %s", e.Status, e.Code, e.Message)
}

// IsConflict 409：资源已存在 / 有数据后改字段类型。
func IsConflict(err error) bool {
	if apiErr, ok := err.(*APIError); ok {
		return apiErr.Status == http.StatusConflict
	}
	return false
}

// IsNotFound 404：资源不存在。
func IsNotFound(err error) bool {
	if apiErr, ok := err.(*APIError); ok {
		return apiErr.Status == http.StatusNotFound
	}
	return false
}

// IsAlreadyExists 兼容把重复当作 400 返回的部署。
func IsAlreadyExists(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "exist") || strings.Contains(msg, "已存在")
}

// Client iotp 元数据客户端。
type Client struct {
	baseURL string
	token   string
	db      string
	http    *http.Client
}

func New(baseURL, token, database string, timeout time.Duration) *Client {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	if strings.TrimSpace(database) == "" {
		database = "iotserver"
	}
	return &Client{
		baseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		token:   strings.TrimSpace(token),
		db:      strings.TrimSpace(database),
		http:    &http.Client{Timeout: timeout},
	}
}

func (c *Client) Database() string {
	return c.db
}

// Realtime 查询设备所有测量字段的最新值。
// iotp 返回的 time/updated_at 是纳秒，这里统一转换为平台使用的毫秒。
func (c *Client) Realtime(ctx context.Context, deviceID int64) (RealtimeDevice, error) {
	var result RealtimeDevice
	err := c.do(ctx, http.MethodGet, fmt.Sprintf("/api/v1/realtime/%d", deviceID), nil, &result)
	if err != nil {
		return result, err
	}
	for i := range result.Fields {
		result.Fields[i].Time = NanosToMillis(result.Fields[i].Time)
	}
	if result.UpdatedAt != nil {
		updatedAt := NanosToMillis(*result.UpdatedAt)
		result.UpdatedAt = &updatedAt
	}
	return result, nil
}

func (c *Client) do(ctx context.Context, method, path string, body any, out any) error {
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		apiErr := &APIError{Status: resp.StatusCode, Message: strings.TrimSpace(string(raw))}
		var payload struct {
			Error string `json:"error"`
			Code  string `json:"code"`
		}
		if json.Unmarshal(raw, &payload) == nil {
			if payload.Error != "" {
				apiErr.Message = payload.Error
			}
			apiErr.Code = payload.Code
		}
		return apiErr
	}
	if out != nil && len(raw) > 0 {
		if err := json.Unmarshal(raw, out); err != nil {
			return fmt.Errorf("iotp decode response: %w", err)
		}
	}
	return nil
}

// EnsureDatabase 确保库存在；已存在视为成功。
func (c *Client) EnsureDatabase(ctx context.Context) error {
	err := c.do(ctx, http.MethodPost, "/api/v1/meta/databases", map[string]string{"name": c.db}, nil)
	if err == nil || IsConflict(err) || IsAlreadyExists(err) {
		return nil
	}
	return err
}

// EnsureTable 确保表存在并补齐字段；表已存在时逐字段 PUT。
func (c *Client) EnsureTable(ctx context.Context, table string, fields []Field) error {
	payload := map[string]any{"db": c.db, "table": table, "fields": fields}
	err := c.do(ctx, http.MethodPost, "/api/v1/meta/tables", payload, nil)
	if err == nil {
		return nil
	}
	if !IsConflict(err) && !IsAlreadyExists(err) {
		return err
	}
	for _, field := range fields {
		if fieldErr := c.AddField(ctx, table, field); fieldErr != nil && !IsConflict(fieldErr) {
			return fieldErr
		}
	}
	return nil
}

// AddField 新增字段或更新字段类型/attrs。
//
// iotp 的语义：字段不存在则新增；存在且有数据时改类型返回 409。调用方决定是否降级。
func (c *Client) AddField(ctx context.Context, table string, field Field) error {
	path := fmt.Sprintf("/api/v1/meta/tables/%s/%s/fields",
		url.PathEscape(c.db), url.PathEscape(table))
	return c.do(ctx, http.MethodPut, path, field, nil)
}

// RegisterDevice 注册设备；已存在时更新 name/attrs/tags。
func (c *Client) RegisterDevice(ctx context.Context, table string, device Device) error {
	payload := map[string]any{
		"db":        c.db,
		"table":     table,
		"device_id": device.DeviceID,
		"name":      device.Name,
		"tags":      device.Tags,
	}
	if device.Attrs != nil {
		payload["attrs"] = device.Attrs
	}
	err := c.do(ctx, http.MethodPost, "/api/v1/meta/devices", payload, nil)
	if err == nil {
		return nil
	}
	if !IsConflict(err) {
		return err
	}
	if device.DeviceID <= 0 {
		return err
	}
	// 已存在：PUT 整体替换标签值（未提供的声明字段置 NULL）。
	update := map[string]any{"name": device.Name, "tags": device.Tags}
	if device.Attrs != nil {
		update["attrs"] = device.Attrs
	}
	path := fmt.Sprintf("/api/v1/meta/devices/%d", device.DeviceID)
	return c.do(ctx, http.MethodPut, path, update, nil)
}

// DeleteDevice 删除设备；不存在视为成功。
func (c *Client) DeleteDevice(ctx context.Context, deviceID int64) error {
	if deviceID <= 0 {
		return nil
	}
	path := fmt.Sprintf("/api/v1/meta/devices/%d", deviceID)
	err := c.do(ctx, http.MethodDelete, path, nil, nil)
	if err == nil || IsNotFound(err) {
		return nil
	}
	return err
}

// DeviceID 把平台设备 ID 转成 iotp 的正整数 device_id。
//
// 平台 ID 默认是 8 位数字（utils.RandomNum），直接解析即可；
// 非数字 ID 退化为 53 位 FNV 哈希，保证在 JS 安全整数范围内。
// DeviceID 把平台设备 ID 映射成 iotp 的 device_id。
//
// 纯数字 ID 直接使用；其它形态用 FNV-64a 派生，但必须落在 2^40 以下——
// iotp 把 [2^40, +∞) 保留给聚合派生的设备，源设备写入该区间会被 400 拒绝。
func DeviceID(id string) int64 {
	trimmed := strings.TrimSpace(id)
	if n, err := strconv.ParseInt(trimmed, 10, 64); err == nil && n > 0 {
		return n
	}
	h := fnv.New64a()
	_, _ = h.Write([]byte(trimmed))
	n := int64(h.Sum64() & uint64(DerivedDeviceIDStart-1))
	if n == 0 {
		n = 1
	}
	return n
}
