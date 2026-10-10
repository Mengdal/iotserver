package ducklake

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"iotserver/internal/dtos"
	"iotserver/internal/models"
	"iotserver/internal/pkg/constants"
	"iotserver/internal/pkg/logger"
)

// newTestClient 在临时目录建湖；扩展不可用时跳过（离线环境需预置 ducklake 扩展）
func newTestClient(t *testing.T) *Client {
	t.Helper()
	dir := t.TempDir()
	cfg := dtos.Configuration{
		DataSource: filepath.Join(dir, "catalog.db"),
		Dsn:        filepath.Join(dir, "data"),
	}
	lc := logger.NewClient("ducklake-test", "error", "")
	client, err := NewClient(cfg, lc)
	if err != nil {
		t.Skipf("ducklake 不可用（需要 ducklake 扩展）: %v", err)
	}
	t.Cleanup(func() { client.CloseSession() })
	return client.(*Client)
}

func TestInsertAndQueryProperty(t *testing.T) {
	c := newTestClient(t)
	ctx := context.Background()
	device := models.Device{Id: "dev-1"}

	base := time.Now().Add(-2*time.Hour).UnixNano() / int64(time.Millisecond)
	// 写入 3 条温度、2 条湿度
	for i := 0; i < 3; i++ {
		ts := base + int64(i)*60_000
		data := map[string]interface{}{"temp": dtos.ReportData{Value: 20.0 + float64(i), Time: ts}}
		if err := c.Insert(ctx, device.Id, data); err != nil {
			t.Fatalf("insert temp: %v", err)
		}
	}
	for i := 0; i < 2; i++ {
		ts := base + int64(i)*60_000
		data := map[string]interface{}{"humidity": dtos.ReportData{Value: 55.0 + float64(i), Time: ts}}
		if err := c.Insert(ctx, device.Id, data); err != nil {
			t.Fatalf("insert humidity: %v", err)
		}
	}

	req := dtos.ThingModelPropertyDataRequest{
		DeviceId: device.Id,
		Code:     "temp",
		ThingModelDataBaseRequest: dtos.ThingModelDataBaseRequest{
			Range: []int64{base - 1000, base + 10*60_000},
		},
		BaseSearchConditionQuery: dtos.BaseSearchConditionQuery{Page: 1, PageSize: 10},
	}
	rows, count, err := c.GetDeviceProperty(req, device)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if count != 3 {
		t.Fatalf("count = %d, want 3", count)
	}
	if len(rows) != 3 {
		t.Fatalf("len(rows) = %d, want 3", len(rows))
	}
	// 时间倒序：最新在前
	if rows[0].Time <= rows[2].Time {
		t.Fatalf("rows not ordered desc: %+v", rows)
	}
	if v, ok := rows[0].Value.(float64); !ok || v != 22.0 {
		t.Fatalf("rows[0].Value = %v, want 22", rows[0].Value)
	}

	// 分页：第 2 页每页 2 条，应剩 1 条
	req.Page = 2
	req.PageSize = 2
	page2, count, err := c.GetDeviceProperty(req, device)
	if err != nil {
		t.Fatalf("query page2: %v", err)
	}
	if count != 3 || len(page2) != 1 {
		t.Fatalf("page2: count=%d len=%d, want 3/1", count, len(page2))
	}

	// 时间窗外的数据查不到
	req.Page = 1
	req.PageSize = 10
	req.Range = []int64{base + 100*60_000, base + 200*60_000}
	empty, count, err := c.GetDeviceProperty(req, device)
	if err != nil {
		t.Fatalf("query out of range: %v", err)
	}
	if count != 0 || len(empty) != 0 {
		t.Fatalf("out of range: count=%d len=%d, want 0/0", count, len(empty))
	}

	// 湿度独立存储，互不干扰
	req.Code = "humidity"
	req.Range = []int64{base - 1000, base + 10*60_000}
	hum, count, err := c.GetDeviceProperty(req, device)
	if err != nil {
		t.Fatalf("query humidity: %v", err)
	}
	if count != 2 || len(hum) != 2 {
		t.Fatalf("humidity: count=%d len=%d, want 2/2", count, len(hum))
	}
}

func TestInsertTextValueAndCount(t *testing.T) {
	c := newTestClient(t)
	ctx := context.Background()
	device := models.Device{Id: "dev-text"}
	ts := time.Now().UnixNano() / int64(time.Millisecond)

	// 非数值（字符串）走 text_value
	if err := c.Insert(ctx, device.Id, map[string]interface{}{"mode": dtos.ReportData{Value: "auto", Time: ts}}); err != nil {
		t.Fatalf("insert text: %v", err)
	}
	req := dtos.ThingModelPropertyDataRequest{
		DeviceId: device.Id,
		Code:     "mode",
		ThingModelDataBaseRequest: dtos.ThingModelDataBaseRequest{
			Range: []int64{ts - 1000, ts + 1000},
		},
		BaseSearchConditionQuery: dtos.BaseSearchConditionQuery{Page: 1, PageSize: 10},
	}
	rows, count, err := c.GetDeviceProperty(req, device)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if count != 1 || len(rows) != 1 {
		t.Fatalf("count=%d len=%d, want 1/1", count, len(rows))
	}
	if rows[0].Value != "auto" {
		t.Fatalf("value = %v, want auto", rows[0].Value)
	}

	n, err := c.GetDevicePropertyCount(req)
	if err != nil || n != 1 {
		t.Fatalf("GetDevicePropertyCount = %d, %v; want 1, nil", n, err)
	}
}

func TestEventAndService(t *testing.T) {
	c := newTestClient(t)
	ctx := context.Background()
	device := models.Device{Id: "dev-evt"}
	ts := time.Now().UnixNano() / int64(time.Millisecond)

	// 事件：payload 里带 eventTime，应被识别为事件时间
	eventPayload := `{"eventCode":"alarm","eventTime":` + itoa(ts) + `,"outputParams":{"level":"high"}}`
	if err := c.Insert(ctx, eventPrefix+device.Id, map[string]interface{}{"alarm": eventPayload}); err != nil {
		t.Fatalf("insert event: %v", err)
	}
	evtReq := dtos.ThingModelEventDataRequest{
		DeviceId:  device.Id,
		EventCode: "alarm",
		ThingModelDataBaseRequest: dtos.ThingModelDataBaseRequest{
			Range: []int64{ts - 1000, ts + 1000},
		},
		BaseSearchConditionQuery: dtos.BaseSearchConditionQuery{Page: 1, PageSize: 10},
	}
	events, count, err := c.GetDeviceEvent(evtReq, device, models.Product{})
	if err != nil {
		t.Fatalf("query event: %v", err)
	}
	if count != 1 || len(events) != 1 {
		t.Fatalf("event count=%d len=%d, want 1/1", count, len(events))
	}
	if events[0].EventCode != "alarm" || events[0].EventTime != ts {
		t.Fatalf("event = %+v, want code=alarm time=%d", events[0], ts)
	}
	if events[0].OutputParams["level"] != "high" {
		t.Fatalf("event outputParams = %+v", events[0].OutputParams)
	}

	// 服务
	svcPayload := `{"code":"reboot","time":` + itoa(ts) + `,"inputParams":{"delay":3},"outputParams":{"ok":true}}`
	if err := c.Insert(ctx, servicePrefix+device.Id, map[string]interface{}{"reboot": svcPayload}); err != nil {
		t.Fatalf("insert service: %v", err)
	}
	svcReq := dtos.ThingModelServiceDataRequest{
		DeviceId: device.Id,
		Code:     "reboot",
		ThingModelDataBaseRequest: dtos.ThingModelDataBaseRequest{
			Range: []int64{ts - 1000, ts + 1000},
		},
		BaseSearchConditionQuery: dtos.BaseSearchConditionQuery{Page: 1, PageSize: 10},
	}
	svcs, count, err := c.GetDeviceService(svcReq, device, models.Product{})
	if err != nil {
		t.Fatalf("query service: %v", err)
	}
	if count != 1 || len(svcs) != 1 {
		t.Fatalf("service count=%d len=%d, want 1/1", count, len(svcs))
	}
	if svcs[0].Code != "reboot" || svcs[0].Time != ts {
		t.Fatalf("service = %+v, want code=reboot time=%d", svcs[0], ts)
	}

	// 消息总数应聚合三张表
	total, err := c.GetDeviceMsgCountByGiveTime(device.Id, ts-1000, ts+1000)
	if err != nil {
		t.Fatalf("msg count: %v", err)
	}
	if total != 2 {
		t.Fatalf("msg count = %d, want 2", total)
	}
}

func TestDropTableByDevice(t *testing.T) {
	c := newTestClient(t)
	ctx := context.Background()
	device := models.Device{Id: "dev-drop"}
	ts := time.Now().UnixNano() / int64(time.Millisecond)
	if err := c.Insert(ctx, device.Id, map[string]interface{}{
		"temp": dtos.ReportData{Value: 9.0, Time: ts},
	}); err != nil {
		t.Fatalf("insert: %v", err)
	}
	if err := c.DropTable(ctx, constants.DB_PREFIX+device.Id); err != nil {
		t.Fatalf("drop: %v", err)
	}
	n, err := c.GetDevicePropertyCount(dtos.ThingModelPropertyDataRequest{
		DeviceId: device.Id,
		Code:     "temp",
		ThingModelDataBaseRequest: dtos.ThingModelDataBaseRequest{
			Range: []int64{ts - 1000, ts + 1000},
		},
	})
	if err != nil {
		t.Fatalf("count after drop: %v", err)
	}
	if n != 0 {
		t.Fatalf("count after drop = %d, want 0", n)
	}
}

func itoa(v int64) string {
	if v == 0 {
		return "0"
	}
	neg := v < 0
	if neg {
		v = -v
	}
	var buf [20]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

// TestFlushInlinedDataToParquet 验证内联数据可物化成 Parquet 数据文件
func TestFlushInlinedDataToParquet(t *testing.T) {
	c := newTestClient(t)
	ctx := context.Background()
	device := models.Device{Id: "dev-flush"}
	ts := time.Now().UnixNano() / int64(time.Millisecond)
	for i := 0; i < 5; i++ {
		if err := c.Insert(ctx, device.Id, map[string]interface{}{
			"temp": dtos.ReportData{Value: float64(i), Time: ts + int64(i)*1000},
		}); err != nil {
			t.Fatalf("insert: %v", err)
		}
	}
	c.flushPending() // 先落盘，才有内联数据可物化

	before, err := c.DataFileCount()
	if err != nil {
		t.Fatalf("DataFileCount: %v", err)
	}

	if err := c.FlushInlinedData(ctx); err != nil {
		t.Fatalf("FlushInlinedData: %v", err)
	}

	after, err := c.DataFileCount()
	if err != nil {
		t.Fatalf("DataFileCount after flush: %v", err)
	}
	if after <= before {
		t.Fatalf("Parquet 文件数未增加: before=%d after=%d", before, after)
	}

	// 物化后数据仍可查
	rows, count, err := c.GetDeviceProperty(dtos.ThingModelPropertyDataRequest{
		DeviceId: device.Id,
		Code:     "temp",
		ThingModelDataBaseRequest: dtos.ThingModelDataBaseRequest{
			Range: []int64{ts - 1000, ts + 10000},
		},
		BaseSearchConditionQuery: dtos.BaseSearchConditionQuery{Page: 1, PageSize: 10},
	}, device)
	if err != nil {
		t.Fatalf("query after flush: %v", err)
	}
	if count != 5 || len(rows) != 5 {
		t.Fatalf("flush 后 count=%d len=%d, want 5/5", count, len(rows))
	}
}

// snapshotCount 当前快照数
func snapshotCount(t *testing.T, c *Client) int {
	t.Helper()
	var n int
	if err := c.db.QueryRow("SELECT count(*) FROM lake.snapshots()").Scan(&n); err != nil {
		t.Fatalf("snapshots: %v", err)
	}
	return n
}

// TestBatchWriteReducesSnapshots 批量写入应显著减少快照数（每批一个事务）
func TestBatchWriteReducesSnapshots(t *testing.T) {
	c := newTestClient(t)
	ctx := context.Background()
	device := models.Device{Id: "dev-batch"}
	ts := time.Now().UnixNano() / int64(time.Millisecond)

	before := snapshotCount(t, c)
	const rows = 100
	for i := 0; i < rows; i++ {
		if err := c.Insert(ctx, device.Id, map[string]interface{}{
			"temp": dtos.ReportData{Value: float64(i), Time: ts + int64(i)},
		}); err != nil {
			t.Fatalf("insert: %v", err)
		}
	}
	c.flushPending()
	after := snapshotCount(t, c)

	if got := after - before; got > 3 {
		t.Fatalf("%d 行写入产生了 %d 个快照，批量写入未生效", rows, got)
	}
	total, _, err := c.GetDeviceProperty(dtos.ThingModelPropertyDataRequest{
		DeviceId: device.Id,
		Code:     "temp",
		ThingModelDataBaseRequest: dtos.ThingModelDataBaseRequest{
			Range: []int64{ts - 1000, ts + rows + 1000},
		},
		BaseSearchConditionQuery: dtos.BaseSearchConditionQuery{Page: 1, PageSize: 200},
	}, device)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(total) != rows {
		t.Fatalf("批量写入后有 %d 行，want %d", len(total), rows)
	}
}

// TestReadAfterWrite 写入后立刻查询应能读到（读前自动落盘）
func TestReadAfterWrite(t *testing.T) {
	c := newTestClient(t)
	ctx := context.Background()
	device := models.Device{Id: "dev-raw"}
	ts := time.Now().UnixNano() / int64(time.Millisecond)

	if err := c.Insert(ctx, device.Id, map[string]interface{}{
		"temp": dtos.ReportData{Value: 33.3, Time: ts},
	}); err != nil {
		t.Fatalf("insert: %v", err)
	}
	// 不再显式 flush，直接查询
	rows, count, err := c.GetDeviceProperty(dtos.ThingModelPropertyDataRequest{
		DeviceId: device.Id,
		Code:     "temp",
		ThingModelDataBaseRequest: dtos.ThingModelDataBaseRequest{
			Range: []int64{ts - 1000, ts + 1000},
		},
		BaseSearchConditionQuery: dtos.BaseSearchConditionQuery{Page: 1, PageSize: 10},
	}, device)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if count != 1 || len(rows) != 1 {
		t.Fatalf("写入后立刻查询 count=%d len=%d, want 1/1（读前落盘失效）", count, len(rows))
	}
}

// TestPurgeBeforeAndRetention 保留策略：删除超期数据，保留期内数据不受影响
func TestPurgeBeforeAndRetention(t *testing.T) {
	c := newTestClient(t)
	ctx := context.Background()
	device := models.Device{Id: "dev-purge"}
	now := time.Now().UnixNano() / int64(time.Millisecond)

	oldTs := now - 40*24*3600*1000 // 40 天前
	newTs := now - 1*24*3600*1000  // 1 天前
	if err := c.Insert(ctx, device.Id, map[string]interface{}{
		"temp": dtos.ReportData{Value: 1.0, Time: oldTs},
	}); err != nil {
		t.Fatalf("insert old: %v", err)
	}
	if err := c.Insert(ctx, device.Id, map[string]interface{}{
		"temp": dtos.ReportData{Value: 2.0, Time: newTs},
	}); err != nil {
		t.Fatalf("insert new: %v", err)
	}
	c.flushPending()

	cutoff := now - 30*24*3600*1000
	deleted, err := c.PurgeBefore(ctx, cutoff)
	if err != nil {
		t.Fatalf("purge: %v", err)
	}
	if deleted != 1 {
		t.Fatalf("purge 删除 %d 行, want 1", deleted)
	}

	rows, count, err := c.GetDeviceProperty(dtos.ThingModelPropertyDataRequest{
		DeviceId: device.Id,
		Code:     "temp",
		ThingModelDataBaseRequest: dtos.ThingModelDataBaseRequest{
			Range: []int64{now - 90*24*3600*1000, now + 1000},
		},
		BaseSearchConditionQuery: dtos.BaseSearchConditionQuery{Page: 1, PageSize: 10},
	}, device)
	if err != nil {
		t.Fatalf("query after purge: %v", err)
	}
	if count != 1 || len(rows) != 1 {
		t.Fatalf("purge 后 count=%d len=%d, want 1/1", count, len(rows))
	}
	if v, ok := rows[0].Value.(float64); !ok || v != 2.0 {
		t.Fatalf("保留的应是新数据, got %v", rows[0].Value)
	}
	if got := c.RetentionDays(); got != defaultRetentionDays {
		t.Fatalf("RetentionDays = %d, want %d（默认值）", got, defaultRetentionDays)
	}
}

// seedAggRows 写入指定时间点的数值/文本行，便于聚合断言
func seedAggRows(t *testing.T, c *Client, deviceId string, pairs ...interface{}) {
	t.Helper()
	ctx := context.Background()
	for i := 0; i+1 < len(pairs); i += 2 {
		ts := int64(pairs[i].(int))
		code := pairs[i+1].(string)
		var data map[string]interface{}
		if i+2 < len(pairs) && pairs[i+2] != nil {
			if f, ok := pairs[i+2].(float64); ok {
				data = map[string]interface{}{code: dtos.ReportData{Value: f, Time: ts}}
				i++
			} else {
				data = map[string]interface{}{code: dtos.ReportData{Value: pairs[i+2], Time: ts}}
				i++
			}
		} else {
			data = map[string]interface{}{code: dtos.ReportData{Value: 0.0, Time: ts}}
		}
		if err := c.Insert(ctx, deviceId, data); err != nil {
			t.Fatalf("insert agg row: %v", err)
		}
	}
}

// TestAggregateProperty 聚合查询：分桶、各聚合方法、桶内均值正确性
func TestAggregateProperty(t *testing.T) {
	c := newTestClient(t)
	ctx := context.Background()
	device := models.Device{Id: "dev-agg"}
	base := int64(1_700_000_000_000) // 毫秒，作为桶起点
	base = base - base%60_000        // 对齐到分钟

	// 桶 1（base）：10/20/30；桶 2（base+60000）：100/200
	type row struct {
		ts    int64
		value float64
	}
	rows := []row{
		{base + 1000, 10}, {base + 2000, 20}, {base + 3000, 30},
		{base + 61000, 100}, {base + 62000, 200},
	}
	for _, r := range rows {
		if err := c.Insert(ctx, device.Id, map[string]interface{}{
			"temp": dtos.ReportData{Value: r.value, Time: r.ts},
		}); err != nil {
			t.Fatalf("insert: %v", err)
		}
	}

	baseReq := dtos.ThingModelPropertyDataRequest{
		DeviceId: device.Id,
		Code:     "temp",
		ThingModelDataBaseRequest: dtos.ThingModelDataBaseRequest{
			Range: []int64{base - 1000, base + 120_000},
		},
		BaseSearchConditionQuery: dtos.BaseSearchConditionQuery{Page: 1, PageSize: 10},
	}

	// 不聚合 → 5 条原始数据
	raw, rawTotal, err := c.GetDeviceProperty(baseReq, device)
	if err != nil {
		t.Fatalf("raw query: %v", err)
	}
	if rawTotal != 5 || len(raw) != 5 {
		t.Fatalf("不聚合 total=%d len=%d, want 5/5", rawTotal, len(raw))
	}

	// 1 分钟分桶
	agg := baseReq
	agg.AggInterval = 60

	avg, total, err := c.GetDeviceProperty(withMethod(agg, "avg"), device)
	if err != nil {
		t.Fatalf("avg query: %v", err)
	}
	if total != 2 || len(avg) != 2 {
		t.Fatalf("avg total=%d len=%d, want 2/2", total, len(avg))
	}
	// 降序：第一个桶是较晚的 base+60000，均值 150
	if avg[0].Time != base+60_000 {
		t.Fatalf("第一个桶 = %d, want %d", avg[0].Time, base+60_000)
	}
	if v, _ := avg[0].Value.(float64); v != 150 {
		t.Fatalf("桶 %d 均值 = %v, want 150", avg[0].Time, avg[0].Value)
	}
	if v, _ := avg[1].Value.(float64); v != 20 {
		t.Fatalf("桶 %d 均值 = %v, want 20", avg[1].Time, avg[1].Value)
	}

	// count
	cnt, total, err := c.GetDeviceProperty(withMethod(agg, "count"), device)
	if err != nil {
		t.Fatalf("count query: %v", err)
	}
	if total != 2 || len(cnt) != 2 {
		t.Fatalf("count total=%d len=%d, want 2/2", total, len(cnt))
	}
	if v, _ := cnt[0].Value.(float64); v != 2 {
		t.Fatalf("桶 %d count = %v, want 2", cnt[0].Time, cnt[0].Value)
	}
	if v, _ := cnt[1].Value.(float64); v != 3 {
		t.Fatalf("桶 %d count = %v, want 3", cnt[1].Time, cnt[1].Value)
	}

	// sum / max / min
	sum, _, err := c.GetDeviceProperty(withMethod(agg, "sum"), device)
	if err != nil {
		t.Fatalf("sum query: %v", err)
	}
	if v, _ := sum[0].Value.(float64); v != 300 {
		t.Fatalf("桶 %d sum = %v, want 300", sum[0].Time, sum[0].Value)
	}
	maxv, _, err := c.GetDeviceProperty(withMethod(agg, "max"), device)
	if err != nil {
		t.Fatalf("max query: %v", err)
	}
	if v, _ := maxv[0].Value.(float64); v != 200 {
		t.Fatalf("桶 %d max = %v, want 200", maxv[0].Time, maxv[0].Value)
	}
	minv, _, err := c.GetDeviceProperty(withMethod(agg, "min"), device)
	if err != nil {
		t.Fatalf("min query: %v", err)
	}
	if v, _ := minv[1].Value.(float64); v != 10 {
		t.Fatalf("桶 %d min = %v, want 10", minv[1].Time, minv[1].Value)
	}

	// 分页作用于桶：每页 1 个桶
	page2 := withMethod(agg, "avg")
	page2.Page = 2
	page2.PageSize = 1
	paged, total, err := c.GetDeviceProperty(page2, device)
	if err != nil {
		t.Fatalf("paged query: %v", err)
	}
	if total != 2 || len(paged) != 1 {
		t.Fatalf("分页 total=%d len=%d, want 2/1", total, len(paged))
	}
	if paged[0].Time != base {
		t.Fatalf("第二页桶 = %d, want %d", paged[0].Time, base)
	}

	// 提前判断一下桶起点是否按 epoch 对齐（秒位为 0）
	for _, item := range avg {
		if item.Time%60_000 != 0 {
			t.Fatalf("桶起点未按分钟对齐: %d", item.Time)
		}
	}
}

// TestAggregateTextProperty 文本属性的计数与首末值
func TestAggregateTextProperty(t *testing.T) {
	c := newTestClient(t)
	ctx := context.Background()
	device := models.Device{Id: "dev-agg-text"}
	base := int64(1_700_000_000_000)
	base = base - base%60_000

	values := []struct {
		ts int64
		v  string
	}{
		{base + 5000, "auto"},
		{base + 1000, "off"},
		{base + 9000, "manual"},
		{base + 65_000, "idle"},
	}
	for _, item := range values {
		if err := c.Insert(ctx, device.Id, map[string]interface{}{
			"mode": dtos.ReportData{Value: item.v, Time: item.ts},
		}); err != nil {
			t.Fatalf("insert: %v", err)
		}
	}

	req := dtos.ThingModelPropertyDataRequest{
		DeviceId: device.Id,
		Code:     "mode",
		ThingModelDataBaseRequest: dtos.ThingModelDataBaseRequest{
			Range: []int64{base - 1000, base + 120_000},
		},
		BaseSearchConditionQuery: dtos.BaseSearchConditionQuery{Page: 1, PageSize: 10},
		AggInterval:              60,
	}

	cnt, _, err := c.GetDeviceProperty(withMethod(req, "count"), device)
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if len(cnt) != 2 {
		t.Fatalf("count 桶数 = %d, want 2", len(cnt))
	}

	first, _, err := c.GetDeviceProperty(withMethod(req, "first"), device)
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	var bucket0 interface{}
	for _, item := range first {
		if item.Time == base {
			bucket0 = item.Value
		}
	}
	if bucket0 != "off" {
		t.Fatalf("桶 %d first = %v, want off（时间最小那行）", base, bucket0)
	}

	last, _, err := c.GetDeviceProperty(withMethod(req, "last"), device)
	if err != nil {
		t.Fatalf("last: %v", err)
	}
	var lastBucket0 interface{}
	for _, item := range last {
		if item.Time == base {
			lastBucket0 = item.Value
		}
	}
	if lastBucket0 != "manual" {
		t.Fatalf("桶 %d last = %v, want manual（时间最大那行）", base, lastBucket0)
	}

	// 文本属性 + 数值方法：无数值行 → 结果为空（不返回 0）
	avg, total, err := c.GetDeviceProperty(withMethod(req, "avg"), device)
	if err != nil {
		t.Fatalf("avg on text: %v", err)
	}
	if total != 0 || len(avg) != 0 {
		t.Fatalf("文本属性求平均应返回空, total=%d len=%d", total, len(avg))
	}
}

func withMethod(req dtos.ThingModelPropertyDataRequest, method string) dtos.ThingModelPropertyDataRequest {
	req.AggMethod = method
	return req
}
