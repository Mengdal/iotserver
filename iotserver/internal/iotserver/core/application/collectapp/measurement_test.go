package collectapp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	"iotserver/internal/dtos"
	"iotserver/internal/iotserver/core/infrastructure/sqlite"
	"iotserver/internal/models"
	"iotserver/internal/pkg/collectgateway"
	"iotserver/internal/pkg/logger"
	"iotserver/internal/pkg/tenant"
	"iotserver/internal/tools/collectengine"
	"iotserver/internal/tools/iotp"
)

func TestMeasurementForDevice(t *testing.T) {
	if got := iotp.DeviceTable("67126069"); got != "67126069" {
		t.Fatalf("有产品设备 measurement = %q, want 67126069", got)
	}
	if got := iotp.DeviceTable(""); got != iotp.DefaultTable {
		t.Fatalf("无产品设备 measurement = %q, want %s", got, iotp.DefaultTable)
	}
}

type measurementFixture struct {
	db       *sqlite.Client
	service  *service
	server   *httptest.Server
	mu       sync.Mutex
	created  map[string]int
	updated  map[string]int
	bindings []collectengine.Device
}

func newMeasurementFixture(t *testing.T) *measurementFixture {
	t.Helper()
	f := &measurementFixture{created: map[string]int{}, updated: map[string]int{}}
	f.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/channels":
			_, _ = w.Write([]byte(`{"code":200,"message":"ok","data":[{"id":1,"channelCode":"C1"}]}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/devices":
			bindings := f.bindings
			if bindings == nil {
				bindings = []collectengine.Device{}
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"code": 200, "message": "ok", "data": bindings})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/devices":
			if err := r.ParseMultipartForm(1 << 20); err != nil {
				t.Errorf("ParseMultipartForm: %v", err)
			}
			measurement := r.FormValue("measurement")
			f.created[measurement]++
			iotpID, _ := strconv.ParseInt(r.FormValue("iotpId"), 10, 64)
			channelID, _ := strconv.ParseInt(r.FormValue("channelId"), 10, 64)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"code": 200, "message": "ok",
				"data": map[string]any{"id": 9, "channelId": channelID, "iotpId": iotpID, "measurement": measurement},
			})
		case r.Method == http.MethodPut && strings.HasPrefix(r.URL.Path, "/api/v1/devices/"):
			if err := r.ParseMultipartForm(1 << 20); err != nil {
				t.Errorf("ParseMultipartForm: %v", err)
			}
			f.updated[r.FormValue("measurement")]++
			_, _ = w.Write([]byte(`{"code":200,"message":"ok"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(f.server.Close)

	lc := logger.NewMockClient()
	c, err := sqlite.NewClient(dtos.Configuration{DataSource: filepath.Join(t.TempDir(), "core.db")}, lc)
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	t.Cleanup(c.CloseSession)
	if err := c.MigrateSystemTables(); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if err := c.Pool.Table(models.TableGateway).Create(&models.Gateway{
		Id: "g1", Sn: "1234567890", Source: models.GatewaySourceIoserver,
		EngineBaseURL: f.server.URL, TenantField: models.TenantField{TenantId: "t1"},
	}).Error; err != nil {
		t.Fatalf("create gateway: %v", err)
	}
	if err := c.Pool.Table(models.TableDevice).Create(&models.Device{
		Id: "12345678", Name: "设备1", ProductId: "67126069",
		TenantField: models.TenantField{TenantId: "t1"},
	}).Error; err != nil {
		t.Fatalf("create device: %v", err)
	}
	f.db = c
	f.service = &service{lc: lc, dbClient: c, engines: make(map[string]*collectengine.Client)}
	return f
}

func TestCollectChannelBindDevicesSendsMeasurement(t *testing.T) {
	f := newMeasurementFixture(t)
	ctx := collectgateway.With(tenant.Platform(context.Background()), "g1")
	if err := f.service.CollectChannelBindDevices(ctx, "1", []string{"12345678"}); err != nil {
		t.Fatalf("CollectChannelBindDevices: %v", err)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.created["67126069"] != 1 {
		t.Fatalf("创建设备 measurement = %v, want 67126069", f.created)
	}
}

func TestCollectChannelBindDevicesUpdatesMeasurement(t *testing.T) {
	f := newMeasurementFixture(t)
	f.bindings = []collectengine.Device{{
		Id: 9, ChannelId: 1, IotpId: iotp.DeviceID("12345678"), Measurement: "cpu",
	}}
	ctx := collectgateway.With(tenant.Platform(context.Background()), "g1")
	if err := f.service.CollectChannelBindDevices(ctx, "1", []string{"12345678"}); err != nil {
		t.Fatalf("CollectChannelBindDevices: %v", err)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.updated["67126069"] != 1 {
		t.Fatalf("更新设备 measurement = %v, want 67126069", f.updated)
	}
}
