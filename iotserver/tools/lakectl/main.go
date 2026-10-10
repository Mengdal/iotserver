// lakectl DuckLake 运维工具：查看湖状态、强制物化、按保留期清理。
//
// 用法（在 iotserver 目录下，需先停掉服务，catalog 为单写者）：
//
//	go run ./tools/lakectl -catalog manifest/docker/db-data/ducklake/catalog.db \
//	                       -data    manifest/docker/db-data/ducklake/data stats
//	go run ./tools/lakectl ... flush            # 内联数据物化成 Parquet
//	go run ./tools/lakectl ... purge -days 30   # 删除 30 天前的数据并回收快照/文件
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"iotserver/internal/dtos"
	"iotserver/internal/pkg/logger"
	"iotserver/internal/tools/datadb/ducklake"
)

func main() {
	catalog := flag.String("catalog", "manifest/docker/db-data/ducklake/catalog.db", "catalog（元数据）路径")
	data := flag.String("data", "manifest/docker/db-data/ducklake/data", "Parquet 数据目录")
	days := flag.Int("days", 30, "purge 子命令：保留天数")
	device := flag.String("device", "", "drop 子命令：要删除历史数据的设备 id")
	flag.Parse()

	// 兼容两种写法：lakectl -days 30 purge / lakectl purge -days 30
	args := flag.Args()
	if len(args) == 0 {
		fmt.Println("用法: lakectl [-catalog ...] [-data ...] <stats|flush|purge|compact|drop> [-days N] [-device id]")
		os.Exit(2)
	}
	sub := args[0]
	if len(args) > 1 {
		// 子命令之后再出现的参数再解析一次
		_ = flag.CommandLine.Parse(args[1:])
	}

	lc := logger.NewClient("lakectl", "warn", "")
	cli, err := ducklake.NewClient(dtos.Configuration{
		DataSource: *catalog,
		Dsn:        *data,
		// 工具里不启动后台维护，避免与显式命令叠加
		RetentionDays: -1,
	}, lc)
	if err != nil {
		fmt.Println("打开湖失败:", err)
		os.Exit(1)
	}
	c := cli.(*ducklake.Client)

	switch sub {
	case "stats":
		stats(c)
	case "flush":
		if err := c.FlushInlinedData(context.Background()); err != nil {
			fmt.Println("物化失败:", err)
			os.Exit(1)
		}
		files, _ := c.DataFileCount()
		fmt.Printf("已物化为 Parquet，当前数据文件数: %d\n", files)
	case "purge":
		cutoff := time.Now().AddDate(0, 0, -*days).UnixNano() / int64(time.Millisecond)
		deleted, err := c.PurgeBefore(context.Background(), cutoff)
		if err != nil {
			fmt.Println("清理失败:", err)
			os.Exit(1)
		}
		fmt.Printf("已删除 %d 行（保留最近 %d 天），并回收过期快照与无引用文件\n", deleted, *days)
	case "compact":
		if err := c.Compact(context.Background(), *days); err != nil {
			fmt.Println("压缩失败:", err)
			os.Exit(1)
		}
		files, _ := c.DataFileCount()
		fmt.Printf("已物化并回收（保留最近 %d 天的快照），当前数据文件数: %d\n", *days, files)
	case "drop":
		if *device == "" {
			fmt.Println("drop 需要 -device <id>")
			os.Exit(2)
		}
		if err := c.DropTable(context.Background(), "device_"+*device); err != nil {
			fmt.Println("删除失败:", err)
			os.Exit(1)
		}
		fmt.Printf("已删除设备 %s 的时序数据\n", *device)
	default:
		fmt.Println("未知子命令:", sub)
		os.Exit(2)
	}

	c.CloseSession()
}

func stats(c *ducklake.Client) {
	for _, t := range []string{"device_property", "device_event", "device_service"} {
		fmt.Printf("%-16s %s\n", t, c.TableStats(t))
	}
	snap, _ := c.SnapshotCount()
	files, _ := c.DataFileCount()
	fmt.Printf("%-16s %d\n", "snapshots", snap)
	fmt.Printf("%-16s %d\n", "parquet_files", files)
	fmt.Printf("%-16s %d 天（0=永久保留）\n", "retention", c.RetentionDays())
}
