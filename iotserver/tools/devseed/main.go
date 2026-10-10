// devseed 生成本地开发/联调用的历史数据：
//  1. 通过正式接口创建产品（含物模型属性 temp / humidity）
//  2. 创建一台设备
//  3. 按数据层的 key 规范写入最近 N 天的属性历史
//
// 用法（在 iotserver 目录下）：
//
//	go run ./tools/devseed -api http://127.0.0.1:3000 -db manifest/docker/db-data/leveldb-core-data/ -days 7
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"math"
	"math/rand"
	"net/http"
	"time"

	"iotserver/internal/dtos"
	"iotserver/internal/pkg/constants"
	"iotserver/internal/pkg/logger"
	"iotserver/internal/tools/datadb/ducklake"
	"iotserver/internal/tools/datadb/leveldb"
)

var (
	apiBase  = flag.String("api", "http://127.0.0.1:3000", "后端地址")
	dbPath   = flag.String("db", "manifest/docker/db-data/leveldb-core-data/", "leveldb 数据目录")
	user     = flag.String("user", "admin", "登录账号")
	pass     = flag.String("pass", "admin123", "登录密码")
	days     = flag.Int("days", 7, "生成最近多少天的数据")
	stepMin  = flag.Int("step", 5, "采样间隔（分钟）")
	clean    = flag.Bool("clean", false, "仅清理，不生成")
	skipDB   = flag.Bool("skip-db", false, "只通过接口建产品/设备，不写数据层（服务在跑时用）")
	deviceF  = flag.String("device", "", "离线模式：直接对指定 deviceId 写历史数据（不需要服务在跑）")
	backend  = flag.String("backend", "leveldb", "数据层类型：leveldb | ducklake")
	lakeCat  = flag.String("lake-catalog", "manifest/docker/db-data/ducklake/catalog.db", "ducklake catalog 路径")
	lakeData = flag.String("lake-data", "manifest/docker/db-data/ducklake/data", "ducklake Parquet 目录")
)

type client struct {
	base  string
	token string
}

func (c *client) do(method, path string, body interface{}) (map[string]interface{}, error) {
	var buf *bytes.Buffer
	if body != nil {
		b, _ := json.Marshal(body)
		buf = bytes.NewBuffer(b)
	} else {
		buf = bytes.NewBuffer(nil)
	}
	req, err := http.NewRequest(method, c.base+path, buf)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.token != "" {
		req.Header.Set("x-token", c.token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := ioutil.ReadAll(resp.Body)
	out := map[string]interface{}{}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("解析响应失败: %s", string(raw))
	}
	return out, nil
}

func (c *client) must(method, path string, body interface{}) map[string]interface{} {
	out, err := c.do(method, path, body)
	if err != nil {
		panic(err)
	}
	if ok, _ := out["success"].(bool); !ok {
		panic(fmt.Sprintf("%s %s 失败: %v", method, path, out["errorMsg"]))
	}
	return out
}

func main() {
	flag.Parse()
	rand.Seed(time.Now().UnixNano())

	lc := logger.NewClient("devseed", "info", "")

	if *deviceF != "" {
		// 离线模式：不访问接口，直接按 DataDBClient 的 key 规范写历史
		writeHistory(lc, *deviceF)
		return
	}

	c := &client{base: *apiBase}

	// 1) 登录
	out := c.must("POST", "/api/v1/auth/login", map[string]string{"username": *user, "password": *pass})
	result := out["result"].(map[string]interface{})
	c.token = result["token"].(string)
	fmt.Println("✅ 登录成功:", *user)

	// 2) 创建产品（自建品类，物模型稍后补充）
	productName := "数据中心联调产品"
	existsProductId := findProduct(c, productName)
	var productId string
	if existsProductId != "" {
		productId = existsProductId
		fmt.Println("ℹ️  复用已有产品:", productId)
	} else {
		out = c.must("POST", "/api/v1/product", map[string]interface{}{
			"name":                 productName,
			"category_template_id": "1",
			"protocol":             "MQTT",
			"node_type":            "direct",
			"net_type":             "wifi",
			"data_format":          "json",
			"factory":              "iotserver",
			"description":          "数据中心历史查询联调数据",
		})
		productId = findProduct(c, productName) // 接口不返回 id，回查列表
		fmt.Println("✅ 创建产品:", productId)
	}
	if productId == "" {
		panic("未拿到 productId")
	}

	// 3) 补充物模型属性（历史查询会校验产品物模型里存在该 code）
	props := []struct {
		code string
		name string
		unit string
	}{
		{"temp", "温度", "℃"},
		{"humidity", "湿度", "%RH"},
	}
	for _, p := range props {
		_, _ = c.do("POST", "/api/v1/thingmodel", map[string]interface{}{
			"product_id":       productId,
			"thing_model_type": "property",
			"name":             p.name,
			"code":             p.code,
			"description":      p.name,
			"property": map[string]interface{}{
				"access_model": "r",
				"require":      false,
				"type":         "float",
				"specs": map[string]interface{}{
					"unit": p.unit,
					"min":  0,
					"max":  100,
				},
			},
		})
		fmt.Printf("✅ 物模型属性: %s(%s)\n", p.name, p.code)
	}

	// 3.5) 发布产品（未发布不允许添加设备）
	if _, err := c.do("POST", "/api/v1/product-release/"+productId, nil); err != nil {
		fmt.Println("⚠️  发布产品请求异常:", err)
	} else {
		fmt.Println("✅ 产品已发布")
	}

	// 4) 创建设备
	deviceName := "数据中心联调设备"
	deviceId := findDevice(c, deviceName)
	if deviceId == "" {
		out = c.must("POST", "/api/v1/device", map[string]interface{}{
			"name":          deviceName,
			"product_id":    productId,
			"device_sn":     "SN-DATACENTER-001",
			"description":   "数据中心历史查询联调设备",
			"platform":      "LocalIot",
			"device_id":     "",
			"device_secret": "",
		})
		deviceId = findDevice(c, deviceName) // 接口不返回 id，回查列表
		fmt.Println("✅ 创建设备:", deviceId)
	} else {
		fmt.Println("ℹ️  复用已有设备:", deviceId)
	}
	if deviceId == "" {
		panic("未拿到 deviceId")
	}

	if *clean {
		fmt.Println("已跳过数据生成（-clean）")
		return
	}
	if *skipDB {
		fmt.Println("已跳过数据层写入（-skip-db），请停掉服务后重新执行以写入历史数据")
		return
	}

	// 5) 写入属性历史
	total := writeHistory(lc, deviceId)

	fmt.Printf("\n完成：设备 %s（产品 %s），共写入 %d 条属性历史，覆盖最近 %d 天，采样间隔 %d 分钟\n",
		deviceId, productId, total, *days, *stepMin)
	fmt.Printf("查询示例：GET /api/v1/device/%s/thing-model/history-property?code=temp&page=1&pageSize=20\n", deviceId)
}

// writeHistory 按 DataDBClient 的 key 规范写入设备属性历史，返回写入条数
func writeHistory(lc logger.LoggingClient, deviceId string) int {
	if *backend == "ducklake" {
		return writeHistoryDuckLake(lc, deviceId)
	}
	cli, err := leveldb.NewClient(dtos.Configuration{DataSource: *dbPath}, lc)
	if err != nil {
		panic(err)
	}
	defer cli.CloseSession()

	codes := []string{"temp", "humidity"}
	now := time.Now()
	total := 0
	for _, code := range codes {
		kvs := map[string]interface{}{}
		for t := now.AddDate(0, 0, -*days); t.Before(now); t = t.Add(time.Duration(*stepMin) * time.Minute) {
			ts := t.UnixNano() / int64(time.Millisecond)
			var v float64
			if code == "temp" {
				v = 22 + 6*math.Sin(float64(t.Hour())/24*2*math.Pi) + rand.Float64()
			} else {
				v = 45 + 15*math.Sin(float64(t.Hour())/24*2*math.Pi) + rand.Float64()*3
			}
			data := dtos.ReportData{Value: math.Round(v*10) / 10, Time: ts}
			b, _ := data.Marshal()
			key := deviceId + "-" + constants.Property + "-" + code + "-" + fmt.Sprintf("%d", ts)
			kvs[key] = b
			total++
		}
		if err := cli.Insert(nil, "", kvs); err != nil {
			panic(err)
		}
		fmt.Printf("✅ 写入 %s 历史 %d 条\n", code, len(kvs))
	}
	return total
}

func findProduct(c *client, name string) string {
	out, err := c.do("GET", "/api/v1/products?page=1&pageSize=100", nil)
	if err != nil {
		return ""
	}
	list, _ := out["result"].(map[string]interface{})
	if list == nil {
		return ""
	}
	items, _ := list["list"].([]interface{})
	for _, it := range items {
		m, _ := it.(map[string]interface{})
		if m["name"] == name {
			id, _ := m["id"].(string)
			return id
		}
	}
	return ""
}

func findDevice(c *client, name string) string {
	out, err := c.do("GET", "/api/v1/devices?page=1&pageSize=100", nil)
	if err != nil {
		return ""
	}
	list, _ := out["result"].(map[string]interface{})
	if list == nil {
		return ""
	}
	items, _ := list["list"].([]interface{})
	for _, it := range items {
		m, _ := it.(map[string]interface{})
		if m["name"] == name {
			id, _ := m["id"].(string)
			return id
		}
	}
	return ""
}

// writeHistoryDuckLake 通过 DuckLake 数据层写入属性历史
func writeHistoryDuckLake(lc logger.LoggingClient, deviceId string) int {
	cli, err := ducklake.NewClient(dtos.Configuration{DataSource: *lakeCat, Dsn: *lakeData}, lc)
	if err != nil {
		panic(err)
	}
	defer cli.CloseSession()

	codes := []string{"temp", "humidity"}
	now := time.Now()
	total := 0
	for _, code := range codes {
		for t := now.AddDate(0, 0, -*days); t.Before(now); t = t.Add(time.Duration(*stepMin) * time.Minute) {
			ts := t.UnixNano() / int64(time.Millisecond)
			var v float64
			if code == "temp" {
				v = 22 + 6*math.Sin(float64(t.Hour())/24*2*math.Pi) + rand.Float64()
			} else {
				v = 45 + 15*math.Sin(float64(t.Hour())/24*2*math.Pi) + rand.Float64()*3
			}
			data := map[string]interface{}{code: dtos.ReportData{Value: math.Round(v*10) / 10, Time: ts}}
			if err := cli.Insert(context.Background(), deviceId, data); err != nil {
				panic(err)
			}
			total++
		}
		fmt.Printf("✅ 写入 %s 历史到 DuckLake 完成\n", code)
	}

	// 文本型属性：用于验证「计数/首值/末值」，每 30 分钟一个状态
	modes := []string{"auto", "manual", "off", "idle"}
	step := 0
	for t := now.AddDate(0, 0, -*days); t.Before(now); t = t.Add(30 * time.Minute) {
		ts := t.UnixNano() / int64(time.Millisecond)
		data := map[string]interface{}{"mode": dtos.ReportData{Value: modes[step%len(modes)], Time: ts}}
		if err := cli.Insert(context.Background(), deviceId, data); err != nil {
			panic(err)
		}
		step++
		total++
	}
	fmt.Printf("✅ 写入 mode 文本历史到 DuckLake 完成（%d 条）\n", step)
	return total
}
