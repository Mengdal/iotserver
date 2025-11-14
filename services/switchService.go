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
		host, port, 5, mqttUsername, mqttPassword, mqttClientId, 60, true, 10,
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
	tdWriter      *TDengineWriter
}

// 创建新的处理器实例
func NewPropertySetProcessor(mqttClient common.MqttConnector, switchService *SwitchService) *PropertySetProcessor {
	service, _ := NewTDengineService()
	writer := service.NewTDengineWriter(2*time.Second, 500)
	initWorkerPool()
	return &PropertySetProcessor{
		mqttClient:    mqttClient,
		switchService: switchService,
		tdWriter:      writer,
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

// 消息处理回调 - 仅负责接收消息并入队
func (p *PropertySetProcessor) handleMessage(client mqtt.Client, msg mqtt.Message) {

	// 检查是否为保留消息
	if msg.Retained() {
		return
	}
	topic := msg.Topic()
	payload := string(msg.Payload())

	// 根据主题选择处理函数
	var jobType string
	if strings.HasPrefix(topic, "lm/gw/ctrlResponse/") {
		jobType = "control_response"
	} else if strings.HasPrefix(topic, "/edge/event/") && strings.HasSuffix(topic, "/post") {
		jobType = "alert_event"
	} else if strings.HasPrefix(topic, "/edge/property/") && strings.HasSuffix(topic, "/post") {
		jobType = "property_message"
	} else if strings.HasPrefix(topic, "/edge/stream/") && strings.HasSuffix(topic, "/post") {
		jobType = "stream_message"
	} else {
		log.Printf("未知的主题类型: %s", topic)
		return
	}

	// 将任务提交到工作池
	job := Job{
		Topic:     topic,
		Payload:   payload,
		Type:      jobType,
		Processor: Processor,
	}

	// 非阻塞地提交任务，如果队列满了就丢弃并记录
	select {
	case jobQueue <- job:
		log.Printf("任务已提交到队列: %s", topic)
	default:
		log.Printf("任务队列已满，丢弃消息: %s", topic)
	}
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

// 转发实时数据主题
func (p *PropertySetProcessor) handlePropertyMessage(topic, payload string) error {
	// 提取SN
	parts := strings.Split(topic, "/")
	if len(parts) < 5 {
		return fmt.Errorf("主题格式错误:%v", topic)
	}
	sn := parts[3]
	// 解析数组数据
	var arr []MqttMessage
	if err := json.Unmarshal([]byte(payload), &arr); err != nil {
		return fmt.Errorf("JSON解析失败:%v", err)
	}

	// 或者完全异步处理，不等待写入完成
	for _, m := range arr {
		go func(msg MqttMessage) {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("TDengine 写入 panic: %v", r)
				}
			}()
			p.tdWriter.Add(msg)
		}(m)
	}

	// 为每个item创建转发任务
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
			log.Printf("JSON序列化失败: %v", err)
			continue
		}

		// 创建转发任务
		tasks := []struct {
			topic string
		}{
			{fmt.Sprintf("/edge/stream/%s/post", sn)},
			{fmt.Sprintf("/edge/event/%s/post", sn)},
		}

		for _, task := range tasks {
			if err := p.mqttClient.Publish(task.topic, 0, newPayload); err != nil {
				log.Printf("消息转发失败: %s -- %v", task.topic, err)
			} else {
				log.Printf("消息转发完成: %s", task.topic)
			}
		}
	}

	return nil
}

var tagService = iotp.TagService{}

// 处理流数据为更新设备状态
func (p *PropertySetProcessor) handleStreamMessage(topic, payload string) error {
	parts := strings.Split(topic, "/")
	if len(parts) < 5 {
		return fmt.Errorf("主题格式错误:%v", topic)
	}
	sn := parts[3]
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
	if message.MessageType == "PROPERTY_REPORT" {
		dn := message.Dn
		//如果是系统点PIN上网关
		if dn == "system" {
			dn = dn + sn
		}
		// - 数据持久化 使用线程服务更新设备状态，避免频繁查询
		UpdateDeviceStatus(sn, dn, tagService)
	}

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
