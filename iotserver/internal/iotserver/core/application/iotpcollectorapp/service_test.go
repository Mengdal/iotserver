package iotpcollectorapp

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"iotserver/internal/dtos"
	"iotserver/internal/models"
	"iotserver/internal/tools/iotp"
)

func TestBrokerKey(t *testing.T) {
	cases := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{in: "tcp://127.0.0.1:1883", want: "tcp://127.0.0.1:1883"},
		{in: "127.0.0.1:1883", want: "tcp://127.0.0.1:1883"},
		{in: "TCP://Host-1:1883", want: "tcp://host-1:1883"},
		{in: " ws://host:8080/mqtt ", want: "ws://host:8080/mqtt"},
		{in: "", wantErr: true},
		{in: "ftp://host:21", wantErr: true},
		{in: "tcp://", wantErr: true},
	}
	for _, tc := range cases {
		got, err := brokerKey(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Fatalf("brokerKey(%q) 期望报错，实际返回 %q", tc.in, got)
			}
			continue
		}
		if err != nil {
			t.Fatalf("brokerKey(%q) 报错: %v", tc.in, err)
		}
		if got != tc.want {
			t.Fatalf("brokerKey(%q) = %q, 期望 %q", tc.in, got, tc.want)
		}
	}
}

func TestParseStaticNodes(t *testing.T) {
	specs, invalid := parseStaticNodes([]string{
		"n01=tcp://127.0.0.1:1883",
		"dup=tcp://127.0.0.1:1883",
		"n02=127.0.0.1:1884",
		"bad",
		"=tcp://127.0.0.1:1885",
		"",
	})
	if len(specs) != 2 {
		t.Fatalf("期望 2 个节点，实际 %d: %+v", len(specs), specs)
	}
	if specs[0].Id != "n01" || specs[0].Broker != "tcp://127.0.0.1:1883" {
		t.Fatalf("第一个节点不符合预期: %+v", specs[0])
	}
	if specs[1].Id != "n02" || specs[1].Broker != "tcp://127.0.0.1:1884" {
		t.Fatalf("第二个节点不符合预期: %+v", specs[1])
	}
	if len(invalid) != 2 {
		t.Fatalf("期望 2 条非法配置，实际 %d: %v", len(invalid), invalid)
	}
}

func TestMergeNodeSpecs(t *testing.T) {
	static := []nodeSpec{{Id: "n01", Broker: "tcp://127.0.0.1:1883"}}
	discovered := map[string]gatewayBroker{
		"2": {Broker: "tcp://127.0.0.1:1883", Label: "gw-same-broker"},
		"1": {Broker: "tcp://127.0.0.1:1884", Label: "gw-4338603974"},
	}
	merged := mergeNodeSpecs(static, discovered)
	if len(merged) != 2 {
		t.Fatalf("期望合并为 2 个节点，实际 %d: %+v", len(merged), merged)
	}
	if got := merged["tcp://127.0.0.1:1883"]; got.Id != "n01" {
		t.Fatalf("同 broker 应保留静态节点，实际 %+v", got)
	}
	if got := merged["tcp://127.0.0.1:1884"]; got.Id != "gw-4338603974" {
		t.Fatalf("自动发现节点标签不符合预期: %+v", got)
	}
}

func TestMergeNodeSpecsRejectsInvalidBroker(t *testing.T) {
	merged := mergeNodeSpecs(nil, map[string]gatewayBroker{
		"1": {Broker: "http://127.0.0.1:1883"},
	})
	if len(merged) != 0 {
		t.Fatalf("非法 broker 不应进入订阅列表: %+v", merged)
	}
}

func TestRefreshDiscoveryCoalesces(t *testing.T) {
	service := &Service{discoverCh: make(chan struct{}, 1)}
	service.RefreshDiscovery()
	service.RefreshDiscovery()
	if got := len(service.discoverCh); got != 1 {
		t.Fatalf("多次触发应合并为 1 个信号，实际 %d", got)
	}
	<-service.discoverCh
	service.RefreshDiscovery()
	if got := len(service.discoverCh); got != 1 {
		t.Fatalf("消费后再次触发应重新入队，实际 %d", got)
	}
}

func TestRefreshDiscoveryWithoutChannel(t *testing.T) {
	service := &Service{}
	service.RefreshDiscovery() // 未初始化时不能 panic/阻塞
}

func TestBuildDeviceInfoIncludesProductName(t *testing.T) {
	device := models.Device{
		Id: "57369006", Name: "身份对齐验证设备", ProductId: "00665329",
		Product: models.Product{Name: "身份对齐验证产品"},
	}
	info := buildDeviceInfo(device, nil, nil)
	if info.Name != "身份对齐验证设备" || info.ProductId != "00665329" {
		t.Fatalf("设备信息不正确: %+v", info)
	}
	if info.ProductName != "身份对齐验证产品" {
		t.Fatalf("产品名称 = %q", info.ProductName)
	}
}

func TestMessageBusCarriesNames(t *testing.T) {
	payload, err := json.Marshal(dtos.MessageBus{
		DeviceId:    "57369006",
		DeviceName:  "身份对齐验证设备",
		ProductId:   "00665329",
		ProductName: "身份对齐验证产品",
		MessageType: "PROPERTY_REPORT",
		Data:        map[string]any{"temperature": 280},
	})
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(payload, &got); err != nil {
		t.Fatal(err)
	}
	if got["deviceName"] != "身份对齐验证设备" {
		t.Fatalf("deviceName = %v", got["deviceName"])
	}
	if got["productId"] != "00665329" {
		t.Fatalf("productId = %v", got["productId"])
	}
	if got["productName"] != "身份对齐验证产品" {
		t.Fatalf("productName = %v", got["productName"])
	}
}

func TestResolveDeviceTenant(t *testing.T) {
	gatewayTenants := map[string]string{
		"gw-1": "tenant-gw",
	}
	bindings := map[int64]gatewayBinding{
		iotp.DeviceID("dev-gw"):  {GatewayId: "gw-1"},
		iotp.DeviceID("dev-own"): {GatewayId: "gw-2", TenantId: "tenant-binding"},
	}

	withTenant := func(id, tenant string) models.Device {
		device := models.Device{Id: id}
		device.TenantId = tenant
		return device
	}
	cases := []struct {
		name   string
		device models.Device
		want   string
	}{
		{name: "设备租户优先", device: withTenant("dev-own", "tenant-device"), want: "tenant-device"},
		{name: "回落到采集网关租户", device: models.Device{Id: "dev-gw"}, want: "tenant-gw"},
		{name: "回落到绑定记录租户", device: models.Device{Id: "dev-own"}, want: "tenant-binding"},
		{name: "都没有则为空", device: models.Device{Id: "dev-none"}, want: ""},
	}
	for _, tc := range cases {
		if got := resolveDeviceTenant(tc.device, bindings, gatewayTenants); got != tc.want {
			t.Fatalf("%s: resolveDeviceTenant = %q, 期望 %q", tc.name, got, tc.want)
		}
	}
}

func TestSanitizeClientID(t *testing.T) {
	cases := map[string]string{
		"gw-2692348952":         "gw-2692348952",
		"Node_A":                "node_a",
		"节点 n01":                "---n01",
		"":                      "node",
		strings.Repeat("a", 40): strings.Repeat("a", 32),
	}
	for in, want := range cases {
		if got := sanitizeClientID(in); got != want {
			t.Fatalf("sanitizeClientID(%q) = %q, 期望 %q", in, got, want)
		}
	}
}

func TestEngineBroker(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":200,"message":"ok","data":{"IP":"127.0.0.1","port":1883}}`))
	}))
	defer server.Close()

	service := &Service{}
	broker, err := service.engineBroker(models.Gateway{
		EngineBaseURL:    server.URL,
		EngineTimeoutSec: 2,
	})
	if err != nil {
		t.Fatalf("engineBroker 报错: %v", err)
	}
	if broker != "tcp://127.0.0.1:1883" {
		t.Fatalf("engineBroker = %q, 期望 tcp://127.0.0.1:1883", broker)
	}
}

func TestEngineBrokerRejectsEmptyTarget(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":200,"message":"ok","data":{"IP":"","port":0}}`))
	}))
	defer server.Close()

	service := &Service{}
	if _, err := service.engineBroker(models.Gateway{EngineBaseURL: server.URL}); err == nil {
		t.Fatal("引擎 MQTT 目标为空时应报错")
	}
}
