package driverapp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"iotserver/internal/iotserver/core/config"
	"iotserver/internal/iotserver/core/container"
	bootstrapConfig "iotserver/internal/pkg/config"
	"iotserver/internal/pkg/di"
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

// 日志源默认走部署路径，允许配置覆盖；Path 为空表示禁用。
func TestComponentLogSources(t *testing.T) {
	sources := componentLogSources(&config.ConfigurationStruct{})
	if got := sources[seed.SystemComponentMosquittoId].Path; got != "/var/log/iotserver/mosquitto.log" {
		t.Fatalf("mosquitto default log = %q", got)
	}
	if _, ok := sources[seed.SystemComponentIotEdgeDBId]; ok {
		t.Fatal("iotedgedb 是远程时序库，默认不应配置本机日志")
	}

	cfg := &config.ConfigurationStruct{ComponentLogs: config.ComponentLogsInfo{Sources: []config.ComponentLogSource{
		{Id: seed.SystemComponentMosquittoId, Path: "/tmp/mosquitto.log"},
		{Id: seed.SystemComponentIotpGWId, Path: ""},
		{Id: "custom", Name: "Custom", Path: "/tmp/custom.log"},
	}}}
	sources = componentLogSources(cfg)
	if got := sources[seed.SystemComponentMosquittoId].Path; got != "/tmp/mosquitto.log" {
		t.Fatalf("mosquitto override = %q", got)
	}
	if _, ok := sources[seed.SystemComponentIotpGWId]; ok {
		t.Fatal("空 Path 应禁用默认日志源")
	}
	if got := sources["custom"].Path; got != "/tmp/custom.log" {
		t.Fatalf("custom log = %q", got)
	}

	// 显式配置（包括空 Path 禁用）要能和「没配置」区分开，供 IotEdgeDB
	// 决定是否回落到远程审计日志。
	if _, ok := explicitComponentLogSource(cfg, seed.SystemComponentIotEdgeDBId); ok {
		t.Fatal("未配置 iotedgedb 时不应返回显式日志源")
	}
	iotEdgeCfg := &config.ConfigurationStruct{ComponentLogs: config.ComponentLogsInfo{Sources: []config.ComponentLogSource{
		{Id: seed.SystemComponentIotEdgeDBId, Path: ""},
	}}}
	source, ok := explicitComponentLogSource(iotEdgeCfg, seed.SystemComponentIotEdgeDBId)
	if !ok || source.Path != "" {
		t.Fatalf("空 Path 也应识别为显式禁用，ok=%v path=%q", ok, source.Path)
	}
}

func TestComponentLogsTail(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "iotpgw.log")
	if err := os.WriteFile(logPath, []byte("line1\nline2\nline3\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := &config.ConfigurationStruct{ComponentLogs: config.ComponentLogsInfo{Sources: []config.ComponentLogSource{
		{Id: seed.SystemComponentIotpGWId, Name: "iotpGW", Path: logPath},
	}}}
	dic := di.NewContainer(di.ServiceConstructorMap{
		container.ConfigurationName: func(get di.Get) interface{} { return cfg },
	})
	app := &driverLibApp{dic: dic}

	result, err := app.ComponentLogs(context.Background(), seed.SystemComponentIotpGWId, 2)
	if err != nil {
		t.Fatalf("component logs: %v", err)
	}
	if !result.Exists || len(result.Lines) != 2 || result.Lines[0] != "line2" || result.Lines[1] != "line3" {
		t.Fatalf("unexpected result: %+v", result)
	}

	// 文件不存在不是接口错误，返回 Exists=false 供前端提示。
	if err = os.Remove(logPath); err != nil {
		t.Fatal(err)
	}
	result, err = app.ComponentLogs(context.Background(), seed.SystemComponentIotpGWId, 2)
	if err != nil {
		t.Fatalf("missing log should not fail: %v", err)
	}
	if result.Exists || len(result.Lines) != 0 {
		t.Fatalf("missing log result: %+v", result)
	}
}

// iotpGW 用 HTTP 探活：任意 HTTP 响应（含 404）都算 running，
// 连不上算 stopped，空地址算 unknown。
func TestProbeHTTPEndpoint(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	if got := probeHTTPEndpoint(srv.URL); got != "running" {
		t.Fatalf("响应 404 也应视为 running，实际 %q", got)
	}
	if got := probeHTTPEndpoint(strings.TrimPrefix(srv.URL, "http://")); got != "running" {
		t.Fatalf("无协议前缀时应补 http://，实际 %q", got)
	}
	srv.Close()

	dead := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	deadURL := dead.URL
	dead.Close()
	if got := probeHTTPEndpoint(deadURL); got != "stopped" {
		t.Fatalf("端口关闭应为 stopped，实际 %q", got)
	}
	if got := probeHTTPEndpoint(""); got != "unknown" {
		t.Fatalf("空地址应为 unknown，实际 %q", got)
	}
	if got := probeHTTPEndpoint("http://"); got != "unknown" {
		t.Fatalf("非法地址应为 unknown，实际 %q", got)
	}

	// 自签证书（内部 CA）说明端口是通的，不能报 stopped
	tlsSrv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer tlsSrv.Close()
	if got := probeHTTPEndpoint(tlsSrv.URL); got != "unknown" {
		t.Fatalf("证书不受信任应为 unknown，实际 %q", got)
	}
}

func TestFormatAuditEntry(t *testing.T) {
	line := formatAuditEntry(map[string]any{
		"timestamp": "2026-11-16T10:00:00Z",
		"actor":     "admin",
		"event":     "device_added",
		"target":    "db.table.device",
		"detail":    "批量注册 3 台",
	})
	for _, want := range []string{"2026-11-16T10:00:00Z", "admin", "device_added", "db.table.device", "批量注册 3 台"} {
		if !strings.Contains(line, want) {
			t.Fatalf("审计行缺少 %q: %s", want, line)
		}
	}
	// 字段完全未知时退回 JSON，不能丢内容。
	raw := formatAuditEntry(map[string]any{"foo": "bar"})
	if !strings.Contains(raw, `"foo":"bar"`) {
		t.Fatalf("未知字段应退回 JSON: %s", raw)
	}
}

// 未配置 BaseURL 时外部时序库地址为空，前端显示 unknown，而不是误报 running。
func TestComponentEndpointIotEdgeDBUnset(t *testing.T) {
	cfg := &config.ConfigurationStruct{}

	if got := componentEndpoint(cfg, seed.SystemComponentIotEdgeDBId); got != "" {
		t.Fatalf("iotedgedb endpoint = %q", got)
	}
}
