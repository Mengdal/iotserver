package thingsapp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"iotserver/internal/dtos"
	coreConfig "iotserver/internal/iotserver/core/config"
	coreContainer "iotserver/internal/iotserver/core/container"
	"iotserver/internal/iotserver/core/infrastructure/sqlite"
	"iotserver/internal/models"
	pkgcontainer "iotserver/internal/pkg/container"
	"iotserver/internal/pkg/di"
	"iotserver/internal/pkg/logger"
	"iotserver/internal/pkg/tenant"
	jwt2 "iotserver/internal/tools/jwt"
	"iotserver/internal/tools/things"
)

func writeEnvelope(w http.ResponseWriter, code int, message string, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"code": code, "message": message, "data": data})
}

func TestThingsServiceProjectGatewayFlow(t *testing.T) {
	jwt2.SetJwtSignKey("things-test-key")
	t.Setenv("IOTSERVER_THINGS_KEY", "things-test-key")

	lc := logger.NewClient("thingsapp-test", "ERROR", "")
	c, err := sqlite.NewClient(dtos.Configuration{DataSource: filepath.Join(t.TempDir(), "core.db")}, lc)
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	t.Cleanup(c.CloseSession)
	if err = c.MigrateSystemTables(); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	password, err := things.EncryptPassword("pwd")
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	var createdGateway bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/edge-user/user/apiLogin" {
			if r.Header.Get("Token") != "tok" {
				t.Errorf("%s missing token", r.URL.Path)
			}
		}
		_ = r.ParseForm()
		switch r.URL.Path {
		case "/edge-user/user/apiLogin":
			writeEnvelope(w, 200, "success", map[string]string{"token": "tok"})
		case "/edge-user/project/create":
			writeEnvelope(w, 200, "success", map[string]interface{}{"id": 101, "name": r.Form.Get("name")})
		case "/edge-user/project/list":
			writeEnvelope(w, 200, "success", map[string]interface{}{
				"list": []map[string]interface{}{{"id": 101, "name": "Tenant A"}}, "totalElements": 1,
			})
		case "/edge-mgmt/gateway/list":
			writeEnvelope(w, 200, "success", map[string]interface{}{
				"list": []map[string]interface{}{
					{"id": 9001, "projectId": 101, "sn": "SN-1", "pk": "SN-1", "name": "Gateway A", "model": "M1", "secret": "must-not-leak"},
				},
				"totalRow": 1,
			})
		case "/edge-mgmt/gateway/create":
			createdGateway = true
			if r.Form.Get("secret") != "company-secret" {
				t.Errorf("secret not forwarded: %q", r.Form.Get("secret"))
			}
			writeEnvelope(w, 200, "success", map[string]interface{}{
				"id": 9002, "projectId": 101, "sn": r.Form.Get("sn"), "pk": r.Form.Get("sn"),
				"name": r.Form.Get("name"), "model": r.Form.Get("model"),
			})
		case "/edge-mgmt/gateway/delete":
			writeEnvelope(w, 200, "success", nil)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	for key, value := range map[string]string{
		"things.enabled":  "true",
		"things.baseUrl":  srv.URL,
		"things.username": "svc",
		"things.password": password,
	} {
		if err := c.Pool.Table(models.TableSystemConfig).Where("config_key = ?", key).
			Update("config_value", value).Error; err != nil {
			t.Fatalf("set config %s: %v", key, err)
		}
	}

	dic := di.NewContainer(di.ServiceConstructorMap{})
	dic.Update(di.ServiceConstructorMap{
		pkgcontainer.LoggingClientInterfaceName: func(get di.Get) interface{} { return lc },
		coreContainer.DBClientInterfaceName:     func(get di.Get) interface{} { return c },
		coreContainer.ConfigurationName:         func(get di.Get) interface{} { return &coreConfig.ConfigurationStruct{} },
	})
	itf := New(dic)
	svc, ok := itf.(*service)
	if !ok {
		t.Fatalf("unexpected service type")
	}
	ctx := tenant.Platform(context.Background())

	if _, err := svc.TestConnection(ctx); err != nil {
		t.Fatalf("test connection: %v", err)
	}
	project, err := svc.CreateProject(ctx, "Tenant A", "desc")
	if err != nil || project.Id != 101 {
		t.Fatalf("create project: %+v %v", project, err)
	}
	if err := svc.BindTenantProject(ctx, "tenant-a", project.Id, project.Name); err != nil {
		t.Fatalf("bind: %v", err)
	}
	if b, found, err := svc.GetTenantBinding(ctx, "tenant-a"); err != nil || !found || b.ThingsProjectId != 101 {
		t.Fatalf("binding: %+v found=%v err=%v", b, found, err)
	}

	tg, err := svc.CreateGateway(ctx, 101, dtos.ThingsGatewayCreateRequest{
		Sn: "SN-2", Secret: "company-secret", Name: "Gateway B", Model: "M2",
	})
	if err != nil || tg.Id != 9002 || !createdGateway {
		t.Fatalf("create gateway: %+v err=%v created=%v", tg, err, createdGateway)
	}

	res, err := svc.ReconcileTenant(ctx, "tenant-a")
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if res.Gateways != 1 {
		t.Fatalf("unexpected reconcile result: %+v", res)
	}
	tenantCtx := tenant.With(context.Background(), tenant.Context{TenantID: "tenant-a"})
	list, _, err := c.WithCtx(tenantCtx).GatewaySearch(0, -1, dtos.GatewaySearchQueryRequest{})
	if err != nil {
		t.Fatalf("search gateways: %v", err)
	}
	if len(list) != 1 || list[0].Sn != "SN-1" || list[0].Source != models.GatewaySourceThings ||
		list[0].ThingsGatewayId != 9001 || list[0].ThingsSyncStatus != models.GatewayThingsSyncSynced {
		t.Fatalf("unexpected imported gateway: %+v", list)
	}
	if list[0].Secret != "" {
		t.Fatalf("things gateway must not store secret")
	}

	if err := svc.DeleteGateway(ctx, 101, 9001); err != nil {
		t.Fatalf("delete gateway: %v", err)
	}
}
