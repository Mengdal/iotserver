package services

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"github.com/beego/beego/v2/client/orm"
	"github.com/beego/beego/v2/core/logs"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/sasl/plain"
	"io/ioutil"
	"iotServer/models"
	"iotServer/models/dtos"
	"net"
	"net/http"
	"strings"
	"time"
)

// ParseOption 根据 Type 返回对应的 Option 结构体实例
func ParseOption(resourceType string, optionStr interface{}) interface{} {
	var data []byte
	switch v := optionStr.(type) {
	case string:
		data = []byte(v)
	case []byte:
		data = v
	case map[string]interface{}:
		b, err := json.Marshal(v) // map 转 JSON
		if err != nil {
			return nil
		}
		data = b
	default:
		// 不支持的类型
		return nil
	}
	switch resourceType {
	case "HTTP推送":
		var opt dtos.HttpOption
		if err := json.Unmarshal(data, &opt); err != nil || opt.URL == "" {
			return nil
		}
		return opt
	case "消息对队列MQTT":
		var opt dtos.MqttOption
		if err := json.Unmarshal(data, &opt); err != nil || opt.Server == "" || opt.Topic == "" {
			return nil
		}
		return opt
	case "消息队列Kafka":
		var opt dtos.KafkaOption
		if err := json.Unmarshal(data, &opt); err != nil || opt.Brokers == "" || opt.Topic == "" {
			return nil
		}
		return opt
	case "InfluxDB":
		var opt dtos.InfluxOption
		if err := json.Unmarshal(data, &opt); err != nil {
			return nil
		}
		return opt
	case "TDengine":
		var opt dtos.TDengineOption
		if err := json.Unmarshal(data, &opt); err != nil {
			return nil
		}
		return opt
	default:
		return nil
	}
}

// ValidateConnection 验证连接
func ValidateConnection(resourceType string, option interface{}) error {
	switch resourceType {
	case "HTTP推送":
		opt, ok := option.(dtos.HttpOption)
		if !ok {
			return fmt.Errorf("参数类型错误")
		}
		return validateHttp(opt)

	case "消息对队列MQTT":
		opt, ok := option.(dtos.MqttOption)
		if !ok {
			return fmt.Errorf("参数类型错误")
		}
		return validateMqtt(opt)

	case "消息队列Kafka":
		opt, ok := option.(dtos.KafkaOption)
		if !ok {
			return fmt.Errorf("参数类型错误")
		}
		return validateKafka(opt)

	case "InfluxDB":
		opt, ok := option.(dtos.InfluxOption)
		if !ok {
			return fmt.Errorf("参数类型错误")
		}
		return validateInflux(opt)

	case "TDengine":
		opt, ok := option.(dtos.TDengineOption)
		if !ok {
			return fmt.Errorf("参数类型错误")
		}
		return validateTDengine(opt)

	default:
		return fmt.Errorf("不支持的数据源类型: %s", resourceType)
	}
}

// HTTP 测试
func validateHttp(opt dtos.HttpOption) error {
	client := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, // 跳过证书验证（如果是 https）
		},
	}
	req, err := http.NewRequest(opt.Method, opt.URL, bytes.NewBuffer([]byte("{}")))
	if err != nil {
		return err
	}
	for k, v := range opt.Headers {
		req.Header.Set(k, v)
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	return fmt.Errorf("http 响应失败: %d", resp.StatusCode)
}

// MQTT 测试
func validateMqtt(opt dtos.MqttOption) error {
	opts := mqtt.NewClientOptions().AddBroker(opt.Server).SetClientID(opt.Client)
	if opt.Username != "" {
		opts.SetUsername(opt.Username)
	}
	if opt.Password != "" {
		opts.SetPassword(opt.Password)
	}
	client := mqtt.NewClient(opts)
	token := client.Connect()
	if token.Wait() && token.Error() != nil {
		return token.Error()
	}
	client.Disconnect(100)
	return nil
}

// Kafka 测试
func validateKafka(opt dtos.KafkaOption) error {
	dialer := &kafka.Dialer{
		Timeout:   10 * time.Second,
		DualStack: true,
	}

	// 如果配置了 SASL
	if opt.SaslUserName != "" && opt.SaslPassword != "" {
		dialer.SASLMechanism = plain.Mechanism{
			Username: opt.SaslUserName,
			Password: opt.SaslPassword,
		}
	}

	// 建立连接
	conn, err := dialer.DialContext(context.Background(), "tcp", opt.Brokers)
	if err != nil {
		return err
	}
	defer conn.Close()

	return nil
}

// InfluxDB 测试
func validateInflux(opt dtos.InfluxOption) error {
	client := influxdb2.NewClient(opt.Url, opt.Token)
	defer client.Close()
	_, err := client.Ready(context.Background())
	if err != nil {
		return err
	}
	return nil
}

// TDengine 测试 (简单 TCP 检查)
func validateTDengine(opt dtos.TDengineOption) error {
	addr := fmt.Sprintf("%s:%d", opt.Host, opt.Port)
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		return err
	}
	defer conn.Close()
	return nil
}

func EngineCallBack(req map[string]interface{}) error {
	o := orm.NewOrm()
	var engine models.RuleEngine
	engineName := req["rule_id"].(string)
	split := strings.Split(engineName, "__")
	engine.Name = split[0]
	req["engine_name"] = split[0]

	if err := o.Read(&engine, "Name"); err != nil {
		return fmt.Errorf(err.Error())
	}
	o.LoadRelated(&engine, "DataResource")
	t := engine.DataResource.Type
	option := ParseOption(t, engine.DataResource.Option)
	logs.Info(option)
	var err error
	switch t {
	case "HTTP推送":
		opt, ok := option.(dtos.HttpOption)
		if !ok {
			return fmt.Errorf("option 类型断言失败: HTTP推送")
		}
		err = sendHttp(opt, req)
	case "消息对队列MQTT":
		opt, ok := option.(dtos.MqttOption)
		if !ok {
			return fmt.Errorf("option 类型断言失败: MQTT")
		}
		err = sendMqtt(opt, req)
	case "消息队列Kafka":
		opt, ok := option.(dtos.KafkaOption)
		if !ok {
			return fmt.Errorf("option 类型断言失败: Kafka")
		}
		err = sendKafka(opt, req)
	case "InfluxDB":
		opt, ok := option.(dtos.InfluxOption)
		if !ok {
			return fmt.Errorf("option 类型断言失败: InfluxDB")
		}
		err = sendInflux(opt, req)
	case "TDengine":
		opt, ok := option.(dtos.TDengineOption)
		if !ok {
			return fmt.Errorf("option 类型断言失败: InfluxDB")
		}
		err = sendTDengine(opt, req)
	default:
		err = fmt.Errorf("unsupported engine type: %s", t)

	}
	return err
}

// ------------------ 各种 Send 实现 ------------------

func sendHttp(opt dtos.HttpOption, req map[string]interface{}) error {
	data, _ := json.Marshal(req)

	client := &http.Client{Timeout: 10 * time.Second}
	reqHttp, err := http.NewRequest(opt.Method, opt.URL, bytes.NewBuffer(data))
	if err != nil {
		return err
	}
	for k, v := range opt.Headers {
		reqHttp.Header.Set(k, v)
	}
	resp, err := client.Do(reqHttp)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := ioutil.ReadAll(resp.Body)
	logs.Info("http response:", string(body))
	return nil
}

func sendMqtt(opt dtos.MqttOption, req map[string]interface{}) error {
	opts := mqtt.NewClientOptions().
		AddBroker(opt.Server).
		SetClientID(opt.Client).
		SetUsername(opt.Username).
		SetPassword(opt.Password)

	client := mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		return token.Error()
	}
	defer client.Disconnect(250)

	data, _ := json.Marshal(req)
	token := client.Publish(opt.Topic, byte(opt.Qos), false, data)
	token.Wait()
	return token.Error()
}

// TODO 以下未测试
func sendKafka(opt dtos.KafkaOption, req map[string]interface{}) error {
	writer := kafka.NewWriter(kafka.WriterConfig{
		Brokers: strings.Split(opt.Brokers, ","),
		Topic:   opt.Topic,
	})
	defer writer.Close()

	data, _ := json.Marshal(req)
	err := writer.WriteMessages(
		nil,
		kafka.Message{Value: data},
	)
	return err
}

func sendInflux(opt dtos.InfluxOption, req map[string]interface{}) error {
	// 简单点：写行协议
	// measurement,tagKey=tagValue field=value timestamp
	data := fmt.Sprintf("%s,%s=%s value=\"%s\" %d",
		opt.Measurement,
		opt.TagKey, opt.TagValue,
		toJSONString(req),
		time.Now().UnixNano(),
	)

	url := fmt.Sprintf("%s/api/v2/write?org=%s&bucket=%s&precision=ns",
		opt.Url, opt.Username, opt.DatabaseName)
	reqHttp, err := http.NewRequest("POST", url, bytes.NewBuffer([]byte(data)))
	if err != nil {
		return err
	}
	reqHttp.Header.Set("Authorization", "Token "+opt.Token)
	client := &http.Client{}
	resp, err := client.Do(reqHttp)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := ioutil.ReadAll(resp.Body)
	logs.Info("influx response:", string(body))
	return nil
}

func sendTDengine(opt dtos.TDengineOption, req map[string]interface{}) error {
	// 这里一般用官方 Go driver 或 REST 接口，这里给一个简化版
	url := fmt.Sprintf("http://%s:%d/rest/sql", opt.Host, opt.Port)
	sql := fmt.Sprintf("INSERT INTO %s.%s VALUES (NOW, '%s')",
		opt.Database, opt.Table, toJSONString(req))

	resp, err := http.Post(url, "text/plain", strings.NewReader(sql))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := ioutil.ReadAll(resp.Body)
	logs.Info("tdengine response:", string(body))
	return nil
}

func toJSONString(v interface{}) string {
	b, _ := json.Marshal(v)
	return string(b)
}
