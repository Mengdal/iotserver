package ruleengineapp

import (
	"context"
	"fmt"
	"strings"
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
	"iotserver/internal/pkg/utils"
	"iotserver/internal/pkg/valid"
	"iotserver/internal/tools/ekuiperclient"
)

const (
	// maxDescriptionLen 描述上限（与模型列宽一致）
	maxDescriptionLen = 255
	// maxSqlLen 规则 SQL 上限，防止超大请求体
	maxSqlLen = 10000
	// monitorInterval 运行状态同步周期
	monitorInterval = 5 * time.Second
	monitorTimeout  = 3 * time.Second
	// monitorBatchSize 每轮同步的规则条数上限
	monitorBatchSize = 200
	// statusSyncSkipWindow 人工启停/编辑后的静默窗口，窗口内不做状态回写（避免 UI 回弹）
	statusSyncSkipWindow = 10 * time.Second
	// resyncTimeout 启动后重下发动作的整体超时
	resyncTimeout = 30 * time.Second
)

type ruleEngineApp struct {
	dic      *di.Container
	dbClient interfaces.DBClient
	lc       logger.LoggingClient
	// syncGate 保证同一时刻只有一轮状态同步在跑
	syncGate chan struct{}
}

func New(dic *di.Container) *ruleEngineApp {
	app := &ruleEngineApp{
		dic:      dic,
		dbClient: container.DBClientFrom(dic.Get),
		lc:       pkgcontainer.LoggingClientFrom(dic.Get),
		syncGate: make(chan struct{}, 1),
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
		dic:      dic,
		dbClient: container.DBClientFrom(dic.Get),
		lc:       pkgcontainer.LoggingClientFrom(dic.Get),
		syncGate: make(chan struct{}, 1),
	}
}

/** ***************************** 查询 ***************************** **/

// RuleEngineSearch 规则列表
func (app *ruleEngineApp) RuleEngineSearch(ctx context.Context, req dtos.RuleEngineSearchQueryRequest) ([]dtos.RuleEngineSearchQueryResponse, uint32, error) {
	offset, limit := dtos.OffsetLimit(req.BaseSearchConditionQuery)
	list, total, err := app.dbClient.RuleEngineSearch(offset, limit, req)
	if err != nil {
		return nil, 0, err
	}
	out, err := app.toResponses(list)
	if err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

// RuleEngineById 规则详情（含绑定的数据资源）
func (app *ruleEngineApp) RuleEngineById(ctx context.Context, id string) (dtos.RuleEngineSearchQueryResponse, error) {
	rule, err := app.dbClient.RuleEngineById(id)
	if err != nil {
		return dtos.RuleEngineSearchQueryResponse{}, err
	}
	list, err := app.toResponses([]models.RuleEngine{rule})
	if err != nil {
		return dtos.RuleEngineSearchQueryResponse{}, err
	}
	return list[0], nil
}

// RuleEngineStatus 规则运行状态（直接返回 eKuiper 的规则指标，前端以 JSON 展示）
func (app *ruleEngineApp) RuleEngineStatus(ctx context.Context, id string) (map[string]interface{}, error) {
	if _, err := app.dbClient.RuleEngineById(id); err != nil {
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
	if err := checkTextLen(req.Description, req.Filter.Sql); err != nil {
		return err
	}
	resource, err := app.checkParam(req.Name, req.DataResourceId, req.ResourceTypeValue())
	if err != nil {
		return err
	}
	sql := dtos.BuildRuleEngineSql(req.Filter.Sql, req.Filter.SelectName, req.Filter.Condition)
	actions, err := dtos.GetRuleEngineEkuiperActions(resource.Type, resource.Option)
	if err != nil {
		return errort.NewCommonErr(errort.DefaultReqParamsError, err)
	}

	rule := models.RuleEngine{
		Id:             utils.RandomNum(),
		Name:           req.Name,
		Describe:       req.Description,
		Status:         string(constants.RuleEngineStop),
		DataResourceId: resource.Id,
		ResourceType:   resource.Type,
		Filter:         req.Filter,
	}
	// SQL 以本次实际下发的内容为准
	rule.Filter.Sql = sql

	ekuiper := app.ekuiper()
	exist, err := ekuiper.RuleExist(ctx, rule.Id)
	if err != nil {
		return err
	}
	created := !exist
	if exist {
		if err = ekuiper.UpdateRule(ctx, actions, rule.Id, sql); err != nil {
			return err
		}
	} else if err = ekuiper.CreateRule(ctx, actions, rule.Id, sql); err != nil {
		return err
	}

	if _, err = app.dbClient.AddRuleEngine(rule); err != nil {
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
	return nil
}

// RuleEngineUpdate 修改规则：同步更新 eKuiper 规则
func (app *ruleEngineApp) RuleEngineUpdate(ctx context.Context, req dtos.RuleEngineUpdateRequest) error {
	if strings.TrimSpace(req.Id) == "" {
		return errort.NewCommonErr(errort.DefaultIdEmpty, fmt.Errorf("id is empty"))
	}
	if err := checkTextLen(req.Description, req.Filter.Sql); err != nil {
		return err
	}
	old, err := app.dbClient.RuleEngineById(req.Id)
	if err != nil {
		return err
	}
	// 运行中的规则先停止再修改，避免下发与运行状态不一致
	if old.Status == string(constants.RuleEngineStart) {
		return errort.NewCommonErr(errort.RuleEngineIsStartingNotAllowUpdate, nil)
	}
	resource, err := app.checkParam(req.Name, req.DataResourceId, req.ResourceTypeValue())
	if err != nil {
		return err
	}
	sql := dtos.BuildRuleEngineSql(req.Filter.Sql, req.Filter.SelectName, req.Filter.Condition)
	actions, err := dtos.GetRuleEngineEkuiperActions(resource.Type, resource.Option)
	if err != nil {
		return errort.NewCommonErr(errort.DefaultReqParamsError, err)
	}

	ekuiper := app.ekuiper()
	exist, err := ekuiper.RuleExist(ctx, old.Id)
	if err != nil {
		return err
	}
	if exist {
		if err = ekuiper.UpdateRule(ctx, actions, old.Id, sql); err != nil {
			return err
		}
	} else if err = ekuiper.CreateRule(ctx, actions, old.Id, sql); err != nil {
		return err
	}

	filter := req.Filter
	filter.Sql = sql
	return app.dbClient.UpdateRuleEngine(models.RuleEngine{
		Id:             old.Id,
		Name:           req.Name,
		Describe:       req.Description,
		DataResourceId: resource.Id,
		ResourceType:   resource.Type,
		Filter:         filter,
	})
}

// RuleEngineDelete 删除规则：先删 eKuiper 规则（不存在则忽略），再删库
func (app *ruleEngineApp) RuleEngineDelete(ctx context.Context, id string) error {
	rule, err := app.dbClient.RuleEngineById(id)
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
	return app.dbClient.DeleteRuleEngine(id)
}

/** ***************************** 启停 ***************************** **/

// RuleEngineStart 启动规则
func (app *ruleEngineApp) RuleEngineStart(ctx context.Context, id string) error {
	rule, err := app.dbClient.RuleEngineById(id)
	if err != nil {
		return err
	}
	// 绑定资源可能已被删除或改动，启动前重新校验并按最新配置下发
	resource, err := app.checkParam(rule.Name, rule.DataResourceId, rule.ResourceType)
	if err != nil {
		return err
	}
	actions, err := dtos.GetRuleEngineEkuiperActions(resource.Type, resource.Option)
	if err != nil {
		return errort.NewCommonErr(errort.DefaultReqParamsError, err)
	}

	ekuiper := app.ekuiper()
	exist, err := ekuiper.RuleExist(ctx, rule.Id)
	if err != nil {
		return err
	}
	// eKuiper 侧规则可能被清理（如 eKuiper 重启/数据卷重建），此处按库中配置重建
	if !exist {
		if err = ekuiper.CreateRule(ctx, actions, rule.Id, rule.Filter.Sql); err != nil {
			return err
		}
	} else {
		if err = ekuiper.UpdateRule(ctx, actions, rule.Id, rule.Filter.Sql); err != nil {
			return err
		}
	}
	if err = ekuiper.StartRule(ctx, rule.Id); err != nil {
		return err
	}
	return app.dbClient.UpdateRuleEngineStatus(rule.Id, string(constants.RuleEngineStart))
}

// RuleEngineStop 停止规则
func (app *ruleEngineApp) RuleEngineStop(ctx context.Context, id string) error {
	rule, err := app.dbClient.RuleEngineById(id)
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
	return app.dbClient.UpdateRuleEngineStatus(rule.Id, string(constants.RuleEngineStop))
}

/** ***************************** 内部 ***************************** **/

// checkParam 校验名称与绑定资源，返回资源实体
func (app *ruleEngineApp) checkParam(name, dataResourceId, resourceType string) (models.DataResource, error) {
	if len([]rune(dataResourceId)) > 64 {
		return models.DataResource{}, errort.NewCommonErr(errort.DefaultReqParamsError, fmt.Errorf("数据资源ID非法"))
	}
	if !valid.Name(name) {
		return models.DataResource{}, errort.NewCommonErr(errort.DefaultNameSpecialCharacters,
			fmt.Errorf("%s", valid.NameRuleMsg))
	}
	if strings.TrimSpace(dataResourceId) == "" {
		return models.DataResource{}, errort.NewCommonErr(errort.DefaultReqParamsError,
			fmt.Errorf("请选择数据资源"))
	}
	resource, err := app.dbClient.DataResourceById(dataResourceId)
	if err != nil {
		return models.DataResource{}, errort.NewCommonErr(errort.InvalidSource,
			fmt.Errorf("数据资源 %s 不存在或已被删除", dataResourceId))
	}
	// 前端单选值优先，其次以资源的真实类型为准
	if strings.TrimSpace(resourceType) != "" && resourceType != resource.Type {
		app.lc.Warnf("rule engine resource type %s mismatch with data resource %s(%s), use data resource type",
			resourceType, resource.Name, resource.Type)
	}
	return resource, nil
}

// toResponses 转出参，并批量补齐绑定的数据资源
func (app *ruleEngineApp) toResponses(list []models.RuleEngine) ([]dtos.RuleEngineSearchQueryResponse, error) {
	if len(list) == 0 {
		return []dtos.RuleEngineSearchQueryResponse{}, nil
	}
	ids := make([]string, 0, len(list))
	for i := range list {
		if list[i].DataResourceId != "" {
			ids = append(ids, list[i].DataResourceId)
		}
	}
	resources, err := app.dbClient.DataResourceByIds(ids)
	if err != nil {
		return nil, err
	}
	resourceMap := make(map[string]models.DataResource, len(resources))
	for i := range resources {
		resourceMap[resources[i].Id] = resources[i]
	}

	out := make([]dtos.RuleEngineSearchQueryResponse, 0, len(list))
	for i := range list {
		rule := list[i]
		item := dtos.RuleEngineSearchQueryResponse{
			Id:             rule.Id,
			Name:           rule.Name,
			Description:    rule.Describe,
			ResourceType:   rule.ResourceType,
			Status:         rule.Status,
			Created:        rule.Created,
			Modified:       rule.Modified,
			Filter:         rule.Filter,
			DataResourceId: rule.DataResourceId,
		}
		// 资源被删除时也要返回空对象：前端会直接读 dataResource.type
		item.DataResource = dtos.DataResourceInfo{}
		if resource, ok := resourceMap[rule.DataResourceId]; ok {
			item.DataResource = dtos.DataResourceInfo{
				Name:   resource.Name,
				Option: map[string]interface{}(resource.Option),
				Type:   resource.Type,
			}
			if item.ResourceType == "" {
				item.ResourceType = resource.Type
			}
		}
		out = append(out, item)
	}
	return out, nil
}

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

	list, _, err := app.dbClient.RuleEngineSearch(0, monitorBatchSize, dtos.RuleEngineSearchQueryRequest{})
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
		if err = app.dbClient.UpdateRuleEngineStatus(rule.Id, status); err != nil {
			app.lc.Warnf("rule engine status sync: update %s failed: %v", rule.Id, err)
			continue
		}
		app.lc.Infof("rule engine %s status synced to %s", rule.Id, status)
	}
}

// checkTextLen 描述与 SQL 长度校验
func checkTextLen(description, sql string) error {
	if len([]rune(description)) > maxDescriptionLen {
		return errort.NewCommonErr(errort.DefaultReqParamsError,
			fmt.Errorf("规则描述长度不能超过 %d 个字符", maxDescriptionLen))
	}
	if len(sql) > maxSqlLen {
		return errort.NewCommonErr(errort.DefaultReqParamsError, fmt.Errorf("规则SQL过长"))
	}
	return nil
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

	list, _, err := app.dbClient.RuleEngineSearch(0, monitorBatchSize,
		dtos.RuleEngineSearchQueryRequest{Status: string(constants.RuleEngineStart)})
	if err != nil {
		app.lc.Warnf("resync rule actions: query failed: %v", err)
		return
	}
	ekuiper := app.ekuiper()
	for i := range list {
		rule := list[i]
		resource, err := app.dbClient.DataResourceById(rule.DataResourceId)
		if err != nil {
			app.lc.Warnf("resync rule %s: data resource %s not found", rule.Id, rule.DataResourceId)
			continue
		}
		actions, err := dtos.GetRuleEngineEkuiperActions(resource.Type, resource.Option)
		if err != nil {
			app.lc.Warnf("resync rule %s: build actions failed: %v", rule.Id, err)
			continue
		}
		exist, err := ekuiper.RuleExist(ctx, rule.Id)
		if err != nil || !exist {
			continue
		}
		// 记录下发前的运行状态：UpdateRule 的动作模板 triggered=false，
		// 会把正在运行的规则停掉，因此下发后需要恢复
		wasRunning := app.isRuleRunning(ctx, rule.Id)
		if err = ekuiper.UpdateRule(ctx, actions, rule.Id, rule.Filter.Sql); err != nil {
			app.lc.Warnf("resync rule %s actions failed: %v", rule.Id, err)
			continue
		}
		if wasRunning {
			if err = ekuiper.StartRule(ctx, rule.Id); err != nil {
				app.lc.Warnf("resync rule %s restart failed: %v", rule.Id, err)
				continue
			}
		}
		app.lc.Infof("rule %s actions resynced (running=%v)", rule.Id, wasRunning)
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
