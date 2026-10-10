package streamclient

import "iotserver/internal/dtos"

type StreamClient interface {
	Send(data dtos.RpcData)
	Recv() <-chan dtos.RpcData
}
