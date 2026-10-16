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
	"iotserver/internal/pkg/constants"
	"iotserver/internal/pkg/logger"
	"iotserver/internal/tools/datadb/duckdb"
)

// newDuckDBClientForTest 建临时 DuckDB 库
func newDuckDBClientForTest(t *testing.T) interfaces.DataDBClient {
	t.Helper()
	dir := t.TempDir()
	cfg := dtos.Configuration{DataSource: filepath.Join(dir, "core-data.duckdb")}
	lc := logger.NewClient("persist-duckdb-test", "error", "")
	client, err := duckdb.NewClient(cfg, lc)
	if err != nil {
		t.Fatalf("duckdb 不可用: %v", err)
	}
	t.Cleanup(func() { client.CloseSession() })
	return client
}

// TestDuckDBDispatchAndBatchTimestamps 验证两件事：
//  1. SaveDeviceThingModelData 能把上行消息正确落库（数据层已收敛为 DuckDB，无后端分发）；
//  2. 断点续传的每个测点落到自己的原始采集时刻。
func TestDuckDBDispatchAndBatchTimestamps(t *testing.T) {
	dl := newDuckDBClientForTest(t)
	if got := dl.GetDataDBType(); got != constants.DuckDB {
		t.Fatalf("GetDataDBType = %s, want %s", got, constants.DuckDB)
	}
	app := &persistApp{
		dataDbClient: dl,
		lc:           logger.NewClient("persist-duckdb-test", "error", ""),
	}

	deviceID := "dev-duckdb-backfill"
	base := time.Now().Add(-24 * time.Hour).UnixMilli()

	report := dtos.DeviceBatchReport{MsgId: "backfill-duckdb-1", Time: base}
	report.Data.Properties = map[string]dtos.BatchProperty{
		"temp": {Value: 23.5, Time: base + 60_000}, // 带自己的时间
		"rpm":  {Value: 1480.0},                    // 回落到批次时间
	}
	body, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	msg := dtos.ThingModelMessage{
		Cid:    deviceID,
		OpType: int32(thingmodel.OperationType_DATA_BATCH_REPORT),
		Data:   string(body),
	}
	if err := app.SaveDeviceThingModelData(context.Background(), msg); err != nil {
		t.Fatalf("批量补传落库失败: %v", err)
	}

	device := models.Device{Id: deviceID}
	rangeReq := func(code string) dtos.ThingModelPropertyDataRequest {
		return dtos.ThingModelPropertyDataRequest{
			DeviceId: deviceID,
			Code:     code,
			ThingModelDataBaseRequest: dtos.ThingModelDataBaseRequest{
				Range: []int64{base - 3_600_000, base + 3_600_000},
			},
			BaseSearchConditionQuery: dtos.BaseSearchConditionQuery{Page: 1, PageSize: 10},
		}
	}

	tempRows, count, err := dl.GetDeviceProperty(rangeReq("temp"), device)
	if err != nil {
		t.Fatalf("查询 temp: %v", err)
	}
	if count != 1 || len(tempRows) != 1 {
		t.Fatalf("temp 应有 1 条，实际 count=%d len=%d", count, len(tempRows))
	}
	if tempRows[0].Time != base+60_000 {
		t.Fatalf("temp 时间被改写：期望 %d，实际 %d", base+60_000, tempRows[0].Time)
	}

	rpmRows, _, err := dl.GetDeviceProperty(rangeReq("rpm"), device)
	if err != nil {
		t.Fatalf("查询 rpm: %v", err)
	}
	if len(rpmRows) != 1 {
		t.Fatalf("rpm 应有 1 条，实际 %d", len(rpmRows))
	}
	if rpmRows[0].Time != base {
		t.Fatalf("rpm 应回落到批次时间 %d，实际 %d", base, rpmRows[0].Time)
	}
}

// TestDuckDBPropertyReportDispatch 属性单点上报也应正确分发并落库
func TestDuckDBPropertyReportDispatch(t *testing.T) {
	dl := newDuckDBClientForTest(t)
	app := &persistApp{
		dataDbClient: dl,
		lc:           logger.NewClient("persist-duckdb-test", "error", ""),
	}

	deviceID := "dev-duckdb-prop"
	now := time.Now().UnixMilli()
	// DevicePropertyReport 的结构是 {msgId, data: {code: {value, time}}}
	payload, _ := json.Marshal(map[string]interface{}{
		"msgId": "prop-duckdb-1",
		"data": map[string]dtos.ReportData{
			"temp": {Value: 26.5, Time: now},
		},
	})
	msg := dtos.ThingModelMessage{
		Cid:    deviceID,
		OpType: int32(thingmodel.OperationType_PROPERTY_REPORT),
		Data:   string(payload),
	}
	if err := app.SaveDeviceThingModelData(context.Background(), msg); err != nil {
		t.Fatalf("属性上报落库失败: %v", err)
	}

	rows, count, err := dl.GetDeviceProperty(dtos.ThingModelPropertyDataRequest{
		DeviceId: deviceID,
		Code:     "temp",
		ThingModelDataBaseRequest: dtos.ThingModelDataBaseRequest{
			Range: []int64{now - 60_000, now + 60_000},
		},
		BaseSearchConditionQuery: dtos.BaseSearchConditionQuery{Page: 1, PageSize: 10},
	}, models.Device{Id: deviceID})
	if err != nil {
		t.Fatalf("查询: %v", err)
	}
	if count != 1 || len(rows) != 1 {
		t.Fatalf("属性应有 1 条，实际 count=%d len=%d", count, len(rows))
	}
	if v, ok := rows[0].Value.(float64); !ok || v != 26.5 {
		t.Fatalf("属性值 = %v, want 26.5", rows[0].Value)
	}
}
