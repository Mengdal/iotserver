package constants

// 触发方式
type Trigger string

const (
	DeviceDataTrigger   Trigger = "设备数据触发"
	DeviceEventTrigger  Trigger = "设备事件触发"
	DeviceStatusTrigger Trigger = "设备状态触发"
)

// 规则状态
type RuleStatus string

const (
	RuleStart RuleStatus = "running"
	RuleStop  RuleStatus = "stopped"
)

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

// 告警方式
type AlertWay string

const (
	SMS      AlertWay = "sms"
	PHONE    AlertWay = "语音告警"
	QYweixin AlertWay = "企业微信机器人"
	DingDing AlertWay = "钉钉机器人"
	FeiShu   AlertWay = "飞书机器人"
	WEBAPI   AlertWay = "API接口"
)

// 获取所有告警方式
func GetAlertWays() []string {
	return []string{string(SMS), string(PHONE), string(QYweixin), string(DingDing), string(FeiShu), string(WEBAPI)}
}

// 数值类型
const (
	Original = "original"
	Avg      = "avg"
	Max      = "max"
	Min      = "min"
	Sum      = "sum"
)

var ValueTypes = []string{Original, Avg, Max, Min, Sum}

// 判断条件
var DecideConditions = []string{">", ">=", "<", "<=", "=", "!="}
