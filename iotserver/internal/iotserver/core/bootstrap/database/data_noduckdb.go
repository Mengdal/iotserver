//go:build noduckdb

package database

import (
	"errors"

	interfaces "iotserver/internal/iotserver/core/interface"
	"iotserver/internal/pkg/logger"
)

// newDuckDBClient noduckdb 构建（CGO_ENABLED=0）不包含 DuckDB。
func newDuckDBClient(string, int, logger.LoggingClient) (interfaces.DataDBClient, error) {
	return nil, errors.New("当前构建未包含 DuckDB，请将 [Databases.Data].Type 配置为 iotp，或使用默认构建")
}
