package container

import (
	interfaces "iotserver/internal/iotserver/core/interface"
	"iotserver/internal/pkg/di"
)

// IotpCollectorItfName iotp 多节点采集服务。
var IotpCollectorItfName = di.TypeInstanceToName((*interfaces.IotpCollectorItf)(nil))

// IotpCollectorFrom 取 iotp 采集器；未注册（单测/精简装配）时返回 nil。
func IotpCollectorFrom(get di.Get) interfaces.IotpCollectorItf {
	value := get(IotpCollectorItfName)
	if value == nil {
		return nil
	}
	collector, _ := value.(interfaces.IotpCollectorItf)
	return collector
}
