package e2e

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"iotserver/internal/dtos"
)

// P0-1：sys_config 全局唯一一份（无 tenant_id），写接口收口到平台管理员。
//   - 租户管理员天然拥有 system:config:* 权限码，但写接口被 PlatformOnly 拒绝；
//   - 读接口保留在 v1Auth，租户仍可读。
//
// P0-2：6 条「废弃」写路由（无权限校验、写全局字典表）已摘除，
// 调用应落到 NoRoute 返回「url path is invalid」。

const errCodePlatformAdminOnly = 21708

func sysConfigAddBody(configKey string) []byte {
	b, _ := json.Marshal(dtos.SysConfigAddRequest{
		ConfigKey: configKey, Name: "e2e配置", ConfigValue: "1", GroupName: "e2e", Type: "string",
	})
	return b
}

// sysConfigTotal 按 configKey 查询全局配置条数（用平台管理员 token）
func sysConfigTotal(t *testing.T, env *testEnv, token, configKey string) int {
	t.Helper()
	w := doRequest(env.engine, http.MethodGet,
		"/api/v1/system-setup?isAll=true&configKey="+configKey, token, nil)
	resp := parseResponse(t, w)
	if !resp.Success {
		t.Fatalf("查询系统配置失败: %v", resp.ErrorMsg)
	}
	page, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatalf("系统配置查询返回类型异常: %T", resp.Result)
	}
	total, _ := page["total"].(float64)
	return int(total)
}

// TestTenantAdminCannotWriteSystemSetup 租户管理员调 4 个写接口都必须被拒绝（平台专属）
func TestTenantAdminCannotWriteSystemSetup(t *testing.T) {
	env := setupTestEnv(t)
	_, username := seedTenant(t, env, "cfgtenant")
	token := loginAs(t, env, username, tenantTestPassword)

	const key = "e2e.tenant.try"
	cases := []struct {
		method string
		path   string
		body   []byte
	}{
		{http.MethodPost, "/api/v1/system-setup", sysConfigAddBody(key)},
		{http.MethodPut, "/api/v1/system-setup", []byte(`{"id":"1","configKey":"` + key + `","name":"x"}`)},
		{http.MethodDelete, "/api/v1/system-setup", []byte(`{"ids":["1"]}`)},
		{http.MethodPost, "/api/v1/system-setup/save", []byte(`{"items":[{"configKey":"` + key + `","configValue":"1"}]}`)},
	}
	for _, c := range cases {
		w := doRequest(env.engine, c.method, c.path, token, c.body)
		resp := parseResponse(t, w)
		if resp.Success {
			t.Fatalf("%s %s 租户管理员不应调用成功", c.method, c.path)
		}
		if resp.ErrorCode != errCodePlatformAdminOnly {
			t.Fatalf("%s %s 期望平台专属错误 %d，实际 %d（%s）",
				c.method, c.path, errCodePlatformAdminOnly, resp.ErrorCode, resp.ErrorMsg)
		}
	}

	// 被拒绝的尝试不能真的写进全局配置表
	if total := sysConfigTotal(t, env, env.token, key); total != 0 {
		t.Fatalf("租户写配置被拒后不应有落库数据，实际 %d 条", total)
	}
}

// TestPlatformAdminCanWriteSystemSetup 平台管理员调同一批写接口应成功（增 / 改 / 删）
func TestPlatformAdminCanWriteSystemSetup(t *testing.T) {
	env := setupTestEnv(t)

	const key = "e2e.platform.cfg"
	w := doRequest(env.engine, http.MethodPost, "/api/v1/system-setup", env.token, sysConfigAddBody(key))
	if resp := parseResponse(t, w); !resp.Success {
		t.Fatalf("平台管理员新增配置失败: %v", resp.ErrorMsg)
	}

	// 查到 id
	w = doRequest(env.engine, http.MethodGet, "/api/v1/system-setup?isAll=true&configKey="+key, env.token, nil)
	resp := parseResponse(t, w)
	if !resp.Success {
		t.Fatalf("平台管理员查询配置失败: %v", resp.ErrorMsg)
	}
	page := resp.Result.(map[string]interface{})
	list, _ := page["list"].([]interface{})
	if len(list) != 1 {
		t.Fatalf("期望新增后查到 1 条，实际 %d 条", len(list))
	}
	id, _ := list[0].(map[string]interface{})["id"].(string)
	if id == "" {
		t.Fatal("配置 id 为空")
	}

	// 修改
	upd, _ := json.Marshal(dtos.SysConfigUpdateRequest{Id: id, ConfigKey: key, Name: "e2e配置-改", ConfigValue: "2"})
	w = doRequest(env.engine, http.MethodPut, "/api/v1/system-setup", env.token, upd)
	if resp = parseResponse(t, w); !resp.Success {
		t.Fatalf("平台管理员修改配置失败: %v", resp.ErrorMsg)
	}

	// 删除
	del, _ := json.Marshal(dtos.SysConfigIdsRequest{Ids: []string{id}})
	w = doRequest(env.engine, http.MethodDelete, "/api/v1/system-setup", env.token, del)
	if resp = parseResponse(t, w); !resp.Success {
		t.Fatalf("平台管理员删除配置失败: %v", resp.ErrorMsg)
	}
	if total := sysConfigTotal(t, env, env.token, key); total != 0 {
		t.Fatalf("删除后不应再查到配置，实际 %d 条", total)
	}
}

// TestTenantCanReadSystemSetup 读接口保留给租户：列表仍可读；
// 同时验证 seed 收口：租户侧不再下发「系统配置」菜单（PlatformOnly=1）。
func TestTenantCanReadSystemSetup(t *testing.T) {
	env := setupTestEnv(t)
	_, username := seedTenant(t, env, "cfgreader")
	token := loginAs(t, env, username, tenantTestPassword)

	w := doRequest(env.engine, http.MethodGet, "/api/v1/system-setup?isAll=true", token, nil)
	resp := parseResponse(t, w)
	if !resp.Success {
		t.Fatalf("租户读取系统配置不应被拒绝: %v（code=%d）", resp.ErrorMsg, resp.ErrorCode)
	}

	// 菜单收口：租户的菜单树里既不应有「平台管理」目录，也不应有「系统配置」
	w = doRequest(env.engine, http.MethodGet, "/api/v1/user/getinfo", token, nil)
	resp = parseResponse(t, w)
	if !resp.Success {
		t.Fatalf("租户 getinfo 失败: %v", resp.ErrorMsg)
	}
	tenantMenus, _ := resp.Result.(map[string]interface{})["menus"].([]interface{})
	if menuTreeHasPath(tenantMenus, "/platform/sysSet") {
		t.Fatal("租户侧不应下发「系统配置」菜单（PlatformOnly=1）")
	}
	if menuTreeHasPath(tenantMenus, "/platform") {
		t.Fatal("租户侧不应下发「平台管理」目录（整组 PlatformOnly=1）")
	}

	// 平台管理员：仍可读配置，且菜单仍在
	w = doRequest(env.engine, http.MethodGet, "/api/v1/system-setup?isAll=true", env.token, nil)
	if resp = parseResponse(t, w); !resp.Success {
		t.Fatalf("平台管理员读取系统配置失败: %v", resp.ErrorMsg)
	}
	w = doRequest(env.engine, http.MethodGet, "/api/v1/user/getinfo", env.token, nil)
	resp = parseResponse(t, w)
	if !resp.Success {
		t.Fatalf("平台管理员 getinfo 失败: %v", resp.ErrorMsg)
	}
	platformMenus, _ := resp.Result.(map[string]interface{})["menus"].([]interface{})
	// 平台管理员：可见「平台管理」目录，其下同时挂租户管理与系统配置
	if !menuTreeHasPath(platformMenus, "/platform") {
		t.Fatal("平台管理员应可见「平台管理」目录")
	}
	if !menuTreeHasPath(platformMenus, "/platform/sysSet") {
		t.Fatal("平台管理员应可见「系统配置」菜单")
	}
	if !menuTreeHasPath(platformMenus, "/platform/tenant") {
		t.Fatal("平台管理员应可见「租户管理」菜单")
	}
	if menuTreeHasPath(platformMenus, "/admin/sysSet") {
		t.Fatal("「系统配置」已移出「系统管理」，不应再挂在 /admin 下")
	}
}

// menuTreeHasPath 在菜单树（children 递归）里查找指定路由地址
func menuTreeHasPath(nodes []interface{}, path string) bool {
	for _, node := range nodes {
		m, ok := node.(map[string]interface{})
		if !ok {
			continue
		}
		if m["path"] == path {
			return true
		}
		if children, ok := m["children"].([]interface{}); ok && menuTreeHasPath(children, path) {
			return true
		}
	}
	return false
}

// TestDeprecatedWriteRoutesRemoved 6 条废弃写路由不再注册，调用返回路径无效
func TestDeprecatedWriteRoutesRemoved(t *testing.T) {
	env := setupTestEnv(t)

	paths := []string{
		"/api/v1/thingmodel/unit-sync",
		"/api/v1/thingmodel/docs-sync",
		"/api/v1/thingmodel/quicknavigation-sync",
		"/api/v1/category-template/sync",
		"/api/v1/thingmodel-template/sync",
		"/api/v1/language-sdk-sync",
	}
	for _, path := range paths {
		w := doRequest(env.engine, http.MethodPost, path, env.token, []byte(`{}`))
		var resp struct {
			Code    int    `json:"code"`
			Msg     string `json:"msg"`
			Success bool   `json:"success"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("POST %s 解码响应失败: %v（%s）", path, err, w.Body.String())
		}
		if resp.Success {
			t.Fatalf("POST %s 路由应已摘除，实际调用成功", path)
		}
		if !strings.Contains(strings.ToLower(resp.Msg), "invalid") {
			t.Fatalf("POST %s 期望「路径无效」，实际 code=%d msg=%q", path, resp.Code, resp.Msg)
		}
	}
}
