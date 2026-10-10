package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"iotserver/internal/dtos"
	"iotserver/internal/pkg/httphelper"
)

/** 假 eKuiper 客户端：记录调用并持有规则，供断言 **/

type fakeRule struct {
	Sql     string
	Actions []dtos.Actions
	Running bool
}

type fakeEkuiper struct {
	mu    sync.Mutex
	rules map[string]*fakeRule
	calls []string
}

func newFakeEkuiper() *fakeEkuiper {
	return &fakeEkuiper{rules: map[string]*fakeRule{}}
}

func (f *fakeEkuiper) record(format string, args ...interface{}) {
	f.calls = append(f.calls, fmt.Sprintf(format, args...))
}

func (f *fakeEkuiper) Called(format string, args ...interface{}) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	want := fmt.Sprintf(format, args...)
	for _, c := range f.calls {
		if c == want {
			return true
		}
	}
	return false
}

func (f *fakeEkuiper) RuleExist(ctx context.Context, ruleId string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.record("exist:%s", ruleId)
	_, ok := f.rules[ruleId]
	return ok, nil
}

func (f *fakeEkuiper) CreateRule(ctx context.Context, actions []dtos.Actions, ruleId, sql string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.record("create:%s", ruleId)
	f.rules[ruleId] = &fakeRule{Sql: sql, Actions: actions}
	return nil
}

func (f *fakeEkuiper) UpdateRule(ctx context.Context, actions []dtos.Actions, ruleId, sql string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.record("update:%s", ruleId)
	f.rules[ruleId] = &fakeRule{Sql: sql, Actions: actions}
	return nil
}

func (f *fakeEkuiper) GetRuleStats(ctx context.Context, ruleId string) (map[string]interface{}, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.record("stats:%s", ruleId)
	r, ok := f.rules[ruleId]
	if !ok {
		return nil, fmt.Errorf("rule not found")
	}
	status := "stopped"
	if r.Running {
		status = "running"
	}
	return map[string]interface{}{"status": status, "rule_id": ruleId, "source_mqtt_stream_0_records_in_total": 12}, nil
}

func (f *fakeEkuiper) StartRule(ctx context.Context, ruleId string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.record("start:%s", ruleId)
	r, ok := f.rules[ruleId]
	if !ok {
		return fmt.Errorf("rule not found")
	}
	r.Running = true
	return nil
}

func (f *fakeEkuiper) StopRule(ctx context.Context, ruleId string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.record("stop:%s", ruleId)
	r, ok := f.rules[ruleId]
	if !ok {
		return fmt.Errorf("rule not found")
	}
	r.Running = false
	return nil
}

func (f *fakeEkuiper) RestartRule(ctx context.Context, ruleId string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.record("restart:%s", ruleId)
	return nil
}

func (f *fakeEkuiper) DeleteRule(ctx context.Context, ruleId string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.record("delete:%s", ruleId)
	delete(f.rules, ruleId)
	return nil
}

func (f *fakeEkuiper) rule(id string) *fakeRule {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.rules[id]
}

/** 测试 **/

// TestRuleEngineFlow 规则引擎全链路：新增 → 列表 → 详情 → 启动 → 状态 → 停止 → 删除
func TestRuleEngineFlow(t *testing.T) {
	env := setupTestEnv(t)
	fake := env.fakeEkuiper

	// 目标 HTTP 服务（作为转发目标）
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer target.Close()

	// 1) 先建一个数据资源并验证通过（规则必须绑定资源）
	addRes := map[string]interface{}{
		"name": "rule-target-http", "type": "HTTP推送",
		"option": map[string]interface{}{"url": target.URL + "/push", "method": "post"},
	}
	body, _ := json.Marshal(addRes)
	assertSuccess(t, doRequest(env.engine, http.MethodPost, "/api/v1/dataresource", env.token, body), "建资源")
	var resPage struct {
		Result struct {
			List []map[string]interface{} `json:"list"`
		} `json:"result"`
	}
	w := doRequest(env.engine, http.MethodGet, "/api/v1/dataresource?type=HTTP推送", env.token, nil)
	_ = json.Unmarshal(w.Body.Bytes(), &resPage)
	resourceId, _ := resPage.Result.List[0]["id"].(string)
	assertVerify(t, doRequest(env.engine, http.MethodPost,
		fmt.Sprintf("/api/v1/dataresource/%s/health", resourceId), env.token, nil), true)

	// 2) 新增规则（请求体完全按前端 AddDirectRule 提交结构）
	addRule := map[string]interface{}{
		"name":             "e2e-rule-1",
		"description":      "端到端规则",
		"resource_type":    "HTTP推送",
		"data_resource_id": resourceId,
		"filter": map[string]interface{}{
			"message_source": "消息总线",
			"select_name":    "deviceId",
			"condition":      `deviceId = "dev-1"`,
			"sql":            `SELECT deviceId FROM mqtt_stream where deviceId = "dev-1"`,
		},
	}
	body, _ = json.Marshal(addRule)
	w = doRequest(env.engine, http.MethodPost, "/api/v1/rule-engine", env.token, body)
	assertSuccess(t, w, "新增规则")

	// 3) 列表：dataResource 必须是对象、created 为毫秒时间戳、状态 stopped
	w = doRequest(env.engine, http.MethodGet, "/api/v1/rule-engine?page=1&pageSize=10", env.token, nil)
	var listResp struct {
		Success bool `json:"success"`
		Result  struct {
			List  []map[string]interface{} `json:"list"`
			Total int                      `json:"total"`
		} `json:"result"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if !listResp.Success || listResp.Result.Total != 1 {
		t.Fatalf("列表异常: success=%v total=%d body=%s", listResp.Success, listResp.Result.Total, w.Body.String())
	}
	item := listResp.Result.List[0]
	ruleId, _ := item["id"].(string)
	if ruleId == "" {
		t.Fatal("规则 id 为空")
	}
	dr, ok := item["dataResource"].(map[string]interface{})
	if !ok {
		t.Fatalf("dataResource 必须是对象（前端列表列直接读 .type），got %T", item["dataResource"])
	}
	if dr["type"] != "HTTP推送" {
		t.Fatalf("dataResource.type = %v, want HTTP推送", dr["type"])
	}
	if item["status"] != "stopped" {
		t.Fatalf("新建规则状态应为 stopped, got %v", item["status"])
	}
	if item["resource_type"] != "HTTP推送" {
		t.Fatalf("resource_type = %v", item["resource_type"])
	}
	if created, _ := item["created"].(float64); created < 1e12 {
		t.Fatalf("created 应为毫秒时间戳, got %v", item["created"])
	}
	// 下发到 eKuiper 的 SQL 与动作
	if !fake.Called("create:%s", ruleId) {
		t.Fatal("未调用 eKuiper CreateRule")
	}
	fr := fake.rule(ruleId)
	if fr == nil || !strings.Contains(fr.Sql, "mqtt_stream") {
		t.Fatalf("eKuiper 规则 SQL 异常: %+v", fr)
	}
	if len(fr.Actions) != 1 || fr.Actions[0].Rest == nil {
		t.Fatalf("HTTP 资源应生成 rest 动作, got %+v", fr.Actions)
	}
	if fr.Actions[0].Rest["url"] != target.URL+"/push" {
		t.Fatalf("rest.url = %v", fr.Actions[0].Rest["url"])
	}

	// 4) 详情：filter / data_resource_id / dataResource.type 都要有（编辑回填依赖）
	w = doRequest(env.engine, http.MethodGet, fmt.Sprintf("/api/v1/rule-engine/%s", ruleId), env.token, nil)
	var detail struct {
		Result map[string]interface{} `json:"result"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &detail)
	if detail.Result["data_resource_id"] != resourceId {
		t.Fatalf("详情缺 data_resource_id: %v", detail.Result["data_resource_id"])
	}
	filter, _ := detail.Result["filter"].(map[string]interface{})
	if filter["sql"] == nil || filter["message_source"] != "消息总线" {
		t.Fatalf("详情 filter 异常: %+v", filter)
	}

	// 5) 启动 → 状态 running
	w = doRequest(env.engine, http.MethodPost, fmt.Sprintf("/api/v1/rule-engine/%s/start", ruleId), env.token, nil)
	assertSuccess(t, w, "启动规则")
	if !fake.Called("start:%s", ruleId) {
		t.Fatal("未调用 eKuiper StartRule")
	}
	assertRuleStatus(t, env, ruleId, "running")

	// 6) 运行中的规则不允许修改（21500）
	editBody, _ := json.Marshal(map[string]interface{}{
		"id": ruleId, "name": "e2e-rule-renamed", "description": "x",
		"resource_type": "HTTP推送", "data_resource_id": resourceId,
		"filter": map[string]interface{}{"select_name": "*"},
	})
	w = doRequest(env.engine, http.MethodPut, "/api/v1/rule-engine", env.token, editBody)
	var ecode struct {
		Success   bool   `json:"success"`
		ErrorCode uint32 `json:"errorCode"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &ecode)
	if ecode.Success || ecode.ErrorCode != 21500 {
		t.Fatalf("运行中修改应返回 21500, got success=%v code=%d", ecode.Success, ecode.ErrorCode)
	}

	// 7) 状态接口（前端 URL 带两个尾随空格，必须仍然可用）
	w = doRequest(env.engine, http.MethodGet, fmt.Sprintf("/api/v1/rule-engine/%s/status%%20%%20", ruleId), env.token, nil)
	var statusResp struct {
		Success bool                   `json:"success"`
		Result  map[string]interface{} `json:"result"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &statusResp); err != nil {
		t.Fatalf("decode status: %v", err)
	}
	if !statusResp.Success {
		t.Fatalf("带尾随空格的 status 接口应可用, body=%s", w.Body.String())
	}
	// 前端 httpRequest 要求 HTTP 200 才解析信封，容忍路径不能让状态行变成 404
	if w.Code != http.StatusOK {
		t.Fatalf("带尾随空格的 status 接口 HTTP 状态应为 200, got %d", w.Code)
	}
	if statusResp.Result["status"] != "running" {
		t.Fatalf("eKuiper 状态 = %v, want running", statusResp.Result["status"])
	}

	// 8) 停止 → 状态 stopped；再修改就允许了
	w = doRequest(env.engine, http.MethodPost, fmt.Sprintf("/api/v1/rule-engine/%s/stop", ruleId), env.token, nil)
	assertSuccess(t, w, "停止规则")
	assertRuleStatus(t, env, ruleId, "stopped")

	editBody, _ = json.Marshal(map[string]interface{}{
		"id": ruleId, "name": "e2e-rule-renamed", "description": "改过了",
		"resource_type": "HTTP推送", "data_resource_id": resourceId,
		"filter": map[string]interface{}{
			"message_source": "消息总线", "select_name": "*",
			"sql": "SELECT * FROM mqtt_stream",
		},
	})
	assertSuccess(t, doRequest(env.engine, http.MethodPut, "/api/v1/rule-engine", env.token, editBody), "修改规则")
	if !fake.Called("update:%s", ruleId) {
		t.Fatal("未调用 eKuiper UpdateRule")
	}

	// 9) 删除：eKuiper 与库都要清掉
	w = doRequest(env.engine, http.MethodDelete, fmt.Sprintf("/api/v1/rule-engine/%s/delete", ruleId), env.token, nil)
	assertSuccess(t, w, "删除规则")
	if !fake.Called("delete:%s", ruleId) {
		t.Fatal("未调用 eKuiper DeleteRule")
	}
	if fake.rule(ruleId) != nil {
		t.Fatal("eKuiper 侧规则未删除")
	}
	w = doRequest(env.engine, http.MethodGet, "/api/v1/rule-engine", env.token, nil)
	_ = json.Unmarshal(w.Body.Bytes(), &listResp)
	if listResp.Result.Total != 0 {
		t.Fatalf("删除后应为空, total=%d", listResp.Result.Total)
	}
	// 删除不存在的规则
	assertFail(t, doRequest(env.engine, http.MethodDelete, "/api/v1/rule-engine/not-exist/delete", env.token, nil), "删除不存在")
}

// TestRuleEngineParamErrors 参数类错误
func TestRuleEngineParamErrors(t *testing.T) {
	env := setupTestEnv(t)

	// 名称非法
	body, _ := json.Marshal(map[string]interface{}{"name": "1bad", "data_resource_id": "x"})
	assertFail(t, doRequest(env.engine, http.MethodPost, "/api/v1/rule-engine", env.token, body), "名称非法")

	// 未选资源
	body, _ = json.Marshal(map[string]interface{}{"name": "ok-name"})
	assertFail(t, doRequest(env.engine, http.MethodPost, "/api/v1/rule-engine", env.token, body), "未选资源")

	// 资源不存在 → 21600 InvalidSource
	body, _ = json.Marshal(map[string]interface{}{"name": "ok-name", "data_resource_id": "not-exist"})
	w := doRequest(env.engine, http.MethodPost, "/api/v1/rule-engine", env.token, body)
	var resp httphelper.CommonResponse
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Success || resp.ErrorCode != 21600 {
		t.Fatalf("资源不存在应返回 21600, got success=%v code=%d", resp.Success, resp.ErrorCode)
	}

	// 重名
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer target.Close()
	addRes := map[string]interface{}{"name": "dup-target", "type": "HTTP推送",
		"option": map[string]interface{}{"url": target.URL}}
	body, _ = json.Marshal(addRes)
	assertSuccess(t, doRequest(env.engine, http.MethodPost, "/api/v1/dataresource", env.token, body), "建资源")
	var resPage struct {
		Result struct {
			List []map[string]interface{} `json:"list"`
		} `json:"result"`
	}
	w = doRequest(env.engine, http.MethodGet, "/api/v1/dataresource?type=HTTP推送", env.token, nil)
	_ = json.Unmarshal(w.Body.Bytes(), &resPage)
	rid, _ := resPage.Result.List[0]["id"].(string)

	rule := map[string]interface{}{"name": "dup-rule", "resource_type": "HTTP推送", "data_resource_id": rid,
		"filter": map[string]interface{}{"select_name": "*"}}
	body, _ = json.Marshal(rule)
	assertSuccess(t, doRequest(env.engine, http.MethodPost, "/api/v1/rule-engine", env.token, body), "首次新增")
	assertFail(t, doRequest(env.engine, http.MethodPost, "/api/v1/rule-engine", env.token, body), "重名")
}

func assertRuleStatus(t *testing.T, env *testEnv, ruleId, want string) {
	t.Helper()
	w := doRequest(env.engine, http.MethodGet, "/api/v1/rule-engine?page=1&pageSize=10", env.token, nil)
	var listResp struct {
		Result struct {
			List []map[string]interface{} `json:"list"`
		} `json:"result"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &listResp)
	for _, it := range listResp.Result.List {
		if it["id"] == ruleId {
			if it["status"] != want {
				t.Fatalf("规则状态 = %v, want %s", it["status"], want)
			}
			return
		}
	}
	t.Fatalf("列表中未找到规则 %s", ruleId)
}
