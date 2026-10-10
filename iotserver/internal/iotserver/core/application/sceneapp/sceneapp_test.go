package sceneapp

import (
	"strings"
	"testing"
	"time"

	"iotserver/internal/pkg/logger"

	"iotserver/internal/models"
)

func TestCronForSchedulerWeekdayMapping(t *testing.T) {
	cases := map[string]string{
		// 前端 0=周一 → 调度器 0=周日, 1=周一
		"01 02 * * 0":             "01 02 * * 1",     // 周一
		"01 02 * * 6":             "01 02 * * 0",     // 周日
		"01 02 * * 0,1,2":         "01 02 * * 1,2,3", // 周一二三
		"01 02 * * 1-5":           "01 02 * * 2-6",   // 周二到周六
		"01 02 * * *":             "01 02 * * *",     // 每天不变
		"01 02 * *":               "01 02 * *",       // 段数不对时不改写
		"01 02 * * 0,1,2,3,4,5,6": "01 02 * * 1,2,3,4,5,6,0",
	}
	for in, want := range cases {
		if got := cronForScheduler(in); got != want {
			t.Fatalf("cronForScheduler(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestParseCron(t *testing.T) {
	// 合法：应给出未来的触发时间（本仓库 Next 的布尔值恒为 false，不能依赖）
	_, next, err := parseCron("01 02 * * 0")
	if err != nil {
		t.Fatalf("合法表达式应解析成功: %v", err)
	}
	if next.IsZero() || !next.After(time.Now().Add(-time.Minute)) {
		t.Fatalf("触发时间异常: %v", next)
	}
	if next.Hour() != 2 || next.Minute() != 1 {
		t.Fatalf("触发时间应为 02:01, got %v", next)
	}
	// 周一是调度器的 1
	if int(next.Weekday()) != 1 {
		t.Fatalf("周一应映射为调度器 dow=1, got weekday=%v", next.Weekday())
	}

	// 非法表达式
	for _, bad := range []string{"not a cron", "99 99 * * *", "01 02 * * 9", ""} {
		if _, _, err := parseCron(bad); err == nil {
			t.Fatalf("非法表达式 %q 应报错", bad)
		}
	}
}

func TestCheckSceneParam(t *testing.T) {
	notify := func(option map[string]string) []models.SceneCondition {
		return []models.SceneCondition{{ConditionType: ConditionTypeNotify, Option: option}}
	}
	timer := func(cron string) []models.SceneCondition {
		return []models.SceneCondition{{ConditionType: ConditionTypeTimer, Option: map[string]string{"cron_expression": cron}}}
	}
	oneAction := []models.SceneAction{{DeviceId: "d1", Code: "switch", Value: "true"}}

	// 合法：设备数据触发
	if err := checkSceneParam(notify(map[string]string{
		"trigger": "设备数据触发", "product_id": "p1", "device_id": "d1",
		"code": "temp", "type": "float", "decide_condition": "> 30",
	}), oneAction); err != nil {
		t.Fatalf("设备数据触发应通过: %v", err)
	}
	// 合法：设备事件触发 / 设备状态触发
	if err := checkSceneParam(notify(map[string]string{
		"trigger": "设备事件触发", "product_id": "p1", "device_id": "d1", "code": "alarm",
	}), oneAction); err != nil {
		t.Fatalf("设备事件触发应通过: %v", err)
	}
	if err := checkSceneParam(notify(map[string]string{
		"trigger": "设备状态触发", "device_id": "d1", "status": "在线",
	}), oneAction); err != nil {
		t.Fatalf("设备状态触发应通过: %v", err)
	}
	// 合法：定时
	if err := checkSceneParam(timer("01 02 * * 0"), oneAction); err != nil {
		t.Fatalf("定时触发应通过: %v", err)
	}

	// 非法分支
	bad := []struct {
		name       string
		conditions []models.SceneCondition
		actions    []models.SceneAction
	}{
		{"无触发条件", nil, oneAction},
		{"无执行动作", timer("01 02 * * 0"), nil},
		{"触发类型非法", []models.SceneCondition{{ConditionType: "xxx"}}, oneAction},
		{"缺少表达式", timer(""), oneAction},
		{"表达式非法", timer("bad cron"), oneAction},
		{"缺少设备", notify(map[string]string{"trigger": "设备数据触发", "product_id": "p1", "code": "temp", "decide_condition": "> 1"}), oneAction},
		{"缺少条件值", notify(map[string]string{"trigger": "设备数据触发", "product_id": "p1", "device_id": "d1", "code": "temp"}), oneAction},
		{"缺少事件编码", notify(map[string]string{"trigger": "设备事件触发", "product_id": "p1", "device_id": "d1"}), oneAction},
		{"缺少状态", notify(map[string]string{"trigger": "设备状态触发", "device_id": "d1"}), oneAction},
		{"触发方式非法", notify(map[string]string{"trigger": "未知"}), oneAction},
		{"动作缺设备", timer("01 02 * * 0"), []models.SceneAction{{Code: "c"}}},
		{"动作缺功能", timer("01 02 * * 0"), []models.SceneAction{{DeviceId: "d1"}}},
	}
	for _, tt := range bad {
		if err := checkSceneParam(tt.conditions, tt.actions); err == nil {
			t.Fatalf("%s 应校验失败", tt.name)
		}
	}
}

// TestCheckSceneParamRejectsSqlInjection 锁住 SQL 越界注入的防护
func TestCheckSceneParamRejectsSqlInjection(t *testing.T) {
	oneAction := []models.SceneAction{{DeviceId: "d1", Code: "switch", Value: "true"}}
	notify := func(option map[string]string) []models.SceneCondition {
		return []models.SceneCondition{{ConditionType: ConditionTypeNotify, Option: option}}
	}
	base := func() map[string]string {
		return map[string]string{"trigger": "设备数据触发", "product_id": "p1", "device_id": "d1",
			"code": "temp", "type": "float", "decide_condition": "> 30"}
	}

	// 条件值里注入 or true（会让过滤条件恒真 → 对总线上所有消息触发）
	bad := base()
	bad["decide_condition"] = "> 1 or true"
	if err := checkSceneParam(notify(bad), oneAction); err == nil {
		t.Fatal("decide_condition 注入 or true 应被拒")
	}
	// 条件值里带引号闭合字符串
	bad = base()
	bad["decide_condition"] = "> 1\" or \"1"
	if err := checkSceneParam(notify(bad), oneAction); err == nil {
		t.Fatal("decide_condition 含引号应被拒")
	}
	// 设备ID 里带引号 + or
	bad = base()
	bad["device_id"] = "d1\" or true or \"x"
	if err := checkSceneParam(notify(bad), oneAction); err == nil {
		t.Fatal("device_id 含引号应被拒")
	}
	// 功能编码里带分号
	bad = base()
	bad["code"] = "temp;drop"
	if err := checkSceneParam(notify(bad), oneAction); err == nil {
		t.Fatal("code 含分号应被拒")
	}
	// 布尔型条件格式
	boolCond := map[string]string{"trigger": "设备数据触发", "product_id": "p1", "device_id": "d1",
		"code": "switch", "type": "bool", "decide_condition": "= true"}
	if err := checkSceneParam(notify(boolCond), oneAction); err != nil {
		t.Fatalf("合法布尔条件应通过: %v", err)
	}
	boolCond["decide_condition"] = "= 1 or true"
	if err := checkSceneParam(notify(boolCond), oneAction); err == nil {
		t.Fatal("布尔条件非法格式应被拒")
	}
	// 文本型合法值（中文/空格）
	textCond := map[string]string{"trigger": "设备数据触发", "product_id": "p1", "device_id": "d1",
		"code": "mode", "type": "text", "decide_condition": "= 自动 模式"}
	if err := checkSceneParam(notify(textCond), oneAction); err != nil {
		t.Fatalf("合法文本条件应通过: %v", err)
	}
	textCond["decide_condition"] = "= a\" or \"b"
	if err := checkSceneParam(notify(textCond), oneAction); err == nil {
		t.Fatal("文本条件含引号应被拒")
	}
	// 超长字段
	long := base()
	long["device_id"] = strings.Repeat("a", 100)
	if err := checkSceneParam(notify(long), oneAction); err == nil {
		t.Fatal("超长 device_id 应被拒")
	}
	// 动作值超长
	bigAction := []models.SceneAction{{DeviceId: "d1", Code: "c", Value: strings.Repeat("v", 300)}}
	if err := checkSceneParam(notify(base()), bigAction); err == nil {
		t.Fatal("超长动作值应被拒")
	}
}

func TestCheckDescription(t *testing.T) {
	if err := checkDescription(strings.Repeat("描", 255)); err != nil {
		t.Fatalf("255 字描述应通过: %v", err)
	}
	if err := checkDescription(strings.Repeat("描", 256)); err == nil {
		t.Fatal("256 字描述应被拒（模型列宽 255）")
	}
}

func TestIsTimerScene(t *testing.T) {
	if isTimerScene(nil) {
		t.Fatal("空条件不应判定为定时")
	}
	if !isTimerScene([]models.SceneCondition{{ConditionType: ConditionTypeTimer}}) {
		t.Fatal("定时条件应判定为定时")
	}
	if isTimerScene([]models.SceneCondition{{ConditionType: ConditionTypeNotify}}) {
		t.Fatal("设备触发不应判定为定时")
	}
}

// TestCollectDue 调度核心逻辑：到期任务被取出且推进到下次；未到期不触发。
// 调度主循环是 1s ticker + 分钟级 cron，无法在测试里等真实到期，因此直接注入 next。
func TestCollectDue(t *testing.T) {
	app := &sceneApp{timers: map[string]*timerEntry{}, lc: logger.NewClient("scene-test", "ERROR", "")}
	scene := &models.Scene{Id: "s1", Name: "定时场景",
		Conditions: models.SceneConditions{{ConditionType: ConditionTypeTimer,
			Option: map[string]string{"cron_expression": "30 2 * * 0"}}},
		Actions: models.SceneActions{{DeviceId: "d1", Code: "switch", Value: "true"}}}
	if err := app.registerTimer(scene); err != nil {
		t.Fatalf("registerTimer: %v", err)
	}

	// 未到期：不该触发
	if due := app.collectDue(time.Now()); len(due) != 0 {
		t.Fatalf("未到期不应触发, got %v", due)
	}
	// 人为把 next 拨到过去 → 应触发，并推进到未来
	app.timers["s1"].next = time.Now().Add(-time.Second)
	due := app.collectDue(time.Now())
	if len(due) != 1 || due[0] != "s1" {
		t.Fatalf("到期应返回 s1, got %v", due)
	}
	if !app.timers["s1"].next.After(time.Now()) {
		t.Fatalf("触发后 next 应推进到未来, got %v", app.timers["s1"].next)
	}
	// 紧接着再扫一次不应重复触发
	if due = app.collectDue(time.Now()); len(due) != 0 {
		t.Fatalf("同一时刻不应重复触发, got %v", due)
	}
	// 停止（注销）后不再触发
	app.unregisterTimer("s1")
	if len(app.timers) != 0 {
		t.Fatal("注销后不应残留定时任务")
	}
}
