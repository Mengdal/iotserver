package e2e

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"iotserver/internal/models"
	"iotserver/internal/pkg/tenant"
	tenantplugin "iotserver/internal/tools/sqldb/tenant"
)

// 看板「指标类数据源」（P0）的端到端验证：
//   ① 平台视图指标值正确（product.total / gateway.total，含值为 0 的指标）
//   ② 租户隔离：租户只统计自己的行，与平台管理员的总数不同
//   ③ platformOnly：tenant.total 租户拿到空、平台管理员拿到数字
//   ④ 未登记 key 被忽略且不报错
//   ⑤ GET /viz/metrics 返回完整清单（key/label/unit/platformOnly）
//
// 隔离用例强制 enforce 策略，验证的是真实隔离而不是迁移期 warn 的兜底行为。

// vizMetricValue 取数响应里的一个指标值
type vizMetricValue struct {
	Key       string  `json:"key"`
	Label     string  `json:"label"`
	Value     float64 `json:"value"`
	Precision int     `json:"precision"`
	Unit      string  `json:"unit"`
}

// queryVizMetrics 以指定 token / 视图执行一次指标取数，返回 key → 指标值
func queryVizMetrics(t *testing.T, env *testEnv, token, tenantHeader string, keys []string) map[string]vizMetricValue {
	t.Helper()
	body, _ := json.Marshal(map[string]interface{}{
		"items": []map[string]interface{}{{"id": "w1", "metrics": keys}},
	})
	w := doRequestWithTenant(env.engine, http.MethodPost, "/api/v1/viz/device-query", token, tenantHeader, body)
	resp := parseResponse(t, w)
	if !resp.Success {
		t.Fatalf("指标取数失败: %s", resp.ErrorMsg)
	}
	raw, _ := json.Marshal(resp.Result)
	var result struct {
		Results []struct {
			Id      string           `json:"id"`
			Metrics []vizMetricValue `json:"metrics"`
		} `json:"results"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatalf("解析取数结果失败: %v（%s）", err, string(raw))
	}
	if len(result.Results) != 1 {
		t.Fatalf("期望 1 个组件的结果，实际 %d", len(result.Results))
	}
	if result.Results[0].Id != "w1" {
		t.Fatalf("组件 id 应原样回显，实际 %q", result.Results[0].Id)
	}
	out := make(map[string]vizMetricValue, len(result.Results[0].Metrics))
	for _, m := range result.Results[0].Metrics {
		out[m.Key] = m
	}
	return out
}

// vizMetricDefs 指标清单原始项
type vizMetricDefs struct {
	Key          string `json:"key"`
	Label        string `json:"label"`
	Unit         string `json:"unit"`
	Kind         string `json:"kind"`
	Precision    int    `json:"precision"`
	PlatformOnly bool   `json:"platformOnly"`
}

// listVizMetrics 查询可绑定的指标清单（原始项，含 platformOnly）
func listVizMetrics(t *testing.T, env *testEnv, token, tenantHeader string) []vizMetricDefs {
	t.Helper()
	w := doRequestWithTenant(env.engine, http.MethodGet, "/api/v1/viz/metrics", token, tenantHeader, nil)
	resp := parseResponse(t, w)
	if !resp.Success {
		t.Fatalf("查询指标清单失败: %s", resp.ErrorMsg)
	}
	raw, _ := json.Marshal(resp.Result)
	var result struct {
		List []vizMetricDefs `json:"list"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatalf("解析指标清单失败: %v（%s）", err, string(raw))
	}
	return result.List
}

// createPlatformProduct 平台管理员建一个平台级产品（tenant_id 为空）
func createPlatformProduct(t *testing.T, env *testEnv, name string) {
	t.Helper()
	body, _ := json.Marshal(map[string]interface{}{
		"name":                 name,
		"category_template_id": "1",
		"protocol":             "MQTT",
		"node_type":            "1",
		"net_type":             "1",
		"data_format":          "1",
	})
	assertSuccess(t, doRequest(env.engine, http.MethodPost, "/api/v1/product", env.token, body), "创建产品 "+name)
}

// createPlatformGateway 平台管理员建一个平台级网关
func createPlatformGateway(t *testing.T, env *testEnv, name, sn string) {
	t.Helper()
	body, _ := json.Marshal(map[string]interface{}{"name": name, "sn": sn})
	assertSuccess(t, doRequest(env.engine, http.MethodPost, "/api/v1/gateway", env.token, body), "创建网关 "+name)
}

// TestVizMetricsPlatformCounts ① 平台视图下指标值正确（0 也要返回）+ ④ 未登记 key 被忽略
func TestVizMetricsPlatformCounts(t *testing.T) {
	env := setupTestEnv(t)
	tenantplugin.SetPolicy(tenantplugin.PolicyEnforce, nil)
	t.Cleanup(func() { tenantplugin.SetPolicy(tenantplugin.PolicyWarn, nil) })

	// 平台管理员建 2 个产品 / 1 个网关
	createPlatformProduct(t, env, "vizmetric_p1")
	createPlatformProduct(t, env, "vizmetric_p2")
	createPlatformGateway(t, env, "vizmetric_gw", "vizmetric_sn")

	got := queryVizMetrics(t, env, env.token, "",
		[]string{"product.total", "product.self", "product.other", "gateway.total", "device.total", "foo.bar"})

	if m, ok := got["product.total"]; !ok || m.Value != 2 {
		t.Fatalf("product.total 应为 2，实际 %+v（返回=%v）", m, ok)
	}
	// 本地/第三方拆分：两项都返回，且相加等于总数
	self, okSelf := got["product.self"]
	other, okOther := got["product.other"]
	if !okSelf || !okOther || self.Value+other.Value != got["product.total"].Value {
		t.Fatalf("product.self/other 应返回且相加等于总数，实际 self=%+v(%v) other=%+v(%v)", self, okSelf, other, okOther)
	}
	if m := got["product.total"]; m.Label != "产品总数" {
		t.Fatalf("product.total 标签应为「产品总数」，实际 %q", m.Label)
	}
	if m, ok := got["gateway.total"]; !ok || m.Value != 1 {
		t.Fatalf("gateway.total 应为 1，实际 %+v（返回=%v）", m, ok)
	}
	// 值为 0 也要正常返回，不能因为 0 而省略
	if m, ok := got["device.total"]; !ok || m.Value != 0 {
		t.Fatalf("device.total 应返回 0，实际 %+v（返回=%v）", m, ok)
	}
	// 未登记 key 直接忽略：不报错、也不出现在结果里
	if m, ok := got["foo.bar"]; ok {
		t.Fatalf("未登记指标 foo.bar 不应返回，实际 %+v", m)
	}
}

// TestVizMetricsTenantIsolation ② 租户只统计自己的行 ③ platformOnly 视图隔离
func TestVizMetricsTenantIsolation(t *testing.T) {
	env := setupTestEnv(t)
	tenantplugin.SetPolicy(tenantplugin.PolicyEnforce, nil)
	t.Cleanup(func() { tenantplugin.SetPolicy(tenantplugin.PolicyWarn, nil) })

	// 平台视图的数据：1 个产品 + 1 个网关，不属于任何租户
	createPlatformProduct(t, env, "vizmetric_platform_p")
	createPlatformGateway(t, env, "vizmetric_platform_gw", "vizmetric_platform_sn")

	tenantA, userA := seedTenant(t, env, "vizmetric_a")
	_, _ = seedTenant(t, env, "vizmetric_b")
	tokenA := loginAs(t, env, userA, tenantTestPassword)

	// 租户 A 自己的 1 个产品 + 1 个网关
	seedTenantProduct(t, env, tenantA.Id, "vizmetric_a_p1")
	if _, err := env.client.WithCtx(tenantCtxOf(tenantA.Id)).AddGateway(models.Gateway{
		Name: "vizmetric_a_gw", Sn: "vizmetric_a_sn",
	}); err != nil {
		t.Fatalf("建租户 A 网关失败: %v", err)
	}

	keys := []string{"product.total", "gateway.total", "device.total", "tenant.total"}
	tenantView := queryVizMetrics(t, env, tokenA, "", keys)
	platformView := queryVizMetrics(t, env, env.token, "", keys)

	// ② 租户 A 只统计自己的行
	if m, ok := tenantView["product.total"]; !ok || m.Value != 1 {
		t.Fatalf("租户 A 的 product.total 应为 1，实际 %+v（返回=%v）", m, ok)
	}
	if m, ok := tenantView["gateway.total"]; !ok || m.Value != 1 {
		t.Fatalf("租户 A 的 gateway.total 应为 1，实际 %+v（返回=%v）", m, ok)
	}
	// 平台管理员看到平台级 + 租户 A 的行：产品 2 / 网关 2
	if m, ok := platformView["product.total"]; !ok || m.Value != 2 {
		t.Fatalf("平台 product.total 应为 2，实际 %+v（返回=%v）", m, ok)
	}
	if m, ok := platformView["gateway.total"]; !ok || m.Value != 2 {
		t.Fatalf("平台 gateway.total 应为 2，实际 %+v（返回=%v）", m, ok)
	}
	if tenantView["product.total"].Value == platformView["product.total"].Value {
		t.Fatal("租户统计结果不应等于平台总数（隔离失效）")
	}

	// ③ platformOnly：租户视图返回空，平台管理员返回数字
	if m, ok := tenantView["tenant.total"]; ok {
		t.Fatalf("租户视图不应返回 platformOnly 指标 tenant.total，实际 %+v", m)
	}
	if m, ok := platformView["tenant.total"]; !ok || m.Value < 2 {
		t.Fatalf("平台 tenant.total 应为已建租户数（>=2），实际 %+v（返回=%v）", m, ok)
	}
}

// TestVizMetricsList ⑤ GET /viz/metrics 返回完整清单
func TestVizMetricsList(t *testing.T) {
	env := setupTestEnv(t)
	tenantplugin.SetPolicy(tenantplugin.PolicyEnforce, nil)
	t.Cleanup(func() { tenantplugin.SetPolicy(tenantplugin.PolicyWarn, nil) })

	// 平台视图：完整清单（12 个常规指标 + tenant.total）
	platformList := listVizMetrics(t, env, env.token, "")
	if len(platformList) != 26 {
		t.Fatalf("平台清单应有 26 个指标，实际 %d：%+v", len(platformList), platformList)
	}
	byKey := make(map[string]vizMetricDefs, len(platformList))
	for _, m := range platformList {
		if m.Key == "" || m.Label == "" {
			t.Fatalf("指标清单项缺少 key/label：%+v", m)
		}
		byKey[m.Key] = m
	}
	for _, key := range []string{
		"product.total", "product.self", "product.other",
		"device.total", "device.self", "device.other",
		"device.online", "device.offline", "device.never",
		"gateway.total", "gateway.online", "gateway.offline", "gateway.never",
		"alarm.pending", "report.today", "report.daily", "user.total",
		"system.cpu.usage", "system.cpu.load", "system.cpu.trend",
		"system.memory.used", "system.memory.total", "system.memory.trend",
		"system.disk.used", "system.disk.total",
	} {
		if _, ok := byKey[key]; !ok {
			t.Fatalf("清单缺少指标 %s", key)
		}
	}
	if m, ok := byKey["tenant.total"]; !ok || !m.PlatformOnly {
		t.Fatalf("tenant.total 应为 platformOnly 指标，实际 %+v（返回=%v）", m, ok)
	}
	if m := byKey["device.total"]; m.Label != "设备总数" || m.Unit != "" {
		t.Fatalf("device.total 的 label/unit 不正确：%+v", m)
	}
	// 时序指标：清单要带 kind=series，前端据此把它限给折线图这类能画序列的组件
	if m, ok := byKey["report.daily"]; !ok || m.Kind != "series" {
		t.Fatalf("report.daily 应为 kind=series 的时序指标，实际 %+v（返回=%v）", m, ok)
	}
	// 系统资源指标：platformOnly + 单位/精度（租户视图看不到，前端据此隐藏对应组件/块）
	if m, ok := byKey["system.cpu.usage"]; !ok || !m.PlatformOnly || m.Unit != "%" || m.Precision != 1 {
		t.Fatalf("system.cpu.usage 应为 platformOnly、unit=%%，precision=1，实际 %+v（返回=%v）", m, ok)
	}
	if m, ok := byKey["system.memory.trend"]; !ok || !m.PlatformOnly || m.Kind != "series" {
		t.Fatalf("system.memory.trend 应为 platformOnly 的时序指标，实际 %+v（返回=%v）", m, ok)
	}

	// 租户视图：不下发平台专属指标
	_, userA := seedTenant(t, env, "vizmetric_list")
	tokenA := loginAs(t, env, userA, tenantTestPassword)
	tenantList := listVizMetrics(t, env, tokenA, "")
	if len(tenantList) != 17 {
		t.Fatalf("租户清单应有 17 个指标，实际 %d：%+v", len(tenantList), tenantList)
	}
	for _, m := range tenantList {
		if m.PlatformOnly {
			t.Fatalf("租户清单不应包含平台专属指标：%+v", m)
		}
		if m.Key == "tenant.total" {
			t.Fatal("租户清单不应包含 tenant.total")
		}
	}
}

/******************************* 时序指标 *******************************/

// seedMsgGather 直接落一条某天的消息汇总（offset = 相对今天的天数，-1 = 昨天）
func seedMsgGather(t *testing.T, env *testEnv, offset int, count int) {
	t.Helper()
	err := env.client.WithCtx(tenant.Platform(context.Background())).AddMsgGather(models.MsgGather{
		Date:  time.Now().AddDate(0, 0, offset).Format("2006-01-02"),
		Count: count,
	})
	if err != nil {
		t.Fatalf("写 msg_gather 失败: %v", err)
	}
}

// vizSeriesPoint 数据序列上的一个点
type vizSeriesPoint struct {
	Time  int64   `json:"time"`
	Value float64 `json:"value"`
}

// vizSeriesItem 取数响应里的一条数据序列（只取断言用得到的字段）
type vizSeriesItem struct {
	Code   string           `json:"code"`
	Name   string           `json:"name"`
	Unit   string           `json:"unit"`
	Points []vizSeriesPoint `json:"points"`
}

// vizSeriesResult 取数响应里一个组件的结果
type vizSeriesResult struct {
	Metrics []vizMetricValue `json:"metrics"`
	Series  []vizSeriesItem  `json:"series"`
}

// queryVizSeries 以指定 token 查一个仅绑了指定指标 key 的组件（近 7 天窗口）
func queryVizSeries(t *testing.T, env *testEnv, token string, keys []string) vizSeriesResult {
	t.Helper()
	body, _ := json.Marshal(map[string]interface{}{
		"items": []map[string]interface{}{{"id": "w1", "metrics": keys, "timeRange": "7d"}},
	})
	w := doRequest(env.engine, http.MethodPost, "/api/v1/viz/device-query", token, body)
	resp := parseResponse(t, w)
	if !resp.Success {
		t.Fatalf("取数失败: %s", resp.ErrorMsg)
	}
	raw, _ := json.Marshal(resp.Result)
	var result struct {
		Results []vizSeriesResult `json:"results"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatalf("解析取数结果失败: %v（%s）", err, string(raw))
	}
	if len(result.Results) != 1 {
		t.Fatalf("期望 1 个组件的结果，实际 %d", len(result.Results))
	}
	return result.Results[0]
}

// TestVizDataDailySeries 时序指标（report.daily）：
//   - 落在 series 而不是 metrics（折线图按序列渲染，零改动接住）；
//   - 近 7 天窗口出 7 个点、按时间正序、缺的天补 0（与首页面积图同口径）；
//   - 租户视图只能看到本租户的汇总（隔离生效）。
func TestVizDataDailySeries(t *testing.T) {
	env := setupTestEnv(t)
	tenantplugin.SetPolicy(tenantplugin.PolicyEnforce, nil)
	t.Cleanup(func() { tenantplugin.SetPolicy(tenantplugin.PolicyWarn, nil) })

	// 前天 5 条、昨天 3 条；今天与更早的 3 天没有记录（应为 0）
	seedMsgGather(t, env, -2, 5)
	seedMsgGather(t, env, -1, 3)

	got := queryVizSeries(t, env, env.token, []string{"report.daily"})
	if len(got.Metrics) != 0 {
		t.Fatalf("时序指标不应出现在 metrics，实际 %+v", got.Metrics)
	}
	if len(got.Series) != 1 {
		t.Fatalf("期望 1 条数据序列，实际 %d：%+v", len(got.Series), got.Series)
	}
	series := got.Series[0]
	if series.Code != "report.daily" || series.Name != "每日消息量" {
		t.Fatalf("序列元信息不对：code=%q name=%q", series.Code, series.Name)
	}
	if len(series.Points) != 7 {
		t.Fatalf("近 7 天窗口应有 7 个点（含今天），实际 %d：%+v", len(series.Points), series.Points)
	}
	for i := 1; i < len(series.Points); i++ {
		if series.Points[i].Time <= series.Points[i-1].Time {
			t.Fatalf("点应按时间正序：%+v", series.Points)
		}
	}
	now := time.Now()
	wantLast := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local).UnixMilli()
	last := series.Points[len(series.Points)-1]
	if last.Time != wantLast || last.Value != 0 {
		t.Fatalf("最后一个点应是今天 0 点且值为 0，实际 %+v（期望 time=%d）", last, wantLast)
	}
	if got := series.Points[len(series.Points)-2]; got.Value != 3 {
		t.Fatalf("昨天应补到 3，实际 %v", got.Value)
	}
	if got := series.Points[len(series.Points)-3]; got.Value != 5 {
		t.Fatalf("前天应补到 5，实际 %v", got.Value)
	}
	if got := series.Points[0]; got.Value != 0 {
		t.Fatalf("窗口起点当天没有记录应为 0，实际 %v", got.Value)
	}

	// 租户隔离：租户自己没写过 msg_gather，视图内应全是 0
	_, userA := seedTenant(t, env, "vizseries_a")
	tokenA := loginAs(t, env, userA, tenantTestPassword)
	tenantGot := queryVizSeries(t, env, tokenA, []string{"report.daily"})
	if len(tenantGot.Series) != 1 {
		t.Fatalf("租户视图期望 1 条序列，实际 %d", len(tenantGot.Series))
	}
	for _, p := range tenantGot.Series[0].Points {
		if p.Value != 0 {
			t.Fatalf("租户视图不应看到平台的汇总，实际 %+v", tenantGot.Series[0].Points)
		}
	}
}
