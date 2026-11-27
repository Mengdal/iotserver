package ruleengineapp

import (
	"strings"
	"testing"

	"iotserver/internal/dtos"
	"iotserver/internal/models"
)

func TestNormalizeTags(t *testing.T) {
	tags, err := normalizeTags([]string{" 温控 ", "告警", "温控", ""})
	if err != nil {
		t.Fatalf("normalizeTags: %v", err)
	}
	if len(tags) != 2 || tags[0] != "温控" || tags[1] != "告警" {
		t.Fatalf("tags = %#v", tags)
	}
	if _, err = normalizeTags([]string{strings.Repeat("x", maxRuleTagLen+1)}); err == nil {
		t.Fatal("超长标签应被拒绝")
	}
	tooMany := make([]string, maxRuleTags+1)
	for i := range tooMany {
		tooMany[i] = string(rune('a' + i%26))
		tooMany[i] = tooMany[i] + "-" + strings.Repeat("x", i%10)
	}
	if _, err = normalizeTags(tooMany); err == nil {
		t.Fatal("超量标签应被拒绝")
	}
}

func TestValidateOptions(t *testing.T) {
	if err := validateOptions(models.RuleEngineOptions{Qos: 3}); err == nil {
		t.Fatal("QoS=3 应被拒绝")
	}
	if err := validateOptions(models.RuleEngineOptions{Concurrency: 2, BufferLength: 1024, Qos: 1}); err != nil {
		t.Fatalf("合法选项被拒绝: %v", err)
	}
}

// 新规则同时保存用户原始 SQL 和编译后 SQL；下发/启动使用 compiled_sql。
func TestBuildRuleFilterKeepsRawSql(t *testing.T) {
	filter, compiled, err := buildRuleFilter(models.RuleEngineFilter{
		Sql: `SELECT deviceId FROM mqtt_stream WHERE temperature > 30`,
	}, "tenant-01")
	if err != nil {
		t.Fatalf("buildRuleFilter: %v", err)
	}
	if filter.Sql != `SELECT deviceId FROM mqtt_stream WHERE temperature > 30` {
		t.Fatalf("raw sql = %q", filter.Sql)
	}
	if !strings.Contains(compiled, "tenantId = 'tenant-01'") {
		t.Fatalf("compiled sql = %q", compiled)
	}
	if filter.CompiledSql != compiled {
		t.Fatalf("filter compiled sql = %q", filter.CompiledSql)
	}
}

// 旧数据：filter.sql 是编译后的 SQL，compiled_sql 为空。
func TestNormalizeStoredFilterSplitsLegacySQL(t *testing.T) {
	legacy := models.RuleEngineFilter{
		Sql: `SELECT deviceId FROM mqtt_stream WHERE tenantId = 't1' AND (temperature > 30)`,
	}
	filter, compiled, err := normalizeStoredFilter(legacy, "t1")
	if err != nil {
		t.Fatalf("normalizeStoredFilter: %v", err)
	}
	if filter.Sql != `SELECT deviceId FROM mqtt_stream WHERE temperature > 30` {
		t.Fatalf("raw sql = %q", filter.Sql)
	}
	if strings.Count(compiled, "tenantId = 't1'") != 1 {
		t.Fatalf("compiled sql = %q", compiled)
	}

	// 新数据直接复用 compiled_sql
	fresh := models.RuleEngineFilter{
		Sql:         `SELECT * FROM mqtt_stream`,
		CompiledSql: `SELECT * FROM mqtt_stream WHERE tenantId = 't1'`,
	}
	gotFilter, gotCompiled, err := normalizeStoredFilter(fresh, "t1")
	if err != nil || gotFilter.Sql != fresh.Sql || gotCompiled != fresh.CompiledSql {
		t.Fatalf("fresh filter normalized unexpectedly: %#v %q %v", gotFilter, gotCompiled, err)
	}
}

func TestReservedRuleIDRejected(t *testing.T) {
	for _, id := range []string{"export", "debug", "DEBUG", "batch-start", "validate", "import"} {
		req := models.RuleEngineFilter{Sql: `SELECT * FROM mqtt_stream`}
		err := (&ruleEngineApp{}).validateRuleRequest(dtos.RuleEngineAddRequest{
			Id:     id,
			Name:   "rule-a",
			Filter: req,
		})
		if err == nil {
			t.Fatalf("保留字 %q 应被拒绝", id)
		}
	}
	// 普通 ID 仍可通过
	err := (&ruleEngineApp{}).validateRuleRequest(dtos.RuleEngineAddRequest{
		Id:     "rule_001",
		Name:   "rule-a",
		Filter: models.RuleEngineFilter{Sql: `SELECT * FROM mqtt_stream`},
	})
	if err != nil {
		t.Fatalf("普通 ID 被拒绝: %v", err)
	}
}

func TestResolveImportResource(t *testing.T) {
	byKey := map[string]string{
		importResourceKey("sink1", "HTTP推送"):    "http-1",
		importResourceKey("sink2", "消息队列Kafka"): "kafka-1",
	}
	byName := map[string][]string{
		"sink1": {"http-1"},
		"sink2": {"kafka-1"},
		"sink3": {"http-2", "kafka-2"},
	}
	if got := resolveImportResource(byKey, byName, "sink2", ""); got != "kafka-1" {
		t.Fatalf("唯一同名资源应回退命中, got %q", got)
	}
	// 类型不匹配时不能绑定同名但类型不同的资源
	if got := resolveImportResource(byKey, byName, "sink2", "HTTP推送"); got != "" {
		t.Fatalf("类型不匹配不应回退, got %q", got)
	}
	if got := resolveImportResource(byKey, byName, "sink3", ""); got != "" {
		t.Fatalf("同名多类型应视为无法确定, got %q", got)
	}
	if got := resolveImportResource(byKey, byName, "sink1", "HTTP推送"); got != "http-1" {
		t.Fatalf("类型匹配应命中, got %q", got)
	}
}

func TestStoredCompiledSQL(t *testing.T) {
	rule := models.RuleEngine{
		Filter: models.RuleEngineFilter{
			Sql:         `SELECT * FROM mqtt_stream`,
			CompiledSql: `SELECT * FROM mqtt_stream WHERE tenantId = 't1'`,
		},
	}
	got, err := storedCompiledSQL(rule, "t1")
	if err != nil {
		t.Fatalf("storedCompiledSQL: %v", err)
	}
	if got != rule.Filter.CompiledSql {
		t.Fatalf("应直接复用 compiled_sql, got %q", got)
	}

	legacy := models.RuleEngine{Filter: models.RuleEngineFilter{Sql: `SELECT * FROM mqtt_stream`}}
	got, err = storedCompiledSQL(legacy, "t1")
	if err != nil {
		t.Fatalf("legacy storedCompiledSQL: %v", err)
	}
	if !strings.Contains(got, "tenantId = 't1'") {
		t.Fatalf("存量规则应补租户过滤, got %q", got)
	}
}
