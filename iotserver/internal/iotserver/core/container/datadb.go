package container

import (
	interfaces "iotserver/internal/iotserver/core/interface"
	"iotserver/internal/pkg/di"
)

var DataDBClientInterfaceName = di.TypeInstanceToName((*interfaces.DataDBClient)(nil))

func DataDBClientFrom(get di.Get) interfaces.DataDBClient {
	return get(DataDBClientInterfaceName).(interfaces.DataDBClient)
}
