package container

import (
	interfaces "iotserver/internal/iotserver/core/interface"
	"iotserver/internal/pkg/di"
)

// EdgeIngestName 参考系统（/edge）上行接入服务
var EdgeIngestName = di.TypeInstanceToName((*interfaces.EdgeIngestItf)(nil))

// EdgeIngestFrom 从容器取参考系统上行接入服务
func EdgeIngestFrom(get di.Get) interfaces.EdgeIngestItf {
	return get(EdgeIngestName).(interfaces.EdgeIngestItf)
}
