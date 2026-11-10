package collectapp

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"

	"iotserver/internal/models"
	"iotserver/internal/pkg/tenant"
	"iotserver/internal/pkg/utils"
)

// normalizeCollectChannelIDs 把已归属网关的本地通道主键改成「网关ID:引擎通道ID」。
//
// 早期版本用引擎通道 ID 直接做主键；不同软网关的通道 ID 会重复，必须补上网关前缀，
// 否则编辑/删除一个网关的通道会命中另一个网关的本地登记行。
func (s *service) normalizeCollectChannelIDs(ctx context.Context) {
	db := s.dbClient.GetDBInstance().WithContext(tenant.Platform(ctx))
	if db == nil || !db.Migrator().HasTable(models.TableCollectChannel) {
		return
	}
	var channels []models.CollectChannel
	if err := db.Table(models.TableCollectChannel).
		Where("COALESCE(gateway_id, '') <> '' AND remote_id > 0").
		Find(&channels).Error; err != nil {
		s.lc.Errorf("规范化采集通道主键失败: %v", err)
		return
	}
	for _, channel := range channels {
		localId := localCollectChannelID(channel.GatewayId, channel.RemoteId)
		if channel.Id == localId {
			continue
		}
		var exists int64
		if err := db.Table(models.TableCollectChannel).Where("id = ?", localId).Count(&exists).Error; err != nil {
			s.lc.Errorf("规范化采集通道主键失败（%s）: %v", channel.Id, err)
			continue
		}
		if exists > 0 {
			// 新主键已存在：保留新行，删除旧格式的重复登记。
			_ = db.Table(models.TableCollectChannel).Where("id = ?", channel.Id).Delete(&models.CollectChannel{}).Error
			continue
		}
		if err := db.Table(models.TableCollectChannel).Where("id = ?", channel.Id).
			Update("id", localId).Error; err != nil {
			s.lc.Errorf("规范化采集通道主键失败（%s）: %v", channel.Id, err)
		}
	}
}

// migrateLegacyCollectData 把旧版没有 gateway_id 的通道/设备配置迁移到软网关下。
//
// 旧版本采集引擎地址是全局配置，本地通道/设备配置的 gateway_id 为空。
// 新版本按软网关保存引擎地址后，这些数据必须归属到具体网关才能被采集页面查询到。
// 迁移按租户分组执行，幂等：没有 gateway_id 为空的数据时直接返回。
func (s *service) migrateLegacyCollectData(ctx context.Context) {
	db := s.dbClient.GetDBInstance().WithContext(tenant.Platform(ctx))
	if db == nil || !db.Migrator().HasTable(models.TableCollectChannel) {
		return
	}
	var tenantIds []string
	// tenant_id 为空表示平台级旧数据；也要迁移，否则这些通道在页面里看不到。
	if err := db.Raw(
		"SELECT tenant_id FROM collect_channel WHERE COALESCE(gateway_id, '') = '' " +
			"UNION SELECT tenant_id FROM collect_device_config WHERE COALESCE(gateway_id, '') = ''",
	).Scan(&tenantIds).Error; err != nil {
		s.lc.Errorf("迁移存量采集配置失败（查询租户）: %v", err)
		return
	}
	for _, tenantId := range tenantIds {
		gateway, err := ensureLegacyCollectGateway(db, tenantId)
		if err != nil {
			s.lc.Errorf("迁移存量采集配置失败（租户 %s 建默认软网关）: %v", tenantId, err)
			continue
		}
		if err := migrateLegacyCollectRows(db, tenantId, gateway.Id); err != nil {
			s.lc.Errorf("迁移存量采集配置失败（租户 %s）: %v", tenantId, err)
			continue
		}
		s.lc.Infof("已将租户 %s 的存量采集配置迁移到软网关 %s", tenantId, gateway.Name)
	}
}

// ensureLegacyCollectGateway 找到（或创建）用于承接存量数据的默认软网关。
// 复用已有的同名网关，避免重复创建；没有则创建一条引擎地址为空的默认网关，
// 由用户在网关管理里补充采集引擎地址。
func ensureLegacyCollectGateway(db *gorm.DB, tenantId string) (models.Gateway, error) {
	var gateway models.Gateway
	result := db.Table(models.TableGateway).
		Where("tenant_id = ? AND source = ? AND name = ?", tenantId, models.GatewaySourceIoserver, "默认软网关").
		Order("created asc").
		Limit(1).Find(&gateway)
	if result.Error != nil {
		return gateway, result.Error
	}
	if result.RowsAffected > 0 {
		return gateway, nil
	}
	now := time.Now().UnixMilli()
	gateway = models.Gateway{
		Timestamps:       models.Timestamps{Created: now, Modified: now},
		Id:               utils.RandomNum(),
		Name:             "默认软网关",
		Sn:               fmt.Sprintf("soft-legacy-%s-%s", tenantId, utils.RandomNum()),
		Source:           models.GatewaySourceIoserver,
		Protocol:         models.GatewayProtocolIoserver,
		ThingsSyncStatus: models.GatewayThingsSyncLocal,
		Status:           models.GatewayStatusNever,
		EngineTimeoutSec: 10,
	}
	gateway.TenantId = tenantId
	if err := db.Table(models.TableGateway).Create(&gateway).Error; err != nil {
		return models.Gateway{}, err
	}
	return gateway, nil
}

// migrateLegacyCollectRows 把租户下 gateway_id 为空的通道/设备配置挂到默认软网关。
// 本地通道主键同时改成「网关ID:引擎通道ID」，避免不同网关的通道 ID 互相覆盖。
func migrateLegacyCollectRows(db *gorm.DB, tenantId, gatewayId string) error {
	var channels []models.CollectChannel
	if err := db.Table(models.TableCollectChannel).
		Where("tenant_id = ? AND COALESCE(gateway_id, '') = ''", tenantId).
		Find(&channels).Error; err != nil {
		return err
	}
	for _, channel := range channels {
		remoteId := channel.RemoteId
		if remoteId <= 0 {
			// 更早的版本把引擎通道 ID 直接存在本地主键里。
			if parsed, err := strconv.ParseInt(strings.TrimSpace(channel.Id), 10, 64); err == nil {
				remoteId = parsed
			}
		}
		localId := channel.Id
		if remoteId > 0 {
			localId = localCollectChannelID(gatewayId, remoteId)
		}
		if err := db.Table(models.TableCollectChannel).
			Where("id = ? AND tenant_id = ?", channel.Id, tenantId).
			Updates(map[string]interface{}{
				"id":         localId,
				"gateway_id": gatewayId,
				"remote_id":  remoteId,
			}).Error; err != nil {
			return err
		}
	}
	return db.Table(models.TableCollectDeviceConfig).
		Where("tenant_id = ? AND COALESCE(gateway_id, '') = ''", tenantId).
		Update("gateway_id", gatewayId).Error
}
