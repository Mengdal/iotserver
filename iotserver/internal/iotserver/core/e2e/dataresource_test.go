package e2e

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"iotserver/internal/pkg/httphelper"
)

// TestDataResourceFlow 高级能力-资源管理全链路：类型 → 增 → 查 → 验证 → 改 → 删
func TestDataResourceFlow(t *testing.T) {
	env := setupTestEnv(t)

	// 目标服务（用于验证成功）
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer target.Close()

	// 1) 类型列表
	w := doRequest(env.engine, http.MethodGet, "/api/v1/typeresource", env.token, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("typeresource status %d", w.Code)
	}
	var typeResp struct {
		Success bool     `json:"success"`
		Result  []string `json:"result"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &typeResp); err != nil {
		t.Fatalf("decode typeresource: %v", err)
	}
	if !typeResp.Success || len(typeResp.Result) != 4 {
		t.Fatalf("类型列表异常: success=%v len=%d", typeResp.Success, len(typeResp.Result))
	}

	// 2) 新增
	add := map[string]interface{}{
		"name": "e2e-http-push",
		"type": "HTTP推送",
		"option": map[string]interface{}{
			"url":    target.URL + "/push",
			"method": "post",
		},
	}
	body, _ := json.Marshal(add)
	w = doRequest(env.engine, http.MethodPost, "/api/v1/dataresource", env.token, body)
	assertSuccess(t, w, "add")

	// 3) 列表（含分页与类型过滤）
	w = doRequest(env.engine, http.MethodGet, "/api/v1/dataresource?type=HTTP推送&page=1&pageSize=10", env.token, nil)
	var pageResp struct {
		Success bool `json:"success"`
		Result  struct {
			List  []map[string]interface{} `json:"list"`
			Total int                      `json:"total"`
		} `json:"result"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &pageResp); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if !pageResp.Success || pageResp.Result.Total != 1 || len(pageResp.Result.List) != 1 {
		t.Fatalf("列表异常: success=%v total=%d len=%d", pageResp.Success, pageResp.Result.Total, len(pageResp.Result.List))
	}
	item := pageResp.Result.List[0]
	id, _ := item["id"].(string)
	if id == "" {
		t.Fatal("id 为空")
	}
	if _, ok := item["option"].(map[string]interface{}); !ok {
		t.Fatalf("option 应为对象, got %T", item["option"])
	}
	if item["health"] != nil {
		t.Fatalf("未验证时 health 应为 null, got %v", item["health"])
	}

	// 其它类型过滤应为空
	w = doRequest(env.engine, http.MethodGet, "/api/v1/dataresource?type=InfluxDB", env.token, nil)
	_ = json.Unmarshal(w.Body.Bytes(), &pageResp)
	if pageResp.Result.Total != 0 {
		t.Fatalf("按类型过滤异常: total=%d", pageResp.Result.Total)
	}

	// 4) 验证成功
	w = doRequest(env.engine, http.MethodPost, fmt.Sprintf("/api/v1/dataresource/%s/health", id), env.token, nil)
	assertVerify(t, w, true)
	w = doRequest(env.engine, http.MethodGet, "/api/v1/dataresource?type=HTTP推送", env.token, nil)
	_ = json.Unmarshal(w.Body.Bytes(), &pageResp)
	if pageResp.Result.List[0]["health"] != true {
		t.Fatalf("验证成功后 health 应为 true, got %v", pageResp.Result.List[0]["health"])
	}

	// 5) 改成不可达地址 → 验证失败
	update := map[string]interface{}{
		"id": id, "name": "e2e-http-push-2", "type": "HTTP推送",
		"option": map[string]interface{}{"url": "http://127.0.0.1:59995/x", "method": "post"},
	}
	body, _ = json.Marshal(update)
	w = doRequest(env.engine, http.MethodPut, "/api/v1/dataresource", env.token, body)
	assertSuccess(t, w, "update")
	w = doRequest(env.engine, http.MethodPost, fmt.Sprintf("/api/v1/dataresource/%s/health", id), env.token, nil)
	assertVerify(t, w, false)

	// 6) 校验类错误
	bad := map[string]interface{}{"name": "1bad", "type": "HTTP推送", "option": map[string]interface{}{"url": "http://x"}}
	body, _ = json.Marshal(bad)
	w = doRequest(env.engine, http.MethodPost, "/api/v1/dataresource", env.token, body)
	assertFail(t, w, "名称非法应失败")

	bad = map[string]interface{}{"name": "ok-name", "type": "不支持的类型", "option": map[string]interface{}{}}
	body, _ = json.Marshal(bad)
	w = doRequest(env.engine, http.MethodPost, "/api/v1/dataresource", env.token, body)
	assertFail(t, w, "类型非法应失败")

	bad = map[string]interface{}{"name": "ok-name", "type": "HTTP推送", "option": map[string]interface{}{}}
	body, _ = json.Marshal(bad)
	w = doRequest(env.engine, http.MethodPost, "/api/v1/dataresource", env.token, body)
	assertFail(t, w, "缺必填应失败")

	// 重名（同类型）
	dup := map[string]interface{}{"name": "e2e-http-push-2", "type": "HTTP推送",
		"option": map[string]interface{}{"url": "http://127.0.0.1:1"}}
	body, _ = json.Marshal(dup)
	w = doRequest(env.engine, http.MethodPost, "/api/v1/dataresource", env.token, body)
	assertFail(t, w, "同类型重名应失败")

	// 7) 删除
	w = doRequest(env.engine, http.MethodDelete, fmt.Sprintf("/api/v1/dataresource/%s", id), env.token, nil)
	assertSuccess(t, w, "delete")
	w = doRequest(env.engine, http.MethodGet, "/api/v1/dataresource?type=HTTP推送", env.token, nil)
	_ = json.Unmarshal(w.Body.Bytes(), &pageResp)
	if pageResp.Result.Total != 0 {
		t.Fatalf("删除后应为空, total=%d", pageResp.Result.Total)
	}

	// 删除不存在 / 验证不存在
	w = doRequest(env.engine, http.MethodDelete, "/api/v1/dataresource/not-exist", env.token, nil)
	assertFail(t, w, "删除不存在应失败")
	w = doRequest(env.engine, http.MethodPost, "/api/v1/dataresource/not-exist/health", env.token, nil)
	assertFail(t, w, "验证不存在应失败")
}

func assertSuccess(t *testing.T, w *httptest.ResponseRecorder, action string) {
	t.Helper()
	var resp httphelper.CommonResponse
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if !resp.Success {
		t.Fatalf("%s 失败: %s", action, resp.ErrorMsg)
	}
}

// assertFail 断言接口返回失败且带错误码
func assertFail(t *testing.T, w *httptest.ResponseRecorder, action string) {
	t.Helper()
	var resp httphelper.CommonResponse
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Success {
		t.Fatalf("%s：期望失败但成功了", action)
	}
	if resp.ErrorCode == 0 {
		t.Fatalf("%s：失败但错误码为 0", action)
	}
}

// assertVerify 断言验证接口返回成功且健康状态符合预期
func assertVerify(t *testing.T, w *httptest.ResponseRecorder, wantHealth bool) {
	t.Helper()
	var resp struct {
		Success bool `json:"success"`
		Result  struct {
			Health  bool   `json:"health"`
			Message string `json:"message"`
		} `json:"result"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode verify: %v", err)
	}
	if !resp.Success {
		t.Fatalf("验证接口应始终成功返回, got body=%s", w.Body.String())
	}
	if resp.Result.Health != wantHealth {
		t.Fatalf("health = %v, want %v (message=%s)", resp.Result.Health, wantHealth, resp.Result.Message)
	}
}
