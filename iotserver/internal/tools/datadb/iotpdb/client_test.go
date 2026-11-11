package iotpdb

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"iotserver/internal/dtos"
	interfaces "iotserver/internal/iotserver/core/interface"
	"iotserver/internal/models"
	"iotserver/internal/tools/iotp"
)

/* ------------------------------ 测试替身 ------------------------------ */

type fakeMeta struct {
	interfaces.DBClient
	devices map[string]models.Device
	all     []models.Device
}

func (f *fakeMeta) WithCtx(context.Context) interfaces.DBClient { return f }

func (f *fakeMeta) DeviceById(id string) (models.Device, error) {
	device, ok := f.devices[id]
	if !ok {
		return models.Device{}, fmt.Errorf("device %s not found", id)
	}
	return device, nil
}

func (f *fakeMeta) DevicesSearch(int, int, dtos.DeviceSearchQueryRequest) ([]models.Device, uint32, error) {
	return f.all, uint32(len(f.all)), nil
}

type fakeIotp struct {
	t   *testing.T
	mu  sync.Mutex
	srv *httptest.Server

	writes      []string
	writeFn     func(body string, attempt int) string
	queries     []string
	queryFn     func(sql string) []map[string]any
	fields      map[string][]iotp.Field
	addedFields []string
	tables      []string
	devices     []map[string]any
}

func newFakeIotp(t *testing.T) *fakeIotp {
	f := &fakeIotp{t: t, fields: map[string][]iotp.Field{}}
	f.srv = httptest.NewServer(http.HandlerFunc(f.handle))
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeIotp) client() *iotp.Client {
	return iotp.New(f.srv.URL, "tok", "iotserver", 5*time.Second)
}

func (f *fakeIotp) handle(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()

	switch {
	case r.URL.Path == "/write":
		body, _ := io.ReadAll(r.Body)
		f.writes = append(f.writes, string(body))
		if f.writeFn != nil {
			_, _ = w.Write([]byte(f.writeFn(string(body), len(f.writes))))
			return
		}
		_, _ = w.Write([]byte(`{"accepted_rows":1,"rejected_rows":0,"tables":{}}`))
	case r.URL.Path == "/api/v1/query":
		var req struct {
			SQL string `json:"sql"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		f.queries = append(f.queries, req.SQL)
		rows := []map[string]any{}
		if f.queryFn != nil {
			rows = f.queryFn(req.SQL)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"rows": rows, "returned_rows": len(rows)})
	case r.URL.Path == "/api/v1/meta/tables" && r.Method == http.MethodGet:
		_ = json.NewEncoder(w).Encode(f.tables)
	case r.URL.Path == "/api/v1/meta/tables" && r.Method == http.MethodPost:
		_, _ = w.Write([]byte(`{}`))
	case strings.HasPrefix(r.URL.Path, "/api/v1/meta/tables/") && r.Method == http.MethodGet:
		parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v1/meta/tables/"), "/")
		table := parts[len(parts)-1]
		_ = json.NewEncoder(w).Encode(map[string]any{
			"db": "iotserver", "table": table, "fields": f.fields[table],
		})
	case strings.HasPrefix(r.URL.Path, "/api/v1/meta/tables/") && r.Method == http.MethodPut:
		var field iotp.Field
		_ = json.NewDecoder(r.Body).Decode(&field)
		f.addedFields = append(f.addedFields, field.Name)
		_, _ = w.Write([]byte(`{}`))
	case r.URL.Path == "/api/v1/meta/databases":
		_, _ = w.Write([]byte(`{}`))
	case r.URL.Path == "/api/v1/meta/devices":
		var payload map[string]any
		_ = json.NewDecoder(r.Body).Decode(&payload)
		f.devices = append(f.devices, payload)
		_, _ = w.Write([]byte(`{}`))
	default:
		http.NotFound(w, r)
	}
}

func (f *fakeIotp) writeBodies() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.writes...)
}

func testDevice() models.Device {
	return models.Device{
		Id:          "81283089",
		Name:        "温湿度仪",
		ProductId:   "67126069",
		DeviceSn:    "THmeter",
		DeviceKey:   "THmeter",
		TenantField: models.TenantField{TenantId: "69597392"},
		DeptId:      "dept-1",
	}
}

func baseFieldSet() []iotp.Field {
	return iotp.BaseFields()
}

func newTestClient(f *fakeIotp, device models.Device) *Client {
	meta := &fakeMeta{devices: map[string]models.Device{device.Id: device}, all: []models.Device{device}}
	return newClient(f.client(), meta, nil, false)
}

/* ------------------------------ 写入 ------------------------------ */

func TestInsertPropertyUsesProductTableAndFloatType(t *testing.T) {
	f := newFakeIotp(t)
	f.fields["67126069"] = append(baseFieldSet(), iotp.Field{Name: "temperature", Type: iotp.TypeF64})
	c := newTestClient(f, testDevice())

	err := c.Insert(context.Background(), "81283089", map[string]interface{}{
		"temperature": dtos.ReportData{Value: 23, Time: 1700000000000},
	})
	if err != nil {
		t.Fatalf("Insert: %v", err)
	}
	if err = c.flush(); err != nil {
		t.Fatalf("flush: %v", err)
	}

	bodies := f.writeBodies()
	if len(bodies) != 1 {
		t.Fatalf("写入次数 = %d, want 1: %v", len(bodies), bodies)
	}
	body := bodies[0]
	for _, want := range []string{
		`"measurement":"67126069"`,
		`"device_id":81283089`, // 必须是数字，字符串会被 iotp 整条丢弃
		`"temperature":23.0`,   // F64 列必须带小数点，否则 iotp 按整数拒绝
		`"timestamp":1700000000000`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("写入报文缺少 %s：%s", want, body)
		}
	}
	if len(f.devices) != 1 {
		t.Errorf("应注册 1 台设备实例，实际 %d", len(f.devices))
	}
}

func TestInsertWithoutProductUsesDefaultTable(t *testing.T) {
	f := newFakeIotp(t)
	device := testDevice()
	device.ProductId = ""
	c := newTestClient(f, device)

	if err := c.Insert(context.Background(), device.Id, map[string]interface{}{"speed": 12.5}); err != nil {
		t.Fatalf("Insert: %v", err)
	}
	if err := c.flush(); err != nil {
		t.Fatalf("flush: %v", err)
	}
	bodies := f.writeBodies()
	if len(bodies) != 1 || !strings.Contains(bodies[0], `"measurement":"default"`) {
		t.Fatalf("无产品设备应写 default: %v", bodies)
	}
}

func TestInsertEventUsesPrefixedFieldAndPayloadTime(t *testing.T) {
	f := newFakeIotp(t)
	c := newTestClient(f, testDevice())

	payload := `{"eventCode":"overheat","eventTime":1700000000}`
	if err := c.Insert(context.Background(), "event:81283089", map[string]interface{}{"overheat": payload}); err != nil {
		t.Fatalf("Insert: %v", err)
	}
	if err := c.flush(); err != nil {
		t.Fatalf("flush: %v", err)
	}
	bodies := f.writeBodies()
	if len(bodies) != 1 {
		t.Fatalf("写入次数 = %d", len(bodies))
	}
	if !strings.Contains(bodies[0], `"evt_overheat"`) {
		t.Errorf("事件字段应加 evt_ 前缀: %s", bodies[0])
	}
	if !strings.Contains(bodies[0], `"timestamp":1700000000000`) {
		t.Errorf("事件时间应从载荷提取并转毫秒: %s", bodies[0])
	}
}

func TestFlushRemediatesUnknownFieldAndRetries(t *testing.T) {
	f := newFakeIotp(t)
	f.fields["67126069"] = append(baseFieldSet(), iotp.Field{Name: "temperature", Type: iotp.TypeF64})
	f.writeFn = func(_ string, attempt int) string {
		if attempt == 1 {
			return `{"accepted_rows":0,"rejected_rows":1,"tables":{"iotserver.67126069":{"accepted":0,"rejected":1,"reasons":{"unknown_field":1}}}}`
		}
		return `{"accepted_rows":1,"rejected_rows":0,"tables":{}}`
	}
	c := newTestClient(f, testDevice())

	if err := c.Insert(context.Background(), "81283089", map[string]interface{}{"temperature": 21.5}); err != nil {
		t.Fatalf("Insert: %v", err)
	}
	if err := c.flush(); err != nil {
		t.Fatalf("flush: %v", err)
	}
	if len(f.writeBodies()) != 2 {
		t.Fatalf("被拒后应重试一次，写入次数 = %d", len(f.writeBodies()))
	}
}

func TestFlushRetriesOnlyRejectedTables(t *testing.T) {
	f := newFakeIotp(t)
	f.fields["67126069"] = append(baseFieldSet(), iotp.Field{Name: "temperature", Type: iotp.TypeF64})
	f.fields["999"] = append(baseFieldSet(), iotp.Field{Name: "temperature", Type: iotp.TypeF64})
	f.writeFn = func(_ string, attempt int) string {
		if attempt == 1 {
			return `{"accepted_rows":1,"rejected_rows":1,"tables":{
				"iotserver.67126069":{"accepted":0,"rejected":1,"reasons":{"unknown_field":1}},
				"iotserver.999":{"accepted":1,"rejected":0,"reasons":{}}}}`
		}
		return `{"accepted_rows":1,"rejected_rows":0,"tables":{}}`
	}
	deviceA := testDevice()
	deviceB := testDevice()
	deviceB.Id = "81283090"
	deviceB.ProductId = "999"

	meta := &fakeMeta{
		devices: map[string]models.Device{deviceA.Id: deviceA, deviceB.Id: deviceB},
		all:     []models.Device{deviceA, deviceB},
	}
	c := newClient(f.client(), meta, nil, false)

	for _, id := range []string{deviceA.Id, deviceB.Id} {
		if err := c.Insert(context.Background(), id, map[string]interface{}{"temperature": 20.5}); err != nil {
			t.Fatalf("Insert(%s): %v", id, err)
		}
	}
	if err := c.flush(); err != nil {
		t.Fatalf("flush: %v", err)
	}
	bodies := f.writeBodies()
	if len(bodies) != 2 {
		t.Fatalf("写入次数 = %d, want 2", len(bodies))
	}
	// 重试只能包含被拒的表，否则已成功的行会被重复写入。
	retry := bodies[1]
	if !strings.Contains(retry, `"measurement":"67126069"`) {
		t.Errorf("重试报文缺少被拒表: %s", retry)
	}
	if strings.Contains(retry, `"measurement":"999"`) {
		t.Errorf("重试报文不应重复已成功的表: %s", retry)
	}
}

/* ------------------------------ 读取 ------------------------------ */

func TestGetDevicePropertyHistoryAndLatest(t *testing.T) {
	f := newFakeIotp(t)
	f.fields["67126069"] = append(baseFieldSet(), iotp.Field{Name: "temperature", Type: iotp.TypeF64})
	f.queryFn = func(sql string) []map[string]any {
		if strings.Contains(sql, "count(") {
			return []map[string]any{{"c": float64(2)}}
		}
		return []map[string]any{{"time": float64(1700000000000), "v": float64(23.5)}}
	}
	c := newTestClient(f, testDevice())

	rows, total, err := c.GetDeviceProperty(dtos.ThingModelPropertyDataRequest{
		DeviceId:                  "81283089",
		Code:                      "temperature",
		ThingModelDataBaseRequest: dtos.ThingModelDataBaseRequest{Range: []int64{1600000000000, 1800000000000}},
	}, testDevice())
	if err != nil {
		t.Fatalf("GetDeviceProperty: %v", err)
	}
	if total != 2 || len(rows) != 1 {
		t.Fatalf("rows=%d total=%d", len(rows), total)
	}
	if rows[0].Time != 1700000000000 || rows[0].Value != 23.5 {
		t.Errorf("行数据 = %+v", rows[0])
	}
	if !strings.Contains(f.queries[1], "ORDER BY time DESC") {
		t.Errorf("历史查询 SQL 未按时间倒序: %s", f.queries[1])
	}

	// Last=true 表示最近 30 分钟窗口
	_, _, err = c.GetDeviceProperty(dtos.ThingModelPropertyDataRequest{
		DeviceId:                  "81283089",
		Code:                      "temperature",
		ThingModelDataBaseRequest: dtos.ThingModelDataBaseRequest{Last: true},
	}, testDevice())
	if err != nil {
		t.Fatalf("latest: %v", err)
	}
	lastSQL := f.queries[len(f.queries)-1]
	if !strings.Contains(lastSQL, "ORDER BY time DESC") {
		t.Errorf("最新值 SQL 异常: %s", lastSQL)
	}
}

func TestAggregateProperty(t *testing.T) {
	f := newFakeIotp(t)
	f.fields["67126069"] = append(baseFieldSet(), iotp.Field{Name: "temperature", Type: iotp.TypeF64})
	f.queryFn = func(sql string) []map[string]any {
		// 聚合走客户端分桶：数据层只返回原始行（iotp v1 查询白名单不支持分桶表达式）。
		return []map[string]any{{"time": float64(1700000000000), "v": float64(23.5)}}
	}
	c := newTestClient(f, testDevice())

	rows, total, err := c.GetDeviceProperty(dtos.ThingModelPropertyDataRequest{
		DeviceId:                  "81283089",
		Code:                      "temperature",
		ThingModelDataBaseRequest: dtos.ThingModelDataBaseRequest{Range: []int64{1600000000000, 1800000000000}},
		AggInterval:               60,
		AggMethod:                 "avg",
	}, testDevice())
	if err != nil {
		t.Fatalf("aggregate: %v", err)
	}
	if total != 1 || len(rows) != 1 || rows[0].Value != 23.5 {
		t.Fatalf("聚合结果 = %+v total=%d", rows, total)
	}
	if !strings.Contains(f.queries[0], "ORDER BY time DESC") || strings.Contains(f.queries[0], "GROUP BY") {
		t.Errorf("聚合应取原始行后在客户端分桶: %s", f.queries[0])
	}
}

func TestCountsAndDeviceRank(t *testing.T) {
	f := newFakeIotp(t)
	f.tables = []string{"67126069"}
	f.queryFn = func(sql string) []map[string]any {
		switch {
		case strings.Contains(sql, "GROUP BY device_id"):
			return []map[string]any{{"device_id": float64(81283089), "c": float64(3)}}
		default:
			return []map[string]any{{"c": float64(5)}}
		}
	}
	device := testDevice()
	c := newTestClient(f, device)

	total, err := c.CountAllMsgByTimeRange("69597392", 0, 1800000000000)
	if err != nil || total != 5 {
		t.Fatalf("CountAllMsgByTimeRange = %d, %v", total, err)
	}
	if !strings.Contains(f.queries[0], "tenant_id") {
		t.Errorf("租户过滤缺失: %s", f.queries[0])
	}

	rank, err := c.CountMsgGroupByDevice("69597392", 0, 1800000000000)
	if err != nil {
		t.Fatalf("CountMsgGroupByDevice: %v", err)
	}
	if rank[device.Id] != 3 {
		t.Errorf("排行 = %+v, want %s=3", rank, device.Id)
	}
}

func TestEventQueryUsesEvtColumns(t *testing.T) {
	f := newFakeIotp(t)
	f.fields["67126069"] = append(baseFieldSet(),
		iotp.Field{Name: "evt_overheat", Type: iotp.TypeString})
	f.queryFn = func(sql string) []map[string]any {
		if strings.Contains(sql, "count(") {
			return []map[string]any{{"c": float64(1)}}
		}
		return []map[string]any{{
			"time":         float64(1700000000000),
			"evt_overheat": `{"eventCode":"overheat","eventTime":1700000000000,"outputParams":{"v":1}}`,
		}}
	}
	c := newTestClient(f, testDevice())

	events, total, err := c.GetDeviceEvent(dtos.ThingModelEventDataRequest{
		DeviceId:                  "81283089",
		ThingModelDataBaseRequest: dtos.ThingModelDataBaseRequest{Range: []int64{1600000000000, 1800000000000}},
	}, testDevice(), models.Product{})
	if err != nil {
		t.Fatalf("GetDeviceEvent: %v", err)
	}
	if total != 1 || len(events) != 1 {
		t.Fatalf("events=%d total=%d", len(events), total)
	}
	if events[0].EventCode != "overheat" || events[0].EventTime != 1700000000000 {
		t.Errorf("事件 = %+v", events[0])
	}
}
