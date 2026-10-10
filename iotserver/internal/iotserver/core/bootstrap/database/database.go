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
	"iotserver/internal/iotserver/core/infrastructure/mysql"
	"iotserver/internal/iotserver/core/infrastructure/sqlite"
	"iotserver/internal/pkg/constants"
	"iotserver/internal/tools/datadb/ducklake"
	"iotserver/internal/tools/datadb/tdengine"
	"iotserver/internal/tools/datadb/tstorage"
	"strconv"
	"strings"

	"iotserver/internal/pkg/di"
	"iotserver/internal/pkg/logger"
	"iotserver/internal/pkg/startup"
	"iotserver/internal/tools/datadb/leveldb"
	"sync"

	interfaces "iotserver/internal/iotserver/core/interface"
	pkgContainer "iotserver/internal/pkg/container"
	"iotserver/internal/pkg/seed"
)

// Database contains references to dependencies required by the database bootstrap implementation.
type Database struct {
	database *config.ConfigurationStruct
}

// NewDatabase is a factory method that returns an initialized Database receiver struct.
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
	case string(constants.MySQL):
		return mysql.NewClient(dtos.Configuration{
			Dsn: databaseInfo.Dsn,
		}, lc)
	case string(constants.SQLite):
		return sqlite.NewClient(dtos.Configuration{
			Username:   databaseInfo.Username,
			Password:   databaseInfo.Password,
			DataSource: databaseInfo.DataSource,
		}, lc)
	default:
		panic(errors.New("database configuration error"))
	}
}

func (d Database) newDataDBClient(
	lc logger.LoggingClient, dbClient interfaces.DBClient) (interfaces.DataDBClient, error) {
	dataDbInfo := d.database.GetDataDatabaseInfo()["Primary"]

	switch dataDbInfo.Type {
	case string(constants.LevelDB):
		return leveldb.NewClient(dtos.Configuration{
			DataSource: dataDbInfo.DataSource,
		}, lc)
	case string(constants.Tstorage):
		return tstorage.NewClient(dtos.Configuration{
			DataSource: dataDbInfo.DataSource,
		}, lc)
	case string(constants.TDengine):
		return tdengine.NewClient(dtos.Configuration{
			Dsn: dataDbInfo.Dsn,
		}, lc)
	case string(constants.DuckLake):
		// DataSource = catalog（元数据）路径；Dsn = Parquet 数据目录
		// 保留天数来自「系统配置 → 数据配置 → 数据保留天数」，让该配置项真实生效
		return ducklake.NewClient(dtos.Configuration{
			DataSource:    dataDbInfo.DataSource,
			Dsn:           dataDbInfo.Dsn,
			RetentionDays: retentionDays(dbClient, lc),
		}, lc)
	default:
		panic(errors.New("database configuration error"))

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

// retentionDays 读取「数据保留天数」配置（缺省 30 天；0 表示永久保留）
func retentionDays(dbClient interfaces.DBClient, lc logger.LoggingClient) int {
	const defaultDays = 30
	cfg, err := dbClient.SysConfigByKey(seed.ConfigKeyRetentionDays)
	if err != nil {
		return defaultDays
	}
	days, convErr := strconv.Atoi(strings.TrimSpace(cfg.ConfigValue))
	if convErr != nil || days < 0 {
		lc.Warnf("database: invalid retention days %q, fallback to %d", cfg.ConfigValue, defaultDays)
		return defaultDays
	}
	return days
}
