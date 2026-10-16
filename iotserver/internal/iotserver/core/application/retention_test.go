//go:build !community
// +build !community

package application

import (
	"context"
	"errors"
	"testing"
	"time"

	interfaces "iotserver/internal/iotserver/core/interface"
	"iotserver/internal/models"
	"iotserver/internal/pkg/logger"
)

// fakePurger 记录每次清理调用，便于断言保留天数换算
type fakePurger struct {
	calls    []purgeCall
	failOnID string
}

type purgeCall struct {
	tenantID string
	cutoff   int64
}

func (f *fakePurger) PurgeBefore(ctx context.Context, cutoffMillis int64) (int64, error) {
	f.calls = append(f.calls, purgeCall{tenantID: "", cutoff: cutoffMillis})
	return 0, nil
}

func (f *fakePurger) PurgeTenantBefore(ctx context.Context, tenantID string, cutoffMillis int64) (int64, error) {
	if f.failOnID != "" && tenantID == f.failOnID {
		return 0, errors.New("boom")
	}
	f.calls = append(f.calls, purgeCall{tenantID: tenantID, cutoff: cutoffMillis})
	return 3, nil
}

func (f *fakePurger) cutoffOf(tenantID string) (int64, bool) {
	for _, c := range f.calls {
		if c.tenantID == tenantID {
			return c.cutoff, true
		}
	}
	return 0, false
}

// 编译期确认 fakePurger 满足能力接口
var _ interfaces.DataPurger = (*fakePurger)(nil)

func TestEffectiveRetentionDays(t *testing.T) {
	cases := []struct {
		name       string
		tenantDays int
		globalDays int
		wantDays   int
		wantSkip   bool
	}{
		{"租户覆盖全局", 7, 30, 7, false},
		{"租户为0用全局", 0, 90, 90, false},
		{"租户负数永久保留", -1, 30, 0, true},
		{"全局为0兜底默认", 0, 0, defaultRetentionDays, false},
		{"全局负数兜底默认", 0, -5, defaultRetentionDays, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			days, skip := effectiveRetentionDays(tc.tenantDays, tc.globalDays)
			if skip != tc.wantSkip {
				t.Fatalf("skip=%v want %v", skip, tc.wantSkip)
			}
			if !skip && days != tc.wantDays {
				t.Fatalf("days=%d want %d", days, tc.wantDays)
			}
		})
	}
}

func TestApplyRetentionPerTenant(t *testing.T) {
	lc := logger.NewClient("retention-test", "error", "")
	purger := &fakePurger{}
	now := time.Date(2026, 3, 10, 12, 0, 0, 0, time.UTC)

	tenants := []models.Tenant{
		{Id: "t-ovr", Name: "覆盖租户", RetentionDays: 7},
		{Id: "t-def", Name: "默认租户", RetentionDays: 0},
		{Id: "t-forever", Name: "永久租户", RetentionDays: -1},
	}

	handled, rows := applyRetention(context.Background(), purger, tenants, 30, now, lc)

	if handled != 2 {
		t.Fatalf("handled=%d want 2（永久保留的租户应被跳过）", handled)
	}
	if rows != 6 { // 两个租户各返回 3
		t.Fatalf("purgedRows=%d want 6", rows)
	}
	if len(purger.calls) != 2 {
		t.Fatalf("调用次数=%d want 2", len(purger.calls))
	}
	if _, ok := purger.cutoffOf("t-forever"); ok {
		t.Fatalf("永久保留的租户不应被清理")
	}

	// 按天换算：cutoff 必须等于 now - 生效天数
	day := int64(24 * 3600 * 1000)
	wantOvr := now.AddDate(0, 0, -7).UnixNano() / int64(time.Millisecond)
	wantDef := now.AddDate(0, 0, -30).UnixNano() / int64(time.Millisecond)
	if got, _ := purger.cutoffOf("t-ovr"); got != wantOvr {
		t.Fatalf("覆盖租户 cutoff=%d want %d（差 %d 天）", got, wantOvr, (got-wantOvr)/day)
	}
	if got, _ := purger.cutoffOf("t-def"); got != wantDef {
		t.Fatalf("默认租户 cutoff=%d want %d（差 %d 天）", got, wantDef, (got-wantDef)/day)
	}
}

// 单个租户清理失败不应中断其它租户
func TestApplyRetentionContinuesAfterError(t *testing.T) {
	lc := logger.NewClient("retention-test", "error", "")
	purger := &fakePurger{failOnID: "t-bad"}
	now := time.Now()

	tenants := []models.Tenant{
		{Id: "t-bad", Name: "坏租户", RetentionDays: 1},
		{Id: "t-ok", Name: "好租户", RetentionDays: 1},
	}
	handled, _ := applyRetention(context.Background(), purger, tenants, 30, now, lc)
	if handled != 1 {
		t.Fatalf("handled=%d want 1", handled)
	}
	if _, ok := purger.cutoffOf("t-ok"); !ok {
		t.Fatalf("失败之后的租户仍应被处理")
	}
}
