package dtos

import (
	"encoding/json"
	"testing"

	"github.com/LMgateways/iotserver-driver-proto/thingmodel"
)

// 驱动/网关的属性报文是 data.<code>={value,time}；总线必须压平成和 iotp 采集链路
// 一致的 data.<code>=value，否则同一张 mqtt_stream 无法同时服务两种来源。
func TestTransformMessageBusFlattensPropertyReport(t *testing.T) {
	msg := ThingModelMessage{
		Cid:    "device-1",
		OpType: int32(thingmodel.OperationType_PROPERTY_REPORT),
		Data:   `{"data":{"temp":{"value":35.5,"time":1795000000000},"switch":{"value":true,"time":1795000000001}}}`,
	}
	var bus MessageBus
	if err := json.Unmarshal(msg.TransformMessageBus(), &bus); err != nil {
		t.Fatalf("decode bus: %v", err)
	}
	data, ok := bus.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("bus data is not an object: %T", bus.Data)
	}
	if got := data["temp"]; got != 35.5 {
		t.Fatalf("temp = %v, want 35.5", got)
	}
	if got := data["switch"]; got != true {
		t.Fatalf("switch = %v, want true", got)
	}
	if bus.ReportTime != 1795000000001 {
		t.Fatalf("reportTime = %d", bus.ReportTime)
	}
}

// 事件/服务报文不是属性结构，保持原样，不能误压平 eventCode/eventTime。
func TestTransformMessageBusKeepsEventShape(t *testing.T) {
	msg := ThingModelMessage{
		Cid:    "device-1",
		OpType: int32(thingmodel.OperationType_EVENT_REPORT),
		Data:   `{"data":{"eventCode":"overheat","eventTime":1795000000000}}`,
	}
	var bus MessageBus
	if err := json.Unmarshal(msg.TransformMessageBus(), &bus); err != nil {
		t.Fatalf("decode bus: %v", err)
	}
	data, ok := bus.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("bus data is not an object: %T", bus.Data)
	}
	if data["eventCode"] != "overheat" {
		t.Fatalf("event data was flattened: %v", data)
	}
}
