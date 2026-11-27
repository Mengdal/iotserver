package dtos

import (
	"strings"
	"testing"

	"iotserver/internal/pkg/constants"
)

// 属性报文在消息总线上是 data.<code>=value + 消息级 time；
// 告警 SQL 不能再用旧的 data.<code>.value / data.<code>.time。
func TestBuildEkuiperSqlUsesFlatPropertyData(t *testing.T) {
	cases := []struct {
		name      string
		specsType constants.SpecsType
		option    map[string]string
		want      []string
	}{
		{
			name:      "数值原始值",
			specsType: constants.SpecsTypeFloat,
			option: map[string]string{
				"code": "temp", "value_type": constants.Original, "decide_condition": "> 30",
			},
			want: []string{
				"SELECT rule_id(),time,deviceId",
				`json_path_query(data, "$.temp") > 30`,
				`json_path_exists(data, "$.temp") = true`,
			},
		},
		{
			name:      "数值周期平均",
			specsType: constants.SpecsTypeFloat,
			option: map[string]string{
				"code": "temp", "value_type": constants.Avg, "value_cycle": "1分钟周期",
				"decide_condition": "> 30",
			},
			want: []string{
				`avg(json_path_query(data, "$.temp")) as avg_temp`,
				"TUMBLINGWINDOW(ss, 60)",
				"HAVING avg_temp > 30",
			},
		},
		{
			name:      "文本",
			specsType: constants.SpecsTypeText,
			option: map[string]string{
				"code": "state", "decide_condition": "= running",
			},
			want: []string{
				"SELECT rule_id(),time,deviceId",
				`json_path_query(data, "$.state") = "running"`,
			},
		},
		{
			name:      "枚举",
			specsType: constants.SpecsTypeEnum,
			option: map[string]string{
				"code": "mode", "decide_condition": "= 2",
			},
			want: []string{
				`json_path_query(data, "$.mode") = 2`,
			},
		},
		{
			name:      "布尔",
			specsType: constants.SpecsTypeBool,
			option: map[string]string{
				"code": "running", "decide_condition": "= true",
			},
			want: []string{
				`json_path_query(data, "$.running") = true`,
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := RuleUpdateRequest{SubRule: []SubRule{{Option: tc.option}}}
			sql := req.BuildEkuiperSql("device-1", tc.specsType)
			if sql == "" {
				t.Fatal("BuildEkuiperSql returned empty")
			}
			if strings.Contains(sql, ".value") || strings.Contains(sql, ".time") {
				t.Fatalf("SQL 仍引用旧嵌套结构: %s", sql)
			}
			for _, want := range tc.want {
				if !strings.Contains(sql, want) {
					t.Fatalf("SQL 缺少 %q: %s", want, sql)
				}
			}
		})
	}
}

func TestBuildEkuiperSqlRejectsInvalidBoolCondition(t *testing.T) {
	req := RuleUpdateRequest{SubRule: []SubRule{{Option: map[string]string{
		"code": "running", "decide_condition": "= 1",
	}}}}
	if sql := req.BuildEkuiperSql("device-1", constants.SpecsTypeBool); sql != "" {
		t.Fatalf("非法布尔条件不应生成 SQL: %s", sql)
	}
}
