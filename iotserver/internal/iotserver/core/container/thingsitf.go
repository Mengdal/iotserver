package container

import (
	interfaces "iotserver/internal/iotserver/core/interface"
	"iotserver/internal/pkg/di"
)

// ThingsItfName things 平台集成
var ThingsItfName = di.TypeInstanceToName((*interfaces.ThingsItf)(nil))

// ThingsItfFrom 从容器取 things 应用
func ThingsItfFrom(get di.Get) interfaces.ThingsItf {
	v := get(ThingsItfName)
	if v == nil {
		return nil
	}
	itf, _ := v.(interfaces.ThingsItf)
	return itf
}
