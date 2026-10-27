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

package constants

type Trigger string

const (
	DeviceDataTrigger   Trigger = "设备数据触发"
	DeviceEventTrigger  Trigger = "设备事件触发"
	DeviceStatusTrigger Trigger = "设备状态触发"
)

type RuleStatus string

const (
	RuleStart RuleStatus = "running"
	RuleStop  RuleStatus = "stopped"
)

type AlertType string

const DeviceAlertType AlertType = "设备告警"

type AlertLevel string

const (
	Urgent        AlertLevel = "紧急"
	Important     AlertLevel = "重要"
	LessImportant AlertLevel = "次要"
	Remind        AlertLevel = "提示"
)

type AlertListStatus string

const (
	Ignore    AlertListStatus = "忽略"
	Treated   AlertListStatus = "已处理"
	Untreated AlertListStatus = "未处理"
)

type WorkerCondition string

const (
	WorkerConditionAnyone WorkerCondition = "anyone"
	WorkerConditionAll    WorkerCondition = "all"
)

type AlertWay string

const (
	// Email 邮件通知（SMTP，保留原有行为）
	Email AlertWay = "email"
	// TwilioEmail 邮件通知（Twilio Email API）
	TwilioEmail AlertWay = "twilio_email"
	// SMS 短信通知（Twilio）
	SMS AlertWay = "sms"
	// WhatsApp WhatsApp 通知（Twilio）
	WhatsApp AlertWay = "whatsapp"
	// WEBAPI 自定义 HTTP 回调
	WEBAPI AlertWay = "API接口"
)

func GetAlertWays() []string {
	return []string{string(Email), string(TwilioEmail), string(SMS), string(WhatsApp), string(WEBAPI)}
}

const (
	Original = "original"
	Avg      = "avg"
	Max      = "max"
	Min      = "min"
	Sum      = "sum"
)

var (
	ValueTypes = []string{Original, Avg, Max, Min, Sum}
)

var (
	DecideConditions = []string{">", ">=", "<", "<=", "=", "!="}
)
