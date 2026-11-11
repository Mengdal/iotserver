package driverapp

import (
	"testing"

	"iotserver/internal/iotserver/core/config"
	bootstrapConfig "iotserver/internal/pkg/config"
	"iotserver/internal/pkg/seed"
)

// 系统组件的地址来自各自配置：Mosquitto 用消息总线，eKuiper 用 Clients，
// IotEdgeDB 用 [Databases.Data].BaseURL（外部时序库）。
func TestComponentEndpoint(t *testing.T) {
	cfg := &config.ConfigurationStruct{
		MessageQueue: config.MessageQueueInfo{Host: "127.0.0.1", Port: 1883, Protocol: "tcp"},
		Clients: map[string]bootstrapConfig.ClientInfo{
			"Ekuiper": {Host: "127.0.0.1", Port: 59720, Protocol: "http"},
		},
		Databases: map[string]bootstrapConfig.Database{
			"Data": {Type: "iotp", BaseURL: "  http://124.223.204.174:8080  "},
		},
	}

	if got := componentEndpoint(cfg, seed.SystemComponentMosquittoId); got != "tcp://127.0.0.1:1883" {
		t.Fatalf("mosquitto endpoint = %q", got)
	}
	if got := componentEndpoint(cfg, seed.SystemComponentEKuiperId); got == "" {
		t.Fatal("ekuiper endpoint should not be empty")
	}
	if got := componentEndpoint(cfg, seed.SystemComponentIotEdgeDBId); got != "http://124.223.204.174:8080" {
		t.Fatalf("iotedgedb endpoint = %q", got)
	}
	if got := componentEndpoint(nil, seed.SystemComponentIotEdgeDBId); got != "" {
		t.Fatalf("nil config endpoint = %q", got)
	}
}

// 未配置 BaseURL 时外部时序库地址为空，前端显示 unknown，而不是误报 running。
func TestComponentEndpointIotEdgeDBUnset(t *testing.T) {
	cfg := &config.ConfigurationStruct{}

	if got := componentEndpoint(cfg, seed.SystemComponentIotEdgeDBId); got != "" {
		t.Fatalf("iotedgedb endpoint = %q", got)
	}
}
