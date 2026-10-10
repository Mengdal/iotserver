package container

import (
	"iotserver/internal/pkg/di"
	"iotserver/internal/tools/streamclient"
)

var StreamClientName = di.TypeInstanceToName((*streamclient.StreamClient)(nil))

func StreamClientFrom(get di.Get) streamclient.StreamClient {
	return get(StreamClientName).(streamclient.StreamClient)
}
