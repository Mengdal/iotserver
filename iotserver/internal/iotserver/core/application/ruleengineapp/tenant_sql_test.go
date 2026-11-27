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
		{
			name:     "group by multiple dimensions",
			sql:      `SELECT deviceId, avg(temperature) FROM mqtt_stream GROUP BY TUMBLINGWINDOW(ss, 10), deviceId`,
			tenantId: "t1",
			want:     []string{`FROM mqtt_stream WHERE tenantId = 't1' GROUP BY TUMBLINGWINDOW(ss, 10), deviceId`},
		},
		{
			name:     "order by multiple fields",
			sql:      `SELECT * FROM mqtt_stream ORDER BY temperature DESC, deviceId ASC`,
			tenantId: "t1",
			want:     []string{`FROM mqtt_stream WHERE tenantId = 't1' ORDER BY temperature DESC, deviceId ASC`},
		},
		{
			name:     "reject multiple sources",
			sql:      `SELECT * FROM mqtt_stream, other_stream`,
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

// 旧版本把编译后的 SQL 存进了 filter.sql，再次注入时必须先剥掉，不能叠加。
func TestEnforceTenantFilterIsIdempotent(t *testing.T) {
	raw := `SELECT deviceId FROM mqtt_stream WHERE temperature > 30`
	once, err := enforceTenantFilter(raw, "t1")
	if err != nil {
		t.Fatalf("first enforce: %v", err)
	}
	twice, err := enforceTenantFilter(once, "t1")
	if err != nil {
		t.Fatalf("second enforce: %v", err)
	}
	if twice != once {
		t.Fatalf("重复注入改变了 SQL:\nfirst  = %q\nsecond = %q", once, twice)
	}
	if strings.Count(twice, "tenantId = 't1'") != 1 {
		t.Fatalf("租户条件应只有一个: %q", twice)
	}

	// 旧规则没有 WHERE，注入后也没有括号可解
	noWhere := `SELECT deviceId FROM mqtt_stream`
	first, err := enforceTenantFilter(noWhere, "t1")
	if err != nil {
		t.Fatalf("no-where first: %v", err)
	}
	second, err := enforceTenantFilter(first, "t1")
	if err != nil {
		t.Fatalf("no-where second: %v", err)
	}
	if second != first {
		t.Fatalf("无 WHERE 重复注入: first=%q second=%q", first, second)
	}

	// 跨租户导入：旧租户条件也应被替换，而不是 AND 叠加
	cross, err := enforceTenantFilter(`SELECT * FROM mqtt_stream WHERE tenantId = 'old' AND (a = 1)`, "new")
	if err != nil {
		t.Fatalf("cross tenant: %v", err)
	}
	if strings.Contains(cross, "old") {
		t.Fatalf("旧租户条件未被移除: %q", cross)
	}
	if !strings.Contains(cross, "tenantId = 'new' AND (a = 1)") {
		t.Fatalf("跨租户编译结果异常: %q", cross)
	}
}

func TestStripTenantPredicates(t *testing.T) {
	cases := []struct {
		sql  string
		want string
	}{
		{
			sql:  `SELECT * FROM mqtt_stream WHERE tenantId = 't1' AND (a = 1 OR b = 2)`,
			want: `SELECT * FROM mqtt_stream WHERE a = 1 OR b = 2`,
		},
		{
			sql:  `SELECT * FROM mqtt_stream WHERE tenantId = 't1'`,
			want: `SELECT * FROM mqtt_stream`,
		},
		{
			sql:  `SELECT * FROM mqtt_stream WHERE tenantId = 't1' AND (tenantId = 't2' AND (a = 1))`,
			want: `SELECT * FROM mqtt_stream WHERE a = 1`,
		},
		{
			sql:  `SELECT deviceId FROM mqtt_stream WHERE tenantId = 't1' GROUP BY deviceId`,
			want: `SELECT deviceId FROM mqtt_stream GROUP BY deviceId`,
		},
		{
			// 用户自己的非顶层条件不能被误删
			sql:  `SELECT * FROM mqtt_stream WHERE data.tenantId = 'x'`,
			want: `SELECT * FROM mqtt_stream WHERE data.tenantId = 'x'`,
		},
	}
	for _, tc := range cases {
		got, _ := stripTenantPredicates(tc.sql)
		if got != tc.want {
			t.Fatalf("strip(%q) = %q, want %q", tc.sql, got, tc.want)
		}
	}
}
