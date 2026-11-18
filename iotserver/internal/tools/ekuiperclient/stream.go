package ekuiperclient

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"iotserver/internal/pkg/logger"
)

const (
	// MQTTStreamName 平台所有规则共享的 MQTT 输入流。
	MQTTStreamName = "mqtt_stream"
	// MQTTStreamSQL 创建 mqtt_stream 的语句（与历史实现保持一致）。
	MQTTStreamSQL = `CREATE STREAM mqtt_stream () WITH (DATASOURCE="eventbus/in", FORMAT="JSON",SHARED = "true")`
)

// EnsureStream 幂等确保 eKuiper 流存在：先查询，已存在就不再 POST。
//
// 直接 POST 会让 eKuiper 在每次平台/组件重启时打一条
// "Create stream fails: Item mqtt_stream already exists." 的 error 日志。
func EnsureStream(ctx context.Context, baseUrl string, lc logger.LoggingClient, name, sql string) (created bool, err error) {
	baseUrl = strings.TrimSpace(baseUrl)
	if baseUrl == "" {
		return false, fmt.Errorf("ekuiper address is empty")
	}
	c := &ekuiperClient{
		baseUrl: strings.TrimRight(baseUrl, "/"),
		lc:      lc,
	}
	exists, err := c.streamExists(ctx, name)
	if err != nil {
		return false, err
	}
	if exists {
		return false, nil
	}
	return c.createStream(ctx, sql)
}

// streamExists 查询单个流；404 或 400+not found 表示不存在。
// eKuiper 2.4.2 对不存在的流返回 400（"is not found"），不是 404，
// 不能只认 404，也不能把所有 400 都当成「已存在」。
func (c *ekuiperClient) streamExists(ctx context.Context, name string) (bool, error) {
	status, body, err := c.do(ctx, http.MethodGet, "/streams/"+url.PathEscape(name), nil)
	if err != nil {
		return false, err
	}
	switch {
	case status == http.StatusOK:
		return true, nil
	case status == http.StatusNotFound:
		return false, nil
	case status == http.StatusBadRequest && streamNotFound(body):
		return false, nil
	default:
		return false, fmt.Errorf("get stream %s failed: status=%d body=%s", name, status, strings.TrimSpace(string(body)))
	}
}

// streamNotFound 兼容 eKuiper 不同版本的 missing 提示。
func streamNotFound(body []byte) bool {
	text := strings.ToLower(string(body))
	return strings.Contains(text, "not found") ||
		strings.Contains(text, "not exist") ||
		strings.Contains(text, "does not exist")
}

// createStream 创建流；eKuiper 的 400 + already exists 视为并发下的成功。
func (c *ekuiperClient) createStream(ctx context.Context, sql string) (bool, error) {
	payload, err := json.Marshal(map[string]string{"sql": sql})
	if err != nil {
		return false, err
	}
	status, body, err := c.do(ctx, http.MethodPost, "/streams", payload)
	if err != nil {
		return false, err
	}
	if status == http.StatusCreated || status == http.StatusOK {
		return true, nil
	}
	if status == http.StatusBadRequest && strings.Contains(string(body), "already exists") {
		return false, nil
	}
	return false, fmt.Errorf("create stream failed: status=%d body=%s", status, strings.TrimSpace(string(body)))
}
