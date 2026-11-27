package ekuiperclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"iotserver/internal/dtos"
)

// 更新规则定义不应改变运行状态：eKuiper 的 PUT 会按 triggered 决定更新后是否继续运行。
func TestUpdateRuleDefinitionPreservesRunningState(t *testing.T) {
	cases := []struct {
		name   string
		status string
		want   bool
	}{
		{name: "running", status: "running", want: true},
		{name: "starting", status: "starting", want: true},
		{name: "stopped", status: "stopped", want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var got dtos.CreateRule
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch {
				case r.Method == http.MethodGet && r.URL.Path == "/rules/r1/status":
					w.Header().Set("Content-Type", "application/json")
					_ = json.NewEncoder(w).Encode(map[string]string{"status": tc.status})
				case r.Method == http.MethodPut && r.URL.Path == "/rules/r1":
					if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
						t.Errorf("decode update body: %v", err)
					}
					_, _ = w.Write([]byte("Rule r1 was updated successfully."))
				default:
					w.WriteHeader(http.StatusNotFound)
				}
			}))
			defer srv.Close()

			client := New(srv.URL, nil)
			if err := client.UpdateRule(context.Background(), nil, "r1", "SELECT * FROM mqtt_stream"); err != nil {
				t.Fatalf("UpdateRule: %v", err)
			}
			if got.Triggered != tc.want {
				t.Fatalf("triggered = %v, want %v", got.Triggered, tc.want)
			}
		})
	}
}
