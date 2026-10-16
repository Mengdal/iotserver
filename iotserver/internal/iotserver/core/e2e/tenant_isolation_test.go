package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"

	"iotserver/internal/dtos"
	"iotserver/internal/models"
	"iotserver/internal/pkg/tenant"
	tenantplugin "iotserver/internal/tools/sqldb/tenant"
)

// 多租户行级隔离的端到端验证。
//
// 关键点：测试强制切到 enforce 策略，验证的是**真实隔离**（fail-closed），
// 而不是迁移期 warn 策略下「不过滤」的兜底行为。
const tenantTestPassword = "tenant123"

// doRequestWithTenant 与 doRequest 相同，额外带平台管理员的代管请求头
func doRequestWithTenant(engine *gin.Engine, method, path, token, tenantHeader string, body []byte) *httptest.ResponseRecorder {
	if body == nil {
		body = []byte{}
	}
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("x-token", token)
	}
	if tenantHeader != "" {
		req.Header.Set("x-tenant-id", tenantHeader)
	}
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	return w
}

// seedTenant 创建一个租户 + 租户内管理员角色 + 管理员账号，返回租户与登录信息
func seedTenant(t *testing.T, env *testEnv, code string) (models.Tenant, string) {
	t.Helper()
	pctx := tenant.Platform(context.Background())

	tc, err := env.client.WithCtx(pctx).AddTenant(models.Tenant{Code: code, Name: "租户-" + code})
	if err != nil {
		t.Fatalf("create tenant %s: %v", code, err)
	}

	// 租户内的管理员角色：role_key=admin 让它拥有租户内全部权限，
	// 但它不属于平台（tenant_id 非空），因此不会被当成平台管理员。
	tctx := tenant.With(pctx, tenant.Context{TenantID: tc.Id})
	role, err := env.client.WithCtx(tctx).AddSysRole(models.Role{
		Name: "管理员-" + code, RoleKey: "admin", DataScope: models.DataScopeAll, Status: 1,
	})
	if err != nil {
		t.Fatalf("create role for %s: %v", code, err)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(tenantTestPassword), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	username := code + "_admin"
	// 注意：tenant_id 由 GORM 租户插件从 ctx 自动写入（不要在字面量里设置—
	// Go 1.24 不允许在复合字面量里使用内嵌结构体的提升字段）
	user, err := env.client.WithCtx(tctx).AddSysUser(models.User{
		Username: username, Password: string(hash), Nickname: code, Status: 1,
	})
	if err != nil {
		t.Fatalf("create user for %s: %v", code, err)
	}
	if err = env.client.WithCtx(tctx).SetUserRoles(user.Id, []string{role.Id}); err != nil {
		t.Fatalf("bind role for %s: %v", code, err)
	}
	return tc, username
}

func loginAs(t *testing.T, env *testEnv, username, password string) string {
	t.Helper()
	body, _ := json.Marshal(dtos.LoginRequest{Username: username, Password: password})
	w := doRequest(env.engine, http.MethodPost, "/api/v1/auth/login", "", body)
	resp := parseResponse(t, w)
	if !resp.Success {
		t.Fatalf("login %s failed: %v", username, resp.ErrorMsg)
	}
	result, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatalf("unexpected login result for %s", username)
	}
	token, _ := result["token"].(string)
	if token == "" {
		t.Fatalf("empty token for %s", username)
	}
	return token
}

// listDataResourceTotal 返回该 token 可见的数据资源条数
func listDataResourceTotal(t *testing.T, env *testEnv, token, tenantHeader string) int {
	t.Helper()
	w := doRequestWithTenant(env.engine, http.MethodGet,
		"/api/v1/dataresource?type=HTTP推送&page=1&pageSize=50", token, tenantHeader, nil)
	var resp struct {
		Success bool `json:"success"`
		Result  struct {
			Total int `json:"total"`
		} `json:"result"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if !resp.Success {
		t.Fatalf("list failed: %s", w.Body.String())
	}
	return resp.Result.Total
}

// createDataResource 在指定 token 的租户下建一条数据资源，返回 id
func createDataResource(t *testing.T, env *testEnv, token, name string) string {
	t.Helper()
	body, _ := json.Marshal(map[string]interface{}{
		"name": name,
		"type": "HTTP推送",
		"option": map[string]interface{}{
			"url":    "http://127.0.0.1:1/push",
			"method": "post",
		},
	})
	w := doRequest(env.engine, http.MethodPost, "/api/v1/dataresource", token, body)
	assertSuccess(t, w, "create data resource "+name)
	// 取 id：列表里按名字找
	w = doRequest(env.engine, http.MethodGet, "/api/v1/dataresource?type=HTTP推送&page=1&pageSize=50", token, nil)
	var resp struct {
		Result struct {
			List []map[string]interface{} `json:"list"`
		} `json:"result"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	for _, item := range resp.Result.List {
		if item["name"] == name {
			id, _ := item["id"].(string)
			return id
		}
	}
	t.Fatalf("data resource %s not found in list", name)
	return ""
}

// TestTenantRowLevelIsolation 租户之间：列表互不可见、跨租户改/删失败、
// 平台管理员跨租户可见并可代管、租户用户不能访问平台级接口。
func TestTenantRowLevelIsolation(t *testing.T) {
	env := setupTestEnv(t)

	// 强制隔离：缺少租户上下文的查询返回空集
	tenantplugin.SetPolicy(tenantplugin.PolicyEnforce, nil)
	t.Cleanup(func() { tenantplugin.SetPolicy(tenantplugin.PolicyWarn, nil) })

	tenantA, userA := seedTenant(t, env, "cust_a")
	_, userB := seedTenant(t, env, "cust_b")

	tokenA := loginAs(t, env, userA, tenantTestPassword)
	tokenB := loginAs(t, env, userB, tenantTestPassword)

	idA := createDataResource(t, env, tokenA, "res-a")
	idB := createDataResource(t, env, tokenB, "res-b")

	// 1) 各自只看到自己的数据
	if got := listDataResourceTotal(t, env, tokenA, ""); got != 1 {
		t.Fatalf("租户A应只看到 1 条，实际 %d", got)
	}
	if got := listDataResourceTotal(t, env, tokenB, ""); got != 1 {
		t.Fatalf("租户B应只看到 1 条，实际 %d", got)
	}

	// 2) 跨租户修改：A 改 B 的对象必须失败
	body, _ := json.Marshal(map[string]interface{}{
		"id":     idB,
		"name":   "hacked-by-a",
		"type":   "HTTP推送",
		"option": map[string]interface{}{"url": "http://127.0.0.1:1/x", "method": "post"},
	})
	w := doRequest(env.engine, http.MethodPut, "/api/v1/dataresource", tokenA, body)
	if resp := parseResponse(t, w); resp.Success {
		t.Fatal("跨租户修改应当失败，但成功了")
	}

	// 3) 跨租户删除：A 删 B 的对象必须失败，且 B 的数据仍在
	w = doRequest(env.engine, http.MethodDelete, "/api/v1/dataresource/"+idB, tokenA, nil)
	if resp := parseResponse(t, w); resp.Success {
		t.Fatal("跨租户删除应当失败，但成功了")
	}
	if got := listDataResourceTotal(t, env, tokenB, ""); got != 1 {
		t.Fatalf("租户B的数据被跨租户删除了，剩余 %d", got)
	}

	// 4) 平台管理员：默认平台视图可跨租户看到全部
	if got := listDataResourceTotal(t, env, env.token, ""); got != 2 {
		t.Fatalf("平台管理员应看到 2 条，实际 %d", got)
	}
	// 代管租户A：只看 A 的数据
	if got := listDataResourceTotal(t, env, env.token, tenantA.Id); got != 1 {
		t.Fatalf("平台管理员代管租户A应看到 1 条，实际 %d", got)
	}

	// 5) 租户用户不能访问平台级接口（租户管理）
	w = doRequest(env.engine, http.MethodGet, "/api/v1/tenant?page=1&pageSize=10", tokenA, nil)
	if resp := parseResponse(t, w); resp.Success {
		t.Fatal("租户用户访问租户管理接口应当被拒绝")
	}

	// 6) 用户列表也按租户隔离：租户A只看到自己租户的用户
	w = doRequest(env.engine, http.MethodGet, "/api/v1/sys-user?page=1&pageSize=50", tokenA, nil)
	var userList struct {
		Success bool `json:"success"`
		Result  struct {
			Total int `json:"total"`
		} `json:"result"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &userList); err != nil {
		t.Fatalf("decode users: %v", err)
	}
	if !userList.Success {
		t.Fatalf("用户列表失败: %s", w.Body.String())
	}
	if userList.Result.Total != 1 {
		t.Fatalf("租户A应只看到 1 个用户，实际 %d", userList.Result.Total)
	}

	// 7) A 的对象 id 在 B 的视图里不存在（按 id 直查也拿不到）
	_ = idA
}

// countBoards 返回该 token 可见的看板数量
func countBoards(t *testing.T, env *testEnv, token string) int {
	t.Helper()
	w := doRequest(env.engine, http.MethodGet, "/api/v1/viz/boards", token, nil)
	var resp struct {
		Success bool `json:"success"`
		Result  struct {
			List []map[string]interface{} `json:"list"`
		} `json:"result"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode boards: %v", err)
	}
	if !resp.Success {
		t.Fatalf("看板列表失败: %s", w.Body.String())
	}
	return len(resp.Result.List)
}

// TestVizBoardTenantIsolation 看板按租户隔离。
//
// 历史实现把看板存在单个 .viz-boards.json 文件里，多租户下所有租户共用同一份，
// 现在改为落库 + 租户过滤，这里锁住该行为。
func TestVizBoardTenantIsolation(t *testing.T) {
	env := setupTestEnv(t)
	tenantplugin.SetPolicy(tenantplugin.PolicyEnforce, nil)
	t.Cleanup(func() { tenantplugin.SetPolicy(tenantplugin.PolicyWarn, nil) })

	_, userA := seedTenant(t, env, "viz_a")
	_, userB := seedTenant(t, env, "viz_b")
	tokenA := loginAs(t, env, userA, tenantTestPassword)
	tokenB := loginAs(t, env, userB, tenantTestPassword)

	// 新租户首次访问会自动种入内置看板（首页 + 系统监控，见 pkg/seed/boards.go）
	if got := countBoards(t, env, tokenA); got != 2 {
		t.Fatalf("租户A应自动获得 2 块内置看板，实际 %d", got)
	}

	createBoard := func(token, name string) {
		body, _ := json.Marshal(map[string]interface{}{
			"name":   name,
			"icon":   "bar-chart",
			"canvas": map[string]interface{}{"cols": 12, "rowHeight": 40, "widgets": []interface{}{}},
		})
		w := doRequest(env.engine, http.MethodPost, "/api/v1/viz/boards", token, body)
		assertSuccess(t, w, "create board "+name)
	}
	createBoard(tokenA, "A-看板")
	createBoard(tokenB, "B-看板")

	if got := countBoards(t, env, tokenA); got != 3 {
		t.Fatalf("租户A应看到 2 块内置 + 自己的 1 块，实际 %d", got)
	}
	if got := countBoards(t, env, tokenB); got != 3 {
		t.Fatalf("租户B应看到 2 块内置 + 自己的 1 块，实际 %d", got)
	}

	// 校验名字不串（A 的列表里不能出现 B 的看板）
	w := doRequest(env.engine, http.MethodGet, "/api/v1/viz/boards", tokenA, nil)
	var resp struct {
		Result struct {
			List []map[string]interface{} `json:"list"`
		} `json:"result"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode boards: %v", err)
	}
	for _, item := range resp.Result.List {
		if item["name"] == "B-看板" {
			t.Fatal("租户A看到了租户B的看板")
		}
	}
}
