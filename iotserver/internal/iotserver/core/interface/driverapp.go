package interfaces

import (
	"context"
	"iotserver/internal/dtos"
	"iotserver/internal/models"
)

type DriverLibApp interface {
	AddDriverLib(ctx context.Context, dl dtos.DeviceLibraryAddRequest) error
	DeleteDeviceLibraryById(ctx context.Context, id string) error
	DeviceLibraryById(ctx context.Context, id string) (models.DeviceLibrary, error)
	DeviceLibrariesSearch(ctx context.Context, req dtos.DeviceLibrarySearchQueryRequest) ([]models.DeviceLibrary, uint32, error)
	UpdateDeviceLibrary(ctx context.Context, update dtos.UpdateDeviceLibrary) error
	UpgradeDeviceLibrary(ctx context.Context, req dtos.DeviceLibraryUpgradeRequest) error
	DriverLibById(dlId string) (models.DeviceLibrary, error)
	GetDriverClassify(ctx context.Context, req dtos.DriverClassifyQueryRequest) ([]dtos.DriverClassifyResponse, uint32, error)
	GetDeviceLibraryAndMirrorConfig(dlId string) (dl models.DeviceLibrary, dc models.DockerConfig, err error)
	// SystemComponents 内置系统组件（Mosquitto / eKuiper / IotEdgeDB 等）及运行状态。
	SystemComponents(ctx context.Context) ([]dtos.SystemComponentDTO, error)
	// ComponentLogs 读取系统组件配置的日志文件尾部。
	ComponentLogs(ctx context.Context, id string, tail int) (dtos.SystemComponentLogsDTO, error)
	DriverDownConfigItf
}
type DriverDownConfigItf interface {
	DownConfigAdd(ctx context.Context, req dtos.DockerConfigAddRequest) error
	DownConfigUpdate(ctx context.Context, req dtos.DockerConfigUpdateRequest) error
	DownConfigSearch(ctx context.Context, req dtos.DockerConfigSearchQueryRequest) ([]models.DockerConfig, uint32, error)
	DownConfigDel(ctx context.Context, id string) error
}
