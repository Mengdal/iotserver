package dtos

import (
	"encoding/json"
	"fmt"
)

type EngineCreate struct {
	Id          int64  `json:"id"`
	Description string `json:"description" example:"描述"`
	Name        string `json:"name" example:"rule_001"`
}

type EngineUpdate struct {
	Id           int64   `json:"id"`
	Description  string  `json:"description" example:"描述"`
	Name         string  `json:"name" example:"rule_001"`
	Status       string  `json:"status"`
	Filter       Filters `json:"filter"`
	DataSourceId int64   `json:"DataSourceId"`
}
type Filters struct {
	Condition     string `json:"condition"`
	MessageSource string `json:"messageSource"`
	SelectName    string `json:"selectName"`
	SQL           string `json:"sql"`
}

type EngineOption struct {
	Id     int64       `json:"id"`     // 对应 DataResource.Id
	Type   string      `json:"type"`   // 不同 Type 对应不同的配置结构体
	Option interface{} `json:"option"` // 转发类型，如 HTTP / MQTT / Kafka / InfluxDB / TDengine
}

type HttpOption struct {
	URL        string            `json:"url"`
	Method     string            `json:"method"`
	BodyType   string            `json:"bodyType"`
	Headers    map[string]string `json:"headers"`
	SendSingle bool              `json:"sendSingle"`
}

type MqttOption struct {
	Broker          string `json:"broker"`
	Topic           string `json:"topic"`
	Client          string `json:"client"`
	ProtocolVersion string `json:"protocolVersion"`
	Qos             int    `json:"qos"`
	Username        string `json:"username"`
	Password        string `json:"password"`
	SendSingle      bool   `json:"sendSingle"`
}

type KafkaOption struct {
	Broker       string `json:"broker"`
	Topic        string `json:"topic"`
	SalAuthType  string `json:"salAuthType"`
	SaslUserName string `json:"saslUserName"`
	SaslPassword string `json:"saslPassword"`
	SendSingle   bool   `json:"sendSingle"`
}

type InfluxOption struct {
	Url          string `json:"url"`
	Token        string `json:"token"`
	Username     string `json:"username"`
	Password     string `json:"password"`
	DatabaseName string `json:"databaseName"`
	Measurement  string `json:"measurement"`
	TagKey       string `json:"tagKey"`
	TagValue     string `json:"tagValue"`
	SendSingle   bool   `json:"sendSingle"`
}

type TDengineOption struct {
	Host           string `json:"host"`
	Port           int    `json:"port"`
	User           string `json:"user"`
	Password       string `json:"password"`
	Database       string `json:"database"`
	Table          string `json:"table"`
	Fields         string `json:"fields"`
	ProvideTs      bool   `json:"provideTs"`
	TsFieldName    string `json:"tsFieldName"`
	STable         string `json:"sTable"`
	TagFields      string `json:"tagFields"`
	TableDataField string `json:"tableDataField"`
	SendSingle     bool   `json:"sendSingle"`
}

// ParseOption 根据 Type 返回对应的 Option 结构体实例
func ParseOption(resourceType string, optionStr string) (interface{}, error) {
	switch resourceType {
	case "HTTP推送":
		var opt HttpOption
		if err := json.Unmarshal([]byte(optionStr), &opt); err != nil {
			return nil, err
		}
		return opt, nil
	case "消息对队列MQTT":
		var opt MqttOption
		if err := json.Unmarshal([]byte(optionStr), &opt); err != nil {
			return nil, err
		}
		return opt, nil
	case "消息队列Kafka":
		var opt KafkaOption
		if err := json.Unmarshal([]byte(optionStr), &opt); err != nil {
			return nil, err
		}
		return opt, nil
	case "InfluxDB":
		var opt InfluxOption
		if err := json.Unmarshal([]byte(optionStr), &opt); err != nil {
			return nil, err
		}
		return opt, nil
	case "TDengine":
		var opt TDengineOption
		if err := json.Unmarshal([]byte(optionStr), &opt); err != nil {
			return nil, err
		}
		return opt, nil
	default:
		return nil, fmt.Errorf("未知的转发类型: %s", resourceType)
	}
}
