package container

import (
	interfaces "iotserver/internal/iotserver/core/interface"
	"iotserver/internal/pkg/di"
)

// DI
var (
	DriverAppName = di.TypeInstanceToName((*interfaces.DriverLibApp)(nil))
)

func DriverAppFrom(get di.Get) interfaces.DriverLibApp {
	return get(DriverAppName).(interfaces.DriverLibApp)
}

var (
	DriverServiceAppName = di.TypeInstanceToName((*interfaces.DriverServiceApp)(nil))
)

func DriverServiceAppFrom(get di.Get) interfaces.DriverServiceApp {
	return get(DriverServiceAppName).(interfaces.DriverServiceApp)
}
