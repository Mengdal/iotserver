package services

import (
	"encoding/json"
	"fmt"
	"github.com/beego/beego/v2/client/orm"
	beego "github.com/beego/beego/v2/server/web"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"iotServer/common"
	"iotServer/iotp"
	"iotServer/models"
	"iotServer/models/constants"
	"iotServer/utils"
	"log"
	"net"
	"regexp"
	"strings"
	"time"
)

var mqttURL, _ = beego.AppConfig.String("mqttServer")
var mqttClientId, _ = beego.AppConfig.String("mqttClientId")
var mqttUsername, _ = beego.AppConfig.String("mqttUsername")
var mqttPassword, _ = beego.AppConfig.String("mqttPassword")

var Connector common.MqttConnector
var Processor *PropertySetProcessor

func InitMQTT() {
	host, port, err := net.SplitHostPort(mqttURL)
	if err != nil {
		log.Fatalf("MQTT配置解析失败: %v", err)
	}

	Connector = common.NewMqttConnector(
		host, port, 5, mqttUsername, mqttPassword, mqttClientId, 60, false, 10,
	)

	if err := Connector.Connect(); err != nil {
		log.Printf("[WARN] MQTT连接失败: %v（将继续启动 Web 项目）", err)
	}

	switchService := NewSwitchService()
	Processor = NewPropertySetProcessor(Connector, switchService)
	if err := Processor.Init(); err != nil {
		log.Printf("订阅控制响应失败: %v", err)
	}

}

func NewSwitchService() *SwitchService {
	return &SwitchService{}
}

type SwitchService struct{}

func (s *SwitchService) WriteBack(clientID string, data map[string]interface{}) error {
	log.Printf("控制响应写入：客户端 %s 响应内容 %+v", clientID, data)

	// 1. 提取必要字段
	seq, _ := data["seq"].(string)
	statusBool, _ := data["status"].(bool)
	val, _ := data["value"].(string)
	var status string
	if statusBool {
		status = "true"
	}
	// 2. 分割
	dn, tag := "", ""
	if tagId, ok := data["TagId"].(string); ok {
		if parts := strings.SplitN(tagId, ".", 2); len(parts) == 2 {
			dn, tag = parts[0], parts[1]
		} else {
			log.Printf("警告：TagId 格式错误: %s", tagId)
		}
	}
	// 3. 异步记录日志
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("writeLog panic: %v", r)
			}
		}()
		writeLog(seq, status, clientID, dn, tag, val, "手动控制", 0)
	}()

	return nil
}

// 控制命令处理器
type PropertySetProcessor struct {
	mqttClient    common.MqttConnector
	switchService *SwitchService
}

// 创建新的处理器实例
func NewPropertySetProcessor(mqttClient common.MqttConnector, switchService *SwitchService) *PropertySetProcessor {
	return &PropertySetProcessor{
		mqttClient:    mqttClient,
		switchService: switchService,
	}
}

// 初始化订阅
func (p *PropertySetProcessor) Init() error {
	// 订阅控制响应主题
	if err := p.mqttClient.Subscribe("lm/gw/ctrlResponse/+", 0, p.handleMessage); err != nil {
		return fmt.Errorf("订阅失败: %v", err)
	} else {
		log.Println("已订阅控制命令响应主题: lm/gw/ctrlResponse/+")
	}
	//订阅报警响应主题
	if err := p.mqttClient.Subscribe("/edge/event/+/post", 0, p.handleMessage); err != nil {
		return fmt.Errorf("订阅失败: %v", err)
	} else {
		log.Println("已订阅报警事件响应主题: /edge/event/+/post")
	}
	//平台预处理实时数据主题供流数据处理
	if err := p.mqttClient.Subscribe("/edge/property/+/post", 0, p.handleMessage); err != nil {
		return fmt.Errorf("订阅失败: %v", err)
	} else {
		log.Println("已订阅实时数据主题: /edge/property/+/post")
	}
	//订阅流处理消息供设备状态处理
	if err := p.mqttClient.Subscribe("/edge/stream/+/post", 0, p.handleMessage); err != nil {
		return fmt.Errorf("订阅失败: %v", err)
	} else {
		log.Println("已订阅实时数据主题: /edge/stream/+/post")
	}
	return nil
}

// 消息处理回调
func (p *PropertySetProcessor) handleMessage(client mqtt.Client, msg mqtt.Message) {
	// 使用 goroutine 异步处理
	go func() {
		start := time.Now()
		topic := msg.Topic()
		payload := string(msg.Payload())

		log.Printf("消息开始处理: %s", topic)

		defer func() {
			log.Printf("消息处理完成: %s (耗时: %v)", topic, time.Since(start))
		}()

		// 处理消息
		if err := p.process(topic, payload); err != nil {
			log.Printf("消息处理失败: %s -- %v", topic, err)
		}
	}()
}

func (p *PropertySetProcessor) process(topic, payload string) error {
	// 判断是控制响应主题还是报警主题
	if strings.HasPrefix(topic, "lm/gw/ctrlResponse/") {
		// 处理控制响应
		return p.processControlResponse(topic, payload)
	} else if strings.HasPrefix(topic, "/edge/event/") && strings.HasSuffix(topic, "/post") {
		// 处理报警事件
		return p.processAlertEvent(topic, payload)
	} else if strings.HasPrefix(topic, "/edge/property/") && strings.HasSuffix(topic, "/post") {
		// 处理实时数据
		return p.handlePropertyMessage(topic, payload)
	} else if strings.HasPrefix(topic, "/edge/stream/") && strings.HasSuffix(topic, "/post") {
		// 处理设备状态
		return p.handleStreamMessage(topic, payload)
	}
	return fmt.Errorf("未知的主题类型: %s", topic)
}

// 处理控制响应
func (p *PropertySetProcessor) processControlResponse(topic, payload string) error {
	// 正则匹配设备ID
	regex := regexp.MustCompile(`(?i)lm/gw/ctrlResponse/(.+)`)
	matches := regex.FindStringSubmatch(topic)
	if len(matches) < 2 {
		return fmt.Errorf("控制响应主题格式不匹配: %s", topic)
	}

	clientID := matches[1]
	log.Printf("控制命令返回: %s %s", topic, payload)

	// 解析JSON
	var object map[string]interface{}
	if err := json.Unmarshal([]byte(payload), &object); err != nil {
		return fmt.Errorf("JSON解析失败: %v", err)
	}

	// 调用业务服务
	return p.switchService.WriteBack(clientID, object)
}

// 处理报警事件
func (p *PropertySetProcessor) processAlertEvent(topic, payload string) error {
	// 从主题中提取设备ID
	parts := strings.Split(topic, "/")
	if len(parts) < 3 {
		return fmt.Errorf("报警主题格式错误: %s", topic)
	}
	//sn := parts[3]

	log.Printf("收到报警事件: %s %s", topic, payload)

	// 解析JSON
	var alertData map[string]interface{}
	if err := json.Unmarshal([]byte(payload), &alertData); err != nil {
		return fmt.Errorf("报警JSON解析失败: %v", err)
	}

	point := alertData["tag"]
	split := strings.Split(point.(string), ".")
	dn := split[0]
	tag := split[1]
	eventTime := alertData["timestamp"]
	event := alertData["event"]
	value := alertData["value"]
	typeName := alertData["type"]
	if typeName == "AlarmTrigger" {
		typeName = "网关事件触发"
	} else {
		typeName = "网关事件解除"
	}
	typeR := alertData["status"]
	if typeR == "Error" && value == "0" {
		typeR = "质量不为Good"
	} else {
		typeR = "点值超出范围"
	}

	timestamp, _ := iotp.GetTimestamp(eventTime.(string))
	out := map[string]interface{}{
		"alert_level": "重要",
		"code":        tag,
		"dn":          dn,
		"start_at":    timestamp,
		"name":        tag,
		"rule_name":   event,
		"trigger":     typeName,
		"type":        typeR,
		"value":       value,
	}
	newPayload, err := json.Marshal(out)
	if err != nil {
		return fmt.Errorf("JSON序列化失败: %v", err)
	}

	// 网关事件直接存入Alert_List
	o := orm.NewOrm()
	// 构建告警记录
	alert := &models.AlertList{
		AlertRule:   nil,
		TriggerTime: time.Now().UnixMilli(),
		IsSend:      false,
		Status:      string(constants.Untreated),
		AlertResult: string(newPayload),
	}

	// 保存到数据库
	if err = alert.BeforeInsert(); err != nil {
		return fmt.Errorf("插入失败: %v", err)
	}
	if _, err = o.Insert(alert); err != nil {
		return fmt.Errorf("保存告警记录失败: %v", err)
	}

	return nil
}

// 转发实时数据主题
func (p *PropertySetProcessor) handlePropertyMessage(topic, payload string) error {
	// 提取SN
	parts := strings.Split(topic, "/")
	if len(parts) < 5 {
		return fmt.Errorf("主题格式错误:%v", topic)
	}
	sn := parts[3]
	// 解析原始数组数据
	var arr []struct {
		Dn         string                 `json:"dn"`
		Properties map[string]interface{} `json:"properties"`
		Time       int64                  `json:"time"`
	}
	if err := json.Unmarshal([]byte(payload), &arr); err != nil {
		return fmt.Errorf("JSON解析失败:%v", err)
	}

	for _, item := range arr {
		// 转换为 eKuiper 友好格式
		data := make(map[string]map[string]interface{})
		for k, v := range item.Properties {
			data[k] = map[string]interface{}{
				"value": v,
				"time":  item.Time,
			}
		}
		out := map[string]interface{}{
			"dn":          item.Dn,
			"messageType": "PROPERTY_REPORT",
			"data":        data,
		}
		newPayload, err := json.Marshal(out)
		if err != nil {
			log.Println("JSON序列化失败:", err)
			continue
		}

		// 转发到新主题（每个设备一个主题）
		newTopic := fmt.Sprintf("/edge/stream/%s/post", sn)
		if err := p.mqttClient.Publish(newTopic, 0, newPayload); err != nil {
			log.Println("转发失败:", err.Error())
		} else {
			log.Printf("已转发到 %s: %s\n", newTopic, string(newPayload))
		}
	}
	return nil
}

var tagService = iotp.TagService{}

// 处理流数据为更新设备状态
func (p *PropertySetProcessor) handleStreamMessage(topic, payload string) error {
	// 解析传入的 payload 数据
	var message struct {
		Data        map[string]map[string]interface{} `json:"data"`
		Dn          string                            `json:"dn"`
		MessageType string                            `json:"messageType"`
	}
	if err := json.Unmarshal([]byte(payload), &message); err != nil {
		return fmt.Errorf("JSON解析失败:%v", err)
	}
	// 属性上报更新设备状态
	if message.MessageType != "PROPERTY_REPORT" {
		return nil
	}
	// - 数据持久化
	tagService.AddTag(message.Dn, "status", "1")
	tagService.AddTag(message.Dn, "lastOnline", utils.InterfaceToString(time.Now().Unix()))

	return nil
}

// Deal 发布控制命令
func (p *PropertySetProcessor) Deal(dn, tag, val, channel string, userId int64) (string, error) {
	//优先掏出SN
	var sn string
	device, err := iotp.NewTagService().ListTagsByDevice(dn)
	sn = device[dn]["GWSN"]

	//控制命令保存
	seq := fmt.Sprintf("seq-%d", time.Now().UnixNano())
	writeLog(seq, "WAIT", sn, dn, tag, val, channel, userId)

	if err != nil || sn == "" {
		return seq, fmt.Errorf("设备未包含网关信息")
	}

	topic := fmt.Sprintf("lm/iot/ctrlRequest/%s", sn)
	body := fmt.Sprintf("[{\"seq\":\"%s\",\"deviceCode\":\"%s\", \"tagCode\": \"%s\", \"val\": \"%s\"}]", seq, dn, tag, val)

	if err := p.mqttClient.Publish(topic, 0, []byte(body)); err != nil {
		return seq, fmt.Errorf("写入控制命令失败: %v", err)
	}
	log.Printf("已发布控制命令: %s", topic)
	return seq, nil
}

func (p *PropertySetProcessor) SendOffline(dn string) error {
	//优先掏出SN
	var sn string
	device, err := iotp.NewTagService().ListTagsByDevice(dn)
	sn = device[dn]["GWSN"]
	if err != nil || sn == "" {
		return fmt.Errorf("设备未包含网关信息")
	}

	topic := fmt.Sprintf("/edge/stream/%s/post", sn)

	out := map[string]interface{}{
		"dn":          dn,
		"messageType": "DEVICE_STATUS",
		"status":      "offline",
		"time":        time.Now().Unix(),
	}

	newPayload, err := json.Marshal(out)
	if err != nil {
		return fmt.Errorf("JSON序列化失败:%v", err)
	}

	if err := p.mqttClient.Publish(topic, 0, newPayload); err != nil {
		return fmt.Errorf("发布离线命令失败: %v", err)
	}
	log.Printf("已发布离线命令: %s", topic)
	return nil
}

func writeLog(seq, status, sn, dn, tag, val, channel string, userId int64) {
	o := orm.NewOrm()
	if status == "WAIT" {
		writeLog := models.WriteLog{
			Seq:     seq,
			Sn:      sn,
			Dn:      dn,
			Tag:     tag,
			Val:     val,
			Status:  "WAIT",
			Channel: channel,
			UserId:  userId,
		}
		_ = writeLog.BeforeInsert()
		_, err := o.Insert(&writeLog)
		if err != nil {
			log.Println("命令写入失败", err.Error())
		}
	} else {
		var writeLog models.WriteLog
		writeLog.Seq = seq
		err := o.Read(&writeLog)
		if err != nil {
			log.Println("未查询到该响应seq")
		} else {
			if status == "true" {
				writeLog.Status = "SUCCESS"
			} else {
				writeLog.Status = "FAIL"
			}
			_ = writeLog.BeforeUpdate()
			if _, err := o.Update(&writeLog); err != nil {
				log.Println("命令状态更新失败：", err)
			} else {
				log.Println("命令状态更新成功")
			}
		}
	}
}
