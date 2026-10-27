package sqlite

import (
	"context"
	"testing"

	"iotserver/internal/models"
	"iotserver/internal/pkg/tenant"
)

func TestTenantThingsBindingCRUD(t *testing.T) {
	c := newTestClient(t)
	if err := c.MigrateSystemTables(); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	ctx := tenant.Platform(context.Background())

	b, err := c.WithCtx(ctx).AddOrUpdateTenantThingsBinding(models.TenantThingsBinding{
		TenantField:       models.TenantField{TenantId: "tenant-a"},
		ThingsProjectId:   101,
		ThingsProjectName: "Tenant A",
		Status:            models.TenantThingsStatusBound,
	})
	if err != nil {
		t.Fatalf("create binding: %v", err)
	}
	if b.Id == "" || b.ThingsProjectId != 101 || b.Status != models.TenantThingsStatusBound {
		t.Fatalf("unexpected binding: %+v", b)
	}

	got, err := c.WithCtx(ctx).TenantThingsBindingByTenant("tenant-a")
	if err != nil {
		t.Fatalf("get binding: %v", err)
	}
	if got.ThingsProjectName != "Tenant A" {
		t.Fatalf("unexpected binding name: %+v", got)
	}

	if err := c.WithCtx(ctx).UpdateTenantThingsBindingStatus("tenant-a", models.TenantThingsStatusError, "boom", 123); err != nil {
		t.Fatalf("update status: %v", err)
	}
	got, _ = c.WithCtx(ctx).TenantThingsBindingByTenant("tenant-a")
	if got.Status != models.TenantThingsStatusError || got.LastError != "boom" || got.LastSyncAt != 123 {
		t.Fatalf("status not updated: %+v", got)
	}

	if err := c.WithCtx(ctx).DeleteTenantThingsBinding("tenant-a"); err != nil {
		t.Fatalf("delete binding: %v", err)
	}
	if _, err := c.WithCtx(ctx).TenantThingsBindingByTenant("tenant-a"); err == nil {
		t.Fatalf("binding should be deleted")
	}
}

func TestTenantThingsBindingProjectUnique(t *testing.T) {
	c := newTestClient(t)
	if err := c.MigrateSystemTables(); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	ctx := tenant.Platform(context.Background())
	if _, err := c.WithCtx(ctx).AddOrUpdateTenantThingsBinding(models.TenantThingsBinding{
		TenantField:     models.TenantField{TenantId: "tenant-a"},
		ThingsProjectId: 101,
		Status:          models.TenantThingsStatusBound,
	}); err != nil {
		t.Fatalf("create first binding: %v", err)
	}
	if _, err := c.WithCtx(ctx).AddOrUpdateTenantThingsBinding(models.TenantThingsBinding{
		TenantField:     models.TenantField{TenantId: "tenant-b"},
		ThingsProjectId: 101,
		Status:          models.TenantThingsStatusBound,
	}); err == nil {
		t.Fatalf("same things project must not bind to two tenants")
	}
}

func TestGatewaySNGloballyUniqueAcrossTenants(t *testing.T) {
	c := newTestClient(t)
	if err := c.MigrateSystemTables(); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	ctxA := tenant.With(context.Background(), tenant.Context{TenantID: "tenant-a"})
	if _, err := c.WithCtx(ctxA).AddGateway(models.Gateway{Id: "gw-a", Sn: "SN-SHARED", Name: "A"}); err != nil {
		t.Fatalf("add gateway A: %v", err)
	}
	ctxB := tenant.With(context.Background(), tenant.Context{TenantID: "tenant-b"})
	if _, err := c.WithCtx(ctxB).AddGateway(models.Gateway{Id: "gw-b", Sn: "SN-SHARED", Name: "B"}); err == nil {
		t.Fatalf("duplicate SN across tenants must be rejected")
	}
}

func TestGatewayThingsSyncFields(t *testing.T) {
	c := newTestClient(t)
	if err := c.MigrateSystemTables(); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	ctx := tenant.With(context.Background(), tenant.Context{TenantID: "tenant-a"})
	g, err := c.WithCtx(ctx).AddGateway(models.Gateway{
		Id:               "gw-a",
		Sn:               "SN-THINGS-1",
		Name:             "old",
		Protocol:         models.GatewayProtocolEdge,
		Source:           models.GatewaySourceThings,
		ThingsGatewayId:  9001,
		ThingsProjectId:  101,
		ThingsSyncStatus: models.GatewayThingsSyncSynced,
	})
	if err != nil {
		t.Fatalf("add gateway: %v", err)
	}
	if g.Secret != "" {
		t.Fatalf("things gateway should not generate a local secret")
	}
	g.Name = "new"
	g.Model = "M1"
	if err := c.WithCtx(ctx).UpdateGatewayThingsSync(g); err != nil {
		t.Fatalf("update sync: %v", err)
	}
	got, err := c.WithCtx(ctx).GatewayByThingsID(9001)
	if err != nil {
		t.Fatalf("by things id: %v", err)
	}
	if got.Name != "new" || got.Model != "M1" || got.Source != models.GatewaySourceThings {
		t.Fatalf("unexpected gateway: %+v", got)
	}
	if err := c.WithCtx(ctx).UpdateGatewayThingsStatus(got.Id, models.GatewayThingsSyncMissing, "gone", 0); err != nil {
		t.Fatalf("update status: %v", err)
	}
	got, _ = c.WithCtx(ctx).GatewayById(got.Id)
	if got.ThingsSyncStatus != models.GatewayThingsSyncMissing || got.ThingsLastError != "gone" {
		t.Fatalf("sync status not updated: %+v", got)
	}
}
