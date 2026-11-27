package sqlite

import (
	"context"
	"testing"

	"iotserver/internal/models"
	"iotserver/internal/pkg/tenant"
)

// 规则名只在租户内唯一：平台视图（不做租户过滤）下编辑 A 租户规则时，
// 不能因为 B 租户有同名规则而报重名。
func TestRuleEngineNameUniquePerTenant(t *testing.T) {
	c := newTestClient(t)
	if err := c.MigrateSystemTables(); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	pctx := tenant.Platform(context.Background())
	ta, err := c.WithCtx(pctx).AddTenant(models.Tenant{Code: "rule_a", Name: "规则租户A"})
	if err != nil {
		t.Fatalf("add tenant A: %v", err)
	}
	tb, err := c.WithCtx(pctx).AddTenant(models.Tenant{Code: "rule_b", Name: "规则租户B"})
	if err != nil {
		t.Fatalf("add tenant B: %v", err)
	}
	ctxA := tenant.With(pctx, tenant.Context{TenantID: ta.Id})
	ctxB := tenant.With(pctx, tenant.Context{TenantID: tb.Id})

	ruleA, err := c.WithCtx(ctxA).AddRuleEngine(models.RuleEngine{Name: "temp-alert"})
	if err != nil {
		t.Fatalf("add rule A: %v", err)
	}
	if _, err = c.WithCtx(ctxB).AddRuleEngine(models.RuleEngine{Name: "temp-alert"}); err != nil {
		t.Fatalf("不同租户允许同名规则: %v", err)
	}
	// 同一租户内仍然不允许重名
	if _, err = c.WithCtx(ctxA).AddRuleEngine(models.RuleEngine{Name: "temp-alert"}); err == nil {
		t.Fatal("同一租户同名规则应被拒绝")
	}

	// 平台视图下保存 A 的规则（名称不变），B 的同名记录不应造成冲突
	ruleA.TenantId = ta.Id
	ruleA.Name = "temp-alert"
	if err = c.WithCtx(pctx).UpdateRuleEngine(ruleA); err != nil {
		t.Fatalf("平台视图保存租户规则: %v", err)
	}
}
