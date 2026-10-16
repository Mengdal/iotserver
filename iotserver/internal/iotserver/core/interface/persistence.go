package interfaces

import (
	"context"

	"iotserver/internal/dtos"
)

type PersistItf interface {
	PersistDeviceItf
}

// PersistDeviceItf 时序数据持久化接口。
//
// 所有方法都接收 context.Context：租户作用域由它承载（见 pkg/tenant）。
// 读方法用该 ctx 查设备/产品，仓储层的租户插件据此做行级隔离；
// 写方法在上行链路里没有租户上下文，由实现按设备归属推导后注入。
type PersistDeviceItf interface {
	SaveDeviceThingModelData(ctx context.Context, req dtos.ThingModelMessage) error
	SearchDeviceThingModelPropertyData(ctx context.Context, req dtos.ThingModelPropertyDataRequest) (interface{}, error)
	SearchDeviceThingModelHistoryPropertyData(ctx context.Context, req dtos.ThingModelPropertyDataRequest) (interface{}, int, error)
	SearchDeviceThingModelEventData(ctx context.Context, req dtos.ThingModelEventDataRequest) ([]dtos.ThingModelEventDataResponse, int, error)
	SearchDeviceThingModelServiceData(ctx context.Context, req dtos.ThingModelServiceDataRequest) ([]dtos.ThingModelServiceDataResponse, int, error)
	SearchDeviceMsgCount(ctx context.Context, startTime, endTime int64) (int, error)
}
