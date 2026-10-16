package driverapp

import (
	"context"

	"iotserver/internal/models"
	"iotserver/internal/pkg/tenant"
)

func (app *driverLibApp) GetDeviceLibraryAndMirrorConfig(dlId string) (dl models.DeviceLibrary, dc models.DockerConfig, err error) {
	// 1. 获取驱动库
	dl, err = app.DriverLibById(dlId)
	if err != nil {
		app.lc.Errorf("1.DeviceLibraryOperate device library id:%s, err:%n", dlId, err)
		return
	}

	// 2.获取docker仓库配置
	if !dl.IsInternal {
		// 平台作用域：系统路径，不按租户过滤
		dc, err = app.dbClient.WithCtx(tenant.Platform(context.Background())).DockerConfigById(dl.DockerConfigId)
		if err != nil {
			app.lc.Errorf("2.DeviceLibraryOperate docker hub, id:%s, DockerConfigId:%s, err:%v", dlId, dl.DockerConfigId, err)
			return
		}
	}
	return
}
