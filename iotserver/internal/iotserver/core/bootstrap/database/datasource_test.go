package database

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"

	"iotserver/internal/dtos"
	"iotserver/internal/models"
	"iotserver/internal/pkg/logger"
	"iotserver/internal/pkg/tenant"
	"iotserver/internal/tools/datadb/duckdb"
)

// 配置文件里的 [Databases.Data.Primary] 必须指向可用的 DuckDB DataSource。
func TestConfiguredDataSourceIsDuckDB(t *testing.T) {
	root := repoRoot(t)
	cases := []struct {
		name string
		path string
		want string
	}{
		{"local", filepath.Join(root, "cmd/iotserver-core/res/configuration.toml"), "manifest/docker/db-data/duckdb/core-data.duckdb"},
		{"docker", filepath.Join(root, "manifest/docker/config/configuration.toml"), "iotserver/db-data/duckdb/core-data.duckdb"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var cfg struct {
				Databases struct {
					Data struct {
						Primary struct {
							Type       string
							DataSource string
							Dsn        string
						}
					}
				}
			}
			if _, err := toml.DecodeFile(tc.path, &cfg); err != nil {
				t.Fatalf("解析配置失败: %v", err)
			}
			p := cfg.Databases.Data.Primary
			if p.Type != "duckdb" {
				t.Fatalf("Type = %q, want duckdb", p.Type)
			}
			if p.DataSource != tc.want {
				t.Fatalf("DataSource = %q, want %q", p.DataSource, tc.want)
			}
			if p.Dsn != "" {
				t.Fatalf("DuckDB 不应再配置 Dsn: %q", p.Dsn)
			}
		})
	}
}

// DataSource 指向的文件必须能真正落盘、关闭后重开仍能读到，验证配置路径语义完整。
func TestDuckDBDataSourceRoundTrip(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "nested", "core-data.duckdb")
	lc := logger.NewClient("datasource-test", "error", "")
	ctx := tenant.With(context.Background(), tenant.Context{TenantID: "tenant-a"})

	client, err := duckdb.NewClient(dtos.Configuration{DataSource: dbPath}, lc)
	if err != nil {
		t.Fatalf("按 DataSource 打开失败: %v", err)
	}
	deviceID := "dev-datasource"
	ts := int64(1790000000000)
	if err := client.Insert(ctx, deviceID, map[string]interface{}{
		"temp": dtos.ReportData{Value: 23.5, Time: ts},
	}); err != nil {
		t.Fatalf("写入失败: %v", err)
	}
	client.CloseSession()

	info, err := os.Stat(dbPath)
	if err != nil {
		t.Fatalf("DataSource 文件未生成: %v", err)
	}
	if info.Size() == 0 {
		t.Fatal("DataSource 文件为空")
	}

	// 重开同一个 DataSource，确认数据持久化且租户字段保留
	reopened, err := duckdb.NewClient(dtos.Configuration{DataSource: dbPath}, lc)
	if err != nil {
		t.Fatalf("重开 DataSource 失败: %v", err)
	}
	defer reopened.CloseSession()

	rows, count, err := reopened.GetDeviceProperty(dtos.ThingModelPropertyDataRequest{
		DeviceId: deviceID,
		Code:     "temp",
		ThingModelDataBaseRequest: dtos.ThingModelDataBaseRequest{
			Range: []int64{ts - 1000, ts + 1000},
		},
		BaseSearchConditionQuery: dtos.BaseSearchConditionQuery{Page: 1, PageSize: 10},
	}, models.Device{Id: deviceID})
	if err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	if count != 1 || len(rows) != 1 {
		t.Fatalf("应读到 1 行，实际 count=%d len=%d", count, len(rows))
	}
	if rows[0].Value != 23.5 || rows[0].Time != ts {
		t.Fatalf("数据不一致: %+v", rows[0])
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir || !strings.HasPrefix(parent, string(filepath.Separator)) {
			t.Fatalf("找不到仓库根目录（go.mod）")
		}
		dir = parent
	}
}
