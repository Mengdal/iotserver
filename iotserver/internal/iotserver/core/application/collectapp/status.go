package collectapp

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"iotserver/internal/dtos"
	interfaces "iotserver/internal/iotserver/core/interface"
	"iotserver/internal/models"
	"iotserver/internal/pkg/collectgateway"
	"iotserver/internal/pkg/constants"
	"iotserver/internal/pkg/tenant"
	"iotserver/internal/tools/collectengine"
	"iotserver/internal/tools/iotp"
)

const (
	// softStatusSyncInterval 软网关/设备在线状态的刷新周期。
	softStatusSyncInterval = 15 * time.Second
	// softStatusStartupDelay 启动后先等服务初始化完成再开始探测。
	softStatusStartupDelay = 10 * time.Second
	// softStatusStaleFactor 点位超过 N 个轮询周期视为不新鲜。
	softStatusStaleFactor = 3
	// softStatusMinStale 最短新鲜窗口，避免轮询周期很小导致状态抖动。
	softStatusMinStale = 15 * time.Second
	// softStatusOfflineFailures 连续探测失败 N 次才判定离线，避免一次超时导致状态抖动。
	softStatusOfflineFailures = 3
)

// StartStatusSync 启动软网关/设备在线状态同步。
//
// 软网关不走平台 MQTT，GatewayOnline/Offline 不会触发；这里以采集引擎的实时点位
// 新鲜度为准：网关下至少一个启用点位在新鲜窗口内 → 网关在线；设备同理。
func (s *service) StartStatusSync(ctx context.Context) {
	if s == nil || s.dbClient == nil {
		return
	}
	go func() {
		timer := time.NewTimer(softStatusStartupDelay)
		defer timer.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-timer.C:
			}
			s.syncSoftGatewayStatuses(ctx)
			timer.Reset(softStatusSyncInterval)
		}
	}()
}

func (s *service) syncSoftGatewayStatuses(ctx context.Context) {
	platformCtx := tenant.Platform(ctx)
	db := s.dbClient.WithCtx(platformCtx)
	gateways, _, err := db.GatewaySearch(0, -1, dtos.GatewaySearchQueryRequest{
		BaseSearchConditionQuery: dtos.BaseSearchConditionQuery{IsAll: true},
		Source:                   models.GatewaySourceIoserver,
	})
	if err != nil {
		s.debugf("软网关状态同步：读取网关失败: %v", err)
		return
	}
	if len(gateways) == 0 {
		return
	}

	channels, err := db.CollectChannels()
	if err != nil {
		s.debugf("软网关状态同步：读取通道失败: %v", err)
		return
	}
	configs, err := db.CollectDeviceConfigs()
	if err != nil {
		s.debugf("软网关状态同步：读取采集绑定失败: %v", err)
		return
	}
	points, err := db.CollectPointConfigsAll()
	if err != nil {
		s.debugf("软网关状态同步：读取点位失败: %v", err)
		return
	}
	devices, _, err := db.DevicesSearch(0, -1, dtos.DeviceSearchQueryRequest{
		BaseSearchConditionQuery: dtos.BaseSearchConditionQuery{IsAll: true},
	})
	if err != nil {
		s.debugf("软网关状态同步：读取设备失败: %v", err)
		return
	}

	deviceByIotpID := make(map[int64]models.Device, len(devices))
	for _, device := range devices {
		deviceByIotpID[iotp.DeviceID(device.Id)] = device
	}
	channelsByGateway := make(map[string][]models.CollectChannel)
	for _, channel := range channels {
		if channel.GatewayId != "" {
			channelsByGateway[channel.GatewayId] = append(channelsByGateway[channel.GatewayId], channel)
		}
	}
	configsByGateway := make(map[string][]models.CollectDeviceConfig)
	for _, cfg := range configs {
		if cfg.GatewayId != "" {
			configsByGateway[cfg.GatewayId] = append(configsByGateway[cfg.GatewayId], cfg)
		}
	}
	pointsByDevice := make(map[string][]models.CollectPointConfig)
	for _, point := range points {
		pointsByDevice[point.DeviceId] = append(pointsByDevice[point.DeviceId], point)
	}

	now := time.Now().UnixMilli()
	for _, gateway := range gateways {
		gatewayCfg := configsByGateway[gateway.Id]
		pollByRemote := make(map[int64]int64)
		for _, channel := range channelsByGateway[gateway.Id] {
			if channel.RemoteId > 0 {
				pollByRemote[channel.RemoteId] = int64(channel.PollIntervalMs)
			}
		}

		engineCtx := collectgateway.With(platformCtx, gateway.Id)
		client, err := s.engineClient(engineCtx)
		if err != nil {
			s.debugf("软网关状态同步：网关 %s 连接采集引擎失败: %v", gateway.Sn, err)
			s.noteSoftGatewayProbeFailure(db, gateway, gatewayCfg, pointsByDevice, deviceByIotpID, now)
			continue
		}
		records, err := loadEngineRealtimeTags(client)
		if err != nil {
			s.debugf("软网关状态同步：网关 %s 读取实时值失败: %v", gateway.Sn, err)
			s.noteSoftGatewayProbeFailure(db, gateway, gatewayCfg, pointsByDevice, deviceByIotpID, now)
			continue
		}
		delete(s.statusFailures, gateway.Id)

		// 没有启用的设备/点位时无从判断是否在采集，保持现有状态，
		// 避免新建网关一上线就被打成「离线」。
		if !hasEvaluableCollect(gatewayCfg, pointsByDevice) {
			continue
		}

		online, deviceOnline := computeSoftGatewayStatus(now, gatewayCfg, pointsByDevice, pollByRemote, records)
		s.applySoftGatewayStatus(db, gateway, gatewayCfg, deviceByIotpID, online, now)
		for deviceKey, isOnline := range deviceOnline {
			device, ok := deviceByIotpID[parseCollectID(deviceKey)]
			if !ok {
				continue
			}
			if isOnline {
				_ = db.DeviceOnlineById(device.Id)
				continue
			}
			if device.Status == constants.DeviceStatusUnKnow || device.Status == "" {
				// 状态未知时不必写离线，保持原值。
				continue
			}
			if device.Status != constants.DeviceStatusOffline {
				_ = db.DeviceOfflineById(device.Id)
			}
		}
	}
}

// noteSoftGatewayProbeFailure 记录一次引擎探测失败；连续失败达到阈值才判定离线。
func (s *service) noteSoftGatewayProbeFailure(db interfaces.DBClient, gateway models.Gateway, configs []models.CollectDeviceConfig, pointsByDevice map[string][]models.CollectPointConfig, deviceByIotpID map[int64]models.Device, now int64) {
	if !hasEvaluableCollect(configs, pointsByDevice) {
		return
	}
	if s.statusFailures == nil {
		s.statusFailures = make(map[string]int)
	}
	s.statusFailures[gateway.Id]++
	if s.statusFailures[gateway.Id] < softStatusOfflineFailures {
		return
	}
	s.applySoftGatewayStatus(db, gateway, configs, deviceByIotpID, false, now)
}

// hasEvaluableCollect 是否有可判断的采集目标（启用设备 + 启用点位）。
func hasEvaluableCollect(configs []models.CollectDeviceConfig, pointsByDevice map[string][]models.CollectPointConfig) bool {
	for _, cfg := range configs {
		if !cfg.Enabled {
			continue
		}
		for _, point := range pointsByDevice[cfg.DeviceId] {
			if point.Enabled {
				return true
			}
		}
	}
	return false
}

// applySoftGatewayStatus 更新网关在线状态；过渡时打一条日志。
func (s *service) applySoftGatewayStatus(db interfaces.DBClient, gateway models.Gateway, configs []models.CollectDeviceConfig, deviceByIotpID map[int64]models.Device, online bool, now int64) {
	// 从未上线过的网关在没有数据时保持「从未上线」，不要显示成离线。
	if !online && gateway.Status == models.GatewayStatusNever {
		return
	}
	status := models.GatewayStatusOffline
	onlineAt := int64(0)
	if online {
		status = models.GatewayStatusOnline
		onlineAt = now
	}
	if gateway.Status != status {
		s.infof("软网关 %s 状态变为 %s", gateway.Sn, gatewayStatusText(status))
	}
	// 在线时每轮都刷新 last_online_at；已离线且状态不变时不必重复写。
	if status == models.GatewayStatusOnline || gateway.Status != status {
		_ = db.UpdateGatewayStatus(gateway.Id, status, "", onlineAt)
	}
	// 引擎不可用时网关下设备一并离线；在线状态由调用方按点位新鲜度逐台刷新。
	if online {
		return
	}
	for _, cfg := range configs {
		device, ok := deviceByIotpID[parseCollectID(cfg.DeviceId)]
		if !ok {
			continue
		}
		if device.Status == constants.DeviceStatusOnline {
			_ = db.DeviceOfflineById(device.Id)
		}
	}
}

// computeSoftGatewayStatus 纯计算：网关是否在线、每台设备是否在线。
//
// 判定口径与采集配置页一致：引擎状态 Good 且时间戳在 3×轮询周期（最少 15 秒）内。
func computeSoftGatewayStatus(
	now int64,
	configs []models.CollectDeviceConfig,
	pointsByDevice map[string][]models.CollectPointConfig,
	pollByRemote map[int64]int64,
	records map[string]collectengine.TagRealtimeRecord,
) (bool, map[string]bool) {
	deviceOnline := make(map[string]bool, len(configs))
	gatewayOnline := false
	for _, cfg := range configs {
		online := false
		if cfg.Enabled {
			staleAfter := softStatusStaleFactor * pollByRemote[parseCollectID(cfg.ChannelId)]
			if staleAfter < int64(softStatusMinStale/time.Millisecond) {
				staleAfter = int64(softStatusMinStale / time.Millisecond)
			}
			iotpID := parseCollectID(cfg.DeviceId)
			for _, point := range pointsByDevice[cfg.DeviceId] {
				if !point.Enabled {
					continue
				}
				record, ok := records[fmt.Sprintf("%d.%s", iotpID, point.TagId)]
				if !ok || record.StatusText() != "Good" {
					continue
				}
				ts := normalizeEngineTimestamp(record.Timestamp)
				if ts > 0 && now-ts <= staleAfter {
					online = true
					break
				}
			}
		}
		deviceOnline[cfg.DeviceId] = online
		if online {
			gatewayOnline = true
		}
	}
	return gatewayOnline, deviceOnline
}

// loadEngineRealtimeTags 分页读取引擎全量点位实时值，按 record.Id 索引。
// 采集状态同步和实时数据查询共用，避免两套分页逻辑各自演化。
func loadEngineRealtimeTags(client *collectengine.Client) (map[string]collectengine.TagRealtimeRecord, error) {
	records := make(map[string]collectengine.TagRealtimeRecord)
	total := -1
	for page := 1; page <= 100; page++ {
		result, err := client.RealtimeTagsPage(0, page, 1000)
		if err != nil {
			return nil, err
		}
		for _, record := range result.Items {
			records[record.Id] = record
		}
		if total < 0 {
			total = result.Total
		}
		if len(result.Items) == 0 || page*1000 >= total {
			break
		}
	}
	return records, nil
}

// parseCollectID 解析采集通道/设备 ID；非法值返回 0。
func parseCollectID(value string) int64 {
	id, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil || id <= 0 {
		return 0
	}
	return id
}

func gatewayStatusText(status int) string {
	switch status {
	case models.GatewayStatusOnline:
		return "在线"
	case models.GatewayStatusOffline:
		return "离线"
	default:
		return "从未上线"
	}
}

func (s *service) debugf(format string, args ...interface{}) {
	if s != nil && s.lc != nil {
		s.lc.Debugf(format, args...)
	}
}

func (s *service) infof(format string, args ...interface{}) {
	if s != nil && s.lc != nil {
		s.lc.Infof(format, args...)
	}
}
