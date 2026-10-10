package container

import (
	interfaces "iotserver/internal/iotserver/core/interface"
	"iotserver/internal/pkg/di"
)

// ResourceItfName 高级能力 - 资源管理
var ResourceItfName = di.TypeInstanceToName((*interfaces.ResourceItf)(nil))

// ResourceItfFrom 从容器取资源管理应用
func ResourceItfFrom(get di.Get) interfaces.ResourceItf {
	return get(ResourceItfName).(interfaces.ResourceItf)
}
