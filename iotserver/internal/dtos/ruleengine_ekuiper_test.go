package dtos

import (
	"testing"

	"iotserver/internal/pkg/constants"
)

// 四种数据资源必须生成对应的 eKuiper sink 动作。
func TestGetRuleEngineEkuiperActions(t *testing.T) {
	cases := []struct {
		name     string
		resource string
		option   map[string]interface{}
		check    func(*testing.T, Actions)
	}{
		{
			name:     "http",
			resource: string(constants.HttpResource),
			option:   map[string]interface{}{"url": "http://127.0.0.1:18080/push"},
			check: func(t *testing.T, a Actions) {
				if a.Rest == nil {
					t.Fatal("HTTP 应生成 rest action")
				}
				if a.Rest["method"] != "POST" || a.Rest["bodyType"] != "json" ||
					a.Rest["format"] != "json" || a.Rest["sendSingle"] != true {
					t.Fatalf("HTTP 默认参数不正确: %#v", a.Rest)
				}
			},
		},
		{
			name:     "mqtt",
			resource: string(constants.MQTTResource),
			option:   map[string]interface{}{"server": "tcp://127.0.0.1:58090", "topic": "sink/mqtt"},
			check: func(t *testing.T, a Actions) {
				if a.MQTT == nil || a.MQTT["topic"] != "sink/mqtt" {
					t.Fatalf("MQTT action 不正确: %#v", a.MQTT)
				}
			},
		},
		{
			name:     "kafka",
			resource: string(constants.KafkaResource),
			option:   map[string]interface{}{"brokers": "127.0.0.1:9092", "topic": "sink-kafka"},
			check: func(t *testing.T, a Actions) {
				if a.Kafka == nil || a.Kafka["brokers"] != "127.0.0.1:9092" {
					t.Fatalf("Kafka action 不正确: %#v", a.Kafka)
				}
			},
		},
		{
			name:     "influx",
			resource: string(constants.InfluxDBResource),
			option: map[string]interface{}{
				"addr": "http://127.0.0.1:8086", "databasename": "iot", "measurement": "sensor",
				"tagkey": "deviceId", "tagvalue": "dev-1",
			},
			check: func(t *testing.T, a Actions) {
				if a.Influx == nil || a.Influx["addr"] != "http://127.0.0.1:8086" {
					t.Fatalf("InfluxDB action 不正确: %#v", a.Influx)
				}
				if a.Influx["database"] != "iot" {
					t.Fatalf("databasename 应映射为 database: %#v", a.Influx)
				}
				tags, _ := a.Influx["tags"].(map[string]string)
				if tags["deviceId"] != "dev-1" {
					t.Fatalf("tagkey/tagvalue 应映射为 tags: %#v", a.Influx)
				}
				for _, k := range []string{"databasename", "tagkey", "tagvalue"} {
					if _, ok := a.Influx[k]; ok {
						t.Fatalf("不应保留 %s: %#v", k, a.Influx)
					}
				}
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			actions, err := GetRuleEngineEkuiperActions(tc.resource, tc.option)
			if err != nil {
				t.Fatalf("生成 action 失败: %v", err)
			}
			if len(actions) != 1 {
				t.Fatalf("应生成 1 个 action，实际 %d", len(actions))
			}
			tc.check(t, actions[0])
		})
	}

	if _, err := GetRuleEngineEkuiperActions("TDengine", map[string]interface{}{"host": "h"}); err == nil {
		t.Fatal("已删除的 TDengine 类型应返回错误")
	}
}
