package healthcheck

import (
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"iotserver/internal/pkg/constants"
)

func TestCheckHTTP(t *testing.T) {
	ok := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Test") != "1" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer ok.Close()

	// 成功（带头、指定方法）
	if err := Check(string(constants.HttpResource), map[string]interface{}{
		"url":     ok.URL + "/push",
		"method":  "POST",
		"headers": map[string]interface{}{"X-Test": "1"},
		"timeout": 3000,
	}); err != nil {
		t.Fatalf("应验证成功, got %v", err)
	}

	// 缺必填
	if err := Check(string(constants.HttpResource), map[string]interface{}{}); err == nil {
		t.Fatal("缺 url 应失败")
	}
	// 协议不合法
	if err := Check(string(constants.HttpResource), map[string]interface{}{"url": "127.0.0.1/x"}); err == nil {
		t.Fatal("非 http(s) 应失败")
	}

	// 5xx 视为失败
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer bad.Close()
	if err := Check(string(constants.HttpResource), map[string]interface{}{"url": bad.URL}); err == nil {
		t.Fatal("5xx 应验证失败")
	}

	// 超时
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(600 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer slow.Close()
	if err := Check(string(constants.HttpResource), map[string]interface{}{
		"url": slow.URL, "timeout": 100,
	}); err == nil {
		t.Fatal("超时应验证失败")
	}

	// 端口不可达
	if err := Check(string(constants.HttpResource), map[string]interface{}{
		"url": "http://127.0.0.1:59999/x", "timeout": 300,
	}); err == nil {
		t.Fatal("不可达应验证失败")
	}
}

func TestCheckInfluxDB(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ping" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	if err := Check(string(constants.InfluxDBResource), map[string]interface{}{
		"addr": srv.URL, "databasename": "iot",
	}); err != nil {
		t.Fatalf("ping 成功应验证通过, got %v", err)
	}
	if err := Check(string(constants.InfluxDBResource), map[string]interface{}{"addr": "127.0.0.1:59998"}); err == nil {
		t.Fatal("不可达应验证失败")
	}
	if err := Check(string(constants.InfluxDBResource), map[string]interface{}{}); err == nil {
		t.Fatal("缺 addr 应失败")
	}
}

func TestCheckTCPBasedTypes(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			_ = conn.Close()
		}
	}()
	addr := ln.Addr().String()
	port := ln.Addr().(*net.TCPAddr).Port

	// Kafka：brokers 可达
	if err := Check(string(constants.KafkaResource), map[string]interface{}{
		"brokers": addr, "topic": "t", "timeout": 1000,
	}); err != nil {
		t.Fatalf("broker 可达应通过, got %v", err)
	}
	// Kafka：多个 broker，只要有一个可达即通过
	if err := Check(string(constants.KafkaResource), map[string]interface{}{
		"brokers": "127.0.0.1:59997," + addr, "topic": "t", "timeout": 1000,
	}); err != nil {
		t.Fatalf("有可达 broker 应通过, got %v", err)
	}
	// Kafka：全部不可达
	if err := Check(string(constants.KafkaResource), map[string]interface{}{
		"brokers": "127.0.0.1:59997", "topic": "t", "timeout": 300,
	}); err == nil {
		t.Fatal("全部不可达应失败")
	}
	// Kafka：缺 brokers
	if err := Check(string(constants.KafkaResource), map[string]interface{}{"topic": "t"}); err == nil {
		t.Fatal("缺 brokers 应失败")
	}

	// TDengine：端口可达但非 TDengine 协议 → Ping 失败（必须返回错误而不是 panic）
	if err := Check(string(constants.TDengineResource), map[string]interface{}{
		"host": "127.0.0.1", "port": port, "database": "iot", "timeout": 800,
	}); err == nil {
		t.Fatal("非 TDengine 服务应验证失败")
	}
	// TDengine：缺 host
	if err := Check(string(constants.TDengineResource), map[string]interface{}{"database": "iot"}); err == nil {
		t.Fatal("缺 host 应失败")
	}
}

func TestCheckUnsupportedType(t *testing.T) {
	if err := Check("Redis", map[string]interface{}{"addr": "127.0.0.1:6379"}); err == nil {
		t.Fatal("不支持的类型应返回错误")
	}
	if err := Check(string(constants.MQTTResource), map[string]interface{}{}); err == nil {
		t.Fatal("MQTT 缺 server 应失败")
	}
}

// TestCheckMQTTAgainstLocalBroker 若本机有 MQTT broker 则验证成功路径，否则跳过
func TestCheckMQTTAgainstLocalBroker(t *testing.T) {
	brokerAddr := "127.0.0.1:58090"
	conn, err := net.DialTimeout("tcp", brokerAddr, 500*time.Millisecond)
	if err != nil {
		t.Skipf("本机无 MQTT broker(%s)，跳过: %v", brokerAddr, err)
	}
	_ = conn.Close()

	if err := Check(string(constants.MQTTResource), map[string]interface{}{
		"server": brokerAddr, "topic": "t", "clientId": "iotserver-hc-test",
		"timeout": 3000,
	}); err != nil {
		t.Fatalf("连接本机 broker 应成功, got %v", err)
	}

	// 不可达端口 → 失败
	if err := Check(string(constants.MQTTResource), map[string]interface{}{
		"server": "127.0.0.1:59996", "topic": "t", "timeout": 800,
	}); err == nil {
		t.Fatal("不可达 broker 应失败")
	}
}

func TestTimeoutOf(t *testing.T) {
	if got := timeoutOf(nil); got != defaultTimeout {
		t.Fatalf("默认超时 = %v, want %v", got, defaultTimeout)
	}
	if got := timeoutOf(map[string]interface{}{"timeout": 100}); got != 100*time.Millisecond {
		t.Fatalf("自定义超时 = %v", got)
	}
	if got := timeoutOf(map[string]interface{}{"timeout": 999999}); got != 30*time.Second {
		t.Fatalf("超时应被限制到 30s, got %v", got)
	}
	if got := timeoutOf(map[string]interface{}{"timeout": "250"}); got != 250*time.Millisecond {
		t.Fatalf("字符串超时 = %v", got)
	}
}
