package seed

import (
	"encoding/json"
	"strings"
	"testing"

	"iotserver/internal/dtos"
)

// TestDefaultSolutions 内置工业模板必须能被「一键建产品」链路消费。
//
// 关键断言：物模型的 rwFlag / eventType 必须被归一化成非空的 AccessMode / EventType。
// 模板文件用的是短写法（R/RW/W、ALERT/INFO/ERROR），而归一化函数同时兼容
// 云端模板的历史长写法（READ_ONLY、ALERT_EVENT_TYPE）——这里锁住该兼容性。
func TestDefaultSolutions(t *testing.T) {
	templates, categories, thingModels, err := DefaultSolutions()
	if err != nil {
		t.Fatalf("加载内置模板失败: %v", err)
	}
	wantKeys := map[string]bool{
		"refrigeration": false, "compressor_chiller": false, "filling_machine": false,
		"packaging_machine": false, "injection_molding": false, "production_line": false,
		"energy_monitoring": false,
		// 每个行业补充的第二个产品
		"quick_freezer": false, "cooling_tower": false, "pv_inverter": false,
		"labeling_machine": false, "extruder": false, "palletizer": false, "cnc_machine": false,
		// 参考系统网关产品模板（系统状态/网络/SIM/定位）
		"industrial_gateway": false,
	}
	if len(templates) < len(wantKeys) {
		t.Fatalf("内置模板至少应有 %d 个，实际 %d", len(wantKeys), len(templates))
	}
	if len(categories) != len(templates) || len(thingModels) != len(templates) {
		t.Fatalf("品类/物模型模板数量应与方案一致：%d/%d/%d",
			len(templates), len(categories), len(thingModels))
	}

	for _, tpl := range templates {
		if _, ok := wantKeys[tpl.Key]; ok {
			wantKeys[tpl.Key] = true
		}

		if tpl.BuiltIn != 1 {
			t.Fatalf("模板 %s 应标记为内置", tpl.Key)
		}
		if tpl.TenantId != "" {
			t.Fatalf("模板 %s 是平台内置，tenant_id 应为空", tpl.Key)
		}
		if strings.TrimSpace(tpl.Industry) == "" || strings.TrimSpace(tpl.Name) == "" {
			t.Fatalf("模板 %s 缺少行业或名称", tpl.Key)
		}

		// 物模型解析：属性/事件/动作都要有，且读写标志与事件类型非空
		props, events, actions := dtos.GetModelPropertyEventActionByThingModelTemplate(tpl.ThingModelJson)
		if len(props) == 0 {
			t.Fatalf("模板 %s 未解析出属性", tpl.Key)
		}
		for _, p := range props {
			if p.AccessMode == "" {
				t.Fatalf("模板 %s 的属性 %s 的 AccessMode 为空（rwFlag 归一化失败）", tpl.Key, p.Code)
			}
			if p.Code == "" || p.Name == "" {
				t.Fatalf("模板 %s 存在缺少 code/name 的属性", tpl.Key)
			}
		}
		if len(events) == 0 {
			t.Fatalf("模板 %s 未解析出事件", tpl.Key)
		}
		for _, e := range events {
			if e.EventType == "" {
				t.Fatalf("模板 %s 的事件 %s 的 EventType 为空（eventType 归一化失败）", tpl.Key, e.Code)
			}
		}
		if len(actions) == 0 {
			t.Fatalf("模板 %s 未解析出控制命令（services）", tpl.Key)
		}

		// 告警预设必须是合法 JSON 数组且非空
		var alarms []SolutionAlarm
		if err := json.Unmarshal([]byte(tpl.AlarmRulesJson), &alarms); err != nil {
			t.Fatalf("模板 %s 的告警预设非法: %v", tpl.Key, err)
		}
		if len(alarms) == 0 {
			t.Fatalf("模板 %s 没有告警预设", tpl.Key)
		}
		propCodes := map[string]bool{}
		for _, p := range props {
			propCodes[p.Code] = true
		}
		for _, a := range alarms {
			if !propCodes[a.PropertyCode] {
				t.Fatalf("模板 %s 的告警规则 %s 引用了不存在的属性 %s", tpl.Key, a.Name, a.PropertyCode)
			}
			switch a.Operator {
			case ">", ">=", "<", "<=", "=", "!=":
			default:
				t.Fatalf("模板 %s 的告警规则 %s 操作符非法: %s", tpl.Key, a.Name, a.Operator)
			}
			if a.AlertLevel < 1 || a.AlertLevel > 4 {
				t.Fatalf("模板 %s 的告警规则 %s 级别非法: %d", tpl.Key, a.Name, a.AlertLevel)
			}
		}

		// 看板预设：至少要有组件，且组件引用的属性存在
		var dash struct {
			Name    string `json:"name"`
			Widgets []struct {
				Title        string `json:"title"`
				Type         string `json:"type"`
				PropertyCode string `json:"propertyCode"`
			} `json:"widgets"`
		}
		if err := json.Unmarshal([]byte(tpl.DashboardJson), &dash); err != nil {
			t.Fatalf("模板 %s 的看板预设非法: %v", tpl.Key, err)
		}
		if len(dash.Widgets) == 0 {
			t.Fatalf("模板 %s 的看板没有组件", tpl.Key)
		}
		for _, w := range dash.Widgets {
			switch w.Type {
			case "line", "kpi", "gauge", "table", "text":
			default:
				t.Fatalf("模板 %s 的看板组件 %s 类型非法: %s", tpl.Key, w.Title, w.Type)
			}
			if w.PropertyCode != "" && !propCodes[w.PropertyCode] {
				t.Fatalf("模板 %s 的看板组件 %s 引用了不存在的属性 %s", tpl.Key, w.Title, w.PropertyCode)
			}
		}
	}

	for k, seen := range wantKeys {
		if !seen {
			t.Fatalf("缺少模板: %s", k)
		}
	}

	// 品类 key 必须带前缀且与物模型模板一一对应（前端「一键建产品」按 category_key 关联）
	for i, c := range categories {
		if !strings.HasPrefix(c.CategoryKey, CategoryTemplateKeyPrefix) {
			t.Fatalf("品类 key %s 缺少前缀 %s", c.CategoryKey, CategoryTemplateKeyPrefix)
		}
		if c.Scene == "" || c.Scene == "公共场景" {
			t.Fatalf("品类 %s 的 scene 应该是行业名，实际 %q", c.CategoryKey, c.Scene)
		}
		if thingModels[i].CategoryKey != c.CategoryKey {
			t.Fatalf("品类 %s 与物模型模板的 category_key 不一致", c.CategoryKey)
		}
	}
}
