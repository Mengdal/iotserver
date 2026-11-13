package sqlite

import (
	"context"
	"testing"

	"iotserver/internal/dtos"
	"iotserver/internal/models"
	"iotserver/internal/pkg/constants"
	"iotserver/internal/pkg/tenant"
)

func TestAlertTenantIsolation(t *testing.T) {
	c := newTestClient(t)
	if err := c.MigrateSystemTables(); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	pctx := tenant.Platform(context.Background())
	ta, err := c.WithCtx(pctx).AddTenant(models.Tenant{Code: "alert_a", Name: "告警租户A"})
	if err != nil {
		t.Fatalf("add tenant A: %v", err)
	}
	tb, err := c.WithCtx(pctx).AddTenant(models.Tenant{Code: "alert_b", Name: "告警租户B"})
	if err != nil {
		t.Fatalf("add tenant B: %v", err)
	}
	ctxA := tenant.With(pctx, tenant.Context{TenantID: ta.Id})
	ctxB := tenant.With(pctx, tenant.Context{TenantID: tb.Id})

	addAlert := func(ctx context.Context, level constants.AlertLevel) {
		t.Helper()
		rule, err := c.WithCtx(ctx).AddAlertRule(models.AlertRule{
			Name: "规则-" + string(level), AlertLevel: level,
			AlertType: constants.DeviceAlertType, Status: constants.RuleStop,
		})
		if err != nil {
			t.Fatalf("add rule: %v", err)
		}
		if _, err = c.WithCtx(ctx).AddAlertList(models.AlertList{
			AlertRuleId: rule.Id, AlertResult: models.MapStringInterface{},
			TriggerTime: 1, Status: constants.Untreated, IsSend: true,
		}); err != nil {
			t.Fatalf("add alert: %v", err)
		}
	}
	addAlert(ctxA, constants.Urgent)
	addAlert(ctxA, constants.Urgent)
	addAlert(ctxB, constants.Important)
	count := func(ctx context.Context, level constants.AlertLevel) int {
		t.Helper()
		plate, err := c.WithCtx(ctx).AlertPlate(0)
		if err != nil {
			t.Fatalf("alert plate: %v", err)
		}
		for _, p := range plate {
			if p.AlertLevel == level {
				return p.Count
			}
		}
		return 0
	}
	if got := count(ctxA, constants.Urgent); got != 2 {
		t.Fatalf("租户A紧急告警应为 2，实际 %d", got)
	}
	if got := count(ctxB, constants.Urgent); got != 0 {
		t.Fatalf("租户B不应看到租户A的紧急告警，实际 %d", got)
	}
	if got := count(pctx, constants.Urgent); got != 2 {
		t.Fatalf("平台视图紧急告警总数应为 2，实际 %d", got)
	}

	// 告警列表同样只能看到本租户的记录。
	listA, totalA, err := c.WithCtx(ctxA).AlertListSearch(0, -1, dtos.AlertSearchQueryRequest{})
	if err != nil {
		t.Fatalf("alert list A: %v", err)
	}
	if totalA != 2 || len(listA) != 2 {
		t.Fatalf("租户A告警列表应为 2，实际 total=%d len=%d", totalA, len(listA))
	}
	listB, totalB, err := c.WithCtx(ctxB).AlertListSearch(0, -1, dtos.AlertSearchQueryRequest{})
	if err != nil {
		t.Fatalf("alert list B: %v", err)
	}
	if totalB != 1 || len(listB) != 1 {
		t.Fatalf("租户B告警列表应为 1，实际 total=%d len=%d", totalB, len(listB))
	}

	// 告警规则列表同样按租户隔离。
	_, rulesA, err := c.WithCtx(ctxA).AlertRuleSearch(0, -1, dtos.AlertRuleSearchQueryRequest{})
	if err != nil {
		t.Fatalf("alert rule search A: %v", err)
	}
	if rulesA != 2 {
		t.Fatalf("租户A告警规则应为 2，实际 %d", rulesA)
	}
	_, rulesB, err := c.WithCtx(ctxB).AlertRuleSearch(0, -1, dtos.AlertRuleSearchQueryRequest{})
	if err != nil {
		t.Fatalf("alert rule search B: %v", err)
	}
	if rulesB != 1 {
		t.Fatalf("租户B告警规则应为 1，实际 %d", rulesB)
	}
	_, rulesAll, err := c.WithCtx(pctx).AlertRuleSearch(0, -1, dtos.AlertRuleSearchQueryRequest{})
	if err != nil {
		t.Fatalf("alert rule search platform: %v", err)
	}
	if rulesAll != 3 {
		t.Fatalf("平台视图告警规则应为 3，实际 %d", rulesAll)
	}
}
