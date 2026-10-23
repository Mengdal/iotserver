// lakemigrate 把旧 DuckLake 数据一次性迁移到 DuckDB 单文件。
//
// 用法（必须先停掉后端，避免 catalog / 数据目录被占用）：
//
//	go run ./tools/lakemigrate \
//	  -catalog manifest/docker/db-data/ducklake/catalog.db \
//	  -data    manifest/docker/db-data/ducklake/data \
//	  -target  manifest/docker/db-data/duckdb/core-data.duckdb.new
//
// 迁移完成后先校验行数，再把目标文件替换为正式库文件。
package main

import (
	"database/sql"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "github.com/duckdb/duckdb-go/v2"
)

func main() {
	catalog := flag.String("catalog", "manifest/docker/db-data/ducklake/catalog.db", "DuckLake catalog 路径")
	dataPath := flag.String("data", "manifest/docker/db-data/ducklake/data", "DuckLake Parquet 数据目录")
	target := flag.String("target", "", "目标 DuckDB 文件（必须不存在，避免覆盖已有数据）")
	flag.Parse()

	if *target == "" {
		fatalf("-target 必填")
	}
	if _, err := os.Stat(*target); err == nil {
		fatalf("目标文件已存在：%s\n请换一个路径，确认后再替换正式库", *target)
	} else if !os.IsNotExist(err) {
		fatalf("检查目标文件失败：%v", err)
	}

	db, err := sql.Open("duckdb", "")
	if err != nil {
		fatalf("打开 DuckDB 失败：%v", err)
	}
	defer db.Close()

	extDir := filepath.Join(filepath.Dir(*catalog), "extensions")
	mustExec(db, "SET extension_directory = "+quote(extDir))
	// 已离线预置扩展时 INSTALL 可能失败，继续尝试 LOAD
	_, _ = db.Exec("INSTALL ducklake")
	mustExec(db, "LOAD ducklake")

	attachLake := fmt.Sprintf("ATTACH %s AS lake (DATA_PATH %s, OVERRIDE_DATA_PATH true)",
		quote("ducklake:"+*catalog), quote(*dataPath))
	mustExec(db, attachLake)
	mustExec(db, "ATTACH "+quote(*target)+" AS target")

	tables := []struct {
		name    string
		columns string
	}{
		{"device_property", "tenant_id, device_id, code, ts_ms, num_value, text_value"},
		{"device_event", "tenant_id, device_id, event_code, ts_ms, payload"},
		{"device_service", "tenant_id, device_id, code, ts_ms, payload"},
	}
	// 兼容早期 lake 表：没有 tenant_id 列时先补列（与原 DuckLake 客户端行为一致）
	for _, t := range tables {
		mustExec(db, fmt.Sprintf("ALTER TABLE lake.%s ADD COLUMN IF NOT EXISTS tenant_id VARCHAR", t.name))
	}

	for _, t := range tables {
		var src, dst int64
		if err := db.QueryRow("SELECT count(*) FROM lake." + t.name).Scan(&src); err != nil {
			fatalf("统计源表 %s 失败：%v", t.name, err)
		}
		mustExec(db, fmt.Sprintf("CREATE TABLE target.%s AS SELECT %s FROM lake.%s",
			t.name, t.columns, t.name))
		if err := db.QueryRow("SELECT count(*) FROM target." + t.name).Scan(&dst); err != nil {
			fatalf("统计目标表 %s 失败：%v", t.name, err)
		}
		if src != dst {
			fatalf("表 %s 行数不一致：源 %d，目标 %d", t.name, src, dst)
		}
		fmt.Printf("✅ %-16s %d 行\n", t.name, dst)
	}

	mustExec(db, "CHECKPOINT target")
	fmt.Printf("\n迁移完成：%s\n校验通过后，用该文件替换正式 DuckDB 文件，并在配置中确认 Type = 'duckdb'。\n", *target)
}

func mustExec(db *sql.DB, stmt string) {
	if _, err := db.Exec(stmt); err != nil {
		fatalf("执行失败：%v\nSQL: %s", err, stmt)
	}
}

func quote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

func fatalf(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "❌ "+format+"\n", args...)
	os.Exit(1)
}
