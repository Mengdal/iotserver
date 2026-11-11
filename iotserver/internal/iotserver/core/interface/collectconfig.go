package interfaces

import (
	"context"

	"iotserver/internal/dtos"
)

// CollectConfigItf 采集配置：通道 / 设备绑定 / 点表全部直接读写采集引擎，
// 设备查询使用平台内部设备。
type CollectConfigItf interface {
	CollectConfigGet(ctx context.Context, deviceId string) (dtos.CollectConfigResponse, error)
	CollectConfigSave(ctx context.Context, req dtos.CollectConfigSaveRequest) error
	CollectConfigTemplate(ctx context.Context, deviceId string) ([]dtos.CollectPointDTO, error)
	CollectConfigLatest(ctx context.Context, deviceId string) (dtos.CollectMonitorResponse, error)
	CollectConfigSetEnabled(ctx context.Context, req dtos.CollectConfigEnabledRequest) error
	CollectConfigExport(ctx context.Context, deviceId string) (*dtos.ExportFile, error)
	CollectConfigExportAll(ctx context.Context, channelId string) (*dtos.ExportFile, error)
	CollectConfigImportAllPreview(ctx context.Context, file *dtos.ImportFile) (dtos.CollectImportPreview, error)
	CollectConfigImportAll(ctx context.Context, file *dtos.ImportFile) (dtos.CollectImportResult, error)

	CollectOverview(ctx context.Context) (dtos.CollectOverviewDTO, error)
	CollectRealtime(ctx context.Context, channelId, deviceId, keyword string) ([]dtos.CollectRealtimeValueDTO, error)
	CollectWrite(ctx context.Context, req dtos.CollectWriteRequest) error
	CollectChannels(ctx context.Context) ([]dtos.CollectChannelDTO, error)
	CollectChannelSave(ctx context.Context, req dtos.CollectChannelSaveRequest) (dtos.CollectChannelDTO, error)
	CollectChannelDelete(ctx context.Context, id string) error
	// ScheduleGatewayCleanup 删除软网关时登记后台清理任务：
	// 清理采集引擎中该网关的通道与设备绑定，避免重新添加同一引擎后出现旧数据。
	ScheduleGatewayCleanup(ctx context.Context, gatewayID string) error

	CollectDevices(ctx context.Context) ([]dtos.CollectDeviceDTO, error)
	CollectChannelDevices(ctx context.Context, channelId string) ([]dtos.CollectDeviceDTO, error)
	CollectChannelBindDevices(ctx context.Context, channelId string, deviceIds []string) error
	CollectChannelUnbindDevice(ctx context.Context, channelId, deviceId string) error
}
