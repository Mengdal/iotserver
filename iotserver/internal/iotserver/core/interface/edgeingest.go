package interfaces

import (
	"context"

	"iotserver/internal/dtos"
	"iotserver/internal/tools/edgeprotocol"
)

// EdgeIngestItf 参考系统（/edge）上行接入服务对外暴露的能力。
//
// 目前只有「同步设备」：把网关实时返回的设备/测点树对齐进平台，
// 与网关自动上报走同一份注册逻辑（含排除名单与自动注册开关）。
type EdgeIngestItf interface {
	SyncDevices(ctx context.Context, gatewayID string, tags []edgeprotocol.Tag) (dtos.EdgeDeviceSyncResult, error)
}
