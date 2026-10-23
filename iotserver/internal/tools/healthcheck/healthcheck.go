// Package healthcheck 提供「高级能力 - 资源管理」的连通性验证。
//
// 每种资源类型的检查方式：
//   - HTTP推送：按 option 里的 url/method/headers/timeout 发一次请求，2xx/3xx 视为成功；
//   - 消息对队列MQTT：用 paho 建立真实 MQTT 连接（CONNECT），连接成功即可；
//   - 消息队列Kafka：对 brokers 做 TCP 连通性检查（项目未引入 Kafka 客户端，仅验证可达性）；
//   - InfluxDB：请求 {addr}/ping，2xx 视为成功。
//
// 所有检查共用 defaultTimeout，失败返回带原因的 error（调用方据此把 health 置为 false）。
package healthcheck

import (
	"context"
	"fmt"
	"io"
	"iotserver/internal/pkg/errort"
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
		return errort.NewMessageErr("目标地址为空", nil)
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
			return errort.NewMessageErr("目标地址 {{.p1}} 属于云元数据/链路本地网段，禁止访问", map[string]interface{}{"p1": ip})
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
		return errort.NewMessageErr("验证任务排队超时，请稍后重试", nil)
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
	default:
		return errort.NewMessageErr("不支持的资源类型: {{.p1}}", map[string]interface{}{"p1": resourceType})
	}
}

// checkHTTP 按配置发一次 HTTP 请求
func checkHTTP(option map[string]interface{}) error {
	rawURL := str(option, "url")
	if rawURL == "" {
		return errort.NewMessageErr("缺少 URL", nil)
	}
	if !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
		return errort.NewMessageErr("URL 必须以 http:// 或 https:// 开头", nil)
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
		return errort.NewMessageErr("请求构造失败: {{.p1}}", map[string]interface{}{"p1": err})
	}
	for key, value := range mapOf(option, "headers") {
		req.Header.Set(key, fmt.Sprintf("%v", value))
	}

	client := newHTTPClient(timeout)
	resp, err := client.Do(req)
	if err != nil {
		return errort.NewMessageErr("请求失败: {{.p1}}", map[string]interface{}{"p1": err})
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))

	if resp.StatusCode >= 200 && resp.StatusCode < 400 {
		return nil
	}
	return errort.NewMessageErr("服务返回状态码 {{.p1}}", map[string]interface{}{"p1": resp.StatusCode})
}

// checkMQTT 建立真实 MQTT 连接
func checkMQTT(option map[string]interface{}) error {
	server := str(option, "server")
	if server == "" {
		return errort.NewMessageErr("缺少服务地址", nil)
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
		return errort.NewMessageErr("连接超时", nil)
	}
	if err := token.Error(); err != nil {
		return errort.NewMessageErr("连接失败: {{.p1}}", map[string]interface{}{"p1": err})
	}
	client.Disconnect(200)
	return nil
}

// checkKafka 对 brokers 做 TCP 连通性检查
func checkKafka(option map[string]interface{}) error {
	brokers := splitList(str(option, "brokers"))
	if len(brokers) == 0 {
		return errort.NewMessageErr("缺少 brokers", nil)
	}
	// 所有 broker 共享一份整体预算，避免 N 个 broker 时总耗时 = N × timeout
	deadline := time.Now().Add(timeoutOf(option))
	var lastErr error
	for _, broker := range brokers {
		remain := time.Until(deadline)
		if remain <= 0 {
			if lastErr == nil {
				lastErr = errort.NewMessageErr("验证超时", nil)
			}
			break
		}
		if err := dialTCP(broker, remain); err != nil {
			lastErr = errort.NewMessageErr("{{.p1}} 不可达: {{.p2}}", map[string]interface{}{"p1": broker, "p2": err})
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
		return errort.NewMessageErr("缺少服务地址", nil)
	}
	if !strings.Contains(addr, "://") {
		addr = "http://" + addr
	}
	timeout := timeoutOf(option)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(addr, "/")+"/ping", nil)
	if err != nil {
		return errort.NewMessageErr("请求构造失败: {{.p1}}", map[string]interface{}{"p1": err})
	}
	if v := str(option, "username"); v != "" {
		req.SetBasicAuth(v, str(option, "password"))
	}
	client := newHTTPClient(timeout)
	resp, err := client.Do(req)
	if err != nil {
		return errort.NewMessageErr("请求失败: {{.p1}}", map[string]interface{}{"p1": err})
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1024))
	if resp.StatusCode >= 200 && resp.StatusCode < 400 {
		return nil
	}
	return errort.NewMessageErr("服务返回状态码 {{.p1}}", map[string]interface{}{"p1": resp.StatusCode})
}

/** ***************************** 工具 ***************************** **/

// dialTCP TCP 拨号检查（host:port）
func dialTCP(address string, timeout time.Duration) error {
	address = strings.TrimSpace(address)
	if address == "" {
		return errort.NewMessageErr("地址为空", nil)
	}
	if !strings.Contains(address, ":") {
		return errort.NewMessageErr("地址缺少端口: {{.p1}}", map[string]interface{}{"p1": address})
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
