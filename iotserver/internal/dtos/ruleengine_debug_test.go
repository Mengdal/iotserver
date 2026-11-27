package dtos

import (
	"encoding/json"
	"testing"
)

// 模拟数据源既接受对象数组，也接受单条 JSON 对象。
func TestRuleEngineDebugDataUnmarshal(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want int
	}{
		{"单条对象", `{"deviceId":"dev-1"}`, 1},
		{"对象数组", `[{"deviceId":"dev-1"},{"deviceId":"dev-2"}]`, 2},
		{"空数组", `[]`, 0},
		{"null", `null`, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var req RuleEngineDebugRequest
			if err := json.Unmarshal([]byte(`{"sql":"SELECT * FROM mqtt_stream","data":`+tc.raw+`}`), &req); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if len(req.Data) != tc.want {
				t.Fatalf("data len = %d, want %d", len(req.Data), tc.want)
			}
		})
	}

	// 非法类型（数字、字符串）必须拒绝
	for _, raw := range []string{`123`, `"abc"`, `true`} {
		var req RuleEngineDebugRequest
		if err := json.Unmarshal([]byte(`{"data":`+raw+`}`), &req); err == nil {
			t.Fatalf("%s 应被拒绝", raw)
		}
	}
}
