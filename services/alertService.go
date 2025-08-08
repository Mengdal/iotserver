package services

import (
	"encoding/json"
	"fmt"
	"github.com/beego/beego/v2/client/orm"
	"github.com/beego/beego/v2/core/logs"
	"iotServer/models"
	"iotServer/models/constants"
	"iotServer/utils"
	"time"
)

type AlertService struct{}

// AddAlert 告警处理方法
func (s *AlertService) AddAlert(req map[string]interface{}) error {
	now := time.Now().UnixMilli()
	alertResult := make(map[string]interface{}) //告警对象
	var subRuleData []map[string]interface{}    //规则内容
	var notifyData []map[string]interface{}     //通知内容
	var content string

	message := req["messageType"]
	ruleId := req["rule_id"].(string)
	deviceId := req["deviceId"].(string)
	alertResult["dn"] = deviceId

	o := orm.NewOrm()
	var rule models.AlertRule
	rule.Name = ruleId
	if err := o.Read(&rule, "Name"); err != nil {
		return fmt.Errorf("查询失败: %v", err)
	}

	// 任意告警若未达到沉默时间跳过
	if rule.SilenceTime > 0 {
		// 查询该规则最新的告警记录
		var latestAlert models.AlertList
		err := o.QueryTable(new(models.AlertList)).
			Filter("AlertRule__Id", rule.Id).
			OrderBy("-TriggerTime").
			Limit(1).
			One(&latestAlert)

		if err != nil {
			return fmt.Errorf("查询最新告警记录失败: %v", err)
		} else {
			// 如果找到了记录且仍在静默期内，则跳过
			if now-latestAlert.TriggerTime < rule.SilenceTime*60*1000 {
				logs.Info("告警规则 %s 处于静默期，跳过本次告警", ruleId)
				return nil
			}
		}
	}

	if err := json.Unmarshal([]byte(rule.SubRule), &subRuleData); err != nil {
		return fmt.Errorf("查询失败: %v", err)
	}
	if err := json.Unmarshal([]byte(rule.Notify), &notifyData); err != nil {
		return fmt.Errorf("查询失败: %v", err)
	}
	alertResult["trigger"] = subRuleData[0]["trigger"]

	//处理不同告警类型的返回格式
	if message == "PROPERTY_REPORT" {
		value := InterfaceToString(req["alert_value"])

		// 正确地提取嵌套的 option.code
		if option, ok := subRuleData[0]["option"].(map[string]interface{}); ok {
			code, codeOk := option["code"].(string)
			name, nameOk := option["name"].(string)
			cycle, cycleOk := option["value_cycle"].(string)
			typeR, typeROk := option["value_type"].(string)

			if !codeOk || !nameOk || !cycleOk || !typeROk {
				return fmt.Errorf("无法提取必要字段: code存在=%t, name存在=%t, cycle存在=%t, type存在=%t", codeOk, nameOk, cycleOk, typeROk)
			}
			alertResult["code"] = code
			alertResult["name"] = name
			alertResult["cycle"] = cycle
			alertResult["type"] = constants.GetValueTypeLabel(typeR)
		}

		if req["window_start"] != nil && req["window_end"] != nil {
			alertResult["start_at"] = req["window_start"]
			alertResult["end_at"] = req["window_end"]
			content = fmt.Sprintf("【告警通知】设备：%s，属性：%s，触发类型：%s，告警时间：%s ~ %s，%s%s：%s，请及时处理！",
				alertResult["dn"], alertResult["name"], alertResult["trigger"], utils.FormatTimestamp(alertResult["start_at"]), utils.FormatTimestamp(alertResult["end_at"].(float64)), alertResult["cycle"], alertResult["type"], value)
		} else if req["report_time"] != nil {
			reportTime := req["report_time"]
			alertResult["start_at"] = reportTime
			alertResult["end_at"] = reportTime
			content = fmt.Sprintf("【告警通知】设备：%s，属性：%s，触发类型：%s，告警时间：%s，当前值：%s，请及时处理！",
				alertResult["dn"], alertResult["name"], alertResult["trigger"], utils.FormatTimestamp(alertResult["start_at"]), value)
		}

	} else if message == "DEVICE_STATUS" {
		if option, ok := subRuleData[0]["option"].(map[string]interface{}); ok {
			status, statusOk := option["status"].(string)
			if !statusOk {
				return fmt.Errorf("无法提取必要字段: code存在=%t", statusOk)
			}
			alertResult["status"] = constants.GetDeviceStatusLabel(status)
			reportTime := req["report_time"]
			alertResult["start_at"] = reportTime
			content = fmt.Sprintf("【告警通知】设备：%s，触发类型：%s，告警时间：%s，当前状态：%s,请及时处理！",
				alertResult["dn"], alertResult["trigger"], utils.FormatTimestamp(alertResult["start_at"]), alertResult["status"])
		}
	} else {
		return nil
	}

	alertMarshal, err := json.Marshal(alertResult)
	if err != nil {
		return fmt.Errorf("查询失败: %v", err)
	}
	// 构建告警记录
	alert := &models.AlertList{
		AlertRule:   &rule,
		TriggerTime: time.Now().UnixMilli(),
		IsSend:      false,
		Status:      string(constants.Untreated),
		AlertResult: string(alertMarshal),
	}

	// 保存到数据库
	if err = alert.BeforeInsert(); err != nil {
		return fmt.Errorf("插入失败: %v", err)
	}
	if _, err = o.Insert(alert); err != nil {
		return fmt.Errorf("保存告警记录失败: %v", err)
	}

	// 异步发送通知
	go s.sendNotifications(content, notifyData, alert)

	return nil
}

// 增强sendNotifications方法
func (s *AlertService) sendNotifications(content string, notify []map[string]interface{}, alert *models.AlertList) {

	defer func() {
		if r := recover(); r != nil {
			logs.Error("通知发送异常: %v", r)
		}
	}()
	// 获取当前时间
	now := time.Now()
	currentTime := now.Format("15:04:05") // 格式化为 HH:MM:SS
	// 标记是否至少有一个通知发送成功
	successSent := false

	// 遍历所有通知配置
	for _, notifyConfig := range notify {
		// 检查是否在有效时间内
		if utils.IsInEffectiveTime(notifyConfig, currentTime) {
			// 根据通知类型发送通知
			if name, ok := notifyConfig["name"].(string); ok {
				sendSuccess := false
				switch name {
				case "企业微信机器人":
					sendSuccess = s.sendWeComNotification(content, notifyConfig)
				case "钉钉机器人":
					s.sendDingTalkNotification(content, notifyConfig)
				case "sms":
					s.sendSmsNotification(content, notifyConfig)
				case "email":
					s.sendSmsNotification(content, notifyConfig)
				case "API接口":
					sendSuccess = s.sendApiNotification(content, notifyConfig)
				default:
					logs.Warn("未知的通知方式: %s", name)
				}
				if sendSuccess {
					successSent = true
				}
			}
		} else {
			logs.Info("当前时间 %s 不在通知有效时间内，跳过发送", currentTime)
		}
	}

	// 更新数据库中的 is_send 字段
	if successSent {
		o := orm.NewOrm()
		alert.IsSend = true
		if _, err := o.Update(alert, "IsSend"); err != nil {
			logs.Error("更新告警记录的IsSend字段失败: %v", err)
		}
	}
}

// sendWeComNotification 发送企业微信通知
func (s *AlertService) sendWeComNotification(content string, notifyConfig map[string]interface{}) bool {
	if option, ok := notifyConfig["option"].(map[string]interface{}); ok {
		if webhook, ok := option["webhook"].(string); ok && webhook != "" {
			// 这里实现企业微信机器人通知逻辑
			logs.Info("发送企业微信通知: %s 到 webhook: %s", content, webhook)
			// 构造企业微信机器人消息格式
			message := map[string]interface{}{
				"msgtype": "text",
				"text": map[string]interface{}{
					"content": content,
				},
			}
			if err := utils.SendHttpPost(webhook, message); err != nil {
				return false
			}
		}
	}
	return true
}

// sendApiNotification 发送API通知
func (s *AlertService) sendApiNotification(content string, notifyConfig map[string]interface{}) bool {
	if option, ok := notifyConfig["option"].(map[string]interface{}); ok {
		if webhook, ok := option["webhook"].(string); ok && webhook != "" {
			// 这里实现API通知逻辑
			logs.Info("发送API通知: %s 到 URL: %s", content, webhook)
			// 实现具体的API调用逻辑
			if err := utils.SendHttpPost(webhook, content); err != nil {
				return false
			}
		}
	}
	return true
}

// sendDingTalkNotification 发送钉钉通知
func (s *AlertService) sendDingTalkNotification(content string, notifyConfig map[string]interface{}) {
	if option, ok := notifyConfig["option"].(map[string]interface{}); ok {
		if webhook, ok := option["webhook"].(string); ok && webhook != "" {
			// 这里实现钉钉机器人通知逻辑
			logs.Info("发送钉钉通知: %s 到 webhook: %s", content, webhook)
			// TODO: 实现具体的钉钉发送逻辑
		}
	}
}

// sendSmsNotification 发送短信通知
func (s *AlertService) sendSmsNotification(content string, notifyConfig map[string]interface{}) {
	if option, ok := notifyConfig["option"].(map[string]interface{}); ok {
		if phone, ok := option["phone"].(string); ok && phone != "" {
			// 这里实现短信通知逻辑
			logs.Info("发送短信通知: %s 到手机号: %s", content, phone)
			// TODO: 实现具体的短信发送逻辑
		}
	}
}

// sendPhoneNotification 发送电话通知
func (s *AlertService) sendPhoneNotification(content string, notifyConfig map[string]interface{}) {
	if option, ok := notifyConfig["option"].(map[string]interface{}); ok {
		if phone, ok := option["phone"].(string); ok && phone != "" {
			// 这里实现电话通知逻辑
			logs.Info("发送电话通知: %s 到手机号: %s", content, phone)
			// TODO: 实现具体的电话拨打逻辑
		}
	}
}

// 新增方法：批量处理告警状态
func (s *AlertService) BatchUpdateAlertStatus(ids []int64, status string, message string) error {
	o := orm.NewOrm()
	_, err := o.QueryTable("alert_list").
		Filter("id__in", ids).
		Update(orm.Params{
			"status":       status,
			"message":      message,
			"treated_time": time.Now().UnixMilli(),
		})
	return err
}

//// 新增方法：获取告警统计
//func (s *AlertService) GetAlertStats(startTime, endTime int64) (map[string]interface{}, error) {
//	o := orm.NewOrm()
//	stats := make(map[string]interface{})
//
//	// 获取未处理告警数
//	var untreated int64
//	err := o.QueryTable("alert_list").
//		Filter("status", constants.Untreated).
//		Filter("trigger_time__gte", startTime).
//		Filter("trigger_time__lte", endTime).
//		Count(&untreated)
//	if err != nil {
//		return nil, err
//	}
//	stats["untreated"] = untreated
//
//	// 获取已处理告警数
//	var treated int64
//	err = o.QueryTable("alert_list").
//		Filter("status", constants.Treated).
//		Filter("trigger_time__gte", startTime).
//		Filter("trigger_time__lte", endTime).
//		Count(&treated)
//	if err != nil {
//		return nil, err
//	}
//	stats["treated"] = treated
//
//	return stats, nil
//}
