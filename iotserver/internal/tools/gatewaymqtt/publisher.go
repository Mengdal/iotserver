// Package gatewaymqtt 提供网关下行通道（平台 -> 网关）的发布出口。
//
// 解耦原因：网关管理应用需要在「下发配置/命令」时发布 MQTT 消息，而持有 MQTT 连接的是
// 接入服务（gatewayingest）。这里用一个可替换的发布器做桥接：
//   - 运行期由接入服务在启动时 SetPublisher 注入真实实现；
//   - 未注入（未启用网关接入/单测）时返回 ErrNotReady，调用方据此降级为「仅持久化，待网关上线后同步」。
package gatewaymqtt

import (
	"errors"
	"sync"
)

// ErrNotReady 下行通道尚未就绪（MQTT 未连接）
var ErrNotReady = errors.New("gateway downlink channel is not ready")

// Publisher 下行发布器
type Publisher interface {
	Publish(topic string, payload []byte) error
}

var (
	mu        sync.RWMutex
	publisher Publisher
)

// SetPublisher 注入下行发布器（接入服务启动时调用）
func SetPublisher(p Publisher) {
	mu.Lock()
	defer mu.Unlock()
	publisher = p
}

// Publish 通过下行通道发布消息
func Publish(topic string, payload []byte) error {
	mu.RLock()
	p := publisher
	mu.RUnlock()
	if p == nil {
		return ErrNotReady
	}
	return p.Publish(topic, payload)
}

// Ready 下行通道是否可用
func Ready() bool {
	mu.RLock()
	defer mu.RUnlock()
	return publisher != nil
}

// TopicUp / TopicDown 主题约定：网关按 SN 唯一标识
const (
	TopicPrefixUp   = "iot/up/"
	TopicPrefixDown = "iot/down/"
)

// UpTopic 上行主题（网关发布，平台订阅）
//   - data  实时数据
//   - batch 断点续传批量补传
//   - status 心跳/状态
//   - offline 离线（MQTT LWT 遗嘱）
func UpTopic(sn, kind string) string {
	return TopicPrefixUp + sn + "/" + kind
}

// DownTopic 下行主题（平台发布，网关订阅）
//   - config 配置下发
//   - cmd    命令（restart / ota / sync）
func DownTopic(sn, kind string) string {
	return TopicPrefixDown + sn + "/" + kind
}
