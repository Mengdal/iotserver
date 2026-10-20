package container

import (
	interfaces "iotserver/internal/iotserver/core/interface"
	"iotserver/internal/pkg/di"
)

// EdgeCtlName 参考系统网关控制
var EdgeCtlName = di.TypeInstanceToName((*interfaces.EdgeCtlItf)(nil))

// EdgeCtlFrom 从容器取参考系统网关控制客户端
func EdgeCtlFrom(get di.Get) interfaces.EdgeCtlItf {
	return get(EdgeCtlName).(interfaces.EdgeCtlItf)
}
