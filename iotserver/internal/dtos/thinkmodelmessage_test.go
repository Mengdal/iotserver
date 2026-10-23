package dtos

import (
	"encoding/json"
	"testing"

	"github.com/LMgateways/iotserver-driver-proto/thingmodel"
)

// 消息总线负载必须带 tenantId，规则引擎才能按租户过滤共享 stream。
func TestTransformMessageBusIncludesTenant(t *testing.T) {
	msg := ThingModelMessage{
		Cid:      "device-1",
		TenantId: "tenant-1",
		OpType:   int32(thingmodel.OperationType_PROPERTY_REPORT),
		Data:     `{"data":{"temp":25}}`,
	}
	raw := msg.TransformMessageBus()
	if len(raw) == 0 {
		t.Fatal("empty message")
	}
	var out MessageBus
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.TenantId != "tenant-1" {
		t.Fatalf("tenantId = %q, want tenant-1", out.TenantId)
	}
	if out.DeviceId != "device-1" {
		t.Fatalf("deviceId = %q", out.DeviceId)
	}
}
