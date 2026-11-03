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
	CollectConfigExport(ctx context.Context, deviceId string) (*dtos.ExportFile, error)
	CollectConfigImport(ctx context.Context, deviceId string, file *dtos.ImportFile) ([]dtos.CollectPointDTO, error)

	CollectChannels(ctx context.Context) ([]dtos.CollectChannelDTO, error)
	CollectChannelSave(ctx context.Context, req dtos.CollectChannelSaveRequest) (dtos.CollectChannelDTO, error)
	CollectChannelDelete(ctx context.Context, id string) error

	CollectDevices(ctx context.Context) ([]dtos.CollectDeviceDTO, error)
	CollectChannelDevices(ctx context.Context, channelId string) ([]dtos.CollectDeviceDTO, error)
	CollectChannelBindDevices(ctx context.Context, channelId string, deviceIds []string) error
	CollectChannelUnbindDevice(ctx context.Context, channelId, deviceId string) error
}
