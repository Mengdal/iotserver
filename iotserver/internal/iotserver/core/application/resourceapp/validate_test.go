package resourceapp

import (
	"testing"

	"iotserver/internal/dtos"
)

func TestValidateName(t *testing.T) {
	valid := []string{"a", "abc", "推送实例", "实例_01", "a-b-c", "A1",
		"很长的名字很长的名字很长的名字很长的名字很长的名字很长的名字很长"} // 恰好 32 个字符
	for _, name := range valid {
		if err := validate(name, "HTTP推送", map[string]interface{}{"url": "http://x"}); err != nil {
			t.Fatalf("名称 %q 应合法: %v", name, err)
		}
	}

	invalid := []string{"", "1abc", "_abc", "-abc", "a b", "a@b", "名字!", "很长的名字很长的名字很长的名字很长的名字很长的名字很长的名字很长的名字"}
	for _, name := range invalid {
		if err := validate(name, "HTTP推送", map[string]interface{}{"url": "http://x"}); err == nil {
			t.Fatalf("名称 %q 应非法", name)
		}
	}
}

func TestValidateType(t *testing.T) {
	if err := validate("ok", "不支持的类型", map[string]interface{}{}); err == nil {
		t.Fatal("非法类型应报错")
	}
	for _, resourceType := range []string{"HTTP推送", "消息对队列MQTT", "消息队列Kafka", "InfluxDB", "TDengine"} {
		if err := validate("ok", resourceType, map[string]interface{}{
			"url": "http://x", "server": "127.0.0.1:1883", "topic": "t",
			"brokers": "127.0.0.1:9092", "addr": "127.0.0.1:8086", "databasename": "db",
			"host": "127.0.0.1", "database": "iot", "table": "tb",
		}); err != nil {
			t.Fatalf("类型 %s 必填齐全时应通过: %v", resourceType, err)
		}
	}
}

func TestValidateRequiredOptions(t *testing.T) {
	cases := []struct {
		resourceType string
		option       map[string]interface{}
		wantErr      bool
	}{
		{"HTTP推送", map[string]interface{}{"url": "http://x"}, false},
		{"HTTP推送", map[string]interface{}{}, true},
		{"HTTP推送", map[string]interface{}{"url": "   "}, true},
		{"HTTP推送", map[string]interface{}{"url": nil}, true},
		{"消息对队列MQTT", map[string]interface{}{"server": "127.0.0.1:1883"}, true}, // 缺 topic
		{"消息对队列MQTT", map[string]interface{}{"server": "s", "topic": "t"}, false},
		{"消息队列Kafka", map[string]interface{}{"brokers": "127.0.0.1:9092"}, true},
		{"消息队列Kafka", map[string]interface{}{"brokers": "127.0.0.1:9092", "topic": "t"}, false},
		{"InfluxDB", map[string]interface{}{"addr": "127.0.0.1:8086"}, true},
		{"InfluxDB", map[string]interface{}{"addr": "127.0.0.1:8086", "databasename": "db"}, false},
		{"TDengine", map[string]interface{}{"host": "127.0.0.1"}, true},
		{"TDengine", map[string]interface{}{"host": "h", "database": "d", "table": "t"}, false},
	}
	for _, tt := range cases {
		err := validate("合法名称", tt.resourceType, tt.option)
		if tt.wantErr && err == nil {
			t.Fatalf("%s %+v 应校验失败", tt.resourceType, tt.option)
		}
		if !tt.wantErr && err != nil {
			t.Fatalf("%s %+v 应校验通过, got %v", tt.resourceType, tt.option, err)
		}
	}
}

func TestNormalizeOption(t *testing.T) {
	if got := normalizeOption(nil); got == nil || len(got) != 0 {
		t.Fatalf("nil option 应归一化为空对象, got %#v", got)
	}
	in := map[string]interface{}{"url": "http://x"}
	if got := normalizeOption(in); len(got) != 1 {
		t.Fatalf("非空 option 应原样返回, got %#v", got)
	}
}

func TestOffsetLimit(t *testing.T) {
	offset, limit := dtos.OffsetLimit(dtos.BaseSearchConditionQuery{Page: 3, PageSize: 20})
	if offset != 40 || limit != 20 {
		t.Fatalf("offset=%d limit=%d, want 40/20", offset, limit)
	}
	// 缺省值
	offset, limit = dtos.OffsetLimit(dtos.BaseSearchConditionQuery{})
	if offset != 0 || limit != 10 {
		t.Fatalf("缺省 offset=%d limit=%d, want 0/10", offset, limit)
	}
}
