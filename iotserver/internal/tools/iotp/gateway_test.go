package iotp

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestGatewayAPIs(t *testing.T) {
	var (
		assignBody  map[string]any
		gatewayPost bool
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/meta/gateways":
			gatewayPost = true
			_, _ = w.Write([]byte(`{"ok":true,"name":"soft-1"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/meta/gateways":
			_, _ = w.Write([]byte(`[{"name":"soft-1","device_count":2}]`))
		case r.Method == http.MethodDelete && r.URL.Path == "/api/v1/meta/gateways/soft-1":
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/meta/devices/gateway":
			raw, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(raw, &assignBody)
			_, _ = w.Write([]byte(`{"ok":true,"changed":1}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/meta/devices/7":
			_, _ = w.Write([]byte(`{"id":7,"db":"iotserver","table":"p_1","gateway":"soft-1"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	client := New(srv.URL, "tok", "iotserver", time.Second)
	ctx := context.Background()

	if err := client.EnsureGateway(ctx, "soft-1", "一号软网关"); err != nil {
		t.Fatalf("EnsureGateway: %v", err)
	}
	if !gatewayPost {
		t.Fatal("EnsureGateway 未调用 POST /meta/gateways")
	}
	list, err := client.Gateways(ctx)
	if err != nil {
		t.Fatalf("Gateways: %v", err)
	}
	if len(list) != 1 || list[0].Name != "soft-1" || list[0].DeviceCount != 2 {
		t.Fatalf("Gateways = %+v", list)
	}

	if err := client.AssignDeviceGateway(ctx, []int64{7}, "soft-1"); err != nil {
		t.Fatalf("AssignDeviceGateway: %v", err)
	}
	if assignBody["gateway"] != "soft-1" {
		t.Fatalf("归属报文 = %+v", assignBody)
	}

	if err := client.AssignDeviceGateway(ctx, []int64{7}, ""); err != nil {
		t.Fatalf("AssignDeviceGateway clear: %v", err)
	}
	if v, ok := assignBody["gateway"]; !ok || v != nil {
		t.Fatalf("清章报文 = %+v, want gateway=null", assignBody)
	}

	device, err := client.Device(ctx, 7)
	if err != nil {
		t.Fatalf("Device: %v", err)
	}
	if device.Table != "p_1" || device.Gateway == nil || *device.Gateway != "soft-1" {
		t.Fatalf("Device = %+v", device)
	}

	if err := client.DeleteGateway(ctx, "soft-1"); err != nil {
		t.Fatalf("DeleteGateway: %v", err)
	}
	// 404 视为成功
	if err := client.DeleteGateway(ctx, "missing"); err != nil {
		t.Fatalf("DeleteGateway missing: %v", err)
	}
}

func TestEnsureGatewayConflictAndNotControlNode(t *testing.T) {
	mode := ""
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch mode {
		case "conflict":
			w.WriteHeader(http.StatusConflict)
			_, _ = w.Write([]byte(`{"error":"already exists","code":"CONFLICT"}`))
		case "node":
			w.WriteHeader(http.StatusConflict)
			_, _ = w.Write([]byte(`{"error":"not control node","code":"NOT_CONTROL_NODE"}`))
		}
	}))
	defer srv.Close()

	client := New(srv.URL, "tok", "iotserver", time.Second)
	ctx := context.Background()

	mode = "conflict"
	if err := client.EnsureGateway(ctx, "soft-1", ""); err != nil {
		t.Fatalf("409 重复声明应幂等: %v", err)
	}

	mode = "node"
	err := client.EnsureGateway(ctx, "soft-1", "")
	if err == nil {
		t.Fatal("NOT_CONTROL_NODE 不应被当成已存在")
	}
	if !IsNotControlNode(err) {
		t.Fatalf("err = %v, want NOT_CONTROL_NODE", err)
	}
}

func TestIsForbidden(t *testing.T) {
	err := &APIError{Status: http.StatusForbidden, Code: "FORBIDDEN", Message: "denied"}
	if !IsForbidden(err) {
		t.Fatal("IsForbidden = false")
	}
	if IsForbidden(&APIError{Status: http.StatusNotFound}) {
		t.Fatal("404 不应判定为 403")
	}
}
