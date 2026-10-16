package container

import (
	interfaces "iotserver/internal/iotserver/core/interface"
	"iotserver/internal/pkg/di"
)

// GatewayItfName 网关管理
var GatewayItfName = di.TypeInstanceToName((*interfaces.GatewayItf)(nil))

// GatewayItfFrom 从容器取网关应用
func GatewayItfFrom(get di.Get) interfaces.GatewayItf {
	return get(GatewayItfName).(interfaces.GatewayItf)
}
