// mqttsub 订阅 MQTT 主题并打印收到的消息，用于验证数据推送链路。
//
// 用法：
//
//	go run ./tools/mqttsub -broker tcp://127.0.0.1:58090 -topic datatest/push -seconds 60
package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

func main() {
	broker := flag.String("broker", "tcp://127.0.0.1:58090", "MQTT broker 地址")
	topic := flag.String("topic", "datatest/push", "订阅主题")
	seconds := flag.Int("seconds", 60, "运行时长(秒)")
	out := flag.String("out", "", "收到消息追加写入的文件")
	flag.Parse()

	opts := mqtt.NewClientOptions().AddBroker(*broker).
		SetClientID(fmt.Sprintf("mqttsub-%d", time.Now().UnixNano()))
	client := mqtt.NewClient(opts)
	if token := client.Connect(); !token.WaitTimeout(5*time.Second) || token.Error() != nil {
		fmt.Println("连接 broker 失败:", token.Error())
		os.Exit(1)
	}
	defer client.Disconnect(200)

	got := make(chan string, 64)
	if token := client.Subscribe(*topic, 1, func(_ mqtt.Client, m mqtt.Message) {
		got <- string(m.Payload())
	}); !token.WaitTimeout(5*time.Second) || token.Error() != nil {
		fmt.Println("订阅失败:", token.Error())
		os.Exit(1)
	}
	fmt.Printf("已订阅 %s（%d 秒）\n", *topic, *seconds)

	deadline := time.After(time.Duration(*seconds) * time.Second)
	count := 0
	for {
		select {
		case msg := <-got:
			count++
			fmt.Printf("[%d] %s\n", count, msg)
			if *out != "" {
				if f, err := os.OpenFile(*out, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644); err == nil {
					_, _ = f.WriteString(msg + "\n")
					_ = f.Close()
				}
			}
		case <-deadline:
			fmt.Printf("结束，共收到 %d 条\n", count)
			return
		}
	}
}
