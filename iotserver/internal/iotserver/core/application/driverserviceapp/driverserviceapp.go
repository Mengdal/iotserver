package driverserviceapp

import (
	"context"
	interfaces "iotserver/internal/iotserver/core/interface"
	"iotserver/internal/pkg/container"
	"iotserver/internal/pkg/di"
	"iotserver/internal/pkg/logger"
)

type driverServiceApp struct {
	dic *di.Container
	lc  logger.LoggingClient

	*driverServiceAppM
}

func NewDriverServiceApp(ctx context.Context, dic *di.Container) interfaces.DriverServiceApp {
	return &driverServiceApp{
		dic: dic,
		lc:  container.LoggingClientFrom(dic.Get),

		driverServiceAppM: newDriverServiceApp(ctx, dic),
	}
}
