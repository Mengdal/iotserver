package e2e

// 菜单多租户改造的验收测试（设计文档 docs/multi-tenant-menu-plan.md §6 的 A1~A8）。
//
// 说明：本轮改造与「租户管理缺口（P0-2/P1-3）」并行开发，sysmanage_test.go 被同时修改，
// 因此验收用例放在同包同目录的独立文件里，避免互相覆盖；运行时与 sysmanage_test.go 完全等价。
//
// A1 平台管理员建/改/删全局菜单，所有租户立刻可见/消失
// A2 租户管理员改/删全局行 → 权限不足；租户自建菜单不允许带 perms；POST 不会生成全局行
// A3 租户A 建自建菜单并授予本租户角色 → 仅该角色可见，租户B / 平台视图不可见
// A4 租户A 隐藏平台菜单 → 仅租户A 不可见；平台重跑 seed 不恢复
// A5 租户A 改名后平台再改名 → 租户A 保留自定义名，租户B 跟随
// A6 平台改全局菜单 path → 有改名覆盖的租户A 也跟随
// A7 平台删除被租户自建子菜单引用的全局菜单 → 拒绝并提示数量；仅覆盖记录 → 允许并级联清理
// A8 平台重置租户A 的菜单定制 → 覆盖与自建菜单清空，恢复平台默认

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"iotserver/internal/dtos"
	"iotserver/internal/models"
	"iotserver/internal/pkg/middleware"
	"iotserver/internal/pkg/permissions"
	"iotserver/internal/pkg/seed"
)

/******************************* 测试工具 *******************************/

// mtLogin 登录并返回 token 与下发的菜单树（登录响应里的 menus 即侧边栏菜单）
func mtLogin(t *testing.T, env *testEnv, username, password string) (string, []interface{}) {
	t.Helper()
	body, _ := json.Marshal(dtos.LoginRequest{Username: username, Password: password})
	w := doRequest(env.engine, http.MethodPost, "/api/v1/auth/login", "", body)
	resp := parseResponse(t, w)
	if !resp.Success {
		t.Fatalf("login %s failed: %v", username, resp.ErrorMsg)
	}
	result, _ := resp.Result.(map[string]interface{})
	token, _ := result["token"].(string)
	if token == "" {
		t.Fatalf("empty token for %s", username)
	}
	menus, _ := result["menus"].([]interface{})
	return token, menus
}

func mtFindMenuById(items []interface{}, id string) map[string]interface{} {
	for _, it := range items {
		node, ok := it.(map[string]interface{})
		if !ok {
			continue
		}
		if node["id"] == id {
			return node
		}
		if children, ok := node["children"].([]interface{}); ok && len(children) > 0 {
			if hit := mtFindMenuById(children, id); hit != nil {
				return hit
			}
		}
	}
	return nil
}

func mtFindMenuByPath(items []interface{}, path string) map[string]interface{} {
	for _, it := range items {
		node, ok := it.(map[string]interface{})
		if !ok {
			continue
		}
		if node["path"] == path {
			return node
		}
		if children, ok := node["children"].([]interface{}); ok && len(children) > 0 {
			if hit := mtFindMenuByPath(children, path); hit != nil {
				return hit
			}
		}
	}
	return nil
}

// mtAddMenu 用指定 token 新增菜单；失败返回 ok=false 与错误信息
func mtAddMenu(t *testing.T, env *testEnv, token string, req dtos.SysMenuAddRequest) (string, bool, string) {
	t.Helper()
	body, _ := json.Marshal(req)
	w := doRequest(env.engine, http.MethodPost, "/api/v1/menu", token, body)
	resp := parseResponse(t, w)
	if !resp.Success {
		return "", false, resp.ErrorMsg
	}
	var menus []models.Menu
	if err := env.client.Pool.Table(models.TableMenu).Where("name = ?", req.Name).
		Order("created DESC").Find(&menus).Error; err != nil {
		t.Fatalf("query menu %s: %v", req.Name, err)
	}
	if len(menus) == 0 {
		t.Fatalf("menu %s not found after add", req.Name)
	}
	return menus[0].Id, true, ""
}

// mtPutMenu 用指定 token 修改菜单
func mtPutMenu(t *testing.T, env *testEnv, token string, body map[string]interface{}) (success bool, code uint32, msg string) {
	t.Helper()
	raw, _ := json.Marshal(body)
	w := doRequest(env.engine, http.MethodPut, "/api/v1/menu", token, raw)
	resp := parseResponse(t, w)
	return resp.Success, resp.ErrorCode, resp.ErrorMsg
}

// mtDeleteMenu 用指定 token 删除菜单
func mtDeleteMenu(t *testing.T, env *testEnv, token string, ids ...string) (success bool, code uint32, msg string) {
	t.Helper()
	raw, _ := json.Marshal(dtos.SysMenuIdsRequest{Ids: ids})
	w := doRequest(env.engine, http.MethodDelete, "/api/v1/menu", token, raw)
	resp := parseResponse(t, w)
	return resp.Success, resp.ErrorCode, resp.ErrorMsg
}

// mtSavePref 用指定 token 保存租户展示层覆盖
func mtSavePref(t *testing.T, env *testEnv, token string, req dtos.SysMenuPrefSaveRequest) (success bool, code uint32, msg string) {
	t.Helper()
	raw, _ := json.Marshal(req)
	w := doRequest(env.engine, http.MethodPut, "/api/v1/menu/pref", token, raw)
	resp := parseResponse(t, w)
	return resp.Success, resp.ErrorCode, resp.ErrorMsg
}

// mtMenuRow 按 id 直查菜单行（不过滤租户，用于校验落库归属）
func mtMenuRow(t *testing.T, env *testEnv, id string) models.Menu {
	t.Helper()
	var m models.Menu
	if err := env.client.Pool.Table(models.TableMenu).Where("id = ?", id).First(&m).Error; err != nil {
		t.Fatalf("query menu %s: %v", id, err)
	}
	return m
}

// mtCountPrefs 统计某个菜单的覆盖记录数（跨租户）
func mtCountPrefs(t *testing.T, env *testEnv, menuId string) int64 {
	t.Helper()
	var cnt int64
	if err := env.client.Pool.Table(models.TableTenantMenuPref).
		Where("menu_id = ?", menuId).Count(&cnt).Error; err != nil {
		t.Fatalf("count menu pref: %v", err)
	}
	return cnt
}

/******************************* A1 *******************************/

func TestMenuTenantA1PlatformMenuWriteVisibleToAllTenants(t *testing.T) {
	env := setupTestEnv(t)
	_, userA := seedTenant(t, env, "menu_a1")

	// 平台管理员新增全局菜单
	id, ok, msg := mtAddMenu(t, env, env.token, dtos.SysMenuAddRequest{
		ParentId: "0", Name: "平台新菜单A1", Path: "/a1/new", MenuType: "C", Status: 1, Visible: 1, IsFrame: 1,
	})
	if !ok {
		t.Fatalf("平台新增全局菜单失败: %s", msg)
	}
	if row := mtMenuRow(t, env, id); row.TenantId != "" {
		t.Fatalf("平台新增菜单的 tenant_id 应为空, got %q", row.TenantId)
	}
	// 所有租户立刻可见
	if _, menus := mtLogin(t, env, userA, tenantTestPassword); mtFindMenuByPath(menus, "/a1/new") == nil {
		t.Fatal("租户A看不到平台新增菜单")
	}

	// 平台修改
	if success, _, msg := mtPutMenu(t, env, env.token, map[string]interface{}{
		"id": id, "parentId": "0", "name": "平台新菜单A1改", "path": "/a1/new2",
		"menuType": "C", "visible": 1, "status": 1, "isFrame": 1,
	}); !success {
		t.Fatalf("平台修改全局菜单失败: %s", msg)
	}
	if _, menus := mtLogin(t, env, userA, tenantTestPassword); mtFindMenuByPath(menus, "/a1/new2") == nil {
		t.Fatal("租户A没有跟随平台修改")
	}

	// 平台删除
	if success, _, msg := mtDeleteMenu(t, env, env.token, id); !success {
		t.Fatalf("平台删除全局菜单失败: %s", msg)
	}
	if _, menus := mtLogin(t, env, userA, tenantTestPassword); mtFindMenuByPath(menus, "/a1/new2") != nil {
		t.Fatal("平台删除后租户A仍能看到该菜单")
	}
}

/******************************* A2 *******************************/

func TestMenuTenantA2TenantCannotWritePlatformMenu(t *testing.T) {
	env := setupTestEnv(t)
	tenantA, userA := seedTenant(t, env, "menu_a2")
	tokenA := loginAs(t, env, userA, tenantTestPassword)

	globalId, ok, msg := mtAddMenu(t, env, env.token, dtos.SysMenuAddRequest{
		ParentId: "0", Name: "平台菜单A2", Path: "/a2/platform", MenuType: "C", Status: 1, Visible: 1, IsFrame: 1,
	})
	if !ok {
		t.Fatalf("平台新增全局菜单失败: %s", msg)
	}

	// 租户改全局行 → 权限不足
	success, code, msg := mtPutMenu(t, env, tokenA, map[string]interface{}{
		"id": globalId, "parentId": "0", "name": "被租户改", "path": "/hacked",
		"menuType": "C", "visible": 1, "status": 1, "isFrame": 1,
	})
	if success {
		t.Fatal("租户修改平台菜单应被拒绝")
	}
	if code != 21708 {
		t.Fatalf("期望权限错误 21708, got %d (%s)", code, msg)
	}

	// 租户删全局行 → 权限不足
	success, code, msg = mtDeleteMenu(t, env, tokenA, globalId)
	if success {
		t.Fatal("租户删除平台菜单应被拒绝")
	}
	if code != 21708 {
		t.Fatalf("期望权限错误 21708, got %d (%s)", code, msg)
	}

	// 全局行未被篡改
	if row := mtMenuRow(t, env, globalId); row.Name != "平台菜单A2" || row.Path != "/a2/platform" {
		t.Fatalf("平台菜单被篡改: %+v", row)
	}

	// 租户自建菜单不允许设置 perms（防自助提权）
	if _, ok, _ := mtAddMenu(t, env, tokenA, dtos.SysMenuAddRequest{
		ParentId: "0", Name: "越权菜单A2", Path: "/a2/elevate", MenuType: "C",
		Perms: "system:user:remove", Status: 1, Visible: 1, IsFrame: 1,
	}); ok {
		t.Fatal("租户自建菜单带 perms 应被拒绝")
	}

	// 租户正常新增只能落在自己租户下（POST 不可能生成全局行）
	mineId, ok, msg := mtAddMenu(t, env, tokenA, dtos.SysMenuAddRequest{
		ParentId: "0", Name: "租户A菜单A2", Path: "/a2/mine", MenuType: "C", Status: 1, Visible: 1, IsFrame: 1,
	})
	if !ok {
		t.Fatalf("租户新增自建菜单失败: %s", msg)
	}
	if row := mtMenuRow(t, env, mineId); row.TenantId != tenantA.Id {
		t.Fatalf("租户自建菜单归属应为 %s, got %q", tenantA.Id, row.TenantId)
	}
}

/******************************* A3 *******************************/

func TestMenuTenantA3CustomMenuOnlyVisibleToGrantedTenantRole(t *testing.T) {
	env := setupTestEnv(t)
	tenantA, userA := seedTenant(t, env, "menu_a3a")
	_, userB := seedTenant(t, env, "menu_a3b")
	tokenA := loginAs(t, env, userA, tenantTestPassword)

	menuId, ok, msg := mtAddMenu(t, env, tokenA, dtos.SysMenuAddRequest{
		ParentId: "0", Name: "租户A专有A3", Path: "/a3/only", MenuType: "C", Status: 1, Visible: 1, IsFrame: 1,
	})
	if !ok {
		t.Fatalf("租户A建自建菜单失败: %s", msg)
	}
	if row := mtMenuRow(t, env, menuId); row.TenantId != tenantA.Id {
		t.Fatalf("自建菜单归属应为 %s, got %q", tenantA.Id, row.TenantId)
	}

	// 租户A：建角色（只授权该自建菜单）+ 用户，登录后应能看到
	roleBody, _ := json.Marshal(dtos.SysRoleAddRequest{
		Name: "A3运维", RoleKey: "a3_ops", DataScope: "1", Status: 1, MenuIds: []string{menuId},
	})
	if resp := parseResponse(t, doRequest(env.engine, http.MethodPost, "/api/v1/role", tokenA, roleBody)); !resp.Success {
		t.Fatalf("租户A建角色失败: %v", resp.ErrorMsg)
	}
	roleId := mtFindRoleIdByKey(t, env, tokenA, "a3_ops")
	userBody, _ := json.Marshal(dtos.SysUserAddRequest{
		Username: "a3ops", Password: "a3pass123", Nickname: "A3运维", Status: 1, RoleIds: []string{roleId},
	})
	if resp := parseResponse(t, doRequest(env.engine, http.MethodPost, "/api/v1/sys-user", tokenA, userBody)); !resp.Success {
		t.Fatalf("租户A建用户失败: %v", resp.ErrorMsg)
	}
	if _, menus := mtLogin(t, env, "a3ops", "a3pass123"); mtFindMenuByPath(menus, "/a3/only") == nil {
		t.Fatal("租户A被授权角色看不到自建菜单")
	}

	// 未授权该菜单的另一个用户（租户A内）看不到
	roleBody2, _ := json.Marshal(dtos.SysRoleAddRequest{Name: "A3空角色", RoleKey: "a3_none", DataScope: "1", Status: 1})
	if resp := parseResponse(t, doRequest(env.engine, http.MethodPost, "/api/v1/role", tokenA, roleBody2)); !resp.Success {
		t.Fatalf("租户A建空角色失败: %v", resp.ErrorMsg)
	}
	roleId2 := mtFindRoleIdByKey(t, env, tokenA, "a3_none")
	userBody2, _ := json.Marshal(dtos.SysUserAddRequest{
		Username: "a3none", Password: "a3pass123", Nickname: "A3未授权", Status: 1, RoleIds: []string{roleId2},
	})
	if resp := parseResponse(t, doRequest(env.engine, http.MethodPost, "/api/v1/sys-user", tokenA, userBody2)); !resp.Success {
		t.Fatalf("租户A建未授权用户失败: %v", resp.ErrorMsg)
	}
	if _, menus := mtLogin(t, env, "a3none", "a3pass123"); mtFindMenuByPath(menus, "/a3/only") != nil {
		t.Fatal("未授权角色不该看到自建菜单")
	}

	// 租户B（管理员视角全量）看不到租户A 的自建菜单
	if _, menus := mtLogin(t, env, userB, tenantTestPassword); mtFindMenuByPath(menus, "/a3/only") != nil {
		t.Fatal("租户B看到了租户A的自建菜单")
	}
	// 平台视图看不到
	if _, menus := mtLogin(t, env, "admin", "admin123"); mtFindMenuByPath(menus, "/a3/only") != nil {
		t.Fatal("平台视图看到了租户自建菜单")
	}
}

func mtFindRoleIdByKey(t *testing.T, env *testEnv, token, roleKey string) string {
	t.Helper()
	w := doRequest(env.engine, http.MethodGet, "/api/v1/role?page=1&pageSize=100", token, nil)
	resp := parseResponse(t, w)
	if !resp.Success {
		t.Fatalf("角色列表失败: %v", resp.ErrorMsg)
	}
	page, _ := resp.Result.(map[string]interface{})
	list, _ := page["list"].([]interface{})
	for _, it := range list {
		role, _ := it.(map[string]interface{})
		if role["roleKey"] == roleKey {
			id, _ := role["id"].(string)
			return id
		}
	}
	t.Fatalf("角色 %s 未找到", roleKey)
	return ""
}

/******************************* A4 *******************************/

func TestMenuTenantA4TenantHideSurvivesSeed(t *testing.T) {
	env := setupTestEnv(t)
	_, userA := seedTenant(t, env, "menu_a4a")
	_, userB := seedTenant(t, env, "menu_a4b")
	tokenA := loginAs(t, env, userA, tenantTestPassword)
	docsId := seed.MenuDocsId // 平台叶子菜单「工具与文档」

	// 基线：租户A 能看到该平台菜单
	if _, menus := mtLogin(t, env, userA, tenantTestPassword); mtFindMenuById(menus, docsId) == nil {
		t.Fatal("基线：租户A 应能看到平台菜单")
	}
	// 租户A 隐藏
	if success, code, msg := mtSavePref(t, env, tokenA, dtos.SysMenuPrefSaveRequest{MenuId: docsId, Hidden: 1}); !success {
		t.Fatalf("隐藏平台菜单失败: %d %s", code, msg)
	}
	if _, menus := mtLogin(t, env, userA, tenantTestPassword); mtFindMenuById(menus, docsId) != nil {
		t.Fatal("租户A 应看不到被隐藏的平台菜单")
	}
	// 租户B 与平台视图照旧
	if _, menus := mtLogin(t, env, userB, tenantTestPassword); mtFindMenuById(menus, docsId) == nil {
		t.Fatal("租户B 不应受租户A 的隐藏影响")
	}
	if _, menus := mtLogin(t, env, "admin", "admin123"); mtFindMenuById(menus, docsId) == nil {
		t.Fatal("平台视图不应受租户隐藏影响")
	}

	// 平台重跑迁移 + seed（等价于升级/重装），租户A 的隐藏不能被恢复
	if err := env.client.MigrateSystemTables(); err != nil {
		t.Fatalf("重跑迁移/seed 失败: %v", err)
	}
	if _, menus := mtLogin(t, env, userA, tenantTestPassword); mtFindMenuById(menus, docsId) != nil {
		t.Fatal("重跑 seed 后租户A 的隐藏被恢复（prefs 被 seed 覆盖）")
	}
	if cnt := mtCountPrefs(t, env, docsId); cnt == 0 {
		t.Fatal("重跑 seed 后 tenant_menu_pref 覆盖记录丢失")
	}
	// 覆盖记录可通过接口读到
	w := doRequest(env.engine, http.MethodGet, "/api/v1/menu/pref", tokenA, nil)
	resp := parseResponse(t, w)
	if !resp.Success {
		t.Fatalf("读取菜单定制失败: %v", resp.ErrorMsg)
	}
	list, _ := resp.Result.([]interface{})
	if len(list) == 0 {
		t.Fatal("菜单定制列表为空")
	}
}

/******************************* A5 + A6 *******************************/

func TestMenuTenantA5A6TenantRenameAndPlatformPathFollow(t *testing.T) {
	env := setupTestEnv(t)
	_, userA := seedTenant(t, env, "menu_a5a")
	_, userB := seedTenant(t, env, "menu_a5b")
	tokenA := loginAs(t, env, userA, tenantTestPassword)
	dataId := seed.MenuDataId // 「数据中心」目录

	// A5：租户A 改名
	if success, code, msg := mtSavePref(t, env, tokenA, dtos.SysMenuPrefSaveRequest{
		MenuId: dataId, CustomName: "我的数据",
	}); !success {
		t.Fatalf("租户改名失败: %d %s", code, msg)
	}
	_, menusA := mtLogin(t, env, userA, tenantTestPassword)
	nodeA := mtFindMenuById(menusA, dataId)
	if nodeA == nil || nodeA["name"] != "我的数据" {
		t.Fatalf("租户A 应看到自定义名称, got %v", nodeA)
	}
	_, menusB := mtLogin(t, env, userB, tenantTestPassword)
	if nodeB := mtFindMenuById(menusB, dataId); nodeB == nil || nodeB["name"] != "数据中心" {
		t.Fatalf("租户B 应跟随平台名称, got %v", nodeB)
	}

	// 平台改名 → 租户A 保留自定义名，租户B 跟随
	if success, _, msg := mtPutMenu(t, env, env.token, map[string]interface{}{
		"id": dataId, "parentId": "0", "name": "平台数据中心", "path": "/data",
		"menuType": "M", "visible": 1, "status": 1, "isFrame": 1,
	}); !success {
		t.Fatalf("平台改名失败: %s", msg)
	}
	_, menusA = mtLogin(t, env, userA, tenantTestPassword)
	if node := mtFindMenuById(menusA, dataId); node == nil || node["name"] != "我的数据" {
		t.Fatalf("租户A 应保留自定义名称, got %v", node)
	}
	_, menusB = mtLogin(t, env, userB, tenantTestPassword)
	if node := mtFindMenuById(menusB, dataId); node == nil || node["name"] != "平台数据中心" {
		t.Fatalf("租户B 应跟随平台新名称, got %v", node)
	}

	// A6：平台改 path —— 有改名覆盖的租户A 也必须跟随（覆盖不含 path）
	if success, _, msg := mtPutMenu(t, env, env.token, map[string]interface{}{
		"id": dataId, "parentId": "0", "name": "平台数据中心", "path": "/data-v2",
		"menuType": "M", "visible": 1, "status": 1, "isFrame": 1,
	}); !success {
		t.Fatalf("平台修改 path 失败: %s", msg)
	}
	_, menusA = mtLogin(t, env, userA, tenantTestPassword)
	nodeA = mtFindMenuById(menusA, dataId)
	if nodeA == nil || nodeA["path"] != "/data-v2" {
		t.Fatalf("租户A 应跟随平台新 path, got %v", nodeA)
	}
	if nodeA["name"] != "我的数据" {
		t.Fatalf("租户A 的自定义名称丢失, got %v", nodeA["name"])
	}
}

/******************************* A7 *******************************/

func TestMenuTenantA7DeletePlatformMenuImpactRules(t *testing.T) {
	env := setupTestEnv(t)
	tenantA, userA := seedTenant(t, env, "menu_a7")
	tokenA := loginAs(t, env, userA, tenantTestPassword)
	docsId := seed.MenuDocsId // 平台叶子菜单：无平台子菜单，便于隔离「租户自建子菜单」这一条件

	// 租户A 把自建菜单挂在平台菜单下（设计已确认允许）
	childId, ok, msg := mtAddMenu(t, env, tokenA, dtos.SysMenuAddRequest{
		ParentId: docsId, Name: "租户A文件夹A7", Path: "/a7/sub", MenuType: "C", Status: 1, Visible: 1, IsFrame: 1,
	})
	if !ok {
		t.Fatalf("把自建菜单挂到平台菜单下失败: %s", msg)
	}
	if row := mtMenuRow(t, env, childId); row.TenantId != tenantA.Id || row.ParentId != docsId {
		t.Fatalf("自建菜单归属/父级不正确: %+v", row)
	}

	// 平台删除 → 拒绝并提示影响面与数量
	success, code, msg := mtDeleteMenu(t, env, env.token, docsId)
	if success {
		t.Fatal("有租户自建子菜单时，平台删除应被拒绝")
	}
	if code != 21714 {
		t.Fatalf("期望 21714 平台菜单被租户自建菜单引用, got %d (%s)", code, msg)
	}
	if !strings.Contains(msg, "租户自建菜单") || !strings.Contains(msg, "1") {
		t.Fatalf("应提示影响面与数量, got %q", msg)
	}

	// 租户删掉自己的自建子菜单，再给平台菜单加一条覆盖记录
	if success, _, msg := mtDeleteMenu(t, env, tokenA, childId); !success {
		t.Fatalf("租户删除自建子菜单失败: %s", msg)
	}
	if success, _, msg := mtSavePref(t, env, tokenA, dtos.SysMenuPrefSaveRequest{
		MenuId: docsId, CustomName: "租户A文档",
	}); !success {
		t.Fatalf("写覆盖记录失败: %s", msg)
	}
	if cnt := mtCountPrefs(t, env, docsId); cnt == 0 {
		t.Fatal("覆盖记录未落库")
	}

	// 仅有覆盖记录 → 允许删除，并级联清理 tenant_menu_pref
	if success, _, msg := mtDeleteMenu(t, env, env.token, docsId); !success {
		t.Fatalf("仅有覆盖记录时应允许删除: %s", msg)
	}
	if cnt := mtCountPrefs(t, env, docsId); cnt != 0 {
		t.Fatalf("删除平台菜单后应级联清理覆盖记录, 剩余 %d", cnt)
	}
	if _, menus := mtLogin(t, env, userA, tenantTestPassword); mtFindMenuById(menus, docsId) != nil {
		t.Fatal("平台菜单删除后租户A 仍可见")
	}
}

/******************************* A8 *******************************/

func TestMenuTenantA8PlatformResetTenantCustomization(t *testing.T) {
	env := setupTestEnv(t)
	tenantA, userA := seedTenant(t, env, "menu_a8")
	tokenA := loginAs(t, env, userA, tenantTestPassword)
	dataId := seed.MenuDataId

	// 自建菜单 + 授权角色 + 覆盖记录（隐藏 + 改名）
	menuId, ok, msg := mtAddMenu(t, env, tokenA, dtos.SysMenuAddRequest{
		ParentId: "0", Name: "租户A自建A8", Path: "/a8/mine", MenuType: "C", Status: 1, Visible: 1, IsFrame: 1,
	})
	if !ok {
		t.Fatalf("租户A建自建菜单失败: %s", msg)
	}
	roleBody, _ := json.Marshal(dtos.SysRoleAddRequest{
		Name: "A8角色", RoleKey: "a8_role", DataScope: "1", Status: 1, MenuIds: []string{menuId},
	})
	if resp := parseResponse(t, doRequest(env.engine, http.MethodPost, "/api/v1/role", tokenA, roleBody)); !resp.Success {
		t.Fatalf("租户A建角色失败: %v", resp.ErrorMsg)
	}
	if success, _, msg := mtSavePref(t, env, tokenA, dtos.SysMenuPrefSaveRequest{
		MenuId: dataId, CustomName: "我的数据A8", Hidden: 1,
	}); !success {
		t.Fatalf("写覆盖记录失败: %s", msg)
	}
	_, menusBefore := mtLogin(t, env, userA, tenantTestPassword)
	if mtFindMenuByPath(menusBefore, "/a8/mine") == nil || mtFindMenuById(menusBefore, dataId) != nil {
		t.Fatal("重置前基线不正确（自建菜单应可见、平台菜单应被隐藏）")
	}

	// 租户用户不能自己重置
	resetBody, _ := json.Marshal(dtos.SysMenuPrefResetRequest{TenantId: tenantA.Id})
	if resp := parseResponse(t, doRequest(env.engine, http.MethodPost, "/api/v1/menu/tenant-reset", tokenA, resetBody)); resp.Success {
		t.Fatal("租户用户不应能重置菜单定制")
	}

	// 平台重置
	if resp := parseResponse(t, doRequest(env.engine, http.MethodPost, "/api/v1/menu/tenant-reset", env.token, resetBody)); !resp.Success {
		t.Fatalf("平台重置失败: %v", resp.ErrorMsg)
	}

	// 库层面：自建菜单、覆盖记录、角色-菜单绑定全部清空
	var menuCnt, prefCnt, roleMenuCnt int64
	env.client.Pool.Table(models.TableMenu).Where("tenant_id = ?", tenantA.Id).Count(&menuCnt)
	env.client.Pool.Table(models.TableTenantMenuPref).Where("tenant_id = ?", tenantA.Id).Count(&prefCnt)
	env.client.Pool.Table(models.TableRoleMenu).Where("menu_id = ?", menuId).Count(&roleMenuCnt)
	if menuCnt != 0 || prefCnt != 0 || roleMenuCnt != 0 {
		t.Fatalf("重置后应清空自建菜单/覆盖/角色绑定, got menu=%d pref=%d rolemenu=%d", menuCnt, prefCnt, roleMenuCnt)
	}

	// 租户A 恢复平台默认：自建菜单消失，平台菜单恢复原名且不再隐藏
	_, menusAfter := mtLogin(t, env, userA, tenantTestPassword)
	if mtFindMenuByPath(menusAfter, "/a8/mine") != nil {
		t.Fatal("重置后租户A 自建菜单仍可见")
	}
	node := mtFindMenuById(menusAfter, dataId)
	if node == nil {
		t.Fatal("重置后租户A 应能看到平台菜单")
	}
	if node["name"] != "数据中心" {
		t.Fatalf("重置后应恢复平台默认名称, got %v", node["name"])
	}
}

/******************************* 权限码登记 *******************************/

// 新增的两个权限码必须同时满足：permissions 常量 + NamedGroups 登记 + middleware 别名 +
// 内置按钮菜单（seed 生成），否则接口会被权限中间件挡住或前端勾不上。
func TestMenuTenantPermissionCodesRegistered(t *testing.T) {
	want := []string{permissions.MenuCustomPref, permissions.MenuCustomReset}
	declared := map[string]bool{}
	for _, group := range permissions.NamedGroups() {
		for _, item := range group {
			declared[item.Value] = true
		}
	}
	seeded := map[string]bool{}
	for _, menu := range seed.DefaultMenus() {
		seeded[menu.Perms] = true
	}
	for _, code := range want {
		if !declared[code] {
			t.Fatalf("权限 %s 未登记到 permissions.NamedGroups()", code)
		}
		if !seeded[code] {
			t.Fatalf("内置菜单/按钮缺少权限 %s", code)
		}
	}
	if middleware.PermMenuCustomPref != permissions.MenuCustomPref ||
		middleware.PermMenuCustomReset != permissions.MenuCustomReset {
		t.Fatal("middleware 权限别名与 permissions 常量不一致")
	}
	if permissions.MenuCustomPref != "system:menu:custom:pref" ||
		permissions.MenuCustomReset != "system:menu:custom:reset" {
		t.Fatal("权限码字面值与设计文档 §2.6 不一致")
	}
}
