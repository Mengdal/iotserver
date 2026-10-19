package e2e

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"iotserver/internal/dtos"
	"iotserver/internal/models"
	"iotserver/internal/pkg/tenant"
	"iotserver/internal/pkg/utils"
	tenantplugin "iotserver/internal/tools/sqldb/tenant"
)

/******************************* 工具：查询与造数 *******************************/

// listAuditLogs 通过接口查询审查日志（tenantHeader 非空时为平台管理员代管租户）
func listAuditLogs(t *testing.T, env *testEnv, token, tenantHeader, query string) []map[string]interface{} {
	t.Helper()
	w := doRequestWithTenant(env.engine, http.MethodGet, "/api/v1/sys-audit-log?"+query, token, tenantHeader, nil)
	resp := parseResponse(t, w)
	if !resp.Success {
		t.Fatalf("查询审查日志失败: %s", resp.ErrorMsg)
	}
	page, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatalf("审查日志返回结构异常: %T", resp.Result)
	}
	list, _ := page["list"].([]interface{})
	out := make([]map[string]interface{}, 0, len(list))
	for _, item := range list {
		if m, ok := item.(map[string]interface{}); ok {
			out = append(out, m)
		}
	}
	return out
}

// platformAuditLogs 直连库按平台作用域查询（跨租户，用于断言落库细节）
func platformAuditLogs(t *testing.T, env *testEnv, req dtos.SysAuditLogSearchRequest) []models.AuditLog {
	t.Helper()
	req.IsAll = true
	list, _, err := env.client.WithCtx(tenant.Platform(context.Background())).SysAuditLogSearch(0, 0, req)
	if err != nil {
		t.Fatalf("直连库查询审查日志失败: %v", err)
	}
	return list
}

// insertAuditLog 直接插入一条审查日志（保留清理用例用）
func insertAuditLog(t *testing.T, env *testEnv, username string, created int64) string {
	t.Helper()
	log := models.AuditLog{
		Id:        utils.RandomNum(),
		Type:      models.AuditTypeLogin,
		Username:  username,
		Status:    models.LoginStatusSuccess,
		Message:   "造数",
		LoginTime: created,
	}
	log.Created = created
	log.Modified = time.Now().UnixMilli()
	if err := env.client.WithCtx(tenant.Platform(context.Background())).AddSysAuditLog(log); err != nil {
		t.Fatalf("插入审查日志失败: %v", err)
	}
	return log.Id
}

// countAuditLogs 记录总数
func countAuditLogs(t *testing.T, env *testEnv) int {
	t.Helper()
	_, total, err := env.client.WithCtx(tenant.Platform(context.Background())).
		SysAuditLogSearch(0, 0, dtos.SysAuditLogSearchRequest{IsAll: true})
	if err != nil {
		t.Fatalf("统计审查日志失败: %v", err)
	}
	return int(total)
}

// auditLogExists 按用户名判断记录是否还在
func auditLogExists(t *testing.T, env *testEnv, username string) bool {
	t.Helper()
	list, _, err := env.client.WithCtx(tenant.Platform(context.Background())).
		SysAuditLogSearch(0, 0, dtos.SysAuditLogSearchRequest{Username: username, IsAll: true})
	if err != nil {
		t.Fatalf("查询审查日志失败: %v", err)
	}
	return len(list) > 0
}

/******************************* 用例 1：登录记录 *******************************/

func TestAuditLogLoginRecords(t *testing.T) {
	env := setupTestEnv(t)

	// 成功
	okBody, _ := json.Marshal(dtos.LoginRequest{Username: "admin", Password: "admin123"})
	if resp := parseResponse(t, doRequest(env.engine, http.MethodPost, "/api/v1/auth/login", "", okBody)); !resp.Success {
		t.Fatalf("登录应成功: %v", resp.ErrorMsg)
	}
	// 密码错误
	badBody, _ := json.Marshal(dtos.LoginRequest{Username: "admin", Password: "not-the-password"})
	if resp := parseResponse(t, doRequest(env.engine, http.MethodPost, "/api/v1/auth/login", "", badBody)); resp.Success {
		t.Fatal("错误密码不应登录成功")
	}
	// 用户不存在
	missingBody, _ := json.Marshal(dtos.LoginRequest{Username: "audit_missing_user", Password: "whatever"})
	if resp := parseResponse(t, doRequest(env.engine, http.MethodPost, "/api/v1/auth/login", "", missingBody)); resp.Success {
		t.Fatal("不存在的用户不应登录成功")
	}

	var success, wrongPwd, missing bool
	for _, r := range platformAuditLogs(t, env, dtos.SysAuditLogSearchRequest{Type: models.AuditTypeLogin}) {
		if r.Type != models.AuditTypeLogin || r.Module != models.AuditModuleAuth || r.Action != models.AuditActionLogin {
			t.Fatalf("登录记录的类型/模块/动作不正确: %+v", r)
		}
		if r.Created == 0 || r.LoginTime == 0 {
			t.Fatalf("登录记录缺少事件时间: %+v", r)
		}
		if r.Username == "admin" && r.Status == models.LoginStatusSuccess && r.Message == "登录成功" {
			success = true
		}
		if r.Username == "admin" && r.Status == models.LoginStatusFail && r.Message == "密码错误" {
			wrongPwd = true
		}
		if r.Username == "audit_missing_user" && r.Status == models.LoginStatusFail && r.Message == "用户不存在" {
			missing = true
		}
	}
	if !success || !wrongPwd || !missing {
		t.Fatalf("登录记录不完整: 成功=%v 密码错误=%v 用户不存在=%v", success, wrongPwd, missing)
	}
}

/******************************* 用例 2：操作记录（新增产品） *******************************/

func TestAuditLogOperationRecordForProductAdd(t *testing.T) {
	env := setupTestEnv(t)

	body, _ := json.Marshal(map[string]interface{}{
		"name":                 "audit-product-1",
		"category_template_id": "1",
		"protocol":             "MQTT",
		"node_type":            "1",
		"net_type":             "1",
		"data_format":          "1",
	})
	w := doRequest(env.engine, http.MethodPost, "/api/v1/product", env.token, body)
	if resp := parseResponse(t, w); !resp.Success {
		t.Fatalf("新增产品失败: %v", resp.ErrorMsg)
	}
	env.auditApp.Flush()

	records := platformAuditLogs(t, env, dtos.SysAuditLogSearchRequest{
		Type: models.AuditTypeOperation, Module: "产品管理",
	})
	if len(records) == 0 {
		t.Fatal("新增产品未写入操作日志")
	}
	r := records[0]
	if r.Action != "新增" || r.Status != models.LoginStatusSuccess {
		t.Fatalf("操作记录的动作/状态不正确: %+v", r)
	}
	if r.Username != "admin" || r.Method != http.MethodPost || !strings.Contains(r.Path, "/product") {
		t.Fatalf("操作记录的操作人/方法/路径不正确: %+v", r)
	}
	if r.Target != "audit-product-1" {
		t.Fatalf("操作对象应为请求体的 name，实际 %q", r.Target)
	}
	if !strings.Contains(r.Params, "audit-product-1") || r.DurationMs < 0 {
		t.Fatalf("操作参数/耗时异常: params=%s duration=%d", r.Params, r.DurationMs)
	}

	// 接口查询同样可见
	for _, item := range listAuditLogs(t, env, env.token, "", "type=operation&module=产品管理&isAll=true") {
		if item["id"] == r.Id {
			return
		}
	}
	t.Fatal("接口未查到新增产品的操作日志")
}

/******************************* 用例 3：失败的操作记录 *******************************/

func TestAuditLogFailedOperation(t *testing.T) {
	env := setupTestEnv(t)

	// 缺少必填 name → 参数校验失败，但仍要留痕 status=0 + error_code
	w := doRequest(env.engine, http.MethodPost, "/api/v1/dept", env.token, []byte(`{}`))
	resp := parseResponse(t, w)
	if resp.Success {
		t.Fatal("缺必填字段的新增部门不应成功")
	}
	env.auditApp.Flush()

	records := platformAuditLogs(t, env, dtos.SysAuditLogSearchRequest{
		Type: models.AuditTypeOperation, Module: "部门管理",
	})
	if len(records) == 0 {
		t.Fatal("失败的操作也应写入审查日志")
	}
	r := records[0]
	if r.Status != models.LoginStatusFail {
		t.Fatalf("失败操作应记 status=0: %+v", r)
	}
	if r.ErrorCode != strconv.FormatUint(uint64(resp.ErrorCode), 10) {
		t.Fatalf("error_code 应与接口返回一致: 记录=%q 接口=%d", r.ErrorCode, resp.ErrorCode)
	}
	if r.Message == "" {
		t.Fatalf("失败操作应记录错误信息: %+v", r)
	}
}

/******************************* 用例 4：敏感字段打码 *******************************/

func TestAuditLogParamsMasked(t *testing.T) {
	env := setupTestEnv(t)
	const plain = "Sup3rSecret!23"

	body, _ := json.Marshal(map[string]interface{}{
		"username": "audit_mask_user", "password": plain, "nickname": "打码测试", "status": 1,
	})
	w := doRequest(env.engine, http.MethodPost, "/api/v1/sys-user", env.token, body)
	if resp := parseResponse(t, w); !resp.Success {
		t.Fatalf("新增用户失败: %v", resp.ErrorMsg)
	}
	env.auditApp.Flush()

	found := false
	for _, r := range platformAuditLogs(t, env, dtos.SysAuditLogSearchRequest{
		Type: models.AuditTypeOperation, Module: "用户管理",
	}) {
		if !strings.Contains(r.Params, "audit_mask_user") {
			continue
		}
		found = true
		if strings.Contains(r.Params, plain) {
			t.Fatalf("params 未打码: %s", r.Params)
		}
		if !strings.Contains(r.Params, "***") {
			t.Fatalf("params 应包含打码占位: %s", r.Params)
		}
	}
	if !found {
		t.Fatal("未找到新增用户的操作记录")
	}

	// 库里任何一行都不能有明文密码
	var count int64
	if err := env.client.WithCtx(tenant.Platform(context.Background())).GetDBInstance().
		Table(models.TableAuditLog).Where("params LIKE ?", "%"+plain+"%").Count(&count).Error; err != nil {
		t.Fatalf("统计明文密码失败: %v", err)
	}
	if count != 0 {
		t.Fatalf("审查日志中残留明文密码，命中 %d 行", count)
	}
}

/******************************* 用例 5：租户隔离 *******************************/

func TestAuditLogTenantIsolation(t *testing.T) {
	env := setupTestEnv(t)
	tenantplugin.SetPolicy(tenantplugin.PolicyEnforce, nil)
	t.Cleanup(func() { tenantplugin.SetPolicy(tenantplugin.PolicyWarn, nil) })

	tenantA, userA := seedTenant(t, env, "audit_a")
	_, userB := seedTenant(t, env, "audit_b")
	tokenA := loginAs(t, env, userA, tenantTestPassword)
	tokenB := loginAs(t, env, userB, tenantTestPassword)

	// 租户A 产生一条操作记录
	roleBody, _ := json.Marshal(dtos.SysRoleAddRequest{
		Name: "审查日志角色A", RoleKey: "audit_a_role", Status: 1, DataScope: "1",
	})
	if resp := parseResponse(t, doRequest(env.engine, http.MethodPost, "/api/v1/role", tokenA, roleBody)); !resp.Success {
		t.Fatalf("租户A建角色失败: %v", resp.ErrorMsg)
	}
	env.auditApp.Flush()

	listA := listAuditLogs(t, env, tokenA, "", "isAll=true")
	if len(listA) == 0 {
		t.Fatal("租户A看不到自己的审查日志")
	}
	for _, r := range listA {
		if r["username"] == userB {
			t.Fatalf("租户A看到了租户B的记录: %+v", r)
		}
	}

	listB := listAuditLogs(t, env, tokenB, "", "isAll=true")
	if len(listB) == 0 {
		t.Fatal("租户B看不到自己的审查日志")
	}
	for _, r := range listB {
		if r["username"] == userA {
			t.Fatalf("租户B看到了租户A的记录: %+v", r)
		}
	}

	// 平台视图：跨租户可见全部
	seenA, seenB := false, false
	for _, r := range listAuditLogs(t, env, env.token, "", "isAll=true") {
		if r["username"] == userA {
			seenA = true
		}
		if r["username"] == userB {
			seenB = true
		}
	}
	if !seenA || !seenB {
		t.Fatalf("平台视图应看到全部租户的记录: A=%v B=%v", seenA, seenB)
	}

	// 平台代管租户A：只看 A
	scoped := listAuditLogs(t, env, env.token, tenantA.Id, "isAll=true")
	if len(scoped) == 0 {
		t.Fatal("平台代管租户A时看不到A的记录")
	}
	for _, r := range scoped {
		if r["username"] == userB {
			t.Fatalf("代管租户A时看到了租户B的记录: %+v", r)
		}
	}
}

/******************************* 用例 6：保留清理 *******************************/

func TestAuditLogRetentionCleanup(t *testing.T) {
	env := setupTestEnv(t)

	// 审查日志保留天数固定为 30 天（按租户的时序数据保留策略是另一套，见租户管理）
	expired := "audit_expired_1"
	fresh := "audit_fresh_1"
	insertAuditLog(t, env, expired, time.Now().AddDate(0, 0, -31).UnixMilli())
	insertAuditLog(t, env, fresh, time.Now().AddDate(0, 0, -1).UnixMilli())

	removed, err := env.auditApp.CleanupExpired(context.Background())
	if err != nil {
		t.Fatalf("清理失败: %v", err)
	}
	if removed < 1 {
		t.Fatalf("应至少删除 1 条过期记录，实际 %d", removed)
	}
	if auditLogExists(t, env, expired) {
		t.Fatal("超过 30 天的记录未被删除")
	}
	if !auditLogExists(t, env, fresh) {
		t.Fatal("30 天内的记录被误删")
	}
}

/******************************* 用例 7：白名单 *******************************/

func TestAuditLogWhitelistPaths(t *testing.T) {
	env := setupTestEnv(t)

	loginBody, _ := json.Marshal(dtos.LoginRequest{Username: "admin", Password: "admin123"})
	doRequest(env.engine, http.MethodPost, "/api/v1/auth/login", "", loginBody)
	doRequest(env.engine, http.MethodPost, "/api/v1/auth/init-password", "", []byte(`{"newPassword":"abcdef"}`))
	doRequest(env.engine, http.MethodPost, "/api/v1/ekuiper/alert", "", []byte(`{}`))
	doRequest(env.engine, http.MethodPost, "/api/v1/ekuiper/scene", "", []byte(`{}`))
	env.auditApp.Flush()

	for _, r := range platformAuditLogs(t, env, dtos.SysAuditLogSearchRequest{Type: models.AuditTypeOperation}) {
		if strings.Contains(r.Path, "auth/login") || strings.Contains(r.Path, "auth/init-password") {
			t.Fatalf("认证白名单路径不应产生操作记录: %+v", r)
		}
		if strings.Contains(r.Path, "ekuiper") {
			t.Fatalf("ekuiper 回调不应产生操作记录: %+v", r)
		}
	}
}

/******************************* 用例 8：旧接口兼容别名 *******************************/

func TestAuditLogLegacyLoginLogRoute(t *testing.T) {
	env := setupTestEnv(t)

	// 造一条操作记录，旧接口不应返回它
	body, _ := json.Marshal(map[string]interface{}{
		"name": "audit-product-legacy", "category_template_id": "1",
		"protocol": "MQTT", "node_type": "1", "net_type": "1", "data_format": "1",
	})
	if resp := parseResponse(t, doRequest(env.engine, http.MethodPost, "/api/v1/product", env.token, body)); !resp.Success {
		t.Fatalf("新增产品失败: %v", resp.ErrorMsg)
	}
	env.auditApp.Flush()

	w := doRequest(env.engine, http.MethodGet, "/api/v1/sys-login-log?isAll=true", env.token, nil)
	resp := parseResponse(t, w)
	if !resp.Success {
		t.Fatalf("旧登入日志接口应保持可用: %v", resp.ErrorMsg)
	}
	list, _ := resp.Result.(map[string]interface{})["list"].([]interface{})
	if len(list) == 0 {
		t.Fatal("旧登入日志接口应返回登录记录")
	}
	for _, item := range list {
		r, _ := item.(map[string]interface{})
		if r["type"] != models.AuditTypeLogin {
			t.Fatalf("旧接口应等价 type=login，实际返回: %+v", r)
		}
	}
}
