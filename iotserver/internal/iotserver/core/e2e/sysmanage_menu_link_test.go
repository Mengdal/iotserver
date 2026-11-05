package e2e

import (
	"testing"

	"iotserver/internal/dtos"
	"iotserver/internal/models"
)

// 链接菜单：内链（IsFrame=1）可在当前页嵌入，地址可以是站内路径或外部 URL；
// 外链（IsFrame=0）只允许外部 URL；两者都保持叶子节点约束。
func TestMenuLinkTypeInternalAndExternal(t *testing.T) {
	env := setupTestEnv(t)

	// 外部 URL 作为内链：允许，保留 IsFrame=1（由前端 iframe 嵌入）
	embedID, ok, msg := mtAddMenu(t, env, env.token, dtos.SysMenuAddRequest{
		ParentId: "0", Name: "内链外部地址", Path: "https://github.com/unclebob/ub-mode",
		MenuType: models.MenuTypeLink, Status: 1, Visible: 1, IsFrame: 1,
	})
	if !ok {
		t.Fatalf("新增内链（外部地址）失败: %s", msg)
	}
	embed := mtMenuRow(t, env, embedID)
	if embed.MenuType != models.MenuTypeLink || embed.IsFrame != 1 ||
		embed.Path != "https://github.com/unclebob/ub-mode" {
		t.Fatalf("内链菜单字段错误: %+v", embed)
	}

	// 站内路径作为内链同样允许
	intID, ok, msg := mtAddMenu(t, env, env.token, dtos.SysMenuAddRequest{
		ParentId: "0", Name: "内链站内路径", Path: "/home/dashboard",
		MenuType: models.MenuTypeLink, Status: 1, Visible: 1, IsFrame: 1,
	})
	if !ok {
		t.Fatalf("新增内链（站内路径）失败: %s", msg)
	}
	if in := mtMenuRow(t, env, intID); in.IsFrame != 1 {
		t.Fatalf("内链菜单字段错误: %+v", in)
	}

	// 外链必须是外部 URL
	extID, ok, msg := mtAddMenu(t, env, env.token, dtos.SysMenuAddRequest{
		ParentId: "0", Name: "外链菜单", Path: "https://example.com/docs",
		MenuType: models.MenuTypeLink, Status: 1, Visible: 1, IsFrame: 0,
	})
	if !ok {
		t.Fatalf("新增外链菜单失败: %s", msg)
	}
	ext := mtMenuRow(t, env, extID)
	if ext.MenuType != models.MenuTypeLink || ext.IsFrame != 0 || ext.Path != "https://example.com/docs" {
		t.Fatalf("外链菜单字段错误: %+v", ext)
	}

	// 外链不允许站内路径；内链不允许相对路径
	if _, ok, _ := mtAddMenu(t, env, env.token, dtos.SysMenuAddRequest{
		ParentId: "0", Name: "外链站内路径", Path: "/home/dashboard", MenuType: models.MenuTypeLink,
		Status: 1, Visible: 1, IsFrame: 0,
	}); ok {
		t.Fatal("外链使用站内路径应被拒绝")
	}
	if _, ok, _ := mtAddMenu(t, env, env.token, dtos.SysMenuAddRequest{
		ParentId: "0", Name: "非法内链", Path: "example.com", MenuType: models.MenuTypeLink,
		Status: 1, Visible: 1, IsFrame: 1,
	}); ok {
		t.Fatal("相对路径应被拒绝")
	}

	// 链接是叶子，不能再挂子级
	if _, ok, _ := mtAddMenu(t, env, env.token, dtos.SysMenuAddRequest{
		ParentId: embedID, Name: "链接下的子菜单", Path: "/x", MenuType: models.MenuTypeMenu,
		Status: 1, Visible: 1,
	}); ok {
		t.Fatal("链接菜单不能作为父级")
	}
}
