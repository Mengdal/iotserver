//go:build !community
// +build !community

package application

import (
	"context"
	"strconv"
	"strings"
	"time"

	resourceContainer "iotserver/internal/iotserver/core/container"
	interfaces "iotserver/internal/iotserver/core/interface"
	"iotserver/internal/pkg/di"
	"iotserver/internal/pkg/logger"
	"iotserver/internal/pkg/seed"
	"iotserver/internal/pkg/tenant"
)

// defaultOfflineTimeoutMinutes 离线判定超时的兜底值（分钟）
const defaultOfflineTimeoutMinutes = 5

// offlineSweepTimeout 单次扫描的总超时
const offlineSweepTimeout = 2 * time.Minute

// SweepOffline 把超过离线超时没有活跃的设备与网关批量置为离线。
//
// 为什么需要它：在线状态原本只有「置在线」这一半——设备/网关收到数据后会被标记
// 在线，但**没有任何机制在停止上报后把它们改回离线**，于是状态会永久停在「在线」。
// 这里按统一超时补齐另一半，设备与网关共用同一个超时值（见 data.offlineTimeout），
// 避免两套语义。
//
// 判定依据：
//   - 设备：device.last_online_time（网关上行落库成功时会刷新）
//   - 网关：gateway.last_online_at（数据上报与状态心跳两条路都会刷新）
func SweepOffline(dic *di.Container, lc logger.LoggingClient) {
	ctx, cancel := context.WithTimeout(tenant.Platform(context.Background()), offlineSweepTimeout)
	defer cancel()

	dbClient := resourceContainer.DBClientFrom(dic.Get)
	minutes := offlineTimeoutMinutes(ctx, dbClient, lc)
	cutoff := time.Now().Add(-time.Duration(minutes) * time.Minute).UnixMilli()

	devices, err := dbClient.WithCtx(ctx).MarkStaleDevicesOffline(cutoff)
	if err != nil {
		lc.Errorf("offline sweep: 设备置离线失败: %v", err)
	}
	gateways, err := dbClient.WithCtx(ctx).MarkStaleGatewaysOffline(cutoff)
	if err != nil {
		lc.Errorf("offline sweep: 网关置离线失败: %v", err)
	}
	if devices > 0 || gateways > 0 {
		lc.Infof("offline sweep: 超时 %d 分钟，置离线 设备=%d 网关=%d", minutes, devices, gateways)
	}
}

// offlineTimeoutMinutes 读取离线判定超时（分钟），缺失或非法时回落默认值。
// 每次扫描都重新读取，因此改配置无需重启。
func offlineTimeoutMinutes(ctx context.Context, dbClient interfaces.DBClient, lc logger.LoggingClient) int {
	cfg, err := dbClient.WithCtx(ctx).SysConfigByKey(seed.ConfigKeyOfflineTimeout)
	if err != nil {
		lc.Warnf("offline sweep: 读取离线超时失败，使用默认 %d 分钟: %v", defaultOfflineTimeoutMinutes, err)
		return defaultOfflineTimeoutMinutes
	}
	n, convErr := strconv.Atoi(strings.TrimSpace(cfg.ConfigValue))
	if convErr != nil || n <= 0 {
		lc.Warnf("offline sweep: 离线超时取值非法(%q)，使用默认 %d 分钟", cfg.ConfigValue, defaultOfflineTimeoutMinutes)
		return defaultOfflineTimeoutMinutes
	}
	return n
}
