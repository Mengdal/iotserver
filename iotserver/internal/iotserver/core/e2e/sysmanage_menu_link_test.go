package e2e

import (
	"testing"

	"iotserver/internal/dtos"
	"iotserver/internal/models"
)

// 链接菜单：isFrame 由链接地址推导（内链 1 / 外链 0），并保持叶子节点约束。
func TestMenuLinkTypeInternalAndExternal(t *testing.T) {
	env := setupTestEnv(t)

	extID, ok, msg := mtAddMenu(t, env, env.token, dtos.SysMenuAddRequest{
		ParentId: "0", Name: "外链菜单", Path: "https://example.com/docs",
		MenuType: models.MenuTypeLink, Status: 1, Visible: 1, IsFrame: 1, // 客户端故意传错，服务端应推导为外链
	})
	if !ok {
		t.Fatalf("新增外链菜单失败: %s", msg)
	}
	ext := mtMenuRow(t, env, extID)
	if ext.MenuType != models.MenuTypeLink || ext.IsFrame != 0 || ext.Path != "https://example.com/docs" {
		t.Fatalf("外链菜单字段错误: %+v", ext)
	}

	intID, ok, msg := mtAddMenu(t, env, env.token, dtos.SysMenuAddRequest{
		ParentId: "0", Name: "内链菜单", Path: "/home/dashboard",
		MenuType: models.MenuTypeLink, Status: 1, Visible: 1, IsFrame: 0, // 客户端故意传错，服务端应推导为内链
	})
	if !ok {
		t.Fatalf("新增内链菜单失败: %s", msg)
	}
	in := mtMenuRow(t, env, intID)
	if in.MenuType != models.MenuTypeLink || in.IsFrame != 1 || in.Path != "/home/dashboard" {
		t.Fatalf("内链菜单字段错误: %+v", in)
	}

	// 链接必须是合法地址
	if _, ok, _ := mtAddMenu(t, env, env.token, dtos.SysMenuAddRequest{
		ParentId: "0", Name: "非法链接", Path: "example.com", MenuType: models.MenuTypeLink,
		Status: 1, Visible: 1,
	}); ok {
		t.Fatal("非法链接地址应被拒绝")
	}

	// 链接是叶子，不能再挂子级
	if _, ok, _ := mtAddMenu(t, env, env.token, dtos.SysMenuAddRequest{
		ParentId: extID, Name: "链接下的子菜单", Path: "/x", MenuType: models.MenuTypeMenu,
		Status: 1, Visible: 1,
	}); ok {
		t.Fatal("链接菜单不能作为父级")
	}
}
