package sqlite

import (
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"iotserver/internal/models"
	"iotserver/internal/pkg/utils"
)

/******************************* 采集通道 *******************************/

func (c *Client) CollectChannels() ([]models.CollectChannel, error) {
	var channels []models.CollectChannel
	err := c.Pool.Order("name asc").Find(&channels).Error
	return channels, err
}

func (c *Client) SaveCollectChannel(channel models.CollectChannel) (models.CollectChannel, error) {
	now := time.Now().UnixMilli()
	if channel.Id == "" {
		channel.Id = utils.GenerateDeviceSecret(16)
		channel.Created = now
		channel.Modified = now
		if err := c.Pool.Create(&channel).Error; err != nil {
			return channel, err
		}
		return channel, nil
	}
	var existing models.CollectChannel
	err := c.Pool.Where("id = ?", channel.Id).First(&existing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		channel.Created = now
		channel.Modified = now
		if err := c.Pool.Create(&channel).Error; err != nil {
			return channel, err
		}
		return channel, nil
	}
	if err != nil {
		return channel, err
	}
	channel.Created = existing.Created
	channel.Modified = now
	err = c.Pool.Model(&models.CollectChannel{}).Where("id = ?", channel.Id).Updates(map[string]any{
		"gateway_id":       channel.GatewayId,
		"remote_id":        channel.RemoteId,
		"name":             channel.Name,
		"channel_type":     channel.ChannelType,
		"protocol":         channel.Protocol,
		"config":           channel.Config,
		"poll_interval_ms": channel.PollIntervalMs,
		"timeout_ms":       channel.TimeoutMs,
		"enabled":          channel.Enabled,
		"remark":           channel.Remark,
		"modified":         channel.Modified,
	}).Error
	return channel, err
}

func (c *Client) DeleteCollectChannel(id string) error {
	return c.Pool.Where("id = ?", id).Delete(&models.CollectChannel{}).Error
}

// UpdateCollectPointRemoteIds 按本地点位 ID 回写采集引擎点位 ID。
func (c *Client) UpdateCollectPointRemoteIds(deviceId string, remoteIds map[string]int64) error {
	if len(remoteIds) == 0 {
		return nil
	}
	return c.Pool.Transaction(func(tx *gorm.DB) error {
		for id, remoteId := range remoteIds {
			if id == "" || remoteId == 0 {
				continue
			}
			if err := tx.Model(&models.CollectPointConfig{}).
				Where("device_id = ? AND id = ?", deviceId, id).
				Update("remote_id", remoteId).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// CollectDeviceConfigs 查询所有设备级采集配置。
func (c *Client) CollectDeviceConfigs() ([]models.CollectDeviceConfig, error) {
	var configs []models.CollectDeviceConfig
	err := c.Pool.Order("device_id asc").Find(&configs).Error
	return configs, err
}

func (c *Client) CollectDeviceConfigsByChannel(channelId string) ([]models.CollectDeviceConfig, error) {
	var configs []models.CollectDeviceConfig
	err := c.Pool.Where("channel_id = ?", channelId).Order("created asc").Find(&configs).Error
	return configs, err
}

func (c *Client) DeleteCollectDeviceConfig(deviceId string) error {
	return c.Pool.Where("device_id = ?", deviceId).Delete(&models.CollectDeviceConfig{}).Error
}

/******************************* 设备级采集配置 *******************************/

func (c *Client) CollectDeviceConfigByDevice(deviceId string) (models.CollectDeviceConfig, bool, error) {
	var cfg models.CollectDeviceConfig
	err := c.Pool.Where("device_id = ?", deviceId).First(&cfg).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return cfg, false, nil
	}
	if err != nil {
		return cfg, false, err
	}
	return cfg, true, nil
}

func (c *Client) SaveCollectDeviceConfig(cfg models.CollectDeviceConfig) error {
	now := time.Now().UnixMilli()
	if cfg.Id == "" {
		cfg.Id = utils.GenerateDeviceSecret(16)
	}
	if cfg.Created == 0 {
		cfg.Created = now
	}
	cfg.Modified = now
	return c.Pool.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "device_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"channel_id", "remote_id", "gateway_id", "enabled", "remark", "modified",
		}),
	}).Create(&cfg).Error
}

func (c *Client) ClearCollectPointRemoteIds(deviceId string) error {
	return c.Pool.Model(&models.CollectPointConfig{}).
		Where("device_id = ?", deviceId).
		Update("remote_id", 0).Error
}

// CollectPointConfigsAll 查询所有点位配置。
func (c *Client) CollectPointConfigsAll() ([]models.CollectPointConfig, error) {
	var points []models.CollectPointConfig
	err := c.Pool.Order("device_id asc, sort asc").Find(&points).Error
	return points, err
}

func (c *Client) CollectPointConfigsByDevice(deviceId string) ([]models.CollectPointConfig, error) {
	var points []models.CollectPointConfig
	err := c.Pool.Where("device_id = ?", deviceId).Order("sort asc").Find(&points).Error
	return points, err
}

// UpdateCollectPointLastWrite 记录点位最后一次写入值。
func (c *Client) UpdateCollectPointLastWrite(deviceId, tagId, value string, ts int64) error {
	return c.Pool.Model(&models.CollectPointConfig{}).
		Where("device_id = ? AND tag_id = ?", deviceId, tagId).
		Updates(map[string]any{
			"last_write_value": value,
			"last_write_time":  ts,
		}).Error
}

// CollectPointCounts 按设备统计点位数。
func (c *Client) CollectPointCounts() (map[string]int64, error) {
	type row struct {
		DeviceId string
		Total    int64
	}
	var rows []row
	err := c.Pool.Model(&models.CollectPointConfig{}).
		Select("device_id, COUNT(*) AS total").
		Group("device_id").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	counts := make(map[string]int64, len(rows))
	for _, item := range rows {
		counts[item.DeviceId] = item.Total
	}
	return counts, nil
}

// ReplaceCollectPointConfigs 整体替换某设备的点位配置（删旧 + 插新，同一事务）。
func (c *Client) ReplaceCollectPointConfigs(deviceId string, points []models.CollectPointConfig) error {
	return c.Pool.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("device_id = ?", deviceId).Delete(&models.CollectPointConfig{}).Error; err != nil {
			return err
		}
		if len(points) == 0 {
			return nil
		}
		now := time.Now().UnixMilli()
		for i := range points {
			if points[i].Id == "" {
				points[i].Id = utils.GenerateDeviceSecret(16)
			}
			points[i].DeviceId = deviceId
			if points[i].Created == 0 {
				points[i].Created = now
			}
			points[i].Modified = now
		}
		return tx.Create(&points).Error
	})
}

/******************************* 采集引擎清理任务 *******************************/

// AddCollectCleanupTask 新增一条引擎资源清理任务。
func (c *Client) AddCollectCleanupTask(t models.CollectCleanupTask) error {
	now := time.Now().UnixMilli()
	if t.Id == "" {
		t.Id = utils.GenerateDeviceSecret(16)
	}
	if t.Created == 0 {
		t.Created = now
	}
	t.Modified = now
	return c.Pool.Create(&t).Error
}

// CollectCleanupTasksDue 取到期待清理的任务。
func (c *Client) CollectCleanupTasksDue(now int64, limit int) ([]models.CollectCleanupTask, error) {
	if limit <= 0 {
		limit = 10
	}
	var list []models.CollectCleanupTask
	err := c.Pool.
		Where("next_retry_at = 0 OR next_retry_at <= ?", now).
		Order("created asc").
		Limit(limit).
		Find(&list).Error
	return list, err
}

// UpdateCollectCleanupTask 更新重试次数、错误与下次重试时间。
func (c *Client) UpdateCollectCleanupTask(t models.CollectCleanupTask) error {
	return c.Pool.Model(&models.CollectCleanupTask{}).Where("id = ?", t.Id).Updates(map[string]interface{}{
		"attempts":      t.Attempts,
		"last_error":    t.LastError,
		"next_retry_at": t.NextRetryAt,
		"modified":      time.Now().UnixMilli(),
	}).Error
}

// DeleteCollectCleanupTask 清理完成后删除任务。
func (c *Client) DeleteCollectCleanupTask(id string) error {
	return c.Pool.Where("id = ?", id).Delete(&models.CollectCleanupTask{}).Error
}
