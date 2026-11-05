//go:build !noduckdb

package database

import (
	"iotserver/internal/dtos"
	interfaces "iotserver/internal/iotserver/core/interface"
	"iotserver/internal/pkg/logger"
	"iotserver/internal/tools/datadb/duckdb"
)

// newDuckDBClient 默认构建包含 DuckDB（需要 CGO）。
func newDuckDBClient(dataSource string, retentionDays int, lc logger.LoggingClient) (interfaces.DataDBClient, error) {
	return duckdb.NewClient(dtos.Configuration{
		DataSource:    dataSource,
		RetentionDays: retentionDays,
	}, lc)
}
