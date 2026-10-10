// mqttpub 向平台消息总线发一条消息，用于本地联调数据面（设备上报模拟）。
//
// 用法：
//
//	go run ./tools/mqttpub -broker tcp://127.0.0.1:58090 -topic eventbus/in \
//	  -device 81501053 -type PROPERTY_REPORT -payload '{"temp":{"value":35.5}}'
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

func main() {
	broker := flag.String("broker", "tcp://127.0.0.1:58090", "MQTT broker 地址")
	topic := flag.String("topic", "eventbus/in", "发布主题")
	device := flag.String("device", "", "deviceId")
	msgType := flag.String("type", "PROPERTY_REPORT", "messageType")
	payload := flag.String("payload", "{}", "data 字段内容(JSON)")
	flag.Parse()

	var data map[string]interface{}
	if err := json.Unmarshal([]byte(*payload), &data); err != nil {
		fmt.Println("payload 不是合法 JSON:", err)
		os.Exit(1)
	}
	msg := map[string]interface{}{
		"deviceId":    *device,
		"messageType": *msgType,
		"time":        time.Now().UnixMilli(),
		"data":        data,
	}
	body, _ := json.Marshal(msg)

	opts := mqtt.NewClientOptions().AddBroker(*broker).
		SetClientID(fmt.Sprintf("mqttpub-%d", time.Now().UnixNano()))
	client := mqtt.NewClient(opts)
	if token := client.Connect(); !token.WaitTimeout(5*time.Second) || token.Error() != nil {
		fmt.Println("连接 broker 失败:", token.Error())
		os.Exit(1)
	}
	defer client.Disconnect(200)

	if token := client.Publish(*topic, 1, false, body); !token.WaitTimeout(5*time.Second) || token.Error() != nil {
		fmt.Println("发布失败:", token.Error())
		os.Exit(1)
	}
	fmt.Printf("已发布到 %s: %s\n", *topic, string(body))
}
