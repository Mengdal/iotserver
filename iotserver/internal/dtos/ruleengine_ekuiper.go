package dtos

import (
	"fmt"
	"strings"

	"iotserver/internal/pkg/constants"
)

// GetRuleEngineEkuiperActions 把规则绑定的数据资源转换为 eKuiper 规则动作。
//
// 数据资源表单字段与 eKuiper sink 属性同名（server/topic/clientId、brokers、addr、host/port…），
// 因此按类型透传即可；同时补齐 eKuiper 侧需要的默认值，并剔除空值避免其解析失败。
func GetRuleEngineEkuiperActions(resourceType string, option map[string]interface{}) ([]Actions, error) {
	props := cleanOption(option)

	switch constants.DataResourceType(resourceType) {
	case constants.HttpResource:
		if _, ok := props["url"]; !ok {
			return nil, fmt.Errorf("数据资源缺少 url")
		}
		if _, ok := props["method"]; !ok {
			props["method"] = "POST"
		}
		if _, ok := props["bodyType"]; !ok {
			props["bodyType"] = "json"
		}
		if _, ok := props["timeout"]; !ok {
			props["timeout"] = 5000
		}
		if _, ok := props["format"]; !ok {
			props["format"] = "json"
		}
		props["sendSingle"] = true
		props["runAsync"] = false
		return []Actions{{Rest: props}}, nil
	case constants.MQTTResource:
		return []Actions{{MQTT: props}}, nil
	case constants.KafkaResource:
		return []Actions{{Kafka: props}}, nil
	case constants.InfluxDBResource:
		// 资源表单字段与 eKuiper influx sink 属性名不同，这里做一次映射：
		//   databasename -> database
		//   tagkey/tagvalue -> tags: {tagkey: tagvalue}
		if db, ok := props["databasename"]; ok {
			props["database"] = db
			delete(props, "databasename")
		}
		if key, ok := props["tagkey"].(string); ok && key != "" {
			value, _ := props["tagvalue"].(string)
			props["tags"] = map[string]string{key: value}
		}
		delete(props, "tagkey")
		delete(props, "tagvalue")
		return []Actions{{Influx: props}}, nil
	default:
		return nil, fmt.Errorf("不支持的数据资源类型: %s", resourceType)
	}
}

// cleanOption 剔除空字符串/空集合的配置项，并去掉前后空格
func cleanOption(option map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{}, len(option))
	for k, v := range option {
		if v == nil {
			continue
		}
		if s, ok := v.(string); ok {
			if strings.TrimSpace(s) == "" {
				continue
			}
			out[k] = strings.TrimSpace(s)
			continue
		}
		out[k] = v
	}
	return out
}

// BuildRuleEngineSql 生成规则 SQL：优先用前端给出的 filter.sql，否则按条件拼装
func BuildRuleEngineSql(filterRule string, selectName, condition string) string {
	if strings.TrimSpace(filterRule) != "" {
		return strings.TrimSpace(filterRule)
	}
	if strings.TrimSpace(selectName) == "" {
		selectName = "*"
	}
	sql := fmt.Sprintf("SELECT %s FROM mqtt_stream", strings.TrimSpace(selectName))
	if strings.TrimSpace(condition) != "" {
		sql += " where " + strings.TrimSpace(condition)
	}
	return sql
}
