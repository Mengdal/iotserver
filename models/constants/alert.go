package constants

import "regexp"

// Trigger 触发方式
type Trigger string

const (
	DeviceDataTrigger   Trigger = "设备数据触发"
	DeviceEventTrigger  Trigger = "设备事件触发"
	DeviceStatusTrigger Trigger = "设备状态触发"
)

// IsTriggerValid 判断是否是合法的 Trigger 枚举值
func IsTriggerValid(value string) bool {
	return value == string(DeviceDataTrigger) || value == string(DeviceEventTrigger) || value == string(DeviceStatusTrigger)
}

// 规则状态
type RuleStatus string

const (
	RuleStart RuleStatus = "running"
	RuleStop  RuleStatus = "stopped"
)

// IsStatusValid 判断是否是合法的 status 枚举值
func IsStatusValid(value string) bool {
	return value == string(RuleStart) || value == string(RuleStop)
}

// 告警类型
type AlertType string

const DeviceAlertType AlertType = "设备告警"

// 告警级别
type AlertLevel string

const (
	Urgent        AlertLevel = "紧急"
	Important     AlertLevel = "重要"
	LessImportant AlertLevel = "次要"
	Remind        AlertLevel = "提示"
)

func IsValidAlertLevel(value string) bool {
	return value == string(Urgent) || value == string(Important) || value == string(LessImportant) || value == string(Remind)
}

// 告警状态
type AlertListStatus string

const (
	Ignore    AlertListStatus = "忽略"
	Treated   AlertListStatus = "已处理"
	Untreated AlertListStatus = "未处理"
)

// 执行条件
type WorkerCondition string

const (
	WorkerConditionAnyone WorkerCondition = "anyone"
	WorkerConditionAll    WorkerCondition = "all"
)

// AlertWay 告警方式 IsValidAlertWay 判断是否为合法通知参数
type AlertWay string

const (
	SMS      AlertWay = "sms"
	PHONE    AlertWay = "语音告警"
	QYweixin AlertWay = "企业微信机器人"
	DingDing AlertWay = "钉钉机器人"
	FeiShu   AlertWay = "飞书机器人"
	WEBAPI   AlertWay = "API接口"
)

func IsValidAlertWay(value string) bool {
	return value == string(SMS) || value == string(PHONE) || value == string(QYweixin) || value == string(DingDing) || value == string(FeiShu) || value == string(WEBAPI)
}

// 数值类型用于取值类型 IsValidValueType 判断是否是合法的 valueType
type valueType string

const (
	Original valueType = "original"
	Avg      valueType = "avg"
	Max      valueType = "max"
	Min      valueType = "min"
	Sum      valueType = "sum"
)

func IsValidValueType(value string) bool {
	switch value {
	case string(Original), string(Avg), string(Max), string(Min), string(Sum):
		return true
	default:
		return false
	}
}

// GetValueTypeLabel 根据 valueType 返回对应的中文标签
func GetValueTypeLabel(valueTypeStr string) string {
	switch valueTypeStr {
	case string(Original):
		return "原始值"
	case string(Avg):
		return "平均值"
	case string(Max):
		return "最大值"
	case string(Min):
		return "最小值"
	case string(Sum):
		return "总和"
	default:
		return "未知"
	}
}

// 取值周期 IsAggregationType 判断是否是聚合类型
type valueCycle string

const (
	One     valueCycle = "1分钟周期"
	Five    valueCycle = "5分钟周期"
	Fifteen valueCycle = "15分钟周期"
	Thirty  valueCycle = "30分钟周期"
	Sixty   valueCycle = "60分钟周期"
)

func IsAggregationType(value string) bool {
	return value == string(One) || value == string(Five) || value == string(Fifteen) || value == string(Thirty) || value == string(Sixty)
}

// 判断条件
var DecideConditions = []string{">", ">=", "<", "<=", "=", "!="}

func IsValidCondition(typeStyle, cond string) bool {
	specsType := IsSpecsType(typeStyle)
	if !specsType {
		return false
	}
	switch typeStyle {
	case "int", "float":
		return isValidNumericCondition(cond)
	case "text", "enum":
		return isValidTextCondition(cond)
	case "bool":
		return isValidBoolCondition(cond)
	default:
		return false
	}
}
func isValidNumericCondition(cond string) bool {
	re := regexp.MustCompile(`^(>=|<=|>|<|=|!=)\s*-?\d+(\.\d+)?$`)
	return re.MatchString(cond)
}

func isValidTextCondition(cond string) bool {
	re := regexp.MustCompile(`^(=|!=)\s*\".*\"$|^(NOT\s+)?LIKE\s*\".*\"$`)
	return re.MatchString(cond)
}

func isValidBoolCondition(cond string) bool {
	return cond == "= 1" || cond == "= 0"
}

// 数据类型
type SpecsType string

const (
	SpecsTypeInt    SpecsType = "int"
	SpecsTypeFloat  SpecsType = "float"
	SpecsTypeText   SpecsType = "text"
	SpecsTypeDate   SpecsType = "date"
	SpecsTypeBool   SpecsType = "bool"
	SpecsTypeEnum   SpecsType = "enum"
	SpecsTypeStruct SpecsType = "struct"
	SpecsTypeArray  SpecsType = "array"
)

func IsSpecsType(value string) bool {
	return value == string(SpecsTypeInt) || value == string(SpecsTypeFloat) || value == string(SpecsTypeText) || value == string(SpecsTypeDate) || value == string(SpecsTypeBool) || value == string(SpecsTypeEnum) || value == string(SpecsTypeStruct) || value == string(SpecsTypeArray)
}

// 执行条件
type ConditionType string

const (
	ConditionTypeTimer  ConditionType = "timer"
	ConditionTypeNotify ConditionType = "notify"
)

func isValidConditionType(value string) bool {
	return value == string(ConditionTypeTimer) || value == string(ConditionTypeNotify)
}

type DeviceStatus string

const (
	DeviceOnline  Trigger = "online"
	DeviceOffline Trigger = "offline"
)

func IsValidDeviceStatus(value string) bool {
	return value == string(DeviceOnline) || value == string(DeviceOffline)
}
func GetDeviceStatusLabel(deviceStatus string) string {
	switch deviceStatus {
	case string(DeviceOnline):
		return "上线"
	case string(DeviceOffline):
		return "离线"
	default:
		return "未知"
	}
}
