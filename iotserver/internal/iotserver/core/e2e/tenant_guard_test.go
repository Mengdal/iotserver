package e2e

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"iotserver/internal/dtos"
	"iotserver/internal/models"
	"iotserver/internal/pkg/constants"
	"iotserver/internal/pkg/tenant"
)

// 租户管理三个缺口的端到端验证：
//   ① 删租户保护（有数据拒绝 / 无数据允许）
//   ② 配额校验（设备/网关/用户超限拒绝；0 = 不限）
//   ③ 到期校验（登录拒绝 + 已登录会话被中间件拦截；平台管理员不受影响）

const (
	errCodeTenantExpired  = 21711
	errCodeTenantQuota    = 21712
	errCodeTenantNotEmpty = 21713
)

// platformCtx / tenantCtxOf 测试用上下文构造
func platformCtx() context.Context { return tenant.Platform(context.Background()) }

func tenantCtxOf(tenantId string) context.Context {
	return tenant.With(platformCtx(), tenant.Context{TenantID: tenantId})
}

// setTenantQuota 直接改租户的配额/到期字段（走仓储，避免依赖租户管理页接口）
func setTenantQuota(t *testing.T, env *testEnv, tenantId string, mutate func(*models.Tenant)) {
	t.Helper()
	raw, err := env.client.WithCtx(platformCtx()).TenantById(tenantId)
	if err != nil {
		t.Fatalf("查询租户失败: %v", err)
	}
	mutate(&raw)
	if err = env.client.WithCtx(platformCtx()).UpdateTenant(raw); err != nil {
		t.Fatalf("更新租户配额失败: %v", err)
	}
}

// seedTenantProduct 在租户下建一个已发布产品（建设备需要产品存在）
func seedTenantProduct(t *testing.T, env *testEnv, tenantId, productId string) {
	t.Helper()
	if _, err := env.client.WithCtx(tenantCtxOf(tenantId)).AddProduct(models.Product{
		Id: productId, Name: productId, Status: constants.ProductRelease,
	}); err != nil {
		t.Fatalf("建产品失败: %v", err)
	}
}

// TestTenantDeleteProtection 有数据的租户删不掉，空租户可以删掉
func TestTenantDeleteProtection(t *testing.T) {
	env := setupTestEnv(t)

	// ===== 有数据：1 个额外用户 + 1 台设备 + 1 个产品（seedTenant 已建 1 个租户管理员）=====
	tc, _ := seedTenant(t, env, "delguard")
	tctx := tenantCtxOf(tc.Id)
	if _, err := env.client.WithCtx(tctx).AddSysUser(models.User{
		Username: "delguard_extra", Password: "x", Nickname: "额外用户", Status: 1,
	}); err != nil {
		t.Fatalf("建用户失败: %v", err)
	}
	seedTenantProduct(t, env, tc.Id, "delguard_p1")
	if _, err := env.client.WithCtx(tctx).AddDevice(models.Device{
		Name: "delguard_d1", ProductId: "delguard_p1",
	}); err != nil {
		t.Fatalf("建设备失败: %v", err)
	}

	w := doRequest(env.engine, http.MethodDelete, "/api/v1/tenant/"+tc.Id, env.token, nil)
	resp := parseResponse(t, w)
	if resp.Success {
		t.Fatal("有数据的租户不应允许删除")
	}
	if resp.ErrorCode != errCodeTenantNotEmpty {
		t.Fatalf("期望 errorCode %d，实际 %d（%s）", errCodeTenantNotEmpty, resp.ErrorCode, resp.ErrorMsg)
	}
	// 错误信息要带上各项数量
	for _, want := range []string{"2 个用户", "1 台设备", "1 个产品", "请先清理"} {
		if !strings.Contains(resp.ErrorMsg, want) {
			t.Fatalf("删除保护错误信息缺少 %q：%s", want, resp.ErrorMsg)
		}
	}
	// 拒绝后租户仍在
	if _, err := env.client.WithCtx(platformCtx()).TenantById(tc.Id); err != nil {
		t.Fatalf("被拒绝删除后租户应仍然存在: %v", err)
	}

	// ===== 无数据：空租户允许删除 =====
	empty, err := env.client.WithCtx(platformCtx()).AddTenant(models.Tenant{Code: "delempty", Name: "空租户"})
	if err != nil {
		t.Fatalf("建空租户失败: %v", err)
	}
	w = doRequest(env.engine, http.MethodDelete, "/api/v1/tenant/"+empty.Id, env.token, nil)
	if resp = parseResponse(t, w); !resp.Success {
		t.Fatalf("空租户应允许删除，实际失败: %v", resp.ErrorMsg)
	}
	if _, err = env.client.WithCtx(platformCtx()).TenantById(empty.Id); err == nil {
		t.Fatal("删除后租户应查不到")
	}

	// ===== 只有内置看板：内置看板是系统种入的，不应挡住删除，删除时一并清理 =====
	builtinTenant, err := env.client.WithCtx(platformCtx()).AddTenant(models.Tenant{Code: "delbuiltin", Name: "内置看板租户"})
	if err != nil {
		t.Fatalf("建内置看板租户失败: %v", err)
	}
	if err = env.client.WithCtx(tenantCtxOf(builtinTenant.Id)).VizBoardsReplace([]models.VizBoard{{
		Id: "builtin-home-" + builtinTenant.Id, Name: "首页看板", IsHome: 1,
	}}); err != nil {
		t.Fatalf("写入内置看板失败: %v", err)
	}
	if resp = parseResponse(t, doRequest(env.engine, http.MethodDelete, "/api/v1/tenant/"+builtinTenant.Id, env.token, nil)); !resp.Success {
		t.Fatalf("只有内置看板的租户应允许删除，实际失败: %v", resp.ErrorMsg)
	}
	if boards, bErr := env.client.WithCtx(tenantCtxOf(builtinTenant.Id)).VizBoards(); bErr != nil || len(boards) != 0 {
		t.Fatalf("删除租户后应清理其看板，实际 %d 块（err=%v）", len(boards), bErr)
	}

	// ===== 用户自建看板：仍然保护，不允许删除 =====
	customTenant, err := env.client.WithCtx(platformCtx()).AddTenant(models.Tenant{Code: "delcustom", Name: "自建看板租户"})
	if err != nil {
		t.Fatalf("建自建看板租户失败: %v", err)
	}
	if err = env.client.WithCtx(tenantCtxOf(customTenant.Id)).VizBoardsReplace([]models.VizBoard{{
		Id: "custom-board", Name: "客户看板",
	}}); err != nil {
		t.Fatalf("写入自建看板失败: %v", err)
	}
	w = doRequest(env.engine, http.MethodDelete, "/api/v1/tenant/"+customTenant.Id, env.token, nil)
	resp = parseResponse(t, w)
	if resp.Success || resp.ErrorCode != errCodeTenantNotEmpty {
		t.Fatalf("有自建看板的租户不应允许删除，实际 success=%v code=%d msg=%s", resp.Success, resp.ErrorCode, resp.ErrorMsg)
	}
	if !strings.Contains(resp.ErrorMsg, "1 个自定义看板") {
		t.Fatalf("删除保护错误信息应只统计自建看板：%s", resp.ErrorMsg)
	}
}

// TestTenantDeviceQuotaEnforced 设备配额：超限拒绝并带当前值/上限；配额=0 不限
func TestTenantDeviceQuotaEnforced(t *testing.T) {
	env := setupTestEnv(t)
	tc, username := seedTenant(t, env, "quotadev")
	token := loginAs(t, env, username, tenantTestPassword)

	// 配额 1，已有 1 台设备 → 再建第 2 台必须被拒
	setTenantQuota(t, env, tc.Id, func(x *models.Tenant) { x.MaxDevices = 1 })
	tctx := tenantCtxOf(tc.Id)
	seedTenantProduct(t, env, tc.Id, "quotadev_p1")
	if _, err := env.client.WithCtx(tctx).AddDevice(models.Device{
		Name: "quota_existing", ProductId: "quotadev_p1",
	}); err != nil {
		t.Fatalf("预置设备失败: %v", err)
	}

	body, _ := json.Marshal(dtos.DeviceAddRequest{Name: "quota_new", ProductId: "quotadev_p1"})
	w := doRequest(env.engine, http.MethodPost, "/api/v1/device", token, body)
	resp := parseResponse(t, w)
	if resp.Success {
		t.Fatal("设备超配额应被拒绝")
	}
	if resp.ErrorCode != errCodeTenantQuota {
		t.Fatalf("期望 errorCode %d，实际 %d（%s）", errCodeTenantQuota, resp.ErrorCode, resp.ErrorMsg)
	}
	if !strings.Contains(resp.ErrorMsg, "当前 1") || !strings.Contains(resp.ErrorMsg, "上限 1") {
		t.Fatalf("配额提示应含当前值/上限：%s", resp.ErrorMsg)
	}

	// 配额 0 = 不限：连建 2 台都成功
	setTenantQuota(t, env, tc.Id, func(x *models.Tenant) { x.MaxDevices = 0 })
	for _, name := range []string{"free_1", "free_2"} {
		b, _ := json.Marshal(dtos.DeviceAddRequest{Name: name, ProductId: "quotadev_p1"})
		w = doRequest(env.engine, http.MethodPost, "/api/v1/device", token, b)
		if resp = parseResponse(t, w); !resp.Success {
			t.Fatalf("配额=0 时建 %s 应成功，实际: %v", name, resp.ErrorMsg)
		}
	}
}

// TestTenantUserAndGatewayQuotaEnforced 用户/网关配额同规则：超限拒绝，0 不限
func TestTenantUserAndGatewayQuotaEnforced(t *testing.T) {
	env := setupTestEnv(t)
	tc, username := seedTenant(t, env, "quotamix")
	token := loginAs(t, env, username, tenantTestPassword)
	tctx := tenantCtxOf(tc.Id)

	// ===== 用户配额：上限 1，seedTenant 已建 1 个用户 → 再建被拒 =====
	setTenantQuota(t, env, tc.Id, func(x *models.Tenant) { x.MaxUsers = 1 })
	userBody, _ := json.Marshal(dtos.SysUserAddRequest{
		Username: "quota_user", Password: "123456", Nickname: "超配额用户", Status: 1,
	})
	w := doRequest(env.engine, http.MethodPost, "/api/v1/sys-user", token, userBody)
	resp := parseResponse(t, w)
	if resp.Success {
		t.Fatal("用户超配额应被拒绝")
	}
	if resp.ErrorCode != errCodeTenantQuota || !strings.Contains(resp.ErrorMsg, "用户") {
		t.Fatalf("期望用户配额错误 %d，实际 %d（%s）", errCodeTenantQuota, resp.ErrorCode, resp.ErrorMsg)
	}
	// 上限放开后同一请求应成功
	setTenantQuota(t, env, tc.Id, func(x *models.Tenant) { x.MaxUsers = 0 })
	w = doRequest(env.engine, http.MethodPost, "/api/v1/sys-user", token, userBody)
	if resp = parseResponse(t, w); !resp.Success {
		t.Fatalf("用户配额=0 应不限，实际: %v", resp.ErrorMsg)
	}

	// ===== 网关配额：上限 1，预置 1 台 → 再建被拒；放开后成功 =====
	setTenantQuota(t, env, tc.Id, func(x *models.Tenant) { x.MaxGateways = 1 })
	if _, err := env.client.WithCtx(tctx).AddGateway(models.Gateway{Name: "gw_existing", Sn: "sn_existing"}); err != nil {
		t.Fatalf("预置网关失败: %v", err)
	}
	gwBody, _ := json.Marshal(dtos.GatewayAddRequest{Name: "gw_new", Sn: "sn_new"})
	w = doRequest(env.engine, http.MethodPost, "/api/v1/gateway", token, gwBody)
	resp = parseResponse(t, w)
	if resp.Success {
		t.Fatal("网关超配额应被拒绝")
	}
	if resp.ErrorCode != errCodeTenantQuota || !strings.Contains(resp.ErrorMsg, "网关") {
		t.Fatalf("期望网关配额错误 %d，实际 %d（%s）", errCodeTenantQuota, resp.ErrorCode, resp.ErrorMsg)
	}
	setTenantQuota(t, env, tc.Id, func(x *models.Tenant) { x.MaxGateways = 0 })
	w = doRequest(env.engine, http.MethodPost, "/api/v1/gateway", token, gwBody)
	if resp = parseResponse(t, w); !resp.Success {
		t.Fatalf("网关配额=0 应不限，实际: %v", resp.ErrorMsg)
	}
}

// TestTenantExpiryGuard 到期校验：登录拒绝 + 已登录会话被中间件拦截；平台管理员不受影响
func TestTenantExpiryGuard(t *testing.T) {
	env := setupTestEnv(t)
	tc, username := seedTenant(t, env, "expireguard")
	// 到期前先登录，拿到一个「合法但之后会失效」的会话
	token := loginAs(t, env, username, tenantTestPassword)

	// 到期时间设为 1 小时前
	past := time.Now().Add(-time.Hour).UnixMilli()
	setTenantQuota(t, env, tc.Id, func(x *models.Tenant) { x.ExpireAt = past })

	// ① 已登录会话继续请求 → 中间件拦截
	w := doRequest(env.engine, http.MethodGet, "/api/v1/user/getinfo", token, nil)
	resp := parseResponse(t, w)
	if resp.Success {
		t.Fatal("已到期租户的会话不应继续可用")
	}
	if resp.ErrorCode != errCodeTenantExpired {
		t.Fatalf("期望 errorCode %d，实际 %d（%s）", errCodeTenantExpired, resp.ErrorCode, resp.ErrorMsg)
	}

	// ② 到期后重新登录 → 拒绝并提示
	loginBody, _ := json.Marshal(dtos.LoginRequest{Username: username, Password: tenantTestPassword})
	w = doRequest(env.engine, http.MethodPost, "/api/v1/auth/login", "", loginBody)
	resp = parseResponse(t, w)
	if resp.Success {
		t.Fatal("已到期租户应拒绝登录")
	}
	// 文案语言取决于 Accept-Language（e2e 未挂 i18n 中间件时为英文），两种都接受
	if resp.ErrorCode != errCodeTenantExpired ||
		(!strings.Contains(resp.ErrorMsg, "租户已到期") && !strings.Contains(resp.ErrorMsg, "expired")) {
		t.Fatalf("期望「租户已到期」提示 %d，实际 %d（%s）", errCodeTenantExpired, resp.ErrorCode, resp.ErrorMsg)
	}

	// ③ 平台管理员（tenant_id=''）不受到期影响
	w = doRequest(env.engine, http.MethodGet, "/api/v1/user/getinfo", env.token, nil)
	if resp = parseResponse(t, w); !resp.Success {
		t.Fatalf("平台管理员不应受到期影响: %v", resp.ErrorMsg)
	}

	// ④ 用户所属租户查不到（已被删）→ 不阻碍、不 panic
	gone, goneUser := seedTenant(t, env, "goneguard")
	goneToken := loginAs(t, env, goneUser, tenantTestPassword)
	// 直接删租户行，模拟「租户已被删除但用户仍在」
	if err := env.client.WithCtx(platformCtx()).DeleteTenant(gone.Id); err != nil {
		t.Fatalf("删除租户失败: %v", err)
	}
	w = doRequest(env.engine, http.MethodGet, "/api/v1/user/getinfo", goneToken, nil)
	resp = parseResponse(t, w)
	if resp.ErrorCode == errCodeTenantExpired {
		t.Fatalf("租户查不到时不应按到期拦截: %s", resp.ErrorMsg)
	}
}
