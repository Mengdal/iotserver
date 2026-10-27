package gatewayapp

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"iotserver/internal/dtos"
	"iotserver/internal/iotserver/core/application/tenantquota"
	"iotserver/internal/iotserver/core/container"
	interfaces "iotserver/internal/iotserver/core/interface"
	"iotserver/internal/models"
	pkgcontainer "iotserver/internal/pkg/container"
	"iotserver/internal/pkg/di"
	"iotserver/internal/pkg/errort"
	"iotserver/internal/pkg/logger"
	"iotserver/internal/pkg/tenant"
	"iotserver/internal/tools/gatewaymqtt"
)

type gatewayApp struct {
	dic      *di.Container
	dbClient interfaces.DBClient
	lc       logger.LoggingClient
	things   interfaces.ThingsItf
}

// New 构造网关应用
func New(dic *di.Container) interfaces.GatewayItf {
	return &gatewayApp{
		dic:      dic,
		dbClient: container.DBClientFrom(dic.Get),
		lc:       pkgcontainer.LoggingClientFrom(dic.Get),
		things:   container.ThingsItfFrom(dic.Get),
	}
}

func toGatewayResponse(g models.Gateway, deviceCount int64, deptNames map[string]string) dtos.GatewayResponse {
	return dtos.GatewayResponse{
		Id:              g.Id,
		Name:            g.Name,
		Sn:              g.Sn,
		Pk:              g.Pk,
		Protocol:        g.Protocol,
		DeptId:          g.DeptId,
		DeptName:        deptNames[g.DeptId],
		Vendor:          g.Vendor,
		Model:           g.Model,
		FirmwareVersion: g.FirmwareVersion,
		Config:          g.Config,
		ConfigVersion:   g.ConfigVersion,
		Status:          g.Status,
		LastOnlineAt:    g.LastOnlineAt,
		LastReportAt:    g.LastReportAt,
		Ip:              g.Ip,
		Remark:          g.Remark,
		DeviceCount:     deviceCount,
		Created:         g.Created,
		Modified:        g.Modified,
		Source:          sourceOf(g),
		ThingsGatewayId: g.ThingsGatewayId,
		ThingsProjectId: g.ThingsProjectId,
		ThingsSyncStatus: func() string {
			if g.ThingsSyncStatus == "" {
				if sourceOf(g) == models.GatewaySourceThings {
					return models.GatewayThingsSyncSynced
				}
				return models.GatewayThingsSyncLocal
			}
			return g.ThingsSyncStatus
		}(),
		ThingsLastSyncAt: g.ThingsLastSyncAt,
		ThingsLastError:  g.ThingsLastError,
	}
}

func sourceOf(g models.Gateway) string {
	if g.Source == "" {
		return models.GatewaySourceLocal
	}
	return g.Source
}

// deptNameMap 批量取组织节点名称（网关列表展示「所属」用，避免 N+1 查询）
func (app *gatewayApp) deptNameMap(ctx context.Context) map[string]string {
	depts, _, err := app.dbClient.WithCtx(ctx).SysDeptSearch(0, -1, dtos.SysDeptSearchRequest{IsAll: true})
	if err != nil {
		return map[string]string{}
	}
	out := make(map[string]string, len(depts))
	for _, d := range depts {
		out[d.Id] = d.Name
	}
	return out
}

// mqttInfo 组装网关接入平台所需的 MQTT 参数（仅新增/重置密钥时返回）
func (app *gatewayApp) mqttInfo(g models.Gateway) *dtos.GatewayMqttInfo {
	cfg := container.ConfigurationFrom(app.dic.Get)
	return &dtos.GatewayMqttInfo{
		Broker:    cfg.MessageQueue.URL(),
		ClientId:  g.Sn,
		Username:  g.Sn,
		Password:  g.Secret,
		TopicUp:   gatewaymqtt.TopicPrefixUp + g.Sn + "/{data|batch|status|offline}",
		TopicDown: gatewaymqtt.TopicPrefixDown + g.Sn + "/{config|cmd}",
	}
}

// GatewayAdd 新增网关
func (app *gatewayApp) GatewayAdd(ctx context.Context, req dtos.GatewayAddRequest) (dtos.GatewayResponse, error) {
	source := strings.TrimSpace(req.Source)
	if source == "" {
		source = models.GatewaySourceLocal
	}
	if source == models.GatewaySourceThings {
		return app.gatewayAddThings(ctx, req)
	}
	if source != models.GatewaySourceLocal {
		return dtos.GatewayResponse{}, errort.NewCommonErr(errort.DefaultReqParamsError,
			errort.NewMessageErr("不支持的网关来源 {{.p1}}", map[string]interface{}{"p1": source}))
	}

	// 配额校验：目标租户的网关数 + 1 是否超过 max_gateways（0=不限；平台视图不校验）
	if err := tenantquota.Ensure(ctx, app.dbClient, app.lc, tenantquota.Gateways); err != nil {
		return dtos.GatewayResponse{}, err
	}
	sn := strings.TrimSpace(req.Sn)
	if sn == "" {
		return dtos.GatewayResponse{}, errort.NewCommonErr(errort.DefaultReqParamsError, errort.NewMessageErr("网关序列号不能为空", nil))
	}
	g, err := app.dbClient.WithCtx(ctx).AddGateway(models.Gateway{
		Name:     strings.TrimSpace(req.Name),
		Sn:       sn,
		Pk:       strings.TrimSpace(req.Pk),
		Protocol: strings.TrimSpace(req.Protocol),
		Source:   models.GatewaySourceLocal,
		DeptId:   req.DeptId,
		Vendor:   req.Vendor,
		Model:    req.Model,
		Remark:   req.Remark,
	})
	if err != nil {
		return dtos.GatewayResponse{}, err
	}
	res := toGatewayResponse(g, 0, map[string]string{})
	// 密钥只在创建时返回一次，供网关侧配置
	res.Mqtt = app.mqttInfo(g)
	return res, nil
}

func (app *gatewayApp) gatewayAddThings(ctx context.Context, req dtos.GatewayAddRequest) (dtos.GatewayResponse, error) {
	if app.things == nil {
		return dtos.GatewayResponse{}, errort.NewCommonErr(errort.AppThingsDisabled,
			errort.NewMessageErr("things 集成未启用", nil))
	}
	tenantID := tenant.FromContext(ctx).TenantID
	if tenantID == "" {
		return dtos.GatewayResponse{}, errort.NewCommonErr(errort.AppThingsBindingMissing,
			errort.NewMessageErr("当前租户尚未绑定 things 项目", nil))
	}
	binding, found, err := app.things.GetTenantBinding(ctx, tenantID)
	if err != nil {
		return dtos.GatewayResponse{}, err
	}
	if !found {
		return dtos.GatewayResponse{}, errort.NewCommonErr(errort.AppThingsBindingMissing,
			errort.NewMessageErr("当前租户尚未绑定 things 项目", nil))
	}
	// 配额校验与本地网关共用同一租户配额
	if err = tenantquota.Ensure(ctx, app.dbClient, app.lc, tenantquota.Gateways); err != nil {
		return dtos.GatewayResponse{}, err
	}
	if strings.TrimSpace(req.Sn) == "" {
		return dtos.GatewayResponse{}, errort.NewCommonErr(errort.DefaultReqParamsError,
			errort.NewMessageErr("网关序列号不能为空", nil))
	}
	// 只做必填检查；secret/license 的格式与有效性完全交给 things 判定。
	if req.Secret == "" {
		return dtos.GatewayResponse{}, errort.NewCommonErr(errort.DefaultReqParamsError,
			errort.NewMessageErr("网关密钥不能为空", nil))
	}
	if strings.TrimSpace(req.Name) == "" || strings.TrimSpace(req.Model) == "" {
		return dtos.GatewayResponse{}, errort.NewCommonErr(errort.DefaultReqParamsError,
			errort.NewMessageErr("网关名称和型号不能为空", nil))
	}

	tg, err := app.things.CreateGateway(ctx, binding.ThingsProjectId, dtos.ThingsGatewayCreateRequest{
		Sn:          strings.TrimSpace(req.Sn),
		Secret:      req.Secret,
		Name:        strings.TrimSpace(req.Name),
		Model:       strings.TrimSpace(req.Model),
		LicenseType: strings.TrimSpace(req.LicenseType),
		License:     req.License,
	})
	if err != nil {
		return dtos.GatewayResponse{}, err
	}

	g, err := app.dbClient.WithCtx(ctx).AddGateway(models.Gateway{
		Name:             tg.Name,
		Sn:               tg.Sn,
		Pk:               tg.Pk,
		Protocol:         models.GatewayProtocolEdge,
		Source:           models.GatewaySourceThings,
		Model:            tg.Model,
		ThingsGatewayId:  tg.Id,
		ThingsProjectId:  binding.ThingsProjectId,
		ThingsSyncStatus: models.GatewayThingsSyncSynced,
		ThingsLastSyncAt: time.Now().UnixMilli(),
		DeptId:           req.DeptId,
		Vendor:           req.Vendor,
		Remark:           req.Remark,
		Status:           models.GatewayStatusNever,
	})
	if err != nil {
		// 本地镜像失败时回滚 things 网关，避免出现只存在于 things 的孤儿网关。
		if dErr := app.things.DeleteGateway(ctx, binding.ThingsProjectId, tg.Id); dErr != nil && app.lc != nil {
			app.lc.Warnf("网关 %s 本地创建失败，回滚 things 网关 %d 也失败: %v", tg.Sn, tg.Id, dErr)
		}
		return dtos.GatewayResponse{}, err
	}
	return toGatewayResponse(g, 0, map[string]string{}), nil
}

// GatewayUpdate 修改网关
func (app *gatewayApp) GatewayUpdate(ctx context.Context, req dtos.GatewayUpdateRequest) error {
	old, err := app.dbClient.WithCtx(ctx).GatewayById(req.Id)
	if err != nil {
		return err
	}
	name := strings.TrimSpace(req.Name)
	model := strings.TrimSpace(req.Model)
	pk := strings.TrimSpace(req.Pk)
	if pk == "" {
		// 编辑表单不提交 pk；空值不能覆盖已有值，否则 /edge 主题会失效。
		pk = old.Pk
	}
	if sourceOf(old) == models.GatewaySourceThings {
		if name != "" && name != old.Name {
			return errort.NewCommonErr(errort.DefaultReqParamsError,
				errort.NewMessageErr("things 网关名称请在「things 重命名」中修改", nil))
		}
		if model != "" && model != old.Model {
			return errort.NewCommonErr(errort.DefaultReqParamsError,
				errort.NewMessageErr("things 网关型号由 things 管理，不能在这里修改", nil))
		}
		if name == "" {
			name = old.Name
		}
		if model == "" {
			model = old.Model
		}
	}
	return app.dbClient.WithCtx(ctx).UpdateGateway(models.Gateway{
		Id:     req.Id,
		Name:   name,
		Pk:     pk,
		DeptId: req.DeptId,
		Vendor: req.Vendor,
		Model:  model,
		Remark: req.Remark,
	})
}

// GatewayDelete 删除网关
//
// 只删网关本身；其下设备保留但解除绑定，避免误删采集数据。
func (app *gatewayApp) GatewayDelete(ctx context.Context, id string) error {
	g, err := app.dbClient.WithCtx(ctx).GatewayById(id)
	if err != nil {
		return err
	}
	if sourceOf(g) == models.GatewaySourceThings || g.ThingsGatewayId > 0 {
		if app.things == nil {
			return errort.NewCommonErr(errort.AppThingsDisabled,
				errort.NewMessageErr("things 集成未启用，无法删除 things 网关", nil))
		}
		tenantID := g.TenantId
		if tenantID == "" {
			tenantID = tenant.FromContext(ctx).TenantID
		}
		binding, found, bErr := app.things.GetTenantBinding(ctx, tenantID)
		if bErr != nil {
			return bErr
		}
		if !found {
			return errort.NewCommonErr(errort.AppThingsBindingMissing,
				errort.NewMessageErr("当前租户尚未绑定 things 项目，无法删除 things 网关", nil))
		}
		if g.ThingsGatewayId <= 0 {
			return errort.NewCommonErr(errort.AppThingsBindingMissing,
				errort.NewMessageErr("该网关缺少 things 网关 ID，请先执行对账", nil))
		}
		if err = app.things.DeleteGateway(ctx, binding.ThingsProjectId, g.ThingsGatewayId); err != nil {
			return err
		}
	}
	return app.dbClient.WithCtx(ctx).DeleteGateway(id)
}

// GatewayRenameThings 在 things 中重命名网关，并同步本地镜像名称。
func (app *gatewayApp) GatewayRenameThings(ctx context.Context, req dtos.GatewayThingsRenameRequest) error {
	if app.things == nil {
		return errort.NewCommonErr(errort.AppThingsDisabled,
			errort.NewMessageErr("things 集成未启用", nil))
	}
	g, err := app.dbClient.WithCtx(ctx).GatewayById(req.Id)
	if err != nil {
		return err
	}
	if sourceOf(g) != models.GatewaySourceThings || g.ThingsGatewayId <= 0 {
		return errort.NewCommonErr(errort.DefaultReqParamsError,
			errort.NewMessageErr("该网关不是 things 网关", nil))
	}
	tenantID := g.TenantId
	if tenantID == "" {
		tenantID = tenant.FromContext(ctx).TenantID
	}
	binding, found, bErr := app.things.GetTenantBinding(ctx, tenantID)
	if bErr != nil {
		return bErr
	}
	if !found {
		return errort.NewCommonErr(errort.AppThingsBindingMissing,
			errort.NewMessageErr("当前租户尚未绑定 things 项目", nil))
	}
	if err = app.things.RenameGateway(ctx, binding.ThingsProjectId, g.ThingsGatewayId, req.Name); err != nil {
		return err
	}
	return app.dbClient.WithCtx(ctx).UpdateGateway(models.Gateway{
		Id:     g.Id,
		Name:   strings.TrimSpace(req.Name),
		Pk:     g.Pk,
		DeptId: g.DeptId,
		Vendor: g.Vendor,
		Model:  g.Model,
		Remark: g.Remark,
	})
}

// GatewayById 网关详情
func (app *gatewayApp) GatewayById(ctx context.Context, id string) (dtos.GatewayResponse, error) {
	db := app.dbClient.WithCtx(ctx)
	g, err := db.GatewayById(id)
	if err != nil {
		return dtos.GatewayResponse{}, err
	}
	count, _ := db.GatewayDeviceCount(id)
	return toGatewayResponse(g, count, app.deptNameMap(ctx)), nil
}

// GatewaySearch 网关列表
func (app *gatewayApp) GatewaySearch(ctx context.Context, req dtos.GatewaySearchQueryRequest) ([]dtos.GatewayResponse, uint32, error) {
	offset, limit := req.GetPage()
	db := app.dbClient.WithCtx(ctx)
	list, total, err := db.GatewaySearch(offset, limit, req)
	if err != nil {
		return nil, 0, err
	}
	deptNames := app.deptNameMap(ctx)
	out := make([]dtos.GatewayResponse, len(list))
	for i, g := range list {
		count, _ := db.GatewayDeviceCount(g.Id)
		out[i] = toGatewayResponse(g, count, deptNames)
	}
	return out, total, nil
}

// GatewayPushConfig 下发采集/上报配置
func (app *gatewayApp) GatewayPushConfig(ctx context.Context, req dtos.GatewayConfigRequest) (int, error) {
	db := app.dbClient.WithCtx(ctx)
	g, err := db.GatewayById(req.Id)
	if err != nil {
		return 0, err
	}
	if !json.Valid([]byte(req.Config)) {
		return 0, errort.NewCommonErr(errort.DefaultReqParamsError, errort.NewMessageErr("配置必须是合法 JSON", nil))
	}
	version := g.ConfigVersion + 1
	if err = db.UpdateGatewayConfig(g.Id, req.Config, version); err != nil {
		return 0, err
	}
	payload, _ := json.Marshal(map[string]interface{}{
		"type":    "config",
		"version": version,
		"config":  json.RawMessage(req.Config),
		"ts":      time.Now().UnixMilli(),
	})
	// 下行通道不可用时不报错：配置已持久化，网关下次上线可通过 sync 命令拉取
	if err = gatewaymqtt.Publish(gatewaymqtt.DownTopic(g.Sn, "config"), payload); err != nil {
		if app.lc != nil {
			app.lc.Warnf("网关 %s 配置已保存(v%d)，但下行通道不可用: %v", g.Sn, version, err)
		}
	}
	return version, nil
}

// GatewayCommand 下发命令（restart / ota / sync）
func (app *gatewayApp) GatewayCommand(ctx context.Context, req dtos.GatewayCommandRequest) error {
	switch req.Command {
	case "restart", "ota", "sync":
	default:
		return errort.NewCommonErr(errort.DefaultReqParamsError,
			errort.NewMessageErr("不支持的命令 {{.p1}}（可选 restart/ota/sync）", map[string]interface{}{"p1": req.Command}))
	}
	g, err := app.dbClient.WithCtx(ctx).GatewayById(req.Id)
	if err != nil {
		return err
	}
	body := map[string]interface{}{
		"type": req.Command,
		"ts":   time.Now().UnixMilli(),
	}
	if strings.TrimSpace(req.Payload) != "" {
		if !json.Valid([]byte(req.Payload)) {
			return errort.NewCommonErr(errort.DefaultReqParamsError, errort.NewMessageErr("命令参数必须是合法 JSON", nil))
		}
		body["payload"] = json.RawMessage(req.Payload)
	}
	payload, _ := json.Marshal(body)
	if err = gatewaymqtt.Publish(gatewaymqtt.DownTopic(g.Sn, "cmd"), payload); err != nil {
		return errort.NewCommonEdgeX(errort.DefaultSystemError,
			"下行通道不可用，命令未下发（请确认网关接入服务已启动）", err)
	}
	return nil
}

// GatewayOnline 网关心跳/上线：更新在线状态与最近时间（接入服务在平台作用域下调用）
func (app *gatewayApp) GatewayOnline(ctx context.Context, sn, ip string, ts int64) error {
	g, err := app.dbClient.WithCtx(ctx).GatewayBySn(sn)
	if err != nil {
		return err
	}
	if ts <= 0 {
		ts = time.Now().UnixMilli()
	}
	return app.dbClient.WithCtx(ctx).UpdateGatewayStatus(g.Id, models.GatewayStatusOnline, ip, ts)
}

// GatewayOffline 网关离线（LWT 遗嘱或超时扫描）
func (app *gatewayApp) GatewayOffline(ctx context.Context, sn string) error {
	g, err := app.dbClient.WithCtx(ctx).GatewayBySn(sn)
	if err != nil {
		return err
	}
	// 平台作用域：由接入服务/LWT 触发，没有请求级租户上下文
	platformCtx := tenant.Platform(ctx)
	return app.dbClient.WithCtx(platformCtx).UpdateGatewayStatus(g.Id, models.GatewayStatusOffline, "", 0)
}
