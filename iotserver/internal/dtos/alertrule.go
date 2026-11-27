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

package dtos

import (
	"fmt"
	"iotserver/internal/models"
	"iotserver/internal/pkg/constants"
	"strings"
)

type RuleAddRequest struct {
	Name        string               `json:"name"`        //名字
	AlertType   constants.AlertType  `json:"alert_type"`  //告警类型
	AlertLevel  constants.AlertLevel `json:"alert_level"` //告警级别
	Description string               `json:"description"` //描述
}

type RuleFieldUpdate struct {
	Id          string               `json:"id"`
	Name        string               `json:"name"`
	AlertLevel  constants.AlertLevel `json:"alert_level"`
	Description string               `json:"description"`
}

type RuleUpdateRequest struct {
	Id        string                    `json:"id"`
	Condition constants.WorkerCondition `json:"condition"` //执行条件
	SubRule   []SubRule                 `json:"sub_rule"`
	Notify    []Notify                  `json:"notify"`
	// SilenceTime 静默时间（毫秒）：nil 表示本次更新不修改，0 表示“无静默”。
	// 用指针区分“未传”和“显式传 0”，否则无法把已有静默时间改回无。
	SilenceTime *int64 `json:"silence_time"` //静默时间
}

type AlertTreatedRequest struct {
	Id      string `json:"id"`
	Message string `json:"message"`
}

type Notify struct {
	Name            constants.AlertWay `json:"name"` //告警方式
	Option          map[string]string  `json:"option"`
	StartEffectTime string             `json:"start_effect_time"` //生效开始时间
	EndEffectTime   string             `json:"end_effect_time"`   //生效结束时间
}

func (b *RuleUpdateRequest) BuildEkuiperSql(deviceId string, specsType constants.SpecsType) string {
	if len(b.SubRule) == 0 {
		return ""
	}
	option := b.SubRule[0].Option
	code := option["code"]
	decideCondition := option["decide_condition"]

	var sql string
	switch specsType {
	case constants.SpecsTypeInt, constants.SpecsTypeFloat:
		var s int
		switch option["value_cycle"] {
		case "1分钟周期":
			s = 60
		case "5分钟周期":
			s = 60 * 5
		case "15分钟周期":
			s = 60 * 15
		case "30分钟周期":
			s = 60 * 30
		case "60分钟周期":
			s = 60 * 60
		default:
			if option["value_type"] != constants.Original {
				return ""
			}
		}
		switch option["value_type"] {
		case constants.Original:
			// 属性报文在总线上已压平为 data.<code>=value，上报时间在消息级 time。
			originalTemp := `SELECT rule_id(),time,deviceId FROM mqtt_stream where deviceId = "%s" and messageType = "PROPERTY_REPORT" and json_path_exists(data, "$.%s") = true and json_path_query(data, "$.%s") %s`
			sql = fmt.Sprintf(originalTemp, deviceId, code, code, decideCondition)

		case constants.Avg:
			sql = buildAggregateEkuiperSql("avg", deviceId, code, s, decideCondition)
		case constants.Max:
			sql = buildAggregateEkuiperSql("max", deviceId, code, s, decideCondition)
		case constants.Min:
			sql = buildAggregateEkuiperSql("min", deviceId, code, s, decideCondition)
		case constants.Sum:
			sql = buildAggregateEkuiperSql("sum", deviceId, code, s, decideCondition)
		}
		return sql
	case constants.SpecsTypeText:
		st := strings.Split(decideCondition, " ")
		if len(st) != 2 {
			return ""
		}
		sqlTemp := `SELECT rule_id(),time,deviceId FROM mqtt_stream where deviceId = "%s" and messageType = "PROPERTY_REPORT" and json_path_exists(data, "$.%s") = true and json_path_query(data, "$.%s") = "%s"`
		sql = fmt.Sprintf(sqlTemp, deviceId, code, code, st[1])
	case constants.SpecsTypeEnum:
		st := strings.Split(decideCondition, " ")
		if len(st) != 2 {
			return ""
		}
		sqlTemp := `SELECT rule_id(),time,deviceId FROM mqtt_stream where deviceId = "%s" and messageType = "PROPERTY_REPORT" and json_path_exists(data, "$.%s") = true and json_path_query(data, "$.%s") = %s`
		sql = fmt.Sprintf(sqlTemp, deviceId, code, code, st[1])
	case constants.SpecsTypeBool:
		st := strings.Split(decideCondition, " ")
		if len(st) != 2 || (st[1] != "true" && st[1] != "false") {
			return ""
		}
		// 总线上的布尔测点就是 JSON true/false，不能和旧的嵌套结构一样比较 1/0。
		sqlTemp := `SELECT rule_id(),time,deviceId FROM mqtt_stream where deviceId = "%s" and messageType = "PROPERTY_REPORT" and json_path_exists(data, "$.%s") = true and json_path_query(data, "$.%s") = %s`
		sql = fmt.Sprintf(sqlTemp, deviceId, code, code, st[1])

	}
	return sql
}

// buildAggregateEkuiperSql 生成周期聚合告警 SQL；属性数据在总线上已是标量。
func buildAggregateEkuiperSql(aggregate, deviceId, code string, seconds int, decideCondition string) string {
	sqlTemp := `SELECT window_start(),window_end(),rule_id(),deviceId,%s(json_path_query(data, "$.%s")) as %s_%s FROM mqtt_stream where deviceId = "%s" and messageType = "PROPERTY_REPORT" and json_path_exists(data, "$.%s") = true GROUP BY TUMBLINGWINDOW(ss, %d) HAVING %s_%s %s`
	return fmt.Sprintf(sqlTemp, aggregate, code, aggregate, code, deviceId, code, seconds, aggregate, code, decideCondition)
}

func ReplaceRuleFields(ds *models.AlertRule, patch RuleFieldUpdate) {
	if patch.Name != "" {
		ds.Name = patch.Name
	}
	if patch.AlertLevel != "" {
		ds.AlertLevel = patch.AlertLevel
	}
	if patch.Description != "" {
		ds.Description = patch.Description
	}
}

func ReplaceRuleModelFields(ds *models.AlertRule, patch RuleUpdateRequest) {
	if patch.Condition != "" {
		ds.Condition = patch.Condition
	}
	if patch.SilenceTime != nil {
		ds.SilenceTime = *patch.SilenceTime
	}
	if len(patch.SubRule) > 0 {
		var newSubRule models.SubRule
		for _, rule := range patch.SubRule {
			newSubRule = append(newSubRule, models.Rule{
				Trigger:   rule.Trigger,
				ProductId: rule.ProductId,
				DeviceId:  rule.DeviceId,
				Option:    rule.Option,
			})
		}
		ds.SubRule = newSubRule
	} else {
		ds.SubRule = nil
	}
	if len(patch.Notify) > 0 {
		var newNotify models.Notify
		for _, notify := range patch.Notify {
			newNotify = append(newNotify, models.SubNotify{
				Name:            notify.Name,
				Option:          notify.Option,
				StartEffectTime: notify.StartEffectTime,
				EndEffectTime:   notify.EndEffectTime,
			})
		}
		ds.Notify = newNotify

	}
}

type SubRule struct {
	Trigger   constants.Trigger `json:"trigger"`
	ProductId string            `json:"product_id"`
	DeviceId  string            `json:"device_id"`
	Option    map[string]string `json:"option"`
}

type RuleResponse struct {
	Id          string                    `json:"id"`
	Name        string                    `json:"name"`
	AlertType   constants.AlertType       `json:"alert_type"`
	AlertLevel  constants.AlertLevel      `json:"alert_level"`
	Status      constants.RuleStatus      `json:"status"`
	Condition   constants.WorkerCondition `json:"condition"`
	SubRule     RuleSubRules              `json:"sub_rule"`
	Notify      models.Notify             `json:"notify"`
	SilenceTime int64                     `json:"silence_time"`
	Description string                    `json:"description"`
	Created     int64                     `json:"created"`
	Modified    int64                     `json:"modified"`
}

type RuleSubRules []RuleSubRule

type RuleSubRule struct {
	Trigger     constants.Trigger `json:"trigger"` //触发方式
	ProductId   string            `json:"product_id"`
	ProductName string            `json:"product_name"`
	DeviceId    string            `json:"device_id"`
	DeviceName  string            `json:"device_name"`
	Code        string            `json:"code"`
	Condition   string            `json:"condition"`
	Option      map[string]string `json:"option"`
}

//type RuleNotifys []RuleNotify
//
//type RuleNotify struct {
//	Name            constants.AlertWay `json:"name"` //告警方式
//	Option          map[string]string  `json:"option"`
//	StartEffectTime string             `json:"start_effect_time"` //生效开始时间
//	EndEffectTime   string             `json:"end_effect_time"`   //生效结束时间
//}

type AlertRuleSearchQueryRequest struct {
	BaseSearchConditionQuery `schema:",inline"`
	Name                     string `schema:"name,omitempty"`
	Status                   string `schema:"status,omitempty"`
	Msg                      string `schema:"msg,omitempty"`
}

type AlertRuleSearchQueryResponse struct {
	Id          string               `json:"id"`
	Name        string               `json:"name"`
	Description string               `json:"description"`
	AlertType   constants.AlertType  `json:"alert_type"`
	AlertLevel  constants.AlertLevel `json:"alert_level"`
	Created     int64                `json:"created"`
	Status      constants.RuleStatus `json:"status"`
	SubRule     []SubRule            `json:"sub_rule"`
}

func RuleSearchQueryResponseFromModel(p models.AlertRule) AlertRuleSearchQueryResponse {
	var subRule []SubRule
	for _, rule := range p.SubRule {
		subRule = append(subRule, SubRule{
			Trigger:   rule.Trigger,
			ProductId: rule.ProductId,
			DeviceId:  rule.DeviceId,
			Option:    rule.Option,
		})
	}
	return AlertRuleSearchQueryResponse{
		Id:          p.Id,
		Name:        p.Name,
		Description: p.Description,
		AlertType:   p.AlertType,
		AlertLevel:  p.AlertLevel,
		Status:      p.Status,
		Created:     p.Created,
		SubRule:     subRule,
	}
}

type AlertSearchQueryRequest struct {
	BaseSearchConditionQuery `schema:",inline"`
	Name                     string `schema:"name,omitempty"`
	AlertLevel               string `schema:"alert_level,omitempty"`
	Status                   string `schema:"status,omitempty"`
	TriggerStartTime         int    `schema:"trigger_start_time,omitempty"`
	TriggerEndTime           int    `schema:"trigger_end_time,omitempty"`
}

type AlertSearchQueryResponse struct {
	Id   string `json:"id"`
	Name string `json:"name"`
	//Description string               `json:"description"`
	AlertResult string               `json:"alert_result"`
	AlertLevel  constants.AlertLevel `json:"alert_level"`
	TriggerTime int64                `json:"trigger_time"`
	TreatedTime int64                `json:"treated_time"`
	Status      string               `json:"status"`
	Message     string               `json:"message"`
	IsSend      bool                 `json:"is_send"`
}

type AlertAddRequest struct {
	DeviceId    string `json:"device_id"`
	TriggerTime int64  `json:"trigger_time"` //触发时间
	RuleId      string `json:"rule_id"`
	Content     string `json:"content"`
}

type AlertPlateQueryResponse struct {
	Count      int                  `json:"count"`
	AlertLevel constants.AlertLevel `json:"alert_level"`
}
