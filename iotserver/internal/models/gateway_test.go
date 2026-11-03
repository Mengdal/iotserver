package models

import "testing"

func TestNormalizeGatewayExcludeKeys(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"", ""},
		{"  ", ""},
		{"system", "system"},
		{" system , user ", "system,user"},
		{",,,", ""},
		{"system,system,user", "system,user"},
		{"user, system ,user", "user,system"},
	}
	for _, c := range cases {
		if got := NormalizeGatewayExcludeKeys(c.in); got != c.want {
			t.Errorf("NormalizeGatewayExcludeKeys(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestMergeGatewayExcludeKeys(t *testing.T) {
	cases := []struct {
		current string
		key     string
		want    string
	}{
		{"", "Device1", "Device1"},
		{"", "", ""},
		{"system", "Device1", "system,Device1"},
		{"system,user", "Device1", "system,user,Device1"},
		// 已存在：不重复追加，同时把既有名单规范化
		{" system , user ", "user", "system,user"},
		{"system", " system ", "system"},
	}
	for _, c := range cases {
		if got := MergeGatewayExcludeKeys(c.current, c.key); got != c.want {
			t.Errorf("MergeGatewayExcludeKeys(%q, %q) = %q, want %q", c.current, c.key, got, c.want)
		}
	}
}
