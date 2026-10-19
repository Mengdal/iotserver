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
}

// New 构造网关应用
func New(dic *di.Container) interfaces.GatewayItf {
	return &gatewayApp{
		dic:      dic,
		dbClient: container.DBClientFrom(dic.Get),
		lc:       pkgcontainer.LoggingClientFrom(dic.Get),
	}
}

func toGatewayResponse(g models.Gateway, deviceCount int64, deptNames map[string]string) dtos.GatewayResponse {
	return dtos.GatewayResponse{
		Id:              g.Id,
		Name:            g.Name,
		Sn:              g.Sn,
		Pk:              g.Pk,
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
	}
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
	var res dtos.GatewayResponse
	// 配额校验：目标租户的网关数 + 1 是否超过 max_gateways（0=不限；平台视图不校验）
	if err := tenantquota.Ensure(ctx, app.dbClient, app.lc, tenantquota.Gateways); err != nil {
		return res, err
	}
	sn := strings.TrimSpace(req.Sn)
	if sn == "" {
		return res, errort.NewCommonErr(errort.DefaultReqParamsError, errort.NewMessageErr("网关序列号不能为空", nil))
	}
	g, err := app.dbClient.WithCtx(ctx).AddGateway(models.Gateway{
		Name:   strings.TrimSpace(req.Name),
		Sn:     sn,
		Pk:     strings.TrimSpace(req.Pk),
		DeptId: req.DeptId,
		Vendor: req.Vendor,
		Model:  req.Model,
		Remark: req.Remark,
	})
	if err != nil {
		return res, err
	}
	res = toGatewayResponse(g, 0, map[string]string{})
	// 密钥只在创建时返回一次，供网关侧配置
	res.Mqtt = app.mqttInfo(g)
	return res, nil
}

// GatewayUpdate 修改网关
func (app *gatewayApp) GatewayUpdate(ctx context.Context, req dtos.GatewayUpdateRequest) error {
	if _, err := app.dbClient.WithCtx(ctx).GatewayById(req.Id); err != nil {
		return err
	}
	return app.dbClient.WithCtx(ctx).UpdateGateway(models.Gateway{
		Id:     req.Id,
		Name:   strings.TrimSpace(req.Name),
		Pk:     strings.TrimSpace(req.Pk),
		DeptId: req.DeptId,
		Vendor: req.Vendor,
		Model:  req.Model,
		Remark: req.Remark,
	})
}

// GatewayDelete 删除网关
//
// 只删网关本身；其下设备保留但解除绑定，避免误删采集数据。
func (app *gatewayApp) GatewayDelete(ctx context.Context, id string) error {
	return app.dbClient.WithCtx(ctx).DeleteGateway(id)
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
