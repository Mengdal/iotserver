package leveldb

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"

	"iotserver/internal/dtos"
	interfaces "iotserver/internal/iotserver/core/interface"
	"iotserver/internal/models"
	"iotserver/internal/pkg/constants"
	"iotserver/internal/pkg/logger"
)

// newTestClient 在临时目录建一个 LevelDB 数据层
func newTestClient(t *testing.T) interfaces.DataDBClient {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "leveldb")
	lc := logger.NewClient("leveldb-test", "error", "")
	client, err := NewClient(dtos.Configuration{DataSource: dir}, lc)
	if err != nil {
		t.Fatalf("new leveldb client: %v", err)
	}
	t.Cleanup(func() { client.CloseSession() })
	return client
}

// seedRows 按 LevelDB 的 key 规范写入属性历史（与 devseed 一致）
func seedRows(t *testing.T, client interfaces.DataDBClient, deviceId, code string, points map[int64]float64) {
	t.Helper()
	kvs := map[string]interface{}{}
	for ts, value := range points {
		data := dtos.ReportData{Value: value, Time: ts}
		b, err := data.Marshal()
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		key := fmt.Sprintf("%s-%s-%s-%d", deviceId, constants.Property, code, ts)
		kvs[key] = b
	}
	if err := client.Insert(context.Background(), "", kvs); err != nil {
		t.Fatalf("insert: %v", err)
	}
}

// TestAggregateDeviceProperty LevelDB 聚合：内存分桶 + 桶分页
func TestAggregateDeviceProperty(t *testing.T) {
	client := newTestClient(t)
	device := models.Device{Id: "dev-ldb-agg"}
	base := int64(1_700_000_000_000)
	base = base - base%60_000

	seedRows(t, client, device.Id, "temp", map[int64]float64{
		base + 1000:  10,
		base + 2000:  20,
		base + 3000:  30,
		base + 61000: 100,
		base + 62000: 200,
	})

	req := dtos.ThingModelPropertyDataRequest{
		DeviceId: device.Id,
		Code:     "temp",
		ThingModelDataBaseRequest: dtos.ThingModelDataBaseRequest{
			Range: []int64{base - 1000, base + 120_000},
		},
		BaseSearchConditionQuery: dtos.BaseSearchConditionQuery{Page: 1, PageSize: 10},
	}

	// 不聚合 → 5 条原始数据
	raw, rawTotal, err := client.GetDeviceProperty(req, device)
	if err != nil {
		t.Fatalf("raw query: %v", err)
	}
	if rawTotal != 5 || len(raw) != 5 {
		t.Fatalf("不聚合 total=%d len=%d, want 5/5", rawTotal, len(raw))
	}

	// 1 分钟分桶 + 平均数
	agg := req
	agg.AggInterval = 60
	agg.AggMethod = "avg"

	buckets, total, err := client.GetDeviceProperty(agg, device)
	if err != nil {
		t.Fatalf("agg query: %v", err)
	}
	if total != 2 || len(buckets) != 2 {
		t.Fatalf("聚合 total=%d len=%d, want 2/2", total, len(buckets))
	}
	// 降序：先 base+60000（均值 (100+200)/2=150），再 base（均值 (10+20+30)/3=20）
	if buckets[0].Time != base+60_000 {
		t.Fatalf("第一个桶 = %d, want %d", buckets[0].Time, base+60_000)
	}
	if v, _ := buckets[0].Value.(float64); v != 150 {
		t.Fatalf("桶 %d 均值 = %v, want 150", buckets[0].Time, buckets[0].Value)
	}
	if v, _ := buckets[1].Value.(float64); v != 20 {
		t.Fatalf("桶 %d 均值 = %v, want 20", buckets[1].Time, buckets[1].Value)
	}
	for _, item := range buckets {
		if item.Time%60_000 != 0 {
			t.Fatalf("桶起点未按分钟对齐: %d", item.Time)
		}
	}

	// count
	countReq := agg
	countReq.AggMethod = "count"
	counts, total, err := client.GetDeviceProperty(countReq, device)
	if err != nil {
		t.Fatalf("count query: %v", err)
	}
	if total != 2 || len(counts) != 2 {
		t.Fatalf("count total=%d len=%d, want 2/2", total, len(counts))
	}
	if v, _ := counts[1].Value.(float64); v != 3 {
		t.Fatalf("桶 %d count = %v, want 3", counts[1].Time, counts[1].Value)
	}

	// 分页作用于桶：每页 1 个桶，第二页是较早的那个桶
	paged := agg
	paged.Page = 2
	paged.PageSize = 1
	page2, total, err := client.GetDeviceProperty(paged, device)
	if err != nil {
		t.Fatalf("paged query: %v", err)
	}
	if total != 2 || len(page2) != 1 || page2[0].Time != base {
		t.Fatalf("分页 total=%d len=%d time=%d, want 2/1/%d", total, len(page2), page2[0].Time, base)
	}

	// 缺时间范围时聚合应报错（而不是猜一个范围）
	noRange := agg
	noRange.Range = nil
	if _, _, err := client.GetDeviceProperty(noRange, device); err == nil {
		t.Fatal("聚合但无时间范围应报错")
	}
}

// TestAggregateTextPropertyLevelDB 文本值在 LevelDB 里的首末值与计数
func TestAggregateTextPropertyLevelDB(t *testing.T) {
	client := newTestClient(t)
	device := models.Device{Id: "dev-ldb-text"}
	base := int64(1_700_000_000_000)
	base = base - base%60_000

	kvs := map[string]interface{}{}
	for ts, v := range map[int64]string{
		base + 5000: "auto",
		base + 1000: "off",
		base + 9000: "manual",
	} {
		data := dtos.ReportData{Value: v, Time: ts}
		b, _ := json.Marshal(data)
		key := fmt.Sprintf("%s-%s-mode-%d", device.Id, constants.Property, ts)
		kvs[key] = b
	}
	if err := client.Insert(context.Background(), "", kvs); err != nil {
		t.Fatalf("insert: %v", err)
	}

	req := dtos.ThingModelPropertyDataRequest{
		DeviceId: device.Id,
		Code:     "mode",
		ThingModelDataBaseRequest: dtos.ThingModelDataBaseRequest{
			Range: []int64{base - 1000, base + 60_000},
		},
		BaseSearchConditionQuery: dtos.BaseSearchConditionQuery{Page: 1, PageSize: 10},
		AggInterval:              60,
	}

	first, _, err := client.GetDeviceProperty(withMethod(req, "first"), device)
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	if len(first) != 1 || first[0].Value != "off" {
		t.Fatalf("first = %+v, want off", first)
	}

	last, _, err := client.GetDeviceProperty(withMethod(req, "last"), device)
	if err != nil {
		t.Fatalf("last: %v", err)
	}
	if len(last) != 1 || last[0].Value != "manual" {
		t.Fatalf("last = %+v, want manual", last)
	}

	// 文本 + 数值方法 → 空结果
	avg, total, err := client.GetDeviceProperty(withMethod(req, "avg"), device)
	if err != nil {
		t.Fatalf("avg: %v", err)
	}
	if total != 0 || len(avg) != 0 {
		t.Fatalf("文本求平均应为空, total=%d len=%d", total, len(avg))
	}
}

func withMethod(req dtos.ThingModelPropertyDataRequest, method string) dtos.ThingModelPropertyDataRequest {
	req.AggMethod = method
	return req
}
