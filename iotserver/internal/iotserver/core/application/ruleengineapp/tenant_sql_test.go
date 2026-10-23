package ruleengineapp

import (
	"strings"
	"testing"
)

func TestEnforceTenantFilter(t *testing.T) {
	cases := []struct {
		name     string
		sql      string
		tenantId string
		want     []string
		wantErr  bool
	}{
		{
			name:     "no where",
			sql:      `SELECT deviceId FROM mqtt_stream`,
			tenantId: "t1",
			want:     []string{`WHERE tenantId = 't1'`},
		},
		{
			name:     "with where",
			sql:      `SELECT deviceId FROM mqtt_stream where deviceId = "dev-1"`,
			tenantId: "t1",
			want:     []string{`where tenantId = 't1' AND (deviceId = "dev-1")`},
		},
		{
			name:     "or condition is grouped",
			sql:      `SELECT * FROM mqtt_stream WHERE a = 1 OR b = 2`,
			tenantId: "t1",
			want:     []string{`WHERE tenantId = 't1' AND (a = 1 OR b = 2)`},
		},
		{
			name:     "group by before having",
			sql:      `SELECT deviceId FROM mqtt_stream WHERE x = 1 GROUP BY deviceId HAVING count(*) > 1`,
			tenantId: "t1",
			want:     []string{`WHERE tenantId = 't1' AND (x = 1) GROUP BY deviceId HAVING`},
		},
		{
			name:     "no where with group by",
			sql:      `SELECT deviceId FROM mqtt_stream GROUP BY deviceId`,
			tenantId: "t1",
			want:     []string{`FROM mqtt_stream WHERE tenantId = 't1' GROUP BY`},
		},
		{
			name:     "platform scope leaves sql untouched",
			sql:      `SELECT * FROM mqtt_stream`,
			tenantId: "",
			want:     []string{`SELECT * FROM mqtt_stream`},
		},
		{
			name:     "reject other stream",
			sql:      `SELECT * FROM other_stream`,
			tenantId: "t1",
			wantErr:  true,
		},
		{
			name:     "reject lookalike stream",
			sql:      `SELECT * FROM mqtt_stream_other`,
			tenantId: "t1",
			wantErr:  true,
		},
		{
			name:     "reject join",
			sql:      `SELECT * FROM mqtt_stream JOIN other_stream ON 1=1`,
			tenantId: "t1",
			wantErr:  true,
		},
		{
			name:     "reject union",
			sql:      `SELECT * FROM mqtt_stream UNION SELECT * FROM mqtt_stream`,
			tenantId: "t1",
			wantErr:  true,
		},
		{
			name:     "reject comment",
			sql:      `SELECT * FROM mqtt_stream -- bypass`,
			tenantId: "t1",
			wantErr:  true,
		},
		{
			name:     "reject multiple statements",
			sql:      `SELECT * FROM mqtt_stream; SELECT * FROM mqtt_stream`,
			tenantId: "t1",
			wantErr:  true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := enforceTenantFilter(tc.sql, tc.tenantId)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			for _, want := range tc.want {
				if !strings.Contains(got, want) {
					t.Fatalf("sql = %q, want contains %q", got, want)
				}
			}
		})
	}
}
