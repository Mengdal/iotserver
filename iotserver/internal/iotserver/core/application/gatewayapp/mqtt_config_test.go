package gatewayapp

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"iotserver/internal/models"
)

func TestConfigureEngineMQTT(t *testing.T) {
	var updated bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/mqtt/config" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case http.MethodGet:
			_, _ = w.Write([]byte(`{"code":200,"message":"ok","data":{"IP":"10.0.0.1","port":1883,"username":"old","hasPassword":true,"clientid":"iotpGW"}}`))
		case http.MethodPut:
			updated = true
			if err := r.ParseMultipartForm(1 << 20); err != nil {
				t.Fatalf("ParseMultipartForm() error = %v", err)
			}
			if got := r.FormValue("IP"); got != "127.0.0.1" {
				t.Errorf("IP = %q, want 网关地址 127.0.0.1", got)
			}
			if got := r.FormValue("clientid"); got != "1234567890" {
				t.Errorf("clientid = %q, want SN", got)
			}
			if got := r.FormValue("username"); got != "" {
				t.Errorf("username = %q, want empty", got)
			}
			if got := r.FormValue("password"); got != "" {
				t.Errorf("password = %q, want empty", got)
			}
			if got := r.FormValue("port"); got != "1883" {
				t.Errorf("port = %q, want 1883", got)
			}
			_, _ = w.Write([]byte(`{"code":200,"message":"ok"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	app := &gatewayApp{}
	if err := app.configureEngineMQTT(models.Gateway{
		Sn:               "1234567890",
		EngineBaseURL:    srv.URL,
		EngineTimeoutSec: 2,
	}); err != nil {
		t.Fatalf("configureEngineMQTT: %v", err)
	}
	if !updated {
		t.Fatal("未调用 MQTT 配置更新接口")
	}
}
