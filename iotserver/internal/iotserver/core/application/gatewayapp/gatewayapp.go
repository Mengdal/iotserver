package gatewayapp

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"strconv"
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
	"iotserver/internal/pkg/utils"
	"iotserver/internal/tools/collectengine"
	"iotserver/internal/tools/gatewaymqtt"
	"iotserver/internal/tools/iotp"
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
		Id:               g.Id,
		Name:             g.Name,
		Sn:               g.Sn,
		Pk:               g.Pk,
		Protocol:         g.Protocol,
		EngineBaseURL:    g.EngineBaseURL,
		EngineTimeoutSec: g.EngineTimeoutSec,
		DeptId:           g.DeptId,
		DeptName:         deptNames[g.DeptId],
		Vendor:           g.Vendor,
		Model:            g.Model,
		FirmwareVersion:  g.FirmwareVersion,
		Config:           g.Config,
		ConfigVersion:    g.ConfigVersion,
		Status:           g.Status,
		LastOnlineAt:     g.LastOnlineAt,
		LastReportAt:     g.LastReportAt,
		Ip:               g.Ip,
		Remark:           g.Remark,
		// 排除名单：让界面能看见/撤销「删除设备后自动排除」的结果。
		ExcludeDeviceKeys: g.ExcludeDeviceKeys,
		DeviceCount:       deviceCount,
		Created:           g.Created,
		Modified:          g.Modified,
		Source:            sourceOf(g),
		ThingsGatewayId:   g.ThingsGatewayId,
		ThingsProjectId:   g.ThingsProjectId,
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

// normalizeEngineConfig 校验并规范化软网关的采集引擎配置。
// 地址必须带 http/https scheme 与主机名；超时缺省 10 秒，夹在 1~300 秒。
func normalizeEngineConfig(baseURL string, timeoutSec int) (string, int, error) {
	base := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if base == "" {
		return "", 0, errort.NewCommonErr(errort.DefaultReqParamsError,
			errort.NewMessageErr("采集引擎地址不能为空", nil))
	}
	parsed, err := url.Parse(base)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return "", 0, errort.NewCommonErr(errort.DefaultReqParamsError,
			errort.NewMessageErr("采集引擎地址格式不正确，应形如 http://host:8081", nil))
	}
	if timeoutSec <= 0 {
		timeoutSec = 10
	}
	if timeoutSec > 300 {
		timeoutSec = 300
	}
	return base, timeoutSec, nil
}

// configureEngineMQTT 把软网关的 MQTT 订阅参数写入采集引擎。
//
// 引擎由自己订阅 MQTT 并写入 iotp：IP 使用网关配置的地址（EngineBaseURL 的主机），
// clientid 使用网关 SN；username/password 不使用，传空清空。
// 端口沿用引擎当前值；写入哪张 iotp 表由每台设备自己的 measurement 决定。
func (app *gatewayApp) configureEngineMQTT(gateway models.Gateway) error {
	baseURL := strings.TrimSpace(gateway.EngineBaseURL)
	if baseURL == "" {
		return nil
	}
	parsed, err := url.Parse(baseURL)
	if err != nil || strings.TrimSpace(parsed.Hostname()) == "" {
		return fmt.Errorf("采集引擎地址格式不正确: %s", baseURL)
	}
	host := parsed.Hostname()
	if net.ParseIP(host) == nil {
		ips, lookupErr := net.LookupHost(host)
		if lookupErr != nil {
			return fmt.Errorf("解析采集引擎主机 %s 失败: %w", host, lookupErr)
		}
		for _, ip := range ips {
			if net.ParseIP(ip) != nil {
				host = ip
				break
			}
		}
	}
	timeoutSec := gateway.EngineTimeoutSec
	if timeoutSec <= 0 {
		timeoutSec = 10
	}
	client := collectengine.New(baseURL, time.Duration(timeoutSec)*time.Second)
	config, err := client.GetMQTTConfig()
	if err != nil {
		return fmt.Errorf("读取采集引擎 MQTT 配置失败: %w", err)
	}
	config.IP = host
	config.ClientID = strings.TrimSpace(gateway.Sn)
	config.Username = ""
	config.Password = ""
	if strings.TrimSpace(config.ClientID) == "" {
		return fmt.Errorf("网关 SN 为空，无法设置 MQTT clientid")
	}
	if err := client.UpdateMQTTConfig(config); err != nil {
		return fmt.Errorf("更新采集引擎 MQTT 配置失败: %w", err)
	}
	return nil
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
	if source != models.GatewaySourceLocal && source != models.GatewaySourceIoserver {
		return dtos.GatewayResponse{}, errort.NewCommonErr(errort.DefaultReqParamsError,
			errort.NewMessageErr("不支持的网关来源 {{.p1}}", map[string]interface{}{"p1": source}))
	}
	// ioserver 软网关：配置完全由本平台维护（Gateway.Config），不依赖 things。
	// 协议默认 ioserver，便于上行接入和后续采集配置下发分流。
	protocol := strings.TrimSpace(req.Protocol)
	engineBaseURL := ""
	engineTimeoutSec := 0
	if source == models.GatewaySourceIoserver {
		// 软网关不走 MQTT，协议固定，避免请求体把它改成 /edge 等接入协议。
		protocol = models.GatewayProtocolIoserver
		var err error
		engineBaseURL, engineTimeoutSec, err = normalizeEngineConfig(req.EngineBaseURL, req.EngineTimeoutSec)
		if err != nil {
			return dtos.GatewayResponse{}, err
		}
	}

	// 配额校验：目标租户的网关数 + 1 是否超过 max_gateways（0=不限；平台视图不校验）
	if err := tenantquota.Ensure(ctx, app.dbClient, app.lc, tenantquota.Gateways); err != nil {
		return dtos.GatewayResponse{}, err
	}
	sn := strings.TrimSpace(req.Sn)
	if sn == "" {
		if source == models.GatewaySourceIoserver {
			// 软网关不走 MQTT，SN 只作为内部唯一标识，自动生成 10 位数字。
			sn = utils.RandomNum10()
		} else {
			return dtos.GatewayResponse{}, errort.NewCommonErr(errort.DefaultReqParamsError, errort.NewMessageErr("网关序列号不能为空", nil))
		}
	}
	g, err := app.dbClient.WithCtx(ctx).AddGateway(models.Gateway{
		Name:             strings.TrimSpace(req.Name),
		Sn:               sn,
		Pk:               strings.TrimSpace(req.Pk),
		Protocol:         protocol,
		Source:           source,
		EngineBaseURL:    engineBaseURL,
		EngineTimeoutSec: engineTimeoutSec,
		DeptId:           req.DeptId,
		Vendor:           req.Vendor,
		Model:            req.Model,
		Remark:           req.Remark,
	})
	if err != nil {
		return dtos.GatewayResponse{}, err
	}
	res := toGatewayResponse(g, 0, map[string]string{})
	// 密钥只在创建时返回一次，供网关侧配置
	res.Mqtt = app.mqttInfo(g)
	// 软网关：更新采集引擎的 MQTT 订阅参数（IP=网关地址，clientid=SN）。
	if source == models.GatewaySourceIoserver {
		if err := app.configureEngineMQTT(g); err != nil && app.lc != nil {
			app.lc.Warnf("软网关 %s 更新采集引擎 MQTT 配置失败: %v", g.Sn, err)
		}
	}
	// 软网关同步到 iotp（异步、失败隔离）：声明网关，随后设备绑定时盖章。
	if source == models.GatewaySourceIoserver {
		if syncer := container.IotpSyncFrom(app.dic.Get); syncer != nil {
			syncer.GatewayCreated(g)
		}
	}
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
	// Secret 由平台生成、License 使用 things 默认值，不再要求用户填写。
	secret := utils.GenerateDeviceSecret(16)
	if strings.TrimSpace(req.Name) == "" || strings.TrimSpace(req.Model) == "" {
		return dtos.GatewayResponse{}, errort.NewCommonErr(errort.DefaultReqParamsError,
			errort.NewMessageErr("网关名称和型号不能为空", nil))
	}

	tg, err := app.things.CreateGateway(ctx, binding.ThingsProjectId, dtos.ThingsGatewayCreateRequest{
		Sn:     strings.TrimSpace(req.Sn),
		Secret: secret,
		Name:   strings.TrimSpace(req.Name),
		Model:  strings.TrimSpace(req.Model),
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
	// 用户重新创建了同一 SN 的 things 网关：清除此前的删除墓碑。
	if err := app.dbClient.WithCtx(ctx).DeleteGatewayTombstone(g.Sn); err != nil && app.lc != nil {
		app.lc.Warnf("清除网关 %s 的删除墓碑失败: %v", g.Sn, err)
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
				errort.NewMessageErr("things 网关名称由 things 平台管理，请到 things 控制台修改", nil))
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
	// 名称是必填展示字段：部分更新（如只改排除名单）没带 name 时保留原值，
	// 避免一次局部更新把网关名清空。
	if name == "" {
		name = old.Name
	}
	engineBaseURL := old.EngineBaseURL
	engineTimeoutSec := old.EngineTimeoutSec
	// 采集引擎地址只在本次提交了非空值时校验和更新：改名、改排除名单等
	// 部分更新不带 engineBaseUrl，不能因此失败；历史迁移出的空值网关也允许先改其它字段。
	if sourceOf(old) == models.GatewaySourceIoserver && strings.TrimSpace(req.EngineBaseURL) != "" {
		var engineErr error
		engineBaseURL, engineTimeoutSec, engineErr = normalizeEngineConfig(req.EngineBaseURL, req.EngineTimeoutSec)
		if engineErr != nil {
			return engineErr
		}
	}
	// 排除名单走独立更新：UpdateGateway 只写展示字段，避免这里漏填就把它清空。
	if req.ExcludeDeviceKeys != nil {
		keys := models.NormalizeGatewayExcludeKeys(*req.ExcludeDeviceKeys)
		if len(keys) > models.GatewayExcludeDeviceKeysMaxLen {
			return errort.NewCommonErr(errort.DefaultReqParamsError,
				errort.NewMessageErr("排除名单过长（上限 {{.p1}} 字符）", map[string]interface{}{
					"p1": models.GatewayExcludeDeviceKeysMaxLen}))
		}
		if err := app.dbClient.WithCtx(ctx).UpdateGatewayExcludeDeviceKeys(req.Id, keys); err != nil {
			return err
		}
	}
	updated := models.Gateway{
		Id:               req.Id,
		Name:             name,
		Pk:               pk,
		DeptId:           req.DeptId,
		Vendor:           req.Vendor,
		Model:            model,
		Remark:           req.Remark,
		EngineBaseURL:    engineBaseURL,
		EngineTimeoutSec: engineTimeoutSec,
	}
	if err := app.dbClient.WithCtx(ctx).UpdateGateway(updated); err != nil {
		return err
	}
	if sourceOf(old) == models.GatewaySourceIoserver {
		config := old
		config.EngineBaseURL = engineBaseURL
		config.EngineTimeoutSec = engineTimeoutSec
		if err := app.configureEngineMQTT(config); err != nil && app.lc != nil {
			app.lc.Warnf("软网关 %s 更新采集引擎 MQTT 配置失败: %v", old.Sn, err)
		}
	}
	return nil
}

// GatewayDelete 删除网关（只删本地记录）
//
// things 同步来的网关在这里只清理本地镜像，不调用 things 接口删除远端网关：
// 远端网关有自己的生命周期，平台不做反向删除。下次同步时远端仍然存在的网关
// 会被重新拉回来，这是预期行为。
//
// 本地删除时同时解除其下设备的采集绑定并删除该网关的通道；点位配置保留，
// 设备重新绑定到其他网关后仍可复用原来的点表。
func (app *gatewayApp) GatewayDelete(ctx context.Context, id string) error {
	g, err := app.dbClient.WithCtx(ctx).GatewayById(id)
	if err != nil {
		return err
	}
	if (sourceOf(g) == models.GatewaySourceThings || g.ThingsGatewayId > 0) && app.lc != nil {
		app.lc.Infof("删除 things 网关 %s 的本地镜像，远端网关保持不变", g.Sn)
	}
	// 删除前记录该网关下已绑定的设备，用于删除后清理 iotp 归属。
	deviceIDs := make([]int64, 0)
	if sourceOf(g) == models.GatewaySourceIoserver {
		if configs, listErr := app.dbClient.WithCtx(ctx).CollectDeviceConfigs(); listErr != nil {
			app.lc.Warnf("删除软网关 %s 前读取采集绑定失败（将跳过 iotp 清章）: %v", g.Sn, listErr)
		} else {
			for _, cfg := range configs {
				if cfg.GatewayId != id {
					continue
				}
				if deviceID, parseErr := strconv.ParseInt(strings.TrimSpace(cfg.DeviceId), 10, 64); parseErr == nil && deviceID > 0 {
					deviceIDs = append(deviceIDs, deviceID)
				}
			}
		}
	}
	// 先删网关下挂的设备，再删网关；有告警规则引用时中止，避免留下孤儿设备。
	if err := app.deleteGatewayDevices(ctx, id); err != nil {
		return err
	}
	// 软网关还要登记采集引擎清理任务，否则重新添加同一引擎会带出旧数据。
	// 只做本地登记，不在这里等待引擎响应。
	if sourceOf(g) == models.GatewaySourceIoserver {
		if collectApp := container.CollectConfigFrom(app.dic.Get); collectApp != nil {
			if err := collectApp.ScheduleGatewayCleanup(ctx, id); err != nil && app.lc != nil {
				app.lc.Warnf("登记软网关 %s 的采集引擎清理任务失败: %v", g.Sn, err)
			}
		}
	}
	if err := app.dbClient.WithCtx(ctx).DeleteGatewayWithCollectConfig(id); err != nil {
		return err
	}
	if sourceOf(g) == models.GatewaySourceIoserver {
		if syncer := container.IotpSyncFrom(app.dic.Get); syncer != nil {
			syncer.GatewayDeleted(g, deviceIDs)
		}
	}
	// things 远端仍是权威：只删本地镜像的话，下一轮对账会把网关重新导入。
	// 记录删除墓碑，对账时按 SN 跳过，保证「删了就是删了」。
	if sourceOf(g) == models.GatewaySourceThings || g.ThingsGatewayId > 0 {
		tenantID := g.TenantId
		if tenantID == "" {
			tenantID = tenant.FromContext(ctx).TenantID
		}
		tombstone := models.GatewayTombstone{
			TenantField:     models.TenantField{TenantId: tenantID},
			Sn:              g.Sn,
			ThingsGatewayId: g.ThingsGatewayId,
		}
		if err := app.dbClient.WithCtx(ctx).AddGatewayTombstone(tombstone); err != nil {
			if app.lc != nil {
				app.lc.Warnf("记录网关 %s 的删除墓碑失败，下一轮 things 对账可能重新导入: %v", g.Sn, err)
			}
		}
	}
	return nil
}

// deleteGatewayDevices 删除网关下挂的设备（Device.GatewayId = 网关 ID）。
//
// 软网关的设备关系在 collect_device_config，Device.GatewayId 为空，因此这里不会删除
// 手工创建、仅绑定到软网关的设备；它们只解除采集绑定，仍可复用于其他网关。
//
// 删除前先检查告警规则引用：任一设备被规则引用时整体中止，不删网关，
// 避免“设备删了一半、网关还在”的中间状态。
func (app *gatewayApp) deleteGatewayDevices(ctx context.Context, gatewayID string) error {
	db := app.dbClient.WithCtx(ctx)
	all, _, err := db.DevicesSearch(0, -1, dtos.DeviceSearchQueryRequest{
		BaseSearchConditionQuery: dtos.BaseSearchConditionQuery{IsAll: true},
	})
	if err != nil {
		return err
	}
	// 软网关的设备关系在 collect_device_config（Device.GatewayId 为空），
	// 因此除了 Device.GatewayId，还要按采集绑定把设备一并纳入删除范围。
	configs, err := db.CollectDeviceConfigs()
	if err != nil {
		return err
	}
	configByIotpID := make(map[int64]string)
	pointKeys := make([]string, 0)
	for _, cfg := range configs {
		if cfg.GatewayId != gatewayID {
			continue
		}
		pointKeys = append(pointKeys, cfg.DeviceId)
		if deviceID, parseErr := strconv.ParseInt(strings.TrimSpace(cfg.DeviceId), 10, 64); parseErr == nil && deviceID > 0 {
			configByIotpID[deviceID] = cfg.DeviceId
		}
	}

	targets := make([]models.Device, 0)
	seen := make(map[string]struct{})
	for _, device := range all {
		_, boundByCollect := configByIotpID[iotp.DeviceID(device.Id)]
		if device.GatewayId != gatewayID && !boundByCollect {
			continue
		}
		if _, ok := seen[device.Id]; ok {
			continue
		}
		seen[device.Id] = struct{}{}
		targets = append(targets, device)
	}
	if len(targets) == 0 && len(pointKeys) == 0 {
		return nil
	}

	alertApp := container.AlertRuleAppNameFrom(app.dic.Get)
	blocked := make([]string, 0)
	for _, device := range targets {
		if err := alertApp.CheckRuleByDeviceId(ctx, device.Id); err != nil {
			blocked = append(blocked, device.Name)
		}
	}
	if len(blocked) > 0 {
		return errort.NewCommonErr(errort.DefaultReqParamsError,
			errort.NewMessageErr("网关下的设备仍被告警规则引用，请先删除相关规则：{{.p1}}",
				map[string]interface{}{"p1": strings.Join(blocked, "、")}))
	}

	deviceApp := container.DeviceItfFrom(app.dic.Get)
	for _, device := range targets {
		if err := deviceApp.DeleteDeviceById(ctx, device.Id); err != nil {
			return err
		}
	}
	// 设备删除不会自动清理采集点表，这里按采集绑定一并清掉。
	for _, key := range pointKeys {
		if err := db.ReplaceCollectPointConfigs(key, nil); err != nil {
			return err
		}
	}
	return nil
}

// GatewayById 网关详情
func (app *gatewayApp) GatewayById(ctx context.Context, id string) (dtos.GatewayResponse, error) {
	db := app.dbClient.WithCtx(ctx)
	g, err := db.GatewayById(id)
	if err != nil {
		return dtos.GatewayResponse{}, err
	}
	return toGatewayResponse(g, app.gatewayDeviceCount(db, g, nil), app.deptNameMap(ctx)), nil
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
	collectCounts, _ := collectDeviceCounts(db)
	out := make([]dtos.GatewayResponse, len(list))
	for i, g := range list {
		out[i] = toGatewayResponse(g, app.gatewayDeviceCount(db, g, collectCounts), deptNames)
	}
	return out, total, nil
}

// gatewayDeviceCount 统计网关下的设备数。
//
// 直连/云网关的设备关系在 device.gateway_id；软网关的关系在 collect_device_config。
// 后者由采集绑定写入，Device.GatewayId 为空，不能复用 GatewayDeviceCount。
func (app *gatewayApp) gatewayDeviceCount(db interfaces.DBClient, g models.Gateway, collectCounts map[string]int64) int64 {
	if sourceOf(g) == models.GatewaySourceIoserver {
		if collectCounts == nil {
			collectCounts, _ = collectDeviceCounts(db)
		}
		return collectCounts[g.Id]
	}
	count, _ := db.GatewayDeviceCount(g.Id)
	return count
}

func collectDeviceCounts(db interfaces.DBClient) (map[string]int64, error) {
	configs, err := db.CollectDeviceConfigs()
	if err != nil {
		return nil, err
	}
	counts := make(map[string]int64, len(configs))
	for _, cfg := range configs {
		if cfg.GatewayId != "" {
			counts[cfg.GatewayId]++
		}
	}
	return counts, nil
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
