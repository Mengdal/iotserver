package container

import (
	interfaces "iotserver/internal/iotserver/core/interface"
	"iotserver/internal/pkg/di"
)

// IotpSyncItfName iotp 元数据同步服务。
var IotpSyncItfName = di.TypeInstanceToName((*interfaces.IotpSyncItf)(nil))

// IotpSyncFrom 取 iotp 同步服务；未注册（单测/精简装配）时返回 nil。
func IotpSyncFrom(get di.Get) interfaces.IotpSyncItf {
	value := get(IotpSyncItfName)
	if value == nil {
		return nil
	}
	sync, _ := value.(interfaces.IotpSyncItf)
	return sync
}
