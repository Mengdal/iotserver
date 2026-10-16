package sqlite

import (
	"testing"

	"iotserver/internal/models"
)

// TestDeviceByGatewayAndKeyMissIsNotAnError 网关绑定未命中时必须返回领域错误、
// 且**不能**让 GORM 产生 ErrRecordNotFound。
//
// 背景：网关上报是「先按网关绑定查找、未绑定时再按设备 SN 兜底」的两步流程
// （gatewayingest.resolveDevice）。第一步查不到是正常分支，但若用 First 查询，
// GORM 默认 logger 会把 ErrRecordNotFound 打成 error 级日志，未绑定网关的设备
// 每上报一次就刷一条看起来像报错的日志。这里锁定「查空不报错」的语义。
func TestDeviceByGatewayAndKeyMissIsNotAnError(t *testing.T) {
	c := newTestClient(t)
	if err := c.MigrateSystemTables(); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	// 未绑定的设备：gateway_id / device_key 均为空
	if err := c.Pool.Table("device").Create(map[string]interface{}{
		"id":        "dev-unbound",
		"name":      "未绑定设备",
		"device_sn": "SN-UNBOUND-1",
		"tenant_id": "",
		"created":   1,
		"modified":  1,
	}).Error; err != nil {
		t.Fatalf("insert device: %v", err)
	}

	// 1) 绑定未命中：应返回错误（调用方据此走 SN 兜底），而不是返回空设备
	_, err := deviceByGatewayAndKey(c, "gw-1", "SN-UNBOUND-1")
	if err == nil {
		t.Fatalf("绑定未命中应返回错误，供调用方走 SN 兜底")
	}

	// 2) 真正绑定后应能查到
	if err := c.Pool.Table("device").
		Where("id = ?", "dev-unbound").
		Updates(map[string]interface{}{"gateway_id": "gw-1", "device_key": "SN-UNBOUND-1"}).Error; err != nil {
		t.Fatalf("bind device: %v", err)
	}
	got, err := deviceByGatewayAndKey(c, "gw-1", "SN-UNBOUND-1")
	if err != nil {
		t.Fatalf("绑定后应能查到: %v", err)
	}
	if got.Id != "dev-unbound" {
		t.Fatalf("查到设备 id=%s want dev-unbound", got.Id)
	}

	// 3) 按 SN 兜底仍可用（第一步失败后的第二步）
	bySn, err := deviceBySn(c, "SN-UNBOUND-1")
	if err != nil {
		t.Fatalf("按 SN 查询失败: %v", err)
	}
	if bySn.Id != "dev-unbound" {
		t.Fatalf("按 SN 查到 id=%s want dev-unbound", bySn.Id)
	}

	// 4) 参数校验：空 gateway/key 仍应报参数错误
	if _, err := deviceByGatewayAndKey(c, "", "x"); err == nil {
		t.Fatalf("空 gatewayId 应报参数错误")
	}
	if _, err := deviceByGatewayAndKey(c, "gw-1", ""); err == nil {
		t.Fatalf("空 deviceKey 应报参数错误")
	}

	var _ = models.Device{}
}
