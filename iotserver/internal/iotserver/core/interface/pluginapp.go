package interfaces

import (
	"context"
	"io"

	"iotserver/internal/dtos"
)

// PluginApp 原生插件包与实例管理。
type PluginApp interface {
	// UploadPackage 上传并安装一个 tar.gz 插件包。
	UploadPackage(ctx context.Context, filename string, reader io.Reader) (dtos.PluginPackageDTO, error)
	ListPackages(ctx context.Context) ([]dtos.PluginPackageDTO, error)
	DeletePackage(ctx context.Context, id string) error

	CreateInstance(ctx context.Context, req dtos.PluginInstanceCreateRequest) (dtos.PluginInstanceDTO, error)
	ListInstances(ctx context.Context) ([]dtos.PluginInstanceDTO, error)
	StartInstance(ctx context.Context, id string) error
	StopInstance(ctx context.Context, id string) error
	RestartInstance(ctx context.Context, id string) error
	DeleteInstance(ctx context.Context, id string) error
	InstanceLogs(ctx context.Context, id string, tail int) (dtos.PluginLogsDTO, error)
}
