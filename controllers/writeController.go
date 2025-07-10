package controllers

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

// Command @Title 控制下发
// @Description 发送控制命令
// @Param Authorization    header   string  true   "Bearer YourToken"
// @Param deviceCode query string false "设备"
// @Param tagCode query string false "tag点"
// @Param val query string true "值"
// @Success 200 {string} string "控制命令发送成功"
// @Failure 500 {object} map[string]interface{} "控制命令发送失败"
// @router /command [post]
func (c *WriteController) Command(deviceCode, tagCode, val string) {
	userId := c.Ctx.Input.GetData("user_id").(int64)
	// 发布控制命令
	if _, err := Processor.Deal(deviceCode, tagCode, val, "手动控制", userId); err != nil {
		c.Error(400, "控制命令发送失败,"+err.Error())
	}
}

type WriteController struct {
	BaseController
}

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
		// 处理报警主题
		return p.processAlertEvent(topic, payload)
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
	//deviceID := parts[2]

	log.Printf("收到报警事件: %s %s", topic, payload)

	// 解析JSON
	var alertData map[string]interface{}
	if err := json.Unmarshal([]byte(payload), &alertData); err != nil {
		return fmt.Errorf("报警JSON解析失败: %v", err)
	}

	// 创建报警记录
	alertList := models.AlertList{
		TriggerTime: time.Now().UnixNano() / 1e6, // 转换为毫秒
		Status:      "未处理",
		AlertResult: payload,
	}

	// 如果有规则ID，可以设置关联
	if ruleID, ok := alertData["rule_id"].(float64); ok {
		alertList.AlertRule = &models.AlertRule{Id: int64(ruleID)}
	}

	// 保存到数据库
	o := orm.NewOrm()
	if _, err := o.Insert(&alertList); err != nil {
		return fmt.Errorf("保存报警记录失败: %v", err)
	}

	log.Printf("报警记录已保存: %v", alertList.Id)
	return nil
}

// Deal 发布控制命令
func (p *PropertySetProcessor) Deal(dn, tag, val, channel string, userId int64) (string, error) {
	//优先掏出SN
	var sn string
	device, err := iotp.NewTagService().ListTagsByDevice(dn)
	sn = device["GWSN"]
	if err != nil || sn == "" {
		return "", fmt.Errorf("设备未包含网关信息")
	}

	seq := fmt.Sprintf("seq-%d", time.Now().UnixNano())
	topic := fmt.Sprintf("lm/iot/ctrlRequest/%s", sn)
	body := fmt.Sprintf("[{\"seq\":\"%s\",\"deviceCode\":\"%s\", \"tagCode\": \"%s\", \"val\": \"%s\"}]", seq, dn, tag, val)
	//控制命令保存
	writeLog(seq, "WAIT", sn, dn, tag, val, channel, userId)
	if err := p.mqttClient.Publish(topic, 0, []byte(body)); err != nil {
		return seq, fmt.Errorf("写入控制命令失败: %v", err)
	}
	log.Printf("已发布控制命令: %s", topic)
	return seq, nil
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
