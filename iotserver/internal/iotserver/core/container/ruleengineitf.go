package container

import (
	interfaces "iotserver/internal/iotserver/core/interface"
	"iotserver/internal/pkg/di"
)

// RuleEngineItfName 高级能力 - 规则引擎
var RuleEngineItfName = di.TypeInstanceToName((*interfaces.RuleEngineItf)(nil))

// RuleEngineItfFrom 从容器取规则引擎应用
func RuleEngineItfFrom(get di.Get) interfaces.RuleEngineItf {
	return get(RuleEngineItfName).(interfaces.RuleEngineItf)
}
