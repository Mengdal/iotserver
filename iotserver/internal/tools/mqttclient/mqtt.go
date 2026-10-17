package mqttclient

import (
	"context"
	"iotserver/internal/dtos"
)

type MQTTClient interface {
	RegisterConnectCallback(dtos.ConnectHandler)
	RegisterDisconnectCallback(dtos.CallbackHandler)
	AsyncPublish(ctx context.Context, topic string, payload []byte, isSync bool)
	Close()
	GetConnectStatus() bool
	// Subscribe 追加订阅主题（会记入内部集合，断线重连后自动重新订阅）
	Subscribe(topics ...string) error
	// Unsubscribe 取消订阅并从内部集合移除
	Unsubscribe(topics ...string) error
}
