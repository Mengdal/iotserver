package sqlite

import (
	"testing"

	"iotserver/internal/dtos"
	"iotserver/internal/models"
)

// TestGatewaySearchStatusFilter 锁定 GatewaySearchQueryRequest.Status 的契约：
// **只有 0/1/2 才过滤，其它值（含前端未选择状态时显式传的 -1）都表示不过滤**。
//
// 背景：网关管理页在「未选择状态」时显式传 status=-1
// （iotserver-ui/src/pages/gateway/GatewayManage/index.tsx:150）。而原来的实现
// 只要 Status 非 nil 就 `WHERE status = ?`，于是 status=-1 过滤掉全部行——
// 该页面对任何人、任何时候都返回空列表、显示 No data（实测不带 status 返回 4 条、
// 带 status=-1 返回 0 条）。契约写在 dtos.GatewaySearchQueryRequest.Status 的注释里。
func TestGatewaySearchStatusFilter(t *testing.T) {
	c := newTestClient(t)
	if err := c.MigrateSystemTables(); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	gws := []struct {
		id     string
		sn     string
		status int
	}{
		{"gw-offline", "SN-OFFLINE", models.GatewayStatusOffline},
		{"gw-online", "SN-ONLINE", models.GatewayStatusOnline},
		{"gw-never", "SN-NEVER", models.GatewayStatusNever},
	}
	for _, g := range gws {
		if err := c.Pool.Table(models.TableGateway).Create(map[string]interface{}{
			"id": g.id, "sn": g.sn, "name": g.sn, "status": g.status,
			"tenant_id": "", "created": 1, "modified": 1,
		}).Error; err != nil {
			t.Fatalf("insert gateway %s: %v", g.sn, err)
		}
	}

	search := func(status *int) int {
		t.Helper()
		list, total, err := gatewaySearch(c, 0, 50, dtos.GatewaySearchQueryRequest{Status: status})
		if err != nil {
			t.Fatalf("search: %v", err)
		}
		if int(total) != len(list) {
			t.Fatalf("total(%d) 与返回条数(%d) 不一致", total, len(list))
		}
		return len(list)
	}
	i := func(v int) *int { return &v }

	// 不过滤：nil 与任意非 0/1/2 的值都必须返回全部
	for _, c := range []struct {
		name   string
		status *int
	}{
		{"nil", nil},
		{"-1（前端未选择状态时的取值）", i(-1)},
		{"99", i(99)},
	} {
		if got := search(c.status); got != 3 {
			t.Errorf("status=%s 应不过滤、返回 3 条，实际 %d 条", c.name, got)
		}
	}

	// 过滤：0/1/2 各自只命中一条
	for _, c := range []struct {
		name   string
		status int
	}{
		{"0（离线）", models.GatewayStatusOffline},
		{"1（在线）", models.GatewayStatusOnline},
		{"2（从未上线）", models.GatewayStatusNever},
	} {
		if got := search(i(c.status)); got != 1 {
			t.Errorf("status=%s 应命中 1 条，实际 %d 条", c.name, got)
		}
	}
}
