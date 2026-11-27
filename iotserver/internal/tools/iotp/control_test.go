package iotp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func newWSTestServer(t *testing.T, code int, message string) (*httptest.Server, *wsWriteFrame) {
	t.Helper()
	got := &wsWriteFrame{}
	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade: %v", err)
			return
		}
		defer conn.Close()
		if err := conn.ReadJSON(got); err != nil {
			t.Errorf("read write frame: %v", err)
			return
		}
		_ = conn.WriteJSON(wsWriteResponse{Code: code, Data: got.Id, Val: got.Val, Message: message})
	}))
	return srv, got
}

func wsURLOf(srv *httptest.Server) string {
	return "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws/"
}

func newRouteServer(t *testing.T, device RouteDevice) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/devices/route" {
			t.Errorf("unexpected route request %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		var req struct {
			DeviceIds []int64 `json:"device_ids"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		if len(req.DeviceIds) > 0 {
			device.DeviceId = req.DeviceIds[0]
		}
		_ = json.NewEncoder(w).Encode(RouteResponse{Mode: "cluster", Devices: []RouteDevice{device}})
	}))
}

func TestWriteControlSendsWriteFrame(t *testing.T) {
	wsSrv, got := newWSTestServer(t, http.StatusOK, "")
	defer wsSrv.Close()
	routeSrv := newRouteServer(t, RouteDevice{
		Online: true,
		Nodes:  []RouteNode{{Id: "n1", WSURL: wsURLOf(wsSrv)}},
	})
	defer routeSrv.Close()

	client := New(routeSrv.URL, "", "iotserver", time.Second)
	if err := client.WriteControl(context.Background(), 57369006, "temperature", "250"); err != nil {
		t.Fatalf("WriteControl: %v", err)
	}
	if got.Type != "write" || got.Id != "57369006.temperature" || got.Val != "250" {
		t.Fatalf("unexpected write frame: %+v", got)
	}
}

func TestWriteControlUsesIotpFieldName(t *testing.T) {
	cases := map[string]string{
		"flow-rate": "flow_rate",
		"name":      "f_name",
		"time":      "f_time",
	}
	for field, want := range cases {
		wsSrv, got := newWSTestServer(t, http.StatusOK, "")
		routeSrv := newRouteServer(t, RouteDevice{
			Online: true,
			Nodes:  []RouteNode{{Id: "n1", WSURL: wsURLOf(wsSrv)}},
		})
		client := New(routeSrv.URL, "", "iotserver", time.Second)
		if err := client.WriteControl(context.Background(), 1, field, "1"); err != nil {
			t.Fatalf("%s: WriteControl: %v", field, err)
		}
		if got.Id != "1."+want {
			t.Fatalf("%s: id = %q, want %q", field, got.Id, "1."+want)
		}
		wsSrv.Close()
		routeSrv.Close()
	}
}

func TestWriteControlRouteReason(t *testing.T) {
	reason := "no_gateway_mapping"
	routeSrv := newRouteServer(t, RouteDevice{Online: false, Reason: &reason})
	defer routeSrv.Close()

	client := New(routeSrv.URL, "", "iotserver", time.Second)
	err := client.WriteControl(context.Background(), 1, "temperature", "1")
	if err == nil || !strings.Contains(err.Error(), "未绑定网关") {
		t.Fatalf("expected no_gateway_mapping error, got %v", err)
	}
}

func TestWriteControlGatewayRejects(t *testing.T) {
	wsSrv, _ := newWSTestServer(t, http.StatusBadRequest, "Write Error")
	defer wsSrv.Close()
	routeSrv := newRouteServer(t, RouteDevice{
		Online: true,
		Nodes:  []RouteNode{{Id: "n1", WSURL: wsURLOf(wsSrv)}},
	})
	defer routeSrv.Close()

	client := New(routeSrv.URL, "", "iotserver", time.Second)
	err := client.WriteControl(context.Background(), 1, "temperature", "1")
	if err == nil || !strings.Contains(err.Error(), "Write Error") {
		t.Fatalf("expected gateway rejection, got %v", err)
	}
}

func TestWriteControlMultipleNodes(t *testing.T) {
	routeSrv := newRouteServer(t, RouteDevice{
		Online: true,
		Nodes: []RouteNode{
			{Id: "n1", WSURL: "ws://127.0.0.1:1/ws/"},
			{Id: "n2", WSURL: "ws://127.0.0.1:2/ws/"},
		},
	})
	defer routeSrv.Close()

	client := New(routeSrv.URL, "", "iotserver", time.Second)
	err := client.WriteControl(context.Background(), 1, "temperature", "1")
	if err == nil || !strings.Contains(err.Error(), "多个节点") {
		t.Fatalf("expected multiple nodes error, got %v", err)
	}
}
