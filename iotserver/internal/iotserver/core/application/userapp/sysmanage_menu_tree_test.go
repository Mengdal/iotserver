package userapp

import (
	"testing"

	"iotserver/internal/models"
)

func TestReRootOrphanTenantMenus(t *testing.T) {
	menus := []models.Menu{
		// 平台菜单：父节点被 platform_only 过滤后也不应被提升，租户不该看到它
		{Id: "p-child", ParentId: "p-hidden", TenantId: ""},
		// 租户自建菜单：父节点已被过滤 → 提升到根
		{Id: "t-orphan", ParentId: "p-hidden", TenantId: "t1"},
		// 租户自建菜单：父节点仍在列表 → 保持原层级
		{Id: "t-child", ParentId: "t-root", TenantId: "t1"},
		{Id: "t-root", ParentId: "0", TenantId: "t1"},
	}

	got := reRootOrphanTenantMenus(menus, "t1")
	byId := make(map[string]models.Menu, len(got))
	for _, item := range got {
		byId[item.Id] = item
	}
	if byId["t-orphan"].ParentId != "0" {
		t.Fatalf("父节点缺失的租户菜单应提升到根，实际 parentId=%q", byId["t-orphan"].ParentId)
	}
	if byId["p-child"].ParentId != "p-hidden" {
		t.Fatalf("平台菜单不应被提升，实际 parentId=%q", byId["p-child"].ParentId)
	}
	if byId["t-child"].ParentId != "t-root" {
		t.Fatalf("父节点存在的租户菜单应保持层级，实际 parentId=%q", byId["t-child"].ParentId)
	}
}

func TestReRootOrphanTenantMenusPlatformScope(t *testing.T) {
	menus := []models.Menu{{Id: "t-orphan", ParentId: "missing", TenantId: "t1"}}
	got := reRootOrphanTenantMenus(menus, "")
	if got[0].ParentId != "missing" {
		t.Fatalf("平台视图不应改写父节点，实际 parentId=%q", got[0].ParentId)
	}
}
