package container

import (
	interfaces "iotserver/internal/iotserver/core/interface"
	"iotserver/internal/pkg/di"
)

// UserItfName
var UserItfName = di.TypeInstanceToName((*interfaces.UserItf)(nil))

// UserItfFrom
func UserItfFrom(get di.Get) interfaces.UserItf {
	return get(UserItfName).(interfaces.UserItf)
}
