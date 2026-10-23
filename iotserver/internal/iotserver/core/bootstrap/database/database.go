//
// Copyright (C) 2020 IOTech Ltd
//
// SPDX-License-Identifier: Apache-2.0

package database

import (
	"context"
	"errors"
	"iotserver/internal/dtos"
	"iotserver/internal/iotserver/core/config"
	"iotserver/internal/iotserver/core/container"
	"iotserver/internal/iotserver/core/infrastructure/sqlite"
	"iotserver/internal/pkg/constants"
	"iotserver/internal/tools/datadb/duckdb"

	"iotserver/internal/pkg/di"
	"iotserver/internal/pkg/logger"
	"iotserver/internal/pkg/startup"
	"sync"

	interfaces "iotserver/internal/iotserver/core/interface"
	pkgContainer "iotserver/internal/pkg/container"
)

// Database contains references to dependencies required by the database bootstrap implementation.
type Database struct {
	database *config.ConfigurationStruct
}

// NewDatabase is a factory method that returns an initialized Database receiver struct.
// defaultDataRetentionDays 数据层默认保留天数（0 表示永久保留）。
// 按租户的保留策略由应用层 retention 任务执行；这里只是数据层启动时的兜底值。
const defaultDataRetentionDays = 30

func NewDatabase(database *config.ConfigurationStruct) Database {
	return Database{
		database: database,
	}
}

// Return the dbClient interfaces
func (d Database) newDBClient(
	lc logger.LoggingClient) (interfaces.DBClient, error) {

	databaseInfo := d.database.GetDatabaseInfo()["Primary"]
	switch databaseInfo.Type {
	case string(constants.SQLite):
		return sqlite.NewClient(dtos.Configuration{
			Username:   databaseInfo.Username,
			Password:   databaseInfo.Password,
			DataSource: databaseInfo.DataSource,
		}, lc)
	default:
		panic(errors.New("database configuration error: 仅支持 sqlite"))
	}
}

func (d Database) newDataDBClient(
	lc logger.LoggingClient, dbClient interfaces.DBClient) (interfaces.DataDBClient, error) {
	dataDbInfo := d.database.GetDataDatabaseInfo()["Primary"]

	switch dataDbInfo.Type {
	case string(constants.DuckDB):
		// 单文件列存：DataSource = DuckDB 数据库文件路径
		// 保留天数默认值；按租户的保留策略由应用层（retention）执行
		return duckdb.NewClient(dtos.Configuration{
			DataSource:    dataDbInfo.DataSource,
			RetentionDays: defaultDataRetentionDays,
		}, lc)
	default:
		panic(errors.New("database configuration error: 仅支持 duckdb"))
	}
}

// BootstrapHandler fulfills the BootstrapHandler contract and initializes the database.
func (d Database) BootstrapHandler(
	ctx context.Context,
	wg *sync.WaitGroup,
	startupTimer startup.Timer,
	dic *di.Container) bool {
	lc := pkgContainer.LoggingClientFrom(dic.Get)

	// initialize Metadata db.
	dbClient, err := d.newDBClient(lc)
	if err != nil {
		panic(err)
	}

	dic.Update(di.ServiceConstructorMap{
		container.DBClientInterfaceName: func(get di.Get) interface{} {
			return dbClient
		},
	})

	// initialize Data db.
	dataDbClient, err := d.newDataDBClient(lc, dbClient)
	if err != nil {
		panic(err)
	}

	dic.Update(di.ServiceConstructorMap{
		container.DataDBClientInterfaceName: func(get di.Get) interface{} {
			return dataDbClient
		},
	})

	lc.Info("DatabaseInfo connected")

	wg.Add(1)
	go func() {
		defer wg.Done()
		select {
		case <-ctx.Done():
			interfaces.DMIFrom(di.GContainer.Get).StopAllInstance() //stop all instance
			container.DBClientFrom(di.GContainer.Get).CloseSession()
			container.DataDBClientFrom(di.GContainer.Get).CloseSession()
			lc.Info("DatabaseInfo disconnected")
		}
	}()
	return true
}
