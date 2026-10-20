package interfaces

import (
	"context"
	"encoding/json"
)

// EdgeCtlItf 参考系统（things / iotddc）网关的下行控制能力。
type EdgeCtlItf interface {
	// Restart 重启网关
	Restart(ctx context.Context, pk, sn string) error
	// SetProperty 写设备属性，返回网关回执的 status/message
	SetProperty(ctx context.Context, pk, sn, dn, identifier, value string) (bool, string, error)
	// TagTree 拉取网关的设备/测点树
	TagTree(ctx context.Context, pk, sn string) (json.RawMessage, error)
}
