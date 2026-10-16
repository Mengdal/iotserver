// telemgen 持续向平台注入网关上行测试数据，用于长时间观察时序库、规则引擎与看板。
//
// 与 devseed 的区别：devseed 是一次性「造历史」（直接写数据层），
// telemgen 是常驻服务，走**真实的网关上行走廊**（MQTT iot/up/<sn>/data），
// 因此落库、租户归属、推 eKuiper 告警链路都会被真实触发。
//
// 用法（本机）：
//
//	go run ./tools/telemgen -api http://127.0.0.1:3000 -broker tcp://127.0.0.1:58090 \
//	  -devices 5 -interval 5s
//
// 首次运行会自动建好：产品（含物模型 temp/humidity/mode）、N 台设备、一个网关，
// 之后每秒按间隔持续上报，按 Ctrl+C 退出。
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

type client struct {
	base   string
	token  string
	tenant string
	http   *http.Client
}

func (c *client) call(method, path string, body interface{}) (interface{}, error) {
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reader = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, c.base+path, reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.token != "" {
		req.Header.Set("x-token", c.token)
	}
	if c.tenant != "" {
		req.Header.Set("x-tenant-id", c.tenant)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	out := map[string]interface{}{}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("解析响应失败(status=%d): %s", resp.StatusCode, string(raw))
	}
	if ok, _ := out["success"].(bool); !ok {
		return nil, fmt.Errorf("%s %s 失败: %v", method, path, out["errorMsg"])
	}
	return out, nil
}

// result 取响应里的 result 节点
func result(out interface{}) map[string]interface{} {
	if m, ok := out.(map[string]interface{}); ok {
		if r, ok := m["result"].(map[string]interface{}); ok {
			return r
		}
		return m
	}
	return map[string]interface{}{}
}

func (c *client) list(path string) []map[string]interface{} {
	out, err := c.call("GET", path, nil)
	if err != nil {
		return nil
	}
	items, _ := result(out)["list"].([]interface{})
	res := make([]map[string]interface{}, 0, len(items))
	for _, it := range items {
		if m, ok := it.(map[string]interface{}); ok {
			res = append(res, m)
		}
	}
	return res
}

// findBy 在列表里按字段匹配，返回 id
func (c *client) findBy(path, field, value string) string {
	for _, it := range c.list(path) {
		if s, _ := it[field].(string); s == value {
			id, _ := it["id"].(string)
			return id
		}
	}
	return ""
}

func str(v interface{}) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

// ---- 设备规格 ----

type deviceRef struct {
	ID string
	SN string
}

const (
	productListPath = "/api/v1/products?page=1&pageSize=500"
	deviceListPath  = "/api/v1/devices?page=1&pageSize=1000"
	gatewayListPath = "/api/v1/gateways?page=1&pageSize=500"
)

// ensureProduct 建产品（已存在则复用）并补齐物模型属性，最后发布
func ensureProduct(c *client, name string) string {
	productID := c.findBy(productListPath, "name", name)
	if productID == "" {
		if _, err := c.call("POST", "/api/v1/product", map[string]interface{}{
			"name":                 name,
			"category_template_id": "1",
			"protocol":             "MQTT",
			"node_type":            "direct",
			"net_type":             "wifi",
			"data_format":          "json",
			"factory":              "iotserver",
			"description":          "telemgen 持续上报测试产品",
		}); err != nil {
			fmt.Printf("⚠️  创建产品失败：%v\n", err)
		}
		productID = c.findBy(productListPath, "name", name)
	}
	if productID == "" {
		return ""
	}
	fmt.Printf("✅ 产品：%s (%s)\n", name, productID)

	// 物模型属性：数值型 + 文本型（文本用于验证 first/last 与计数）
	props := []struct {
		code, name, typ string
		specs           map[string]interface{}
	}{
		{"temp", "温度", "float", map[string]interface{}{"unit": "℃", "min": -40, "max": 125}},
		{"humidity", "湿度", "float", map[string]interface{}{"unit": "%RH", "min": 0, "max": 100}},
		{"mode", "运行模式", "text", map[string]interface{}{}},
	}
	for _, p := range props {
		if _, err := c.call("POST", "/api/v1/thingmodel", map[string]interface{}{
			"product_id":       productID,
			"thing_model_type": "property",
			"name":             p.name,
			"code":             p.code,
			"description":      p.name,
			"property": map[string]interface{}{
				"access_model": "r",
				"require":      false,
				"type":         p.typ,
				"specs":        p.specs,
			},
		}); err != nil {
			// 重复添加会报错，属正常
			fmt.Printf("   ℹ️  物模型 %s 已存在或添加失败（可忽略）：%v\n", p.code, err)
		}
	}

	if _, err := c.call("POST", "/api/v1/product-release/"+productID, nil); err != nil {
		fmt.Printf("   ℹ️  产品发布（已发布可忽略）：%v\n", err)
	}
	return productID
}

// ensureDevices 建 N 台设备，返回 (id, sn)
func ensureDevices(c *client, productID, prefix string, n int) []deviceRef {
	out := make([]deviceRef, 0, n)
	for i := 1; i <= n; i++ {
		name := fmt.Sprintf("%s-%d", prefix, i)
		// deviceKey 走 ASCII，避免中文进入设备编码
		sn := fmt.Sprintf("TELEMGEN-DEV-%d", i)
		id := c.findBy(deviceListPath, "name", name)
		if id == "" {
			if _, err := c.call("POST", "/api/v1/device", map[string]interface{}{
				"name":        name,
				"product_id":  productID,
				"device_sn":   sn,
				"description": "telemgen 持续上报测试设备",
				"platform":    "LocalIot",
			}); err != nil {
				fmt.Printf("⚠️  创建设备 %s 失败：%v\n", name, err)
			}
			id = c.findBy(deviceListPath, "name", name)
		}
		if id == "" {
			continue
		}
		out = append(out, deviceRef{ID: id, SN: sn})
	}
	fmt.Printf("✅ 设备：%d 台可用（前缀 %s）\n", len(out), prefix)
	return out
}

// ensureGateway 建网关（上行主题里的 sn 必须是已注册网关）
func ensureGateway(c *client, sn string) bool {
	if id := c.findBy(gatewayListPath, "sn", sn); id != "" {
		fmt.Printf("✅ 网关（复用）：%s (%s)\n", sn, id)
		return true
	}
	if _, err := c.call("POST", "/api/v1/gateway", map[string]interface{}{
		"name":   "telemgen 测试网关",
		"sn":     sn,
		"vendor": "iotserver",
		"model":  "TELEMGEN-GW",
		"remark": "telemgen 持续上报用",
	}); err != nil {
		fmt.Printf("⚠️  创建网关失败：%v\n", err)
	}
	ok := c.findBy(gatewayListPath, "sn", sn) != ""
	if ok {
		fmt.Printf("✅ 网关：%s\n", sn)
	}
	return ok
}

func main() {
	apiBase := flag.String("api", "http://127.0.0.1:3000", "平台 API 地址")
	broker := flag.String("broker", "tcp://127.0.0.1:58090", "MQTT broker 地址")
	upPrefix := flag.String("up-prefix", "iot/up/", "网关上行主题前缀")
	user := flag.String("user", "admin", "登录账号")
	pass := flag.String("pass", "admin123", "登录密码")
	tenantHeader := flag.String("tenant", "", "可选：x-tenant-id（平台管理员代管租户时用）")
	productName := flag.String("product", "监控联调产品", "产品名称")
	devicePrefix := flag.String("device-prefix", "监控设备", "设备名称前缀")
	gatewaySN := flag.String("gateway-sn", "TELEMGEN-GW1", "网关 SN（同时是上行主题段）")
	devices := flag.Int("devices", 5, "设备数量")
	interval := flag.Duration("interval", 5*time.Second, "上报间隔")
	once := flag.Bool("once", false, "只上报一轮后退出（冒烟测试用）")
	flag.Parse()

	fmt.Printf("telemgen 启动: api=%s broker=%s 设备数=%d 间隔=%s\n", *apiBase, *broker, *devices, *interval)

	c := &client{
		base:   *apiBase,
		tenant: *tenantHeader,
		http:   &http.Client{Timeout: 15 * time.Second},
	}

	// 1) 登录
	login, err := c.call("POST", "/api/v1/auth/login", map[string]string{"username": *user, "password": *pass})
	if err != nil {
		fmt.Println("❌ 登录失败:", err)
		os.Exit(1)
	}
	token := str(result(login)["token"])
	if token == "" {
		fmt.Println("❌ 登录未返回 token")
		os.Exit(1)
	}
	c.token = token
	fmt.Println("✅ 登录成功:", *user)

	// 2) 产品 + 物模型 + 设备 + 网关
	productID := ensureProduct(c, *productName)
	if productID == "" {
		fmt.Println("❌ 未拿到 productId，退出")
		os.Exit(1)
	}
	devs := ensureDevices(c, productID, *devicePrefix, *devices)
	if len(devs) == 0 {
		fmt.Println("❌ 没有可用设备，退出")
		os.Exit(1)
	}
	if !ensureGateway(c, *gatewaySN) {
		fmt.Println("❌ 网关未就绪，退出")
		os.Exit(1)
	}

	// 3) 连 MQTT
	opts := mqtt.NewClientOptions().AddBroker(*broker).
		SetClientID(fmt.Sprintf("telemgen-%d", time.Now().UnixNano())).
		SetAutoReconnect(true).
		SetConnectRetry(true).
		SetConnectRetryInterval(5 * time.Second)
	mc := mqtt.NewClient(opts)
	if tk := mc.Connect(); !tk.WaitTimeout(10*time.Second) || tk.Error() != nil {
		fmt.Println("❌ 连接 broker 失败:", tk.Error())
		os.Exit(1)
	}
	defer mc.Disconnect(300)
	topic := *upPrefix + *gatewaySN + "/data"
	fmt.Printf("✅ 已连 broker，上行主题 %s\n\n开始持续上报（Ctrl+C 退出）...\n", topic)

	// 4) 退出信号
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	modes := []string{"auto", "manual", "off", "idle"}
	seq := time.Now().Unix()
	published, failed := 0, 0
	ticker := time.NewTicker(*interval)
	defer ticker.Stop()

	for {
		now := time.Now()
		seq++
		records := make([]map[string]interface{}, 0, len(devs))
		// 以「当天小时」为相位，产生日内正弦波形 + 噪声，便于看板上看到趋势
		phase := float64(now.Hour())*3600 + float64(now.Minute())*60 + float64(now.Second())
		for i, d := range devs {
			// 每台设备给一点相位偏移，曲线不会完全重合
			off := float64(i) * 0.4
			temp := 22 + 6*math.Sin((phase/86400)*2*math.Pi+off) + rand.Float64()*0.6
			hum := 50 + 18*math.Sin((phase/86400)*2*math.Pi+off+1.2) + rand.Float64()*2
			records = append(records, map[string]interface{}{
				"ts":        now.UnixMilli(),
				"deviceKey": d.SN,
				"properties": map[string]interface{}{
					"temp":     map[string]interface{}{"value": math.Round(temp*10) / 10, "time": now.UnixMilli()},
					"humidity": map[string]interface{}{"value": math.Round(hum*10) / 10, "time": now.UnixMilli()},
					"mode":     map[string]interface{}{"value": modes[int(phase/60)%len(modes)], "time": now.UnixMilli()},
				},
			})
		}
		body, _ := json.Marshal(map[string]interface{}{
			"seq":     seq,
			"ts":      now.UnixMilli(),
			"records": records,
		})

		tk := mc.Publish(topic, 1, false, body)
		if !tk.WaitTimeout(5*time.Second) || tk.Error() != nil {
			failed++
			fmt.Printf("⚠️  上报失败：%v\n", tk.Error())
		} else {
			published++
			if published%12 == 1 { // 每 12 轮打一次心跳，避免刷屏
				fmt.Printf("↑ 已上报 %d 轮（失败 %d），设备 %d 台，本轮 %d 条测点\n",
					published, failed, len(devs), len(records)*3)
			}
		}

		if *once {
			fmt.Println("（-once）完成一轮，退出")
			return
		}
		select {
		case <-stop:
			fmt.Printf("\n收到退出信号，共上报 %d 轮（失败 %d）\n", published, failed)
			return
		case <-ticker.C:
		}
	}
}
