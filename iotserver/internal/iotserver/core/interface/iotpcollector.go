package interfaces

// IotpCollectorItf iotp 采集器的拓扑刷新入口。
// 网关新增/修改/删除后由 gatewayApp 调用，重新发现受管软网关的 MQTT 目标。
type IotpCollectorItf interface {
	// RefreshDiscovery 触发一次异步刷新；多次调用会合并。
	RefreshDiscovery()
}
