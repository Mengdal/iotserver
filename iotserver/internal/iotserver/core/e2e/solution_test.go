package e2e

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"iotserver/internal/dtos"
	"iotserver/internal/models"
	"iotserver/internal/pkg/constants"
	"iotserver/internal/pkg/errort"
	"iotserver/internal/pkg/httphelper"
	"iotserver/internal/pkg/middleware"
	"iotserver/internal/pkg/permissions"
	"iotserver/internal/pkg/seed"
	"iotserver/internal/pkg/tenant"
)

/** 假数据存储客户端：方案一键应用会发布产品/建设备，需要 DataDBClient 建库表 **/

type fakeDataDB struct{}

func (fakeDataDB) GetDataDBType() constants.DataType { return constants.TDengine }
func (fakeDataDB) CloseSession()                     {}

func (fakeDataDB) Insert(ctx context.Context, table string, data map[string]interface{}) error {
	return nil
}

func (fakeDataDB) GetDeviceProperty(req dtos.ThingModelPropertyDataRequest, device models.Device) ([]dtos.ReportData, int, error) {
	return nil, 0, nil
}

func (fakeDataDB) GetDeviceService(req dtos.ThingModelServiceDataRequest, device models.Device, product models.Product) ([]dtos.SaveServiceIssueData, int, error) {
	return nil, 0, nil
}

func (fakeDataDB) GetDeviceEvent(req dtos.ThingModelEventDataRequest, device models.Device, product models.Product) ([]dtos.EventData, int, error) {
	return nil, 0, nil
}

func (fakeDataDB) CreateTable(ctx context.Context, stable, table string) error { return nil }
func (fakeDataDB) DropTable(ctx context.Context, table string) error           { return nil }

func (fakeDataDB) CreateStable(ctx context.Context, product models.Product) error { return nil }
func (fakeDataDB) DropStable(ctx context.Context, table string) error             { return nil }

func (fakeDataDB) AddDatabaseField(ctx context.Context, tableName string, specsType constants.SpecsType, code string, name string) error {
	return nil
}

func (fakeDataDB) DelDatabaseField(ctx context.Context, tableName, code string) error { return nil }

func (fakeDataDB) ModifyDatabaseField(ctx context.Context, tableName string, specsType constants.SpecsType, code string, name string) error {
	return nil
}

func (fakeDataDB) GetDevicePropertyCount(dtos.ThingModelPropertyDataRequest) (int, error) {
	return 0, nil
}

func (fakeDataDB) GetDeviceEventCount(req dtos.ThingModelEventDataRequest) (int, error) {
	return 0, nil
}

func (fakeDataDB) GetDeviceMsgCountByGiveTime(deviceId string, startTime, endTime int64) (int, error) {
	return 0, nil
}

// applySolution 调用一键应用接口：成功返回结果体，失败返回完整响应
func applySolution(t *testing.T, env *testEnv, key string, body []byte) (map[string]interface{}, httphelper.CommonResponse) {
	t.Helper()
	w := doRequest(env.engine, http.MethodPost, "/api/v1/solution/"+key+"/apply", env.token, body)
	resp := parseResponse(t, w)
	if !resp.Success {
		return nil, resp
	}
	result, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatalf("unexpected apply result type %T", resp.Result)
	}
	return result, resp
}

// ① 模板列表返回 15 个内置模板（7 个行业 × 2 个产品 + 1 个网关模板）
func TestSolutionSearchReturnsBuiltInTemplates(t *testing.T) {
	env := setupTestEnv(t)
	w := doRequest(env.engine, http.MethodGet, "/api/v1/solutions", env.token, nil)
	resp := parseResponse(t, w)
	if !resp.Success {
		t.Fatalf("solutions failed: %v", resp.ErrorMsg)
	}
	list, ok := resp.Result.([]interface{})
	if !ok {
		t.Fatalf("unexpected result type %T", resp.Result)
	}
	if len(list) != 15 {
		t.Fatalf("内置模板应为 15 个（7 个行业 × 2 个产品 + 1 个网关模板），实际 %d", len(list))
	}
	keys := map[string]bool{}
	for _, item := range list {
		tpl, ok := item.(map[string]interface{})
		if !ok {
			t.Fatalf("unexpected item type %T", item)
		}
		if builtIn, _ := tpl["builtIn"].(bool); !builtIn {
			t.Fatalf("模板 %v 应标记为内置", tpl["key"])
		}
		if name, _ := tpl["name"].(string); name == "" {
			t.Fatalf("模板 %v 缺少名称", tpl["key"])
		}
		key, _ := tpl["key"].(string)
		keys[key] = true
	}
	for _, want := range []string{"injection_molding", "refrigeration", "production_line", "industrial_gateway"} {
		if !keys[want] {
			t.Fatalf("缺少内置模板 %s", want)
		}
	}

	// 摘要模式：列表不返回物模型/告警/看板等大字段，详情接口才返回完整内容
	w = doRequest(env.engine, http.MethodGet, "/api/v1/solutions?summary=true", env.token, nil)
	resp = parseResponse(t, w)
	summaryList, ok := resp.Result.([]interface{})
	if !ok || len(summaryList) != 15 {
		t.Fatalf("摘要列表异常: %T len=%d", resp.Result, len(summaryList))
	}
	for _, item := range summaryList {
		tpl := item.(map[string]interface{})
		for _, field := range []string{"thingModelJson", "alarmRulesJson", "dashboardJson", "metricsJson"} {
			if raw, _ := tpl[field].(string); raw != "" {
				t.Fatalf("摘要列表不应返回 %s（模板 %v）", field, tpl["key"])
			}
		}
	}
	w = doRequest(env.engine, http.MethodGet, "/api/v1/solution/industrial_gateway", env.token, nil)
	resp = parseResponse(t, w)
	detail, ok := resp.Result.(map[string]interface{})
	if !ok || !resp.Success {
		t.Fatalf("网关模板详情失败: %v", resp.ErrorMsg)
	}
	tm, _ := detail["thingModelJson"].(string)
	if !strings.Contains(tm, `"CPU_USAGE"`) || !strings.Contains(tm, `"IOPoints"`) {
		t.Fatalf("网关模板物模型缺少系统点: %s", tm[:min(200, len(tm))])
	}

	// 行业过滤：用列表中注塑机模板的行业做过滤，结果必须都属于该行业
	industry := ""
	for _, item := range list {
		tpl := item.(map[string]interface{})
		if tpl["key"] == "injection_molding" {
			industry, _ = tpl["industry"].(string)
		}
	}
	if industry == "" {
		t.Fatal("注塑机模板缺少行业字段")
	}
	w = doRequest(env.engine, http.MethodGet, "/api/v1/solutions?industry="+url.QueryEscape(industry), env.token, nil)
	resp = parseResponse(t, w)
	if !resp.Success {
		t.Fatalf("solutions industry filter failed: %v", resp.ErrorMsg)
	}
	filtered := resp.Result.([]interface{})
	if len(filtered) == 0 {
		t.Fatalf("行业 %q 过滤后为空", industry)
	}
	for _, item := range filtered {
		tpl := item.(map[string]interface{})
		if tpl["industry"] != industry {
			t.Fatalf("行业过滤失效，返回了 %v", tpl["industry"])
		}
	}
}

// ② 一键应用注塑机模板：产品 + 发布 + 锚定设备 + 5 条告警规则
func TestSolutionApplyCreatesProductAndAlarmRules(t *testing.T) {
	env := setupTestEnv(t)

	result, fail := applySolution(t, env, "injection_molding", []byte(`{}`))
	if !fail.Success {
		t.Fatalf("apply failed: code=%d msg=%s", fail.ErrorCode, fail.ErrorMsg)
	}

	productId, _ := result["productId"].(string)
	if productId == "" {
		t.Fatal("apply 未返回 productId")
	}
	if name, _ := result["productName"].(string); name != "注塑机" {
		t.Fatalf("默认产品名应为模板名「注塑机」，实际 %q", name)
	}

	deviceIds, _ := result["deviceIds"].([]interface{})
	if len(deviceIds) != 1 {
		t.Fatalf("告警规则需要锚定设备，应创建 1 台设备，实际 %d", len(deviceIds))
	}

	alarmRuleIds, _ := result["alarmRuleIds"].([]interface{})
	if len(alarmRuleIds) != 5 {
		t.Fatalf("注塑机模板预设 5 条告警规则，实际 %d", len(alarmRuleIds))
	}

	// 产品应为已发布状态，且带上了品类物模型属性
	w := doRequest(env.engine, http.MethodGet, "/api/v1/product/"+productId, env.token, nil)
	resp := parseResponse(t, w)
	if !resp.Success {
		t.Fatalf("product detail failed: %v", resp.ErrorMsg)
	}
	product := resp.Result.(map[string]interface{})
	if status, _ := product["status"].(string); status != string(constants.ProductRelease) {
		t.Fatalf("产品应已发布，实际状态 %q", status)
	}
	if protocol, _ := product["protocol"].(string); protocol != string(constants.ProductProtocolMQTT) {
		t.Fatalf("产品协议应为 MQTT，实际 %q", protocol)
	}
	if nodeType, _ := product["node_type"].(string); nodeType != string(constants.ProductNodeTypeSubDevice) {
		t.Fatalf("产品节点类型应为网关子设备，实际 %q", nodeType)
	}
	if netType, _ := product["net_type"].(string); netType != string(constants.ProductNetTypeEthernet) {
		t.Fatalf("产品联网方式应为以太网，实际 %q", netType)
	}
	props, _ := product["properties"].([]interface{})
	if len(props) == 0 {
		t.Fatal("产品物模型属性为空：品类模板未生效")
	}

	// 告警规则确实落库（列表可查到，且状态有效）
	w = doRequest(env.engine, http.MethodGet, "/api/v1/alert-rule?page=1&pageSize=20", env.token, nil)
	resp = parseResponse(t, w)
	if !resp.Success {
		t.Fatalf("alert-rule list failed: %v", resp.ErrorMsg)
	}
	rulePage := resp.Result.(map[string]interface{})
	rules := rulePage["list"].([]interface{})
	if len(rules) != 5 {
		t.Fatalf("告警中心应有 5 条规则，实际 %d", len(rules))
	}
	for _, item := range rules {
		rule := item.(map[string]interface{})
		subRules, _ := rule["sub_rule"].([]interface{})
		if len(subRules) != 1 {
			t.Fatalf("告警规则 %v 缺少触发条件", rule["name"])
		}
		sub := subRules[0].(map[string]interface{})
		option, _ := sub["option"].(map[string]interface{})
		if option["value_type"] != constants.Original {
			t.Fatalf("value_type 应为 %s，实际 %v", constants.Original, option["value_type"])
		}
		if option["decide_condition"] == nil || option["code"] == nil {
			t.Fatalf("告警规则 %v 的触发条件不完整: %v", rule["name"], option)
		}
	}
}

// ③ 重复 apply：第二次是明确的「资源重复」业务错误，不产生脏数据
func TestSolutionApplyTwiceReturnsBusinessError(t *testing.T) {
	env := setupTestEnv(t)

	if _, fail := applySolution(t, env, "injection_molding", []byte(`{}`)); !fail.Success {
		t.Fatalf("first apply failed: code=%d msg=%s", fail.ErrorCode, fail.ErrorMsg)
	}

	// 第二次同名（默认产品名 = 模板名）应用：产品名唯一约束生效，返回 10009 而不是 500/panic
	w := doRequest(env.engine, http.MethodPost, "/api/v1/solution/injection_molding/apply", env.token, []byte(`{}`))
	if w.Code != http.StatusOK {
		t.Fatalf("重复 apply 不应返回 HTTP %d", w.Code)
	}
	resp := parseResponse(t, w)
	if resp.Success {
		t.Fatal("重复 apply 应返回「产品名称已存在」业务错误")
	}
	if resp.ErrorCode != errort.DefaultResourcesRepeat {
		t.Fatalf("期望错误码 %d，实际 %d (%s)", errort.DefaultResourcesRepeat, resp.ErrorCode, resp.ErrorMsg)
	}

	// 失败不产生第二份数据
	w = doRequest(env.engine, http.MethodGet, "/api/v1/products?page=1&pageSize=20", env.token, nil)
	resp = parseResponse(t, w)
	if !resp.Success {
		t.Fatalf("products list failed: %v", resp.ErrorMsg)
	}
	if total := int(resp.Result.(map[string]interface{})["total"].(float64)); total != 1 {
		t.Fatalf("重复 apply 失败后产品数应仍为 1，实际 %d", total)
	}

	// 换一个产品名再应用则成功（行为一致：同名拒绝、改名可重试）
	body, _ := json.Marshal(dtos.SolutionApplyRequest{ProductName: "注塑机-二号产线"})
	result, fail := applySolution(t, env, "injection_molding", body)
	if !fail.Success {
		t.Fatalf("改名后 apply 失败: code=%d msg=%s", fail.ErrorCode, fail.ErrorMsg)
	}
	if names, _ := result["productName"].(string); names != "注塑机-二号产线" {
		t.Fatalf("产品名应使用请求值，实际 %q", names)
	}
	alarmRuleIds, _ := result["alarmRuleIds"].([]interface{})
	if len(alarmRuleIds) != 5 {
		t.Fatalf("改名后应用也应创建 5 条告警规则，实际 %d", len(alarmRuleIds))
	}
}

// ⑤ 15 个内置模板都能一键应用：告警规则数与预设条数一致
//
// 该用例同时守住「预设阈值必须能通过平台告警校验」这条契约——
// 例如布尔属性在模板里写 0/1，而校验只认 true/false，需要应用层归一化。
func TestSolutionApplyAllBuiltInTemplates(t *testing.T) {
	env := setupTestEnv(t)

	w := doRequest(env.engine, http.MethodGet, "/api/v1/solutions", env.token, nil)
	resp := parseResponse(t, w)
	if !resp.Success {
		t.Fatalf("solutions failed: %v", resp.ErrorMsg)
	}
	list := resp.Result.([]interface{})
	if len(list) != 15 {
		t.Fatalf("内置模板应为 15 个（7 个行业 × 2 个产品 + 1 个网关模板），实际 %d", len(list))
	}

	for _, item := range list {
		tpl := item.(map[string]interface{})
		key, _ := tpl["key"].(string)
		var alarms []map[string]interface{}
		if raw, _ := tpl["alarmRulesJson"].(string); raw != "" {
			if err := json.Unmarshal([]byte(raw), &alarms); err != nil {
				t.Fatalf("模板 %s 的告警预设非法: %v", key, err)
			}
		}

		body, _ := json.Marshal(dtos.SolutionApplyRequest{ProductName: "模板-" + key})
		result, fail := applySolution(t, env, key, body)
		if !fail.Success {
			t.Fatalf("模板 %s 应用失败: code=%d msg=%s", key, fail.ErrorCode, fail.ErrorMsg)
		}
		ruleIds, _ := result["alarmRuleIds"].([]interface{})
		if len(ruleIds) != len(alarms) {
			t.Fatalf("模板 %s 应创建 %d 条告警规则，实际 %d", key, len(alarms), len(ruleIds))
		}
	}

	// 每个模板都生成了独立产品
	w = doRequest(env.engine, http.MethodGet, "/api/v1/products?page=1&pageSize=50", env.token, nil)
	resp = parseResponse(t, w)
	if !resp.Success {
		t.Fatalf("products list failed: %v", resp.ErrorMsg)
	}
	if total := int(resp.Result.(map[string]interface{})["total"].(float64)); total != 15 {
		t.Fatalf("15 个模板应生成 15 个产品，实际 %d", total)
	}
}

// ⑥ apply 选项：CreateAlarmRules=false 不建规则；DeviceCount>0 按数量建设备
func TestSolutionApplyOptions(t *testing.T) {
	env := setupTestEnv(t)

	// 关闭告警规则：不建规则也不需要锚定设备
	noAlarm := false
	body, _ := json.Marshal(dtos.SolutionApplyRequest{ProductName: "无告警产品", CreateAlarmRules: &noAlarm})
	result, fail := applySolution(t, env, "packaging_machine", body)
	if !fail.Success {
		t.Fatalf("apply(no alarm) failed: code=%d msg=%s", fail.ErrorCode, fail.ErrorMsg)
	}
	if ids, _ := result["alarmRuleIds"].([]interface{}); len(ids) != 0 {
		t.Fatalf("CreateAlarmRules=false 时不应创建告警规则，实际 %d", len(ids))
	}
	if ids, _ := result["deviceIds"].([]interface{}); len(ids) != 0 {
		t.Fatalf("CreateAlarmRules=false 且 DeviceCount=0 时不应创建设备，实际 %d", len(ids))
	}

	// 指定设备数量：建 2 台设备 + 5 条规则
	body, _ = json.Marshal(dtos.SolutionApplyRequest{ProductName: "包装机-双线", DeviceCount: 2})
	result, fail = applySolution(t, env, "packaging_machine", body)
	if !fail.Success {
		t.Fatalf("apply(2 devices) failed: code=%d msg=%s", fail.ErrorCode, fail.ErrorMsg)
	}
	if ids, _ := result["deviceIds"].([]interface{}); len(ids) != 2 {
		t.Fatalf("DeviceCount=2 应创建 2 台设备，实际 %d", len(ids))
	}
	if ids, _ := result["alarmRuleIds"].([]interface{}); len(ids) != 5 {
		t.Fatalf("应创建 5 条告警规则，实际 %d", len(ids))
	}
	skipped, _ := result["skipped"].([]interface{})
	if len(skipped) == 0 {
		t.Fatal("多设备时应提示告警规则只挂载第 1 台设备")
	}

	// 不存在的模板 key
	w := doRequest(env.engine, http.MethodPost, "/api/v1/solution/not_exist/apply", env.token, []byte(`{}`))
	resp := parseResponse(t, w)
	if resp.Success {
		t.Fatal("不存在的模板应返回失败")
	}
	if resp.ErrorCode != errort.DefaultResourcesNotFound {
		t.Fatalf("期望错误码 %d，实际 %d (%s)", errort.DefaultResourcesNotFound, resp.ErrorCode, resp.ErrorMsg)
	}
}

// ④ 应用后的产品能在产品列表查到
func TestSolutionAppliedProductVisibleInList(t *testing.T) {
	env := setupTestEnv(t)

	if _, fail := applySolution(t, env, "refrigeration", []byte(`{"productName":"冷库机组-A"}`)); !fail.Success {
		t.Fatalf("apply failed: code=%d msg=%s", fail.ErrorCode, fail.ErrorMsg)
	}

	w := doRequest(env.engine, http.MethodGet, "/api/v1/products?page=1&pageSize=20", env.token, nil)
	resp := parseResponse(t, w)
	if !resp.Success {
		t.Fatalf("products list failed: %v", resp.ErrorMsg)
	}
	page := resp.Result.(map[string]interface{})
	list := page["list"].([]interface{})
	found := false
	for _, item := range list {
		product := item.(map[string]interface{})
		if product["name"] == "冷库机组-A" {
			found = true
		}
	}
	if !found {
		t.Fatalf("产品列表未找到应用后的产品: %v", list)
	}
}

/******************************* 方案物模型（TSL）条目维护 *******************************/

// solutionRowInDB 直接读库中的方案行（tenantId="" 即内置行）
func solutionRowInDB(t *testing.T, env *testEnv, key, tenantId string) (models.Solution, error) {
	t.Helper()
	var row models.Solution
	err := env.client.Pool.Table(models.TableSolution).
		Where("`key` = ? AND tenant_id = ?", key, tenantId).First(&row).Error
	return row, err
}

// thingModelIdentifiers 取物模型某个能力数组的全部标识符
func thingModelIdentifiers(t *testing.T, thingModelJSON, arrayKey string) []string {
	t.Helper()
	var doc map[string][]map[string]interface{}
	if err := json.Unmarshal([]byte(thingModelJSON), &doc); err != nil {
		t.Fatalf("物模型 JSON 解析失败: %v (%s)", err, thingModelJSON)
	}
	out := []string{}
	for _, item := range doc[arrayKey] {
		identifier, _ := item["identifier"].(string)
		out = append(out, identifier)
	}
	return out
}

func containsIdentifier(list []string, want string) bool {
	for _, v := range list {
		if v == want {
			return true
		}
	}
	return false
}

// 新增内置方案的物模型属性：接口成功、内置行不动、租户副本生成
func TestSolutionThingModelItemAddOnBuiltInCreatesTenantCopy(t *testing.T) {
	env := setupTestEnv(t)
	tenantRow, username := seedTenant(t, env, "tsladd")
	token := loginAs(t, env, username, tenantTestPassword)
	const key = "injection_molding"

	builtInBefore, err := solutionRowInDB(t, env, key, "")
	if err != nil {
		t.Fatalf("内置方案 %s 未 seed: %v", key, err)
	}

	body := []byte(`{"type":"property","item":{"name":"模腔压力","identifier":"e2e_cavity_pressure","dataType":"FLOAT","description":"e2e","required":false,"rwFlag":"RW","dataSpecs":{"max":"200","min":"0","step":"0.1","unit":"bar","unitName":"巴"},"dataSpecsList":null}}`)
	w := doRequest(env.engine, http.MethodPost, "/api/v1/solution/"+key+"/thing-model/item", token, body)
	resp := parseResponse(t, w)
	if !resp.Success {
		t.Fatalf("租户在内置方案上新增属性失败: code=%d msg=%s", resp.ErrorCode, resp.ErrorMsg)
	}
	result, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatalf("unexpected result type %T", resp.Result)
	}
	if builtIn, _ := result["builtIn"].(bool); builtIn {
		t.Fatal("修改内置方案后应返回租户副本(builtIn=false)")
	}
	thingModel, _ := result["thingModelJson"].(string)
	if !containsIdentifier(thingModelIdentifiers(t, thingModel, "properties"), "e2e_cavity_pressure") {
		t.Fatalf("接口返回的物模型缺少新属性: %s", thingModel)
	}

	// 内置行绝不被原地修改
	builtInAfter, err := solutionRowInDB(t, env, key, "")
	if err != nil {
		t.Fatalf("读取内置行失败: %v", err)
	}
	if builtInAfter.ThingModelJson != builtInBefore.ThingModelJson {
		t.Fatal("库中内置行(tenant_id='')的 ThingModelJson 被原地修改")
	}
	if builtInAfter.BuiltIn != 1 || builtInAfter.TenantId != "" {
		t.Fatalf("内置行被改写: built_in=%d tenant_id=%q", builtInAfter.BuiltIn, builtInAfter.TenantId)
	}

	// 租户副本已生成
	copyRow, err := solutionRowInDB(t, env, key, tenantRow.Id)
	if err != nil {
		t.Fatalf("租户副本未生成: %v", err)
	}
	if copyRow.BuiltIn != 0 {
		t.Fatalf("租户副本 built_in 应为 0，实际 %d", copyRow.BuiltIn)
	}
	if !strings.HasPrefix(copyRow.Id, "sol-"+key+"-") || copyRow.Id == builtInAfter.Id {
		t.Fatalf("租户副本 id 应为 sol-%s-<随机>，实际 %s", key, copyRow.Id)
	}
	if !containsIdentifier(thingModelIdentifiers(t, copyRow.ThingModelJson, "properties"), "e2e_cavity_pressure") {
		t.Fatal("租户副本库里缺少新增属性")
	}

	// 详情：平台视图仍是未改动的内置模板；按租户视图能读到自己的副本
	platformRow, err := env.client.WithCtx(tenant.Platform(context.Background())).SolutionByKey("", key)
	if err != nil {
		t.Fatalf("平台视图读取内置方案失败: %v", err)
	}
	if containsIdentifier(thingModelIdentifiers(t, platformRow.ThingModelJson, "properties"), "e2e_cavity_pressure") {
		t.Fatal("平台视图的内置方案不应包含租户新增的属性")
	}
	tenantRowFromDB, err := env.client.WithCtx(context.Background()).SolutionByKey(tenantRow.Id, key)
	if err != nil {
		t.Fatalf("租户视图读取方案失败: %v", err)
	}
	if tenantRowFromDB.Id != copyRow.Id ||
		!containsIdentifier(thingModelIdentifiers(t, tenantRowFromDB.ThingModelJson, "properties"), "e2e_cavity_pressure") {
		t.Fatal("租户视图应按 key 返回自己的副本")
	}
}

// solutionRowInDBByBuiltIn 按 key + tenant_id + built_in 精确定位（平台副本与内置行 tenant_id 都是 ”）
func solutionRowInDBByBuiltIn(t *testing.T, env *testEnv, key, tenantId string, builtIn int) (models.Solution, error) {
	t.Helper()
	var row models.Solution
	err := env.client.Pool.Table(models.TableSolution).
		Where("`key` = ? AND tenant_id = ? AND built_in = ?", key, tenantId, builtIn).First(&row).Error
	return row, err
}

// 平台管理员（无租户归属）修改内置方案：生成「平台级覆盖副本」，内置行保持只读。
//
// 语义：built_in=1 的内置行永不原地修改；无租户上下文时改动写入 tenant_id=” 且 built_in=0 的
// 平台级副本，读取按「非内置优先」覆盖内置行 —— 因此界面上的「添加物模型」对平台管理员同样可用。
func TestSolutionThingModelItemPlatformOverrideWithoutTenant(t *testing.T) {
	env := setupTestEnv(t)
	const key = "injection_molding"
	builtInBefore, err := solutionRowInDBByBuiltIn(t, env, key, "", 1)
	if err != nil {
		t.Fatalf("内置方案 %s 未 seed: %v", key, err)
	}

	body := []byte(`{"type":"property","item":{"name":"平台覆盖","identifier":"e2e_platform_override","dataType":"INT","description":"e2e","required":false,"rwFlag":"R"}}`)
	w := doRequest(env.engine, http.MethodPost, "/api/v1/solution/"+key+"/thing-model/item", env.token, body)
	resp := parseResponse(t, w)
	if !resp.Success {
		t.Fatalf("平台管理员修改内置方案应生成平台级副本，实际失败: code=%d msg=%s", resp.ErrorCode, resp.ErrorMsg)
	}
	result, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatalf("unexpected result type %T", resp.Result)
	}
	if builtIn, _ := result["builtIn"].(bool); builtIn {
		t.Fatal("修改内置方案后应返回非内置副本(builtIn=false)")
	}
	if tm, _ := result["thingModelJson"].(string); !containsIdentifier(thingModelIdentifiers(t, tm, "properties"), "e2e_platform_override") {
		t.Fatal("接口返回的物模型缺少新增属性")
	}

	// 内置行绝不被原地修改
	builtInAfter, err := solutionRowInDBByBuiltIn(t, env, key, "", 1)
	if err != nil {
		t.Fatalf("读取内置行失败: %v", err)
	}
	if builtInAfter.ThingModelJson != builtInBefore.ThingModelJson {
		t.Fatal("库中内置行(built_in=1)被原地修改")
	}

	// 平台级副本已生成
	copyRow, err := solutionRowInDBByBuiltIn(t, env, key, "", 0)
	if err != nil {
		t.Fatalf("平台级副本未生成: %v", err)
	}
	if !strings.HasPrefix(copyRow.Id, "sol-"+key+"-") {
		t.Fatalf("平台级副本 id 应为 sol-%s-<随机>，实际 %s", key, copyRow.Id)
	}
	if !containsIdentifier(thingModelIdentifiers(t, copyRow.ThingModelJson, "properties"), "e2e_platform_override") {
		t.Fatal("平台级副本库里缺少新增属性")
	}

	// 平台视图按 key 读到的是覆盖副本（非内置优先），而不是内置行
	row, err := env.client.WithCtx(tenant.Platform(context.Background())).SolutionByKey("", key)
	if err != nil {
		t.Fatalf("平台视图读取方案失败: %v", err)
	}
	if row.Id != copyRow.Id {
		t.Fatalf("平台视图应按 key 返回覆盖副本 %s，实际 %s", copyRow.Id, row.Id)
	}

	// 列表里同 key 只有一条，且为副本
	list, err := env.client.WithCtx(tenant.Platform(context.Background())).SolutionSearch("", "")
	if err != nil {
		t.Fatalf("平台视图列方案失败: %v", err)
	}
	seen := 0
	for _, item := range list {
		if item.Key != key {
			continue
		}
		seen++
		if item.Id != copyRow.Id {
			t.Fatalf("列表中的 %s 应为覆盖副本，实际 %s", key, item.Id)
		}
	}
	if seen != 1 {
		t.Fatalf("列表同 key 应只有 1 条，实际 %d", seen)
	}
}

// type → 数组映射：event→events、action→services，且重复修改不再克隆第二份副本
func TestSolutionThingModelItemTypeMapping(t *testing.T) {
	env := setupTestEnv(t)
	tenantRow, username := seedTenant(t, env, "tsltype")
	token := loginAs(t, env, username, tenantTestPassword)
	const key = "filling_machine"

	event := `{"type":"event","item":{"eventName":"缺料告警","identifier":"e2e_material_lack","eventType":"ALERT","outputData":[],"description":"e2e","required":false}}`
	action := `{"type":"action","item":{"serviceName":"启动清洗","identifier":"e2e_start_clean","callType":"SYNC","inputParams":[],"outputParams":[],"description":"e2e","required":false}}`
	for _, body := range []string{event, action} {
		w := doRequest(env.engine, http.MethodPost, "/api/v1/solution/"+key+"/thing-model/item", token, []byte(body))
		resp := parseResponse(t, w)
		if !resp.Success {
			t.Fatalf("新增 %s 失败: code=%d msg=%s", body[:20], resp.ErrorCode, resp.ErrorMsg)
		}
	}

	copyRow, err := solutionRowInDB(t, env, key, tenantRow.Id)
	if err != nil {
		t.Fatalf("租户副本未生成: %v", err)
	}
	if !containsIdentifier(thingModelIdentifiers(t, copyRow.ThingModelJson, "events"), "e2e_material_lack") {
		t.Fatal("event 应写入 events 数组")
	}
	if !containsIdentifier(thingModelIdentifiers(t, copyRow.ThingModelJson, "services"), "e2e_start_clean") {
		t.Fatal("action 应写入 services 数组")
	}
	if containsIdentifier(thingModelIdentifiers(t, copyRow.ThingModelJson, "properties"), "e2e_material_lack") {
		t.Fatal("event 不应写入 properties 数组")
	}

	var rows int64
	env.client.Pool.Table(models.TableSolution).Where("`key` = ?", key).Count(&rows)
	if rows != 2 {
		t.Fatalf("同一 key 应只有内置行 + 1 份租户副本，实际 %d 行", rows)
	}
}

// 改（按 identifier）与删：只落在租户副本，内置行始终不变
func TestSolutionThingModelItemEditAndDeleteOnlyTouchTenantCopy(t *testing.T) {
	env := setupTestEnv(t)
	tenantRow, username := seedTenant(t, env, "tsledit")
	token := loginAs(t, env, username, tenantTestPassword)
	const key = "refrigeration"

	builtInBefore, err := solutionRowInDB(t, env, key, "")
	if err != nil {
		t.Fatalf("内置方案未 seed: %v", err)
	}
	baseProps := thingModelIdentifiers(t, builtInBefore.ThingModelJson, "properties")

	addBody := `{"type":"property","item":{"name":"回风温度","identifier":"e2e_air_temp","dataType":"FLOAT","rwFlag":"R","required":false}}`
	w := doRequest(env.engine, http.MethodPost, "/api/v1/solution/"+key+"/thing-model/item", token, []byte(addBody))
	if resp := parseResponse(t, w); !resp.Success {
		t.Fatalf("新增失败: code=%d msg=%s", resp.ErrorCode, resp.ErrorMsg)
	}

	// 改：按旧 identifier 定位，同时换 identifier 与数据类型
	putBody := `{"type":"property","identifier":"e2e_air_temp","item":{"name":"回风温度2","identifier":"e2e_air_temp_v2","dataType":"INT","rwFlag":"RW","required":true}}`
	w = doRequest(env.engine, http.MethodPut, "/api/v1/solution/"+key+"/thing-model/item", token, []byte(putBody))
	resp := parseResponse(t, w)
	if !resp.Success {
		t.Fatalf("编辑失败: code=%d msg=%s", resp.ErrorCode, resp.ErrorMsg)
	}
	edited := resp.Result.(map[string]interface{})
	ids := thingModelIdentifiers(t, edited["thingModelJson"].(string), "properties")
	if containsIdentifier(ids, "e2e_air_temp") || !containsIdentifier(ids, "e2e_air_temp_v2") {
		t.Fatalf("编辑后标识符不符: %v", ids)
	}
	if len(ids) != len(baseProps)+1 {
		t.Fatalf("编辑不应增删条目，期望 %d 条，实际 %d", len(baseProps)+1, len(ids))
	}

	// 删
	w = doRequest(env.engine, http.MethodDelete,
		"/api/v1/solution/"+key+"/thing-model/item?type=property&identifier=e2e_air_temp_v2", token, nil)
	resp = parseResponse(t, w)
	if !resp.Success {
		t.Fatalf("删除失败: code=%d msg=%s", resp.ErrorCode, resp.ErrorMsg)
	}
	deleted := resp.Result.(map[string]interface{})
	ids = thingModelIdentifiers(t, deleted["thingModelJson"].(string), "properties")
	if containsIdentifier(ids, "e2e_air_temp_v2") {
		t.Fatal("删除未生效")
	}
	if len(ids) != len(baseProps) {
		t.Fatalf("删除后属性数应回到 %d，实际 %d", len(baseProps), len(ids))
	}

	// 内置行始终未变
	builtInAfter, err := solutionRowInDB(t, env, key, "")
	if err != nil {
		t.Fatalf("读取内置行失败: %v", err)
	}
	if builtInAfter.ThingModelJson != builtInBefore.ThingModelJson {
		t.Fatal("改动泄漏到内置行(tenant_id='')")
	}
	// 改动只落在该租户的副本
	copyRow, err := solutionRowInDB(t, env, key, tenantRow.Id)
	if err != nil {
		t.Fatalf("租户副本丢失: %v", err)
	}
	copyProps := thingModelIdentifiers(t, copyRow.ThingModelJson, "properties")
	if containsIdentifier(copyProps, "e2e_air_temp_v2") || len(copyProps) != len(baseProps) {
		t.Fatalf("改/删未正确落在租户副本: %v", copyProps)
	}
}

// 重复 identifier 新增 → 明确业务错误（不静默覆盖）
func TestSolutionThingModelItemDuplicateIdentifierRejected(t *testing.T) {
	env := setupTestEnv(t)
	tenantRow, username := seedTenant(t, env, "tsldup")
	token := loginAs(t, env, username, tenantTestPassword)
	const key = "compressor_chiller"

	body := []byte(`{"type":"property","item":{"name":"排气压力","identifier":"e2e_dup","dataType":"FLOAT","rwFlag":"R","required":false}}`)
	w := doRequest(env.engine, http.MethodPost, "/api/v1/solution/"+key+"/thing-model/item", token, body)
	if resp := parseResponse(t, w); !resp.Success {
		t.Fatalf("首次新增失败: code=%d msg=%s", resp.ErrorCode, resp.ErrorMsg)
	}

	w = doRequest(env.engine, http.MethodPost, "/api/v1/solution/"+key+"/thing-model/item", token, body)
	resp := parseResponse(t, w)
	if resp.Success {
		t.Fatal("重复 identifier 新增必须报错")
	}
	if resp.ErrorCode != errort.ThingModelCodeExist {
		t.Fatalf("重复 identifier 应返回 %d，实际 %d (%s)", errort.ThingModelCodeExist, resp.ErrorCode, resp.ErrorMsg)
	}
	if strings.TrimSpace(resp.ErrorMsg) == "" {
		t.Fatal("重复 identifier 的错误信息不能为空")
	}

	copyRow, err := solutionRowInDB(t, env, key, tenantRow.Id)
	if err != nil {
		t.Fatalf("租户副本丢失: %v", err)
	}
	count := 0
	for _, id := range thingModelIdentifiers(t, copyRow.ThingModelJson, "properties") {
		if id == "e2e_dup" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("重复标识符应只有 1 份，实际 %d", count)
	}
}

// 非法 type / 缺 identifier / 定位不到条目 → 明确错误且不产生半成品副本
func TestSolutionThingModelItemParamErrors(t *testing.T) {
	env := setupTestEnv(t)
	tenantRow, username := seedTenant(t, env, "tslbad")
	token := loginAs(t, env, username, tenantTestPassword)
	const key = "energy_monitoring"
	itemURL := "/api/v1/solution/" + key + "/thing-model/item"

	badRequests := []struct {
		name   string
		method string
		url    string
		body   []byte
		code   uint32
	}{
		{"非法 type", http.MethodPost, itemURL, []byte(`{"type":"method","item":{"identifier":"e2e_bad"}}`), errort.DefaultReqParamsError},
		{"新增缺 identifier", http.MethodPost, itemURL, []byte(`{"type":"property","item":{"name":"没有标识符"}}`), errort.DefaultReqParamsError},
		{"编辑缺旧 identifier", http.MethodPut, itemURL, []byte(`{"type":"property","item":{"identifier":"e2e_any"}}`), errort.DefaultReqParamsError},
		{"编辑条目缺 identifier", http.MethodPut, itemURL, []byte(`{"type":"property","identifier":"e2e_any","item":{"name":"x"}}`), errort.DefaultReqParamsError},
		{"删除缺 identifier", http.MethodDelete, itemURL + "?type=property", nil, errort.DefaultReqParamsError},
		{"删除非法 type", http.MethodDelete, itemURL + "?type=thing&identifier=x", nil, errort.DefaultReqParamsError},
		{"编辑不存在的 identifier", http.MethodPut, itemURL, []byte(`{"type":"property","identifier":"e2e_missing","item":{"identifier":"e2e_missing_2","name":"x","dataType":"INT"}}`), errort.ThingModelNotExist},
		{"删除不存在的 identifier", http.MethodDelete, itemURL + "?type=property&identifier=e2e_missing", nil, errort.ThingModelNotExist},
	}
	for _, tc := range badRequests {
		w := doRequest(env.engine, tc.method, tc.url, token, tc.body)
		resp := parseResponse(t, w)
		if resp.Success {
			t.Fatalf("%s 应返回错误", tc.name)
		}
		if resp.ErrorCode != tc.code {
			t.Fatalf("%s 应返回错误码 %d，实际 %d (%s)", tc.name, tc.code, resp.ErrorCode, resp.ErrorMsg)
		}
	}

	// 失败请求不应留下租户副本
	if row, err := solutionRowInDB(t, env, key, tenantRow.Id); err == nil {
		t.Fatalf("校验失败的请求不应创建租户副本: %+v", row)
	}
}

// 列表去重：同一 key 只返回一条，租户副本优先；租户自建方案不能漏
func TestSolutionSearchDedupPrefersTenantCopy(t *testing.T) {
	env := setupTestEnv(t)
	tenantA, userA := seedTenant(t, env, "tslsta")
	tokenA := loginAs(t, env, userA, tenantTestPassword)
	_, userB := seedTenant(t, env, "tslstb")
	tokenB := loginAs(t, env, userB, tenantTestPassword)
	const key = "packaging_machine"

	// 租户 A 修改内置方案 → 生成副本
	body := []byte(`{"type":"property","item":{"name":"包装速度","identifier":"e2e_pack_speed","dataType":"FLOAT","rwFlag":"R","required":false}}`)
	w := doRequest(env.engine, http.MethodPost, "/api/v1/solution/"+key+"/thing-model/item", tokenA, body)
	if resp := parseResponse(t, w); !resp.Success {
		t.Fatalf("租户 A 新增属性失败: code=%d msg=%s", resp.ErrorCode, resp.ErrorMsg)
	}
	// 租户 A 的自建方案（无创建接口，直接落库）
	if err := env.client.AddSolution(models.Solution{
		Id: "sol-tenant-own-a", Key: "tenant_own_demo", TenantId: tenantA.Id,
		Name: "租户自建方案", Industry: "自定义", BuiltIn: 0, ThingModelJson: `{"properties":[]}`,
	}); err != nil {
		t.Fatalf("写入租户自建方案失败: %v", err)
	}

	listFor := func(token string) []map[string]interface{} {
		t.Helper()
		w := doRequest(env.engine, http.MethodGet, "/api/v1/solutions", token, nil)
		resp := parseResponse(t, w)
		if !resp.Success {
			t.Fatalf("方案列表失败: %v", resp.ErrorMsg)
		}
		raw, ok := resp.Result.([]interface{})
		if !ok {
			t.Fatalf("unexpected list type %T", resp.Result)
		}
		out := make([]map[string]interface{}, 0, len(raw))
		for _, item := range raw {
			out = append(out, item.(map[string]interface{}))
		}
		return out
	}

	countByKey := func(list []map[string]interface{}) map[string]int {
		counts := map[string]int{}
		for _, tpl := range list {
			k, _ := tpl["key"].(string)
			counts[k]++
		}
		return counts
	}

	// 租户 A：key 不重复，目标 key 返回副本内容，自建方案在列表中
	listA := listFor(tokenA)
	for k, n := range countByKey(listA) {
		if n != 1 {
			t.Fatalf("方案 key %s 在租户 A 列表中重复出现 %d 次", k, n)
		}
	}
	if len(listA) != 16 {
		t.Fatalf("租户 A 应看到 15 个内置方案(其中一个被副本替换) + 1 个自建方案，实际 %d", len(listA))
	}
	var entryA map[string]interface{}
	for _, tpl := range listA {
		if tpl["key"] == key {
			entryA = tpl
		}
	}
	if entryA == nil {
		t.Fatalf("租户 A 列表缺少 %s", key)
	}
	if builtIn, _ := entryA["builtIn"].(bool); builtIn {
		t.Fatal("列表应优先返回租户副本(builtIn=false)")
	}
	if tm, _ := entryA["thingModelJson"].(string); !strings.Contains(tm, "e2e_pack_speed") {
		t.Fatal("列表返回的不是租户副本内容")
	}
	if countByKey(listA)["tenant_own_demo"] != 1 {
		t.Fatal("租户自建方案被漏掉")
	}

	// 租户 B：未修改，仍看到内置模板，且看不到租户 A 的副本与自建方案
	countsB := countByKey(listFor(tokenB))
	if countsB[key] != 1 {
		t.Fatalf("租户 B 列表中 %s 应只有 1 条，实际 %d", key, countsB[key])
	}
	if countsB["tenant_own_demo"] != 0 {
		t.Fatal("租户 B 不应看到租户 A 的自建方案")
	}
	for _, tpl := range listFor(tokenB) {
		if tpl["key"] != key {
			continue
		}
		if builtIn, _ := tpl["builtIn"].(bool); !builtIn {
			t.Fatal("未修改的租户应看到内置模板")
		}
		if tm, _ := tpl["thingModelJson"].(string); strings.Contains(tm, "e2e_pack_speed") {
			t.Fatal("租户 B 不应看到租户 A 的副本内容")
		}
	}
}

// 权限单一来源与种子按钮组必须登记方案物模型的增/改/删
func TestSolutionThingModelPermissionsRegistered(t *testing.T) {
	want := []string{permissions.SolutionTslAdd, permissions.SolutionTslEdit, permissions.SolutionTslRemove}
	declared := map[string]bool{}
	for _, group := range permissions.NamedGroups() {
		for _, item := range group {
			declared[item.Value] = true
		}
	}
	for _, code := range want {
		if !declared[code] {
			t.Fatalf("权限 %s 未登记到 permissions.NamedGroups()", code)
		}
	}
	if middleware.PermSolutionTslAdd != permissions.SolutionTslAdd ||
		middleware.PermSolutionTslEdit != permissions.SolutionTslEdit ||
		middleware.PermSolutionTslRemove != permissions.SolutionTslRemove {
		t.Fatal("middleware 权限别名与 permissions 常量不一致")
	}

	// 方案中心按钮组（perms 前缀 solution）需生成三个按钮权限
	seeded := map[string]bool{}
	for _, menu := range seed.DefaultMenus() {
		seeded[menu.Perms] = true
	}
	for _, code := range want {
		if !seeded[code] {
			t.Fatalf("内置菜单/按钮缺少权限 %s", code)
		}
	}
}
