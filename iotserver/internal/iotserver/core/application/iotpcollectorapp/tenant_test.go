package iotpcollectorapp

import (
	"testing"

	"iotserver/internal/models"
)

// 平台设备 ID 可能带前导零，采集配置里的 iotp 数字键不带；查绑定必须归一化。
func TestResolveDeviceTenantNormalizesDeviceID(t *testing.T) {
	bindings := map[string]gatewayBinding{
		"1234567": {GatewayId: "gw-1", TenantId: "binding-tenant"},
	}

	got := resolveDeviceTenant(models.Device{Id: "01234567"}, bindings, nil)
	if got != "binding-tenant" {
		t.Fatalf("tenant = %q, want binding-tenant", got)
	}

	got = resolveDeviceTenant(models.Device{Id: "01234567"}, bindings, map[string]string{"gw-1": "gateway-tenant"})
	if got != "gateway-tenant" {
		t.Fatalf("tenant = %q, want gateway-tenant", got)
	}

	device := models.Device{Id: "01234567"}
	device.TenantId = "device-tenant"
	got = resolveDeviceTenant(device, bindings, nil)
	if got != "device-tenant" {
		t.Fatalf("tenant = %q, want device-tenant", got)
	}
}
