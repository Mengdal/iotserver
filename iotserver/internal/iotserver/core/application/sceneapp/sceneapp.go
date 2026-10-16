package sceneapp

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"iotserver/internal/dtos"
	"iotserver/internal/iotserver/core/container"
	interfaces "iotserver/internal/iotserver/core/interface"
	"iotserver/internal/models"
	"iotserver/internal/pkg/constants"
	pkgcontainer "iotserver/internal/pkg/container"
	"iotserver/internal/pkg/di"
	"iotserver/internal/pkg/errort"
	"iotserver/internal/pkg/logger"
	"iotserver/internal/pkg/tenant"
	"iotserver/internal/pkg/timer/jobs"
	"iotserver/internal/pkg/valid"
	"iotserver/internal/tools/ekuiperclient"
)

const (
	// ConditionTypeNotify 设备触发（下发 eKuiper 规则）
	ConditionTypeNotify = "notify"
	// ConditionTypeTimer 定时触发（平台内置调度器执行）
	ConditionTypeTimer = "timer"

	timerTick     = time.Second
	sceneRunnerId = "iotserver-scene"
	// resyncTimeout 启动后重下发动作的整体超时
	resyncTimeout = 30 * time.Second
)

// timerEntry 定时场景调度项
type timerEntry struct {
	schedule *jobs.JobSchedule
	next     time.Time
}

type sceneApp struct {
	dic      *di.Container
	dbClient interfaces.DBClient
	lc       logger.LoggingClient

	timerMu sync.Mutex
	timers  map[string]*timerEntry
}

func New(dic *di.Container) *sceneApp {
	app := &sceneApp{
		dic:      dic,
		dbClient: container.DBClientFrom(dic.Get),
		lc:       pkgcontainer.LoggingClientFrom(dic.Get),
		timers:   map[string]*timerEntry{},
	}
	go app.startTimerScheduler()
	// 启动后一次性重下发运行中场景的动作（回调密钥等配置变更后自愈）
	go app.resyncRunningSceneActions()
	return app
}

// NewWithoutScheduler 供测试使用：不启动定时调度与状态恢复
func NewWithoutScheduler(dic *di.Container) *sceneApp {
	return &sceneApp{
		dic:      dic,
		dbClient: container.DBClientFrom(dic.Get),
		lc:       pkgcontainer.LoggingClientFrom(dic.Get),
		timers:   map[string]*timerEntry{},
	}
}

/** ***************************** 查询 ***************************** **/

// SceneSearch 场景列表（前端编辑抽屉直接吃列表行数据，这里必须返回完整的 conditions/actions）
func (app *sceneApp) SceneSearch(ctx context.Context, req dtos.SceneSearchQueryRequest) ([]dtos.SceneSearchQueryResponse, uint32, error) {
	offset, limit := dtos.OffsetLimit(req.BaseSearchConditionQuery)
	list, total, err := app.dbClient.WithCtx(ctx).SceneSearch(offset, limit, req)
	if err != nil {
		return nil, 0, err
	}
	out := make([]dtos.SceneSearchQueryResponse, 0, len(list))
	for i := range list {
		out = append(out, toResponse(&list[i]))
	}
	return out, total, nil
}

// SceneLog 场景联动日志（按场景 + 时间范围分页，倒序）
func (app *sceneApp) SceneLog(ctx context.Context, sceneId string, req dtos.SceneLogSearchQueryRequest) ([]dtos.SceneLogResponse, uint32, error) {
	if _, err := app.dbClient.WithCtx(ctx).SceneById(sceneId); err != nil {
		return nil, 0, err
	}
	offset, limit := dtos.OffsetLimit(req.BaseSearchConditionQuery)
	req.SceneId = sceneId
	logs, total, err := app.dbClient.WithCtx(ctx).SceneLogSearch(offset, limit, req)
	if err != nil {
		return nil, 0, err
	}
	out := make([]dtos.SceneLogResponse, 0, len(logs))
	for i := range logs {
		out = append(out, dtos.SceneLogResponse{
			Id:          logs[i].Id,
			Name:        logs[i].Name,
			SceneId:     logs[i].SceneId,
			ExecRes:     logs[i].ExecRes,
			Success:     logs[i].Success,
			TriggerTime: logs[i].TriggerTime,
			Created:     logs[i].Created,
		})
	}
	return out, total, nil
}

/** ***************************** 增删改 ***************************** **/

// SceneAdd 新建场景：只落库（前端建完再编辑配置条件与动作）
func (app *sceneApp) SceneAdd(ctx context.Context, req dtos.SceneAddRequest) error {
	if !valid.Name(req.Name) {
		return errort.NewCommonErr(errort.DefaultNameSpecialCharacters, fmt.Errorf("%s", valid.NameRuleMsg))
	}
	if err := checkDescription(req.Description); err != nil {
		return err
	}
	if _, err := app.dbClient.WithCtx(ctx).AddScene(models.Scene{
		Name:     req.Name,
		Describe: req.Description,
		Status:   string(constants.SceneStop),
	}); err != nil {
		return err
	}
	return nil
}

// SceneUpdate 编辑场景：校验条件与动作，必要时下发/更新 eKuiper 规则
func (app *sceneApp) SceneUpdate(ctx context.Context, req dtos.SceneUpdateRequest) error {
	if strings.TrimSpace(req.Id) == "" {
		return errort.NewCommonErr(errort.DefaultIdEmpty, fmt.Errorf("id is empty"))
	}
	old, err := app.dbClient.WithCtx(ctx).SceneById(req.Id)
	if err != nil {
		return err
	}
	// 运行中的场景先停止再修改
	if old.Status == string(constants.SceneStart) {
		return errort.NewCommonErr(errort.SceneTimerIsStartingNotAllowUpdate, nil)
	}
	if !valid.Name(req.Name) {
		return errort.NewCommonErr(errort.DefaultNameSpecialCharacters, fmt.Errorf("%s", valid.NameRuleMsg))
	}
	if err = checkDescription(req.Description); err != nil {
		return err
	}
	if err = checkSceneParam(req.Conditions, req.Actions); err != nil {
		return err
	}

	if isTimerScene(req.Conditions) {
		// 定时场景由平台调度器执行，不产生 eKuiper 规则；若历史上有规则则清掉
		if err = app.deleteEkuiperRuleIfExist(ctx, old.Id); err != nil {
			return err
		}
	} else {
		sql := app.buildSql(req.Conditions)
		if sql == "" {
			return errort.NewCommonErr(errort.SceneRuleParamsError, fmt.Errorf("触发条件无法生成查询语句"))
		}
		if err = app.syncEkuiperRule(ctx, old.Id, sql); err != nil {
			return err
		}
	}

	return app.dbClient.WithCtx(ctx).UpdateScene(models.Scene{
		Id:         old.Id,
		Name:       req.Name,
		Describe:   req.Description,
		Conditions: models.SceneConditions(req.Conditions),
		Actions:    models.SceneActions(req.Actions),
	})
}

// SceneDelete 删除场景：清理 eKuiper 规则、定时任务与日志
func (app *sceneApp) SceneDelete(ctx context.Context, id string) error {
	scene, err := app.dbClient.WithCtx(ctx).SceneById(id)
	if err != nil {
		return err
	}
	if err = app.deleteEkuiperRuleIfExist(ctx, scene.Id); err != nil {
		return err
	}
	app.unregisterTimer(scene.Id)
	if err = app.dbClient.WithCtx(ctx).DeleteScene(id); err != nil {
		return err
	}
	// 日志随场景一起清理，避免残留孤儿数据
	if err = app.dbClient.WithCtx(ctx).DeleteSceneLogsBySceneId(id); err != nil {
		app.lc.Warnf("delete scene %s logs failed: %v", id, err)
	}
	return nil
}

/** ***************************** 启停 ***************************** **/

// SceneStart 启动场景
func (app *sceneApp) SceneStart(ctx context.Context, id string) error {
	scene, err := app.dbClient.WithCtx(ctx).SceneById(id)
	if err != nil {
		return err
	}
	if len(scene.Conditions) == 0 {
		return errort.NewCommonErr(errort.SceneRuleParamsError, fmt.Errorf("请先配置触发条件"))
	}
	if len(scene.Actions) == 0 {
		return errort.NewCommonErr(errort.SceneRuleParamsError, fmt.Errorf("请先配置执行动作"))
	}
	// 库中数据可能早于加固写入（或被人直接改库），启动前再校验一次
	if err := checkSceneParam(scene.Conditions, scene.Actions); err != nil {
		return err
	}

	if isTimerScene(scene.Conditions) {
		if err = app.registerTimer(&scene); err != nil {
			return err
		}
	} else {
		sql := app.buildSql(scene.Conditions)
		if sql == "" {
			return errort.NewCommonErr(errort.SceneRuleParamsError, fmt.Errorf("触发条件无法生成查询语句"))
		}
		if err = app.syncEkuiperRule(ctx, scene.Id, sql); err != nil {
			return err
		}
		if err = app.ekuiper().StartRule(ctx, scene.Id); err != nil {
			return err
		}
	}
	return app.dbClient.WithCtx(ctx).UpdateSceneStatus(scene.Id, string(constants.SceneStart))
}

// SceneStop 停止场景
func (app *sceneApp) SceneStop(ctx context.Context, id string) error {
	scene, err := app.dbClient.WithCtx(ctx).SceneById(id)
	if err != nil {
		return err
	}
	app.unregisterTimer(scene.Id)
	if err = app.stopEkuiperRuleIfExist(ctx, scene.Id); err != nil {
		return err
	}
	return app.dbClient.WithCtx(ctx).UpdateSceneStatus(scene.Id, string(constants.SceneStop))
}

/** ***************************** 触发执行 ***************************** **/

// ExecuteSceneByRule 供 eKuiper 回调：按规则ID找到场景并执行动作
func (app *sceneApp) ExecuteSceneByRule(ctx context.Context, payload map[string]interface{}) error {
	ruleId := stringOf(payload, "rule_id")
	if ruleId == "" {
		ruleId = stringOf(payload, "ruleId")
	}
	if ruleId == "" {
		return errort.NewCommonErr(errort.DefaultReqParamsError, fmt.Errorf("缺少 rule_id"))
	}
	scene, err := app.dbClient.WithCtx(ctx).SceneById(ruleId)
	if err != nil {
		return err
	}
	// 已停止的场景不得执行动作：回调接口免鉴权，若不校验启用状态，
	// 拿到场景ID即可反复触发设备写入
	if scene.Status != string(constants.SceneStart) {
		app.lc.Warnf("scene %s(%s) is not running, trigger ignored", scene.Name, scene.Id)
		return errort.NewCommonErr(errort.SceneRuleParamsError, fmt.Errorf("场景未启用，忽略触发"))
	}
	app.runSceneActions(ctx, &scene)
	return nil
}

// ExecuteScene 定时触发：按场景ID执行动作（并在执行前复核启用状态，
// 避免「已到期但在途被停止」的场景仍然写设备）
func (app *sceneApp) ExecuteScene(ctx context.Context, sceneId string) {
	scene, err := app.dbClient.WithCtx(ctx).SceneById(sceneId)
	if err != nil {
		app.lc.Warnf("timer scene %s not found: %v", sceneId, err)
		return
	}
	if scene.Status != string(constants.SceneStart) {
		app.lc.Infof("timer scene %s is not running, skip", sceneId)
		return
	}
	app.runSceneActions(ctx, &scene)
}

// safeExecuteScene 定时任务的 goroutine 入口：兜住 panic，避免打挂整个进程
func (app *sceneApp) safeExecuteScene(sceneId string) {
	defer func() {
		if r := recover(); r != nil {
			app.lc.Errorf("scene %s timer execution panic: %v", sceneId, r)
		}
	}()
	app.ExecuteScene(context.Background(), sceneId)
}

// runSceneActions 依次执行场景动作并落日志
func (app *sceneApp) runSceneActions(ctx context.Context, scene *models.Scene) {
	deviceApp := container.DeviceItfFrom(app.dic.Get)

	results := make([]string, 0, len(scene.Actions))
	allOk := true
	for i := range scene.Actions {
		action := scene.Actions[i]
		err := deviceApp.SetDeviceProperty(dtos.OpenApiSetDeviceThingModel{
			DeviceId: action.DeviceId,
			Item:     map[string]interface{}{action.Code: action.Value},
		})
		if err != nil {
			allOk = false
			results = append(results, fmt.Sprintf("%s.%s=%s 执行失败(%v)", action.DeviceName, action.Code, action.Value, err))
			app.lc.Warnf("scene %s action failed: %v", scene.Name, err)
			continue
		}
		results = append(results, fmt.Sprintf("%s.%s=%s 执行成功", action.DeviceName, action.Code, action.Value))
	}

	execRes := strings.Join(results, "; ")
	if execRes == "" {
		execRes = "无执行动作"
		allOk = false
	}
	if len([]rune(execRes)) > maxExecResLen {
		execRes = string([]rune(execRes)[:maxExecResLen]) + "..."
	}
	if err := app.dbClient.WithCtx(ctx).AddSceneLog(models.SceneLog{
		Name:        scene.Name,
		SceneId:     scene.Id,
		ExecRes:     execRes,
		Success:     allOk,
		TriggerTime: time.Now().UnixMilli(),
	}); err != nil {
		app.lc.Errorf("scene %s write log failed: %v", scene.Name, err)
	}
}

/** ***************************** 定时调度 ***************************** **/

func (app *sceneApp) startTimerScheduler() {
	// 服务重启后恢复运行中的定时场景
	// 平台作用域：系统路径，不按租户过滤
	scenes, err := app.dbClient.WithCtx(tenant.Platform(context.Background())).SceneRunningTimers()
	if err != nil {
		app.lc.Warnf("load running scenes failed: %v", err)
	} else {
		for i := range scenes {
			if !isTimerScene(scenes[i].Conditions) {
				continue
			}
			if err := app.registerTimer(&scenes[i]); err != nil {
				app.lc.Warnf("restore timer scene %s failed: %v", scenes[i].Name, err)
			}
		}
	}

	ticker := time.NewTicker(timerTick)
	defer ticker.Stop()
	for range ticker.C {
		func() {
			defer func() {
				if r := recover(); r != nil {
					app.lc.Errorf("scene timer scheduler panic: %v", r)
				}
			}()
			app.tickTimers()
		}()
	}
}

// tickTimers 扫描到期任务并触发（抽出来便于加 recover 与测试）
func (app *sceneApp) tickTimers() {
	for _, id := range app.collectDue(time.Now()) {
		go app.safeExecuteScene(id)
	}
}

// collectDue 返回本次到期的场景ID，并推进其下次触发时间。
// 抽成独立函数便于单测（调度主循环是 1s ticker，无法在测试里等真实 cron 到期）。
func (app *sceneApp) collectDue(now time.Time) []string {
	due := make([]string, 0, 2)
	app.timerMu.Lock()
	defer app.timerMu.Unlock()
	for id, entry := range app.timers {
		if !now.Before(entry.next) {
			due = append(due, id)
			// 先算出下次时间，避免执行耗时导致重复触发
			// （Next 的第二个返回值在本仓库实现里恒为 false，只判断零值）
			if next, _ := entry.schedule.Next(now); !next.IsZero() {
				entry.next = next
			} else {
				delete(app.timers, id)
			}
		}
	}
	return due
}

// cronForScheduler 把前端的星期编码转换为调度器编码。
// 前端 SelectWeekDays：0=周一 … 6=周日；Go cron 的 dow：0=周日, 1=周一 … 6=周六。
func cronForScheduler(cron string) string {
	fields := strings.Fields(cron)
	if len(fields) != 5 {
		return cron
	}
	dow := fields[4]
	if dow == "*" || dow == "?" || dow == "" {
		return cron
	}
	parts := strings.Split(dow, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if seg := strings.SplitN(part, "-", 2); len(seg) == 2 {
			a, errA := strconv.Atoi(seg[0])
			b, errB := strconv.Atoi(seg[1])
			// 越界值原样保留，交给解析器报错，避免取模后静默变成合法值
			if errA != nil || errB != nil || a < 0 || a > 6 || b < 0 || b > 6 {
				out = append(out, part)
				continue
			}
			out = append(out, fmt.Sprintf("%d-%d", (a+1)%7, (b+1)%7))
			continue
		}
		n, err := strconv.Atoi(part)
		if err != nil || n < 0 || n > 6 {
			out = append(out, part)
			continue
		}
		out = append(out, strconv.Itoa((n+1)%7))
	}
	fields[4] = strings.Join(out, ",")
	return strings.Join(fields, " ")
}

// parseCron 解析定时表达式并按调度器约定返回下一次触发时间。
// 注意：本仓库 cron 实现的 Next 第二个返回值恒为 false（schedule.go 末尾硬编码），
// 因此只以返回时间是否为零值作为「无法匹配」的判断。
func parseCron(cron string) (*jobs.JobSchedule, time.Time, error) {
	schedule, err := jobs.NewJobSchedule(&jobs.RuntimeJobStu{
		TimeData: jobs.TimeData{Expression: cronForScheduler(cron)},
	})
	if err != nil {
		return nil, time.Time{}, errort.NewCommonErr(errort.SceneRuleParamsError, fmt.Errorf("定时表达式不合法: %v", err))
	}
	next, _ := schedule.Next(time.Now())
	if next.IsZero() {
		return nil, time.Time{}, errort.NewCommonErr(errort.SceneRuleParamsError, fmt.Errorf("定时表达式无法匹配任何时间"))
	}
	return schedule, next, nil
}

// validateCron 校验定时表达式
func validateCron(cron string) error {
	_, _, err := parseCron(cron)
	return err
}

// registerTimer 注册/更新定时场景
func (app *sceneApp) registerTimer(scene *models.Scene) error {
	cron := cronOf(scene.Conditions)
	if cron == "" {
		return errort.NewCommonErr(errort.SceneRuleParamsError, fmt.Errorf("缺少定时表达式"))
	}
	schedule, next, err := parseCron(cron)
	if err != nil {
		return err
	}
	app.timerMu.Lock()
	app.timers[scene.Id] = &timerEntry{schedule: schedule, next: next}
	app.timerMu.Unlock()
	app.lc.Infof("scene %s timer registered, next fire at %s", scene.Name, next.Format(time.RFC3339))
	return nil
}

func (app *sceneApp) unregisterTimer(sceneId string) {
	app.timerMu.Lock()
	delete(app.timers, sceneId)
	app.timerMu.Unlock()
}

/** ***************************** eKuiper ***************************** **/

// resyncRunningSceneActions 启动后把运行中「设备触发」场景的 eKuiper 动作重新下发一次
func (app *sceneApp) resyncRunningSceneActions() {
	defer func() {
		if r := recover(); r != nil {
			app.lc.Errorf("resync scene actions panic: %v", r)
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), resyncTimeout)
	defer cancel()

	// 平台作用域：系统路径，不按租户过滤
	scenes, err := app.dbClient.WithCtx(tenant.Platform(context.Background())).SceneRunningTimers()
	if err != nil {
		app.lc.Warnf("resync scene actions: query failed: %v", err)
		return
	}
	configapp := container.ConfigurationFrom(app.dic.Get)
	actions := dtos.GetRuleSceneEkuiperActions(configapp.Service.CallbackBaseUrl())
	ekuiper := app.ekuiper()
	for i := range scenes {
		scene := scenes[i]
		if isTimerScene(scene.Conditions) {
			continue // 定时场景由平台调度，不涉及 eKuiper 动作
		}
		sql := app.buildSql(scene.Conditions)
		if sql == "" {
			continue
		}
		exist, err := ekuiper.RuleExist(ctx, scene.Id)
		if err != nil || !exist {
			continue
		}
		// UpdateRule 的动作模板 triggered=false 会停掉运行中的规则，下发后按原状态恢复
		wasRunning := app.isSceneRuleRunning(ctx, scene.Id)
		if err = ekuiper.UpdateRule(ctx, actions, scene.Id, sql); err != nil {
			app.lc.Warnf("resync scene %s actions failed: %v", scene.Id, err)
			continue
		}
		if wasRunning {
			if err = ekuiper.StartRule(ctx, scene.Id); err != nil {
				app.lc.Warnf("resync scene %s restart failed: %v", scene.Id, err)
				continue
			}
		}
		app.lc.Infof("scene %s actions resynced (running=%v)", scene.Id, wasRunning)
	}
}

// isSceneRuleRunning 查询 eKuiper 侧场景规则是否在运行
func (app *sceneApp) isSceneRuleRunning(ctx context.Context, ruleId string) bool {
	stats, err := app.ekuiper().GetRuleStats(ctx, ruleId)
	if err != nil {
		return false
	}
	status, _ := stats["status"].(string)
	return status == string(constants.SceneStart)
}

func (app *sceneApp) ekuiper() ekuiperclient.EkuiperClient {
	return container.EkuiperAppFrom(app.dic.Get)
}

// syncEkuiperRule 创建或更新场景对应的 eKuiper 规则（动作回到平台 /api/v1/ekuiper/scene）
func (app *sceneApp) syncEkuiperRule(ctx context.Context, sceneId, sql string) error {
	configapp := container.ConfigurationFrom(app.dic.Get)
	actions := dtos.GetRuleSceneEkuiperActions(configapp.Service.CallbackBaseUrl())
	ekuiper := app.ekuiper()
	exist, err := ekuiper.RuleExist(ctx, sceneId)
	if err != nil {
		return err
	}
	if exist {
		return ekuiper.UpdateRule(ctx, actions, sceneId, sql)
	}
	return ekuiper.CreateRule(ctx, actions, sceneId, sql)
}

func (app *sceneApp) deleteEkuiperRuleIfExist(ctx context.Context, sceneId string) error {
	ekuiper := app.ekuiper()
	exist, err := ekuiper.RuleExist(ctx, sceneId)
	if err != nil {
		return err
	}
	if exist {
		return ekuiper.DeleteRule(ctx, sceneId)
	}
	return nil
}

func (app *sceneApp) stopEkuiperRuleIfExist(ctx context.Context, sceneId string) error {
	ekuiper := app.ekuiper()
	exist, err := ekuiper.RuleExist(ctx, sceneId)
	if err != nil {
		return err
	}
	if exist {
		return ekuiper.StopRule(ctx, sceneId)
	}
	return nil
}

/** ***************************** 内部 ***************************** **/

// buildSql 由触发条件生成 eKuiper SQL（复用告警规则的模板：属性/周期/事件/状态）
func (app *sceneApp) buildSql(conditions []models.SceneCondition) string {
	if len(conditions) == 0 {
		return ""
	}
	option := conditions[0].Option
	trigger := option["trigger"]

	switch constants.Trigger(trigger) {
	case constants.DeviceDataTrigger:
		specsType := constants.SpecsType(option["type"])
		req := &dtos.RuleUpdateRequest{
			SubRule: []dtos.SubRule{{
				Trigger:   constants.DeviceDataTrigger,
				ProductId: option["product_id"],
				DeviceId:  option["device_id"],
				Option:    option,
			}},
		}
		return req.BuildEkuiperSql(option["device_id"], specsType)
	case constants.DeviceEventTrigger:
		return fmt.Sprintf(`SELECT rule_id(),json_path_query(data, "$.eventTime") as report_time,deviceId FROM mqtt_stream where deviceId = "%s" and messageType = "EVENT_REPORT" and json_path_exists(data, "$.eventCode") = true and json_path_query(data, "$.eventCode") = "%s"`,
			option["device_id"], option["code"])
	case constants.DeviceStatusTrigger:
		status := constants.DeviceOnline
		if option["status"] == "离线" {
			status = constants.DeviceOffline
		}
		return fmt.Sprintf(`SELECT rule_id(),json_path_query(data, "$.time") as report_time,deviceId FROM mqtt_stream where deviceId = "%s" and messageType = "DEVICE_STATUS" and json_path_exists(data, "$.status") = true and json_path_query(data, "$.status") = "%s"`,
			option["device_id"], status)
	default:
		return ""
	}
}

// 长度上限（与模型列宽/DB 严格模式对齐，避免写爆列或拖垮 eKuiper）
const (
	maxDescriptionLen = 255
	maxConditionField = 255
	maxCodeLen        = 64
	maxIdLen          = 64
	maxExecResLen     = 500
)

// 数值条件的合法格式：操作符 + 数字
var numericConditionPattern = regexp.MustCompile(`^(>=|<=|!=|>|<|=)\s*-?\d+(\.\d+)?$`)

// 文本/枚举条件：= 值（值里禁止引号/分号/反斜杠，避免越出 eKuiper SQL 的字符串字面量）
var textConditionPattern = regexp.MustCompile(`^=\s*[^"';\\\r\n]{1,128}$`)

// 会被拼进 eKuiper SQL 的标识类字段：禁止引号、分号与控制字符
var unsafeSqlChars = regexp.MustCompile("[\"'`;\\\r\n\t]")

// checkSqlSafeField 校验会被拼接进 eKuiper SQL 的标识字段
func checkSqlSafeField(label, value string, maxLen int) error {
	if len([]rune(value)) > maxLen {
		return errort.NewCommonErr(errort.SceneRuleParamsError, fmt.Errorf("%s 长度不能超过 %d 个字符", label, maxLen))
	}
	if unsafeSqlChars.MatchString(value) {
		return errort.NewCommonErr(errort.SceneRuleParamsError, fmt.Errorf("%s 包含非法字符", label))
	}
	return nil
}

// checkDecideCondition 按属性类型校验判断条件格式
func checkDecideCondition(specsType, decideCondition string) error {
	switch constants.SpecsType(specsType) {
	case constants.SpecsTypeInt, constants.SpecsTypeFloat, "":
		if !numericConditionPattern.MatchString(decideCondition) {
			return errort.NewCommonErr(errort.SceneRuleParamsError,
				fmt.Errorf("判断条件格式错误，应为 操作符+数值，例如 \"> 30\""))
		}
	case constants.SpecsTypeBool:
		if !regexp.MustCompile(`^=\s*(true|false)$`).MatchString(decideCondition) {
			return errort.NewCommonErr(errort.SceneRuleParamsError, fmt.Errorf("布尔型判断条件应为 = true 或 = false"))
		}
	default:
		if !textConditionPattern.MatchString(decideCondition) {
			return errort.NewCommonErr(errort.SceneRuleParamsError,
				fmt.Errorf("判断条件格式错误，应为 = 值（不含引号/分号）"))
		}
	}
	return nil
}

// checkSceneParam 触发条件与执行动作校验
func checkSceneParam(conditions []models.SceneCondition, actions []models.SceneAction) error {
	if len(conditions) == 0 {
		return errort.NewCommonErr(errort.SceneRuleParamsError, fmt.Errorf("请配置触发条件"))
	}
	condition := conditions[0]
	switch condition.ConditionType {
	case ConditionTypeTimer:
		cron := cronOf(conditions)
		if cron == "" {
			return errort.NewCommonErr(errort.SceneRuleParamsError, fmt.Errorf("请配置定时表达式"))
		}
		// 提前校验表达式合法性，避免编辑时通过、启动时才报错
		if err := validateCron(cron); err != nil {
			return err
		}
	case ConditionTypeNotify:
		option := condition.Option
		if option == nil {
			return errort.NewCommonErr(errort.SceneRuleParamsError, fmt.Errorf("请配置触发条件"))
		}
		trigger := constants.Trigger(option["trigger"])
		switch trigger {
		case constants.DeviceDataTrigger:
			for _, key := range []string{"product_id", "device_id", "code", "decide_condition"} {
				if strings.TrimSpace(option[key]) == "" {
					return errort.NewCommonErr(errort.SceneRuleParamsError, fmt.Errorf("触发条件缺少 %s", key))
				}
			}
			// 这些字段会被拼进 eKuiper SQL，必须校验，避免越出字符串字面量导致条件恒真
			for label, key := range map[string]string{"产品ID": "product_id", "设备ID": "device_id", "功能编码": "code"} {
				if err := checkSqlSafeField(label, option[key], maxIdLen); err != nil {
					return err
				}
			}
			if len([]rune(option["decide_condition"])) > maxConditionField {
				return errort.NewCommonErr(errort.SceneRuleParamsError, fmt.Errorf("判断条件过长"))
			}
			if err := checkDecideCondition(option["type"], option["decide_condition"]); err != nil {
				return err
			}
		case constants.DeviceEventTrigger:
			for _, key := range []string{"product_id", "device_id", "code"} {
				if strings.TrimSpace(option[key]) == "" {
					return errort.NewCommonErr(errort.SceneRuleParamsError, fmt.Errorf("触发条件缺少 %s", key))
				}
			}
			for label, key := range map[string]string{"产品ID": "product_id", "设备ID": "device_id", "事件编码": "code"} {
				if err := checkSqlSafeField(label, option[key], maxIdLen); err != nil {
					return err
				}
			}
		case constants.DeviceStatusTrigger:
			for _, key := range []string{"device_id", "status"} {
				if strings.TrimSpace(option[key]) == "" {
					return errort.NewCommonErr(errort.SceneRuleParamsError, fmt.Errorf("触发条件缺少 %s", key))
				}
			}
			for label, key := range map[string]string{"设备ID": "device_id"} {
				if err := checkSqlSafeField(label, option[key], maxIdLen); err != nil {
					return err
				}
			}
		default:
			return errort.NewCommonErr(errort.SceneRuleParamsError, fmt.Errorf("不支持的触发方式: %s", option["trigger"]))
		}
	default:
		return errort.NewCommonErr(errort.SceneRuleParamsError, fmt.Errorf("不支持的触发类型: %s", condition.ConditionType))
	}

	if len(actions) == 0 {
		return errort.NewCommonErr(errort.SceneRuleParamsError, fmt.Errorf("请配置执行动作"))
	}
	for i := range actions {
		if strings.TrimSpace(actions[i].DeviceId) == "" {
			return errort.NewCommonErr(errort.SceneRuleParamsError, fmt.Errorf("第 %d 个动作未选择设备", i+1))
		}
		if strings.TrimSpace(actions[i].Code) == "" {
			return errort.NewCommonErr(errort.SceneRuleParamsError, fmt.Errorf("第 %d 个动作未选择功能", i+1))
		}
		for label, value := range map[string]string{"设备ID": actions[i].DeviceId, "功能编码": actions[i].Code, "动作值": actions[i].Value} {
			if len([]rune(value)) > maxConditionField {
				return errort.NewCommonErr(errort.SceneRuleParamsError, fmt.Errorf("第 %d 个动作的%s过长", i+1, label))
			}
		}
	}
	return nil
}

// checkDescription 描述长度校验（与模型列宽一致，避免 MySQL 严格模式报错/SQLite 无感超长）
func checkDescription(description string) error {
	if len([]rune(description)) > maxDescriptionLen {
		return errort.NewCommonErr(errort.DefaultReqParamsError,
			fmt.Errorf("描述长度不能超过 %d 个字符", maxDescriptionLen))
	}
	return nil
}

func isTimerScene(conditions []models.SceneCondition) bool {
	return len(conditions) > 0 && conditions[0].ConditionType == ConditionTypeTimer
}

func cronOf(conditions []models.SceneCondition) string {
	if len(conditions) == 0 || conditions[0].Option == nil {
		return ""
	}
	return strings.TrimSpace(conditions[0].Option["cron_expression"])
}

func toResponse(scene *models.Scene) dtos.SceneSearchQueryResponse {
	conditions := scene.Conditions
	if conditions == nil {
		conditions = models.SceneConditions{}
	}
	actions := scene.Actions
	if actions == nil {
		actions = models.SceneActions{}
	}
	return dtos.SceneSearchQueryResponse{
		Id:          scene.Id,
		Name:        scene.Name,
		Description: scene.Describe,
		Status:      scene.Status,
		Created:     scene.Created,
		Modified:    scene.Modified,
		Conditions:  conditions,
		Actions:     actions,
	}
}

func stringOf(m map[string]interface{}, key string) string {
	if m == nil {
		return ""
	}
	if v, ok := m[key]; ok && v != nil {
		return strings.TrimSpace(fmt.Sprintf("%v", v))
	}
	return ""
}
