package iotp

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

// RouteNode iotp 中网关当前在线的节点。
type RouteNode struct {
	Id        string `json:"id"`
	WSEnabled *bool  `json:"ws_enabled"`
	WSURL     string `json:"ws_url"`
}

// RouteDevice 一台设备当前的网关/节点归属。
type RouteDevice struct {
	DeviceId  int64       `json:"device_id"`
	Gateway   *string     `json:"gateway"`
	Online    bool        `json:"online"`
	Nodes     []RouteNode `json:"nodes"`
	Placement []string    `json:"placement"`
	Reason    *string     `json:"reason"`
}

// RouteResponse POST /api/v1/devices/route 的响应。
type RouteResponse struct {
	Mode    string        `json:"mode"`
	Devices []RouteDevice `json:"devices"`
}

// RouteDevices 查询设备当前由哪个节点接收数据。
//
// iotp 的 WS 写控制只在网关连接的节点执行，不会跨节点转发；
// 客户端必须先通过这个接口找到网关所在节点，再连接该节点的 ws_url。
func (c *Client) RouteDevices(ctx context.Context, deviceIds []int64) (RouteResponse, error) {
	var result RouteResponse
	if len(deviceIds) == 0 {
		return result, fmt.Errorf("iotp route: device_ids 为空")
	}
	if err := c.do(ctx, http.MethodPost, "/api/v1/devices/route",
		map[string]any{"device_ids": deviceIds}, &result); err != nil {
		return result, err
	}
	return result, nil
}

type wsWriteFrame struct {
	Type string `json:"type"`
	Id   string `json:"id"`
	Val  string `json:"val"`
}

type wsWriteResponse struct {
	Code    int    `json:"code"`
	Data    string `json:"data"`
	Val     string `json:"val"`
	Message string `json:"message"`
}

// WriteControl 通过 iotp WebSocket 写控制下发一个设备字段。
//
// 链路由 iotp 负责：校验权限/网关归属 → MQTT devices/<gwId>/write →
// 等待网关 reply → 通过 WS 返回。这里只负责按路由找到节点并等待结果。
// 返回 nil 表示网关已受理；网关回复非 200 或超时都会返回错误。
func (c *Client) WriteControl(ctx context.Context, deviceId int64, field, value string) error {
	if deviceId <= 0 {
		return fmt.Errorf("iotp 写控制: device_id 无效")
	}
	if strings.TrimSpace(field) == "" {
		return fmt.Errorf("iotp 写控制: 字段名为空")
	}
	route, err := c.RouteDevices(ctx, []int64{deviceId})
	if err != nil {
		return fmt.Errorf("iotp 写控制: 查询设备路由失败: %w", err)
	}
	target, err := pickRouteDevice(route, deviceId)
	if err != nil {
		return err
	}
	wsURL, err := pickWSURL(target)
	if err != nil {
		return err
	}

	timeout := c.http.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	header := http.Header{}
	if c.token != "" {
		header.Set("Authorization", "Bearer "+c.token)
	}
	dialer := *websocket.DefaultDialer
	dialer.HandshakeTimeout = timeout
	conn, _, err := dialer.DialContext(ctx, wsURL, header)
	if err != nil {
		return fmt.Errorf("iotp 写控制: 连接节点 %s 失败: %w", wsURL, err)
	}
	defer conn.Close()

	if err = conn.SetWriteDeadline(time.Now().Add(timeout)); err != nil {
		return fmt.Errorf("iotp 写控制: 设置写超时失败: %w", err)
	}
	// iotp 的写控制 id 校验的是表字段名；平台写入时用 FieldName 做过清洗
	// （- → _、status/time 等保留名 → f_ 前缀），这里必须用同一套名称。
	frame := wsWriteFrame{Type: "write", Id: fmt.Sprintf("%d.%s", deviceId, FieldName(field)), Val: value}
	if err = conn.WriteJSON(frame); err != nil {
		return fmt.Errorf("iotp 写控制: 发送命令失败: %w", err)
	}
	if err = conn.SetReadDeadline(time.Now().Add(timeout)); err != nil {
		return fmt.Errorf("iotp 写控制: 设置读超时失败: %w", err)
	}
	var resp wsWriteResponse
	if err = conn.ReadJSON(&resp); err != nil {
		return fmt.Errorf("iotp 写控制: 等待网关回复失败: %w", err)
	}
	if resp.Code != http.StatusOK {
		msg := strings.TrimSpace(resp.Message)
		if msg == "" {
			msg = "网关拒绝写入"
		}
		return fmt.Errorf("iotp 写控制失败(%d): %s", resp.Code, msg)
	}
	return nil
}

func pickRouteDevice(route RouteResponse, deviceId int64) (RouteDevice, error) {
	for _, device := range route.Devices {
		if device.DeviceId == deviceId {
			if device.Reason != nil && strings.TrimSpace(*device.Reason) != "" {
				return device, fmt.Errorf("iotp 写控制: 设备 %d 不可写（%s）", deviceId, routeReasonText(*device.Reason))
			}
			if !device.Online {
				return device, fmt.Errorf("iotp 写控制: 设备 %d 的网关不在线", deviceId)
			}
			return device, nil
		}
	}
	return RouteDevice{}, fmt.Errorf("iotp 写控制: 设备 %d 未返回路由", deviceId)
}

func pickWSURL(device RouteDevice) (string, error) {
	if len(device.Nodes) == 0 {
		return "", fmt.Errorf("iotp 写控制: 设备 %d 没有可用节点", device.DeviceId)
	}
	if len(device.Nodes) > 1 {
		return "", fmt.Errorf("iotp 写控制: 设备 %d 同时在多个节点在线，暂不支持", device.DeviceId)
	}
	node := device.Nodes[0]
	if node.WSEnabled != nil && !*node.WSEnabled {
		return "", fmt.Errorf("iotp 写控制: 节点 %s 未开启 WebSocket", node.Id)
	}
	if strings.TrimSpace(node.WSURL) == "" {
		return "", fmt.Errorf("iotp 写控制: 节点 %s 未配置 WebSocket 地址", node.Id)
	}
	return node.WSURL, nil
}

func routeReasonText(reason string) string {
	switch reason {
	case "no_gateway_mapping":
		return "设备未绑定网关"
	case "mqtt_disabled":
		return "节点未启用 MQTT"
	case "gateway_offline":
		return "网关离线"
	case "incomplete":
		return "节点信息不完整"
	case "multiple_nodes":
		return "网关同时在多个节点在线"
	case "ws_disabled":
		return "节点未开启 WebSocket"
	default:
		return reason
	}
}
