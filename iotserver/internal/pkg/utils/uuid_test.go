package utils

import "testing"

func TestRandomNum10(t *testing.T) {
	for i := 0; i < 200; i++ {
		sn := RandomNum10()
		if len(sn) != 10 {
			t.Fatalf("RandomNum10 长度 = %d, want 10: %q", len(sn), sn)
		}
		if sn[0] == '0' {
			t.Fatalf("RandomNum10 首位不能为 0: %q", sn)
		}
		for _, r := range sn {
			if r < '0' || r > '9' {
				t.Fatalf("RandomNum10 含非数字字符: %q", sn)
			}
		}
	}
}
