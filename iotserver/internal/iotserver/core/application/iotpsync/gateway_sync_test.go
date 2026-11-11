package iotpsync

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"iotserver/internal/dtos"
	interfaces "iotserver/internal/iotserver/core/interface"
	"iotserver/internal/models"
	"iotserver/internal/pkg/logger"
	"iotserver/internal/tools/iotp"
)

type fakeGatewayDB struct {
	interfaces.DBClient
	gateways []models.Gateway
	configs  []models.CollectDeviceConfig
	devices  []models.Device
}

func (f *fakeGatewayDB) WithCtx(context.Context) interfaces.DBClient { return f }

func (f *fakeGatewayDB) GatewaySearch(int, int, dtos.GatewaySearchQueryRequest) ([]models.Gateway, uint32, error) {
	return f.gateways, uint32(len(f.gateways)), nil
}

func (f *fakeGatewayDB) CollectDeviceConfigs() ([]models.CollectDeviceConfig, error) {
	return f.configs, nil
}

func (f *fakeGatewayDB) DevicesSearch(int, int, dtos.DeviceSearchQueryRequest) ([]models.Device, uint32, error) {
	return f.devices, uint32(len(f.devices)), nil
}

func (f *fakeGatewayDB) ProductById(id string) (models.Product, error) {
	return models.Product{Id: id}, nil
}

func TestReconcileGateways(t *testing.T) {
	var (
		mu          sync.Mutex
		gateways    []string
		registered  []map[string]any
		assignments []map[string]any
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/meta/gateways":
			_, _ = w.Write([]byte(`[]`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/meta/gateways":
			var body struct {
				Name string `json:"name"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			gateways = append(gateways, body.Name)
			_, _ = w.Write([]byte(`{"ok":true}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/meta/databases":
			_, _ = w.Write([]byte(`{"ok":true}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/meta/tables":
			_, _ = w.Write([]byte(`{"ok":true}`))
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/api/v1/meta/tables/"):
			_, _ = w.Write([]byte(`[]`))
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/api/v1/meta/devices/"):
			http.NotFound(w, r)
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/meta/devices":
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			registered = append(registered, body)
			_, _ = w.Write([]byte(`{"id":123}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/meta/devices/gateway":
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			assignments = append(assignments, body)
			_, _ = w.Write([]byte(`{"ok":true}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	device := models.Device{
		TenantField: models.TenantField{TenantId: "t1"},
		Id:          "dev1",
		Name:        "设备1",
		ProductId:   "p1",
	}
	db := &fakeGatewayDB{
		gateways: []models.Gateway{{
			Id: "g1", Sn: "soft-1", Name: "软网关1", Source: models.GatewaySourceIoserver,
		}},
		configs: []models.CollectDeviceConfig{{
			DeviceId:  strconv.FormatInt(iotp.DeviceID(device.Id), 10),
			GatewayId: "g1",
		}},
		devices: []models.Device{device},
	}
	service := &Service{
		enabled:  true,
		dataType: "iotp",
		dbClient: db,
		lc:       logger.NewMockClient(),
		client:   iotp.New(srv.URL, "tok", "iotserver", time.Second),
	}
	if !service.GatewaySyncEnabled() {
		t.Fatal("GatewaySyncEnabled = false")
	}

	if err := service.reconcileGateways(context.Background()); err != nil {
		t.Fatalf("reconcileGateways: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(gateways) != 1 || gateways[0] != "soft-1" {
		t.Fatalf("声明的网关 = %v", gateways)
	}
	if len(registered) != 1 {
		t.Fatalf("注册设备次数 = %d, want 1", len(registered))
	}
	if registered[0]["table"] != "p1" {
		t.Fatalf("注册表 = %v, want p1", registered[0]["table"])
	}
	if len(assignments) != 1 {
		t.Fatalf("盖章次数 = %d, want 1", len(assignments))
	}
	if assignments[0]["gateway"] != "soft-1" {
		t.Fatalf("盖章网关 = %v", assignments[0]["gateway"])
	}
	ids, _ := assignments[0]["device_ids"].([]any)
	if len(ids) != 1 || int64(ids[0].(float64)) != iotp.DeviceID(device.Id) {
		t.Fatalf("盖章设备 = %v", assignments[0]["device_ids"])
	}
}

// 设备已注册且归属正确时，对账不应产生任何写请求。
func TestReconcileGatewaysNoChanges(t *testing.T) {
	device := models.Device{
		TenantField: models.TenantField{TenantId: "t1"},
		Id:          "dev1",
		Name:        "设备1",
		ProductId:   "p1",
	}
	gatewayName := "soft-1"
	var writes int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/meta/gateways":
			_ = json.NewEncoder(w).Encode([]iotp.GatewayInfo{{Name: "soft-1"}})
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/api/v1/meta/tables/"):
			_ = json.NewEncoder(w).Encode([]iotp.DeviceInfo{{
				ID: iotp.DeviceID(device.Id), Table: "p1", Gateway: &gatewayName,
			}})
		default:
			writes++
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	service := &Service{
		enabled:  true,
		dataType: "iotp",
		dbClient: &fakeGatewayDB{
			gateways: []models.Gateway{{Id: "g1", Sn: "soft-1", Source: models.GatewaySourceIoserver}},
			configs: []models.CollectDeviceConfig{{
				DeviceId: strconv.FormatInt(iotp.DeviceID(device.Id), 10), GatewayId: "g1",
			}},
			devices: []models.Device{device},
		},
		lc:     logger.NewMockClient(),
		client: iotp.New(srv.URL, "tok", "iotserver", time.Second),
	}
	if err := service.reconcileGateways(context.Background()); err != nil {
		t.Fatalf("reconcileGateways: %v", err)
	}
	if writes != 0 {
		t.Fatalf("归属正确时不应写 iotp，实际 %d 次", writes)
	}
}

// 设备归属不一致时只盖章，不重新注册设备。
func TestReconcileGatewaysStampsMismatch(t *testing.T) {
	device := models.Device{
		TenantField: models.TenantField{TenantId: "t1"},
		Id:          "dev1",
		Name:        "设备1",
		ProductId:   "p1",
	}
	otherGateway := "other"
	var (
		mu          sync.Mutex
		registered  int
		assignments []map[string]any
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/meta/gateways":
			_ = json.NewEncoder(w).Encode([]iotp.GatewayInfo{{Name: "soft-1"}})
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/api/v1/meta/tables/"):
			_ = json.NewEncoder(w).Encode([]iotp.DeviceInfo{{
				ID: iotp.DeviceID(device.Id), Table: "p1", Gateway: &otherGateway,
			}})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/meta/devices":
			registered++
			_, _ = w.Write([]byte(`{"id":123}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/meta/devices/gateway":
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			assignments = append(assignments, body)
			_, _ = w.Write([]byte(`{"ok":true}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	service := &Service{
		enabled:  true,
		dataType: "iotp",
		dbClient: &fakeGatewayDB{
			gateways: []models.Gateway{{Id: "g1", Sn: "soft-1", Source: models.GatewaySourceIoserver}},
			configs: []models.CollectDeviceConfig{{
				DeviceId: strconv.FormatInt(iotp.DeviceID(device.Id), 10), GatewayId: "g1",
			}},
			devices: []models.Device{device},
		},
		lc:     logger.NewMockClient(),
		client: iotp.New(srv.URL, "tok", "iotserver", time.Second),
	}
	if err := service.reconcileGateways(context.Background()); err != nil {
		t.Fatalf("reconcileGateways: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if registered != 0 {
		t.Fatalf("归属不一致不应重新注册设备，实际 %d 次", registered)
	}
	if len(assignments) != 1 || assignments[0]["gateway"] != "soft-1" {
		t.Fatalf("盖章 = %+v", assignments)
	}
}

func TestGatewayDeletedClearsThenDeletes(t *testing.T) {
	var (
		mu     sync.Mutex
		events []string
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/meta/devices/gateway":
			body, _ := io.ReadAll(r.Body)
			var payload map[string]any
			_ = json.Unmarshal(body, &payload)
			if payload["gateway"] == nil {
				events = append(events, "clear")
			}
			_, _ = w.Write([]byte(`{"ok":true}`))
		case r.Method == http.MethodDelete && r.URL.Path == "/api/v1/meta/gateways/soft-1":
			events = append(events, "delete")
			w.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	service := &Service{
		enabled:  true,
		dataType: "iotp",
		lc:       logger.NewMockClient(),
		client:   iotp.New(srv.URL, "tok", "iotserver", time.Second),
	}
	service.GatewayDeleted(models.Gateway{
		Id: "g1", Sn: "soft-1", Source: models.GatewaySourceIoserver,
	}, []int64{7})

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		done := len(events) >= 2
		mu.Unlock()
		if done {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	mu.Lock()
	defer mu.Unlock()
	want := []string{"clear", "delete"}
	if len(events) != len(want) || events[0] != want[0] || events[1] != want[1] {
		t.Fatalf("事件顺序 = %v, want %v", events, want)
	}
}
