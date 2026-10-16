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
	"iotserver/internal/tools/datadb/duckdb"
	"iotserver/internal/tools/datadb/ducklake"
	"strconv"
	"strings"

	"iotserver/internal/pkg/di"
	"iotserver/internal/pkg/logger"
	"iotserver/internal/pkg/startup"
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
	case string(constants.DuckDB):
		// 单文件列存（默认）：DataSource = DuckDB 数据库文件路径
		// 保留天数来自「系统配置 → 数据配置 → 数据保留天数」
		return duckdb.NewClient(dtos.Configuration{
			DataSource:    dataDbInfo.DataSource,
			RetentionDays: retentionDays(dbClient, lc),
		}, lc)
	case string(constants.DuckLake):
		// 湖仓形态：DataSource = catalog（元数据）路径；Dsn = Parquet 数据目录
		// 保留天数同样来自「系统配置 → 数据配置 → 数据保留天数」
		return ducklake.NewClient(dtos.Configuration{
			DataSource:       dataDbInfo.DataSource,
			Dsn:              dataDbInfo.Dsn,
			RetentionDays:    retentionDays(dbClient, lc),
			OverrideDataPath: dataDbInfo.OverrideDataPath,
		}, lc)
	default:
		panic(errors.New("database configuration error: 仅支持 duckdb / ducklake"))
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
