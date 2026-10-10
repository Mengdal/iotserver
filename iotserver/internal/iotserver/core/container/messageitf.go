package container

import (
	interfaces "iotserver/internal/iotserver/core/interface"
	"iotserver/internal/pkg/di"
)

var MessageItfName = di.TypeInstanceToName((*interfaces.MessageItf)(nil))

func MessageItfFrom(get di.Get) interfaces.MessageItf {
	return get(MessageItfName).(interfaces.MessageItf)
}

var MessageStoreItfName = di.TypeInstanceToName((*interfaces.MessageStores)(nil))

func MessageStoreItfFrom(get di.Get) interfaces.MessageStores {
	return get(MessageStoreItfName).(interfaces.MessageStores)
}
