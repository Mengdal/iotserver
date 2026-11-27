package ruleengineapp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"

	"iotserver/internal/dtos"
	"iotserver/internal/models"
)

const (
	// debugActionTimeout 调试动作默认超时；资源未配置 timeout 时使用
	debugActionTimeout = 5 * time.Second
	// debugActionMaxTimeout 调试动作最大超时，避免一次推送长期阻塞 SSE
	debugActionMaxTimeout = 30 * time.Second
	// debugActionRespLimit 失败时读取的响应体上限，用于展示错误信息
	debugActionRespLimit = 512
)

// resolveDebugActions 解析调试时需要执行的动作。
// eKuiper 的 /ruletest 只把结果写到 SSE，不会执行规则配置的 sink，
// 所以由平台按资源定义复用 HTTP/MQTT 推送；Kafka/InfluxDB 调试暂不执行。
func (app *ruleEngineApp) resolveDebugActions(ctx context.Context, enabled *bool, ids []string) ([]debugAction, error) {
	if enabled == nil || !*enabled {
		return nil, nil
	}
	refs := make(models.RuleEngineActions, 0, len(ids))
	for _, raw := range ids {
		id := strings.TrimSpace(raw)
		if id == "" {
			continue
		}
		refs = append(refs, models.RuleEngineAction{ResourceId: id})
	}
	if len(refs) == 0 {
		return nil, nil
	}
	// 复用规则保存时的校验：资源存在、动作不重复、数量上限
	stored, ekuiperActions, err := app.resolveActionRefs(ctx, refs)
	if err != nil {
		return nil, err
	}
	actions := make([]debugAction, 0, len(stored))
	for i := range stored {
		if i >= len(ekuiperActions) {
			break
		}
		props := ekuiperActions[i]
		if props.Rest == nil && props.MQTT == nil {
			continue
		}
		actions = append(actions, debugAction{
			resourceID:   stored[i].ResourceId,
			name:         stored[i].ResourceName,
			resourceType: stored[i].ResourceType,
			props:        props,
		})
	}
	return actions, nil
}

// RuleEngineDebugDeliver 把一条试运行输出真实推送到调试会话绑定的动作。
func (app *ruleEngineApp) RuleEngineDebugDeliver(ctx context.Context, id, payload string) []dtos.RuleEngineDebugActionResult {
	actions, hasDataTemplate, ok := app.debugSessionActions(id)
	if !ok || len(actions) == 0 {
		return nil
	}
	payload = strings.TrimSpace(payload)
	if payload == "" {
		return nil
	}
	// 没有数据模板时，eKuiper 的报错信息也是 data 行；只对合法 JSON 执行业务动作，
	// 避免把 "fail to calculate props ..." 这类错误当成业务数据推出去
	if !hasDataTemplate && !json.Valid([]byte(payload)) {
		return nil
	}
	// 数据模板本身报错时 eKuiper 会输出纯文本错误，同样不能当业务数据推送
	if strings.HasPrefix(payload, "fail to ") || strings.HasPrefix(payload, "sink:") {
		return nil
	}

	results := make([]dtos.RuleEngineDebugActionResult, len(actions))
	var wg sync.WaitGroup
	for i := range actions {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			results[index] = executeDebugAction(ctx, actions[index], []byte(payload))
		}(i)
	}
	wg.Wait()

	for i := range results {
		if !results[i].Ok {
			app.lc.Warnf("rule debug action failed: rule=%s resource=%s type=%s err=%s",
				id, results[i].ResourceID, results[i].Type, results[i].Error)
		}
	}
	return results
}

func executeDebugAction(ctx context.Context, action debugAction, payload []byte) dtos.RuleEngineDebugActionResult {
	started := time.Now()
	result := dtos.RuleEngineDebugActionResult{
		ResourceID: action.resourceID,
		Name:       action.name,
		Type:       action.resourceType,
	}
	var err error
	switch {
	case action.props.Rest != nil:
		result.StatusCode, err = doDebugHTTPAction(ctx, action.props.Rest, payload)
	case action.props.MQTT != nil:
		err = doDebugMQTTAction(ctx, action.props.MQTT, payload)
	}
	if err != nil {
		result.Error = err.Error()
	} else {
		result.Ok = true
	}
	result.DurationMs = time.Since(started).Milliseconds()
	return result
}

func doDebugHTTPAction(ctx context.Context, props map[string]interface{}, payload []byte) (int, error) {
	rawURL := debugString(props["url"])
	if rawURL == "" {
		return 0, fmt.Errorf("action url is empty")
	}
	method := strings.ToUpper(debugString(props["method"]))
	if method == "" {
		method = http.MethodPost
	}
	bodyType := strings.ToLower(debugString(props["bodyType"]))
	timeout := debugTimeout(props["timeout"])

	var body io.Reader
	if method != http.MethodGet && method != http.MethodHead && bodyType != "none" {
		body = bytes.NewReader(payload)
	}
	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, method, rawURL, body)
	if err != nil {
		return 0, err
	}
	for key, value := range debugStringMap(props["headers"]) {
		if strings.TrimSpace(key) != "" {
			req.Header.Set(key, value)
		}
	}
	if body != nil && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", debugContentType(bodyType))
	}

	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, debugActionRespLimit))
		msg := strings.TrimSpace(string(snippet))
		if msg == "" {
			return resp.StatusCode, fmt.Errorf("HTTP %d", resp.StatusCode)
		}
		return resp.StatusCode, fmt.Errorf("HTTP %d: %s", resp.StatusCode, msg)
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, debugActionRespLimit))
	return resp.StatusCode, nil
}

func doDebugMQTTAction(ctx context.Context, props map[string]interface{}, payload []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	server := debugString(props["server"])
	topic := debugString(props["topic"])
	if server == "" {
		return fmt.Errorf("mqtt server is empty")
	}
	if topic == "" {
		return fmt.Errorf("mqtt topic is empty")
	}
	broker := server
	if !strings.Contains(broker, "://") {
		broker = "tcp://" + broker
	}
	timeout := debugTimeout(props["timeout"])

	// 调试推送使用独立 clientId：如果沿用资源配置里的 clientId，
	// 可能把正在运行的规则 MQTT sink 顶下线，导致正式规则中断
	clientID := fmt.Sprintf("iotserver-debug-%d", time.Now().UnixNano())
	opts := mqtt.NewClientOptions().
		AddBroker(broker).
		SetClientID(clientID).
		SetConnectTimeout(timeout).
		// 调试推送失败要尽快返回，不能让 paho 在后台无限重连
		SetAutoReconnect(false).
		SetConnectRetry(false).
		SetConnectionLostHandler(func(mqtt.Client, error) {})
	if username := debugString(props["username"]); username != "" {
		opts.SetUsername(username)
	}
	if password := debugString(props["password"]); password != "" {
		opts.SetPassword(password)
	}
	switch version := debugString(props["protocolVersion"]); {
	case strings.HasPrefix(version, "5"):
		opts.SetProtocolVersion(5)
	case strings.HasPrefix(version, "3.1.1"):
		opts.SetProtocolVersion(4)
	case strings.HasPrefix(version, "3.1"):
		opts.SetProtocolVersion(3)
	}

	client := mqtt.NewClient(opts)
	defer client.Disconnect(200)
	token := client.Connect()
	if !token.WaitTimeout(timeout) {
		return fmt.Errorf("mqtt connect timeout")
	}
	if err := token.Error(); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	qos := debugInt(props["qos"])
	if qos < 0 || qos > 2 {
		qos = 0
	}
	token = client.Publish(topic, byte(qos), false, payload)
	if !token.WaitTimeout(timeout) {
		return fmt.Errorf("mqtt publish timeout")
	}
	return token.Error()
}

func debugString(v interface{}) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return strings.TrimSpace(s)
	}
	return strings.TrimSpace(fmt.Sprint(v))
}

func debugInt(v interface{}) int {
	switch value := v.(type) {
	case int:
		return value
	case int32:
		return int(value)
	case int64:
		return int(value)
	case float32:
		return int(value)
	case float64:
		return int(value)
	case json.Number:
		n, _ := value.Int64()
		return int(n)
	case string:
		n, _ := strconv.Atoi(strings.TrimSpace(value))
		return n
	}
	return 0
}

func debugTimeout(v interface{}) time.Duration {
	timeout := debugActionTimeout
	switch value := v.(type) {
	case string:
		if parsed, err := time.ParseDuration(strings.TrimSpace(value)); err == nil {
			timeout = parsed
		} else if ms, err := strconv.Atoi(strings.TrimSpace(value)); err == nil {
			timeout = time.Duration(ms) * time.Millisecond
		}
	default:
		if ms := debugInt(value); ms > 0 {
			timeout = time.Duration(ms) * time.Millisecond
		}
	}
	if timeout <= 0 {
		timeout = debugActionTimeout
	}
	if timeout > debugActionMaxTimeout {
		timeout = debugActionMaxTimeout
	}
	return timeout
}

func debugStringMap(v interface{}) map[string]string {
	out := make(map[string]string)
	switch value := v.(type) {
	case map[string]string:
		for key, item := range value {
			out[key] = item
		}
	case map[string]interface{}:
		for key, item := range value {
			if item == nil {
				continue
			}
			out[key] = debugString(item)
		}
	}
	return out
}

func debugContentType(bodyType string) string {
	switch bodyType {
	case "text":
		return "text/plain; charset=utf-8"
	case "html":
		return "text/html; charset=utf-8"
	case "xml":
		return "application/xml; charset=utf-8"
	case "javascript":
		return "application/javascript; charset=utf-8"
	case "form":
		return "application/x-www-form-urlencoded"
	default:
		return "application/json"
	}
}
