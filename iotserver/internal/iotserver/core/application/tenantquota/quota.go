// Package tenantquota 提供租户资源配额的统一校验。
//
// 设备 / 网关 / 用户的新增入口都调用 Ensure，规则集中在这里，避免三处漂移：
//   - max* == 0 表示不限；
//   - 计数在**目标租户**作用域内进行（平台管理员带 x-tenant-id 代管时按被代管租户计）；
//   - 平台视图（tenant_id=”，Platform=true）不校验：该视图下的数据是平台级资产，
//     不属于任何租户，谈配额没有意义（详见实现报告）；
//   - 目标租户查不到（如已被删）时「不阻碍」并记警告，避免把删除竞态变成 500。
package tenantquota

import (
	"context"
	"fmt"

	interfaces "iotserver/internal/iotserver/core/interface"
	"iotserver/internal/pkg/errort"
	"iotserver/internal/pkg/logger"
	"iotserver/internal/pkg/tenant"
)

// Kind 资源类型
type Kind int

const (
	// Devices 设备配额（tenant.max_devices）
	Devices Kind = iota
	// Gateways 网关配额（tenant.max_gateways）
	Gateways
	// Users 用户配额（tenant.max_users）
	Users
)

// Ensure 校验目标租户是否还能再创建 1 个 kind 资源。
//
// 返回 errort.AppTenantQuotaExceeded 时表示超限，message 里含当前值与上限。
func Ensure(ctx context.Context, db interfaces.DBClient, lc logger.LoggingClient, kind Kind) error {
	if db == nil {
		return nil
	}
	tc := tenant.FromContext(ctx)
	// 平台视图：平台级数据无租户配额概念，不校验（平台管理员不受配额限制）
	if tc.Platform || tc.TenantID == "" {
		return nil
	}

	// 关键：把作用域显式切到目标租户，保证计数与配额归属一致。
	// 不带 x-tenant-id 的平台管理员走上面的 Platform 分支，不会到这里。
	scoped := db.WithCtx(tenant.With(ctx, tenant.Context{TenantID: tc.TenantID}))

	t, err := scoped.TenantById(tc.TenantID)
	if err != nil {
		// 租户已被删/查不到：不阻碍业务写入，仅告警
		if lc != nil {
			lc.Warnf("配额校验: 租户 %s 查询失败，跳过配额校验: %v", tc.TenantID, err)
		}
		return nil
	}

	var (
		limit   int
		current int64
		label   string
	)
	switch kind {
	case Devices:
		limit, label = t.MaxDevices, "设备"
		current, err = scoped.TenantDeviceCount(tc.TenantID)
	case Gateways:
		limit, label = t.MaxGateways, "网关"
		current, err = scoped.TenantGatewayCount(tc.TenantID)
	case Users:
		limit, label = t.MaxUsers, "用户"
		current, err = scoped.TenantUserCount(tc.TenantID)
	default:
		return nil
	}
	if err != nil {
		return err
	}
	// 0（或负数）视为不限
	if limit > 0 && current+1 > int64(limit) {
		return errort.NewCommonErr(errort.AppTenantQuotaExceeded,
			fmt.Errorf("%s数量已达上限（当前 %d，上限 %d）", label, current, limit))
	}
	return nil
}
