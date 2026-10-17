package mqttclient

import (
	"context"
	"crypto/tls"
	"fmt"
	"iotserver/internal/dtos"
	"iotserver/internal/pkg/errort"
	"iotserver/internal/pkg/logger"
	"iotserver/internal/pkg/middleware"
	"log"
	"os"
	"runtime/debug"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	pkgerr "github.com/pkg/errors"
	"go.uber.org/atomic"
)

var (
	cache sync.Map
)

type message struct {
	ctx     context.Context
	isSync  bool
	topic   string
	payload []byte
}

type mqttClient struct {
	ctx               context.Context
	cancel            context.CancelFunc
	lc                logger.LoggingClient
	clientId          string
	client            mqtt.Client
	connectHandler    dtos.ConnectHandler
	disconnectHandler dtos.CallbackHandler
	messageChannel    chan message
	status            *atomic.Bool
	pubTopic          string
	// consumeCallback 收到消息时的回调。订阅可能发生在连接建立之后
	// （见 Subscribe），因此必须存下来，不能只靠构造时的闭包。
	consumeCallback func(mqtt.Client, mqtt.Message)
	// topicsMu / topics 当前**应当**订阅的主题集合。
	// 之所以要维护集合而不是只依赖构造参数：重连时 OnConnect 会重新订阅，
	// 若只重订构造时那批，运行期新增的订阅会在一次断线后静默丢失。
	topicsMu sync.Mutex
	topics   map[string]struct{}
}

func NewMQTTClient(req dtos.NewMQTTClient, lc logger.LoggingClient,
	consumeCallback func(mqtt.Client, mqtt.Message), connF dtos.ConnectHandler, disConnF dtos.CallbackHandler) (MQTTClient, error) {
	mqttCli, exist := cache.Load(req.ClientId)
	if exist {
		return mqttCli.(MQTTClient), nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	var mc = &mqttClient{
		ctx:               ctx,
		cancel:            cancel,
		lc:                lc,
		clientId:          req.ClientId,
		connectHandler:    connF,
		disconnectHandler: disConnF,
		status:            atomic.NewBool(false),
		messageChannel:    make(chan message, 200),
		consumeCallback:   consumeCallback,
		topics:            make(map[string]struct{}, len(req.SubTopics)),
	}
	for _, topic := range req.SubTopics {
		if topic != "" {
			mc.topics[topic] = struct{}{}
		}
	}
	opts := mqtt.NewClientOptions().SetClientID(req.ClientId).SetUsername(req.Username).SetPassword(req.Password)
	opts = opts.SetAutoReconnect(true).SetCleanSession(true).SetKeepAlive(5 * time.Second).SetMaxReconnectInterval(10 * time.Second).
		SetConnectRetry(true).SetConnectRetryInterval(time.Second)
	w := os.Stdout
	opts = opts.SetTLSConfig(&tls.Config{
		KeyLogWriter: w,
		//InsecureSkipVerify: true,	//如果本地出现证书过期，可以打开这个选项
	})

	// add mqttclient internal log
	mqtt.ERROR = log.New(os.Stdout, "", log.LstdFlags)

	opts.AddBroker(req.Broker)
	opts.SetOnConnectHandler(func(client mqtt.Client) {
		defer func() {
			if err := recover(); err != nil {
				err = pkgerr.WithStack(pkgerr.New(fmt.Sprintf("internel err: %v", err)))
				mc.lc.Errorf("internal error: %s,%s", err, string(debug.Stack()))
			}
		}()

		// 订阅「应当订阅的全部主题」，而不是只重订构造时那批：运行期可能通过
		// Subscribe 追加过（例如参考系统接入按网关逐个订阅），断线重连后必须一并恢复，
		// 否则那些订阅会在一次抖动之后静默失效。
		for _, topic := range mc.snapshotTopics() {
			token := client.Subscribe(topic, 1, consumeCallback)
			token.Wait()
			if err := token.Error(); err != nil {
				mc.lc.Errorf("failed to consume message: %s", err.Error())
				continue
			}
			mc.lc.Infof("Subscribe success,topic: %s", topic)
		}

		// business process
		mc.setStatus(true)
		ctx := middleware.WithCorrelationId(context.Background())
		lc.Info("gateway online, mqttclient connected!", middleware.FromContext(ctx))

		// callback connectHandler
		if mc.connectHandler != nil {
			mc.connectHandler(ctx)
		}
	}).SetConnectionLostHandler(func(client mqtt.Client, err error) {
		mc.setStatus(false)
		ctx := middleware.WithCorrelationId(context.Background())

		err = pkgerr.Wrap(err, "gateway offline, mqttclient disconnect")
		err = errort.NewCommonErr(errort.MqttConnFail, err)

		mc.lc.Errorf("%v, %v", err.Error(), middleware.FromContext(ctx))
		if mc.disconnectHandler != nil {
			mc.disconnectHandler(ctx, dtos.CallbackMessage{
				Error: err,
			})
		}
	})
	opts.SetDefaultPublishHandler(consumeCallback)

	var err error
	mc.client = mqtt.NewClient(opts)
	token := mc.client.Connect()

	if ok := token.WaitTimeout(time.Second * 10); !ok {
		err = errort.NewCommonErr(errort.MqttConnFail, pkgerr.New("mqttclient client connect timeout"))
	}

	if err1 := token.Error(); err1 != nil {
		err = fmt.Errorf("%v, %v", err1, err)
		err = errort.NewCommonErr(errort.MqttConnFail, err)
	}
	mc.pubTopic = req.PubTopic
	go mc.run()
	cache.Store(req.ClientId, mc)
	return mc, err
}

func (c *mqttClient) Close() {
	c.cancel()
	c.client.Disconnect(3000)
	cache.Delete(c.clientId)
}

func (c *mqttClient) RegisterConnectCallback(cb dtos.ConnectHandler) {
	if c.connectHandler == nil {
		c.connectHandler = cb
	}
}

func (c *mqttClient) RegisterDisconnectCallback(cb dtos.CallbackHandler) {
	if c.disconnectHandler == nil {
		c.disconnectHandler = cb
	}
}

func (c *mqttClient) setStatus(status bool) {
	c.status.Store(status)
}

func (c *mqttClient) getStatus() bool {
	return c.status.Load()
}

func (c *mqttClient) publish(msg message) (err error) {
	token := c.client.Publish(msg.topic, 1, false, msg.payload)
	if !msg.isSync {
		return
	}

	go func() {
		// error handing https://github.com/eclipse/paho.mqtt.golang#error-handling
		_ = token.WaitTimeout(3 * time.Second)
		if token.Error() != nil {
			c.lc.Errorf("failed to publish %s, err: %v, %v", msg.topic, token.Error(), middleware.FromContext(msg.ctx))
			return
		}
		c.lc.Infof("sync publish to %s message success, %v", msg.topic, middleware.FromContext(msg.ctx))
	}()
	return
}

func (c *mqttClient) run() {
	for {
		select {
		case <-c.ctx.Done():
			close(c.messageChannel)
			c.lc.Error("mqttclient client is closed")
			return
		case msg := <-c.messageChannel:
			if c.getStatus() {
				// 消息发送
				if err := c.publish(msg); err != nil {
					c.lc.Error("failed to publish message, topic: %v, %v", msg.topic, middleware.FromContext(msg.ctx))
				}
			} else {
				// 消息丢弃
				c.lc.Warnf("failed to publish, mqttclient client is reconnecting, message must be lost, topic: %v, %v", msg.topic, middleware.FromContext(msg.ctx))
				if len(c.messageChannel) < cap(c.messageChannel) {
					// 等待 100ms 缓冲
					time.After(100 * time.Millisecond)
				}
			}
		}
	}
}

// 异步发送，实际上是发送到channel中排队
func (c *mqttClient) AsyncPublish(ctx context.Context, topic string, payload []byte, isSync bool) {
	//c.messageChannel <- message{
	//	ctx:     ctx,
	//	isSync:  isSync,
	//	topic:   topic,
	//	payload: payload,
	//}
	if c.getStatus() {
		_ = c.publish(message{
			ctx:     ctx,
			isSync:  isSync,
			topic:   topic,
			payload: payload,
		})
	} else {
		c.lc.Warnf("failed to publish, mqttclient client is reconnecting, message must be lost, topic: %v, %v", topic, middleware.FromContext(ctx))
	}

}

func (c *mqttClient) GetConnectStatus() bool {
	return c.getStatus()
}

// snapshotTopics 取当前应当订阅的主题快照
func (c *mqttClient) snapshotTopics() []string {
	c.topicsMu.Lock()
	defer c.topicsMu.Unlock()
	out := make([]string, 0, len(c.topics))
	for t := range c.topics {
		out = append(out, t)
	}
	return out
}

// Subscribe 追加订阅主题。
//
// 传入的主题会记入内部集合，因此**断线重连后仍会重新订阅**（见 OnConnect）；
// 未连接时只登记不实际订阅，等连上时由 OnConnect 统一订阅。
func (c *mqttClient) Subscribe(topics ...string) error {
	if len(topics) == 0 {
		return nil
	}
	added := make([]string, 0, len(topics))
	c.topicsMu.Lock()
	for _, topic := range topics {
		if topic == "" {
			continue
		}
		if _, ok := c.topics[topic]; ok {
			continue
		}
		c.topics[topic] = struct{}{}
		added = append(added, topic)
	}
	c.topicsMu.Unlock()
	if len(added) == 0 {
		return nil
	}
	if !c.getStatus() {
		c.lc.Warnf("mqttclient 未连接，已登记 %d 个主题待连接后订阅", len(added))
		return nil
	}
	for _, topic := range added {
		token := c.client.Subscribe(topic, 1, c.consumeCallback)
		token.Wait()
		if err := token.Error(); err != nil {
			c.lc.Errorf("failed to subscribe %s: %s", topic, err.Error())
			return err
		}
		c.lc.Infof("Subscribe success,topic: %s", topic)
	}
	return nil
}

// Unsubscribe 取消订阅主题，并从内部集合移除（避免重连时又被订回来）。
func (c *mqttClient) Unsubscribe(topics ...string) error {
	if len(topics) == 0 {
		return nil
	}
	removed := make([]string, 0, len(topics))
	c.topicsMu.Lock()
	for _, topic := range topics {
		if _, ok := c.topics[topic]; !ok {
			continue
		}
		delete(c.topics, topic)
		removed = append(removed, topic)
	}
	c.topicsMu.Unlock()
	if len(removed) == 0 || !c.getStatus() {
		return nil
	}
	for _, topic := range removed {
		token := c.client.Unsubscribe(topic)
		token.Wait()
		if err := token.Error(); err != nil {
			c.lc.Errorf("failed to unsubscribe %s: %s", topic, err.Error())
			return err
		}
	}
	return nil
}
