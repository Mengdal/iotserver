package tenantapp

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"iotserver/internal/dtos"
	"iotserver/internal/iotserver/core/infrastructure/sqlite"
	interfaces "iotserver/internal/iotserver/core/interface"
	"iotserver/internal/models"
	"iotserver/internal/pkg/logger"
	"iotserver/internal/pkg/seed"
	"iotserver/internal/pkg/tenant"
)

// 外部 things 清理失败不应阻塞本地租户删除。
type failingThings struct {
	interfaces.ThingsItf
}

func (failingThings) GetTenantBinding(context.Context, string) (models.TenantThingsBinding, bool, error) {
	return models.TenantThingsBinding{ThingsProjectId: 44}, true, nil
}

func (failingThings) DeleteProject(context.Context, int64) error {
	return fmt.Errorf("things login failed")
}

func (failingThings) UnbindTenant(context.Context, string) error { return nil }

func TestTenantDeleteIgnoresThingsFailure(t *testing.T) {
	lc := logger.NewMockClient()
	c, err := sqlite.NewClient(dtos.Configuration{DataSource: filepath.Join(t.TempDir(), "core.db")}, lc)
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	t.Cleanup(c.CloseSession)
	if err = c.MigrateSystemTables(); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	pctx := tenant.Platform(context.Background())
	tc, err := c.WithCtx(pctx).AddTenant(models.Tenant{Code: "del_me", Name: "待删租户"})
	if err != nil {
		t.Fatalf("add tenant: %v", err)
	}
	tctx := tenant.With(pctx, tenant.Context{TenantID: tc.Id})
	if _, err = c.WithCtx(tctx).AddSysRole(models.Role{
		Name: seed.RoleTenantUserName, BuiltinKey: seed.BuiltinTenantUser,
		DataScope: models.DataScopeAll, Status: 1,
	}); err != nil {
		t.Fatalf("add tenant role: %v", err)
	}

	app := &tenantApp{dbClient: c, lc: lc, things: failingThings{}}
	if err = app.TenantDelete(pctx, tc.Id); err != nil {
		t.Fatalf("things 清理失败不应阻塞租户删除: %v", err)
	}
	if _, err = c.WithCtx(pctx).TenantById(tc.Id); err == nil {
		t.Fatal("租户应已删除")
	}
	roles, err := c.WithCtx(tctx).SysRoleAll()
	if err != nil {
		t.Fatalf("query roles: %v", err)
	}
	if len(roles) != 0 {
		t.Fatalf("租户角色应随租户删除，剩余 %d 个", len(roles))
	}
}
