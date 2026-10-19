//go:build !community
// +build !community

package application

import (
	"context"
	resourceContainer "iotserver/internal/iotserver/core/container"
	"iotserver/internal/pkg/crontab"
	"iotserver/internal/pkg/di"
	"iotserver/internal/pkg/logger"
	"time"
)

func InitSchedule(dic *di.Container, lc logger.LoggingClient) {
	lc.Info("init schedule")

	// 每天 1 点
	crontab.Schedule.AddFunc("0 1 * * *", func() {
		lc.Debugf("schedule statistic device msg conut: %v", time.Now().Format("2006-01-02 15:04:05"))
		deviceItf := resourceContainer.DeviceItfFrom(dic.Get)
		err := deviceItf.DevicesReportMsgGather(context.Background())
		if err != nil {
			lc.Error("schedule statistic device err:", err)
		}
	})

	// 每小时按租户执行时序数据保留策略（保留天数可按租户配置，每次运行重新读配置）
	crontab.Schedule.AddFunc("0 * * * *", func() {
		RunRetention(dic, lc)
	})

	// 每分钟把超时未活跃的设备/网关置为离线（超时值见 devicestatus.defaultOfflineTimeoutMinutes）
	crontab.Schedule.AddFunc("* * * * *", func() {
		SweepOffline(dic, lc)
	})

	crontab.Start()
}
