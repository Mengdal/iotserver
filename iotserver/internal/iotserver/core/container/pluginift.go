package container

import (
	interfaces "iotserver/internal/iotserver/core/interface"
	"iotserver/internal/pkg/di"
)

// PluginAppName 原生插件管理服务在 DI 容器中的名字。
var PluginAppName = di.TypeInstanceToName((*interfaces.PluginApp)(nil))

// PluginAppFrom 从容器取原生插件管理服务。
func PluginAppFrom(get di.Get) interfaces.PluginApp {
	return get(PluginAppName).(interfaces.PluginApp)
}
