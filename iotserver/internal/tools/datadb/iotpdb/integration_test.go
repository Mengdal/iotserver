//go:build iotp_e2e

package iotpdb

import (
	"context"
	"os"
	"strconv"
	"testing"
	"time"

	"iotserver/internal/dtos"
	"iotserver/internal/models"
	"iotserver/internal/pkg/logger"
	"iotserver/internal/tools/iotp"
)

// 对真实 iotp 实例的端到端验证（默认不跑）：
//
//	IOTP_BASEURL=http://host:8080 IOTP_TOKEN=... IOTP_DB=iotserver_e2e \
//	  go test -tags iotp_e2e -run TestIntegration -v ./internal/tools/datadb/iotpdb/
func TestIntegrationWriteQuery(t *testing.T) {
	baseURL := os.Getenv("IOTP_BASEURL")
	token := os.Getenv("IOTP_TOKEN")
	db := os.Getenv("IOTP_DB")
	if baseURL == "" || token == "" || db == "" {
		t.Skip("未设置 IOTP_BASEURL / IOTP_TOKEN / IOTP_DB")
	}
	lc := logger.NewClient("iotp-e2e", "info", "")
	api := iotp.New(baseURL, token, db, 10*time.Second)
	device := testDevice()
	// 允许用环境变量指定独立的产品/设备，避免往目标库的真实产品表里写测试数据。
	if v := os.Getenv("IOTP_DEVICE"); v != "" {
		device.Id = v
	}
	if v := os.Getenv("IOTP_PRODUCT"); v != "" {
		device.ProductId = v
	}
	if v := os.Getenv("IOTP_TENANT"); v != "" {
		device.TenantField = models.TenantField{TenantId: v}
	}
	meta := &fakeMeta{devices: map[string]models.Device{device.Id: device}, all: []models.Device{device}}
	c := newClient(api, meta, lc, false)
	defer c.CloseSession()

	ts := time.Now().UnixNano() / int64(time.Millisecond)
	ctx := context.Background()
	if err := c.Insert(ctx, device.Id, map[string]interface{}{
		"temperature": dtos.ReportData{Value: 23, Time: ts},
		"switch":      dtos.ReportData{Value: true, Time: ts},
		"model":       dtos.ReportData{Value: "TH-1", Time: ts},
	}); err != nil {
		t.Fatalf("写入属性失败: %v", err)
	}
	if err := c.Insert(ctx, "event:"+device.Id, map[string]interface{}{
		"overheat": `{"eventCode":"overheat","eventTime":` + itoa(ts) + `,"outputParams":{"v":1}}`,
	}); err != nil {
		t.Fatalf("写入事件失败: %v", err)
	}
	if err := c.flush(); err != nil {
		t.Fatalf("flush 失败: %v", err)
	}

	rows, count, err := c.GetDeviceProperty(dtos.ThingModelPropertyDataRequest{
		DeviceId:                  device.Id,
		Code:                      "temperature",
		ThingModelDataBaseRequest: dtos.ThingModelDataBaseRequest{Range: []int64{ts - 1000, ts + 1000}},
	}, device)
	if err != nil {
		t.Fatalf("查询属性失败: %v", err)
	}
	if count < 1 || len(rows) == 0 || rows[0].Value != 23.0 {
		t.Fatalf("属性数据不一致: count=%d rows=%+v", count, rows)
	}

	agg, total, err := c.GetDeviceProperty(dtos.ThingModelPropertyDataRequest{
		DeviceId:                  device.Id,
		Code:                      "temperature",
		ThingModelDataBaseRequest: dtos.ThingModelDataBaseRequest{Range: []int64{ts - 1000, ts + 1000}},
		AggInterval:               60,
		AggMethod:                 "avg",
	}, device)
	if err != nil {
		t.Fatalf("聚合查询失败: %v", err)
	}
	if total < 1 || len(agg) == 0 || agg[0].Value != 23.0 {
		t.Fatalf("聚合结果不一致: total=%d rows=%+v", total, agg)
	}

	events, eventTotal, err := c.GetDeviceEvent(dtos.ThingModelEventDataRequest{
		DeviceId:                  device.Id,
		ThingModelDataBaseRequest: dtos.ThingModelDataBaseRequest{Range: []int64{ts - 1000, ts + 1000}},
	}, device, models.Product{})
	if err != nil {
		t.Fatalf("查询事件失败: %v", err)
	}
	if eventTotal < 1 || len(events) == 0 || events[0].EventCode != "overheat" {
		t.Fatalf("事件数据不一致: total=%d rows=%+v", eventTotal, events)
	}

	rank, err := c.CountMsgGroupByDevice(device.TenantId, ts-1000, ts+1000)
	if err != nil {
		t.Fatalf("设备排行失败: %v", err)
	}
	if rank[device.Id] < 1 {
		t.Fatalf("设备排行缺少写入设备: %v", rank)
	}
}

func itoa(v int64) string {
	return strconv.FormatInt(v, 10)
}
