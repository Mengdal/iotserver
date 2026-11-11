package collectapp

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	interfaces "iotserver/internal/iotserver/core/interface"
	"iotserver/internal/models"
	"iotserver/internal/pkg/tenant"
	"iotserver/internal/tools/collectengine"
)

const (
	cleanupWorkerInterval = 20 * time.Second
	cleanupWorkerStartup  = 10 * time.Second
	cleanupRequestTimeout = 10 * time.Second
	cleanupBatchSize      = 10
	cleanupRetryBaseDelay = time.Minute
	cleanupRetryMaxDelay  = time.Hour
)

// StartCleanupWorker 启动删除软网关后的采集引擎清理任务。
func (s *service) StartCleanupWorker(ctx context.Context) {
	if s == nil || s.dbClient == nil {
		return
	}
	go func() {
		timer := time.NewTimer(cleanupWorkerStartup)
		defer timer.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-timer.C:
			}
			s.runCleanupTasks(ctx)
			timer.Reset(cleanupWorkerInterval)
		}
	}()
}

func (s *service) runCleanupTasks(ctx context.Context) {
	db := s.dbClient.WithCtx(tenant.Platform(ctx))
	tasks, err := db.CollectCleanupTasksDue(time.Now().UnixMilli(), cleanupBatchSize)
	if err != nil {
		s.debugf("采集引擎清理任务：读取失败: %v", err)
		return
	}
	for _, task := range tasks {
		s.executeCleanupTask(ctx, db, task)
	}
}

func (s *service) executeCleanupTask(ctx context.Context, db interfaces.DBClient, task models.CollectCleanupTask) {
	engineURL := strings.TrimRight(strings.TrimSpace(task.EngineBaseURL), "/")
	if engineURL == "" {
		_ = db.DeleteCollectCleanupTask(task.Id)
		return
	}
	var channelIDs []int64
	if strings.TrimSpace(task.ChannelIDs) != "" {
		_ = json.Unmarshal([]byte(task.ChannelIDs), &channelIDs)
	}
	client := collectengine.New(engineURL, cleanupRequestTimeout)

	// 任务排队期间引擎可能已被新建的软网关接管。执行前重新确认，
	// 避免把新网关的通道/设备/点位当成遗留数据清掉。
	if task.DeleteAll {
		shared, sharedErr := s.engineSharedByOtherGatewayID(db, engineURL, task.GatewayId)
		if sharedErr != nil {
			task.Attempts++
			task.LastError = sharedErr.Error()
			task.NextRetryAt = time.Now().Add(cleanupRetryDelay(task.Attempts)).UnixMilli()
			_ = db.UpdateCollectCleanupTask(task)
			s.debugf("采集引擎清理任务 %s 无法确认引擎归属，稍后重试: %v", task.Id, sharedErr)
			return
		}
		if shared {
			_ = db.DeleteCollectCleanupTask(task.Id)
			s.infof("采集引擎 %s 已由其它软网关接管，放弃清空任务（网关 %s）", engineURL, task.GatewayId)
			return
		}
	}

	var firstErr error
	// 引擎专用的场景先用「清空所有点位」快速清掉遗留点位；
	// 这个接口不会被通道连接锁阻塞，比逐台删设备/通道更可靠。
	if task.DeleteAll {
		if err := client.ClearTags(); err != nil && !isEngineNotFound(err) {
			firstErr = err
		}
	}
	var devices []collectengine.Device
	if firstErr == nil {
		devices, firstErr = client.ListDevices(0)
	}
	if firstErr == nil {
		for _, device := range devices {
			if !task.DeleteAll && !containsInt64(channelIDs, device.ChannelId) {
				continue
			}
			if err := client.DeleteDevice(device.Id); err != nil && !isEngineNotFound(err) && firstErr == nil {
				firstErr = err
			}
		}
	}
	if firstErr == nil {
		remoteChannels := channelIDs
		if task.DeleteAll {
			list, err := client.ListChannels()
			if err != nil {
				firstErr = err
			} else {
				remoteChannels = make([]int64, 0, len(list))
				for _, channel := range list {
					remoteChannels = append(remoteChannels, channel.Id)
				}
			}
		}
		for _, channelID := range remoteChannels {
			if err := client.DeleteChannel(channelID); err != nil && !isEngineNotFound(err) && firstErr == nil {
				firstErr = err
			}
		}
	}

	if firstErr == nil {
		_ = db.DeleteCollectCleanupTask(task.Id)
		s.infof("采集引擎清理完成: %s（网关 %s，通道 %v）", engineURL, task.GatewayId, channelIDs)
		return
	}

	task.Attempts++
	task.LastError = firstErr.Error()
	task.NextRetryAt = time.Now().Add(cleanupRetryDelay(task.Attempts)).UnixMilli()
	if err := db.UpdateCollectCleanupTask(task); err != nil {
		s.debugf("采集引擎清理任务 %s 更新失败: %v", task.Id, err)
	}
	s.debugf("采集引擎清理失败（第 %d 次，稍后重试）: %s: %v", task.Attempts, engineURL, firstErr)
}

func cleanupRetryDelay(attempts int) time.Duration {
	delay := cleanupRetryBaseDelay
	for i := 1; i < attempts && delay < cleanupRetryMaxDelay; i++ {
		delay *= 2
	}
	if delay > cleanupRetryMaxDelay {
		delay = cleanupRetryMaxDelay
	}
	return delay
}

func containsInt64(values []int64, want int64) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func isEngineNotFound(err error) bool {
	var apiErr *collectengine.APIError
	return errors.As(err, &apiErr) && apiErr.Code == 404
}
