package container

import (
	interfaces "iotserver/internal/iotserver/core/interface"
	"iotserver/internal/pkg/di"
)

// DeviceItfName
var DeviceItfName = di.TypeInstanceToName((*interfaces.DeviceItf)(nil))

// DeviceItfFrom
func DeviceItfFrom(get di.Get) interfaces.DeviceItf {
	return get(DeviceItfName).(interfaces.DeviceItf)
}
