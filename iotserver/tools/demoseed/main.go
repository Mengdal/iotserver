// demoseed 一键生成「多租户 + 多工厂/产线 + 网关 + 设备 + 历史数据 + 告警」演示环境。
//
// 设计原则：
//  1. 只走正式 HTTP 接口创建资源（租户/组织/网关/产品/设备/告警规则），不直连业务库；
//  2. 内置方案（品类/物模型/告警阈值）直接复用 internal/pkg/seed 的 LoadSolutions()，
//     与启动迁移写入 category_template(sol_<key>) / solution 的数据同源；
//  3. 历史数据按 devseed 的方式直写 DuckLake（带历史时间戳）；
//  4. 幂等：先查后建，创建失败（如重名/409）时打印警告并复用已有对象。
//
// 用法（在 iotserver 目录下）：
//
//	go run ./tools/demoseed -url http://127.0.0.1:3000 -tenants 2 -devices 2 -days 30 -reset
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"iotserver/internal/dtos"
	"iotserver/internal/pkg/constants"
	"iotserver/internal/pkg/logger"
	"iotserver/internal/pkg/seed"
	"iotserver/internal/tools/datadb/ducklake"
)

var (
	apiURL   = flag.String("url", "http://127.0.0.1:3000", "后端地址")
	tenants  = flag.Int("tenants", 2, "租户数量（演示租户A/B/...，code demo_a/demo_b/...）")
	devices  = flag.Int("devices", 2, "每个模板（产品）下的设备数量")
	days     = flag.Int("days", 30, "历史数据天数")
	stepMin  = flag.Int("step", 5, "历史数据采样间隔（分钟）")
	doReset  = flag.Bool("reset", false, "先清理本工具创建的演示数据（失败仅告警并继续）")
	user     = flag.String("user", "admin", "平台管理员账号")
	pass     = flag.String("pass", "admin123", "平台管理员密码")
	skipHist = flag.Bool("skip-history", false, "不写历史数据（后端在跑、DuckLake 被占用时用）")
	lakeCat  = flag.String("lake-catalog", "manifest/docker/db-data/ducklake/catalog.db", "ducklake catalog 路径")
	lakeData = flag.String("lake-data", "manifest/docker/db-data/ducklake/data", "ducklake Parquet 目录")
	lakeOver = flag.Bool("override-data-path", false, "ducklake attach 时强制使用 -lake-data（catalog 由容器路径创建时需要）")
)

/** *********************** 资源模型 *********************** **/

// numProp 产品物模型里的数值属性（用于造历史数据）
type numProp struct {
	Code string
	Min  float64
	Max  float64
	Int  bool
}

// tmpl 内置工业模板在本工具里的视图
type tmpl struct {
	Key            string
	Name           string
	CategoryKey    string // sol_<key>，即 category_template_id
	ThingModelJSON string
	Numeric        []numProp
	Alarms         []seed.SolutionAlarm // 来自 solution.AlarmRulesJson
}

// devRef 已创建/复用的设备
type devRef struct {
	Id        string
	Name      string
	ProductId string
	Tpl       *tmpl
}

type summary struct {
	Tenants  int
	Gateways int
	Products int
	Devices  int
	Rules    int
	Points   int
	Reused   int
}

/** *********************** HTTP 客户端 *********************** **/

type client struct {
	base   string
	token  string
	tenant string // 平台管理员代管租户：x-tenant-id
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
	resp, err := http.DefaultClient.Do(req)
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
		msg, _ := out["errorMsg"].(string)
		if strings.TrimSpace(msg) == "" {
			msg = fmt.Sprintf("HTTP %d", resp.StatusCode)
		}
		return out["result"], fmt.Errorf("%s", msg)
	}
	return out["result"], nil
}

// list 读取分页接口的 result.list
func (c *client) list(path string) ([]map[string]interface{}, error) {
	res, err := c.call("GET", path, nil)
	if err != nil {
		return nil, err
	}
	page, _ := res.(map[string]interface{})
	items, _ := page["list"].([]interface{})
	out := make([]map[string]interface{}, 0, len(items))
	for _, it := range items {
		if m, ok := it.(map[string]interface{}); ok {
			out = append(out, m)
		}
	}
	return out, nil
}

// index 拉一次列表并建立 key -> id 索引
func (c *client) index(path, key string) map[string]string {
	out := map[string]string{}
	items, err := c.list(path)
	if err != nil {
		return out
	}
	for _, it := range items {
		if k := str(it[key]); k != "" {
			out[k] = str(it["id"])
		}
	}
	return out
}

// ensure 先查索引，缺失则 create()，创建后回查列表补齐 id。
// 返回 (id, 本次是否新建成功)。创建失败（如重名/409）只告警，能查到就复用。
func (c *client) ensure(idx map[string]string, listPath, keyField, keyValue, what string, create func() error) (string, bool) {
	if id := idx[keyValue]; id != "" {
		fmt.Printf("   ℹ️  复用已有%s：%s (%s)\n", what, keyValue, id)
		return id, false
	}
	created := false
	if err := create(); err != nil {
		fmt.Printf("   ⚠️  创建%s %s 失败：%v（尝试复用已有对象）\n", what, keyValue, err)
	} else {
		created = true
	}
	for k, v := range c.index(listPath, keyField) {
		idx[k] = v
	}
	if id := idx[keyValue]; id != "" {
		if created {
			fmt.Printf("   ✅ 创建%s：%s (%s)\n", what, keyValue, id)
		} else {
			fmt.Printf("   ℹ️  复用已有%s：%s (%s)\n", what, keyValue, id)
		}
		return id, created
	}
	return "", false
}

func (c *client) delete(path, what string) {
	if _, err := c.call("DELETE", path, nil); err != nil {
		fmt.Printf("   ⚠️  删除%s失败（继续）：%v\n", what, err)
		return
	}
	fmt.Printf("   🗑  已删除%s\n", what)
}

/** *********************** 主流程 *********************** **/

func main() {
	flag.Parse()
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	lc := logger.NewClient("demoseed", "info", "")

	tpls, err := loadTemplates()
	if err != nil {
		fatal("加载内置工业模板失败：%v", err)
	}
	names := make([]string, 0, len(tpls))
	for _, t := range tpls {
		names = append(names, t.Key)
	}
	fmt.Printf("📦 已加载 %d 个内置模板：%s\n", len(tpls), strings.Join(names, ", "))

	c := &client{base: strings.TrimRight(*apiURL, "/")}
	if err := login(c, *user, *pass); err != nil {
		fatal("登录失败：%v\n请先启动后端（默认 %s），确认账号 %s 存在。", err, *apiURL, *user)
	}
	fmt.Printf("✅ 登录成功：%s\n", *user)

	codes := make([]string, 0, *tenants)
	for i := 0; i < *tenants; i++ {
		codes = append(codes, "demo_"+tenantSuffix(i))
	}

	sum := &summary{}
	if *doReset {
		cleanup(c, tpls, codes)
	}

	var lake dataClient
	if !*skipHist {
		cli, err := ducklake.NewClient(dtos.Configuration{
			DataSource:       *lakeCat,
			Dsn:              *lakeData,
			OverrideDataPath: *lakeOver,
		}, lc)
		if err != nil {
			fmt.Printf("⚠️  打开 DuckLake 失败：%v\n   已跳过历史数据写入（可显式加 -skip-history）\n", err)
		} else {
			lake = cli
			defer lake.CloseSession()
		}
	} else {
		fmt.Println("ℹ️  -skip-history：跳过历史数据写入")
	}

	for i, code := range codes {
		name := "演示租户" + strings.ToUpper(tenantSuffix(i))
		fmt.Printf("\n===== 租户 %d/%d：%s (%s) =====\n", i+1, len(codes), name, code)

		tenantID := ensureTenant(c, code, name)
		if tenantID == "" {
			fmt.Printf("⚠️  未拿到租户 id，跳过 %s\n", name)
			continue
		}
		c.tenant = tenantID
		sum.Tenants++

		rootID := findCompanyRoot(c)
		factoryID := ensureDept(c, rootID, "演示工厂1", 2)
		lineID := ensureDept(c, factoryID, "1号产线", 4)
		ensureGateway(c, code, lineID, sum)
		devRefs := ensureProductsAndDevices(c, tpls, code, sum)
		ensureAlertRules(c, tpls, devRefs, sum)
		if lake != nil {
			writeHistory(lake, devRefs, rng, sum)
		}
	}
	c.tenant = ""

	printSummary(sum)
}

/** *********************** 各步骤 *********************** **/

// loadTemplates 复用 seed 的内置模板（与启动迁移写入 solution 的数据同源），
// 并解析出物模型的数值属性 min/max 供随机游走使用。
func loadTemplates() ([]*tmpl, error) {
	sets, err := seed.LoadSolutions()
	if err != nil {
		return nil, err
	}
	out := make([]*tmpl, 0, len(sets))
	for _, s := range sets {
		t := &tmpl{
			Key:            s.Template.Key,
			Name:           s.Template.Name,
			CategoryKey:    s.CategoryTpl.CategoryKey,
			ThingModelJSON: s.ThingModelTpl.ThingModelJSON,
		}
		if err := json.Unmarshal([]byte(s.Template.AlarmRulesJson), &t.Alarms); err != nil {
			return nil, fmt.Errorf("解析模板 %s 告警预设失败：%w", t.Key, err)
		}
		props, _, _ := dtos.GetModelPropertyEventActionByThingModelTemplate(t.ThingModelJSON)
		for _, p := range props {
			if p.TypeSpec.Type != constants.SpecsTypeInt && p.TypeSpec.Type != constants.SpecsTypeFloat {
				continue
			}
			var sp struct {
				Min string `json:"min"`
				Max string `json:"max"`
			}
			_ = json.Unmarshal([]byte(p.TypeSpec.Specs), &sp)
			lo, _ := strconv.ParseFloat(sp.Min, 64)
			hi, _ := strconv.ParseFloat(sp.Max, 64)
			if hi <= lo {
				hi = lo + 100
			}
			t.Numeric = append(t.Numeric, numProp{
				Code: p.Code, Min: lo, Max: hi,
				Int: p.TypeSpec.Type == constants.SpecsTypeInt,
			})
		}
		out = append(out, t)
	}
	return out, nil
}

func login(c *client, username, password string) error {
	res, err := c.call("POST", "/api/v1/auth/login", map[string]string{"username": username, "password": password})
	if err != nil {
		return err
	}
	m, _ := res.(map[string]interface{})
	token := str(m["token"])
	if token == "" {
		return fmt.Errorf("响应里没有 token")
	}
	c.token = token
	return nil
}

// ensureTenant 平台级：按 code 查租户，不存在则新建
func ensureTenant(c *client, code, name string) string {
	saved := c.tenant
	c.tenant = "" // 平台视图
	defer func() { c.tenant = saved }()

	if id := c.findTenant(code); id != "" {
		fmt.Printf("   ℹ️  复用已有租户：%s (%s)\n", name, id)
		return id
	}
	if _, err := c.call("POST", "/api/v1/tenant", map[string]interface{}{
		"code":        code,
		"name":        name,
		"contactName": "演示联系人",
		"remark":      "demoseed 演示数据（可用 -reset 清理）",
	}); err != nil {
		fmt.Printf("   ⚠️  创建租户 %s 失败：%v（尝试复用已有对象）\n", name, err)
	}
	id := c.findTenant(code)
	if id != "" {
		fmt.Printf("   ✅ 租户就绪：%s (%s)\n", name, id)
	}
	return id
}

func (c *client) findTenant(code string) string {
	for _, path := range []string{
		"/api/v1/tenant?page=1&pageSize=100&code=" + code,
		"/api/v1/tenant?page=1&pageSize=100",
	} {
		items, err := c.list(path)
		if err != nil {
			continue
		}
		for _, it := range items {
			if str(it["code"]) == code || str(it["name"]) == "演示租户"+strings.ToUpper(strings.TrimPrefix(code, "demo_")) {
				return str(it["id"])
			}
		}
	}
	return ""
}

// findCompanyRoot 租户根组织节点（建租户时自动创建的公司节点）
func findCompanyRoot(c *client) string {
	nodes, err := c.deptTree()
	if err != nil {
		return ""
	}
	if n, ok := treeFind(nodes, func(m map[string]interface{}) bool {
		return intOf(m["deptType"]) == 1
	}); ok {
		return str(n["id"])
	}
	return ""
}

// ensureDept 幂等地创建组织节点（工厂/产线）
func ensureDept(c *client, parentID, name string, deptType int) string {
	if id := findDeptByName(c, name); id != "" {
		fmt.Printf("   ℹ️  复用已有组织节点：%s (%s)\n", name, id)
		return id
	}
	if _, err := c.call("POST", "/api/v1/dept", map[string]interface{}{
		"parentId": parentID,
		"name":     name,
		"deptType": deptType,
		"sort":     1,
		"status":   1,
	}); err != nil {
		fmt.Printf("   ⚠️  创建组织节点 %s 失败：%v（尝试复用已有对象）\n", name, err)
	}
	id := findDeptByName(c, name)
	if id != "" {
		fmt.Printf("   ✅ 组织节点就绪：%s (%s)\n", name, id)
	}
	return id
}

func findDeptByName(c *client, name string) string {
	nodes, err := c.deptTree()
	if err != nil {
		return ""
	}
	if n, ok := treeFind(nodes, func(m map[string]interface{}) bool { return str(m["name"]) == name }); ok {
		return str(n["id"])
	}
	return ""
}

func (c *client) deptTree() ([]interface{}, error) {
	res, err := c.call("GET", "/api/v1/dept-tree", nil)
	if err != nil {
		return nil, err
	}
	nodes, _ := res.([]interface{})
	return nodes, nil
}

// ensureGateway 建网关（sn: DEMO-<code>-GW1）
func ensureGateway(c *client, code, deptID string, sum *summary) {
	const listPath = "/api/v1/gateways?page=1&pageSize=500"
	sn := fmt.Sprintf("DEMO-%s-GW1", code)
	idx := c.index(listPath, "sn")
	id, created := c.ensure(idx, listPath, "sn", sn, "网关", func() error {
		_, err := c.call("POST", "/api/v1/gateway", map[string]interface{}{
			"name":   "演示网关1",
			"sn":     sn,
			"deptId": deptID,
			"vendor": "ROBX",
			"model":  "GW-2000",
			"remark": "demoseed 演示网关",
		})
		return err
	})
	if id == "" {
		fmt.Printf("   ⚠️  网关 %s 未创建成功，跳过\n", sn)
		return
	}
	if created {
		sum.Gateways++
	} else {
		sum.Reused++
	}
}

// ensureProductsAndDevices 用 7 个模板建产品并发布，每个产品建 N 台设备
func ensureProductsAndDevices(c *client, tpls []*tmpl, code string, sum *summary) []devRef {
	const (
		prodPath = "/api/v1/products?page=1&pageSize=500"
		devPath  = "/api/v1/devices?page=1&pageSize=1000"
	)
	prodIdx := c.index(prodPath, "name")
	devIdx := c.index(devPath, "name")
	out := make([]devRef, 0, len(tpls)*maxInt(1, *devices))

	for _, t := range tpls {
		productID, created := c.ensure(prodIdx, prodPath, "name", t.Name, "产品", func() error {
			_, err := c.call("POST", "/api/v1/product", map[string]interface{}{
				"name":                 t.Name,
				"key":                  "",
				"category_template_id": t.CategoryKey,
				"protocol":             "MQTT",
				"node_type":            "direct",
				"net_type":             "wifi",
				"data_format":          "json",
				"description":          "demoseed 演示产品（模板 " + t.Key + "）",
			})
			return err
		})
		if productID == "" {
			fmt.Printf("   ⚠️  产品 %s 未创建成功，跳过该模板\n", t.Name)
			continue
		}
		if created {
			sum.Products++
		} else {
			sum.Reused++
		}
		// 未发布的产品不能建设备；已发布时接口会返回“已发布”类错误，仅提示
		if _, err := c.call("POST", "/api/v1/product-release/"+productID, nil); err != nil {
			fmt.Printf("   ⚠️  发布产品 %s 未成功（若已发布可忽略）：%v\n", t.Name, err)
		} else {
			fmt.Printf("   ✅ 已发布产品：%s\n", t.Name)
		}

		for i := 1; i <= maxInt(1, *devices); i++ {
			devName := fmt.Sprintf("%s-%d", t.Name, i)
			sn := fmt.Sprintf("DEMO-%s-%s-%d", code, t.Key, i)
			devID, devCreated := c.ensure(devIdx, devPath, "name", devName, "设备", func() error {
				_, err := c.call("POST", "/api/v1/device", map[string]interface{}{
					"name":        devName,
					"product_id":  productID,
					"device_sn":   sn,
					"description": "demoseed 演示设备",
					"platform":    "LocalIot",
				})
				return err
			})
			if devID == "" {
				fmt.Printf("   ⚠️  设备 %s 未创建成功，跳过\n", devName)
				continue
			}
			if devCreated {
				sum.Devices++
			} else {
				sum.Reused++
			}
			out = append(out, devRef{Id: devID, Name: devName, ProductId: productID, Tpl: t})
		}
	}
	return out
}

// ensureAlertRules 给「注塑机」「冷库」各建 2 条告警规则（两步：POST 建规则 + PUT 补条件）
func ensureAlertRules(c *client, tpls []*tmpl, devs []devRef, sum *summary) {
	const listPath = "/api/v1/alert-rule?page=1&pageSize=500"
	idx := c.index(listPath, "name")

	for _, key := range []string{"injection_molding", "refrigeration"} {
		t := findTpl(tpls, key)
		dev := firstDev(devs, key)
		if t == nil || dev == nil {
			fmt.Printf("   ⚠️  模板 %s 没有对应设备，跳过其告警规则\n", key)
			continue
		}
		numeric := map[string]bool{}
		for _, p := range t.Numeric {
			numeric[p.Code] = true
		}
		picked := 0
		for _, al := range t.Alarms {
			if picked >= 2 {
				break
			}
			if !numeric[al.PropertyCode] {
				continue // 只对数值属性建「大于/小于」比较型规则
			}
			ruleName := "演示-" + t.Name + "-" + al.Name
			if id := idx[ruleName]; id != "" {
				fmt.Printf("   ℹ️  复用已有告警规则：%s (%s)\n", ruleName, id)
				sum.Reused++
				picked++
				continue
			}
			if id := createAlertRule(c, idx, listPath, ruleName, al, dev); id != "" {
				sum.Rules++
				picked++
			}
		}
	}
}

func createAlertRule(c *client, idx map[string]string, listPath, ruleName string, al seed.SolutionAlarm, dev *devRef) string {
	if _, err := c.call("POST", "/api/v1/alert-rule", map[string]interface{}{
		"name":        ruleName,
		"alert_type":  string(constants.DeviceAlertType),
		"alert_level": levelText(al.AlertLevel),
		"description": al.Description,
	}); err != nil {
		fmt.Printf("   ⚠️  创建告警规则 %s 失败：%v（尝试复用已有对象）\n", ruleName, err)
	}
	for k, v := range c.index(listPath, "name") {
		idx[k] = v
	}
	ruleID := idx[ruleName]
	if ruleID == "" {
		fmt.Printf("   ⚠️  告警规则 %s 未创建成功，跳过\n", ruleName)
		return ""
	}
	fmt.Printf("   ✅ 告警规则（基础信息）：%s (%s)\n", ruleName, ruleID)

	// 第二步：补触发条件（阈值来自模板 solution.AlarmRulesJson）
	if _, err := c.call("PUT", "/api/v1/alert-rule/"+ruleID, map[string]interface{}{
		"id":           ruleID,
		"condition":    string(constants.WorkerConditionAnyone),
		"silence_time": al.SilenceTime,
		"notify":       []interface{}{},
		"sub_rule": []map[string]interface{}{{
			"trigger":    string(constants.DeviceDataTrigger),
			"product_id": dev.ProductId,
			"device_id":  dev.Id,
			"option": map[string]string{
				"code":             al.PropertyCode,
				"decide_condition": strings.TrimSpace(al.Operator + " " + al.Value),
				"value_type":       constants.Original,
			},
		}},
	}); err != nil {
		fmt.Printf("   ⚠️  告警规则 %s 触发条件未生效（需 ekuiper 可用）：%v\n", ruleName, err)
	} else {
		fmt.Printf("   ✅ 告警规则触发条件已生效：%s %s %s（%s）\n", al.PropertyCode, al.Operator, al.Value, levelText(al.AlertLevel))
	}
	return ruleID
}

/** *********************** 历史数据 *********************** **/

type dataClient interface {
	Insert(ctx context.Context, table string, data map[string]interface{}) error
	CloseSession()
}

// writeHistory 为每台设备的每个数值属性生成随机游走序列，直写 DuckLake
func writeHistory(lake dataClient, devs []devRef, rng *rand.Rand, sum *summary) {
	if len(devs) == 0 {
		return
	}
	step := time.Duration(*stepMin) * time.Minute
	if step <= 0 {
		step = 5 * time.Minute
	}
	n := int(int64(*days) * 24 * 60 / int64(maxInt(1, *stepMin)))
	if n <= 0 {
		n = 1
	}
	start := time.Now().Add(-time.Duration(*days) * 24 * time.Hour)
	fmt.Printf("\n📈 历史数据：%d 台设备 × 数值属性，%d 天 / %d 分钟间隔（每设备每属性 %d 点）\n",
		len(devs), *days, *stepMin, n)

	ctx := context.Background()
	for i := range devs {
		d := &devs[i]
		props := d.Tpl.Numeric
		if len(props) == 0 {
			continue
		}
		series := make([][]float64, len(props))
		for j, p := range props {
			series[j] = genSeries(p.Min, p.Max, n, rng)
		}
		errCount := 0
		written := 0
		data := make(map[string]interface{}, len(props))
		for k := 0; k < n; k++ {
			ts := start.Add(time.Duration(k) * step).UnixMilli()
			for key := range data {
				delete(data, key)
			}
			for j, p := range props {
				v := series[j][k]
				if p.Int {
					v = math.Round(v)
				} else {
					v = math.Round(v*100) / 100
				}
				data[p.Code] = dtos.ReportData{Value: v, Time: ts}
			}
			if err := lake.Insert(ctx, d.Id, data); err != nil {
				errCount++
				if errCount <= 3 {
					fmt.Printf("   ⚠️  写入 %s 历史失败：%v\n", d.Name, err)
				}
				continue
			}
			written += len(props)
		}
		sum.Points += written
		fmt.Printf("   ✅ %s（%s）：%d 个数值属性 × %d 点 = %d 条\n", d.Name, d.Tpl.Name, len(props), n, written)
	}
}

// genSeries 在 [min,max] 内做随机游走，返回 n 个采样值
func genSeries(min, max float64, n int, rng *rand.Rand) []float64 {
	span := max - min
	if span <= 0 {
		span = 1
	}
	amp := span * 0.02
	v := min + span/2
	out := make([]float64, n)
	for i := range out {
		v += (rng.Float64()*2 - 1) * amp
		if v < min {
			v = min + rng.Float64()*amp
		}
		if v > max {
			v = max - rng.Float64()*amp
		}
		out[i] = v
	}
	return out
}

/** *********************** -reset 清理 *********************** **/

// cleanup 只清理本工具创建的演示对象（租户 code=demo_*，且名称符合模板命名），
// 任何一步失败只告警并继续。
func cleanup(c *client, tpls []*tmpl, codes []string) {
	fmt.Println("\n🧹 -reset：开始清理本工具创建的演示数据…")
	for _, code := range codes {
		saved := c.tenant
		c.tenant = ""
		id := c.findTenant(code)
		c.tenant = saved
		if id == "" {
			continue
		}
		fmt.Printf("  → 租户 %s (%s)\n", code, id)
		c.tenant = id

		if items, err := c.list("/api/v1/alert-rule?page=1&pageSize=500"); err == nil {
			for _, it := range items {
				name := str(it["name"])
				if strings.HasPrefix(name, "演示-") {
					c.delete("/api/v1/alert-rule/"+str(it["id"]), "告警规则 "+name)
				}
			}
		} else {
			fmt.Printf("   ⚠️  读取告警规则失败（继续）：%v\n", err)
		}

		if items, err := c.list("/api/v1/devices?page=1&pageSize=1000"); err == nil {
			for _, it := range items {
				name := str(it["name"])
				if matchDeviceName(tpls, name) {
					c.delete("/api/v1/device/"+str(it["id"]), "设备 "+name)
				}
			}
		} else {
			fmt.Printf("   ⚠️  读取设备失败（继续）：%v\n", err)
		}

		if items, err := c.list("/api/v1/products?page=1&pageSize=500"); err == nil {
			for _, it := range items {
				name := str(it["name"])
				if tplNameMatch(tpls, name) {
					c.delete("/api/v1/product/"+str(it["id"]), "产品 "+name)
				}
			}
		} else {
			fmt.Printf("   ⚠️  读取产品失败（继续）：%v\n", err)
		}

		if items, err := c.list("/api/v1/gateways?page=1&pageSize=500"); err == nil {
			for _, it := range items {
				sn := str(it["sn"])
				if strings.HasPrefix(sn, "DEMO-"+code+"-") {
					c.delete("/api/v1/gateway/"+str(it["id"]), "网关 "+sn)
				}
			}
		} else {
			fmt.Printf("   ⚠️  读取网关失败（继续）：%v\n", err)
		}

		// 先删产线再删工厂（父节点有子节点时接口会拒绝删除）
		for _, deptName := range []string{"1号产线", "演示工厂1"} {
			if did := findDeptByName(c, deptName); did != "" {
				if _, err := c.call("DELETE", "/api/v1/dept", map[string]interface{}{"ids": []string{did}}); err != nil {
					fmt.Printf("   ⚠️  删除组织节点 %s 失败（继续）：%v\n", deptName, err)
				} else {
					fmt.Printf("   🗑  已删除组织节点 %s\n", deptName)
				}
			}
		}

		c.tenant = ""
		c.delete("/api/v1/tenant/"+id, "租户 "+code)
	}
	c.tenant = ""
	fmt.Println("🧹 清理结束（DuckLake 里已写入的历史数据没有删除接口，会保留）")
}

/** *********************** 小工具 *********************** **/

func printSummary(s *summary) {
	fmt.Println("\n========== 演示数据生成完成 ==========")
	fmt.Printf("租户(租户)      ：%d\n", s.Tenants)
	fmt.Printf("网关            ：%d\n", s.Gateways)
	fmt.Printf("产品            ：%d\n", s.Products)
	fmt.Printf("设备            ：%d\n", s.Devices)
	fmt.Printf("告警规则        ：%d\n", s.Rules)
	fmt.Printf("历史数据点      ：%d\n", s.Points)
	fmt.Printf("复用的已有对象  ：%d\n", s.Reused)
	fmt.Println("提示：历史数据写入 DuckLake；查询示例：GET /api/v1/device/<deviceId>/thing-model/history-property?code=<code>&page=1&pageSize=20")
}

func tenantSuffix(i int) string {
	if i < 26 {
		return string(rune('a' + i))
	}
	return fmt.Sprintf("t%d", i+1)
}

func levelText(level int) string {
	switch level {
	case 1:
		return string(constants.Urgent)
	case 2:
		return string(constants.Important)
	case 3:
		return string(constants.LessImportant)
	case 4:
		return string(constants.Remind)
	default:
		return string(constants.Important)
	}
}

func findTpl(tpls []*tmpl, key string) *tmpl {
	for _, t := range tpls {
		if t.Key == key {
			return t
		}
	}
	return nil
}

func firstDev(devs []devRef, key string) *devRef {
	for i := range devs {
		if devs[i].Tpl.Key == key {
			return &devs[i]
		}
	}
	return nil
}

func tplNameMatch(tpls []*tmpl, name string) bool {
	for _, t := range tpls {
		if t.Name == name {
			return true
		}
	}
	return false
}

func matchDeviceName(tpls []*tmpl, name string) bool {
	for _, t := range tpls {
		if strings.HasPrefix(name, t.Name+"-") {
			return true
		}
	}
	return false
}

// treeFind 在 dept-tree 结果里递归查找第一个满足条件的节点
func treeFind(nodes []interface{}, pred func(map[string]interface{}) bool) (map[string]interface{}, bool) {
	for _, n := range nodes {
		m, ok := n.(map[string]interface{})
		if !ok {
			continue
		}
		if pred(m) {
			return m, true
		}
		if children, ok := m["children"].([]interface{}); ok {
			if found, ok := treeFind(children, pred); ok {
				return found, true
			}
		}
	}
	return nil, false
}

func str(v interface{}) string {
	s, _ := v.(string)
	return s
}

func intOf(v interface{}) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case string:
		i, _ := strconv.Atoi(n)
		return i
	}
	return 0
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func fatal(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "❌ "+format+"\n", args...)
	os.Exit(1)
}
