//go:build !community
// +build !community

package application

import (
	"context"
	"time"

	"iotserver/internal/dtos"
	resourceContainer "iotserver/internal/iotserver/core/container"
	interfaces "iotserver/internal/iotserver/core/interface"
	"iotserver/internal/models"
	"iotserver/internal/pkg/di"
	"iotserver/internal/pkg/logger"
	"iotserver/internal/pkg/tenant"
)

// defaultRetentionDays 系统全局保留天数的兜底值。
// 与数据层（duckdb 的 defaultRetentionDays）保持一致。
const defaultRetentionDays = 30

// retentionRunTimeout 单次清理的总超时
const retentionRunTimeout = 30 * time.Minute

// RunRetention 按租户执行时序数据保留策略。
//
// 之所以放在应用层按租户驱动，而不是让数据层用启动时读到的固定值自己清理：
//   - 保留天数按租户配置（tenant.retention_days，见租户管理）；
//   - 租户为 0 时使用默认值（30 天），无需系统级配置。
//
// 语义：
//
//	租户 retention_days > 0  → 按该天数
//	租户 retention_days = 0  → 用默认保留天数（30 天）
//	租户 retention_days < 0  → 永久保留，跳过
//
// 数据层未实现 DataPurger 能力时直接返回，保留其自带的维护逻辑。
func RunRetention(dic *di.Container, lc logger.LoggingClient) {
	// 平台作用域：保留策略是系统级后台任务，要覆盖所有租户
	ctx, cancel := context.WithTimeout(tenant.Platform(context.Background()), retentionRunTimeout)
	defer cancel()

	purger, ok := resourceContainer.DataDBClientFrom(dic.Get).(interfaces.DataPurger)
	if !ok {
		lc.Debugf("retention: 当前数据层不支持按租户清理，跳过（沿用数据层自带的全局维护）")
		return
	}

	dbClient := resourceContainer.DBClientFrom(dic.Get)
	globalDays := defaultRetentionDays

	tenants, _, err := dbClient.WithCtx(ctx).TenantSearch(0, -1, dtos.TenantSearchQueryRequest{})
	if err != nil {
		lc.Errorf("retention: 查询租户列表失败: %v", err)
		return
	}

	handled, purgedRows := applyRetention(ctx, purger, tenants, globalDays, time.Now(), lc)
	lc.Infof("retention: 完成，处理 %d 个租户，共清理 %d 行（全局默认 %d 天）",
		handled, purgedRows, globalDays)
}

// applyRetention 按租户逐个执行清理。抽成独立函数以便单测（不依赖 DI 容器与真实数据层）。
func applyRetention(
	ctx context.Context,
	purger interfaces.DataPurger,
	tenants []models.Tenant,
	globalDays int,
	now time.Time,
	lc logger.LoggingClient,
) (handled int, purgedRows int64) {
	for _, t := range tenants {
		days, skip := effectiveRetentionDays(t.RetentionDays, globalDays)
		if skip {
			continue
		}
		cutoff := now.AddDate(0, 0, -days).UnixNano() / int64(time.Millisecond)
		deleted, err := purger.PurgeTenantBefore(ctx, t.Id, cutoff)
		if err != nil {
			lc.Errorf("retention: 清理租户 %s(%s) 失败: %v", t.Name, t.Id, err)
			continue
		}
		handled++
		purgedRows += deleted
		if deleted > 0 {
			lc.Infof("retention: 租户 %s 保留 %d 天，清理 %d 行", t.Name, days, deleted)
		}
	}
	return handled, purgedRows
}

// effectiveRetentionDays 计算租户实际生效的保留天数。
//
//	租户 >0  → 按该天数（覆盖全局）
//	租户 =0  → 用全局配置
//	租户 <0  → 永久保留，skip=true
//
// 全局值非法或为 0 时兜底为 defaultRetentionDays。
func effectiveRetentionDays(tenantDays, globalDays int) (days int, skip bool) {
	if tenantDays < 0 {
		return 0, true
	}
	if tenantDays > 0 {
		return tenantDays, false
	}
	if globalDays <= 0 {
		globalDays = defaultRetentionDays
	}
	return globalDays, false
}
