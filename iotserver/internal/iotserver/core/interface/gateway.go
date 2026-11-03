package interfaces

import (
	"context"

	"iotserver/internal/dtos"
)

// GatewayItf 网关管理：第三方工业网关的注册、配置下发与设备绑定
type GatewayItf interface {
	GatewayAdd(ctx context.Context, req dtos.GatewayAddRequest) (dtos.GatewayResponse, error)
	GatewayUpdate(ctx context.Context, req dtos.GatewayUpdateRequest) error
	GatewayDelete(ctx context.Context, id string) error
	GatewayById(ctx context.Context, id string) (dtos.GatewayResponse, error)
	GatewaySearch(ctx context.Context, req dtos.GatewaySearchQueryRequest) ([]dtos.GatewayResponse, uint32, error)
	// GatewayPushConfig 下发采集/上报配置（版本 +1 并通过 MQTT 下行通知网关）
	GatewayPushConfig(ctx context.Context, req dtos.GatewayConfigRequest) (int, error)
	// GatewayCommand 下发命令（restart / ota / sync）
	GatewayCommand(ctx context.Context, req dtos.GatewayCommandRequest) error
	// GatewayOnline 处理网关心跳/上线（供接入服务调用）
	GatewayOnline(ctx context.Context, sn, ip string, ts int64) error
	// GatewayOffline 处理网关离线（LWT / 超时扫描）
	GatewayOffline(ctx context.Context, sn string) error
}
