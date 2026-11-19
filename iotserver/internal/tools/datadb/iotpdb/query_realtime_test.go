package iotpdb

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"iotserver/internal/dtos"
	"iotserver/internal/models"
	"iotserver/internal/tools/iotp"
)

// 最新值走 realtime 接口：一次请求返回所有字段，不再逐属性发 SQL。
func TestGetDeviceLatestPropertiesUsesRealtime(t *testing.T) {
	var queryCalls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/realtime/42":
			if got := r.Header.Get("Authorization"); got != "Bearer tok" {
				t.Fatalf("authorization = %q", got)
			}
			_, _ = io.WriteString(w, `{
				"id": 42,
				"fields": [
					{"name": "temperature", "type": "F64", "value": 12.5, "time": 1794908662017416807},
					{"name": "switch", "type": "BOOL", "value": true, "time": 0}
				],
				"updated_at": 1794908662017416807
			}`)
		case "/api/v1/query":
			queryCalls++
			_, _ = io.WriteString(w, `{"rows":[]}`)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	c := newClient(iotp.New(srv.URL, "tok", "iotserver", 5*time.Second), nil, nil, false)
	// 预置设备缓存，避免 resolve 访问元数据库。
	c.devices["dev1"] = cachedDevice{at: time.Now(), iotpID: 42}

	values, err := c.GetDeviceLatestProperties(context.Background(),
		dtos.ThingModelPropertyDataRequest{}, models.Device{Id: "dev1"})
	if err != nil {
		t.Fatalf("GetDeviceLatestProperties: %v", err)
	}
	if queryCalls != 0 {
		t.Fatalf("latest values should not query SQL, got %d calls", queryCalls)
	}
	if got := values["temperature"]; got.Value != 12.5 || got.Time != 1794908662017 {
		t.Fatalf("temperature = %+v", got)
	}
	// 字段时间为 0 时用 updated_at 兜底。
	if got := values["switch"]; got.Value != true || got.Time != 1794908662017 {
		t.Fatalf("switch = %+v", got)
	}
}

func TestGetDeviceLatestPropertiesMissingDevice(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer srv.Close()

	c := newClient(iotp.New(srv.URL, "tok", "iotserver", 5*time.Second), nil, nil, false)
	c.devices["dev1"] = cachedDevice{at: time.Now(), iotpID: 42}

	values, err := c.GetDeviceLatestProperties(context.Background(),
		dtos.ThingModelPropertyDataRequest{}, models.Device{Id: "dev1"})
	if err != nil {
		t.Fatalf("404 should mean no data: %v", err)
	}
	if len(values) != 0 {
		t.Fatalf("values = %+v", values)
	}
}
