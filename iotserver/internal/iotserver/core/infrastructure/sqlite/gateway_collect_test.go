package sqlite

import (
	"testing"

	"iotserver/internal/models"
)

// 删除软网关时必须同时清理该网关的采集通道与设备绑定，
// 否则设备会永远卡在「已绑定到其他软网关」的校验上。
func TestDeleteGatewayWithCollectConfig(t *testing.T) {
	c := newTestClient(t)
	if err := c.MigrateSystemTables(); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	gw, err := c.AddGateway(models.Gateway{Name: "gw-a", Sn: "SN-A"})
	if err != nil {
		t.Fatalf("add gateway: %v", err)
	}
	other, err := c.AddGateway(models.Gateway{Name: "gw-b", Sn: "SN-B"})
	if err != nil {
		t.Fatalf("add gateway b: %v", err)
	}

	ch, err := c.SaveCollectChannel(models.CollectChannel{
		GatewayId: gw.Id, Name: "C1", ChannelType: "network", Enabled: true,
	})
	if err != nil {
		t.Fatalf("save channel: %v", err)
	}
	otherCh, err := c.SaveCollectChannel(models.CollectChannel{
		GatewayId: other.Id, Name: "C2", ChannelType: "network", Enabled: true,
	})
	if err != nil {
		t.Fatalf("save other channel: %v", err)
	}

	if err := c.SaveCollectDeviceConfig(models.CollectDeviceConfig{
		DeviceId: "dev-1", ChannelId: ch.Id, GatewayId: gw.Id, Enabled: true,
	}); err != nil {
		t.Fatalf("save device config: %v", err)
	}
	if err := c.SaveCollectDeviceConfig(models.CollectDeviceConfig{
		DeviceId: "dev-2", ChannelId: otherCh.Id, GatewayId: other.Id, Enabled: true,
	}); err != nil {
		t.Fatalf("save other device config: %v", err)
	}
	// 点位属于设备，不随网关删除：设备重新绑定后还能复用点表。
	if err := c.ReplaceCollectPointConfigs("dev-1", []models.CollectPointConfig{
		{TagId: "D1.a", Address: "400001", DataType: "int16"},
	}); err != nil {
		t.Fatalf("save points: %v", err)
	}

	if err := c.DeleteGatewayWithCollectConfig(gw.Id); err != nil {
		t.Fatalf("delete gateway: %v", err)
	}

	if _, err := c.GatewayById(gw.Id); err == nil {
		t.Fatal("gateway should be deleted")
	}

	channels, err := c.CollectChannels()
	if err != nil {
		t.Fatalf("channels: %v", err)
	}
	for _, item := range channels {
		if item.GatewayId == gw.Id {
			t.Fatalf("channel of deleted gateway still present: %+v", item)
		}
	}
	if len(channels) != 1 || channels[0].GatewayId != other.Id {
		t.Fatalf("other gateway's channel should be kept, got %+v", channels)
	}

	configs, err := c.CollectDeviceConfigs()
	if err != nil {
		t.Fatalf("device configs: %v", err)
	}
	if len(configs) != 1 || configs[0].DeviceId != "dev-2" {
		t.Fatalf("only the other gateway's binding should remain, got %+v", configs)
	}

	points, err := c.CollectPointConfigsByDevice("dev-1")
	if err != nil {
		t.Fatalf("points: %v", err)
	}
	if len(points) != 1 {
		t.Fatalf("point config should be preserved, got %d", len(points))
	}

	// 重复删除应返回资源不存在，而不是静默成功。
	if err := c.DeleteGatewayWithCollectConfig(gw.Id); err == nil {
		t.Fatal("second delete should fail with not found")
	}
}
