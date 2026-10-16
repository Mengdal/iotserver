package duckdb

import (
	"context"
	"fmt"
	"testing"
	"time"

	"iotserver/internal/dtos"
	"iotserver/internal/models"
)

// 本文件用真实规模数据回答「要不要给时序表建索引」这个悬而未决的问题。
//
// 背景：DuckDB 原生表按写入顺序落 row group，zone map 记录每组的列 min/max。
// 时序数据是**按时间递增追加**的，所以 ts_ms 的 zone map 天然狭窄，时间范围
// 查询能直接跳过大量 row group；而 device_id 在写入时是交错的（几十台设备并发上报），
// 单个 row group 的 device_id min/max 会横跨全部设备，设备维度的裁剪能力存疑。
//
// 结论判据：
//   - 若「窄时间范围」查询明显快于「宽时间范围」→ 时间裁剪有效；
//   - 若加设备过滤后耗时与不过滤接近 → 设备维度没有裁剪，扫描量由时间范围主导；
//   - 再对比 Optimize() 建索引后的耗时，决定是否值得默认开启。
//
// 用 -short 跳过（数据量较大，不适合常规回归）。

func TestPruningAtScale(t *testing.T) {
	if testing.Short() {
		t.Skip("数据量较大，-short 下跳过")
	}

	c := newTestClient(t)
	ctx := context.Background()

	const devices = 200
	const perDevice = 500 // 合计 10 万行
	const days = 30

	now := time.Now().UnixMilli()
	start := now - int64(days)*24*3600*1000
	step := int64(days) * 24 * 3600 * 1000 / perDevice

	// 按时间递增、设备交错写入——与真实上行（缓冲攒批）一致
	insertStart := time.Now()
	for i := 0; i < perDevice; i++ {
		ts := start + int64(i)*step
		for d := 0; d < devices; d++ {
			if err := c.Insert(ctx, fmt.Sprintf("dev-%03d", d), map[string]interface{}{
				"temp": dtos.ReportData{Value: float64(d%100) + 0.5, Time: ts},
			}); err != nil {
				t.Fatalf("insert: %v", err)
			}
		}
	}
	c.flushPending()
	insertDur := time.Since(insertStart)
	total := devices * perDevice
	t.Logf("写入 %d 行（%d 设备 × %d 点，跨 %d 天）耗时 %s", total, devices, perDevice, days, insertDur)

	query := func(deviceID, code string, from, to int64) time.Duration {
		req := dtos.ThingModelPropertyDataRequest{
			DeviceId: deviceID,
			Code:     code,
			ThingModelDataBaseRequest: dtos.ThingModelDataBaseRequest{
				Range: []int64{from, to},
			},
			BaseSearchConditionQuery: dtos.BaseSearchConditionQuery{Page: 1, PageSize: 2000},
		}
		dev := models.Device{Id: deviceID}
		// 预热一次
		if _, _, err := c.GetDeviceProperty(req, dev); err != nil {
			t.Fatalf("query: %v", err)
		}
		const reps = 5
		st := time.Now()
		for i := 0; i < reps; i++ {
			rows, _, err := c.GetDeviceProperty(req, dev)
			if err != nil {
				t.Fatalf("query: %v", err)
			}
			_ = rows
		}
		return time.Since(st) / reps
	}

	// 窄窗：1 小时；宽窗：整段 30 天
	narrowFrom := now - 3600_000
	narrowTo := now
	wideFrom := start
	wideTo := now

	measure := func(label string) {
		n := query("dev-007", "temp", narrowFrom, narrowTo)
		w := query("dev-007", "temp", wideFrom, wideTo)
		// 不加设备过滤（空 deviceId 会返回错误，这里用另一台设备做对照意义不大），
		// 改用「同一时间窗、不同设备」验证设备过滤是否带来额外成本
		n2 := query("dev-150", "temp", narrowFrom, narrowTo)
		t.Logf("%-14s 窄窗(1h)=%-12s 宽窗(30d)=%-12s 另一设备窄窗=%-12s",
			label, n, w, n2)
	}

	measure("无索引")

	idxStart := time.Now()
	if err := c.Optimize(ctx); err != nil {
		t.Fatalf("optimize: %v", err)
	}
	t.Logf("建索引耗时 %s", time.Since(idxStart))
	measure("已建索引")

	// 索引会拖慢写入：对比一次等量插入
	insertStart2 := time.Now()
	for i := 0; i < perDevice/10; i++ {
		ts := start + int64(i)*step
		for d := 0; d < devices; d++ {
			if err := c.Insert(ctx, fmt.Sprintf("dev-%03d", d), map[string]interface{}{
				"temp": dtos.ReportData{Value: 1.0, Time: ts},
			}); err != nil {
				t.Fatalf("insert: %v", err)
			}
		}
	}
	c.flushPending()
	t.Logf("建索引后再写 %d 行耗时 %s（可与首次写入按行数折算对比）", devices*perDevice/10, time.Since(insertStart2))
}
