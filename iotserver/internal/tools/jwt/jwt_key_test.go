package jwt

import "testing"

func TestDeriveRefreshKey(t *testing.T) {
	if got := DeriveRefreshKey("  "); got != "" {
		t.Fatalf("空 base 应返回空串，实际 %q", got)
	}
	a := DeriveRefreshKey("install-secret")
	b := DeriveRefreshKey("install-secret")
	if a == "" || a != b {
		t.Fatalf("派生结果应稳定且非空: %q / %q", a, b)
	}
	if a == "install-secret" {
		t.Fatal("派生密钥不应等于原始密钥")
	}
	if DeriveRefreshKey("other-secret") == a {
		t.Fatal("不同 base 应派生出不同密钥")
	}
}
