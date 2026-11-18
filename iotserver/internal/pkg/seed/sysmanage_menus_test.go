package seed

import (
	"strings"
	"testing"

	"iotserver/internal/models"
)

// 平台级菜单（租户管理 / 系统配置）下的按钮必须继承 platform_only：
// 否则租户管理员会通过按钮权限码拿到平台接口权限。
func TestPlatformOnlyButtonsInheritParent(t *testing.T) {
	menus := DefaultMenus()
	byId := make(map[string]models.Menu, len(menus))
	for _, m := range menus {
		byId[m.Id] = m
	}

	for _, m := range menus {
		if m.MenuType != models.MenuTypeButton {
			continue
		}
		parent, ok := byId[m.ParentId]
		if !ok {
			t.Fatalf("按钮 %s 的父菜单 %s 不存在", m.Id, m.ParentId)
		}
		if parent.PlatformOnly == 1 && m.PlatformOnly != 1 {
			t.Fatalf("平台级菜单 %s 下的按钮 %s(%s) 未标记 platform_only", parent.Name, m.Name, m.Perms)
		}
		if parent.PlatformOnly == 0 && m.PlatformOnly == 1 {
			t.Fatalf("普通菜单 %s 下的按钮 %s 不应标记 platform_only", parent.Name, m.Name)
		}
	}

	// 显式校验两条平台权限前缀，避免以后新增按钮组时漏标
	for _, m := range menus {
		if !strings.HasPrefix(m.Perms, "system:config:") && !strings.HasPrefix(m.Perms, "platform:tenant:") {
			continue
		}
		if m.PlatformOnly != 1 {
			t.Fatalf("平台权限 %s 必须是 platform_only 菜单", m.Perms)
		}
	}
}

// 平台管理目录挂在系统管理下，租户管理/系统配置仍作为它的子菜单。
// 移动父级不能改变 platform_only：否则租户侧会看到平台管理分组。
func TestPlatformMenuNestedUnderSystemManage(t *testing.T) {
	byId := make(map[string]models.Menu)
	for _, m := range DefaultMenus() {
		byId[m.Id] = m
	}
	platform, ok := byId[MenuPlatformId]
	if !ok {
		t.Fatalf("平台管理菜单 %s 不存在", MenuPlatformId)
	}
	if platform.ParentId != MenuAdminId {
		t.Fatalf("平台管理应挂在系统管理(%s)下，实际父级 %s", MenuAdminId, platform.ParentId)
	}
	if platform.PlatformOnly != 1 {
		t.Fatal("平台管理目录必须保持 platform_only")
	}
	for _, child := range []string{MenuPlatformTenant, MenuAdminConfig} {
		if byId[child].ParentId != MenuPlatformId {
			t.Fatalf("菜单 %s(%s) 的父级应为平台管理，实际 %s", byId[child].Name, child, byId[child].ParentId)
		}
		if byId[child].PlatformOnly != 1 {
			t.Fatalf("菜单 %s(%s) 必须保持 platform_only", byId[child].Name, child)
		}
	}
}

// 公共导航菜单（首页 / 工具与文档）不带权限码，所有登录用户默认可见。
func TestPublicMenusHaveNoPerms(t *testing.T) {
	menus := DefaultMenus()
	byId := make(map[string]models.Menu, len(menus))
	for _, m := range menus {
		byId[m.Id] = m
	}
	for _, id := range []string{MenuHomeId, MenuDocsId} {
		menu, ok := byId[id]
		if !ok {
			t.Fatalf("公共菜单 %s 不存在", id)
		}
		if !IsPublicMenu(id) {
			t.Fatalf("菜单 %s 应被识别为公共菜单", id)
		}
		if menu.Perms != "" {
			t.Fatalf("公共菜单 %s 不应带权限码，实际 %s", menu.Name, menu.Perms)
		}
	}
	if IsPublicMenu(MenuProduct) {
		t.Fatal("普通业务菜单不应被识别为公共菜单")
	}
}

// 已下线的按钮权限（切换租户 / collect:config:*）不再生成菜单。
func TestRetiredButtonPermsNotSeeded(t *testing.T) {
	for _, m := range DefaultMenus() {
		if _, retired := retiredButtonPerms[m.Perms]; retired {
			t.Fatalf("已下线的权限 %s 仍在生成菜单 %s", m.Perms, m.Name)
		}
	}
}

// 租户管理员模板不应把平台级菜单（含按钮）或公共导航菜单写进授权列表。
func TestTenantAdminMenuIdsExcludePlatformOnly(t *testing.T) {
	menus := DefaultMenus()
	byId := make(map[string]models.Menu, len(menus))
	for _, m := range menus {
		byId[m.Id] = m
	}
	for _, id := range TenantAdminMenuIds(menus) {
		menu, ok := byId[id]
		if !ok {
			t.Fatalf("授权列表里的菜单 %s 不存在", id)
		}
		if menu.PlatformOnly == 1 {
			t.Fatalf("租户管理员角色不应包含平台级菜单 %s(%s)", menu.Name, menu.Perms)
		}
		if IsPublicMenu(id) {
			t.Fatalf("租户管理员角色不应包含公共导航菜单 %s", menu.Name)
		}
	}
}

// 高级能力（资源管理 / 规则引擎 / 场景联动）允许普通用户操作：
// 内置「普通用户」角色默认带上菜单和全部 advanced:* 按钮权限。
func TestTenantUserMenuIdsIncludeAdvanced(t *testing.T) {
	owned := make(map[string]struct{})
	for _, id := range TenantUserMenuIds() {
		owned[id] = struct{}{}
	}
	if _, ok := owned[MenuAdvancedId]; !ok {
		t.Fatalf("普通用户角色缺少高级能力目录 %s", MenuAdvancedId)
	}
	for _, m := range DefaultMenus() {
		if !strings.HasPrefix(m.Perms, "advanced:") {
			continue
		}
		if _, ok := owned[m.Id]; !ok {
			t.Fatalf("普通用户角色缺少高级能力权限 %s", m.Perms)
		}
	}
	// 基础只读权限不能被新增的高级能力挤掉
	for _, m := range DefaultMenus() {
		if m.Perms != "gateway:device:query" {
			continue
		}
		if _, ok := owned[m.Id]; !ok {
			t.Fatalf("普通用户角色缺少基础只读权限 %s", m.Perms)
		}
	}
}
