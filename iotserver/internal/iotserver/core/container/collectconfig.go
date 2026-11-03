package container

import (
	interfaces "iotserver/internal/iotserver/core/interface"
	"iotserver/internal/pkg/di"
)

// CollectConfigItfName 设备级采集配置。
var CollectConfigItfName = di.TypeInstanceToName((*interfaces.CollectConfigItf)(nil))

func CollectConfigFrom(get di.Get) interfaces.CollectConfigItf {
	value := get(CollectConfigItfName)
	if value == nil {
		return nil
	}
	app, _ := value.(interfaces.CollectConfigItf)
	return app
}
