package e2e

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"iotserver/internal/pkg/errort"
	"iotserver/internal/pkg/httphelper"
)

/** 场景联动 **/

type sceneListResult struct {
	Success bool `json:"success"`
	Result  struct {
		List  []map[string]interface{} `json:"list"`
		Total int                      `json:"total"`
	} `json:"result"`
}

// TestSceneFlow 场景联动全链路：新建 → 列表 → 编辑（设备触发，下发 eKuiper）→ 启动/停止 → 删除
func TestSceneFlow(t *testing.T) {
	env := setupTestEnv(t)
	fake := env.fakeEkuiper

	// 1) 新建场景（前端只提交名称与描述）
	body, _ := json.Marshal(map[string]interface{}{"name": "e2e场景一", "description": "端到端"})
	assertSuccess(t, doRequest(env.engine, http.MethodPost, "/api/v1/scene", env.token, body), "新建场景")

	// 2) 列表必须返回完整字段（编辑抽屉直接吃列表行）
	w := doRequest(env.engine, http.MethodGet, "/api/v1/scene?page=1&pageSize=10", env.token, nil)
	var list sceneListResult
	if err := json.Unmarshal(w.Body.Bytes(), &list); err != nil {
		t.Fatalf("decode scene list: %v", err)
	}
	if !list.Success || list.Result.Total != 1 {
		t.Fatalf("场景列表异常: success=%v total=%d", list.Success, list.Result.Total)
	}
	item := list.Result.List[0]
	sceneId, _ := item["id"].(string)
	if sceneId == "" {
		t.Fatal("场景 id 为空")
	}
	if item["name"] != "e2e场景一" || item["description"] != "端到端" {
		t.Fatalf("名称/描述回填字段缺失: %v", item)
	}
	if item["status"] != "stopped" {
		t.Fatalf("新建场景状态应为 stopped, got %v", item["status"])
	}
	if _, ok := item["conditions"].([]interface{}); !ok {
		t.Fatalf("conditions 应为数组, got %T", item["conditions"])
	}
	if _, ok := item["actions"].([]interface{}); !ok {
		t.Fatalf("actions 应为数组, got %T", item["actions"])
	}
	if created, _ := item["created"].(float64); created < 1e12 {
		t.Fatalf("created 应为毫秒时间戳, got %v", item["created"])
	}

	// 3) 编辑：设备数据触发（decide_condition 必须原样保存）
	update := map[string]interface{}{
		"id": sceneId, "name": "e2e场景一", "description": "端到端",
		"conditions": []map[string]interface{}{{
			"condition_type": "notify",
			"option": map[string]string{
				"trigger": "设备数据触发", "product_id": "p1", "product_name": "产品",
				"device_id": "d1", "device_name": "设备", "code": "temp", "type": "float",
				"value_type": "avg", "value_cycle": "1分钟周期", "decide_condition": "> 30",
			},
		}},
		"actions": []map[string]interface{}{{
			"product_id": "p1", "product_name": "产品", "device_id": "d1", "device_name": "设备",
			"code": "switch", "data_type": "bool", "value": "true",
		}},
	}
	body, _ = json.Marshal(update)
	assertSuccess(t, doRequest(env.engine, http.MethodPut, "/api/v1/scene", env.token, body), "编辑场景")

	// eKuiper 规则已下发，动作为回调平台
	if !fake.Called("create:%s", sceneId) {
		t.Fatal("未调用 eKuiper CreateRule")
	}
	fr := fake.rule(sceneId)
	if fr == nil || fr.Actions[0].Rest == nil {
		t.Fatalf("场景动作应指向平台回调, got %+v", fr)
	}
	if url, _ := fr.Actions[0].Rest["url"].(string); url == "" || !contains(url, "/api/v1/ekuiper/scene") {
		t.Fatalf("回调地址异常: %v", fr.Actions[0].Rest["url"])
	}

	// 回读：conditions/actions 与 decide_condition 原样保留
	w = doRequest(env.engine, http.MethodGet, "/api/v1/scene", env.token, nil)
	_ = json.Unmarshal(w.Body.Bytes(), &list)
	conds := list.Result.List[0]["conditions"].([]interface{})
	opt := conds[0].(map[string]interface{})["option"].(map[string]interface{})
	if opt["decide_condition"] != "> 30" {
		t.Fatalf("decide_condition 未原样保存: %v", opt["decide_condition"])
	}

	// 4) 启动（notify：StartRule）→ 状态 running
	assertSuccess(t, doRequest(env.engine, http.MethodPost, fmt.Sprintf("/api/v1/scene/%s/start", sceneId), env.token, nil), "启动场景")
	if !fake.Called("start:%s", sceneId) {
		t.Fatal("未调用 eKuiper StartRule")
	}
	assertSceneStatus(t, env, "running")

	// 5) 运行中编辑被拒（21400）
	body, _ = json.Marshal(update)
	w = doRequest(env.engine, http.MethodPut, "/api/v1/scene", env.token, body)
	var resp httphelper.CommonResponse
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Success || resp.ErrorCode != errort.SceneTimerIsStartingNotAllowUpdate {
		t.Fatalf("运行中编辑应返回 21400, got success=%v code=%d", resp.Success, resp.ErrorCode)
	}

	// 6) eKuiper 回调 → 执行动作 + 落日志
	//    设备不存在，动作执行应失败，但日志必须写入且接口本身返回成功
	cb := map[string]interface{}{"rule_id": sceneId, "deviceId": "d1", "temp": 35}
	body, _ = json.Marshal(cb)
	assertSuccess(t, doRequest(env.engine, http.MethodPost, "/api/v1/ekuiper/scene", "", body), "eKuiper 回调")

	w = doRequest(env.engine, http.MethodGet, fmt.Sprintf("/api/v1/scene/%s/log?page=1&pageSize=10", sceneId), env.token, nil)
	var logResp struct {
		Success bool `json:"success"`
		Result  struct {
			List  []map[string]interface{} `json:"list"`
			Total int                      `json:"total"`
		} `json:"result"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &logResp); err != nil {
		t.Fatalf("decode scene log: %v", err)
	}
	if !logResp.Success || logResp.Result.Total != 1 {
		t.Fatalf("场景日志异常: success=%v total=%d body=%s", logResp.Success, logResp.Result.Total, w.Body.String())
	}
	logItem := logResp.Result.List[0]
	if logItem["name"] != "e2e场景一" {
		t.Fatalf("日志 name 应为场景名, got %v", logItem["name"])
	}
	if logItem["exec_res"] == "" || logItem["exec_res"] == nil {
		t.Fatalf("日志 exec_res 为空")
	}
	if id, _ := logItem["id"].(string); id == "" {
		t.Fatal("日志 id 为空（前端 rowKey）")
	}
	if created, _ := logItem["created"].(float64); created < 1e12 {
		t.Fatalf("日志 created 应为毫秒, got %v", logItem["created"])
	}
	// 时间范围过滤：未来时间应查不到
	w = doRequest(env.engine, http.MethodGet,
		fmt.Sprintf("/api/v1/scene/%s/log?page=1&pageSize=10&start_time=%d", sceneId, 4102444800000), env.token, nil)
	_ = json.Unmarshal(w.Body.Bytes(), &logResp)
	if logResp.Result.Total != 0 {
		t.Fatalf("时间范围过滤无效, total=%d", logResp.Result.Total)
	}

	// 7) 停止 → 状态 stopped
	assertSuccess(t, doRequest(env.engine, http.MethodPost, fmt.Sprintf("/api/v1/scene/%s/stop", sceneId), env.token, nil), "停止场景")
	assertSceneStatus(t, env, "stopped")

	// 8) 删除（eKuiper 与库、日志都要清）
	assertSuccess(t, doRequest(env.engine, http.MethodDelete, fmt.Sprintf("/api/v1/scene/%s", sceneId), env.token, nil), "删除场景")
	if !fake.Called("delete:%s", sceneId) {
		t.Fatal("未调用 eKuiper DeleteRule")
	}
	w = doRequest(env.engine, http.MethodGet, "/api/v1/scene", env.token, nil)
	_ = json.Unmarshal(w.Body.Bytes(), &list)
	if list.Result.Total != 0 {
		t.Fatalf("删除后场景应为空, total=%d", list.Result.Total)
	}
	assertFail(t, doRequest(env.engine, http.MethodDelete, "/api/v1/scene/not-exist", env.token, nil), "删除不存在")
}

// TestSceneTimerAndParamErrors 定时场景与参数校验
func TestSceneTimerAndParamErrors(t *testing.T) {
	env := setupTestEnv(t)
	fake := env.fakeEkuiper

	// 名称非法
	body, _ := json.Marshal(map[string]interface{}{"name": "1bad"})
	assertFail(t, doRequest(env.engine, http.MethodPost, "/api/v1/scene", env.token, body), "名称非法")

	// 建场景
	body, _ = json.Marshal(map[string]interface{}{"name": "定时场景"})
	assertSuccess(t, doRequest(env.engine, http.MethodPost, "/api/v1/scene", env.token, body), "新建")
	w := doRequest(env.engine, http.MethodGet, "/api/v1/scene", env.token, nil)
	var list sceneListResult
	_ = json.Unmarshal(w.Body.Bytes(), &list)
	sceneId, _ := list.Result.List[0]["id"].(string)

	// 条件为空 → 参数错误
	body, _ = json.Marshal(map[string]interface{}{"id": sceneId, "name": "定时场景"})
	assertFail(t, doRequest(env.engine, http.MethodPut, "/api/v1/scene", env.token, body), "无触发条件")

	// 定时表达式非法 → 参数错误
	bad := map[string]interface{}{
		"id": sceneId, "name": "定时场景",
		"conditions": []map[string]interface{}{{"condition_type": "timer",
			"option": map[string]string{"cron_expression": "not a cron"}}},
		"actions": []map[string]interface{}{{"device_id": "d1", "code": "switch", "value": "true"}},
	}
	body, _ = json.Marshal(bad)
	assertFail(t, doRequest(env.engine, http.MethodPut, "/api/v1/scene", env.token, body), "非法定时表达式")

	// 合法定时：cron 原样保存，且不下发 eKuiper（由平台调度器执行）
	ok := map[string]interface{}{
		"id": sceneId, "name": "定时场景",
		"conditions": []map[string]interface{}{{"condition_type": "timer",
			"option": map[string]string{"cron_expression": "01 02 * * 0,1"}}},
		"actions": []map[string]interface{}{{"device_id": "d1", "code": "switch", "value": "true"}},
	}
	body, _ = json.Marshal(ok)
	assertSuccess(t, doRequest(env.engine, http.MethodPut, "/api/v1/scene", env.token, body), "合法定时")
	w = doRequest(env.engine, http.MethodGet, "/api/v1/scene", env.token, nil)
	_ = json.Unmarshal(w.Body.Bytes(), &list)
	conds := list.Result.List[0]["conditions"].([]interface{})
	cron := conds[0].(map[string]interface{})["option"].(map[string]interface{})["cron_expression"]
	if cron != "01 02 * * 0,1" {
		t.Fatalf("cron_expression 未原样保存: %v", cron)
	}
	if fake.Called("create:%s", sceneId) {
		t.Fatal("定时场景不应下发 eKuiper 规则")
	}

	// 定时场景启动/停止（走平台调度器，不调用 eKuiper）
	assertSuccess(t, doRequest(env.engine, http.MethodPost, fmt.Sprintf("/api/v1/scene/%s/start", sceneId), env.token, nil), "启动定时场景")
	assertSceneStatus(t, env, "running")
	assertSuccess(t, doRequest(env.engine, http.MethodPost, fmt.Sprintf("/api/v1/scene/%s/stop", sceneId), env.token, nil), "停止定时场景")
	assertSceneStatus(t, env, "stopped")

	// 缺少执行动作
	noAction := map[string]interface{}{
		"id": sceneId, "name": "定时场景",
		"conditions": []map[string]interface{}{{"condition_type": "timer",
			"option": map[string]string{"cron_expression": "01 02 * * 0"}}},
	}
	body, _ = json.Marshal(noAction)
	assertFail(t, doRequest(env.engine, http.MethodPut, "/api/v1/scene", env.token, body), "缺少动作")

	// 回调缺少 rule_id
	body, _ = json.Marshal(map[string]interface{}{"foo": "bar"})
	assertFail(t, doRequest(env.engine, http.MethodPost, "/api/v1/ekuiper/scene", "", body), "回调缺 rule_id")
}

func assertSceneStatus(t *testing.T, env *testEnv, want string) {
	t.Helper()
	w := doRequest(env.engine, http.MethodGet, "/api/v1/scene", env.token, nil)
	var list sceneListResult
	_ = json.Unmarshal(w.Body.Bytes(), &list)
	if len(list.Result.List) == 0 {
		t.Fatal("场景列表为空")
	}
	if list.Result.List[0]["status"] != want {
		t.Fatalf("场景状态 = %v, want %s", list.Result.List[0]["status"], want)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
