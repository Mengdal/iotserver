package iotp

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// 网关归属（集群拓扑）API。
//
// iotp 的 gateway 不是数据模型层级，而是「注册主体 / 设备归属」概念：
//   - POST /api/v1/meta/gateways 声明一个网关（name 即网关键）；
//   - POST /api/v1/meta/devices/gateway 给设备盖章，gateway 为空表示清除归属；
//   - DELETE /api/v1/meta/gateways/{name} 删除声明，设备章变为悬挂/未归属。
//
// 这些接口全部走 /api/v1/meta/*，需要 admin 权限；集群模式下管理写只收控制节点
// （role=node 返回 409 NOT_CONTROL_NODE）。

// GatewayInfo 已声明的网关。
type GatewayInfo struct {
	Name        string   `json:"name"`
	Note        string   `json:"note,omitempty"`
	CreatedAt   any      `json:"created_at,omitempty"`
	Nodes       []string `json:"nodes,omitempty"`
	DeviceCount int      `json:"device_count,omitempty"`
}

// DeviceInfo 设备实例的元数据（只取归属相关字段）。
type DeviceInfo struct {
	ID      int64   `json:"id"`
	DB      string  `json:"db"`
	Table   string  `json:"table"`
	Name    string  `json:"name"`
	Gateway *string `json:"gateway"`
}

// EnsureGateway 确保网关已声明；已存在（409）视为成功。
func (c *Client) EnsureGateway(ctx context.Context, name, note string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("iotp: gateway name is empty")
	}
	payload := map[string]string{"name": name}
	if note = strings.TrimSpace(note); note != "" {
		payload["note"] = note
	}
	err := c.do(ctx, http.MethodPost, "/api/v1/meta/gateways", payload, nil)
	if err == nil {
		return nil
	}
	// 重复声明是幂等的；但集群数据节点返回的 409 不是「已存在」，不能吞掉。
	if IsConflict(err) && !IsNotControlNode(err) {
		return nil
	}
	return err
}

// DeleteGateway 删除网关声明；不存在（404）视为成功。
//
// 注意：删除只移除声明，设备上的章会变成悬挂章（语义上按未归属处理），
// 历史时序数据不受影响。调用方应先清理自己管理的设备归属。
func (c *Client) DeleteGateway(ctx context.Context, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil
	}
	path := "/api/v1/meta/gateways/" + url.PathEscape(name)
	err := c.do(ctx, http.MethodDelete, path, nil, nil)
	if err == nil || IsNotFound(err) {
		return nil
	}
	return err
}

// Gateways 列出已声明的网关。
func (c *Client) Gateways(ctx context.Context) ([]GatewayInfo, error) {
	var result []GatewayInfo
	if err := c.do(ctx, http.MethodGet, "/api/v1/meta/gateways", nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// AssignDeviceGateway 批量设置设备归属；gateway 为空表示清除归属。
//
// 设备必须已注册；任一设备不存在时 iotp 返回 404 并整体回滚，调用方应保证
// 设备实例先注册成功。
func (c *Client) AssignDeviceGateway(ctx context.Context, deviceIDs []int64, gateway string) error {
	if len(deviceIDs) == 0 {
		return nil
	}
	payload := map[string]any{"device_ids": deviceIDs}
	if gateway = strings.TrimSpace(gateway); gateway == "" {
		payload["gateway"] = nil
	} else {
		payload["gateway"] = gateway
	}
	return c.do(ctx, http.MethodPost, "/api/v1/meta/devices/gateway", payload, nil)
}

// TableDevices 列出某张表下的设备实例，用于对账时一次取全量归属。
//
// iotp 的全局设备列表会把所有库混在一起且不支持分页，按表查询更可控；
// 调用方只需对本地存在绑定的表逐一拉取。
func (c *Client) TableDevices(ctx context.Context, table string) ([]DeviceInfo, error) {
	path := fmt.Sprintf("/api/v1/meta/tables/%s/%s/devices",
		url.PathEscape(c.db), url.PathEscape(table))
	var result []DeviceInfo
	if err := c.do(ctx, http.MethodGet, path, nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// Device 读取单台设备实例；不存在时返回 404（可用 IsNotFound 判断）。
func (c *Client) Device(ctx context.Context, deviceID int64) (DeviceInfo, error) {
	var result DeviceInfo
	if deviceID <= 0 {
		return result, &APIError{Status: http.StatusNotFound, Message: "device id is empty"}
	}
	err := c.do(ctx, http.MethodGet, fmt.Sprintf("/api/v1/meta/devices/%d", deviceID), nil, &result)
	return result, err
}

// IsNotControlNode 集群数据节点拒绝管理写（409 NOT_CONTROL_NODE）。
func IsNotControlNode(err error) bool {
	apiErr, ok := err.(*APIError)
	return ok && apiErr.Code == "NOT_CONTROL_NODE"
}

// IsForbidden 已认证但缺少权限（403）。
func IsForbidden(err error) bool {
	apiErr, ok := err.(*APIError)
	return ok && apiErr.Status == http.StatusForbidden
}
