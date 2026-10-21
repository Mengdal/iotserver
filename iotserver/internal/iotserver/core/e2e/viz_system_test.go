package e2e

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"iotserver/internal/dtos"
	"iotserver/internal/pkg/tenant"
	tenantplugin "iotserver/internal/tools/sqldb/tenant"
)

// 系统资源指标（system.*）与内置看板种子的端到端验证。

// seedSystemSample 写一条系统采样（平台作用域，时间戳自己指定）
func seedSystemSample(t *testing.T, env *testEnv, at time.Time, cpu float64, memPercent float64, memUsed uint64) {
	t.Helper()
	metrics := dtos.SystemMetrics{
		Timestamp:      at.UnixMilli(),
		CpuUsedPercent: cpu,
		CpuAvg:         2,
		Memory: dtos.SystemMemory{
			Total:       16 * 1024 * 1024 * 1024,
			Used:        memUsed,
			UsedPercent: memPercent,
		},
		Disk: dtos.SystemDisk{
			Path:        "/",
			Total:       100 * 1024 * 1024 * 1024,
			Used:        40 * 1024 * 1024 * 1024,
			UsedPercent: 40,
		},
	}
	if err := env.client.WithCtx(tenant.Platform(context.Background())).UpdateSystemMetrics(metrics); err != nil {
		t.Fatalf("写系统采样失败: %v", err)
	}
}

// queryVizTrend 以显式时间窗与聚合间隔查一个时序指标，返回那一条序列
func queryVizTrend(t *testing.T, env *testEnv, token, key string, from, to int64, aggInterval int) vizSeriesItem {
	t.Helper()
	body, _ := json.Marshal(map[string]interface{}{
		"items": []map[string]interface{}{{
			"id": "w1", "metrics": []string{key},
			"range": []int64{from, to}, "aggInterval": aggInterval, "limit": 100,
		}},
	})
	w := doRequest(env.engine, http.MethodPost, "/api/v1/viz/device-query", token, body)
	resp := parseResponse(t, w)
	if !resp.Success {
		t.Fatalf("时序指标取数失败: %s", resp.ErrorMsg)
	}
	raw, _ := json.Marshal(resp.Result)
	var result struct {
		Results []vizSeriesResult `json:"results"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatalf("解析取数结果失败: %v（%s）", err, string(raw))
	}
	if len(result.Results) != 1 || len(result.Results[0].Series) != 1 {
		t.Fatalf("期望 1 个组件 1 条序列，实际 %+v", result.Results)
	}
	return result.Results[0].Series[0]
}

// TestVizSystemMetrics 系统指标：瞬时值取最新采样；租户视图拿不到（platformOnly）
func TestVizSystemMetrics(t *testing.T) {
	env := setupTestEnv(t)
	tenantplugin.SetPolicy(tenantplugin.PolicyEnforce, nil)
	t.Cleanup(func() { tenantplugin.SetPolicy(tenantplugin.PolicyWarn, nil) })

	// 一条较旧 + 一条较新：瞬时值要取最新那条
	now := time.Now()
	seedSystemSample(t, env, now.Add(-3*time.Minute), 10, 30, 3*1024*1024*1024)
	seedSystemSample(t, env, now.Add(-time.Minute), 42.5, 50, 4*1024*1024*1024)

	got := queryVizMetrics(t, env, env.token, "",
		[]string{"system.cpu.usage", "system.memory.used", "system.memory.total"})
	if m, ok := got["system.cpu.usage"]; !ok || m.Value != 42.5 || m.Precision != 1 || m.Unit != "%" {
		t.Fatalf("system.cpu.usage 应取最新采样的 42.5%%，实际 %+v（返回=%v）", m, ok)
	}
	if m, ok := got["system.memory.used"]; !ok || m.Value != 4 || m.Precision != 1 || m.Unit != "GB" {
		t.Fatalf("system.memory.used 应为最新的 4 GB，实际 %+v（返回=%v）", m, ok)
	}
	if m, ok := got["system.memory.total"]; !ok || m.Value != 16 {
		t.Fatalf("system.memory.total 应为 16 GB，实际 %+v（返回=%v）", m, ok)
	}

	// 租户视图：系统指标不下发、取数也拿不到
	_, userA := seedTenant(t, env, "vizsys_a")
	tokenA := loginAs(t, env, userA, tenantTestPassword)
	tenantGot := queryVizMetrics(t, env, tokenA, "", []string{"system.cpu.usage", "system.memory.used"})
	if len(tenantGot) != 0 {
		t.Fatalf("租户视图不应拿到系统指标，实际 %+v", tenantGot)
	}
}

// TestVizSystemTrendBuckets 趋势：按聚合间隔分桶求平均，桶内多采样取平均
func TestVizSystemTrendBuckets(t *testing.T) {
	env := setupTestEnv(t)
	tenantplugin.SetPolicy(tenantplugin.PolicyEnforce, nil)
	t.Cleanup(func() { tenantplugin.SetPolicy(tenantplugin.PolicyWarn, nil) })

	// 对齐到 5 分钟桶边界，前 5 条落在同一个桶（10/20/30/40/50，平均 30），第 6 条落下一个桶（60）
	base := (time.Now().UnixMilli() / (300 * 1000)) * 300 * 1000
	for i, cpu := range []float64{10, 20, 30, 40, 50} {
		seedSystemSample(t, env, time.UnixMilli(base+int64(i)*60_000), cpu, 50, 0)
	}
	seedSystemSample(t, env, time.UnixMilli(base+300_000), 60, 50, 0)

	series := queryVizTrend(t, env, env.token, "system.cpu.trend", base, base+360_000, 300)
	if series.Code != "system.cpu.trend" || series.Unit != "%" {
		t.Fatalf("序列元信息不对：%+v", series)
	}
	if len(series.Points) != 2 {
		t.Fatalf("5 分钟桶聚合应出 2 个点，实际 %d：%+v", len(series.Points), series.Points)
	}
	if series.Points[0].Value != 30 {
		t.Fatalf("第 1 桶应为 5 条平均 30，实际 %v", series.Points[0].Value)
	}
	if series.Points[1].Value != 60 {
		t.Fatalf("第 2 桶应为 60，实际 %v", series.Points[1].Value)
	}
}

/******************************* 内置看板种子 *******************************/

type boardListItem struct {
	Id          string `json:"id"`
	Name        string `json:"name"`
	IsHome      bool   `json:"isHome"`
	IsDefault   bool   `json:"isDefault"`
	WidgetCount int    `json:"widgetCount"`
}

// listBoards 该 token 可见的看板列表
func listBoards(t *testing.T, env *testEnv, token string) []boardListItem {
	t.Helper()
	w := doRequest(env.engine, http.MethodGet, "/api/v1/viz/boards", token, nil)
	resp := parseResponse(t, w)
	if !resp.Success {
		t.Fatalf("看板列表失败: %s", resp.ErrorMsg)
	}
	raw, _ := json.Marshal(resp.Result)
	var result struct {
		List []boardListItem `json:"list"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatalf("解析看板列表失败: %v（%s）", err, string(raw))
	}
	return result.List
}

/*
TestVizBoardSeeding 新装即自带内置看板（看板列表为空时自动种入）：

	平台租户：首页看板（isHome）+ 组件演示看板（isDefault）+ 系统监控看板（3 块）
	普通租户：首页看板 + 组件演示看板（2 块，系统监控看板仅平台拥有）
*/
func TestVizBoardSeeding(t *testing.T) {
	env := setupTestEnv(t)
	tenantplugin.SetPolicy(tenantplugin.PolicyEnforce, nil)
	t.Cleanup(func() { tenantplugin.SetPolicy(tenantplugin.PolicyWarn, nil) })

	platformBoards := listBoards(t, env, env.token)
	if len(platformBoards) != 3 {
		t.Fatalf("平台应自动种入 3 块内置看板，实际 %d：%+v", len(platformBoards), platformBoards)
	}
	byId := make(map[string]boardListItem, len(platformBoards))
	for _, b := range platformBoards {
		byId[b.Id] = b
		if b.WidgetCount == 0 {
			t.Fatalf("内置看板 %s 不应是空看板", b.Id)
		}
	}
	if home, ok := byId["builtin-home"]; !ok || !home.IsHome || home.Name != "Home Board" {
		t.Fatalf("应有 isHome 的首页看板，实际 %+v（返回=%v）", home, ok)
	}
	if demo, ok := byId["builtin-demo"]; !ok || !demo.IsDefault || demo.Name != "Component Demo" {
		t.Fatalf("应有 isDefault 的组件演示看板，实际 %+v（返回=%v）", demo, ok)
	}
	if _, ok := byId["builtin-ops"]; !ok {
		t.Fatalf("应有系统监控看板，实际 %+v", platformBoards)
	}

	// 再查一次不应重复种（列表非空即不再触发）
	if again := listBoards(t, env, env.token); len(again) != 3 {
		t.Fatalf("重复查询不应重复种入，实际 %d：%+v", len(again), again)
	}

	// 租户：首页看板 + 组件演示看板，且拿不到平台自用的系统监控看板。
	// id 带租户后缀（全局主键，不能和平台的 builtin-home 撞），按名字断言。
	_, userA := seedTenant(t, env, "vizseed_a")
	tokenA := loginAs(t, env, userA, tenantTestPassword)
	tenantBoards := listBoards(t, env, tokenA)
	if len(tenantBoards) != 2 {
		t.Fatalf("租户应自动种入 2 块内置看板，实际 %d：%+v", len(tenantBoards), tenantBoards)
	}
	tenantNames := make(map[string]boardListItem, len(tenantBoards))
	for _, b := range tenantBoards {
		tenantNames[b.Name] = b
	}
	if home, ok := tenantNames["Home Board"]; !ok || !home.IsHome {
		t.Fatalf("租户应有 isHome 的首页看板，实际 %+v", tenantBoards)
	}
	if demo, ok := tenantNames["Component Demo"]; !ok || !demo.IsDefault {
		t.Fatalf("租户应有 isDefault 的组件演示看板，实际 %+v", tenantBoards)
	}
	if _, ok := tenantNames["System Monitoring"]; ok {
		t.Fatalf("租户不应拿到平台自用的系统监控看板，实际 %+v", tenantBoards)
	}

	// 平台侧再确认一次：不同租户的种子互不干扰（各 2 块，id 不冲突）
	_, userB := seedTenant(t, env, "vizseed_b")
	tokenB := loginAs(t, env, userB, tenantTestPassword)
	if got := listBoards(t, env, tokenB); len(got) != 2 {
		t.Fatalf("租户B应同样种入 2 块内置看板，实际 %d：%+v", len(got), got)
	}

	// 平台视图只应看到平台自己的 3 块看板，不能把各租户的看板混进来
	// （否则同名看板会重复，且「设为首页/默认」会互相覆盖）。
	if got := listBoards(t, env, env.token); len(got) != 3 {
		t.Fatalf("平台视图应只看到 3 块平台看板，实际 %d：%+v", len(got), got)
	}
}

/*
TestVizBoardHomeSelfHeal 存量安装自愈：有看板、但一块都没设「首页看板」时，
再次访问列表自动补回内置首页看板；且补种是合并保存，不能动到已有的其它看板
（saveVizBoards 是整体替换，种入路径必须走 merge）。
*/
func TestVizBoardHomeSelfHeal(t *testing.T) {
	env := setupTestEnv(t)
	tenantplugin.SetPolicy(tenantplugin.PolicyEnforce, nil)
	t.Cleanup(func() { tenantplugin.SetPolicy(tenantplugin.PolicyWarn, nil) })

	// 首次访问：列表为空 → 种入 3 块（含首页看板）
	boards := listBoards(t, env, env.token)
	if len(boards) != 3 {
		t.Fatalf("首次应种入 3 块，实际 %d：%+v", len(boards), boards)
	}
	homeId := ""
	otherIds := make(map[string]bool)
	for _, b := range boards {
		if b.IsHome {
			homeId = b.Id
			continue
		}
		otherIds[b.Id] = true
	}
	if homeId == "" {
		t.Fatalf("种入后应有首页看板：%+v", boards)
	}

	// 删掉首页看板，制造「有看板但没设首页」的存量状态
	assertSuccess(t, doRequest(env.engine, http.MethodDelete, "/api/v1/viz/boards/"+homeId, env.token, nil), "删除首页看板")

	// 再访问：自动补回首页看板，且原有其它看板一个不少
	after := listBoards(t, env, env.token)
	if len(after) != 3 {
		t.Fatalf("自愈后应回到 3 块（2 块保留 + 1 块补回），实际 %d：%+v", len(after), after)
	}
	homeCount := 0
	for _, b := range after {
		if b.IsHome {
			homeCount++
			if b.Id != homeId {
				t.Fatalf("补回的首页看板 id 应稳定为 %s，实际 %s", homeId, b.Id)
			}
		}
		if otherIds[b.Id] {
			delete(otherIds, b.Id)
		}
	}
	if homeCount != 1 {
		t.Fatalf("自愈后应有且仅有一块首页看板，实际 %d：%+v", homeCount, after)
	}
	if len(otherIds) != 0 {
		t.Fatalf("补种不应动到已有看板，丢失：%+v", otherIds)
	}
}
