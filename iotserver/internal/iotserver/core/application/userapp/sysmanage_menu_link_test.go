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

func TestNormalizeLinkMode(t *testing.T) {
	if got := normalizeLinkMode(models.MenuTypeLink, 1); got != 1 {
		t.Fatalf("链接菜单应允许嵌入模式，实际 %d", got)
	}
	if got := normalizeLinkMode(models.MenuTypeLink, 0); got != 0 {
		t.Fatalf("链接菜单默认跳转，实际 %d", got)
	}
	if got := normalizeLinkMode(models.MenuTypeMenu, 1); got != 0 {
		t.Fatalf("非链接菜单不允许嵌入，实际 %d", got)
	}
}

func TestNormalizeMenuTarget(t *testing.T) {
	cases := []struct {
		name     string
		menuType string
		path     string
		wantPath string
		wantLink int // 0 外链，1 内链
		wantErr  bool
	}{
		{"菜单保留内链并标记非外链", models.MenuTypeMenu, "/home/dashboard", "/home/dashboard", 1, false},
		{"普通菜单忽略外部 isFrame", models.MenuTypeMenu, "https://example.com", "https://example.com", 1, false},
		{"内链", models.MenuTypeLink, " /home/dashboard ", "/home/dashboard", 1, false},
		{"外链 http", models.MenuTypeLink, "http://example.com/a", "http://example.com/a", 0, false},
		{"外链 https 大写", models.MenuTypeLink, "HTTPS://example.com", "HTTPS://example.com", 0, false},
		{"空链接", models.MenuTypeLink, "  ", "", 0, true},
		{"非法协议", models.MenuTypeLink, "ftp://example.com", "", 0, true},
		{"缺少前导斜杠", models.MenuTypeLink, "home/dashboard", "", 0, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path, link, err := normalizeMenuTarget(tc.menuType, tc.path, 9)
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
