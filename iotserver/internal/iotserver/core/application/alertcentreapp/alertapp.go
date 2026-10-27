/*******************************************************************************
 * Copyright 2017.
 *
 * Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software distributed under the License
 * is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express
 * or implied. See the License for the specific language governing permissions and limitations under
 * the License.
 *******************************************************************************/

package alertcentreapp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"iotserver/internal/dtos"
	resourceContainer "iotserver/internal/iotserver/core/container"
	interfaces "iotserver/internal/iotserver/core/interface"
	"iotserver/internal/models"
	"iotserver/internal/pkg/constants"
	"iotserver/internal/pkg/container"
	"iotserver/internal/pkg/di"
	"iotserver/internal/pkg/errort"
	"iotserver/internal/pkg/i18n"
	"iotserver/internal/pkg/logger"
	"iotserver/internal/pkg/tenant"
	"iotserver/internal/pkg/utils"
	"iotserver/internal/tools/notify/email"
	"iotserver/internal/tools/notify/twilio"

	"gorm.io/gorm"
	"iotserver/internal/tools/notify/webapi"
	"strconv"
	"strings"
	"time"
)

type alertApp struct {
	dic      *di.Container
	dbClient interfaces.DBClient
	lc       logger.LoggingClient
}

func NewAlertCentreApp(ctx context.Context, dic *di.Container) interfaces.AlertRuleApp {
	lc := container.LoggingClientFrom(dic.Get)
	dbClient := resourceContainer.DBClientFrom(dic.Get)

	app := &alertApp{
		dic:      dic,
		dbClient: dbClient,
		lc:       lc,
	}
	go app.monitor()
	return app
}

func (p alertApp) AddAlertRule(ctx context.Context, req dtos.RuleAddRequest) (string, error) {
	var insertAlertRule models.AlertRule
	insertAlertRule.Id = utils.RandomNum()
	insertAlertRule.Name = req.Name
	insertAlertRule.AlertType = constants.DeviceAlertType
	//insertAlertRule.Status = constants.RuleStop
	insertAlertRule.AlertLevel = req.AlertLevel
	insertAlertRule.Description = req.Description
	resp, err := p.dbClient.WithCtx(ctx).AddAlertRule(insertAlertRule)
	if err != nil {
		return "", err
	}
	return resp.Id, nil
}

func (p alertApp) UpdateAlertField(ctx context.Context, req dtos.RuleFieldUpdate) error {
	if req.Id == "" {
		return errort.NewCommonEdgeX(errort.DefaultReqParamsError, "update req id is required", nil)
	}
	alertRule, err := p.dbClient.WithCtx(ctx).AlertRuleById(req.Id)
	if err != nil {
		return err
	}
	dtos.ReplaceRuleFields(&alertRule, req)
	err = p.dbClient.WithCtx(ctx).GetDBInstance().Table(alertRule.TableName()).Select("*").Updates(alertRule).Error
	if err != nil {
		return err
	}
	return nil
}

func (p alertApp) UpdateAlertRule(ctx context.Context, req dtos.RuleUpdateRequest) error {
	if req.Id == "" {
		return errort.NewCommonEdgeX(errort.DefaultReqParamsError, "update req id is required", nil)
	}
	if len(req.SubRule) != 1 {
		return errors.New("")
	}
	device, err := p.dbClient.WithCtx(ctx).DeviceById(req.SubRule[0].DeviceId)
	if err != nil {
		return err
	}

	product, err := p.dbClient.WithCtx(ctx).ProductById(device.ProductId)
	if err != nil {
		return err
	}

	alertRule, err := p.dbClient.WithCtx(ctx).AlertRuleById(req.Id)
	if err != nil {
		return err
	}

	if req.SubRule[0].ProductId != device.ProductId {
		return errort.NewCommonEdgeX(errort.AlertRuleParamsError, "device product id not equal to req product id", nil)
	}
	if len(req.Notify) > 0 {
		if err = checkNotifyParam(req.Notify); err != nil {
			return err
		}
	}

	var sql string

	switch req.SubRule[0].Trigger {
	case constants.DeviceDataTrigger:
		var code string
		if v, ok := req.SubRule[0].Option["code"]; ok {
			code = v
		} else {
			return errort.NewCommonEdgeX(errort.AlertRuleParamsError, "update rule code is required", nil)
		}

		var find bool
		var productProperty models.Properties
		for _, property := range product.Properties {
			if property.Code == code {
				find = true
				productProperty = property
				break
			}
		}
		if !find {
			return errort.NewCommonEdgeX(errort.ProductPropertyCodeNotExist, "product property code exist", nil)
		}

		switch productProperty.TypeSpec.Type {
		case constants.SpecsTypeInt, constants.SpecsTypeFloat:
			if err = checkSpecsTypeIntOrFloatParam(req.SubRule[0]); err != nil {
				return err
			}
		case constants.SpecsTypeText:
			if err = checkSpecsTypeTextParam(req.SubRule[0]); err != nil {
				return err
			}
		case constants.SpecsTypeBool:
			if err = checkSpecsTypeBoolParam(req.SubRule[0]); err != nil {
				return err
			}
		case constants.SpecsTypeEnum:
			if err = checkSpecsTypeEnumParam(req.SubRule[0]); err != nil {
				return err
			}
		default:
			return errort.NewCommonEdgeX(errort.DefaultReqParamsError, "update rule code verify failed", nil)
		}

		sql = req.BuildEkuiperSql(device.Id, productProperty.TypeSpec.Type)

	case constants.DeviceEventTrigger:
		var code string
		if v, ok := req.SubRule[0].Option["code"]; ok {
			code = v
		} else {
			return errort.NewCommonEdgeX(errort.AlertRuleParamsError, "update rule code is required", nil)
		}
		var find bool
		//var productProperty models.Properties
		for _, event := range product.Events {
			if event.Code == code {
				find = true
				//productProperty = property
				break
			}
		}
		if !find {
			return errort.NewCommonEdgeX(errort.ProductPropertyCodeNotExist, "product event code exist", nil)
		}
		sqlTemp := `SELECT rule_id(),json_path_query(data, "$.eventTime") as report_time,deviceId FROM mqtt_stream where deviceId = "%s" and messageType = "EVENT_REPORT" and  json_path_exists(data, "$.eventCode") = true and json_path_query(data, "$.eventCode") = "%s"`
		sql = fmt.Sprintf(sqlTemp, device.Id, code)
	case constants.DeviceStatusTrigger:
		//{"code":"","device_id":"2499708","end_at":null,"start_at":null}

		var status string
		deviceStatus := req.SubRule[0].Option["status"]
		if deviceStatus == "" {
			err = errort.NewCommonEdgeX(errort.DefaultReqParamsError, "required status parameter missing", nil)
			return err
		}
		if deviceStatus == "在线" {
			status = constants.DeviceOnline
		} else if deviceStatus == "离线" {
			status = constants.DeviceOffline
		} else {
			err = errort.NewCommonEdgeX(errort.DefaultReqParamsError, "required status parameter missing", nil)
			return err
		}
		sqlTemp := `SELECT rule_id(),json_path_query(data, "$.time") as report_time,deviceId FROM mqtt_stream where deviceId = "%s" and messageType = "DEVICE_STATUS" and  json_path_exists(data, "$.status") = true and json_path_query(data, "$.status") = "%s"`
		sql = fmt.Sprintf(sqlTemp, device.Id, status)
	default:
		return errort.NewCommonEdgeX(errort.DefaultReqParamsError, "update rule trigger is required", nil)
	}

	if sql == "" {
		return errort.NewCommonEdgeX(errort.AlertRuleParamsError, "sql is null", nil)

	}

	ekuiperApp := resourceContainer.EkuiperAppFrom(p.dic.Get)
	exist, err := ekuiperApp.RuleExist(ctx, alertRule.Id)
	if err != nil {
		return err
	}
	configapp := resourceContainer.ConfigurationFrom(p.dic.Get)
	if !exist {
		if err = ekuiperApp.CreateRule(ctx, dtos.GetRuleAlertEkuiperActions(configapp.Service.CallbackBaseUrl()), alertRule.Id, sql); err != nil {
			return err
		}
	} else {
		if err = ekuiperApp.UpdateRule(ctx, dtos.GetRuleAlertEkuiperActions(configapp.Service.CallbackBaseUrl()), alertRule.Id, sql); err != nil {
			return err
		}
	}

	dtos.ReplaceRuleModelFields(&alertRule, req)
	//alertRule.Status = constants.RuleStop
	alertRule.DeviceId = device.Id
	err = p.dbClient.WithCtx(ctx).GetDBInstance().Table(alertRule.TableName()).Select("*").Updates(alertRule).Error
	if err != nil {
		return err
	}

	return nil
}

func checkNotifyParam(notify []dtos.Notify) error {
	for _, d := range notify {
		if !utils.InStringSlice(string(d.Name), constants.GetAlertWays()) {
			return errort.NewCommonEdgeX(errort.DefaultReqParamsError, "notify name not in alertways", nil)
		}
		start := strings.TrimSpace(d.StartEffectTime)
		end := strings.TrimSpace(d.EndEffectTime)
		if (start == "") != (end == "") {
			return errort.NewCommonEdgeX(errort.DefaultReqParamsError, "startEffectTime and endEffectTime must both be set or both be empty", nil)
		}
		if start != "" && !checkEffectTimeParam(start, end) {
			return errort.NewCommonEdgeX(errort.EffectTimeParamsError, "The format of the effective time is"+
				" incorrect. The end time should be greater than the start time.", nil)
		}

	}
	return nil
}

func checkSpecsTypeBoolParam(req dtos.SubRule) error {
	var decideCondition string
	if v, ok := req.Option["decide_condition"]; ok {
		decideCondition = v
	} else {
		return errort.NewCommonEdgeX(errort.DefaultReqParamsError, "update rule decide_condition is required", nil)
	}

	st := strings.Split(decideCondition, " ")
	if len(st) != 2 {
		return errort.NewCommonEdgeX(errort.DefaultReqParamsError, "update rule decide_condition verify failed", nil)
	}
	//if st[0] != "==" {
	//	return errort.NewCommonEdgeX(errort.DefaultReqParamsError, "update rule decide_condition verify failed", nil)
	//}
	if !(st[1] == "true" || st[1] == "false") {
		return errort.NewCommonEdgeX(errort.DefaultReqParamsError, "update rule decide_condition verify failed", nil)
	}
	return nil
}

func checkSpecsTypeEnumParam(req dtos.SubRule) error {
	var decideCondition string
	if v, ok := req.Option["decide_condition"]; ok {
		decideCondition = v
	} else {
		return errort.NewCommonEdgeX(errort.DefaultReqParamsError, "update rule decide_condition is required", nil)
	}

	st := strings.Split(decideCondition, " ")
	if len(st) != 2 {
		return errort.NewCommonEdgeX(errort.DefaultReqParamsError, "update rule decide_condition verify failed", nil)
	}
	//if st[0] != "==" {
	//	return errort.NewCommonEdgeX(errort.DefaultReqParamsError, "update rule decide_condition verify failed", nil)
	//}
	if st[0] == "" {
		return errort.NewCommonEdgeX(errort.DefaultReqParamsError, "update rule decide_condition verify failed", nil)
	}
	return nil
}

func checkSpecsTypeTextParam(req dtos.SubRule) error {
	var decideCondition string
	if v, ok := req.Option["decide_condition"]; ok {
		decideCondition = v
	} else {
		return errort.NewCommonEdgeX(errort.DefaultReqParamsError, "update rule decide_condition is required", nil)
	}

	st := strings.Split(decideCondition, " ")
	if len(st) != 2 {
		return errort.NewCommonEdgeX(errort.DefaultReqParamsError, "update rule decide_condition verify failed", nil)
	}
	//if st[0] != "==" {
	//	return errort.NewCommonEdgeX(errort.DefaultReqParamsError, "update rule decide_condition verify failed", nil)
	//}
	if st[1] == "" {
		return errort.NewCommonEdgeX(errort.DefaultReqParamsError, "update rule decide_condition verify failed", nil)
	}
	return nil
}

func checkSpecsTypeIntOrFloatParam(req dtos.SubRule) error {
	var valueType, decideCondition string

	if v, ok := req.Option["value_type"]; ok {
		valueType = v
	} else {
		return errort.NewCommonEdgeX(errort.DefaultReqParamsError, "update rule value_type is required", nil)
	}
	find := false
	for _, s := range constants.ValueTypes {
		if s == valueType {
			find = true
			break
		}
	}
	if !find {
		return errort.NewCommonEdgeX(errort.DefaultReqParamsError, "update rule value_type verify failed", nil)
	}

	if v, ok := req.Option["decide_condition"]; ok {
		decideCondition = v
	} else {
		return errort.NewCommonEdgeX(errort.DefaultReqParamsError, "update rule decide_condition is required", nil)
	}

	st := strings.Split(decideCondition, " ")
	if len(st) != 2 {
		return errort.NewCommonEdgeX(errort.DefaultReqParamsError, "update rule decide_condition verify failed", nil)
	}
	find = false
	for _, condition := range constants.DecideConditions {
		if condition == st[0] {
			find = true
			break
		}
	}
	if !find {
		return errort.NewCommonEdgeX(errort.DefaultReqParamsError, "update rule decide_condition verify failed", nil)
	}
	return nil
}

func (p alertApp) AlertRuleById(ctx context.Context, id string) (dtos.RuleResponse, error) {
	alertRule, err := p.dbClient.WithCtx(ctx).AlertRuleById(id)
	var response dtos.RuleResponse
	if err != nil {
		return response, err
	}
	var ruleResponse dtos.RuleResponse
	ruleResponse.Id = alertRule.Id
	ruleResponse.Name = alertRule.Name
	ruleResponse.AlertType = alertRule.AlertType
	ruleResponse.AlertLevel = alertRule.AlertLevel
	ruleResponse.Status = alertRule.Status
	ruleResponse.Condition = alertRule.Condition
	ruleResponse.SilenceTime = alertRule.SilenceTime
	ruleResponse.Description = alertRule.Description
	ruleResponse.Created = alertRule.Created
	ruleResponse.Modified = alertRule.Modified
	ruleResponse.Notify = alertRule.Notify
	if len(ruleResponse.Notify) == 0 {
		ruleResponse.Notify = make([]models.SubNotify, 0)
	}

	var ruleSubRules dtos.RuleSubRules
	for _, rule := range alertRule.SubRule {
		device, err := p.dbClient.WithCtx(ctx).DeviceById(alertRule.DeviceId)
		if err != nil {
			return response, err
		}
		product, err := p.dbClient.WithCtx(ctx).ProductById(device.ProductId)
		if err != nil {
			return response, err
		}
		code := rule.Option["code"]
		var (
			eventCodeName    string
			propertyCodeName string
		)

		for _, event := range product.Events {
			if event.Code == code {
				eventCodeName = event.Name
			}
		}
		for _, property := range product.Properties {
			if property.Code == code {
				propertyCodeName = property.Name
			}
		}
		var valueType string
		switch rule.Option["value_type"] {
		case "original":
			valueType = "原始值"
		case "avg":
			valueType = "平均值"
		case "max":
			valueType = "最大值"
		case "min":
			valueType = "最小值"
		case "sum":
			valueType = "求和值"
		}
		// 条件描述由后端拼装：标签部分按请求语言翻译，产品/设备/功能名仍是用户数据。
		lang := i18n.GetLang(ctx)
		tr := func(key string) string { return i18n.Trans(lang, key, nil) }
		var condition string
		switch rule.Trigger {
		case constants.DeviceDataTrigger:
			if rule.Option["value_cycle"] == "" {
				if valueType == "" {
					valueType = "原始值"
				}
				condition = tr(string(constants.DeviceDataTrigger)) + ": " + tr("产品") + ": " + product.Name + " | " +
					tr("设备") + ": " + device.Name + " | " +
					tr("功能") + ": " + propertyCodeName + " | " +
					tr("触发条件") + ": " + tr(valueType) + " " + rule.Option["decide_condition"]
			} else {
				condition = tr(string(constants.DeviceDataTrigger)) + ": " + tr("产品") + ": " + product.Name + " | " +
					tr("设备") + ": " + device.Name + " | " +
					tr("功能") + ": " + propertyCodeName + " | " +
					tr("触发条件") + ": " + tr(valueType) + " " + fmt.Sprintf("(%s)", rule.Option["value_cycle"]) + " " + rule.Option["decide_condition"]
			}
		case constants.DeviceEventTrigger:
			condition = tr(string(constants.DeviceEventTrigger)) + ": " + tr("产品") + ": " + product.Name + " | " +
				tr("设备") + ": " + device.Name + " | " +
				tr("事件") + " = " + eventCodeName
		case constants.DeviceStatusTrigger:
			condition = tr(string(constants.DeviceStatusTrigger)) + ": " + tr("产品") + ": " + product.Name + " | " +
				tr("设备") + ": " + device.Name + " | " +
				tr("设备状态") + " = " + tr(rule.Option["status"])
		default:
			condition = ""
		}
		ruleSubRules = append(ruleSubRules, dtos.RuleSubRule{
			ProductId:   rule.ProductId,
			ProductName: product.Name,
			DeviceId:    rule.DeviceId,
			DeviceName:  device.Name,
			Trigger:     rule.Trigger,
			Code:        code,
			Condition:   condition,
			Option:      rule.Option,
		})
	}

	ruleResponse.SubRule = ruleSubRules
	return ruleResponse, nil
}

func (p alertApp) AlertRulesSearch(ctx context.Context, req dtos.AlertRuleSearchQueryRequest) ([]dtos.AlertRuleSearchQueryResponse, uint32, error) {
	offset, limit := req.BaseSearchConditionQuery.GetPage()
	resp, total, err := p.dbClient.WithCtx(ctx).AlertRuleSearch(offset, limit, req)
	if err != nil {
		return []dtos.AlertRuleSearchQueryResponse{}, 0, err
	}
	alertRules := make([]dtos.AlertRuleSearchQueryResponse, len(resp))
	for i, p := range resp {
		alertRules[i] = dtos.RuleSearchQueryResponseFromModel(p)
	}
	return alertRules, total, nil
}

func (p alertApp) AlertSearch(ctx context.Context, req dtos.AlertSearchQueryRequest) ([]dtos.AlertSearchQueryResponse, uint32, error) {
	offset, limit := req.BaseSearchConditionQuery.GetPage()
	resp, total, err := p.dbClient.WithCtx(ctx).AlertListSearch(offset, limit, req)
	if err != nil {
		return []dtos.AlertSearchQueryResponse{}, 0, err
	}
	return resp, total, nil
}

func (p alertApp) AlertPlate(ctx context.Context, beforeTime int64) ([]dtos.AlertPlateQueryResponse, error) {
	data, err := p.dbClient.WithCtx(ctx).AlertPlate(beforeTime)
	if err != nil {
		return []dtos.AlertPlateQueryResponse{}, err
	}
	var dealData []dtos.AlertPlateQueryResponse
	dealData = append(append(append(append(dealData, dtos.AlertPlateQueryResponse{
		AlertLevel: constants.Urgent,
		Count:      p.getAlertDataCount(data, constants.Urgent),
	}), dtos.AlertPlateQueryResponse{
		AlertLevel: constants.Important,
		Count:      p.getAlertDataCount(data, constants.Important),
	}), dtos.AlertPlateQueryResponse{
		AlertLevel: constants.LessImportant,
		Count:      p.getAlertDataCount(data, constants.LessImportant),
	}), dtos.AlertPlateQueryResponse{
		AlertLevel: constants.Remind,
		Count:      p.getAlertDataCount(data, constants.Remind),
	})
	return dealData, nil
}

func (p alertApp) getAlertDataCount(data []dtos.AlertPlateQueryResponse, level constants.AlertLevel) int {
	for _, datum := range data {
		if datum.AlertLevel == level {
			return datum.Count
		}
	}
	return 0
}

func (p alertApp) AlertRulesDelete(ctx context.Context, id string) error {
	_, err := p.dbClient.WithCtx(ctx).AlertRuleById(id)
	if err != nil {
		return err
	}

	ekuiperApp := resourceContainer.EkuiperAppFrom(p.dic.Get)
	err = ekuiperApp.DeleteRule(ctx, id)
	if err != nil {
		return err
	}
	return p.dbClient.WithCtx(ctx).DeleteAlertRuleById(id)
}

func (p alertApp) AlertRulesRestart(ctx context.Context, id string) error {
	alertRule, err := p.dbClient.WithCtx(ctx).AlertRuleById(id)
	if err != nil {
		return err
	}
	if err = p.checkAlertRuleParam(ctx, alertRule, "restart"); err != nil {
		return err
	}
	if alertRule.EkuiperRule() {
		ekuiperApp := resourceContainer.EkuiperAppFrom(p.dic.Get)
		err = ekuiperApp.RestartRule(ctx, id)
		if err != nil {
			return err
		}
	}
	return p.dbClient.WithCtx(ctx).AlertRuleStart(id)
}

func (p alertApp) AlertRulesStop(ctx context.Context, id string) error {
	_, err := p.dbClient.WithCtx(ctx).AlertRuleById(id)
	if err != nil {
		return err
	}
	ekuiperApp := resourceContainer.EkuiperAppFrom(p.dic.Get)
	err = ekuiperApp.StopRule(ctx, id)
	if err != nil {
		return err
	}
	return p.dbClient.WithCtx(ctx).AlertRuleStop(id)
}

func (p alertApp) AlertRulesStart(ctx context.Context, id string) error {
	alertRule, err := p.dbClient.WithCtx(ctx).AlertRuleById(id)
	if err != nil {
		return err
	}
	if err = p.checkAlertRuleParam(ctx, alertRule, "start"); err != nil {
		return err
	}
	ekuiperApp := resourceContainer.EkuiperAppFrom(p.dic.Get)
	err = ekuiperApp.StartRule(ctx, id)
	if err != nil {
		return err
	}
	return p.dbClient.WithCtx(ctx).AlertRuleStart(id)
}

func (p alertApp) AlertIgnore(ctx context.Context, id string) error {
	return p.dbClient.WithCtx(ctx).AlertIgnore(id)
}

func (p alertApp) TreatedIgnore(ctx context.Context, id, message string) error {
	return p.dbClient.WithCtx(ctx).TreatedIgnore(id, message)
}

func (p alertApp) AlertRuleStatus(ctx context.Context, id string) (constants.RuleStatus, error) {
	alertRule, err := p.dbClient.WithCtx(ctx).AlertRuleById(id)
	if err != nil {
		return "", err
	}
	return alertRule.Status, nil
}

func (p alertApp) checkAlertRuleParam(ctx context.Context, rule models.AlertRule, operate string) error {
	if operate == "start" {
		if rule.Status == constants.RuleStart {
			return errort.NewCommonErr(errort.AlertRuleStatusStarting, fmt.Errorf("alertRule id(%s) is runing ,not allow start", rule.Id))
		}
	}

	if rule.AlertType == "" || rule.AlertLevel == "" {
		return errort.NewCommonErr(errort.AlertRuleParamsError, fmt.Errorf("alertRule id(%s) alertType or alertLevel is null", rule.Id))
	}

	if len(rule.SubRule) == 0 {
		return errort.NewCommonErr(errort.AlertRuleParamsError, fmt.Errorf("alertRule id(%s) subrule is null", rule.Id))
	}

	for _, subRule := range rule.SubRule {
		if subRule.Trigger == "" {
			return errort.NewCommonErr(errort.AlertRuleParamsError, fmt.Errorf("alertRule id(%s) subrule trigger is null", rule.Id))
		}
		if subRule.ProductId == "" || subRule.DeviceId == "" {
			return errort.NewCommonErr(errort.AlertRuleParamsError, fmt.Errorf("alertRule id(%s) device id or product id is null", rule.Id))
		}
		product, err := p.dbClient.WithCtx(ctx).ProductById(subRule.ProductId)
		if err != nil {
			return errort.NewCommonErr(errort.AlertRuleProductOrDeviceUpdate, fmt.Errorf("alertRule id(%s) device id or product id is null", rule.Id))
		}
		device, err := p.dbClient.WithCtx(ctx).DeviceById(subRule.DeviceId)
		if err != nil {
			return errort.NewCommonErr(errort.AlertRuleProductOrDeviceUpdate, fmt.Errorf("alertRule id(%s) product or device has been modified. Please edit the rule again", rule.Id))
		}

		if device.ProductId != product.Id {
			return errort.NewCommonErr(errort.AlertRuleProductOrDeviceUpdate, fmt.Errorf("alertRule id(%s) product or device has been modified. Please edit the rule again", rule.Id))
		}
		code := subRule.Option["code"]
		switch subRule.Trigger {
		case constants.DeviceDataTrigger:
			if code == "" {
				return errort.NewCommonErr(errort.AlertRuleParamsError, fmt.Errorf("alertRule id(%s) code is null", rule.Id))
			}
			var find bool
			var typeSpecType constants.SpecsType
			for _, property := range product.Properties {
				if property.Code == code {
					find = true
					typeSpecType = property.TypeSpec.Type
					break
				}
			}
			if !find {
				return errort.NewCommonErr(errort.AlertRuleProductOrDeviceUpdate, fmt.Errorf("alertRule id(%s) product or device has been modified. Please edit the rule again", rule.Id))
			}
			if !typeSpecType.AllowSendInEkuiper() {
				return errort.NewCommonErr(errort.AlertRuleParamsError, fmt.Errorf("alertRule id(%s) %s allowSendInEkuiper", rule.Id, typeSpecType))
			}
			valueType := subRule.Option["value_type"]
			if valueType == "" {
				return errort.NewCommonErr(errort.AlertRuleParamsError, fmt.Errorf("alertRule id(%s) valueType is null", rule.Id))
			}
			var valueTypeFind bool
			for _, s := range constants.ValueTypes {
				if s == valueType {
					valueTypeFind = true
					break
				}
			}
			if !valueTypeFind {
				return errort.NewCommonErr(errort.AlertRuleParamsError, fmt.Errorf("alertRule id(%s) valueTypeFind error", rule.Id))
			}

			valueCycle := subRule.Option["value_cycle"]
			if valueType != constants.Original {
				if valueCycle == "" {
					return errort.NewCommonErr(errort.AlertRuleParamsError, fmt.Errorf("alertRule id(%s) valueCycle is null", rule.Id))
				}
			}

			decideCondition := subRule.Option["decide_condition"]
			if decideCondition == "" {
				return errort.NewCommonErr(errort.AlertRuleParamsError, fmt.Errorf("alertRule id(%s) decideCondition is null", rule.Id))
			}

		case constants.DeviceEventTrigger:
			if code == "" {
				return errort.NewCommonErr(errort.AlertRuleParamsError, fmt.Errorf("alertRule id(%s) code is null", rule.Id))
			}
			var find bool
			for _, event := range product.Events {
				if event.Code == code {
					find = true
					break
				}
			}
			if !find {
				return errort.NewCommonErr(errort.AlertRuleProductOrDeviceUpdate, fmt.Errorf("alertRule id(%s) product or device has been modified. Please edit the rule again", rule.Id))
			}
		case constants.DeviceStatusTrigger:

		}

	}
	return nil
}

func (p alertApp) AddAlert(ctx context.Context, req map[string]interface{}) error {

	deviceId, ok := req["deviceId"]
	if !ok {
		return errort.NewCommonErr(errort.DefaultReqParamsError, errors.New(""))
	}

	ruleId, ok := req["rule_id"]
	if !ok {
		return errort.NewCommonErr(errort.DefaultReqParamsError, errors.New(""))
	}
	var (
		coverDeviceId string
		coverRuleId   string
	)
	switch deviceId.(type) {
	case string:
		coverDeviceId = deviceId.(string)
	case int:
		coverDeviceId = strconv.Itoa(deviceId.(int))
	case int64:
		coverDeviceId = strconv.Itoa(int(deviceId.(int64)))
	case float64:
		coverDeviceId = fmt.Sprintf("%f", deviceId.(float64))
	case float32:
		coverDeviceId = fmt.Sprintf("%f", deviceId.(float64))
	}
	if coverDeviceId == "" {
		return errort.NewCommonErr(errort.DefaultReqParamsError, errors.New(""))
	}

	switch ruleId.(type) {
	case string:
		coverRuleId = ruleId.(string)
	case int:
		coverRuleId = strconv.Itoa(ruleId.(int))
	case int64:
		coverRuleId = strconv.Itoa(int(ruleId.(int64)))
	case float64:
		coverRuleId = fmt.Sprintf("%f", ruleId.(float64))
	case float32:
		coverRuleId = fmt.Sprintf("%f", ruleId.(float64))
	}

	if coverRuleId == "" {
		return errort.NewCommonErr(errort.DefaultReqParamsError, errors.New(""))
	}

	device, err := p.dbClient.WithCtx(ctx).DeviceById(coverDeviceId)
	if err != nil {
		return err
	}
	product, err := p.dbClient.WithCtx(ctx).ProductById(device.ProductId)
	if err != nil {
		return err
	}
	alertRule, err := p.dbClient.WithCtx(ctx).AlertRuleById(coverRuleId)
	if err != nil {
		return err
	}
	// 告警由 eKuiper 回调触发，请求本身没有租户上下文：从告警规则反查租户，
	// 后续写入都落到该租户下，否则租户在「告警中心」看不到自己的告警。
	if alertRule.TenantId != "" {
		ctx = tenant.With(ctx, tenant.Context{TenantID: alertRule.TenantId})
	}

	alertResult := make(map[string]interface{})
	alertResult["device_id"] = device.Id
	alertResult["code"] = alertRule.SubRule[0].Option["code"]
	if req["window_start"] != nil && req["window_end"] != nil {
		p.lc.Info("msg report1:", req["window_start"])
		alertResult["start_at"] = req["window_start"]
		alertResult["end_at"] = req["window_end"]
	} else if req["report_time"] != nil {
		reportTime := utils.InterfaceToString(req["report_time"])
		if len(reportTime) > 3 {
			sa, err := strconv.Atoi(reportTime[0:len(reportTime)-3] + "000")
			if err == nil {
				alertResult["start_at"] = sa
			}
			ea, err := strconv.Atoi(reportTime[0:len(reportTime)-3] + "999")
			if err == nil {
				alertResult["end_at"] = ea
			}
		}
	}

	if len(alertRule.SubRule) > 0 {
		switch alertRule.SubRule[0].Trigger {
		case constants.DeviceEventTrigger:
			alertResult["trigger"] = string(constants.DeviceEventTrigger)
		case constants.DeviceDataTrigger:
			alertResult["trigger"] = string(constants.DeviceDataTrigger)
		}
	}

	if alertRule.SilenceTime > 0 {
		alertSend, err := p.dbClient.WithCtx(ctx).AlertListLastSend(alertRule.Id)
		if err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				// 处理不是记录未找到的情况
				return err
			}
		} else {
			if alertSend.Created+alertRule.SilenceTime > utils.MakeTimestamp() {
				// 在静默期内，不发送
				return nil
			}
		}
	}

	var alertList models.AlertList
	alertList.AlertRuleId = alertRule.Id
	alertList.AlertResult = alertResult
	alertList.TriggerTime = time.Now().UnixMilli()
	alertList.IsSend = true
	alertList.Status = constants.Untreated

	_, err = p.dbClient.WithCtx(ctx).AddAlertList(alertList)
	if err != nil {
		return err
	}

	var (
		emailCfg   email.Config
		twilioCfg  twilio.Config
		needSMTP   bool
		needTwilio bool
	)
	for _, notify := range alertRule.Notify {
		switch notify.Name {
		case constants.Email:
			if emailProvider(notify) == "twilio" {
				needTwilio = true
			} else {
				needSMTP = true
			}
		case constants.TwilioEmail, constants.SMS, constants.WhatsApp:
			needTwilio = true
		}
	}
	if needSMTP {
		emailCfg = p.loadEmailConfig(ctx)
	}
	if needTwilio {
		twilioCfg = p.loadTwilioConfig(ctx)
	}
	for _, notify := range alertRule.Notify {
		if !checkEffectTime(notify.StartEffectTime, notify.EndEffectTime) {
			continue
		}
		lang := notifyLang(notify)
		content := buildAlertContent(alertRule, device, product, lang)
		switch notify.Name {
		case constants.Email, constants.TwilioEmail:
			recipients := firstNonEmpty(notify.Option["email"], notify.Option["recipient"])
			if strings.TrimSpace(recipients) == "" {
				p.lc.Warnf("alert %s: email recipient is empty", alertRule.Id)
				continue
			}
			useTwilio := notify.Name == constants.TwilioEmail || emailProvider(notify) == "twilio"
			if useTwilio {
				go func(cfg twilio.Config, to, subject, body string) {
					ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
					defer cancel()
					if err := twilio.New(cfg).SendEmail(ctx, to, subject, body); err != nil {
						p.lc.Errorf("twilio email alert send error: %v", err)
					}
				}(twilioCfg, recipients, content.Subject, content.Body)
			} else {
				go func(cfg email.Config, to, subject, body string) {
					if err := email.Send(cfg, to, subject, body); err != nil {
						p.lc.Errorf("email alert send error: %v", err)
					}
				}(emailCfg, recipients, content.Subject, content.Body)
			}
		case constants.SMS:
			phone := firstNonEmpty(notify.Option["phoneNumber"], notify.Option["phone"])
			if strings.TrimSpace(phone) == "" {
				p.lc.Warnf("alert %s: sms phone number is empty", alertRule.Id)
				continue
			}
			go func(cfg twilio.Config, to, body string) {
				ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
				defer cancel()
				if err := twilio.New(cfg).SendSMS(ctx, to, body); err != nil {
					p.lc.Errorf("sms alert send error: %v", err)
				}
			}(twilioCfg, phone, content.Body)
		case constants.WhatsApp:
			phone := firstNonEmpty(notify.Option["phoneNumber"], notify.Option["phone"])
			if strings.TrimSpace(phone) == "" {
				p.lc.Warnf("alert %s: whatsapp phone number is empty", alertRule.Id)
				continue
			}
			variables := map[string]string{
				"1": content.DeviceName,
				"2": content.RuleName,
				"3": content.Level,
				"4": content.Time,
				"5": content.Condition,
			}
			go func(cfg twilio.Config, to, body string, vars map[string]string) {
				ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
				defer cancel()
				if err := twilio.New(cfg).SendWhatsApp(ctx, to, body, "", vars); err != nil {
					p.lc.Errorf("whatsapp alert send error: %v", err)
				}
			}(twilioCfg, phone, content.Body, variables)
		case constants.WEBAPI:
			webApiClient := webapi.NewWebApiClient(p.lc, p.dic)
			headermap := make([]map[string]string, 0)
			if header, ok := notify.Option["header"]; ok {
				err := json.Unmarshal([]byte(header), &headermap)
				if err != nil {
					return err
				}
			}
			go webApiClient.Send(notify.Option["webhook"], headermap, alertRule, device, product, req)
		}
	}

	return nil
}

// emailProvider 邮件发送渠道：smtp（默认）或 twilio。
func emailProvider(notify models.SubNotify) string {
	if notify.Option == nil {
		return "smtp"
	}
	switch strings.ToLower(strings.TrimSpace(notify.Option["provider"])) {
	case "twilio", "twilio_email":
		return "twilio"
	default:
		return "smtp"
	}
}

// alertContent 告警通知正文的渲染结果。
type alertContent struct {
	Subject    string
	Body       string
	DeviceName string
	RuleName   string
	Level      string
	Time       string
	Condition  string
}

const alertSubjectTemplate = `设备告警：{{.p1}} - {{.p2}}`

// alertTextTemplate 短信 / 邮件 / WhatsApp 共用的纯文本正文。
// 中文原文即词条 ID；英文译文在 internal/pkg/i18n/locales/en.go。
const alertTextTemplate = `设备告警通知
设备：{{.p1}}
产品：{{.p2}}
规则：{{.p3}}
级别：{{.p4}}
触发方式：{{.p5}}
触发条件：{{.p6}}
时间：{{.p7}}`

func buildAlertContent(rule models.AlertRule, device models.Device, product models.Product, lang string) alertContent {
	now := time.Now().Format("2006-01-02 15:04:05")
	level := i18n.Trans(lang, string(rule.AlertLevel), nil)
	condition := alertCondition(rule, product)
	trigger := ""
	if len(rule.SubRule) > 0 {
		trigger = i18n.Trans(lang, string(rule.SubRule[0].Trigger), nil)
	}
	body := i18n.Trans(lang, alertTextTemplate, map[string]interface{}{
		"p1": device.Name,
		"p2": product.Name,
		"p3": rule.Name,
		"p4": level,
		"p5": trigger,
		"p6": condition,
		"p7": now,
	})
	subject := i18n.Trans(lang, alertSubjectTemplate, map[string]interface{}{
		"p1": device.Name,
		"p2": rule.Name,
	})
	return alertContent{
		Subject:    subject,
		Body:       body,
		DeviceName: device.Name,
		RuleName:   rule.Name,
		Level:      level,
		Time:       now,
		Condition:  condition,
	}
}

// alertCondition 把规则条件转成便于人阅读的一行文案。
func alertCondition(rule models.AlertRule, product models.Product) string {
	if len(rule.SubRule) == 0 {
		return ""
	}
	sub := rule.SubRule[0]
	option := sub.Option
	switch sub.Trigger {
	case constants.DeviceDataTrigger:
		code := option["code"]
		name := code
		for _, property := range product.Properties {
			if property.Code == code && property.Name != "" {
				name = property.Name + "[" + code + "]"
				break
			}
		}
		parts := []string{name}
		if condition := strings.TrimSpace(option["decide_condition"]); condition != "" {
			parts = append(parts, condition)
		}
		if cycle := strings.TrimSpace(option["value_cycle"]); cycle != "" {
			parts = append(parts, cycle)
		}
		return strings.Join(parts, " ")
	case constants.DeviceEventTrigger:
		return option["code"]
	case constants.DeviceStatusTrigger:
		return option["status"]
	default:
		return ""
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

// loadEmailConfig 读取平台级 SMTP 配置。配置缺失时返回零值，发送端会记录错误而不会阻塞告警入库。
func (p alertApp) loadEmailConfig(ctx context.Context) email.Config {
	port := 587
	if raw := p.sysConfig(ctx, "notify.smtp.port"); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v > 0 && v <= 65535 {
			port = v
		}
	}
	tlsEnabled := true
	if raw := p.sysConfig(ctx, "notify.smtp.tls"); raw != "" {
		tlsEnabled = !strings.EqualFold(raw, "false")
	}
	return email.Config{
		Host:     p.sysConfig(ctx, "notify.smtp.host"),
		Port:     port,
		Username: p.sysConfig(ctx, "notify.smtp.username"),
		Password: p.sysConfig(ctx, "notify.smtp.password"),
		From:     p.sysConfig(ctx, "notify.smtp.from"),
		TLS:      tlsEnabled,
	}
}

// loadTwilioConfig 读取平台级 Twilio 配置。
func (p alertApp) loadTwilioConfig(ctx context.Context) twilio.Config {
	return twilio.Config{
		AccountSID:         p.sysConfig(ctx, "notify.twilio.accountSid"),
		AuthToken:          p.sysConfig(ctx, "notify.twilio.authToken"),
		EmailFrom:          p.sysConfig(ctx, "notify.twilio.emailFrom"),
		SMSFrom:            p.sysConfig(ctx, "notify.twilio.smsFrom"),
		WhatsAppFrom:       p.sysConfig(ctx, "notify.twilio.whatsappFrom"),
		WhatsAppContentSID: p.sysConfig(ctx, "notify.whatsapp.contentSid"),
		DefaultCountryCode: p.sysConfig(ctx, "notify.phone.defaultCountryCode"),
	}
}

func (p alertApp) sysConfig(ctx context.Context, key string) string {
	cfg, err := p.dbClient.WithCtx(tenant.Platform(ctx)).SysConfigByKey(key)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(cfg.ConfigValue)
}

// notifyLang 通知语言：按通知配置里的 lang 走（前端「通知语言」可选择），没配就用默认语言。
func notifyLang(notify models.SubNotify) string {
	if notify.Option != nil {
		if lang := notify.Option["lang"]; lang != "" {
			return lang
		}
	}
	return i18n.DefaultLang
}

func checkEffectTime(startTime, endTime string) bool {
	if strings.TrimSpace(startTime) == "" || strings.TrimSpace(endTime) == "" {
		// 未配置生效时间段 = 全天生效
		return true
	}
	timeTemplate := "2006-01-02 15:04:05"
	startstamp, _ := time.ParseInLocation(timeTemplate, fmt.Sprintf("%d-%02d-%02d", time.Now().Year(), time.Now().Month(), time.Now().Day())+" "+startTime, time.Local)
	endstamp, _ := time.ParseInLocation(timeTemplate, fmt.Sprintf("%d-%02d-%02d", time.Now().Year(), time.Now().Month(), time.Now().Day())+" "+endTime, time.Local)
	if startstamp.Unix() < time.Now().Unix() && endstamp.Unix() > time.Now().Unix() {
		//发送
		return true
	} else {
		return false
	}
}
func checkEffectTimeParam(startTime, endTime string) bool {
	timeTemplate := "2006-01-02 15:04:05"
	startstamp, _ := time.ParseInLocation(timeTemplate, fmt.Sprintf("%d-%02d-%02d", time.Now().Year(), time.Now().Month(), time.Now().Day())+" "+startTime, time.Local)
	endstamp, _ := time.ParseInLocation(timeTemplate, fmt.Sprintf("%d-%02d-%02d", time.Now().Year(), time.Now().Month(), time.Now().Day())+" "+endTime, time.Local)
	if endstamp.Unix()-startstamp.Unix() <= 0 {
		return false
	}
	return true
}

func (p alertApp) CheckRuleByProductId(ctx context.Context, productId string) error {
	var req dtos.AlertRuleSearchQueryRequest
	req.Status = string(constants.RuleStart)
	alertRules, _, err := p.AlertRulesSearch(ctx, req)
	if err != nil {
		return err
	}
	for _, rule := range alertRules {
		for _, subRule := range rule.SubRule {
			if subRule.ProductId == productId {
				return errort.NewCommonEdgeX(errort.ProductAssociationAlertRule, "This product has been bound"+
					" to alarm rules. Please stop reporting relevant alarm rules before proceeding with the operation.", nil)
			}
		}
	}
	return nil
}

func (p alertApp) CheckRuleByDeviceId(ctx context.Context, deviceId string) error {
	var req dtos.AlertRuleSearchQueryRequest
	req.Status = string(constants.RuleStart)
	alertRules, _, err := p.AlertRulesSearch(ctx, req)
	if err != nil {
		return err
	}
	for _, rule := range alertRules {
		for _, subRule := range rule.SubRule {
			if subRule.DeviceId == deviceId {
				return errort.NewCommonEdgeX(errort.DeviceAssociationAlertRule, "This device has been bound to alarm"+
					" rules. Please stop reporting relevant alarm rules before proceeding with the operation", nil)
			}
		}
	}
	return nil
}
