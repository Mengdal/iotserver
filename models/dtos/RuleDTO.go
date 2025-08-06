package dtos

import (
	"errors"
	"fmt"
	"iotServer/models"
	"iotServer/models/constants"
	"strings"
	"time"
)

type RuleCreate struct {
	Id          int64  `json:"id"`
	AlertLevel  string `json:"alert_level" example:"紧急/重要/次要/提示"`
	Description string `json:"description" example:"描述"`
	Name        string `json:"name" example:"rule_001"`
}

// RuleRequest 规则创建请求
type RuleRequest struct {
	RuleID    string `json:"ruleId" example:"rule_001"`
	SQL       string `json:"sql" example:"SELECT temperature, humidity FROM sensor_stream WHERE temperature > 30"`
	ActionURL string `json:"actionUrl" example:"http://192.168.0.29:8088/api/ekuiper/callback"`
}

// OperateRuleReq 规则启停请求
type OperateRuleReq struct {
	Action string `json:"action"  example:"stop"` // start 或 stop
	RuleID string `json:"ruleId"  example:"rule_001"`
}

// RuleResponse 规则状态响应
type RuleResponse struct {
	Id     string `json:"id"`
	Status string `json:"status"`
	Name   string `json:"name"`
	Trace  bool   `json:"trace"`
}

type RuleUpdateRequest struct {
	Name        string                    `json:"name" example:"48023899"`    // 规则ID
	Condition   constants.WorkerCondition `json:"condition" example:"anyone"` // 执行条件
	SubRule     []models.SubRule          `json:"sub_rule"`                   // 子规则列
	Notify      []models.Notify           `json:"notify"`                     // 通知配置
	SilenceTime int64                     `json:"silence_time" example:"0"`   // 静默时间
}

// 生成不同事件触发器

func (req *RuleUpdateRequest) BuildEkuiperSql(deviceIDs []string, dataType string) string {
	var sql string

	switch req.SubRule[0].Trigger {
	case "设备数据触发":
		sql = req.buildMultiDeviceDataSql(deviceIDs, dataType)
	case "设备事件触发":
		sql = req.buildMultiDeviceEventSql(deviceIDs)
	case "设备状态触发":
		sql = req.buildMultiDeviceStatusSql(deviceIDs)
	}

	return sql
}

// 1. 创建设备数据触发
func (req *RuleUpdateRequest) buildMultiDeviceDataSql(deviceIDs []string, dataType string) string {
	code := req.SubRule[0].Option["code"]
	decideCondition := req.SubRule[0].Option["decide_condition"] //判断条件
	valueType := req.SubRule[0].Option["value_type"]

	// 构建设备ID条件
	deviceCondition := buildDeviceCondition(deviceIDs)

	switch dataType {
	case "int", "float":
		return req.buildMultiDeviceNumericSql(deviceCondition, code, decideCondition, valueType)
	case "text":
		return req.buildMultiDeviceTextSql(deviceCondition, code, decideCondition)
	case "bool":
		return req.buildMultiDeviceBoolSql(deviceCondition, code, decideCondition)
	}
	return ""
}

// 1.1 设备数据数字类型触发SQL查询修改
func (req *RuleUpdateRequest) buildMultiDeviceNumericSql(deviceCondition, code, decideCondition, valueType string) string {
	windowSize := req.getWindowSize()

	// WHERE 条件部分：设备判断 + 报文类型 + 属性存在判断
	baseWhere := fmt.Sprintf(`(%s) AND messageType = "PROPERTY_REPORT" AND json_path_exists(data, "$.%s") = true`, deviceCondition, code)

	// 判断表达式：数值型属性解析并比较
	decideExpr := fmt.Sprintf(`CAST(json_path_query(data, "$.%s.value"), "float") %s`, code, decideCondition)

	switch valueType {
	case "original":
		return fmt.Sprintf(
			`SELECT rule_id(), json_path_query(data, "$.%s.time") AS report_time, json_path_query(data, "$.%s.value") as alert_value, dn AS deviceId,messageType FROM stream WHERE %s AND %s`,
			code, code, baseWhere, decideExpr,
		)

	case "avg":
		return fmt.Sprintf(
			`SELECT window_start(), window_end(), rule_id(), dn AS deviceId, avg(CAST(json_path_query(data, "$.%s.value"), "float")) AS alert_value,messageType FROM stream WHERE %s GROUP BY TUMBLINGWINDOW(ss, %d), dn HAVING alert_value %s`,
			code, baseWhere, windowSize, decideCondition,
		)

	case "max":
		return fmt.Sprintf(
			`SELECT window_start(), window_end(), rule_id(), dn AS deviceId, max(CAST(json_path_query(data, "$.%s.value"), "float")) AS alert_value,messageType FROM stream WHERE %s GROUP BY TUMBLINGWINDOW(ss, %d), dn HAVING alert_value %s`,
			code, baseWhere, windowSize, decideCondition,
		)

	case "min":
		return fmt.Sprintf(
			`SELECT window_start(), window_end(), rule_id(), dn AS deviceId, min(CAST(json_path_query(data, "$.%s.value"), "float")) AS alert_value,messageType FROM stream WHERE %s GROUP BY TUMBLINGWINDOW(ss, %d), dn HAVING alert_value %s`,
			code, baseWhere, windowSize, decideCondition,
		)

	case "sum":
		return fmt.Sprintf(
			`SELECT window_start(), window_end(), rule_id(), dn AS deviceId, sum(CAST(json_path_query(data, "$.%s.value"), "float")) AS alert_value,messageType FROM stream WHERE %s GROUP BY TUMBLINGWINDOW(ss, %d), dn HAVING alert_value %s`,
			code, baseWhere, windowSize, decideCondition,
		)

	default:
		return ""
	}
}

// TODO 1.2 设备数据字符类型触发SQL查询修改
func (req *RuleUpdateRequest) buildMultiDeviceTextSql(deviceCondition, code, decideCondition string) string {
	// WHERE 条件部分：设备判断 + 报文类型 + 属性存在判断
	baseWhere := fmt.Sprintf(`(%s) AND messageType = "PROPERTY_REPORT" AND json_path_exists(data, "$.%s") = true`, deviceCondition, code)

	// 判断表达式：文本型属性解析并比较
	decideExpr := fmt.Sprintf(`json_path_query(data, "$.%s.value") %s`, code, decideCondition)

	return fmt.Sprintf(
		`SELECT rule_id(), json_path_query(data, "$.%s.time") AS report_time,json_path_query(data, "$.%s.value") as alert_value, dn AS deviceId,messageType FROM stream WHERE %s AND %s`,
		code, baseWhere, decideExpr,
	)
}

// 1.3 设备数据布尔类型触发SQL查询修改
func (req *RuleUpdateRequest) buildMultiDeviceBoolSql(deviceCond string, code, decideCondition string) string {
	// decideCondition 形如 = true/false
	value := "0"
	if decideCondition == "true" {
		value = "1"
	}
	sql := fmt.Sprintf(
		`SELECT rule_id(),json_path_query(data, "$.%s.time") as report_time,deviceId FROM stream WHERE %s AND messageType = "PROPERTY_REPORT" AND json_path_exists(data, "$.%s") = true AND json_path_query(data, "$.%s.value") = %s`,
		code, deviceCond, code, code, value,
	)
	return sql
}

// 创建设备事件SQL
func (req *RuleUpdateRequest) buildMultiDeviceEventSql(deviceIDs []string) string {
	code := req.SubRule[0].Option["code"]
	deviceCondition := buildDeviceCondition(deviceIDs)

	return fmt.Sprintf(`SELECT rule_id(),json_path_query(data, "$.eventTime") as report_time,dn AS deviceId FROM stream WHERE (%s) AND messageType = "EVENT_REPORT" AND json_path_exists(data, "$.eventCode") = true AND json_path_query(data, "$.eventCode") = "%s"`,
		deviceCondition, code)
}

// 创建设备离线SQL
func (req *RuleUpdateRequest) buildMultiDeviceStatusSql(deviceIDs []string) string {
	status := req.SubRule[0].Option["status"]
	deviceCondition := buildDeviceCondition(deviceIDs)

	return fmt.Sprintf(`SELECT rule_id(),time as report_time,dn AS deviceId,messageType FROM stream WHERE (%s) AND messageType = "DEVICE_STATUS"  AND status = "%s"`,
		deviceCondition, status)
}

// 拼接多设备语句
func buildDeviceCondition(deviceIDs []string) string {
	if len(deviceIDs) == 0 {
		return ""
	}
	if len(deviceIDs) == 1 {
		return fmt.Sprintf(`deviceId = "%s"`, deviceIDs[0])
	}
	// 拼接 IN 语句
	var quotedIDs []string
	for _, id := range deviceIDs {
		quotedIDs = append(quotedIDs, fmt.Sprintf(`"%s"`, id))
	}
	return fmt.Sprintf(`deviceId IN (%s)`, strings.Join(quotedIDs, ", "))
}

func ValidateRuleUpdateRequest(req *RuleUpdateRequest, typeStyle string) error {
	// 1. 校验规则ID
	if req.Name == "" {
		return errors.New("规则ID不能为空")
	}

	// 2. 校验执行条件
	if req.Condition != constants.WorkerConditionAnyone && req.Condition != constants.WorkerConditionAll {
		return errors.New("执行条件必须为 'anyone' 或 'all'")
	}

	// 3. 校验子规则列表
	if len(req.SubRule) == 0 {
		return errors.New("子规则不能为空")
	}

	for _, subRule := range req.SubRule {
		// 3.1 校验触发方式
		if !constants.IsTriggerValid(subRule.Trigger) {
			return fmt.Errorf("非法的触发方式: %s", subRule.Trigger)
		}

		// 3.2 校验设备ID
		if len(subRule.DeviceId) == 0 {
			return errors.New("设备ID不能为空")
		}

		// 3.3 校验 Option 字段
		if subRule.Option == nil {
			return errors.New("规则选项不能为空")
		}

		// 3.4 设备数据触发校验
		if subRule.Trigger == string(constants.DeviceDataTrigger) {
			code, ok := subRule.Option["code"]
			if !ok || code == "" {
				return errors.New("属性点 code 不能为空")
			}
			name, ok := subRule.Option["name"]
			if !ok || name == "" {
				return errors.New("属性点 name 不能为空")
			}

			valueType, ok := subRule.Option["value_type"]
			if !ok || !constants.IsValidValueType(valueType) {
				return errors.New("value_type 必须为 original/avg/max/min/sum")
			}
			// 如果是聚合类型（avg/max/min/sum），必须有 value_cycle
			if valueType != string(constants.Original) {
				valueCycle, ok := subRule.Option["value_cycle"]
				if !ok || !constants.IsAggregationType(valueCycle) {
					return errors.New("聚合类型必须指定合法的 value_cycle（如 '1分钟周期'）")
				}
			}
			decideCondition, ok := subRule.Option["decide_condition"]
			if !ok || decideCondition == "" {
				return errors.New("decide_condition 不能为空")
			}
			if !constants.IsValidCondition(typeStyle, decideCondition) {
				return errors.New("判断条件不合法")
			}
		}
		// 3.5 设备状态触发校验
		if subRule.Trigger == string(constants.DeviceStatusTrigger) {
			status, ok := subRule.Option["status"]
			if !ok || status == "" {
				return errors.New("属性点 status 不能为空")
			}
			if !constants.IsValidDeviceStatus(status) {
				return errors.New("判断条件不合法 online/offline 上线通知/离线报警")
			}
		}

	}

	// 4. 校验通知配置（可选）
	for _, notify := range req.Notify {
		if notify.Name == "" || !constants.IsValidAlertWay(notify.Name) {
			return errors.New("通知方式非法")
		}
		if notify.Option == nil || (notify.Option["webhook"] == "" && notify.Option["email"] == "" && notify.Option["phone"] == "") {
			return errors.New("通知配置必须包含 webhook(包含url)、email、phone 任意方式")
		}
		if err := ValidateTimeRange(notify.StartEffectTime, notify.EndEffectTime); err != nil {
			return errors.New("时间非法" + err.Error())
		}
	}

	// 5. 校验静默时间（可选）
	if req.SilenceTime < 0 {
		return errors.New("静默时间不能为负数")
	}

	return nil
}

// 聚合窗口时间
func (req *RuleUpdateRequest) getWindowSize() int {
	switch req.SubRule[0].Option["value_cycle"] {
	case "1分钟周期":
		return 60
	case "5分钟周期":
		return 5 * 60
	case "15分钟周期":
		return 15 * 60
	case "30分钟周期":
		return 30 * 60
	case "60分钟周期":
		return 60 * 60
	default:
		return 60 // 默认1分钟
	}
}

// ValidateTimeRange 校验时间范围格式和逻辑
func ValidateTimeRange(start, end string) error {
	const layout = "15:04:05"

	// 校验格式
	startTime, err := time.Parse(layout, start)
	if err != nil {
		return fmt.Errorf("开始时间格式错误，应为 HH:mm:ss")
	}

	endTime, err := time.Parse(layout, end)
	if err != nil {
		return fmt.Errorf("结束时间格式错误，应为 HH:mm:ss")
	}

	// 校验结束时间是否大于开始时间
	if !endTime.After(startTime) {
		return fmt.Errorf("结束时间必须大于开始时间")
	}

	return nil
}
