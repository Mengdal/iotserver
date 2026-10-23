package e2e

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"iotserver/internal/models"
)

// 场景联动：租户只能引用自己租户的设备/产品，引用其他租户的设备要被拒绝。
func TestSceneRejectsCrossTenantDevice(t *testing.T) {
	env := setupTestEnv(t)

	tenantA, _ := seedTenant(t, env, "scene_a")
	tenantB, userB := seedTenant(t, env, "scene_b")
	tokenB := loginAs(t, env, userB, tenantTestPassword)

	// 租户 A 的产品与设备
	seedTenantProduct(t, env, tenantA.Id, "scene_p1")
	deviceA, err := env.client.WithCtx(tenantCtxOf(tenantA.Id)).AddDevice(models.Device{
		Name: "scene_d1", ProductId: "scene_p1",
	})
	if err != nil {
		t.Fatalf("create device A: %v", err)
	}

	// 租户 B 新建一个场景
	body, _ := json.Marshal(map[string]interface{}{"name": "cross-tenant", "description": ""})
	assertSuccess(t, doRequest(env.engine, http.MethodPost, "/api/v1/scene", tokenB, body), "新建场景")
	w := doRequest(env.engine, http.MethodGet, "/api/v1/scene?page=1&pageSize=10", tokenB, nil)
	var list sceneListResult
	if err := json.Unmarshal(w.Body.Bytes(), &list); err != nil {
		t.Fatalf("decode scene list: %v", err)
	}
	if !list.Success || len(list.Result.List) == 0 {
		t.Fatalf("场景列表异常: %s", w.Body.String())
	}
	sceneId, _ := list.Result.List[0]["id"].(string)

	// 引用租户 A 的设备 → 必须被拒绝
	update := map[string]interface{}{
		"id": sceneId, "name": "cross-tenant", "description": "",
		"conditions": []map[string]interface{}{{
			"condition_type": "notify",
			"option": map[string]string{
				"trigger": "设备数据触发", "product_id": "scene_p1", "device_id": deviceA,
				"code": "temp", "type": "float", "value_type": "avg",
				"value_cycle": "1分钟周期", "decide_condition": "> 1",
			},
		}},
		"actions": []map[string]interface{}{{
			"product_id": "scene_p1", "device_id": deviceA, "code": "switch",
			"data_type": "bool", "value": "true",
		}},
	}
	body, _ = json.Marshal(update)
	w = doRequest(env.engine, http.MethodPut, "/api/v1/scene", tokenB, body)
	resp := parseResponse(t, w)
	if resp.Success {
		t.Fatal("引用其他租户设备的场景不应保存成功")
	}
	if !strings.Contains(resp.ErrorMsg, "不属于当前租户") &&
		!strings.Contains(resp.ErrorMsg, "does not belong") {
		t.Fatalf("错误信息应提示设备归属: %s", resp.ErrorMsg)
	}

	// 换成租户 B 自己的设备 → 允许保存
	seedTenantProduct(t, env, tenantB.Id, "scene_p2")
	deviceB, err := env.client.WithCtx(tenantCtxOf(tenantB.Id)).AddDevice(models.Device{
		Name: "scene_d2", ProductId: "scene_p2",
	})
	if err != nil {
		t.Fatalf("create device B: %v", err)
	}
	update["conditions"] = []map[string]interface{}{{
		"condition_type": "notify",
		"option": map[string]string{
			"trigger": "设备数据触发", "product_id": "scene_p2", "device_id": deviceB,
			"code": "temp", "type": "float", "value_type": "avg",
			"value_cycle": "1分钟周期", "decide_condition": "> 1",
		},
	}}
	update["actions"] = []map[string]interface{}{{
		"product_id": "scene_p2", "device_id": deviceB, "code": "switch",
		"data_type": "bool", "value": "true",
	}}
	body, _ = json.Marshal(update)
	assertSuccess(t, doRequest(env.engine, http.MethodPut, "/api/v1/scene", tokenB, body), "保存本租户场景")
}

// 规则引擎：租户规则必须强制带 tenantId 过滤，且不能绑定其他租户的数据资源。
func TestRuleEngineTenantIsolation(t *testing.T) {
	env := setupTestEnv(t)

	tenantA, userA := seedTenant(t, env, "rule_a")
	tenantB, _ := seedTenant(t, env, "rule_b")
	tokenA := loginAs(t, env, userA, tenantTestPassword)

	resourceA, err := env.client.WithCtx(tenantCtxOf(tenantA.Id)).AddDataResource(models.DataResource{
		Id: "rule_res_a", Name: "http-a", Type: "HTTP推送",
		Option: models.DataResourceOption{"url": "http://127.0.0.1:1/push"},
	})
	if err != nil {
		t.Fatalf("create resource A: %v", err)
	}
	_, err = env.client.WithCtx(tenantCtxOf(tenantB.Id)).AddDataResource(models.DataResource{
		Id: "rule_res_b", Name: "http-b", Type: "HTTP推送",
		Option: models.DataResourceOption{"url": "http://127.0.0.1:1/push"},
	})
	if err != nil {
		t.Fatalf("create resource B: %v", err)
	}

	body, _ := json.Marshal(map[string]interface{}{
		"name": "rule-a", "description": "",
		"resource_type": "HTTP推送", "data_resource_id": resourceA.Id,
		"filter": map[string]interface{}{
			"message_source": "消息总线", "select_name": "deviceId",
			"sql": `SELECT deviceId FROM mqtt_stream`,
		},
	})
	assertSuccess(t, doRequest(env.engine, http.MethodPost, "/api/v1/rule-engine", tokenA, body), "新增规则")

	w := doRequest(env.engine, http.MethodGet, "/api/v1/rule-engine?page=1&pageSize=10", tokenA, nil)
	var list struct {
		Success bool `json:"success"`
		Result  struct {
			List []map[string]interface{} `json:"list"`
		} `json:"result"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &list); err != nil || !list.Success || len(list.Result.List) != 1 {
		t.Fatalf("规则列表异常: %s", w.Body.String())
	}
	filter := list.Result.List[0]["filter"].(map[string]interface{})
	sql, _ := filter["sql"].(string)
	if !strings.Contains(sql, "tenantId = '"+tenantA.Id+"'") {
		t.Fatalf("规则 SQL 未注入租户过滤: %s", sql)
	}

	// 绑定其他租户的资源必须失败
	body, _ = json.Marshal(map[string]interface{}{
		"name": "rule-cross", "description": "",
		"resource_type": "HTTP推送", "data_resource_id": "rule_res_b",
		"filter": map[string]interface{}{
			"message_source": "消息总线", "select_name": "deviceId",
			"sql": `SELECT deviceId FROM mqtt_stream`,
		},
	})
	w = doRequest(env.engine, http.MethodPost, "/api/v1/rule-engine", tokenA, body)
	if resp := parseResponse(t, w); resp.Success {
		t.Fatal("不应允许租户绑定其他租户的数据资源")
	}
}

// 场景联动：eKuiper 回调没有请求租户，执行日志仍必须归属场景所在租户，
// 否则租户在「场景日志」页永远看不到自己的执行记录。
func TestSceneLogKeepsTenant(t *testing.T) {
	env := setupTestEnv(t)

	tenantA, userA := seedTenant(t, env, "scene_log_a")
	tokenA := loginAs(t, env, userA, tenantTestPassword)

	seedTenantProduct(t, env, tenantA.Id, "scene_log_p1")
	device, err := env.client.WithCtx(tenantCtxOf(tenantA.Id)).AddDevice(models.Device{
		Name: "scene_log_d1", ProductId: "scene_log_p1",
	})
	if err != nil {
		t.Fatalf("create device: %v", err)
	}

	body, _ := json.Marshal(map[string]interface{}{"name": "scene-log", "description": ""})
	assertSuccess(t, doRequest(env.engine, http.MethodPost, "/api/v1/scene", tokenA, body), "新建场景")
	w := doRequest(env.engine, http.MethodGet, "/api/v1/scene?page=1&pageSize=10", tokenA, nil)
	var list sceneListResult
	if err := json.Unmarshal(w.Body.Bytes(), &list); err != nil || !list.Success || len(list.Result.List) == 0 {
		t.Fatalf("场景列表异常: %s", w.Body.String())
	}
	sceneId, _ := list.Result.List[0]["id"].(string)

	update := map[string]interface{}{
		"id": sceneId, "name": "scene-log", "description": "",
		"conditions": []map[string]interface{}{{
			"condition_type": "notify",
			"option": map[string]string{
				"trigger": "设备数据触发", "product_id": "scene_log_p1", "device_id": device,
				"code": "temp", "type": "float", "value_type": "avg",
				"value_cycle": "1分钟周期", "decide_condition": "> 1",
			},
		}},
		"actions": []map[string]interface{}{{
			"product_id": "scene_log_p1", "device_id": device, "code": "switch",
			"data_type": "bool", "value": "true",
		}},
	}
	body, _ = json.Marshal(update)
	assertSuccess(t, doRequest(env.engine, http.MethodPut, "/api/v1/scene", tokenA, body), "保存场景")
	assertSuccess(t, doRequest(env.engine, http.MethodPost, fmt.Sprintf("/api/v1/scene/%s/start", sceneId), tokenA, nil), "启动场景")

	body, _ = json.Marshal(map[string]interface{}{"rule_id": sceneId})
	assertSuccess(t, doRequest(env.engine, http.MethodPost, "/api/v1/ekuiper/scene", "", body), "eKuiper 回调")

	w = doRequest(env.engine, http.MethodGet, fmt.Sprintf("/api/v1/scene/%s/log?page=1&pageSize=10", sceneId), tokenA, nil)
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
		t.Fatalf("租户应能看到自己的场景日志: success=%v total=%d body=%s",
			logResp.Success, logResp.Result.Total, w.Body.String())
	}
	if logResp.Result.List[0]["name"] != "scene-log" {
		t.Fatalf("日志名称异常: %v", logResp.Result.List[0]["name"])
	}
}
