package ruleengineapp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

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
	"iotserver/internal/pkg/utils"
	"iotserver/internal/pkg/valid"
	"iotserver/internal/tools/ekuiperclient"
)

const (
	// maxDescriptionLen 描述上限（与模型列宽一致）
	maxDescriptionLen = 255
	// maxSqlLen 规则 SQL 上限，防止超大请求体
	maxSqlLen = 10000
	// maxRuleTags/maxRuleTagLen 规则标签限制
	maxRuleTags   = 20
	maxRuleTagLen = 32
	// maxRuleActions 单条规则的动作数量上限
	maxRuleActions = 10
	// maxBatchRules 批量启停上限
	maxBatchRules = 100
	// maxDebugData 试运行模拟数据条数上限
	maxDebugData = 200
	// maxDebugBytes 试运行模拟数据体积上限
	maxDebugBytes = 1 << 20
	// maxDebugTemplateLen 试运行输出模板长度上限
	maxDebugTemplateLen = 10000
	// monitorInterval 运行状态同步周期
	monitorInterval = 5 * time.Second
	monitorTimeout  = 3 * time.Second
	// monitorBatchSize 每轮同步的规则条数上限
	monitorBatchSize = 200
	// statusSyncSkipWindow 人工启停/编辑后的静默窗口，窗口内不做状态回写（避免 UI 回弹）
	statusSyncSkipWindow = 10 * time.Second
	// resyncTimeout 启动后重下发动作的整体超时
	resyncTimeout = 30 * time.Second
	// maxImportRules 单次导入规则数上限
	maxImportRules = 100
	// maxExportRules 单次导出规则数上限
	maxExportRules = 1000
	// debugSessionTTL 试运行会话端口缓存的有效期
	debugSessionTTL = time.Hour
)

var ruleIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)
var ruleDebugIDPattern = regexp.MustCompile(`^test_[0-9]+_[0-9]+$`)

// reservedRuleIDs 与 /api/v1/rule-engine 下的静态路由同名，作为规则 ID 会被路由优先匹配。
var reservedRuleIDs = map[string]struct{}{
	"export":      {},
	"debug":       {},
	"batch-start": {},
	"batch-stop":  {},
	"validate":    {},
	"import":      {},
}

func isReservedRuleID(id string) bool {
	_, ok := reservedRuleIDs[strings.ToLower(strings.TrimSpace(id))]
	return ok
}

// debugSession 试运行规则对应的 eKuiper SSE 端口（/ruletest 返回，可能被配置成非 10081）
type debugSession struct {
	port    int
	created time.Time
	// actions 调试时额外执行的动作；eKuiper 的 /ruletest 只输出 SSE，不会执行规则 sink，
	// 所以需要平台按动作资源配置把调试结果真实推送出去
	actions []debugAction
	// hasDataTemplate 输出已经过数据模板格式化；没有模板时只对合法 JSON 执行动作，
	// 避免把 eKuiper 的纯文本错误当成业务数据推送
	hasDataTemplate bool
}

// debugAction 调试时会执行的动作配置（只保留平台支持的 HTTP/MQTT 动作）
type debugAction struct {
	resourceID   string
	name         string
	resourceType string
	props        dtos.Actions
}

type ruleEngineApp struct {
	dic      *di.Container
	dbClient interfaces.DBClient
	lc       logger.LoggingClient
	// syncGate 保证同一时刻只有一轮状态同步在跑
	syncGate chan struct{}
	// debugSessions 记录试运行 ID → SSE 端口；创建和订阅是两个请求，需要记住端口
	debugMu       sync.Mutex
	debugSessions map[string]debugSession
}

func New(dic *di.Container) *ruleEngineApp {
	app := &ruleEngineApp{
		dic:           dic,
		dbClient:      container.DBClientFrom(dic.Get),
		lc:            pkgcontainer.LoggingClientFrom(dic.Get),
		syncGate:      make(chan struct{}, 1),
		debugSessions: make(map[string]debugSession),
	}
	// 后台同步 eKuiper 真实运行状态
	go app.monitor()
	// 启动后一次性重下发运行中规则的动作：配置变更（如回调密钥）或 eKuiper 数据重建后自愈
	go app.resyncRunningRuleActions()
	return app
}

// NewWithoutMonitor 供测试使用：不启动后台状态同步
func NewWithoutMonitor(dic *di.Container) *ruleEngineApp {
	return &ruleEngineApp{
		dic:           dic,
		dbClient:      container.DBClientFrom(dic.Get),
		lc:            pkgcontainer.LoggingClientFrom(dic.Get),
		syncGate:      make(chan struct{}, 1),
		debugSessions: make(map[string]debugSession),
	}
}

/** ***************************** 查询 ***************************** **/

// RuleEngineSearch 规则列表
func (app *ruleEngineApp) RuleEngineSearch(ctx context.Context, req dtos.RuleEngineSearchQueryRequest) ([]dtos.RuleEngineSearchQueryResponse, uint32, error) {
	offset, limit := dtos.OffsetLimit(req.BaseSearchConditionQuery)
	list, total, err := app.dbClient.WithCtx(ctx).RuleEngineSearch(offset, limit, req)
	if err != nil {
		return nil, 0, err
	}
	out, err := app.toResponses(ctx, list)
	if err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

// RuleEngineById 规则详情（含绑定的数据资源）
func (app *ruleEngineApp) RuleEngineById(ctx context.Context, id string) (dtos.RuleEngineSearchQueryResponse, error) {
	rule, err := app.dbClient.WithCtx(ctx).RuleEngineById(id)
	if err != nil {
		return dtos.RuleEngineSearchQueryResponse{}, err
	}
	list, err := app.toResponses(ctx, []models.RuleEngine{rule})
	if err != nil {
		return dtos.RuleEngineSearchQueryResponse{}, err
	}
	return list[0], nil
}

// RuleEngineStatus 规则运行状态（直接返回 eKuiper 的规则指标，前端以 JSON 展示）
func (app *ruleEngineApp) RuleEngineStatus(ctx context.Context, id string) (map[string]interface{}, error) {
	if _, err := app.dbClient.WithCtx(ctx).RuleEngineById(id); err != nil {
		return nil, err
	}
	stats, err := app.ekuiper().GetRuleStats(ctx, id)
	if err != nil {
		return nil, err
	}
	if stats == nil {
		stats = map[string]interface{}{}
	}
	return stats, nil
}

/** ***************************** 增删改 ***************************** **/

// RuleEngineAdd 新增规则：先下发到 eKuiper，成功后再入库
func (app *ruleEngineApp) RuleEngineAdd(ctx context.Context, req dtos.RuleEngineAddRequest) error {
	tenantID := tenant.FromContext(ctx).TenantID
	if err := app.validateRuleRequest(req); err != nil {
		return err
	}
	tags, err := normalizeTags(req.Tags)
	if err != nil {
		return err
	}
	storedActions, ekuiperActions, err := app.resolveRequestActions(ctx, req.Actions, req.DataResourceId)
	if err != nil {
		return err
	}
	filter, compiledSQL, err := buildRuleFilter(req.Filter, tenantID)
	if err != nil {
		return err
	}

	ekuiper := app.ekuiper()
	ruleID := strings.TrimSpace(req.Id)
	if ruleID == "" {
		// 自动生成 ID 时也必须同时避开平台规则库和 eKuiper（告警/场景共用 ID 空间）。
		// 否则随机 ID 撞到已有规则会走 UpdateRule 分支，覆盖别人的 SQL/动作。
		ruleID, err = app.generateRuleID(ctx, ekuiper)
		if err != nil {
			return err
		}
	} else if err = app.ensureRuleIDAvailable(ctx, ruleID, true); err != nil {
		return err
	}

	rule := models.RuleEngine{
		TenantField:    models.TenantField{TenantId: tenantID},
		Id:             ruleID,
		Name:           req.Name,
		Describe:       req.Description,
		Status:         string(constants.RuleEngineStop),
		Tags:           tags,
		Actions:        storedActions,
		Options:        req.Options,
		DataResourceId: storedActions[0].ResourceId,
		ResourceType:   storedActions[0].ResourceType,
		Filter:         filter,
	}

	def := ekuiperRule(rule, compiledSQL, ekuiperActions)
	defClient, supportsDef := ekuiper.(ekuiperclient.RuleDefinitionClient)
	exist, err := ekuiper.RuleExist(ctx, rule.Id)
	if err != nil {
		return err
	}
	// 任何已存在的 eKuiper ID 都不能被新增流程覆盖：告警/场景规则共用 ID 空间。
	// 自动生成的 ID 已在 generateRuleID 里避开现有规则，这里出现 exist 只可能是竞态。
	if exist {
		return paramError("规则 ID 已存在", nil)
	}
	created := true
	if supportsDef {
		if err = defClient.CreateRuleDefinition(ctx, def); err != nil {
			return err
		}
	} else if err = ekuiper.CreateRule(ctx, ekuiperActions, rule.Id, compiledSQL); err != nil {
		return err
	}

	if _, err = app.dbClient.WithCtx(ctx).AddRuleEngine(rule); err != nil {
		// eKuiper 已创建但入库失败：回滚 eKuiper 规则。
		// 只有「本次新建」的规则才能回滚——ID 碰撞时会命中已存在的同名规则，
		// 无条件删除会把别人的规则删掉，导致其静默失效
		if created {
			if delErr := ekuiper.DeleteRule(ctx, rule.Id); delErr != nil {
				app.lc.Errorf("rollback ekuiper rule %s failed: %v", rule.Id, delErr)
			}
		}
		return err
	}
	if req.RunImmediately {
		if startErr := app.RuleEngineStart(ctx, rule.Id); startErr != nil {
			// 保存后启动失败：回滚本次新增，避免界面提示保存失败但规则已入库
			app.rollbackAddedRule(ctx, rule.Id, created, ekuiper)
			return startErr
		}
	}
	return nil
}

// RuleEngineUpdate 修改规则：同步更新 eKuiper 规则
func (app *ruleEngineApp) RuleEngineUpdate(ctx context.Context, req dtos.RuleEngineUpdateRequest) error {
	if strings.TrimSpace(req.Id) == "" {
		return errort.NewCommonErr(errort.DefaultIdEmpty, fmt.Errorf("id is empty"))
	}
	old, err := app.dbClient.WithCtx(ctx).RuleEngineById(req.Id)
	if err != nil {
		return err
	}
	// 运行中的规则先停止再修改，避免下发与运行状态不一致
	if old.Status == string(constants.RuleEngineStart) {
		return errort.NewCommonErr(errort.RuleEngineIsStartingNotAllowUpdate, nil)
	}
	if err = app.validateRuleRequest(req.RuleEngineAddRequest); err != nil {
		return err
	}
	tags, err := normalizeTags(req.Tags)
	if err != nil {
		return err
	}
	storedActions, ekuiperActions, err := app.resolveRequestActions(ctx, req.Actions, req.DataResourceId)
	if err != nil {
		return err
	}
	tenantID := old.TenantId
	if tenantID == "" {
		tenantID = tenant.FromContext(ctx).TenantID
	}
	filter, compiledSQL, err := buildRuleFilter(req.Filter, tenantID)
	if err != nil {
		return err
	}

	rule := models.RuleEngine{
		TenantField:    models.TenantField{TenantId: tenantID},
		Id:             old.Id,
		Name:           req.Name,
		Describe:       req.Description,
		Tags:           tags,
		Actions:        storedActions,
		Options:        req.Options,
		DataResourceId: storedActions[0].ResourceId,
		ResourceType:   storedActions[0].ResourceType,
		Filter:         filter,
	}
	def := ekuiperRule(rule, compiledSQL, ekuiperActions)
	if err = app.applyEkuiperRule(ctx, def, old.Id); err != nil {
		return err
	}
	if err = app.dbClient.WithCtx(ctx).UpdateRuleEngine(rule); err != nil {
		// DB 更新失败（如重名）时 eKuiper 已经换成了新定义，必须把旧定义恢复回去，
		// 否则界面提示保存失败，但运行时实际使用的是失败那次的 SQL/动作。
		app.rollbackUpdatedRule(ctx, old, tenantID)
		return err
	}
	if req.RunImmediately {
		if startErr := app.RuleEngineStart(ctx, old.Id); startErr != nil {
			// 保存后启动失败：回滚库和 eKuiper 定义，保证界面提示失败时规则没有被改掉
			app.rollbackUpdatedRule(ctx, old, tenantID)
			return startErr
		}
	}
	return nil
}

// rollbackAddedRule 新增后启动失败时回滚：删库，并且只删除本次在 eKuiper 新建的规则。
// createdInEkuiper=false 表示命中了一条已存在但不在平台库里的规则，不能删除它。
func (app *ruleEngineApp) rollbackAddedRule(ctx context.Context, id string, createdInEkuiper bool, ekuiper ekuiperclient.EkuiperClient) {
	if err := app.dbClient.WithCtx(ctx).DeleteRuleEngine(id); err != nil && app.lc != nil {
		app.lc.Warnf("回滚规则 %s 入库记录失败: %v", id, err)
	}
	if createdInEkuiper {
		if err := ekuiper.DeleteRule(ctx, id); err != nil && app.lc != nil {
			app.lc.Warnf("回滚规则 %s 的 eKuiper 定义失败: %v", id, err)
		}
	}
}

// rollbackUpdatedRule 修改后启动失败时恢复修改前的库记录和 eKuiper 定义。
func (app *ruleEngineApp) rollbackUpdatedRule(ctx context.Context, old models.RuleEngine, tenantID string) {
	if err := app.dbClient.WithCtx(ctx).UpdateRuleEngine(old); err != nil && app.lc != nil {
		app.lc.Warnf("回滚规则 %s 库记录失败: %v", old.Id, err)
	}
	_, actions, err := app.resolveRuleActions(ctx, old)
	if err != nil {
		if app.lc != nil {
			app.lc.Warnf("回滚规则 %s 动作解析失败: %v", old.Id, err)
		}
		return
	}
	sql, err := storedCompiledSQL(old, tenantID)
	if err != nil {
		if app.lc != nil {
			app.lc.Warnf("回滚规则 %s 编译 SQL 失败: %v", old.Id, err)
		}
		return
	}
	if err := app.applyEkuiperRule(ctx, ekuiperRule(old, sql, actions), old.Id); err != nil && app.lc != nil {
		app.lc.Warnf("回滚规则 %s 的 eKuiper 定义失败: %v", old.Id, err)
	}
}

// RuleEngineDelete 删除规则：先删 eKuiper 规则（不存在则忽略），再删库
func (app *ruleEngineApp) RuleEngineDelete(ctx context.Context, id string) error {
	rule, err := app.dbClient.WithCtx(ctx).RuleEngineById(id)
	if err != nil {
		return err
	}
	ekuiper := app.ekuiper()
	exist, err := ekuiper.RuleExist(ctx, rule.Id)
	if err != nil {
		return err
	}
	if exist {
		if err = ekuiper.DeleteRule(ctx, rule.Id); err != nil {
			return err
		}
	}
	return app.dbClient.WithCtx(ctx).DeleteRuleEngine(id)
}

/** ***************************** 启停 ***************************** **/

// RuleEngineStart 启动规则
func (app *ruleEngineApp) RuleEngineStart(ctx context.Context, id string) error {
	rule, err := app.dbClient.WithCtx(ctx).RuleEngineById(id)
	if err != nil {
		return err
	}
	_, ekuiperActions, err := app.resolveRuleActions(ctx, rule)
	if err != nil {
		return err
	}
	tenantID := rule.TenantId
	if tenantID == "" {
		tenantID = tenant.FromContext(ctx).TenantID
	}
	var compiledSQL string
	if strings.TrimSpace(rule.Filter.CompiledSql) != "" {
		compiledSQL = rule.Filter.CompiledSql
	} else {
		// 存量规则：filter.sql 是旧版本编译后的 SQL，拆分回原始 SQL + compiled_sql 并回写
		normalized, compiled, nErr := normalizeStoredFilter(rule.Filter, tenantID)
		if nErr != nil {
			return nErr
		}
		rule.Filter = normalized
		rule.Filter.CompiledSql = compiled
		compiledSQL = compiled
		if uErr := app.dbClient.WithCtx(ctx).UpdateRuleEngine(rule); uErr != nil && app.lc != nil {
			app.lc.Warnf("回写规则 %s 的编译 SQL 失败: %v", rule.Id, uErr)
		}
	}
	def := ekuiperRule(rule, compiledSQL, ekuiperActions)
	if err = app.applyEkuiperRule(ctx, def, rule.Id); err != nil {
		return err
	}
	if err = app.ekuiper().StartRule(ctx, rule.Id); err != nil {
		return err
	}
	return app.dbClient.WithCtx(ctx).UpdateRuleEngineStatus(rule.Id, string(constants.RuleEngineStart))
}

// RuleEngineStop 停止规则
func (app *ruleEngineApp) RuleEngineStop(ctx context.Context, id string) error {
	rule, err := app.dbClient.WithCtx(ctx).RuleEngineById(id)
	if err != nil {
		return err
	}
	ekuiper := app.ekuiper()
	exist, err := ekuiper.RuleExist(ctx, rule.Id)
	if err != nil {
		return err
	}
	if exist {
		if err = ekuiper.StopRule(ctx, rule.Id); err != nil {
			return err
		}
	}
	return app.dbClient.WithCtx(ctx).UpdateRuleEngineStatus(rule.Id, string(constants.RuleEngineStop))
}

// RuleEngineBatchStart 批量启动
func (app *ruleEngineApp) RuleEngineBatchStart(ctx context.Context, ids []string) dtos.RuleEngineBatchResult {
	return app.batch(ctx, ids, app.RuleEngineStart)
}

// RuleEngineBatchStop 批量停止
func (app *ruleEngineApp) RuleEngineBatchStop(ctx context.Context, ids []string) dtos.RuleEngineBatchResult {
	return app.batch(ctx, ids, app.RuleEngineStop)
}

func (app *ruleEngineApp) batch(ctx context.Context, ids []string, op func(context.Context, string) error) dtos.RuleEngineBatchResult {
	result := dtos.RuleEngineBatchResult{Failed: []dtos.RuleEngineBatchFailure{}}
	seen := make(map[string]struct{}, len(ids))
	for _, raw := range ids {
		id := strings.TrimSpace(raw)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		if len(seen) > maxBatchRules {
			result.Failed = append(result.Failed, dtos.RuleEngineBatchFailure{Id: id, Message: "批量操作数量超过上限"})
			continue
		}
		if err := op(ctx, id); err != nil {
			result.Failed = append(result.Failed, dtos.RuleEngineBatchFailure{Id: id, Message: errorMessage(err)})
			continue
		}
		result.Success++
	}
	return result
}

/** ***************************** 校验 / 试运行 ***************************** **/

// RuleEngineValidate 校验 SQL（注入租户条件后交给 eKuiper /rules/validate）
func (app *ruleEngineApp) RuleEngineValidate(ctx context.Context, sql string) (dtos.RuleEngineValidateResponse, error) {
	resp := dtos.RuleEngineValidateResponse{TenantId: tenant.FromContext(ctx).TenantID}
	raw := strings.TrimSpace(sql)
	if raw == "" {
		resp.Message = "规则 SQL 不能为空"
		return resp, nil
	}
	if len(raw) > maxSqlLen {
		resp.Message = "规则SQL过长"
		return resp, nil
	}
	compiled, err := enforceTenantFilter(raw, resp.TenantId)
	if err != nil {
		resp.Message = errorMessage(err)
		return resp, nil
	}
	resp.CompiledSql = compiled
	client, ok := app.ekuiper().(ekuiperclient.RuleDefinitionClient)
	if !ok {
		resp.Valid = true
		return resp, nil
	}
	if err = client.ValidateRule(ctx, compiled); err != nil {
		resp.Message = errorMessage(err)
		return resp, nil
	}
	resp.Valid = true
	return resp, nil
}

// RuleEngineDebugStart 创建 eKuiper 试运行规则。
// 真正的启动放在 RuleEngineDebugStream：先让浏览器订阅 SSE，再启动，避免单次模拟数据在订阅前被丢弃。
func (app *ruleEngineApp) RuleEngineDebugStart(ctx context.Context, req dtos.RuleEngineDebugRequest) (string, error) {
	raw := strings.TrimSpace(req.Sql)
	if raw == "" {
		return "", paramError("规则 SQL 不能为空", nil)
	}
	if len(raw) > maxSqlLen {
		return "", paramError("规则SQL过长", nil)
	}
	mockEnabled := true
	if req.MockEnabled != nil {
		mockEnabled = *req.MockEnabled
	}
	if mockEnabled {
		if len(req.Data) == 0 {
			return "", paramError("请填写模拟数据", nil)
		}
		if len(req.Data) > maxDebugData {
			return "", paramError("模拟数据不能超过 {{.p1}} 条", map[string]interface{}{"p1": maxDebugData})
		}
		payload, err := json.Marshal(req.Data)
		if err != nil {
			return "", paramError("模拟数据格式非法", nil)
		}
		if len(payload) > maxDebugBytes {
			return "", paramError("模拟数据过大", nil)
		}
	}
	compiled, err := enforceTenantFilter(raw, tenant.FromContext(ctx).TenantID)
	if err != nil {
		return "", err
	}
	interval := req.Interval
	if interval <= 0 {
		interval = 1000
	}
	if interval < 100 {
		interval = 100
	}
	if interval > 10000 {
		interval = 10000
	}
	loop := true
	if req.Loop != nil {
		loop = *req.Loop
	}
	if utf8.RuneCountInString(req.DataTemplate) > maxDebugTemplateLen {
		return "", paramError("数据模版过长", nil)
	}
	hasDataTemplate := strings.TrimSpace(req.DataTemplate) != ""
	// sendSingle 与真实 sink 保持一致：每条结果单独输出，数据模板也按单条记录渲染
	sinkProps := map[string]interface{}{"sendSingle": true}
	if hasDataTemplate {
		sinkProps["dataTemplate"] = req.DataTemplate
	}
	// 调试动作需要提前解析并校验资源；eKuiper 试运行本身不会执行 sink
	debugActions, err := app.resolveDebugActions(ctx, req.ExecuteActions, req.ActionResourceIDs)
	if err != nil {
		return "", err
	}
	client, ok := app.ekuiper().(ekuiperclient.RuleTestClient)
	if !ok {
		return "", errort.NewCommonEdgeX(errort.SystemErrorCode, "当前 eKuiper 客户端不支持规则试运行", nil)
	}
	var mockSource map[string]map[string]interface{}
	if mockEnabled {
		mockSource = map[string]map[string]interface{}{
			ekuiperclient.MQTTStreamName: {
				"data":     req.Data,
				"interval": interval,
				"loop":     loop,
			},
		}
	}
	id := fmt.Sprintf("test_%s_%s", utils.RandomNum(), utils.RandomNum())
	createdID, port, err := client.CreateRuleTest(ctx, dtos.RuleTestRequest{
		Id:         id,
		Sql:        compiled,
		MockSource: mockSource,
		SinkProps:  sinkProps,
	})
	if err != nil {
		return "", err
	}
	if createdID != "" {
		id = createdID
	}
	app.rememberDebugSession(id, port, debugActions, hasDataTemplate)
	return id, nil
}

// RuleEngineDebugStream 先订阅 eKuiper 试运行 SSE，再启动试运行规则。
// 顺序不能反：eKuiper 启动时会立即拉取第一条模拟数据，没有订阅者时该条会被丢弃。
func (app *ruleEngineApp) RuleEngineDebugStream(ctx context.Context, id string) (io.ReadCloser, error) {
	if !ruleDebugIDPattern.MatchString(id) {
		return nil, paramError("试运行 ID 非法", nil)
	}
	client, ok := app.ekuiper().(ekuiperclient.RuleTestClient)
	if !ok {
		return nil, errort.NewCommonEdgeX(errort.SystemErrorCode, "当前 eKuiper 客户端不支持规则试运行", nil)
	}
	body, err := client.OpenRuleTestStream(ctx, id, app.debugSessionPort(id))
	if err != nil {
		return nil, err
	}
	// 给 eKuiper 一点时间注册 SSE 订阅者（握手完成后通常已注册，这里只做兜底）
	time.Sleep(50 * time.Millisecond)
	if err = client.StartRuleTest(ctx, id); err != nil {
		_ = body.Close()
		return nil, err
	}
	return body, nil
}

// RuleEngineDebugStop 停止试运行
func (app *ruleEngineApp) RuleEngineDebugStop(ctx context.Context, id string) error {
	if !ruleDebugIDPattern.MatchString(id) {
		return paramError("试运行 ID 非法", nil)
	}
	client, ok := app.ekuiper().(ekuiperclient.RuleTestClient)
	if !ok {
		return errort.NewCommonEdgeX(errort.SystemErrorCode, "当前 eKuiper 客户端不支持规则试运行", nil)
	}
	err := client.DeleteRuleTest(ctx, id)
	app.forgetDebugSession(id)
	return err
}

// rememberDebugSession 记录试运行 ID 对应的 SSE 端口和调试动作，并顺带清理过期会话。
func (app *ruleEngineApp) rememberDebugSession(id string, port int, actions []debugAction, hasDataTemplate bool) {
	if id == "" {
		return
	}
	now := time.Now()
	expired := make([]string, 0)
	app.debugMu.Lock()
	if app.debugSessions == nil {
		app.debugSessions = make(map[string]debugSession)
	}
	for key, session := range app.debugSessions {
		if now.Sub(session.created) > debugSessionTTL {
			delete(app.debugSessions, key)
			expired = append(expired, key)
		}
	}
	app.debugSessions[id] = debugSession{
		port:            port,
		created:         now,
		actions:         actions,
		hasDataTemplate: hasDataTemplate,
	}
	app.debugMu.Unlock()

	// 会话过期只删 map 不够：/ruletest 规则还在 eKuiper 里，必须显式删除。
	if len(expired) == 0 {
		return
	}
	client, ok := app.ekuiper().(ekuiperclient.RuleTestClient)
	if !ok {
		return
	}
	go func() {
		for _, expiredID := range expired {
			if err := client.DeleteRuleTest(context.Background(), expiredID); err != nil {
				app.lc.Warnf("cleanup expired debug rule %s failed: %v", expiredID, err)
			}
		}
	}()
}

func (app *ruleEngineApp) debugSessionPort(id string) int {
	app.debugMu.Lock()
	defer app.debugMu.Unlock()
	return app.debugSessions[id].port
}

// debugSessionActions 读取调试会话的动作配置。返回副本，避免执行期间被清理线程改写。
func (app *ruleEngineApp) debugSessionActions(id string) ([]debugAction, bool, bool) {
	app.debugMu.Lock()
	defer app.debugMu.Unlock()
	session, ok := app.debugSessions[id]
	if !ok {
		return nil, false, false
	}
	actions := make([]debugAction, len(session.actions))
	copy(actions, session.actions)
	return actions, session.hasDataTemplate, true
}

func (app *ruleEngineApp) forgetDebugSession(id string) {
	app.debugMu.Lock()
	defer app.debugMu.Unlock()
	delete(app.debugSessions, id)
}

/** ***************************** 导入导出 ***************************** **/

// RuleEngineExport 导出规则（按动作资源名称，便于跨环境导入）
func (app *ruleEngineApp) RuleEngineExport(ctx context.Context, ids []string) (dtos.RuleEngineExportFile, error) {
	cleanIDs := make([]string, 0, len(ids))
	for _, id := range ids {
		if id = strings.TrimSpace(id); id != "" {
			cleanIDs = append(cleanIDs, id)
		}
	}
	if len(cleanIDs) > maxExportRules {
		return dtos.RuleEngineExportFile{}, paramError("规则数量超过导出上限 {{.p1}} 条，请分批导出",
			map[string]interface{}{"p1": maxExportRules})
	}
	var list []models.RuleEngine
	if len(cleanIDs) > 0 {
		for _, id := range cleanIDs {
			rule, err := app.dbClient.WithCtx(ctx).RuleEngineById(id)
			if err != nil {
				return dtos.RuleEngineExportFile{}, err
			}
			list = append(list, rule)
		}
	} else {
		var (
			total uint32
			err   error
		)
		list, total, err = app.dbClient.WithCtx(ctx).RuleEngineSearch(0, maxExportRules, dtos.RuleEngineSearchQueryRequest{})
		if err != nil {
			return dtos.RuleEngineExportFile{}, err
		}
		if total > maxExportRules {
			return dtos.RuleEngineExportFile{}, paramError("规则数量超过导出上限 {{.p1}} 条，请分批导出",
				map[string]interface{}{"p1": maxExportRules})
		}
	}
	resources, err := app.resourceMap(ctx, list)
	if err != nil {
		return dtos.RuleEngineExportFile{}, err
	}
	out := dtos.RuleEngineExportFile{Version: 1, Rules: make([]dtos.RuleEngineExportRule, 0, len(list))}
	for i := range list {
		rule := list[i]
		ruleActions := rule.Actions
		if len(ruleActions) == 0 && strings.TrimSpace(rule.DataResourceId) != "" {
			ruleActions = models.RuleEngineActions{{ResourceId: rule.DataResourceId, ResourceType: rule.ResourceType}}
		}
		actions := make([]dtos.RuleEngineExportAction, 0, len(ruleActions))
		for _, action := range ruleActions {
			name, typ := action.ResourceName, action.ResourceType
			if resource, ok := resources[action.ResourceId]; ok {
				name, typ = resource.Name, resource.Type
			}
			actions = append(actions, dtos.RuleEngineExportAction{ResourceName: name, ResourceType: typ})
		}
		filter := rule.Filter
		filter.CompiledSql = ""
		// 旧规则的 filter.sql 里带着注入的租户条件，导出时恢复成原始 SQL
		if rule.TenantId != "" {
			if stripped, changed := stripTenantPredicates(filter.Sql); changed {
				filter.Sql = stripped
			}
		}
		out.Rules = append(out.Rules, dtos.RuleEngineExportRule{
			Name:        rule.Name,
			Description: rule.Describe,
			Tags:        []string(rule.Tags),
			Filter:      filter,
			Actions:     actions,
			Options:     rule.Options,
		})
	}
	return out, nil
}

// RuleEngineImport 导入规则：按资源名称解析当前租户的数据资源
func (app *ruleEngineApp) RuleEngineImport(ctx context.Context, req dtos.RuleEngineImportRequest) dtos.RuleEngineImportResult {
	result := dtos.RuleEngineImportResult{Failed: []dtos.RuleEngineImportFailure{}}
	if len(req.Rules) == 0 {
		return result
	}
	if len(req.Rules) > maxImportRules {
		result.Failed = append(result.Failed, dtos.RuleEngineImportFailure{
			Name:    "",
			Message: fmt.Sprintf("单次最多导入 %d 条规则，本次 %d 条未处理", maxImportRules, len(req.Rules)),
		})
		return result
	}
	resourceList, _, err := app.dbClient.WithCtx(ctx).DataResourceSearch(0, 1000, dtos.DataResourceSearchRequest{})
	if err != nil {
		result.Failed = append(result.Failed, dtos.RuleEngineImportFailure{Name: "", Message: errorMessage(err)})
		return result
	}
	resourceByKey := make(map[string]string, len(resourceList))
	resourceByName := make(map[string][]string, len(resourceList))
	for _, resource := range resourceList {
		resourceByKey[importResourceKey(resource.Name, resource.Type)] = resource.Id
		name := strings.TrimSpace(resource.Name)
		resourceByName[name] = append(resourceByName[name], resource.Id)
	}
	for _, item := range req.Rules {
		actions := make([]dtos.RuleEngineActionRequest, 0, len(item.Actions))
		missing := ""
		for _, action := range item.Actions {
			id := resolveImportResource(resourceByKey, resourceByName, action.ResourceName, action.ResourceType)
			if id == "" {
				missing = action.ResourceName
				break
			}
			actions = append(actions, dtos.RuleEngineActionRequest{ResourceId: id})
		}
		if missing != "" {
			result.Failed = append(result.Failed, dtos.RuleEngineImportFailure{Name: item.Name, Message: "应用不存在: " + missing})
			continue
		}
		filter := item.Filter
		filter.CompiledSql = ""
		addReq := dtos.RuleEngineAddRequest{
			Name:        item.Name,
			Description: item.Description,
			Tags:        item.Tags,
			Actions:     actions,
			Filter:      filter,
			Options:     item.Options,
		}
		if err := app.RuleEngineAdd(ctx, addReq); err != nil {
			result.Failed = append(result.Failed, dtos.RuleEngineImportFailure{Name: item.Name, Message: errorMessage(err)})
			continue
		}
		result.Imported++
	}
	return result
}

/** ***************************** 内部 ***************************** **/

func (app *ruleEngineApp) validateRuleRequest(req dtos.RuleEngineAddRequest) error {
	if !valid.Name(req.Name) {
		return errort.NewCommonErr(errort.DefaultNameSpecialCharacters, fmt.Errorf("%s", valid.NameRuleMsg))
	}
	if len([]rune(req.Description)) > maxDescriptionLen {
		return paramError("规则描述长度不能超过 {{.p1}} 个字符", map[string]interface{}{"p1": maxDescriptionLen})
	}
	rawSQL := dtos.BuildRuleEngineSql(req.Filter.Sql, req.Filter.SelectName, req.Filter.Condition)
	if len(rawSQL) > maxSqlLen {
		return paramError("规则SQL过长", nil)
	}
	if err := validateOptions(req.Options); err != nil {
		return err
	}
	if id := strings.TrimSpace(req.Id); id != "" {
		if !ruleIDPattern.MatchString(id) {
			return paramError("规则 ID 只能是 1-64 位字母、数字、下划线或中划线", nil)
		}
		if isReservedRuleID(id) {
			return paramError("规则 ID 不能使用保留字 export、debug、batch-start、batch-stop、validate、import", nil)
		}
	}
	return nil
}

func validateOptions(opts models.RuleEngineOptions) error {
	if opts.Concurrency < 0 || opts.Concurrency > 1024 {
		return paramError("线程数必须在 0-1024 之间", nil)
	}
	if opts.BufferLength < 0 || opts.BufferLength > 10000000 {
		return paramError("缓存大小非法", nil)
	}
	if opts.Qos < 0 || opts.Qos > 2 {
		return paramError("QoS 只能是 0/1/2", nil)
	}
	if opts.CheckpointInterval < 0 || opts.LateTolerance < 0 {
		return paramError("时间参数不能为负数", nil)
	}
	if opts.RestartStrategy != nil && opts.RestartStrategy.Attempts < 0 {
		return paramError("最大重试次数不能为负数", nil)
	}
	if utf8.RuneCountInString(opts.Cron) > 64 || utf8.RuneCountInString(opts.Duration) > 64 {
		return paramError("周期性触发策略/运行持续时间过长", nil)
	}
	return nil
}

func normalizeTags(tags []string) (models.RuleEngineTags, error) {
	out := make(models.RuleEngineTags, 0, len(tags))
	seen := make(map[string]struct{}, len(tags))
	for _, raw := range tags {
		tag := strings.TrimSpace(raw)
		if tag == "" {
			continue
		}
		if utf8.RuneCountInString(tag) > maxRuleTagLen {
			return nil, paramError("单个标签不能超过 {{.p1}} 个字符", map[string]interface{}{"p1": maxRuleTagLen})
		}
		if _, ok := seen[tag]; ok {
			continue
		}
		seen[tag] = struct{}{}
		out = append(out, tag)
	}
	if len(out) > maxRuleTags {
		return nil, paramError("标签数量不能超过 {{.p1}} 个", map[string]interface{}{"p1": maxRuleTags})
	}
	return out, nil
}

// resolveRequestActions 解析请求里的动作；兼容旧的 data_resource_id 单动作字段
func (app *ruleEngineApp) resolveRequestActions(ctx context.Context, actions []dtos.RuleEngineActionRequest, legacyID string) (models.RuleEngineActions, []dtos.Actions, error) {
	refs := make([]models.RuleEngineAction, 0, len(actions))
	for _, action := range actions {
		refs = append(refs, models.RuleEngineAction{ResourceId: strings.TrimSpace(action.ResourceId)})
	}
	if len(refs) == 0 && strings.TrimSpace(legacyID) != "" {
		refs = append(refs, models.RuleEngineAction{ResourceId: strings.TrimSpace(legacyID)})
	}
	return app.resolveActionRefs(ctx, refs)
}

// resolveRuleActions 解析库中已保存的动作
func (app *ruleEngineApp) resolveRuleActions(ctx context.Context, rule models.RuleEngine) (models.RuleEngineActions, []dtos.Actions, error) {
	refs := rule.Actions
	if len(refs) == 0 && strings.TrimSpace(rule.DataResourceId) != "" {
		refs = models.RuleEngineActions{{ResourceId: rule.DataResourceId, ResourceType: rule.ResourceType}}
	}
	return app.resolveActionRefs(ctx, refs)
}

func (app *ruleEngineApp) resolveActionRefs(ctx context.Context, refs models.RuleEngineActions) (models.RuleEngineActions, []dtos.Actions, error) {
	if len(refs) == 0 {
		return nil, nil, paramError("请选择应用", nil)
	}
	if len(refs) > maxRuleActions {
		return nil, nil, paramError("应用数量不能超过 {{.p1}} 个", map[string]interface{}{"p1": maxRuleActions})
	}
	stored := make(models.RuleEngineActions, 0, len(refs))
	actions := make([]dtos.Actions, 0, len(refs))
	seen := make(map[string]struct{}, len(refs))
	for _, ref := range refs {
		id := strings.TrimSpace(ref.ResourceId)
		if id == "" {
			return nil, nil, paramError("应用不能为空", nil)
		}
		if _, ok := seen[id]; ok {
			return nil, nil, paramError("同一个应用不能重复添加", nil)
		}
		seen[id] = struct{}{}
		resource, err := app.dbClient.WithCtx(ctx).DataResourceById(id)
		if err != nil {
			return nil, nil, errort.NewCommonErr(errort.InvalidSource,
				errort.NewMessageErr("应用 {{.p1}} 不存在或已被删除", map[string]interface{}{"p1": id}))
		}
		action, err := dtos.GetRuleEngineEkuiperActions(resource.Type, resource.Option)
		if err != nil {
			return nil, nil, errort.NewCommonErr(errort.DefaultReqParamsError, err)
		}
		actions = append(actions, action...)
		stored = append(stored, models.RuleEngineAction{
			ResourceId:   resource.Id,
			ResourceName: resource.Name,
			ResourceType: resource.Type,
		})
	}
	return stored, actions, nil
}

func buildRuleFilter(filter models.RuleEngineFilter, tenantID string) (models.RuleEngineFilter, string, error) {
	raw := dtos.BuildRuleEngineSql(filter.Sql, filter.SelectName, filter.Condition)
	// 保存前清掉历史注入的租户条件，避免原始 SQL 里混入平台条件
	if tenantID != "" {
		if stripped, changed := stripTenantPredicates(raw); changed {
			raw = stripped
		}
	}
	compiled, err := enforceTenantFilter(raw, tenantID)
	if err != nil {
		return filter, "", err
	}
	filter.Sql = raw
	filter.CompiledSql = compiled
	return filter, compiled, nil
}

// normalizeStoredFilter 兼容旧数据：旧版本把编译后的 SQL 存在 filter.sql（没有 compiled_sql）。
// 此时把 filter.sql 视为编译结果，剥掉注入的租户条件后恢复出原始 SQL。
func normalizeStoredFilter(filter models.RuleEngineFilter, tenantID string) (models.RuleEngineFilter, string, error) {
	if strings.TrimSpace(filter.CompiledSql) != "" {
		return filter, filter.CompiledSql, nil
	}
	if tenantID != "" {
		if stripped, changed := stripTenantPredicates(filter.Sql); changed {
			filter.Sql = stripped
		}
	}
	compiled, err := enforceTenantFilter(filter.Sql, tenantID)
	if err != nil {
		return filter, "", err
	}
	return filter, compiled, nil
}

func storedCompiledSQL(rule models.RuleEngine, tenantID string) (string, error) {
	if strings.TrimSpace(rule.Filter.CompiledSql) != "" {
		return rule.Filter.CompiledSql, nil
	}
	_, compiled, err := normalizeStoredFilter(rule.Filter, tenantID)
	return compiled, err
}

func ekuiperRule(rule models.RuleEngine, sql string, actions []dtos.Actions) dtos.EkuiperRule {
	options := rule.Options
	return dtos.EkuiperRule{
		Id:      rule.Id,
		Name:    rule.Name,
		Sql:     sql,
		Actions: actions,
		Options: &options,
		Tags:    []string(rule.Tags),
	}
}

func (app *ruleEngineApp) applyEkuiperRule(ctx context.Context, def dtos.EkuiperRule, ruleID string) error {
	ekuiper := app.ekuiper()
	exist, err := ekuiper.RuleExist(ctx, ruleID)
	if err != nil {
		return err
	}
	if client, ok := ekuiper.(ekuiperclient.RuleDefinitionClient); ok {
		if exist {
			return client.UpdateRuleDefinition(ctx, def)
		}
		return client.CreateRuleDefinition(ctx, def)
	}
	if exist {
		return ekuiper.UpdateRule(ctx, def.Actions, ruleID, def.Sql)
	}
	return ekuiper.CreateRule(ctx, def.Actions, ruleID, def.Sql)
}

func (app *ruleEngineApp) ensureRuleIDAvailable(ctx context.Context, id string, custom bool) error {
	if !custom {
		return nil
	}
	list, total, err := app.dbClient.WithCtx(tenant.Platform(ctx)).RuleEngineSearch(0, 1,
		dtos.RuleEngineSearchQueryRequest{BaseSearchConditionQuery: dtos.BaseSearchConditionQuery{Id: id}})
	if err != nil {
		return err
	}
	if total > 0 || len(list) > 0 {
		return paramError("规则 ID 已存在", nil)
	}
	return nil
}

// generateRuleID 生成同时避开平台规则表和 eKuiper 的 ID（告警/场景规则也占用 eKuiper ID 空间）。
func (app *ruleEngineApp) generateRuleID(ctx context.Context, ekuiper ekuiperclient.EkuiperClient) (string, error) {
	for i := 0; i < 10; i++ {
		id := utils.RandomNum()
		list, total, err := app.dbClient.WithCtx(tenant.Platform(ctx)).RuleEngineSearch(0, 1,
			dtos.RuleEngineSearchQueryRequest{BaseSearchConditionQuery: dtos.BaseSearchConditionQuery{Id: id}})
		if err != nil {
			return "", err
		}
		if total > 0 || len(list) > 0 {
			continue
		}
		exist, err := ekuiper.RuleExist(ctx, id)
		if err != nil {
			return "", err
		}
		if exist {
			continue
		}
		return id, nil
	}
	return "", paramError("生成规则 ID 失败，请重试", nil)
}

func (app *ruleEngineApp) resourceMap(ctx context.Context, list []models.RuleEngine) (map[string]models.DataResource, error) {
	ids := make([]string, 0, len(list))
	for i := range list {
		for _, action := range list[i].Actions {
			ids = append(ids, action.ResourceId)
		}
		if len(list[i].Actions) == 0 && list[i].DataResourceId != "" {
			ids = append(ids, list[i].DataResourceId)
		}
	}
	if len(ids) == 0 {
		return map[string]models.DataResource{}, nil
	}
	resources, err := app.dbClient.WithCtx(ctx).DataResourceByIds(ids)
	if err != nil {
		return nil, err
	}
	out := make(map[string]models.DataResource, len(resources))
	for i := range resources {
		out[resources[i].Id] = resources[i]
	}
	return out, nil
}

// toResponses 转出参，并批量补齐绑定的数据资源
func (app *ruleEngineApp) toResponses(ctx context.Context, list []models.RuleEngine) ([]dtos.RuleEngineSearchQueryResponse, error) {
	if len(list) == 0 {
		return []dtos.RuleEngineSearchQueryResponse{}, nil
	}
	resourceMap, err := app.resourceMap(ctx, list)
	if err != nil {
		return nil, err
	}

	out := make([]dtos.RuleEngineSearchQueryResponse, 0, len(list))
	for i := range list {
		rule := list[i]
		actions := make([]dtos.RuleEngineActionInfo, 0, len(rule.Actions))
		for _, action := range rule.Actions {
			info := dtos.RuleEngineActionInfo{
				ResourceId:   action.ResourceId,
				ResourceName: action.ResourceName,
				ResourceType: action.ResourceType,
			}
			if resource, ok := resourceMap[action.ResourceId]; ok {
				info.ResourceName = resource.Name
				info.ResourceType = resource.Type
			}
			actions = append(actions, info)
		}
		if len(actions) == 0 && rule.DataResourceId != "" {
			info := dtos.RuleEngineActionInfo{ResourceId: rule.DataResourceId, ResourceType: rule.ResourceType}
			if resource, ok := resourceMap[rule.DataResourceId]; ok {
				info.ResourceName = resource.Name
				info.ResourceType = resource.Type
			}
			actions = append(actions, info)
		}
		tags := []string(rule.Tags)
		if tags == nil {
			tags = []string{}
		}
		filter := rule.Filter
		// 旧数据的 filter.sql 实际是编译后的 SQL：返回前剥掉租户条件，编辑器和详情展示原始 SQL
		if strings.TrimSpace(filter.CompiledSql) == "" && rule.TenantId != "" {
			if stripped, changed := stripTenantPredicates(filter.Sql); changed {
				filter.Sql = stripped
			}
		}
		item := dtos.RuleEngineSearchQueryResponse{
			Id:             rule.Id,
			Name:           rule.Name,
			Description:    rule.Describe,
			Tags:           tags,
			Actions:        actions,
			Options:        rule.Options,
			ResourceType:   rule.ResourceType,
			Status:         rule.Status,
			Created:        rule.Created,
			Modified:       rule.Modified,
			Filter:         filter,
			DataResourceId: rule.DataResourceId,
		}
		// 资源被删除时也要返回空对象：前端会直接读 dataResource.type
		item.DataResource = dtos.DataResourceInfo{}
		if resource, ok := resourceMap[rule.DataResourceId]; ok {
			item.DataResource = dtos.DataResourceInfo{
				Name: resource.Name,
				Type: resource.Type,
			}
			if item.ResourceType == "" {
				item.ResourceType = resource.Type
			}
		}
		if len(actions) > 0 && item.ResourceType == "" {
			item.ResourceType = actions[0].ResourceType
		}
		out = append(out, item)
	}
	return out, nil
}

func importResourceKey(name, typ string) string {
	return strings.TrimSpace(name) + "\x00" + strings.TrimSpace(typ)
}

// resolveImportResource 按导出的 (name,type) 找当前租户的数据资源。
// 只有导出动作没有类型时才按名称回退；名称对应多个类型时视为无法确定，避免绑定到错误类型的资源。
func resolveImportResource(byKey map[string]string, byName map[string][]string, name, typ string) string {
	name = strings.TrimSpace(name)
	typ = strings.TrimSpace(typ)
	if typ != "" {
		return byKey[importResourceKey(name, typ)]
	}
	ids := byName[name]
	if len(ids) == 1 {
		return ids[0]
	}
	return ""
}

func paramError(message string, params map[string]interface{}) error {
	return errort.NewCommonErr(errort.DefaultReqParamsError, errort.NewMessageErr(message, params))
}

func errorMessage(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

/** ***************************** 状态同步 ***************************** **/

// monitor 周期性把 eKuiper 的真实运行状态同步回库
// （规则可能因 SQL 出错被 eKuiper 自行停止，仅靠启停操作写库会与真实状态不一致）
func (app *ruleEngineApp) monitor() {
	ticker := time.NewTicker(monitorInterval)
	defer ticker.Stop()
	for range ticker.C {
		// 上一轮未结束时跳过本轮，避免在 eKuiper/DB 变慢时任务堆积
		select {
		case app.syncGate <- struct{}{}:
			func() {
				defer func() {
					<-app.syncGate
					if r := recover(); r != nil {
						app.lc.Errorf("rule engine status sync panic: %v", r)
					}
				}()
				app.syncStatus()
			}()
		default:
			app.lc.Warnf("previous rule engine status sync still running, skip this round")
		}
	}
}

func (app *ruleEngineApp) syncStatus() {
	ctx, cancel := context.WithTimeout(context.Background(), monitorTimeout)
	defer cancel()

	// 平台作用域：系统路径，不按租户过滤
	list, _, err := app.dbClient.WithCtx(tenant.Platform(context.Background())).RuleEngineSearch(0, monitorBatchSize, dtos.RuleEngineSearchQueryRequest{})
	if err != nil {
		app.lc.Warnf("rule engine status sync: query failed: %v", err)
		return
	}
	ekuiper := app.ekuiper()
	for i := range list {
		rule := list[i]
		// 最近刚被启停/编辑过的规则跳过本轮同步，避免与人工操作竞态把状态改回去
		if rule.Modified > 0 && time.Now().UnixMilli()-rule.Modified < statusSyncSkipWindow.Milliseconds() {
			continue
		}
		stats, err := ekuiper.GetRuleStats(ctx, rule.Id)
		if err != nil {
			// eKuiper 不可达或规则不存在时不改动库中状态
			continue
		}
		status, _ := stats["status"].(string)
		if status == "" || status == rule.Status {
			continue
		}
		if status != string(constants.RuleEngineStart) && status != string(constants.RuleEngineStop) {
			continue
		}
		if err = app.dbClient.WithCtx(tenant.Platform(context.Background())).UpdateRuleEngineStatus(rule.Id, status); err != nil {
			app.lc.Warnf("rule engine status sync: update %s failed: %v", rule.Id, err)
			continue
		}
		app.lc.Infof("rule engine %s status synced to %s", rule.Id, status)
	}
}

// resyncRunningRuleActions 启动后把运行中规则的动作重新下发一次。
// 规则动作里可能带回调密钥等配置，配置变更后若不重下发，旧规则会一直用旧动作。
func (app *ruleEngineApp) resyncRunningRuleActions() {
	defer func() {
		if r := recover(); r != nil {
			app.lc.Errorf("resync rule actions panic: %v", r)
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), resyncTimeout)
	defer cancel()

	// 平台作用域：系统路径，不按租户过滤
	list, _, err := app.dbClient.WithCtx(tenant.Platform(context.Background())).RuleEngineSearch(0, monitorBatchSize,
		dtos.RuleEngineSearchQueryRequest{Status: string(constants.RuleEngineStart)})
	if err != nil {
		app.lc.Warnf("resync rule actions: query failed: %v", err)
		return
	}
	ekuiper := app.ekuiper()
	for i := range list {
		rule := list[i]
		// 资源按规则所属租户解析：后台任务本身没有租户上下文，直接查会漏掉租户资源
		ruleCtx := tenant.Platform(ctx)
		if rule.TenantId != "" {
			ruleCtx = tenant.With(ctx, tenant.Context{TenantID: rule.TenantId})
		}
		_, actions, err := app.resolveRuleActions(ruleCtx, rule)
		if err != nil {
			app.lc.Warnf("resync rule %s: resolve actions failed: %v", rule.Id, err)
			continue
		}
		tenantID := rule.TenantId
		sql, err := storedCompiledSQL(rule, tenantID)
		if err != nil {
			app.lc.Warnf("resync rule %s: compile sql failed: %v", rule.Id, err)
			continue
		}
		exist, err := ekuiper.RuleExist(ctx, rule.Id)
		if err != nil || !exist {
			continue
		}
		// 记录下发前的运行状态：UpdateRule 的动作模板 triggered=false，
		// 会把正在运行的规则停掉，因此下发后需要恢复
		wasRunning := app.isRuleRunning(ctx, rule.Id)
		def := ekuiperRule(rule, sql, actions)
		if err = app.applyEkuiperRule(ctx, def, rule.Id); err != nil {
			app.lc.Warnf("resync rule %s failed: %v", rule.Id, err)
			continue
		}
		if wasRunning {
			if err = ekuiper.StartRule(ctx, rule.Id); err != nil {
				app.lc.Warnf("resync rule %s restart failed: %v", rule.Id, err)
				continue
			}
		}
		app.lc.Infof("rule %s resynced (running=%v)", rule.Id, wasRunning)
	}
}

// isRuleRunning 查询 eKuiper 侧规则是否在运行
func (app *ruleEngineApp) isRuleRunning(ctx context.Context, ruleId string) bool {
	stats, err := app.ekuiper().GetRuleStats(ctx, ruleId)
	if err != nil {
		return false
	}
	status, _ := stats["status"].(string)
	return status == string(constants.RuleEngineStart)
}

func (app *ruleEngineApp) ekuiper() ekuiperclient.EkuiperClient {
	return container.EkuiperAppFrom(app.dic.Get)
}
