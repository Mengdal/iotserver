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
}
