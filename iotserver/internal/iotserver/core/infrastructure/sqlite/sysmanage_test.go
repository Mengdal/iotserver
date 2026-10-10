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

	menus, err := c.SysMenuAll()
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
	menusOfUser, err := c.RoleMenusByUserId(admin.Id)
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
	allMenus, err := c.SysMenuAll()
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
	userMenus, err := c.RoleMenusByUserId(user.Id)
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

func TestSysLoginLogAndConfig(t *testing.T) {
	c := newTestClient(t)
	if err := c.MigrateSystemTables(); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	if err := c.AddSysLoginLog(models.LoginLog{Username: "admin", Ip: "127.0.0.1", Status: 1, Message: "登录成功", LoginTime: 1000}); err != nil {
		t.Fatalf("add log: %v", err)
	}
	logs, total, err := c.SysLoginLogSearch(0, -1, dtos.SysLoginLogSearchRequest{Username: "admin", IsAll: true})
	if err != nil {
		t.Fatalf("log search: %v", err)
	}
	if total != 1 || len(logs) != 1 {
		t.Fatalf("expect 1 log, got %d", total)
	}
	if err := c.ClearSysLoginLog(); err != nil {
		t.Fatalf("clear log: %v", err)
	}
	if _, total, _ = c.SysLoginLogSearch(0, -1, dtos.SysLoginLogSearchRequest{IsAll: true}); total != 0 {
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
