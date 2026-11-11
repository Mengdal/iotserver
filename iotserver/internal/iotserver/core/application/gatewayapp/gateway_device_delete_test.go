package gatewayapp

import (
	"context"
	"path/filepath"
	"strconv"
	"testing"

	"iotserver/internal/dtos"
	coreContainer "iotserver/internal/iotserver/core/container"
	"iotserver/internal/iotserver/core/infrastructure/sqlite"
	interfaces "iotserver/internal/iotserver/core/interface"
	"iotserver/internal/models"
	"iotserver/internal/pkg/di"
	"iotserver/internal/pkg/logger"
	"iotserver/internal/pkg/tenant"
	"iotserver/internal/tools/iotp"
)

type fakeAlertRuleApp struct {
	interfaces.AlertRuleApp
	blocked map[string]bool
}

func (f fakeAlertRuleApp) CheckRuleByDeviceId(_ context.Context, deviceID string) error {
	if f.blocked[deviceID] {
		return context.Canceled
	}
	return nil
}

type fakeDeviceApp struct {
	interfaces.DeviceItf
	db interfaces.DBClient
}

func (f fakeDeviceApp) DeleteDeviceById(ctx context.Context, id string) error {
	return f.db.WithCtx(ctx).DeleteDeviceById(id)
}

func TestDeleteGatewayDevices(t *testing.T) {
	lc := logger.NewMockClient()
	c, err := sqlite.NewClient(dtos.Configuration{DataSource: filepath.Join(t.TempDir(), "core.db")}, lc)
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	t.Cleanup(c.CloseSession)
	if err := c.MigrateSystemTables(); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	ctx := tenant.With(context.Background(), tenant.Context{TenantID: "t1"})
	for _, d := range []models.Device{
		{Id: "d1", Name: "网关设备1", GatewayId: "g1", TenantField: models.TenantField{TenantId: "t1"}},
		{Id: "d2", Name: "网关设备2", GatewayId: "g1", TenantField: models.TenantField{TenantId: "t1"}},
		{Id: "d3", Name: "其他网关设备", GatewayId: "g2", TenantField: models.TenantField{TenantId: "t1"}},
		// d4 是软网关设备：Device.GatewayId 为空，只能通过 collect_device_config 找到。
		{Id: "d4", Name: "软网关设备", TenantField: models.TenantField{TenantId: "t1"}},
	} {
		if err := c.Pool.Table(models.TableDevice).Create(&d).Error; err != nil {
			t.Fatalf("create device %s: %v", d.Id, err)
		}
	}
	softKey := strconv.FormatInt(iotp.DeviceID("d4"), 10)
	if err := c.Pool.Table(models.TableCollectDeviceConfig).Create(&models.CollectDeviceConfig{
		DeviceId: softKey, ChannelId: "1", GatewayId: "g1", Enabled: true,
		TenantField: models.TenantField{TenantId: "t1"},
	}).Error; err != nil {
		t.Fatalf("create collect device config: %v", err)
	}
	if err := c.Pool.Table(models.TableCollectPointConfig).Create(&models.CollectPointConfig{
		DeviceId: softKey, TagId: "t1", Enabled: true,
		TenantField: models.TenantField{TenantId: "t1"},
	}).Error; err != nil {
		t.Fatalf("create collect point config: %v", err)
	}

	dic := di.NewContainer(di.ServiceConstructorMap{})
	dic.Update(di.ServiceConstructorMap{
		coreContainer.AlertRuleAppName: func(get di.Get) interface{} {
			return fakeAlertRuleApp{blocked: map[string]bool{}}
		},
		coreContainer.DeviceItfName: func(get di.Get) interface{} {
			return fakeDeviceApp{db: c}
		},
	})
	app := &gatewayApp{dic: dic, dbClient: c, lc: lc}

	if err := app.deleteGatewayDevices(ctx, "g1"); err != nil {
		t.Fatalf("deleteGatewayDevices: %v", err)
	}
	list, _, err := c.WithCtx(ctx).DevicesSearch(0, -1, dtos.DeviceSearchQueryRequest{
		BaseSearchConditionQuery: dtos.BaseSearchConditionQuery{IsAll: true},
	})
	if err != nil {
		t.Fatalf("search devices: %v", err)
	}
	if len(list) != 1 || list[0].Id != "d3" {
		t.Fatalf("剩余设备 = %+v, want d3", list)
	}
	points, err := c.WithCtx(ctx).CollectPointConfigsByDevice(softKey)
	if err != nil {
		t.Fatalf("query point configs: %v", err)
	}
	if len(points) != 0 {
		t.Fatalf("软网关设备的点表应被清理: %+v", points)
	}
}

func TestDeleteGatewayDevicesBlockedByAlert(t *testing.T) {
	lc := logger.NewMockClient()
	c, err := sqlite.NewClient(dtos.Configuration{DataSource: filepath.Join(t.TempDir(), "core.db")}, lc)
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	t.Cleanup(c.CloseSession)
	if err := c.MigrateSystemTables(); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	ctx := tenant.With(context.Background(), tenant.Context{TenantID: "t1"})
	device := models.Device{Id: "d1", Name: "网关设备1", GatewayId: "g1", TenantField: models.TenantField{TenantId: "t1"}}
	if err := c.Pool.Table(models.TableDevice).Create(&device).Error; err != nil {
		t.Fatalf("create device: %v", err)
	}

	dic := di.NewContainer(di.ServiceConstructorMap{})
	dic.Update(di.ServiceConstructorMap{
		coreContainer.AlertRuleAppName: func(get di.Get) interface{} {
			return fakeAlertRuleApp{blocked: map[string]bool{"d1": true}}
		},
		coreContainer.DeviceItfName: func(get di.Get) interface{} {
			return fakeDeviceApp{db: c}
		},
	})
	app := &gatewayApp{dic: dic, dbClient: c, lc: lc}

	if err := app.deleteGatewayDevices(ctx, "g1"); err == nil {
		t.Fatal("设备被告警引用时应中止删除")
	}
	got, err := c.WithCtx(ctx).DeviceById("d1")
	if err != nil || got.Id != "d1" {
		t.Fatalf("设备不应被删除: %+v err=%v", got, err)
	}
}
