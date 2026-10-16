package persistence

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/LMgateways/iotserver-driver-proto/thingmodel"

	"iotserver/internal/dtos"
	interfaces "iotserver/internal/iotserver/core/interface"
	"iotserver/internal/models"
	"iotserver/internal/pkg/logger"
	"iotserver/internal/tools/datadb/ducklake"
)

// newDuckLakeClientForTest 建临时湖；扩展不可用时跳过（离线环境需预置 ducklake 扩展）
func newDuckLakeClientForTest(t *testing.T) interfaces.DataDBClient {
	t.Helper()
	dir := t.TempDir()
	cfg := dtos.Configuration{
		DataSource: filepath.Join(dir, "catalog.db"),
		Dsn:        filepath.Join(dir, "data"),
	}
	lc := logger.NewClient("persist-batch-test", "error", "")
	client, err := ducklake.NewClient(cfg, lc)
	if err != nil {
		t.Skipf("ducklake 不可用（需要 ducklake 扩展）: %v", err)
	}
	t.Cleanup(func() { client.CloseSession() })
	return client
}

// TestBatchBackfillKeepsOriginalTimestamps 断点续传的核心断言：
// 网关补传的历史数据必须落到**原始采集时刻**，而不是平台接收时刻。
//
// 覆盖三件事：
//  1. DuckLake 分支支持 DATA_BATCH_REPORT（此前缺失会静默丢数据）；
//  2. 每个测点带自己的时间戳（同一批可包含不同时刻）；
//  3. 未带时间的测点回落到批次时间。
func TestBatchBackfillKeepsOriginalTimestamps(t *testing.T) {
	dl := newDuckLakeClientForTest(t)
	app := &persistApp{
		dataDbClient: dl,
		lc:           logger.NewClient("persist-batch-test", "error", ""),
	}

	deviceId := "dev-batch-backfill"
	base := time.Now().Add(-24 * time.Hour).UnixMilli()

	report := dtos.DeviceBatchReport{
		MsgId: "backfill-1",
		Time:  base,
	}
	report.Data.Properties = map[string]dtos.BatchProperty{
		// 带自己的时间：比批次时间晚 60 秒
		"temp": {Value: 23.5, Time: base + 60_000},
		// 不带时间：应回落到批次时间 base
		"rpm": {Value: 1480.0},
	}
	body, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("marshal batch report: %v", err)
	}

	msg := dtos.ThingModelMessage{
		Cid:    deviceId,
		OpType: int32(thingmodel.OperationType_DATA_BATCH_REPORT),
		Data:   string(body),
	}
	if err := app.saveDeviceThingModelData(context.Background(), msg); err != nil {
		t.Fatalf("批量补传落库失败: %v", err)
	}

	device := models.Device{Id: deviceId}
	rangeReq := func(code string) dtos.ThingModelPropertyDataRequest {
		return dtos.ThingModelPropertyDataRequest{
			DeviceId: deviceId,
			Code:     code,
			ThingModelDataBaseRequest: dtos.ThingModelDataBaseRequest{
				Range: []int64{base - 3_600_000, base + 3_600_000},
			},
			BaseSearchConditionQuery: dtos.BaseSearchConditionQuery{Page: 1, PageSize: 10},
		}
	}

	// temp：应保留测点自带的时间
	tempRows, count, err := dl.GetDeviceProperty(rangeReq("temp"), device)
	if err != nil {
		t.Fatalf("查询 temp 失败: %v", err)
	}
	if count != 1 || len(tempRows) != 1 {
		t.Fatalf("temp 应有 1 条，实际 count=%d len=%d", count, len(tempRows))
	}
	if tempRows[0].Time != base+60_000 {
		t.Fatalf("temp 时间被改写：期望 %d，实际 %d", base+60_000, tempRows[0].Time)
	}

	// rpm：未带时间 → 回落批次时间
	rpmRows, _, err := dl.GetDeviceProperty(rangeReq("rpm"), device)
	if err != nil {
		t.Fatalf("查询 rpm 失败: %v", err)
	}
	if len(rpmRows) != 1 {
		t.Fatalf("rpm 应有 1 条，实际 %d", len(rpmRows))
	}
	if rpmRows[0].Time != base {
		t.Fatalf("rpm 应回落到批次时间 %d，实际 %d", base, rpmRows[0].Time)
	}
}

// TestBatchBackfillEventsAndIdempotentKey 批量补传事件 + 同一时间点重复写入的行为说明
func TestBatchBackfillEvents(t *testing.T) {
	dl := newDuckLakeClientForTest(t)
	app := &persistApp{
		dataDbClient: dl,
		lc:           logger.NewClient("persist-batch-test", "error", ""),
	}
	deviceId := "dev-batch-event"
	base := time.Now().Add(-2 * time.Hour).UnixMilli()

	report := dtos.DeviceBatchReport{MsgId: "evt-1", Time: base}
	report.Data.Events = map[string]dtos.BatchEvent{
		"alarm": {EventTime: base - 30_000, OutputParams: map[string]interface{}{"code": "E01"}},
	}
	body, _ := json.Marshal(report)
	msg := dtos.ThingModelMessage{
		Cid:    deviceId,
		OpType: int32(thingmodel.OperationType_DATA_BATCH_REPORT),
		Data:   string(body),
	}
	if err := app.saveDeviceThingModelData(context.Background(), msg); err != nil {
		t.Fatalf("事件补传落库失败: %v", err)
	}

	rows, count, err := dl.GetDeviceEvent(dtos.ThingModelEventDataRequest{
		DeviceId:  deviceId,
		EventCode: "alarm",
		ThingModelDataBaseRequest: dtos.ThingModelDataBaseRequest{
			Range: []int64{base - 3_600_000, base + 3_600_000},
		},
		BaseSearchConditionQuery: dtos.BaseSearchConditionQuery{Page: 1, PageSize: 10},
	}, models.Device{Id: deviceId}, models.Product{})
	if err != nil {
		t.Fatalf("查询事件失败: %v", err)
	}
	if count != 1 || len(rows) != 1 {
		t.Fatalf("事件应有 1 条，实际 count=%d len=%d", count, len(rows))
	}
	_ = context.Background()
}
