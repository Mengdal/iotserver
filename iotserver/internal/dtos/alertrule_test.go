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

// 当前消息总线的 PROPERTY_REPORT 是扁平结构：data.<code> 直接是值。
// 告警 SQL 不能再按旧的 data.<code>.value / .time 生成。
func TestBuildEkuiperSqlUsesFlatPropertyValue(t *testing.T) {
	req := buildSubRule("discharge_pressure", "> 100", constants.Original)
	sql := req.BuildEkuiperSql("29365864", constants.SpecsTypeFloat)
	if sql == "" {
		t.Fatal("empty sql")
	}
	if !strings.Contains(sql, `json_path_query(data, "$.discharge_pressure") > 100`) {
		t.Fatalf("sql does not read flat property value: %s", sql)
	}
	if !strings.Contains(sql, "reportTime as report_time") {
		t.Fatalf("sql does not expose report_time: %s", sql)
	}
	if strings.Contains(sql, ".value") || strings.Contains(sql, ".time") {
		t.Fatalf("sql still contains legacy .value/.time path: %s", sql)
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
