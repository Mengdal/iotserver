// Package healthcheck 提供「高级能力 - 资源管理」的连通性验证。
//
// 每种资源类型的检查方式：
//   - HTTP推送：按 option 里的 url/method/headers/timeout 发一次请求，2xx/3xx 视为成功；
//   - 消息对队列MQTT：用 paho 建立真实 MQTT 连接（CONNECT），连接成功即可；
//   - 消息队列Kafka：对 brokers 做 TCP 连通性检查（项目未引入 Kafka 客户端，仅验证可达性）；
//   - InfluxDB：请求 {addr}/ping，2xx 视为成功；
//   - TDengine：用 taosWS 驱动按 host/port/user/password/database 连接并 Ping。
//
// 所有检查共用 defaultTimeout，失败返回带原因的 error（调用方据此把 health 置为 false）。
package healthcheck

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"

	"iotserver/internal/pkg/constants"
)

// defaultTimeout 单次验证的超时时间
const defaultTimeout = 5 * time.Second

// verifySemaphore 同时在跑的验证任务上限
var verifySemaphore = make(chan struct{}, 8)

// 云元数据与链路本地地址：任何情况下都不允许作为验证目标（SSRF 防护）。
// 说明：内网/环回地址是「资源连通性验证」的正当目标（IoT 网关、内网 broker），因此不默认封禁，
// 但云元数据地址（169.254.0.0/16、阿里云 100.100.100.200）绝无正当用途。
var deniedNets = func() []*net.IPNet {
	out := make([]*net.IPNet, 0, 3)
	for _, cidr := range []string{"169.254.0.0/16", "100.100.100.200/32", "fd00:ec2::254/128"} {
		if _, n, err := net.ParseCIDR(cidr); err == nil {
			out = append(out, n)
		}
	}
	return out
}()

// checkTargetAddr 校验目标地址不是被禁止的元数据地址（host 或 host:port）
func checkTargetAddr(hostPort string) error {
	host := hostPort
	if h, _, err := net.SplitHostPort(hostPort); err == nil {
		host = h
	}
	host = strings.Trim(host, "[]")
	if host == "" {
		return fmt.Errorf("目标地址为空")
	}
	if ip := net.ParseIP(host); ip != nil {
		return denyIP(ip)
	}
	ips, err := net.LookupIP(host)
	if err != nil {
		return nil // 解析失败交由后续拨号报错
	}
	for _, ip := range ips {
		if err := denyIP(ip); err != nil {
			return err
		}
	}
	return nil
}

func denyIP(ip net.IP) error {
	for _, n := range deniedNets {
		if n.Contains(ip) {
			return fmt.Errorf("目标地址 %s 属于云元数据/链路本地网段，禁止访问", ip)
		}
	}
	return nil
}

// guardedDialContext 在真正拨号前校验解析后的地址，避免 DNS rebinding 绕过
func guardedDialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	if err := checkTargetAddr(addr); err != nil {
		return nil, err
	}
	d := &net.Dialer{Timeout: defaultTimeout}
	return d.DialContext(ctx, network, addr)
}

// newHTTPClient 统一构造带 SSRF 防护与超时的 HTTP 客户端
func newHTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			DialContext:         guardedDialContext,
			TLSHandshakeTimeout: timeout,
		},
	}
}

// Check 按资源类型执行连通性验证
func Check(resourceType string, option map[string]interface{}) error {
	if option == nil {
		option = map[string]interface{}{}
	}
	// 限制并发验证数，避免被并发点击放大成对目标/自身的压力
	select {
	case verifySemaphore <- struct{}{}:
		defer func() { <-verifySemaphore }()
	case <-time.After(defaultTimeout):
		return fmt.Errorf("验证任务排队超时，请稍后重试")
	}
	switch constants.DataResourceType(resourceType) {
	case constants.HttpResource:
		return checkHTTP(option)
	case constants.MQTTResource:
		return checkMQTT(option)
	case constants.KafkaResource:
		return checkKafka(option)
	case constants.InfluxDBResource:
		return checkInfluxDB(option)
	case constants.TDengineResource:
		return checkTDengine(option)
	default:
		return fmt.Errorf("不支持的资源类型: %s", resourceType)
	}
}

// checkHTTP 按配置发一次 HTTP 请求
func checkHTTP(option map[string]interface{}) error {
	rawURL := str(option, "url")
	if rawURL == "" {
		return fmt.Errorf("缺少 URL")
	}
	if !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
		return fmt.Errorf("URL 必须以 http:// 或 https:// 开头")
	}

	method := strings.ToUpper(str(option, "method"))
	if method == "" {
		method = http.MethodGet
	}
	timeout := timeoutOf(option)

	var body io.Reader
	if bt := str(option, "bodyType"); bt != "" && bt != "none" && method != http.MethodGet && method != http.MethodHead {
		// 验证只关心目标可达，发送一个最小占位体
		body = strings.NewReader("{}")
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, method, rawURL, body)
	if err != nil {
		return fmt.Errorf("请求构造失败: %v", err)
	}
	for key, value := range mapOf(option, "headers") {
		req.Header.Set(key, fmt.Sprintf("%v", value))
	}

	client := newHTTPClient(timeout)
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("请求失败: %v", err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))

	if resp.StatusCode >= 200 && resp.StatusCode < 400 {
		return nil
	}
	return fmt.Errorf("服务返回状态码 %d", resp.StatusCode)
}

// checkMQTT 建立真实 MQTT 连接
func checkMQTT(option map[string]interface{}) error {
	server := str(option, "server")
	if server == "" {
		return fmt.Errorf("缺少服务地址")
	}
	broker := server
	if !strings.Contains(broker, "://") {
		broker = "tcp://" + broker
	}
	timeout := timeoutOf(option)

	if err := checkTargetAddr(strings.TrimPrefix(strings.TrimPrefix(server, "tcp://"), "ssl://")); err != nil {
		return err
	}
	clientId := str(option, "clientId")
	if clientId == "" {
		clientId = fmt.Sprintf("iotserver-verify-%d", time.Now().UnixNano())
	}
	opts := mqtt.NewClientOptions().
		AddBroker(broker).
		SetClientID(clientId).
		SetConnectTimeout(timeout).
		// 关闭自动重连与连接重试：验证失败时必须立即结束，
		// 否则 paho 会留一个后台goroutine无限重连（每次失败泄漏一个客户端）
		SetAutoReconnect(false).
		SetConnectRetry(false).
		SetConnectionLostHandler(func(mqtt.Client, error) {})

	if v := str(option, "username"); v != "" {
		opts.SetUsername(v)
	}
	if v := str(option, "password"); v != "" {
		opts.SetPassword(v)
	}

	client := mqtt.NewClient(opts)
	// 无论成功失败都断开，避免连接残留
	defer client.Disconnect(200)

	token := client.Connect()
	if !token.WaitTimeout(timeout) {
		return fmt.Errorf("连接超时")
	}
	if err := token.Error(); err != nil {
		return fmt.Errorf("连接失败: %v", err)
	}
	client.Disconnect(200)
	return nil
}

// checkKafka 对 brokers 做 TCP 连通性检查
func checkKafka(option map[string]interface{}) error {
	brokers := splitList(str(option, "brokers"))
	if len(brokers) == 0 {
		return fmt.Errorf("缺少 brokers")
	}
	// 所有 broker 共享一份整体预算，避免 N 个 broker 时总耗时 = N × timeout
	deadline := time.Now().Add(timeoutOf(option))
	var lastErr error
	for _, broker := range brokers {
		remain := time.Until(deadline)
		if remain <= 0 {
			if lastErr == nil {
				lastErr = fmt.Errorf("验证超时")
			}
			break
		}
		if err := dialTCP(broker, remain); err != nil {
			lastErr = fmt.Errorf("%s 不可达: %v", broker, err)
			continue
		}
		return nil
	}
	return lastErr
}

// checkInfluxDB 请求 {addr}/ping
func checkInfluxDB(option map[string]interface{}) error {
	addr := str(option, "addr")
	if addr == "" {
		return fmt.Errorf("缺少服务地址")
	}
	if !strings.Contains(addr, "://") {
		addr = "http://" + addr
	}
	timeout := timeoutOf(option)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(addr, "/")+"/ping", nil)
	if err != nil {
		return fmt.Errorf("请求构造失败: %v", err)
	}
	if v := str(option, "username"); v != "" {
		req.SetBasicAuth(v, str(option, "password"))
	}
	client := newHTTPClient(timeout)
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("请求失败: %v", err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1024))
	if resp.StatusCode >= 200 && resp.StatusCode < 400 {
		return nil
	}
	return fmt.Errorf("服务返回状态码 %d", resp.StatusCode)
}

// checkTDengine 通过 taosAdapter 的 REST 接口验证连通性。
// 说明：这里刻意不用 taos 驱动——它的 init 里有 unsafe 指针运算，
// 会把驱动依赖带进应用层并让 `go test -race` 直接崩溃（checkptr fatal）。
func checkTDengine(option map[string]interface{}) error {
	host := str(option, "host")
	if host == "" {
		return fmt.Errorf("缺少主机地址")
	}
	port := intOf(option, "port")
	if port <= 0 {
		port = 6041 // taosAdapter 默认 REST 端口
	}
	user := str(option, "user")
	if user == "" {
		user = "root"
	}
	if err := checkTargetAddr(fmt.Sprintf("%s:%d", host, port)); err != nil {
		return err
	}

	timeout := timeoutOf(option)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	body := strings.NewReader("SHOW DATABASES")
	url := fmt.Sprintf("http://%s:%d/rest/sql", host, port)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
	if err != nil {
		return fmt.Errorf("请求构造失败: %v", err)
	}
	req.SetBasicAuth(user, str(option, "password"))
	req.Header.Set("Content-Type", "text/plain")

	resp, err := newHTTPClient(timeout).Do(req)
	if err != nil {
		return fmt.Errorf("连接失败: %v", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("服务返回状态码 %d", resp.StatusCode)
	}
	// taosAdapter 用 {"code":0,...} 表示成功
	var result struct {
		Code int    `json:"code"`
		Desc string `json:"desc"`
	}
	if err = json.Unmarshal(respBody, &result); err != nil {
		return fmt.Errorf("响应解析失败: %v", err)
	}
	if result.Code != 0 {
		return fmt.Errorf("TDengine 返回错误: %s", result.Desc)
	}
	return nil
}

/** ***************************** 工具 ***************************** **/

// dialTCP TCP 拨号检查（host:port）
func dialTCP(address string, timeout time.Duration) error {
	address = strings.TrimSpace(address)
	if address == "" {
		return fmt.Errorf("地址为空")
	}
	if !strings.Contains(address, ":") {
		return fmt.Errorf("地址缺少端口: %s", address)
	}
	if err := checkTargetAddr(address); err != nil {
		return err
	}
	conn, err := net.DialTimeout("tcp", address, timeout)
	if err != nil {
		return err
	}
	_ = conn.Close()
	return nil
}

func timeoutOf(option map[string]interface{}) time.Duration {
	if ms := intOf(option, "timeout"); ms > 0 {
		d := time.Duration(ms) * time.Millisecond
		if d > 30*time.Second {
			return 30 * time.Second
		}
		return d
	}
	return defaultTimeout
}

func str(option map[string]interface{}, key string) string {
	v, ok := option[key]
	if !ok || v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case int:
		return strconv.Itoa(t)
	case int64:
		return strconv.FormatInt(t, 10)
	default:
		return strings.TrimSpace(fmt.Sprintf("%v", v))
	}
}

func intOf(option map[string]interface{}, key string) int {
	v, ok := option[key]
	if !ok || v == nil {
		return 0
	}
	switch t := v.(type) {
	case float64:
		return int(t)
	case int:
		return t
	case int64:
		return int(t)
	case string:
		n, _ := strconv.Atoi(strings.TrimSpace(t))
		return n
	default:
		n, _ := strconv.Atoi(strings.TrimSpace(fmt.Sprintf("%v", v)))
		return n
	}
}

func mapOf(option map[string]interface{}, key string) map[string]interface{} {
	v, ok := option[key]
	if !ok || v == nil {
		return nil
	}
	switch t := v.(type) {
	case map[string]interface{}:
		return t
	case map[string]string:
		out := make(map[string]interface{}, len(t))
		for k, v := range t {
			out[k] = v
		}
		return out
	default:
		return nil
	}
}

func splitList(raw string) []string {
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ';' || r == ' '
	})
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
