package container

import (
	interfaces "iotserver/internal/iotserver/core/interface"
	"iotserver/internal/pkg/di"
)

// SolutionItfName 方案（行业模板一键应用）
var SolutionItfName = di.TypeInstanceToName((*interfaces.SolutionItf)(nil))

// SolutionItfFrom 从容器取方案应用
func SolutionItfFrom(get di.Get) interfaces.SolutionItf {
	return get(SolutionItfName).(interfaces.SolutionItf)
}
