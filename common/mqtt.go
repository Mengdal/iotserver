package common

import (
	"fmt"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"log"
	"time"
)

// MqttConnector 接口定义
type MqttConnector interface {
	Connect(clientID string) error
	Disconnect()
	Publish(topic string, qos byte, payload []byte) error
	Subscribe(topic string, qos byte, callback mqtt.MessageHandler) error
	Unsubscribe(topic string) error
	IsConnected() bool
}

// MqttConnectorImpl 实现结构体
type MqttConnectorImpl struct {
	url           string
	port          string
	user          string
	password      string
	timeout       time.Duration
	keepAlive     int
	cleanSession  bool
	retryInterval time.Duration
	client        mqtt.Client
	retryCount    int
	topicHandlers map[string]mqtt.MessageHandler
}

// NewMqttConnector 创建新的MQTT连接器
func NewMqttConnector(url string, port string, timeoutSec int,
	user, password string, keepAlive int, cleanSession bool, retryIntervalSec int) *MqttConnectorImpl {

	return &MqttConnectorImpl{
		url:           url,
		port:          port,
		user:          user,
		password:      password,
		timeout:       time.Duration(timeoutSec) * time.Second,
		keepAlive:     keepAlive,
		cleanSession:  cleanSession,
		retryInterval: time.Duration(retryIntervalSec) * time.Second,
	}
}

// Connect 连接到MQTT代理
func (m *MqttConnectorImpl) Connect(clientID string) error {
	if clientID == "" {
		clientID = fmt.Sprintf("MqttClient_%d", time.Now().UnixNano())
	}

	opts := mqtt.NewClientOptions()
	broker := fmt.Sprintf("tcp://%s:%s", m.url, m.port)
	opts.AddBroker(broker)
	opts.SetClientID(clientID)
	opts.SetUsername(m.user)
	opts.SetPassword(m.password)
	opts.SetCleanSession(m.cleanSession)
	opts.SetKeepAlive(time.Duration(m.keepAlive) * time.Second)
	opts.SetAutoReconnect(true)
	opts.SetConnectTimeout(m.timeout)
	opts.SetConnectRetryInterval(m.retryInterval)

	opts.OnConnect = func(client mqtt.Client) {
		log.Println("MQTT连接成功")
		m.client = client

		// 订阅主题
		for topic, handler := range m.getSavedSubscriptions() {
			if token := client.Subscribe(topic, 0, handler); token.Wait() && token.Error() != nil {
				log.Printf("订阅失败 topic=%s: %v", topic, token.Error())
			} else {
				log.Printf("订阅成功 topic=%s", topic)
			}
		}
	}

	opts.OnConnectionLost = func(client mqtt.Client, err error) {
		log.Printf("MQTT连接丢失: %v", err)
	}

	opts.OnReconnecting = func(client mqtt.Client, options *mqtt.ClientOptions) {
		m.retryCount++
		log.Printf("MQTT开始重连... (尝试次数: %d)", m.retryCount)
	}

	m.client = mqtt.NewClient(opts)
	//首次连接失败时自动重试
	for {
		token := m.client.Connect()
		if token.Wait() && token.Error() != nil {
			log.Printf("初次连接失败，%v，将在 %v 后重试...", token.Error(), m.retryInterval)
			time.Sleep(m.retryInterval)
			continue
		}
		break // 首次连接成功后退出循环，后续由自动重连机制处理
	}

	return nil
}

// IsConnected 检查连接状态
func (m *MqttConnectorImpl) IsConnected() bool {
	return m.client != nil && m.client.IsConnected()
}

// Disconnect 断开连接
func (m *MqttConnectorImpl) Disconnect() {
	if m.IsConnected() {
		m.client.Disconnect(250)
		log.Println("MQTT已断开连接")
	}
}

// Publish 发布消息
func (m *MqttConnectorImpl) Publish(topic string, qos byte, payload []byte) error {
	if !m.IsConnected() {
		return fmt.Errorf("客户端未连接")
	}

	token := m.client.Publish(topic, qos, false, payload)
	token.Wait()
	return token.Error()
}

// Subscribe 注册订阅
func (m *MqttConnectorImpl) Subscribe(topic string, qos byte, callback mqtt.MessageHandler) error {
	if m.topicHandlers == nil {
		m.topicHandlers = make(map[string]mqtt.MessageHandler)
	}
	m.topicHandlers[topic] = callback

	if !m.IsConnected() {
		log.Printf("当前未连接，订阅缓存: %s", topic)
		return nil
	}

	token := m.client.Subscribe(topic, qos, callback)
	token.Wait()
	return token.Error()
}

// 获取已注册订阅
func (m *MqttConnectorImpl) getSavedSubscriptions() map[string]mqtt.MessageHandler {
	if m.topicHandlers == nil {
		return make(map[string]mqtt.MessageHandler)
	}
	return m.topicHandlers
}

// Unsubscribe 取消订阅
func (m *MqttConnectorImpl) Unsubscribe(topic string) error {
	if !m.IsConnected() {
		return fmt.Errorf("客户端未连接")
	}

	token := m.client.Unsubscribe(topic)
	token.Wait()
	return token.Error()
}
