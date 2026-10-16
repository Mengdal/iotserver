package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"iotserver/internal/dtos"
	"iotserver/internal/iotserver/core/application/alertcentreapp"
	"iotserver/internal/iotserver/core/application/auditlogapp"
	"iotserver/internal/iotserver/core/application/categorytemplate"
	"iotserver/internal/iotserver/core/application/deviceapp"
	"iotserver/internal/iotserver/core/application/docapp"
	"iotserver/internal/iotserver/core/application/gatewayapp"
	"iotserver/internal/iotserver/core/application/homepageapp"
	"iotserver/internal/iotserver/core/application/languagesdkapp"
	"iotserver/internal/iotserver/core/application/productapp"
	"iotserver/internal/iotserver/core/application/quicknavigationapp"
	"iotserver/internal/iotserver/core/application/resourceapp"
	"iotserver/internal/iotserver/core/application/ruleengineapp"
	"iotserver/internal/iotserver/core/application/sceneapp"
	"iotserver/internal/iotserver/core/application/solutionapp"
	"iotserver/internal/iotserver/core/application/tenantapp"
	"iotserver/internal/iotserver/core/application/thingmodelapp"
	"iotserver/internal/iotserver/core/application/thingmodeltemplate"
	"iotserver/internal/iotserver/core/application/unittemplate"
	"iotserver/internal/iotserver/core/application/userapp"
	"iotserver/internal/iotserver/core/config"
	"iotserver/internal/iotserver/core/container"
	"iotserver/internal/iotserver/core/infrastructure/sqlite"
	interfaces "iotserver/internal/iotserver/core/interface"
	"iotserver/internal/iotserver/core/route"
	pkgcontainer "iotserver/internal/pkg/container"
	"iotserver/internal/pkg/di"
	"iotserver/internal/pkg/httphelper"
	"iotserver/internal/pkg/logger"
	"iotserver/internal/tools/streamclient"
)

type testEnv struct {
	engine      *gin.Engine
	token       string
	fakeEkuiper *fakeEkuiper
	// client 直连数据库，供多租户隔离测试准备数据（建租户/角色/用户）
	client *sqlite.Client
	// auditApp 审查日志应用：测试用 Flush/CleanupExpired 做确定性断言
	auditApp interfaces.AuditLogApp
}

func setupTestEnv(t *testing.T) *testEnv {
	t.Helper()
	gin.SetMode(gin.TestMode)

	lc := logger.NewClient("iotserver-core-test", "ERROR", "")
	cfg := &config.ConfigurationStruct{}

	client, err := sqlite.NewClient(dtos.Configuration{DataSource: filepath.Join(t.TempDir(), "core.db")}, lc)
	if err != nil {
		t.Fatalf("init sqlite: %v", err)
	}
	t.Cleanup(client.CloseSession)

	ctx := context.Background()
	dic := di.NewContainer(di.ServiceConstructorMap{})
	dic.Update(di.ServiceConstructorMap{
		pkgcontainer.LoggingClientInterfaceName: func(get di.Get) interface{} { return lc },
		pkgcontainer.StreamClientName:           func(get di.Get) interface{} { return streamclient.NewStreamClient(lc) },
		container.ConfigurationName:             func(get di.Get) interface{} { return cfg },
		container.DBClientInterfaceName:         func(get di.Get) interface{} { return client },
		container.HomePageAppName:               func(get di.Get) interface{} { return homepageapp.NewHomePageApp(ctx, dic) },
		container.LanguageSDKAppName:            func(get di.Get) interface{} { return languagesdkapp.NewLanguageSDKApp(ctx, dic) },
		container.DriverAppName:                 func(get di.Get) interface{} { return nil },
		container.DriverServiceAppName:          func(get di.Get) interface{} { return nil },
		container.ThingModelAppName:             func(get di.Get) interface{} { return thingmodelapp.NewThingModelApp(ctx, dic) },
		container.CategoryTemplateAppName:       func(get di.Get) interface{} { return categorytemplate.NewCategoryTemplateApp(ctx, dic) },
		container.UnitTemplateAppName:           func(get di.Get) interface{} { return unittemplate.NewUnitTemplateApp(ctx, dic) },
		container.DocsAppName:                   func(get di.Get) interface{} { return docapp.NewDocsApp(ctx, dic) },
		container.QuickNavigationAppName:        func(get di.Get) interface{} { return quicknavigationapp.NewQuickNavigationApp(ctx, dic) },
		container.ThingModelTemplateAppName:     func(get di.Get) interface{} { return thingmodeltemplate.NewThingModelTemplateApp(ctx, dic) },
	})

	userItf := userapp.New(dic)
	dic.Update(di.ServiceConstructorMap{
		container.UserItfName: func(get di.Get) interface{} { return userItf },
	})
	// 审查日志：异步写入 + 过期清理（构造函数内部访问 dic，必须在工厂闭包外构造）
	auditApp := auditlogapp.New(dic)
	dic.Update(di.ServiceConstructorMap{
		container.AuditLogAppName: func(get di.Get) interface{} { return auditApp },
	})
	t.Cleanup(auditApp.Close)
	// 租户管理：删租户保护 / 配额 / 到期相关用例需要走真实路由
	tenantItf := tenantapp.New(dic)
	dic.Update(di.ServiceConstructorMap{
		container.TenantItfName: func(get di.Get) interface{} { return tenantItf },
	})
	// 网关管理：网关配额用例需要走真实路由
	gatewayItf := gatewayapp.New(dic)
	dic.Update(di.ServiceConstructorMap{
		container.GatewayItfName: func(get di.Get) interface{} { return gatewayItf },
	})
	// productApp/alertApp 的构造函数内部会访问 dic，必须在工厂闭包外构造（闭包内构造会在解析时自锁）
	productItf := productapp.NewProductApp(ctx, dic)
	dic.Update(di.ServiceConstructorMap{
		container.ProductAppName: func(get di.Get) interface{} { return productItf },
	})
	alertItf := alertcentreapp.NewAlertCentreApp(ctx, dic)
	dic.Update(di.ServiceConstructorMap{
		container.AlertRuleAppName: func(get di.Get) interface{} { return alertItf },
	})
	// deviceApp 的构造函数内部会访问 dic，必须在工厂闭包外构造（闭包内构造会在解析时自锁）
	deviceItf := deviceapp.NewDeviceApp(ctx, dic)
	dic.Update(di.ServiceConstructorMap{
		container.DeviceItfName: func(get di.Get) interface{} { return deviceItf },
	})
	// 高级能力 - 资源管理（在工厂闭包外构造：构造函数内部会访问 dic，闭包内构造会自锁）
	resourceItf := resourceapp.New(dic)
	dic.Update(di.ServiceConstructorMap{
		container.ResourceItfName: func(get di.Get) interface{} { return resourceItf },
	})
	// 高级能力 - 规则引擎：用假 eKuiper 客户端，验证下发内容而不依赖真实 eKuiper
	// 同样必须在工厂闭包外构造（构造函数内部访问 dic，闭包内构造会自锁）
	fakeEkuiper := newFakeEkuiper()
	dic.Update(di.ServiceConstructorMap{
		container.EkuiperAppName: func(get di.Get) interface{} { return fakeEkuiper },
	})
	ruleEngineItf := ruleengineapp.NewWithoutMonitor(dic)
	dic.Update(di.ServiceConstructorMap{
		container.RuleEngineItfName: func(get di.Get) interface{} { return ruleEngineItf },
	})
	// 高级能力 - 场景联动（同样在工厂闭包外构造；测试不启动定时调度）
	sceneItf := sceneapp.NewWithoutScheduler(dic)
	dic.Update(di.ServiceConstructorMap{
		container.SceneItfName: func(get di.Get) interface{} { return sceneItf },
	})
	// 数据存储：产品发布/建设备会建库表，e2e 用假客户端（不入库，只保证链路可达）
	dic.Update(di.ServiceConstructorMap{
		container.DataDBClientInterfaceName: func(get di.Get) interface{} { return fakeDataDB{} },
	})
	// 设备接入 - 方案（构造函数内部访问 dic，必须在工厂闭包外构造）
	machineTemplateItf := solutionapp.New(dic)
	dic.Update(di.ServiceConstructorMap{
		container.SolutionItfName: func(get di.Get) interface{} { return machineTemplateItf },
	})

	if err = userItf.MigrateSystemData(); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	userItf.InitJwtKey()

	engine := gin.New()
	route.RegisterGateway(engine, dic)
	// 与服务真实启动顺序一致（route.go: RegisterGateway → RegisterOpenApi），
	// 后者注册全局 NoRoute（含尾随空白路径容错）
	route.RegisterOpenApi(engine, dic)

	// 登录拿 token
	body, _ := json.Marshal(dtos.LoginRequest{Username: "admin", Password: "admin123"})
	w := doRequest(engine, http.MethodPost, "/api/v1/auth/login", "", body)
	if w.Code != http.StatusOK {
		t.Fatalf("login status %d", w.Code)
	}
	var loginResp httphelper.CommonResponse
	if err = json.Unmarshal(w.Body.Bytes(), &loginResp); err != nil {
		t.Fatalf("decode login: %v", err)
	}
	if !loginResp.Success {
		t.Fatalf("login failed: %v", loginResp.ErrorMsg)
	}
	result, ok := loginResp.Result.(map[string]interface{})
	if !ok {
		t.Fatalf("unexpected login result type %T", loginResp.Result)
	}
	token, _ := result["token"].(string)
	if token == "" {
		t.Fatal("token empty")
	}
	return &testEnv{engine: engine, token: token, fakeEkuiper: fakeEkuiper, client: client, auditApp: auditApp}
}

func doRequest(engine *gin.Engine, method, path, token string, body []byte) *httptest.ResponseRecorder {
	var reader *bytes.Reader
	if body == nil {
		reader = bytes.NewReader([]byte{})
	} else {
		reader = bytes.NewReader(body)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("x-token", token)
	}
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	return w
}

func parseResponse(t *testing.T, w *httptest.ResponseRecorder) httphelper.CommonResponse {
	t.Helper()
	// httptest.ResponseRecorder 会累积多次写入，仅解析第一个 JSON 对象
	body := w.Body.Bytes()
	dec := json.NewDecoder(bytes.NewReader(body))
	var resp httphelper.CommonResponse
	if err := dec.Decode(&resp); err != nil {
		t.Fatalf("decode response %s: %v", string(body), err)
	}
	return resp
}

func TestLoginReturnsPermissionsAndMenus(t *testing.T) {
	env := setupTestEnv(t)
	w := doRequest(env.engine, http.MethodGet, "/api/v1/user/getinfo", env.token, nil)
	resp := parseResponse(t, w)
	if !resp.Success {
		t.Fatalf("getinfo failed: %v", resp.ErrorMsg)
	}
	result := resp.Result.(map[string]interface{})
	perms, _ := result["permissions"].([]interface{})
	if len(perms) == 0 {
		t.Fatal("permissions empty")
	}
	menus, _ := result["menus"].([]interface{})
	if len(menus) == 0 {
		t.Fatal("menus empty")
	}
	if admin, _ := result["admin"].(bool); !admin {
		t.Fatal("expect admin true")
	}
}

func TestSysUserCrudFlow(t *testing.T) {
	env := setupTestEnv(t)

	// 创建角色
	roleBody, _ := json.Marshal(dtos.SysRoleAddRequest{Name: "测试角色", RoleKey: "tester", Status: 1, DataScope: "1"})
	w := doRequest(env.engine, http.MethodPost, "/api/v1/role", env.token, roleBody)
	if resp := parseResponse(t, w); !resp.Success {
		t.Fatalf("add role failed: %v", resp.ErrorMsg)
	}

	// 查询角色列表拿到 id
	w = doRequest(env.engine, http.MethodGet, "/api/v1/role?page=1&pageSize=10", env.token, nil)
	resp := parseResponse(t, w)
	if !resp.Success {
		t.Fatalf("role list failed: %v", resp.ErrorMsg)
	}
	page := resp.Result.(map[string]interface{})
	roles := page["list"].([]interface{})
	if len(roles) == 0 {
		t.Fatal("role list empty")
	}
	var roleId string
	for _, item := range roles {
		role := item.(map[string]interface{})
		if role["roleKey"] == "tester" {
			roleId, _ = role["id"].(string)
		}
	}
	if roleId == "" {
		t.Fatal("role id not found")
	}

	// 新增用户
	userBody, _ := json.Marshal(dtos.SysUserAddRequest{
		Username: "u_tester",
		Password: "123456",
		Nickname: "测试用户",
		Status:   1,
		RoleIds:  []string{roleId},
	})
	w = doRequest(env.engine, http.MethodPost, "/api/v1/sys-user", env.token, userBody)
	if resp = parseResponse(t, w); !resp.Success {
		t.Fatalf("add user failed: %v", resp.ErrorMsg)
	}

	// 列表查询
	w = doRequest(env.engine, http.MethodGet, "/api/v1/sys-user?page=1&pageSize=10", env.token, nil)
	resp = parseResponse(t, w)
	if !resp.Success {
		t.Fatalf("user list failed: %v", resp.ErrorMsg)
	}
	userPage := resp.Result.(map[string]interface{})
	userList := userPage["list"].([]interface{})
	var userId string
	for _, item := range userList {
		user := item.(map[string]interface{})
		if user["username"] == "u_tester" {
			userId, _ = user["id"].(string)
			roleNames, _ := user["roleNames"].([]interface{})
			if len(roleNames) != 1 || roleNames[0] != "测试角色" {
				t.Fatalf("expect roleNames [测试角色], got %v", roleNames)
			}
		}
	}
	if userId == "" {
		t.Fatal("user id not found")
	}

	// 新用户用该账号登录应成功
	loginBody, _ := json.Marshal(dtos.LoginRequest{Username: "u_tester", Password: "123456"})
	w = doRequest(env.engine, http.MethodPost, "/api/v1/auth/login", "", loginBody)
	if resp = parseResponse(t, w); !resp.Success {
		t.Fatalf("new user login failed: %v", resp.ErrorMsg)
	}

	// 新用户只有 system:user:list 之外的权限，访问角色列表应被拒绝
	newToken := resp.Result.(map[string]interface{})["token"].(string)
	w = doRequest(env.engine, http.MethodGet, "/api/v1/role?page=1&pageSize=10", newToken, nil)
	denied := parseResponse(t, w)
	if denied.Success {
		t.Fatalf("expect permission denied for role list")
	}
	if denied.ErrorCode != 21708 {
		t.Fatalf("expect errorCode 21708, got %d (%s)", denied.ErrorCode, denied.ErrorMsg)
	}

	// 停用用户
	w = doRequest(env.engine, http.MethodPut, "/api/v1/sys-user/status/"+userId, env.token, []byte(`{"status":0}`))
	if resp = parseResponse(t, w); !resp.Success {
		t.Fatalf("change status failed: %v", resp.ErrorMsg)
	}
	// 停用后不允许登录
	w = doRequest(env.engine, http.MethodPost, "/api/v1/auth/login", "", loginBody)
	if resp = parseResponse(t, w); resp.Success {
		t.Fatal("expect disabled account login to fail")
	}

	// 重置密码
	w = doRequest(env.engine, http.MethodPut, "/api/v1/pwd/reset/"+userId, env.token, []byte(`{"password":"abcdef"}`))
	if resp = parseResponse(t, w); !resp.Success {
		t.Fatalf("reset password failed: %v", resp.ErrorMsg)
	}

	// 删除用户
	w = doRequest(env.engine, http.MethodDelete, "/api/v1/sys-user", env.token, []byte(`{"ids":["`+userId+`"]}`))
	if resp = parseResponse(t, w); !resp.Success {
		t.Fatalf("delete user failed: %v", resp.ErrorMsg)
	}
}

func TestSysDeptMenuConfigFlow(t *testing.T) {
	env := setupTestEnv(t)

	// 部门树
	w := doRequest(env.engine, http.MethodGet, "/api/v1/dept-tree", env.token, nil)
	resp := parseResponse(t, w)
	if !resp.Success {
		t.Fatalf("dept tree failed: %v", resp.ErrorMsg)
	}
	if len(resp.Result.([]interface{})) == 0 {
		t.Fatal("dept tree empty")
	}

	// 新增部门
	deptBody, _ := json.Marshal(dtos.SysDeptAddRequest{ParentId: "0", Name: "研发中心", Status: 1})
	w = doRequest(env.engine, http.MethodPost, "/api/v1/dept", env.token, deptBody)
	if resp = parseResponse(t, w); !resp.Success {
		t.Fatalf("add dept failed: %v", resp.ErrorMsg)
	}

	// 菜单树
	w = doRequest(env.engine, http.MethodGet, "/api/v1/menu/tree", env.token, nil)
	resp = parseResponse(t, w)
	if !resp.Success {
		t.Fatalf("menu tree failed: %v", resp.ErrorMsg)
	}
	if len(resp.Result.([]interface{})) == 0 {
		t.Fatal("menu tree empty")
	}

	// 新增菜单
	menuBody, _ := json.Marshal(dtos.SysMenuAddRequest{ParentId: "0", Name: "自定义菜单", Path: "/custom", MenuType: "C", Status: 1, Visible: 1, IsFrame: 1})
	w = doRequest(env.engine, http.MethodPost, "/api/v1/menu", env.token, menuBody)
	if resp = parseResponse(t, w); !resp.Success {
		t.Fatalf("add menu failed: %v", resp.ErrorMsg)
	}

	// 系统配置
	w = doRequest(env.engine, http.MethodGet, "/api/v1/system-setup?isAll=true", env.token, nil)
	resp = parseResponse(t, w)
	if !resp.Success {
		t.Fatalf("config search failed: %v", resp.ErrorMsg)
	}
	cfgPage := resp.Result.(map[string]interface{})
	if len(cfgPage["list"].([]interface{})) == 0 {
		t.Fatal("config list empty")
	}

	// 批量保存配置
	saveBody, _ := json.Marshal(dtos.SysSetupRequest{Items: []dtos.SysSetupItem{{ConfigKey: "platform.name", ConfigValue: "测试平台"}}})
	w = doRequest(env.engine, http.MethodPost, "/api/v1/system-setup/save", env.token, saveBody)
	if resp = parseResponse(t, w); !resp.Success {
		t.Fatalf("save config failed: %v", resp.ErrorMsg)
	}
	w = doRequest(env.engine, http.MethodGet, "/api/v1/system-setup?isAll=true", env.token, nil)
	resp = parseResponse(t, w)
	found := false
	for _, item := range resp.Result.(map[string]interface{})["list"].([]interface{}) {
		cfg := item.(map[string]interface{})
		if cfg["configKey"] == "platform.name" && cfg["configValue"] == "测试平台" {
			found = true
		}
	}
	if !found {
		t.Fatal("config value not persisted")
	}

	// 个人中心
	w = doRequest(env.engine, http.MethodGet, "/api/v1/user/profile", env.token, nil)
	resp = parseResponse(t, w)
	if !resp.Success {
		t.Fatalf("profile failed: %v", resp.ErrorMsg)
	}
	profile := resp.Result.(map[string]interface{})
	user := profile["user"].(map[string]interface{})
	if user["username"] != "admin" {
		t.Fatalf("expect admin profile, got %v", user["username"])
	}

	// 登入日志应有记录
	w = doRequest(env.engine, http.MethodGet, "/api/v1/sys-login-log?page=1&pageSize=10", env.token, nil)
	resp = parseResponse(t, w)
	if !resp.Success {
		t.Fatalf("login log failed: %v", resp.ErrorMsg)
	}
	logs := resp.Result.(map[string]interface{})["list"].([]interface{})
	if len(logs) == 0 {
		t.Fatal("login log empty")
	}
}

func TestUnauthorizedWithoutToken(t *testing.T) {
	env := setupTestEnv(t)
	w := doRequest(env.engine, http.MethodGet, "/api/v1/sys-user", "", nil)
	resp := parseResponse(t, w)
	if resp.Success {
		t.Fatal("expect unauthorized")
	}
	if resp.ErrorCode != 10006 {
		t.Fatalf("expect 10006, got %d", resp.ErrorCode)
	}
}

// 角色只勾选子菜单（没勾父目录）时，登录下发的菜单树要自动补全父目录，否则前端侧边栏挂不出这棵子树
func TestNonAdminMenusIncludeParentDirs(t *testing.T) {
	env := setupTestEnv(t)

	var findMenu func(items []interface{}, path string) map[string]interface{}
	findMenu = func(items []interface{}, path string) map[string]interface{} {
		for _, it := range items {
			node := it.(map[string]interface{})
			if node["path"] == path {
				return node
			}
			if children, ok := node["children"].([]interface{}); ok {
				if hit := findMenu(children, path); hit != nil {
					return hit
				}
			}
		}
		return nil
	}

	w := doRequest(env.engine, http.MethodGet, "/api/v1/menu/tree", env.token, nil)
	resp := parseResponse(t, w)
	if !resp.Success {
		t.Fatalf("menu tree failed: %v", resp.ErrorMsg)
	}
	userMenu := findMenu(resp.Result.([]interface{}), "/admin/sysUser")
	if userMenu == nil {
		t.Fatal("seed menu /admin/sysUser not found")
	}
	parentId, _ := userMenu["parentId"].(string)
	if parentId == "" || parentId == "0" {
		t.Fatalf("expect /admin/sysUser has a parent dir, got %q", parentId)
	}

	// 只把子菜单授权给角色（模拟漏勾父目录的历史数据）
	roleBody, _ := json.Marshal(dtos.SysRoleAddRequest{
		Name: "父目录补全角色", RoleKey: "parentfill", Sort: 9, DataScope: "1", Status: 1,
		MenuIds: []string{userMenu["id"].(string)},
	})
	w = doRequest(env.engine, http.MethodPost, "/api/v1/role", env.token, roleBody)
	if resp = parseResponse(t, w); !resp.Success {
		t.Fatalf("add role failed: %v", resp.ErrorMsg)
	}

	w = doRequest(env.engine, http.MethodGet, "/api/v1/role?page=1&pageSize=50", env.token, nil)
	resp = parseResponse(t, w)
	if !resp.Success {
		t.Fatalf("role list failed: %v", resp.ErrorMsg)
	}
	var roleId string
	for _, it := range resp.Result.(map[string]interface{})["list"].([]interface{}) {
		role := it.(map[string]interface{})
		if role["roleKey"] == "parentfill" {
			roleId = role["id"].(string)
		}
	}
	if roleId == "" {
		t.Fatal("created role not found")
	}

	userBody, _ := json.Marshal(dtos.SysUserAddRequest{
		Username: "parentfilluser", Password: "parent123", Nickname: "补全测试", Status: 1,
		RoleIds: []string{roleId},
	})
	w = doRequest(env.engine, http.MethodPost, "/api/v1/sys-user", env.token, userBody)
	if resp = parseResponse(t, w); !resp.Success {
		t.Fatalf("add user failed: %v", resp.ErrorMsg)
	}

	// 非管理员登录：菜单树里应出现父目录，且父目录下挂着被授权的子菜单
	loginBody, _ := json.Marshal(dtos.LoginRequest{Username: "parentfilluser", Password: "parent123"})
	w = doRequest(env.engine, http.MethodPost, "/api/v1/auth/login", "", loginBody)
	resp = parseResponse(t, w)
	if !resp.Success {
		t.Fatalf("login failed: %v", resp.ErrorMsg)
	}
	result := resp.Result.(map[string]interface{})
	if admin, _ := result["admin"].(bool); admin {
		t.Fatal("expect non-admin user")
	}
	menus, _ := result["menus"].([]interface{})
	if len(menus) == 0 {
		t.Fatal("non-admin menus empty: 父目录没有补全")
	}
	if findMenu(menus, "/admin/sysUser") == nil {
		t.Fatalf("granted child menu missing in tree: %v", menus)
	}
	root := menus[0].(map[string]interface{})
	if root["id"] != parentId {
		t.Fatalf("expect补全的父目录 %s 作为根节点, got %v", parentId, root["id"])
	}

	// 权限集合不应因为补全父目录而变多
	perms, _ := result["permissions"].([]interface{})
	for _, p := range perms {
		if p == "system:menu:list" {
			t.Fatalf("补全父目录后不应引入父目录的权限: %v", perms)
		}
	}
}

// 权限字符（role_key）只在同一租户内唯一：同租户重复要被拒绝，不同租户可重复。
//
// 背景：角色是租户隔离表，但"平台视图"下 GORM 插件不过滤，
// 若用 WithCtx 直接查重会跨租户误判，因此实现里显式带上 tenant_id。
func TestSysRoleKeyUniquePerTenant(t *testing.T) {
	env := setupTestEnv(t)

	bodyFirst, _ := json.Marshal(dtos.SysRoleAddRequest{Name: "运维同名", RoleKey: "ops_same", Status: 1, DataScope: "1"})
	bodyDup, _ := json.Marshal(dtos.SysRoleAddRequest{Name: "运维同名2", RoleKey: "ops_same", Status: 1, DataScope: "1"})

	// 平台级角色
	if resp := parseResponse(t, doRequest(env.engine, http.MethodPost, "/api/v1/role", env.token, bodyFirst)); !resp.Success {
		t.Fatalf("平台建角色失败: %v", resp.ErrorMsg)
	}
	// 平台内重复权限字符 → 拒绝
	if resp := parseResponse(t, doRequest(env.engine, http.MethodPost, "/api/v1/role", env.token, bodyDup)); resp.Success {
		t.Fatal("同一租户内重复的权限字符应被拒绝")
	}

	// 租户A：与平台同名同权限字符也要能建（不同租户互不影响）
	_, userA := seedTenant(t, env, "rolekeya")
	tokenA := loginAs(t, env, userA, tenantTestPassword)
	if resp := parseResponse(t, doRequest(env.engine, http.MethodPost, "/api/v1/role", tokenA, bodyFirst)); !resp.Success {
		t.Fatalf("租户A建同名同权限字符角色应成功: %v", resp.ErrorMsg)
	}
	if resp := parseResponse(t, doRequest(env.engine, http.MethodPost, "/api/v1/role", tokenA, bodyDup)); resp.Success {
		t.Fatal("租户A内重复的权限字符应被拒绝")
	}

	// 租户B：同样可以建（两个租户各自有一份 ops_same）
	_, userB := seedTenant(t, env, "rolekeyb")
	tokenB := loginAs(t, env, userB, tenantTestPassword)
	if resp := parseResponse(t, doRequest(env.engine, http.MethodPost, "/api/v1/role", tokenB, bodyFirst)); !resp.Success {
		t.Fatalf("租户B建同名同权限字符角色应成功: %v", resp.ErrorMsg)
	}
}
