package sqlite

import (
	"testing"

	"iotserver/internal/models"
)

// 只有 status=1 的记录才算已处理；失败记录必须允许重试，且重试成功后原地更新为成功。
func TestGatewayIngestLogRetryAfterFailure(t *testing.T) {
	c := newTestClient(t)
	if err := c.MigrateSystemTables(); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	const gatewayID = "gw-ingest-test"
	if err := addGatewayIngestLog(c, models.GatewayIngestLog{
		GatewayId: gatewayID,
		Seq:       100,
		MsgCount:  1,
		Status:    0,
		Error:     "db error",
	}); err != nil {
		t.Fatalf("写入失败记录: %v", err)
	}

	exists, err := gatewayIngestLogExists(c, gatewayID, 100)
	if err != nil {
		t.Fatalf("查询失败记录: %v", err)
	}
	if exists {
		t.Fatal("status=0 的失败记录不应被当作已处理")
	}

	// 网关重发后处理成功：同一 (gateway_id, seq) 原地更新为 status=1。
	if err := addGatewayIngestLog(c, models.GatewayIngestLog{
		GatewayId: gatewayID,
		Seq:       100,
		MsgCount:  3,
		Status:    1,
	}); err != nil {
		t.Fatalf("更新成功记录: %v", err)
	}

	exists, err = gatewayIngestLogExists(c, gatewayID, 100)
	if err != nil {
		t.Fatalf("查询成功记录: %v", err)
	}
	if !exists {
		t.Fatal("status=1 的记录应被当作已处理")
	}

	var got models.GatewayIngestLog
	if err := c.Pool.Table(models.TableGatewayIngestLog).
		Where("gateway_id = ? AND seq = ?", gatewayID, 100).First(&got).Error; err != nil {
		t.Fatalf("读取记录: %v", err)
	}
	if got.Status != 1 || got.MsgCount != 3 || got.Error != "" {
		t.Fatalf("重试后记录未更新: %+v", got)
	}
}
