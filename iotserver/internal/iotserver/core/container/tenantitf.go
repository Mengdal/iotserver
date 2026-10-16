package container

import (
	interfaces "iotserver/internal/iotserver/core/interface"
	"iotserver/internal/pkg/di"
)

// TenantItfName 租户管理
var TenantItfName = di.TypeInstanceToName((*interfaces.TenantItf)(nil))

// TenantItfFrom 从容器取租户应用
func TenantItfFrom(get di.Get) interfaces.TenantItf {
	return get(TenantItfName).(interfaces.TenantItf)
}
