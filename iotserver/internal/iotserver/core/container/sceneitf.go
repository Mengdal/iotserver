package container

import (
	interfaces "iotserver/internal/iotserver/core/interface"
	"iotserver/internal/pkg/di"
)

// SceneItfName 高级能力 - 场景联动
var SceneItfName = di.TypeInstanceToName((*interfaces.SceneItf)(nil))

// SceneItfFrom 从容器取场景联动应用
func SceneItfFrom(get di.Get) interfaces.SceneItf {
	return get(SceneItfName).(interfaces.SceneItf)
}
