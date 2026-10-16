// Package solutionapp 行业方案：内置模板查询与「一键应用」。
//
// 一键应用会把模板包落地成平台对象：
//
//	solution（模板包） → 产品（含品类物模型） → 发布产品 → 锚定设备 → 告警规则
//
// 为什么必须建「锚定设备」：平台的告警规则是**设备级**规则，
// alertApp.UpdateAlertRule 会强校验 SubRule[0].DeviceId 对应的设备存在、
// 且该设备的 product_id 与 SubRule[0].ProductId 一致（见 alertcentreapp/alertapp.go）。
// 因此即使 req.DeviceCount == 0，只要需要按模板建告警规则，
// 也会自动创建 1 台锚定设备并在 Skipped 里说明。
package solutionapp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"iotserver/internal/dtos"
	"iotserver/internal/iotserver/core/container"
	interfaces "iotserver/internal/iotserver/core/interface"
	"iotserver/internal/models"
	"iotserver/internal/pkg/constants"
	pkgcontainer "iotserver/internal/pkg/container"
	"iotserver/internal/pkg/di"
	"iotserver/internal/pkg/errort"
	"iotserver/internal/pkg/logger"
	"iotserver/internal/pkg/seed"
	"iotserver/internal/pkg/tenant"
)

// 产品默认取值：与 constants/product.go 中的合法常量对齐
const (
	// defaultProductProtocol 方案统一走 MQTT 接入
	defaultProductProtocol = string(constants.ProductProtocolMQTT)
	// defaultProductNodeType 方案按网关子设备建模（网关下挂机床/产线设备）
	defaultProductNodeType = string(constants.ProductNodeTypeSubDevice)
	// defaultProductNetType 产线设备默认以太网联网
	defaultProductNetType = string(constants.ProductNetTypeEthernet)
	// defaultProductDataFormat 平台默认 JSON 数据格式（Product.DataFormat 无枚举常量，沿用前端约定值）
	defaultProductDataFormat = "JSON"
)

type machineTemplateApp struct {
	dic      *di.Container
	dbClient interfaces.DBClient
	lc       logger.LoggingClient
}

// New 构造方案应用
func New(dic *di.Container) interfaces.SolutionItf {
	return &machineTemplateApp{
		dic:      dic,
		dbClient: container.DBClientFrom(dic.Get),
		lc:       pkgcontainer.LoggingClientFrom(dic.Get),
	}
}

// tenantIDFromCtx 取当前租户 ID（空串=平台内置视图/平台管理员）
func tenantIDFromCtx(ctx context.Context) string {
	return strings.TrimSpace(tenant.FromContext(ctx).TenantID)
}

// machineTemplateResponseFromModel 模型 → 响应 DTO
func machineTemplateResponseFromModel(tpl models.Solution) dtos.SolutionResponse {
	return dtos.SolutionResponse{
		Key:            tpl.Key,
		Name:           tpl.Name,
		Industry:       tpl.Industry,
		Version:        tpl.Version,
		BuiltIn:        tpl.BuiltIn == 1,
		Icon:           tpl.Icon,
		Description:    tpl.Description,
		ThingModelJson: tpl.ThingModelJson,
		AlarmRulesJson: tpl.AlarmRulesJson,
		DashboardJson:  tpl.DashboardJson,
		MetricsJson:    tpl.MetricsJson,
	}
}

// alertLevelFromSeed 模板预设的 1~4 级别 → 平台告警级别文案
func alertLevelFromSeed(level int) constants.AlertLevel {
	switch level {
	case 1:
		return constants.Urgent
	case 2:
		return constants.Important
	case 3:
		return constants.LessImportant
	default:
		return constants.Remind
	}
}

// parseAlarmRules 解析模板里的告警预设
func parseAlarmRules(raw string) ([]seed.SolutionAlarm, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "null" || raw == "[]" {
		return nil, nil
	}
	var alarms []seed.SolutionAlarm
	if err := json.Unmarshal([]byte(raw), &alarms); err != nil {
		return nil, err
	}
	return alarms, nil
}

// propertyDataTypes 解析模板物模型里的 标识符 → 数据类型（INT/FLOAT/BOOL/TEXT/ENUM）
func propertyDataTypes(thingModelJSON string) map[string]string {
	out := map[string]string{}
	var tm struct {
		Properties []struct {
			Identifier string `json:"identifier"`
			DataType   string `json:"dataType"`
		} `json:"properties"`
	}
	if err := json.Unmarshal([]byte(thingModelJSON), &tm); err != nil {
		return out
	}
	for _, p := range tm.Properties {
		out[p.Identifier] = strings.ToUpper(strings.TrimSpace(p.DataType))
	}
	return out
}

// normalizeAlarmValue 归一化模板预设的阈值。
//
// 平台告警校验对布尔属性只接受 true/false（见 alertcentreapp.checkSpecsTypeBoolParam），
// 而模板文件里布尔开关习惯写 0/1，这里统一转换，避免预设规则在应用时被校验拒绝。
func normalizeAlarmValue(dataType, value string) string {
	v := strings.TrimSpace(value)
	if !strings.EqualFold(dataType, string(constants.SpecsTypeBool)) {
		return v
	}
	switch strings.ToLower(v) {
	case "0", "false", "关闭", "否":
		return "false"
	case "1", "true", "开启", "是":
		return "true"
	default:
		return v
	}
}

// SolutionSearch 查询当前租户可见的模板（内置 + 本租户自定义）
func (app *machineTemplateApp) SolutionSearch(ctx context.Context, req dtos.SolutionSearchRequest) ([]dtos.SolutionResponse, error) {
	list, err := app.dbClient.WithCtx(ctx).
		SolutionSearch(tenantIDFromCtx(ctx), strings.TrimSpace(req.Industry))
	if err != nil {
		return nil, err
	}
	out := make([]dtos.SolutionResponse, 0, len(list))
	for _, tpl := range list {
		out = append(out, machineTemplateResponseFromModel(tpl))
	}
	return out, nil
}

// SolutionByKey 模板详情（租户自定义优先于内置）
func (app *machineTemplateApp) SolutionByKey(ctx context.Context, key string) (dtos.SolutionResponse, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return dtos.SolutionResponse{}, errort.NewCommonErr(errort.DefaultReqParamsError,
			fmt.Errorf("模板 key 不能为空"))
	}
	tpl, err := app.dbClient.WithCtx(ctx).SolutionByKey(tenantIDFromCtx(ctx), key)
	if err != nil {
		return dtos.SolutionResponse{}, errort.NewCommonErr(errort.DefaultResourcesNotFound,
			fmt.Errorf("方案 %s 不存在", key))
	}
	return machineTemplateResponseFromModel(tpl), nil
}

// SolutionApply 一键应用：生成产品（含品类物模型）+ 发布 + 锚定设备 + 告警规则
func (app *machineTemplateApp) SolutionApply(ctx context.Context, key string, req dtos.SolutionApplyRequest) (dtos.SolutionApplyResponse, error) {
	res := dtos.SolutionApplyResponse{DeviceIds: []string{}, AlarmRuleIds: []string{}, Skipped: []string{}}
	key = strings.TrimSpace(key)
	if key == "" {
		return res, errort.NewCommonErr(errort.DefaultReqParamsError, fmt.Errorf("模板 key 不能为空"))
	}
	tpl, err := app.dbClient.WithCtx(ctx).SolutionByKey(tenantIDFromCtx(ctx), key)
	if err != nil {
		return res, errort.NewCommonErr(errort.DefaultResourcesNotFound, fmt.Errorf("方案 %s 不存在", key))
	}

	productName := strings.TrimSpace(req.ProductName)
	if productName == "" {
		productName = tpl.Name
	}

	// 1) 建产品：CategoryTemplateId 用 seed 写入的品类记录 ID（即 CategoryTemplateKeyPrefix+key），
	//    AddProduct 会据此从 thing_model_template 拉取物模型。
	//    注意 AddProduct 内部会用随机 secret 覆盖 Product.Key，req.Key 实际不生效。
	productApp := container.ProductAppNameFrom(app.dic.Get)
	productId, err := productApp.AddProduct(ctx, dtos.ProductAddRequest{
		Name:               productName,
		Key:                seed.CategoryTemplateKeyPrefix + key,
		CategoryTemplateId: seed.CategoryTemplateKeyPrefix + key,
		Protocol:           defaultProductProtocol,
		NodeType:           defaultProductNodeType,
		NetType:            defaultProductNetType,
		DataFormat:         defaultProductDataFormat,
		Description:        tpl.Description,
	})
	if err != nil {
		// 名称重复（10009）等由仓储层判定，原样上抛为业务错误，供前端提示改名重试
		return res, err
	}
	res.ProductId = productId
	res.ProductName = productName

	// 2) 发布产品：未发布的产品不允许建设备 / 下发数据
	if err = productApp.ProductRelease(ctx, productId); err != nil {
		return res, errort.NewCommonEdgeX(errort.DefaultSystemError,
			fmt.Sprintf("模板 %s 的产品发布失败", key), err)
	}

	alarms, err := parseAlarmRules(tpl.AlarmRulesJson)
	if err != nil {
		return res, errort.NewCommonEdgeX(errort.DefaultJsonParseError,
			fmt.Sprintf("模板 %s 的告警预设解析失败", key), err)
	}
	createAlarms := req.CreateAlarmRules == nil || *req.CreateAlarmRules

	deviceCount := req.DeviceCount
	if deviceCount < 0 {
		deviceCount = 0
	}
	needAnchorDevice := createAlarms && len(alarms) > 0
	if needAnchorDevice && deviceCount == 0 {
		// 告警规则必须挂在设备上，因此补 1 台锚定设备
		deviceCount = 1
		res.Skipped = append(res.Skipped, "为挂载模板告警规则，已自动创建 1 台锚定设备")
	}

	// 3) 建设备（可选）
	if deviceCount > 0 {
		deviceApp := container.DeviceItfFrom(app.dic.Get)
		for i := 0; i < deviceCount; i++ {
			deviceId, dErr := deviceApp.AddDevice(ctx, dtos.DeviceAddRequest{
				Name:        fmt.Sprintf("%s-%d", productName, i+1),
				ProductId:   productId,
				Description: tpl.Description,
			})
			if dErr != nil {
				return res, errort.NewCommonEdgeX(errort.DefaultSystemError,
					fmt.Sprintf("模板 %s 的设备创建失败（产品已创建，可在设备管理中重试）", key), dErr)
			}
			res.DeviceIds = append(res.DeviceIds, deviceId)
		}
		// 组织归属：DeviceAddRequest 没有 dept_id 字段，创建后单独回写
		if deptId := strings.TrimSpace(req.DeptId); deptId != "" {
			if deptErr := app.assignDevicesDept(ctx, res.DeviceIds, deptId); deptErr != nil {
				res.Skipped = append(res.Skipped, "设备组织归属设置失败，请在设备管理中调整")
			}
		}
	}

	// 4) 建告警规则（每条规则必须「先建主体、再补触发条件」两步）
	if needAnchorDevice && len(res.DeviceIds) == 0 {
		res.Skipped = append(res.Skipped, "未创建锚定设备，已跳过告警规则创建")
	} else if needAnchorDevice {
		alertApp := container.AlertRuleAppNameFrom(app.dic.Get)
		anchorDeviceId := res.DeviceIds[0]
		dataTypes := propertyDataTypes(tpl.ThingModelJson)
		for _, rule := range alarms {
			ruleId, aErr := alertApp.AddAlertRule(ctx, dtos.RuleAddRequest{
				Name:        rule.Name,
				AlertType:   constants.DeviceAlertType,
				AlertLevel:  alertLevelFromSeed(rule.AlertLevel),
				Description: rule.Description,
			})
			if aErr != nil {
				return res, errort.NewCommonEdgeX(errort.AlertRuleParamsError,
					fmt.Sprintf("模板 %s 的告警规则 %s 创建失败", key, rule.Name), aErr)
			}
			uErr := alertApp.UpdateAlertRule(ctx, dtos.RuleUpdateRequest{
				Id:          ruleId,
				Condition:   constants.WorkerConditionAnyone,
				SilenceTime: rule.SilenceTime,
				SubRule: []dtos.SubRule{{
					Trigger:   constants.DeviceDataTrigger,
					ProductId: productId,
					DeviceId:  anchorDeviceId,
					Option: map[string]string{
						"code":             rule.PropertyCode,
						"decide_condition": rule.Operator + " " + normalizeAlarmValue(dataTypes[rule.PropertyCode], rule.Value),
						"value_type":       constants.Original,
					},
				}},
			})
			if uErr != nil {
				// 回滚半成品规则（只有主体、没有触发条件），避免告警中心出现空规则
				_ = app.dbClient.WithCtx(ctx).DeleteAlertRuleById(ruleId)
				return res, errort.NewCommonEdgeX(errort.AlertRuleParamsError,
					fmt.Sprintf("模板 %s 的告警规则 %s 触发条件配置失败", key, rule.Name), uErr)
			}
			res.AlarmRuleIds = append(res.AlarmRuleIds, ruleId)
		}
		if len(res.DeviceIds) > 1 {
			res.Skipped = append(res.Skipped,
				fmt.Sprintf("告警规则仅挂载第 1 台设备(%s)，其余设备如需同类告警请在告警中心复制规则", anchorDeviceId))
		}
	}

	return res, nil
}

// assignDevicesDept 回写设备的组织归属（DeptId）
func (app *machineTemplateApp) assignDevicesDept(ctx context.Context, deviceIds []string, deptId string) error {
	for _, deviceId := range deviceIds {
		device, err := app.dbClient.WithCtx(ctx).DeviceById(deviceId)
		if err != nil {
			return err
		}
		device.DeptId = deptId
		if err = app.dbClient.WithCtx(ctx).UpdateDevice(device); err != nil {
			return err
		}
	}
	return nil
}
