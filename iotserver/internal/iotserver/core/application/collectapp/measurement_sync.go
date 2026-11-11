package collectapp

import (
	"context"
	"strings"
	"time"

	"iotserver/internal/dtos"
	"iotserver/internal/models"
	"iotserver/internal/pkg/collectgateway"
	"iotserver/internal/pkg/tenant"
	"iotserver/internal/tools/iotp"
)

const (
	measurementSyncStartup  = 15 * time.Second
	measurementSyncInterval = 60 * time.Second
)

// StartMeasurementSync 启动设备 measurement 自动修正任务。
//
// 设备要写入哪张 iotp 表由设备自身的产品决定；引擎里已有设备可能是旧配置（如 cpu），
// 这里定期按平台设备产品重新计算并更新，产品变更后也能自动跟上。
func (s *service) StartMeasurementSync(ctx context.Context) {
	if s == nil || s.dbClient == nil {
		return
	}
	go func() {
		timer := time.NewTimer(measurementSyncStartup)
		defer timer.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-timer.C:
			}
			s.syncEngineMeasurements(ctx)
			timer.Reset(measurementSyncInterval)
		}
	}()
}

func (s *service) syncEngineMeasurements(ctx context.Context) {
	platformCtx := tenant.Platform(ctx)
	db := s.dbClient.WithCtx(platformCtx)
	gateways, _, err := db.GatewaySearch(0, -1, dtos.GatewaySearchQueryRequest{
		BaseSearchConditionQuery: dtos.BaseSearchConditionQuery{IsAll: true},
		Source:                   models.GatewaySourceIoserver,
	})
	if err != nil || len(gateways) == 0 {
		return
	}
	devices, _, err := db.DevicesSearch(0, -1, dtos.DeviceSearchQueryRequest{
		BaseSearchConditionQuery: dtos.BaseSearchConditionQuery{IsAll: true},
	})
	if err != nil {
		return
	}
	deviceByIotpID := make(map[int64]models.Device, len(devices))
	for _, device := range devices {
		deviceByIotpID[iotp.DeviceID(device.Id)] = device
	}

	for _, gateway := range gateways {
		gatewayCtx := collectgateway.With(platformCtx, gateway.Id)
		client, err := s.engineClient(gatewayCtx)
		if err != nil {
			s.debugf("measurement 同步：网关 %s 连接采集引擎失败: %v", gateway.Sn, err)
			continue
		}
		bindings, err := client.ListDevices(0)
		if err != nil {
			s.debugf("measurement 同步：网关 %s 读取设备失败: %v", gateway.Sn, err)
			continue
		}
		for _, binding := range bindings {
			device, ok := deviceByIotpID[binding.IotpId]
			if !ok {
				continue
			}
			measurement := iotp.DeviceTable(device.ProductId)
			if strings.TrimSpace(binding.Measurement) == measurement {
				continue
			}
			if err := client.UpdateDevice(binding.Id, binding.ChannelId, binding.IotpId, measurement); err != nil {
				s.debugf("measurement 同步：设备 %d 更新为 %s 失败: %v", binding.IotpId, measurement, err)
				continue
			}
			s.infof("measurement 同步：设备 %d 目标表 %s", binding.IotpId, measurement)
		}
	}
}
