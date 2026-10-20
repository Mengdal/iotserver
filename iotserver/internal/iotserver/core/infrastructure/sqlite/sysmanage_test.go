package sqlite

import (
	"path/filepath"
	"testing"

	"golang.org/x/crypto/bcrypt"
	"iotserver/internal/dtos"
	"iotserver/internal/models"
	"iotserver/internal/pkg/permissions"
	"iotserver/internal/pkg/seed"
)

func newTestClient(t *testing.T) *Client {
	t.Helper()
	dsn := filepath.Join(t.TempDir(), "test.db")
	c, err := NewClient(dtos.Configuration{DataSource: dsn}, nil)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	return c
}

func TestMigrateSystemTablesSeedsData(t *testing.T) {
	c := newTestClient(t)
	if err := c.MigrateSystemTables(); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	// 幂等：再执行一次不应报错
	if err := c.MigrateSystemTables(); err != nil {
		t.Fatalf("migrate twice: %v", err)
	}

	// 内置工业方案：三个模板列表各 7 条，且重复迁移不重复写入
	templates, categories, thingModels, err := seed.DefaultSolutions()
	if err != nil {
		t.Fatalf("load machine templates: %v", err)
	}
	if len(templates) < 7 {
		t.Fatalf("expect at least 7 machine templates, got %d", len(templates))
	}
	tplRows, err := c.SolutionSearch("", "")
	if err != nil {
		t.Fatalf("machine template search: %v", err)
	}
	if len(tplRows) != len(templates) {
		t.Fatalf("expect %d machine templates in db, got %d", len(templates), len(tplRows))
	}
	tpl, err := c.SolutionByKey("", "injection_molding")
	if err != nil {
		t.Fatalf("machine template by key: %v", err)
	}
	if tpl.Name == "" || tpl.ThingModelJson == "" || tpl.AlarmRulesJson == "" {
		t.Fatalf("注塑机模板内容为空: %+v", tpl)
	}
	if len(categories) != 0 {
		var catCount int64
		if err := c.Pool.Table("category_template").
			Where("category_key LIKE ?", seed.CategoryTemplateKeyPrefix+"%").Count(&catCount).Error; err != nil {
			t.Fatalf("count category template: %v", err)
		}
		if int(catCount) != len(categories) {
			t.Fatalf("expect %d category templates, got %d", len(categories), catCount)
		}
	}
	var tmCount int64
	if err := c.Pool.Table("thing_model_template").
		Where("category_key LIKE ?", seed.CategoryTemplateKeyPrefix+"%").Count(&tmCount).Error; err != nil {
		t.Fatalf("count thing model template: %v", err)
	}
	if int(tmCount) != len(thingModels) {
		t.Fatalf("expect %d thing model templates, got %d", len(thingModels), tmCount)
	}

	menus, err := c.SysMenuAll("")
	if err != nil {
		t.Fatalf("menu all: %v", err)
	}
	if len(menus) != len(seed.DefaultMenus()) {
		t.Fatalf("expect %d menus, got %d", len(seed.DefaultMenus()), len(menus))
	}

	users, total, err := c.SysUserSearch(0, -1, dtos.SysUserSearchRequest{IsAll: true})
	if err != nil {
		t.Fatalf("user search: %v", err)
	}
	if total != 1 || len(users) != 1 {
		t.Fatalf("expect 1 user, got %d", total)
	}
	admin := users[0]
	if admin.Username != "admin" {
		t.Fatalf("expect admin, got %s", admin.Username)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(admin.Password), []byte(DefaultAdminPassword)); err != nil {
		t.Fatalf("default admin password mismatch: %v", err)
	}

	// 超管应拿到全部权限
	menusOfUser, err := c.RoleMenusByUserId(admin.Id, "")
	if err != nil {
		t.Fatalf("role menus by user: %v", err)
	}
	if len(menusOfUser) != len(menus) {
		t.Fatalf("expect admin owns %d menus, got %d", len(menus), len(menusOfUser))
	}

	cfgs, cfgTotal, err := c.SysConfigSearch(0, -1, dtos.SysConfigSearchRequest{IsAll: true})
	if err != nil {
		t.Fatalf("config search: %v", err)
	}
	if int(cfgTotal) != len(seed.DefaultConfigs()) {
		t.Fatalf("expect %d configs, got %d", len(seed.DefaultConfigs()), cfgTotal)
	}
	if len(cfgs) == 0 {
		t.Fatal("configs empty")
	}

	depts, err := c.SysDeptAll()
	if err != nil {
		t.Fatalf("dept all: %v", err)
	}
	if len(depts) != len(seed.DefaultDepts()) {
		t.Fatalf("expect %d depts, got %d", len(seed.DefaultDepts()), len(depts))
	}
}

// TestSysMenuTenantScope 菜单多租户改造（P1-1/P1-2）的存储层行为：
// 作用域过滤、租户自建菜单、展示层覆盖表，以及「重跑 seed 不删租户菜单/不覆盖租户定制」。
func TestSysMenuTenantScope(t *testing.T) {
	c := newTestClient(t)
	if err := c.MigrateSystemTables(); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	global, err := c.SysMenuAll("")
	if err != nil {
		t.Fatalf("menu all platform: %v", err)
	}
	globalCount := len(global)
	for _, m := range global {
		if m.TenantId != "" {
			t.Fatalf("平台视图混入了租户菜单: %+v", m)
		}
	}

	// 租户自建菜单：允许挂在平台菜单下
	if _, err = c.AddSysMenu(models.Menu{
		Id: "tenant-menu-1", ParentId: seed.MenuDocsId, Name: "租户自建", Path: "/t/only",
		MenuType: models.MenuTypeMenu, Status: 1, Visible: 1, IsFrame: 1, TenantId: "t1",
	}); err != nil {
		t.Fatalf("add tenant menu: %v", err)
	}
	scoped, err := c.SysMenuAll("t1")
	if err != nil {
		t.Fatalf("menu all tenant: %v", err)
	}
	if len(scoped) != globalCount+1 {
		t.Fatalf("租户视图应为平台 %d + 自建 1, got %d", globalCount, len(scoped))
	}
	other, err := c.SysMenuAll("t2")
	if err != nil {
		t.Fatalf("menu all other tenant: %v", err)
	}
	if len(other) != globalCount {
		t.Fatalf("其他租户不应看到 t1 的自建菜单, got %d", len(other))
	}
	if _, total, err := c.SysMenuSearch(0, -1, "t2", dtos.SysMenuSearchRequest{IsAll: true}); err != nil || int(total) != globalCount {
		t.Fatalf("租户 t2 列表应为 %d, got %d (%v)", globalCount, total, err)
	}
	if _, total, err := c.SysMenuSearch(0, -1, "t1", dtos.SysMenuSearchRequest{IsAll: true}); err != nil || int(total) != globalCount+1 {
		t.Fatalf("租户 t1 列表应为 %d, got %d (%v)", globalCount+1, total, err)
	}

	// 展示层覆盖：(tenant_id, menu_id) 幂等 upsert
	pref, err := c.UpsertSysMenuPref(models.TenantMenuPref{TenantId: "t1", MenuId: seed.MenuDocsId, Hidden: 1})
	if err != nil {
		t.Fatalf("upsert pref: %v", err)
	}
	if _, err = c.UpsertSysMenuPref(models.TenantMenuPref{
		TenantId: "t1", MenuId: seed.MenuDocsId, Hidden: 0, CustomName: "改名",
	}); err != nil {
		t.Fatalf("upsert pref twice: %v", err)
	}
	prefs, err := c.SysMenuPrefsByTenant("t1")
	if err != nil {
		t.Fatalf("prefs by tenant: %v", err)
	}
	if len(prefs) != 1 || prefs[0].Id != pref.Id || prefs[0].CustomName != "改名" || prefs[0].Hidden != 0 {
		t.Fatalf("同一租户同一菜单应只有一条覆盖记录, got %+v", prefs)
	}

	// 重跑迁移/seed：不得删除租户自建菜单，也不得覆盖租户定制
	if err = c.MigrateSystemTables(); err != nil {
		t.Fatalf("migrate twice: %v", err)
	}
	if m, mErr := c.SysMenuById("tenant-menu-1"); mErr != nil || m.TenantId != "t1" {
		t.Fatalf("重跑 seed 把租户自建菜单删了: %+v (%v)", m, mErr)
	}
	if prefs, err = c.SysMenuPrefsByTenant("t1"); err != nil || len(prefs) != 1 || prefs[0].CustomName != "改名" {
		t.Fatalf("重跑 seed 覆盖了租户菜单定制: %+v (%v)", prefs, err)
	}
	if cnt, cErr := c.SysMenuCountTenantChildrenByParentIds([]string{seed.MenuDocsId}); cErr != nil || cnt != 1 {
		t.Fatalf("应统计到 1 个租户自建子菜单, got %d (%v)", cnt, cErr)
	}

	// 删除平台菜单时的级联：清理所有租户对该菜单的覆盖记录
	if err = c.DeleteSysMenuPrefsByMenuIds([]string{seed.MenuDocsId}); err != nil {
		t.Fatalf("delete prefs by menu ids: %v", err)
	}
	if prefs, _ = c.SysMenuPrefsByTenant("t1"); len(prefs) != 0 {
		t.Fatalf("级联清理后不应残留覆盖记录: %+v", prefs)
	}
	if err = c.DeleteSysMenuByIds([]string{"tenant-menu-1"}); err != nil {
		t.Fatalf("delete tenant menu: %v", err)
	}
}

func TestSysUserRoleCRUD(t *testing.T) {
	c := newTestClient(t)
	if err := c.MigrateSystemTables(); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	role, err := c.AddSysRole(models.Role{Name: "运营", RoleKey: "operator", Status: 1, DataScope: models.DataScopeAll})
	if err != nil {
		t.Fatalf("add role: %v", err)
	}
	// 按权限标识查找「用户管理查询」按钮，避免依赖按钮 ID 生成规则
	allMenus, err := c.SysMenuAll("")
	if err != nil {
		t.Fatalf("menu all: %v", err)
	}
	var userQueryMenu models.Menu
	found := false
	for _, m := range allMenus {
		if m.Perms == permissions.UserQuery {
			userQueryMenu = m
			found = true
			break
		}
	}
	if !found {
		t.Fatal("未找到权限为 system:user:query 的菜单")
	}
	if err := c.SetRoleMenus(role.Id, []string{seed.MenuAdminUser, userQueryMenu.Id}); err != nil {
		t.Fatalf("set role menus: %v", err)
	}
	menuIds, err := c.GetRoleMenuIds(role.Id)
	if err != nil {
		t.Fatalf("get role menus: %v", err)
	}
	if len(menuIds) != 2 {
		t.Fatalf("expect 2 menus, got %d", len(menuIds))
	}

	user, err := c.AddSysUser(models.User{Username: "tester", Password: "x", Status: 1, DeptId: seed.DeptRootId})
	if err != nil {
		t.Fatalf("add user: %v", err)
	}
	if err := c.SetUserRoles(user.Id, []string{role.Id}); err != nil {
		t.Fatalf("set user roles: %v", err)
	}
	names, err := c.RoleNamesByUserId(user.Id)
	if err != nil {
		t.Fatalf("role names: %v", err)
	}
	if len(names) != 1 || names[0] != "运营" {
		t.Fatalf("expect role 运营, got %v", names)
	}
	userMenus, err := c.RoleMenusByUserId(user.Id, "")
	if err != nil {
		t.Fatalf("user menus: %v", err)
	}
	if len(userMenus) != 2 {
		t.Fatalf("expect 2 user menus, got %d", len(userMenus))
	}

	// 重复用户名应报错
	if _, err = c.AddSysUser(models.User{Username: "tester", Password: "x", Status: 1}); err == nil {
		t.Fatal("expect duplicate username error")
	}

	// 级联清理
	if err := c.DeleteUserRolesByRoleIds([]string{role.Id}); err != nil {
		t.Fatalf("delete user roles: %v", err)
	}
	if err := c.DeleteRoleMenusByRoleIds([]string{role.Id}); err != nil {
		t.Fatalf("delete role menus: %v", err)
	}
	if err := c.DeleteSysRoleByIds([]string{role.Id}); err != nil {
		t.Fatalf("delete role: %v", err)
	}
}

func TestSysAuditLogAndConfig(t *testing.T) {
	c := newTestClient(t)
	if err := c.MigrateSystemTables(); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	log := models.AuditLog{
		Username: "admin", Ip: "127.0.0.1", Status: 1, Message: "登录成功", LoginTime: 1000,
		Type: models.AuditTypeLogin, Module: models.AuditModuleAuth, Action: models.AuditActionLogin,
	}
	if err := c.AddSysAuditLog(log); err != nil {
		t.Fatalf("add log: %v", err)
	}
	logs, total, err := c.SysAuditLogSearch(0, -1, dtos.SysAuditLogSearchRequest{Username: "admin", IsAll: true})
	if err != nil {
		t.Fatalf("log search: %v", err)
	}
	if total != 1 || len(logs) != 1 {
		t.Fatalf("expect 1 log, got %d", total)
	}
	if err := c.ClearSysAuditLog(); err != nil {
		t.Fatalf("clear log: %v", err)
	}
	if _, total, _ = c.SysAuditLogSearch(0, -1, dtos.SysAuditLogSearchRequest{IsAll: true}); total != 0 {
		t.Fatalf("expect 0 log after clear, got %d", total)
	}

	if err := c.SaveSysConfigValues([]dtos.SysSetupItem{{ConfigKey: "platform.name", ConfigValue: "新名称"}}); err != nil {
		t.Fatalf("save config: %v", err)
	}
	cfg, err := c.SysConfigByKey("platform.name")
	if err != nil {
		t.Fatalf("config by key: %v", err)
	}
	if cfg.ConfigValue != "新名称" {
		t.Fatalf("expect 新名称, got %s", cfg.ConfigValue)
	}
	// 新 key 自动创建
	if err := c.SaveSysConfigValues([]dtos.SysSetupItem{{ConfigKey: "custom.new", ConfigValue: "1"}}); err != nil {
		t.Fatalf("save new config: %v", err)
	}
	if _, err = c.SysConfigByKey("custom.new"); err != nil {
		t.Fatalf("new config not created: %v", err)
	}
}

// 审查日志表改名迁移：老库里的 sys_login_log（含数据）必须原样搬到 sys_audit_log，
// 并回填 type=login / module=认证 / created=login_time。
func TestMigrateLegacyLoginLogTable(t *testing.T) {
	c := newTestClient(t)

	// 造一个「改造前」的库：旧表 + 一条失败的登入记录
	legacy := `CREATE TABLE sys_login_log (
		id TEXT PRIMARY KEY, tenant_id TEXT, username TEXT, ip TEXT,
		browser TEXT, os TEXT, status INTEGER, message TEXT, login_time INTEGER)`
	if err := c.Pool.Exec(legacy).Error; err != nil {
		t.Fatalf("create legacy table: %v", err)
	}
	if err := c.Pool.Exec(`INSERT INTO sys_login_log
		(id, tenant_id, username, ip, status, message, login_time)
		VALUES ('legacy-1', '', 'admin', '127.0.0.1', 0, '密码错误', 1700000000000)`).Error; err != nil {
		t.Fatalf("insert legacy log: %v", err)
	}

	if err := c.MigrateSystemTables(); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if err := c.MigrateSystemTables(); err != nil {
		t.Fatalf("migrate twice: %v", err)
	}

	if c.Pool.Migrator().HasTable("sys_login_log") {
		t.Fatal("旧表 sys_login_log 应已改名")
	}
	if !c.Pool.Migrator().HasTable(models.TableAuditLog) {
		t.Fatal("新表 sys_audit_log 不存在")
	}
	logs, total, err := c.SysAuditLogSearch(0, -1, dtos.SysAuditLogSearchRequest{IsAll: true})
	if err != nil {
		t.Fatalf("query audit log: %v", err)
	}
	if total != 1 || len(logs) != 1 {
		t.Fatalf("历史登入记录应保留 1 条，实际 %d", total)
	}
	got := logs[0]
	if got.Id != "legacy-1" || got.Type != models.AuditTypeLogin || got.Module != models.AuditModuleAuth {
		t.Fatalf("历史记录回填不正确: %+v", got)
	}
	if got.Created != 1700000000000 || got.LoginTime != 1700000000000 {
		t.Fatalf("created 应由 login_time 回填: created=%d loginTime=%d", got.Created, got.LoginTime)
	}
	if got.Status != models.LoginStatusFail {
		t.Fatalf("失败状态应保持 0，实际 %d", got.Status)
	}
	// 索引已建立
	for _, name := range []string{"idx_audit_log_tenant_created", "idx_audit_log_tenant_type_created"} {
		if !c.Pool.Migrator().HasIndex(models.TableAuditLog, name) {
			t.Fatalf("缺少索引 %s", name)
		}
	}
}
