package userapp

import (
	"testing"

	"iotserver/internal/models"
)

func TestNormalizeMenuTypeLink(t *testing.T) {
	if got := normalizeMenuType(models.MenuTypeLink); got != models.MenuTypeLink {
		t.Fatalf("链接类型应保留，实际 %q", got)
	}
	if got := normalizeMenuType("unknown"); got != models.MenuTypeMenu {
		t.Fatalf("未知类型应回落为菜单，实际 %q", got)
	}
}

func TestNormalizeMenuTarget(t *testing.T) {
	cases := []struct {
		name     string
		menuType string
		path     string
		isFrame  int // 1 内链（嵌入），0 外链（跳转）
		wantPath string
		wantLink int
		wantErr  bool
	}{
		{"菜单保留内链并标记非外链", models.MenuTypeMenu, "/home/dashboard", 0, "/home/dashboard", 1, false},
		{"普通菜单忽略外部 isFrame", models.MenuTypeMenu, "https://example.com", 0, "https://example.com", 1, false},
		{"内链站内路径", models.MenuTypeLink, " /home/dashboard ", 1, "/home/dashboard", 1, false},
		{"内链外部地址也允许嵌入", models.MenuTypeLink, "https://github.com/unclebob/ub-mode", 1, "https://github.com/unclebob/ub-mode", 1, false},
		{"外链 http", models.MenuTypeLink, "http://example.com/a", 0, "http://example.com/a", 0, false},
		{"外链 https 大写", models.MenuTypeLink, "HTTPS://example.com", 0, "HTTPS://example.com", 0, false},
		{"外链不允许站内路径", models.MenuTypeLink, "/home/dashboard", 0, "", 0, true},
		{"空链接", models.MenuTypeLink, "  ", 1, "", 0, true},
		{"非法协议", models.MenuTypeLink, "ftp://example.com", 1, "", 0, true},
		{"内链缺少前导斜杠", models.MenuTypeLink, "home/dashboard", 1, "", 0, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path, link, err := normalizeMenuTarget(tc.menuType, tc.path, tc.isFrame)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("应返回错误，实际 path=%q isFrame=%d", path, link)
				}
				return
			}
			if err != nil {
				t.Fatalf("不应返回错误: %v", err)
			}
			if path != tc.wantPath || link != tc.wantLink {
				t.Fatalf("path=%q isFrame=%d, want path=%q isFrame=%d", path, link, tc.wantPath, tc.wantLink)
			}
		})
	}
}
