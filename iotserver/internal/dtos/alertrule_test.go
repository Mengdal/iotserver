package dtos

import (
	"strings"
	"testing"

	"iotserver/internal/models"
	"iotserver/internal/pkg/constants"
)

func buildSubRule(code, condition, valueType string) RuleUpdateRequest {
	return RuleUpdateRequest{
		SubRule: []SubRule{{
			Option: map[string]string{
				"code":             code,
				"decide_condition": condition,
				"value_type":       valueType,
			},
		}},
	}
}

// 当前消息总线的 PROPERTY_REPORT 是扁平结构：data.<code> 直接是值，
// 上报时间在消息级 time。告警 SQL 不能再按旧的 data.<code>.value / .time 生成。
func TestBuildEkuiperSqlUsesFlatPropertyValue(t *testing.T) {
	req := buildSubRule("discharge_pressure", "> 100", constants.Original)
	sql := req.BuildEkuiperSql("29365864", constants.SpecsTypeFloat)
	if sql == "" {
		t.Fatal("empty sql")
	}
	if !strings.Contains(sql, `json_path_query(data, "$.discharge_pressure") > 100`) {
		t.Fatalf("sql does not read flat property value: %s", sql)
	}
	if !strings.Contains(sql, "SELECT rule_id(),time,deviceId") {
		t.Fatalf("sql does not expose message time: %s", sql)
	}
	if strings.Contains(sql, ".value") || strings.Contains(sql, ".time") {
		t.Fatalf("sql still contains legacy .value/.time path: %s", sql)
	}
}

// 覆盖数值/聚合/文本/枚举/布尔各类型，确保 SQL 只引用扁平字段。
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

func TestBuildEkuiperSqlAggregateUsesFlatPropertyValue(t *testing.T) {
	req := buildSubRule("discharge_pressure", "> 100", constants.Avg)
	req.SubRule[0].Option["value_cycle"] = "1分钟周期"
	sql := req.BuildEkuiperSql("29365864", constants.SpecsTypeFloat)
	if !strings.Contains(sql, `avg(json_path_query(data, "$.discharge_pressure"))`) {
		t.Fatalf("aggregate does not read flat property value: %s", sql)
	}
	if strings.Contains(sql, ".value") || strings.Contains(sql, ".time") {
		t.Fatalf("aggregate still contains legacy path: %s", sql)
	}
	if !strings.Contains(sql, "TUMBLINGWINDOW(ss, 60)") {
		t.Fatalf("aggregate window missing: %s", sql)
	}
}

func TestBuildEkuiperSqlTextUsesFlatPropertyValue(t *testing.T) {
	req := buildSubRule("mode", "= auto", "")
	sql := req.BuildEkuiperSql("29365864", constants.SpecsTypeText)
	if !strings.Contains(sql, `json_path_query(data, "$.mode") = "auto"`) {
		t.Fatalf("text sql does not read flat property value: %s", sql)
	}
	if strings.Contains(sql, ".value") || strings.Contains(sql, ".time") {
		t.Fatalf("text sql still contains legacy path: %s", sql)
	}
}

func TestBuildEkuiperSqlBoolUsesJSONBoolean(t *testing.T) {
	for _, condition := range []string{"= true", "= false"} {
		req := buildSubRule("power", condition, "")
		sql := req.BuildEkuiperSql("29365864", constants.SpecsTypeBool)
		expected := strings.TrimPrefix(condition, "= ")
		if !strings.Contains(sql, `json_path_query(data, "$.power") = `+expected) {
			t.Fatalf("bool sql does not use json boolean: %s", sql)
		}
		if strings.Contains(sql, ".value") || strings.Contains(sql, ".time") {
			t.Fatalf("bool sql still contains legacy path: %s", sql)
		}
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

// 静默时间用指针区分“未传”和“显式 0”：前者保持原值，后者允许改回“无静默”。
func TestReplaceRuleModelFieldsSilenceTime(t *testing.T) {
	ds := models.AlertRule{SilenceTime: 60000}

	ReplaceRuleModelFields(&ds, RuleUpdateRequest{})
	if ds.SilenceTime != 60000 {
		t.Fatalf("未传静默时间时被改为 %d", ds.SilenceTime)
	}

	zero := int64(0)
	ReplaceRuleModelFields(&ds, RuleUpdateRequest{SilenceTime: &zero})
	if ds.SilenceTime != 0 {
		t.Fatalf("显式传 0 未清空静默时间，仍为 %d", ds.SilenceTime)
	}
}
