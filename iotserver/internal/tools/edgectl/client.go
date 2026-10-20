// Package edgectl 参考系统（things /iotddc）的下行控制通道。
//
// 实测 things 的 Broker 有两个入口：
//   - 上行/属性写：EdgeIngest.Broker（通常是 1885）；
//   - 重启/标签树等管理命令：EdgeIngest.ControlBroker（通常是 1883）。
//
// 本包只负责“发请求 + 等 /reply”，不参与上行解析。
package edgectl

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"

	"iotserver/internal/pkg/logger"
)

// ErrNotConfigured 未配置控制通道 Broker
var ErrNotConfigured = errors.New("edge control broker is not configured")

// ErrNotConnected 控制通道未连接
var ErrNotConnected = errors.New("edge control broker is not connected")

// Client 参考系统控制客户端
type Client struct {
	broker   string
	username string
	password string
	lc       logger.LoggingClient

	mu      sync.Mutex
	client  mqtt.Client
	pending map[string]chan []byte
}

// New 创建控制客户端（Start 才会真正连接）
func New(broker, username, password string, lc logger.LoggingClient) *Client {
	return &Client{
		broker:   strings.TrimSpace(broker),
		username: username,
		password: password,
		lc:       lc,
		pending:  make(map[string]chan []byte),
	}
}

// Start 连接控制 Broker；失败不阻塞平台启动，调用方按错误降级。
func (c *Client) Start() error {
	if c.broker == "" {
		return ErrNotConfigured
	}
	opts := mqtt.NewClientOptions().
		AddBroker(c.broker).
		SetClientID(fmt.Sprintf("iotserver-edge-ctl-%d", time.Now().UnixNano())).
		SetAutoReconnect(true).
		SetConnectTimeout(8 * time.Second)
	if c.username != "" {
		opts.SetUsername(c.username)
	}
	if c.password != "" {
		opts.SetPassword(c.password)
	}
	client := mqtt.NewClient(opts)
	tok := client.Connect()
	tok.Wait()
	if tok.Error() != nil {
		return tok.Error()
	}
	c.mu.Lock()
	c.client = client
	c.mu.Unlock()
	if c.lc != nil {
		c.lc.Infof("参考系统控制通道已连接: %s", c.broker)
	}
	return nil
}

// Restart 重启网关：/edge/restart/{pk}/{sn}/notify，空报文，等 /reply。
func (c *Client) Restart(ctx context.Context, pk, sn string) error {
	if pk == "" {
		pk = sn
	}
	pub := fmt.Sprintf("/edge/restart/%s/%s/notify", pk, sn)
	reply := fmt.Sprintf("/edge/restart/%s/%s/reply", pk, sn)
	_, err := c.call(ctx, pub, reply, []byte{}, "", 10*time.Second)
	return err
}

// SetProperty 写设备属性：/edge/property/{pk}/{sn}/set，按 seq 匹配回执。
func (c *Client) SetProperty(ctx context.Context, pk, sn, dn, identifier, value string) (bool, string, error) {
	if pk == "" {
		pk = sn
	}
	seq := fmt.Sprintf("%d_%d", time.Now().UnixMilli(), time.Now().UnixNano()%1000)
	body, _ := json.Marshal(map[string]any{
		"seq":        seq,
		"value":      value,
		"dn":         dn,
		"identifier": identifier,
	})
	pub := fmt.Sprintf("/edge/property/%s/%s/set", pk, sn)
	reply := pub + "/reply"
	obj, err := c.call(ctx, pub, reply, body, seq, 10*time.Second)
	if err != nil {
		return false, "", err
	}
	status, _ := obj["status"].(bool)
	message, _ := obj["message"].(string)
	return status, message, nil
}

// TagTree 拉取网关设备/测点树：/edge/scadaGetTagsTree/{pk}/{sn}/get。
func (c *Client) TagTree(ctx context.Context, pk, sn string) (json.RawMessage, error) {
	if pk == "" {
		pk = sn
	}
	requestID := fmt.Sprintf("sync-%d", time.Now().UnixNano())
	body, _ := json.Marshal(map[string]string{"requestId": requestID})
	pub := fmt.Sprintf("/edge/scadaGetTagsTree/%s/%s/get", pk, sn)
	reply := fmt.Sprintf("/edge/scadaGetTagsTree/%s/%s/get/reply", pk, sn)
	obj, err := c.call(ctx, pub, reply, body, requestID, 15*time.Second)
	if err != nil {
		return nil, err
	}
	return json.Marshal(obj)
}

// call 发布请求并等待回执。key 为空时用回复主题匹配（适用于空报文/无业务号的重启）。
func (c *Client) call(ctx context.Context, pubTopic, replyTopic string, payload []byte, key string, timeout time.Duration) (map[string]any, error) {
	c.mu.Lock()
	client := c.client
	c.mu.Unlock()
	if client == nil || !client.IsConnected() {
		return nil, ErrNotConnected
	}

	pendingKey := key
	if pendingKey == "" {
		pendingKey = "topic:" + replyTopic
	}
	ch := make(chan []byte, 1)
	c.mu.Lock()
	c.pending[pendingKey] = ch
	c.mu.Unlock()
	defer func() {
		c.mu.Lock()
		delete(c.pending, pendingKey)
		c.mu.Unlock()
	}()

	subTok := client.Subscribe(replyTopic, 0, c.onMessage)
	subTok.Wait()
	if subTok.Error() != nil {
		return nil, subTok.Error()
	}
	pubTok := client.Publish(pubTopic, 0, false, payload)
	pubTok.Wait()
	if pubTok.Error() != nil {
		return nil, pubTok.Error()
	}

	select {
	case raw := <-ch:
		var obj map[string]any
		if err := json.Unmarshal(raw, &obj); err != nil {
			return nil, err
		}
		return obj, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(timeout):
		return nil, errors.New("等待网关回执超时")
	}
}

// onMessage 把 /reply 消息按 requestId/seq 投递给等待中的请求。
func (c *Client) onMessage(_ mqtt.Client, m mqtt.Message) {
	var obj map[string]any
	if json.Unmarshal(m.Payload(), &obj) != nil {
		return
	}
	key := ""
	if v, ok := obj["requestId"].(string); ok {
		key = v
	}
	if key == "" {
		if v, ok := obj["seq"].(string); ok {
			key = v
		}
	}
	if key == "" {
		key = "topic:" + m.Topic()
	}
	c.mu.Lock()
	ch := c.pending[key]
	c.mu.Unlock()
	if ch != nil {
		select {
		case ch <- m.Payload():
		default:
		}
	}
}
