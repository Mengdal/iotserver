package services

import (
	"encoding/json"
	"fmt"
	"github.com/beego/beego/v2/client/orm"
	"github.com/beego/beego/v2/core/logs"
	"iotServer/common"
	"iotServer/models"
	"iotServer/models/constants"
	"strconv"
	"time"
)

type AlertService struct {
	ekuiperClient *common.EkuiperClient
	notifier      AlertNotifier // 新增通知器接口
}

//func (s *AlertService) CreateMultiDeviceAlertRule(ctx context.Context, req *dtos.RuleUpdateRequest) error {
//	// 1. 验证规则参数
//	if err := s.validateMultiDeviceRule(req); err != nil {
//		return err
//	}
//
//	// 2. 获取设备列表
//	devices, err := s.getDevices(req.DeviceIDs)
//	if err != nil {
//		return err
//	}
//
//	// 3. 验证所有设备属于同一产品
//	productID := devices[0].ProductID
//	for _, device := range devices {
//		if device.ProductID != productID {
//			return fmt.Errorf("all devices must belong to the same product")
//		}
//	}
//
//	// 4. 获取产品信息
//	product, err := s.getProduct(productID)
//	if err != nil {
//		return err
//	}
//
//	// 5. 获取属性类型
//	dataType := s.getPropertyDataType(product, req.SubRule[0].Option["code"])
//
//	// 6. 生成多设备SQL
//	sql := req.BuildEkuiperSql(req.DeviceIDs, dataType)
//	if sql == "" {
//		return fmt.Errorf("failed to build SQL")
//	}
//
//	// 7. 生成动作模板
//	actions := s.buildActions(req.Notify)
//
//	// 8. 创建或更新eKuiper规则
//	ruleID := fmt.Sprintf("rule_%s", req.ID)
//	exist, err := s.ekuiperClient.RuleExist(ctx, ruleID)
//	if err != nil {
//		return err
//	}
//
//	if !exist {
//		err = s.ekuiperClient.CreateRule(ctx, ruleID, sql, actions)
//	} else {
//		err = s.ekuiperClient.UpdateRule(ctx, ruleID, sql, actions)
//	}
//
//	if err != nil {
//		return err
//	}
//
//	// 9. 保存规则到数据库
//	return s.saveMultiDeviceAlertRule(req)
//}

// AlertNotifier 通知器接口
type AlertNotifier interface {
	Send(rule *models.AlertRule, alert *models.AlertList) error
}

// 新增邮件通知实现
type EmailNotifier struct {
	smtpServer string
}

func (n *EmailNotifier) Send(rule *models.AlertRule, alert *models.AlertList) error {
	// 实现邮件发送逻辑
	return nil
}

// 新增企业微信通知实现
type WeComNotifier struct {
	webhookURL string
}

func (n *WeComNotifier) Send(rule *models.AlertRule, alert *models.AlertList) error {
	// 实现企业微信机器人通知
	return nil
}

// NewAlertService 构造函数增强
func NewAlertService(ekuiperURL string, notifiers ...AlertNotifier) *AlertService {
	svc := &AlertService{
		ekuiperClient: common.NewEkuiperClient(ekuiperURL),
	}

	// 组合多个通知器
	if len(notifiers) > 0 {
		svc.notifier = &CompositeNotifier{notifiers: notifiers}
	}
	return svc
}

// CompositeNotifier 组合通知器
type CompositeNotifier struct {
	notifiers []AlertNotifier
}

func (n *CompositeNotifier) Send(rule *models.AlertRule, alert *models.AlertList) error {
	for _, notifier := range n.notifiers {
		if err := notifier.Send(rule, alert); err != nil {
			logs.Error("通知发送失败: %v", err)
		}
	}
	return nil
}

// 增强AddAlert方法
func (s *AlertService) AddAlert(req map[string]interface{}) (*models.AlertList, error) {
	// 参数校验增强
	if req == nil {
		return nil, fmt.Errorf("请求参数不能为空")
	}

	ruleIdStr, ok := req["rule_id"].(string)
	if !ok {
		return nil, fmt.Errorf("缺少rule_id参数")
	}

	ruleId, err := strconv.ParseInt(ruleIdStr, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("无效的rule_id: %v", err)
	}

	// 获取规则时添加缓存
	rule, err := s.GetAlertRuleByID(ruleId)
	if err != nil {
		return nil, fmt.Errorf("获取规则失败: %v", err)
	}

	// 构建告警记录
	alert := &models.AlertList{
		AlertRule:   &models.AlertRule{Id: ruleId},
		TriggerTime: time.Now().UnixMilli(),
		IsSend:      true,
		Status:      string(constants.Untreated),
	}

	// 处理告警内容
	if err := s.processAlertContent(alert, req, rule); err != nil {
		return nil, err
	}

	// 保存到数据库
	if err := s.saveAlertToDB(alert); err != nil {
		return nil, err
	}

	// 异步发送通知
	go s.sendNotifications(rule, alert)

	return alert, nil
}

// 新增方法：处理告警内容
func (s *AlertService) processAlertContent(alert *models.AlertList, req map[string]interface{}, rule *models.AlertRule) error {
	content := make(map[string]interface{})
	content["device_id"] = req["deviceId"]
	content["trigger_time"] = alert.TriggerTime

	// 添加规则信息
	content["rule_name"] = rule.Name
	content["rule_level"] = rule.AlertLevel

	// 序列化内容
	contentBytes, err := json.Marshal(content)
	if err != nil {
		return fmt.Errorf("序列化告警内容失败: %v", err)
	}
	alert.AlertResult = string(contentBytes)
	return nil
}

// 新增方法：保存告警到数据库
func (s *AlertService) saveAlertToDB(alert *models.AlertList) error {
	o := orm.NewOrm()
	if _, err := o.Insert(alert); err != nil {
		return fmt.Errorf("保存告警记录失败: %v", err)
	}
	return nil
}

// 增强sendNotifications方法
func (s *AlertService) sendNotifications(rule *models.AlertRule, alert *models.AlertList) {
	if s.notifier == nil {
		return
	}

	defer func() {
		if r := recover(); r != nil {
			logs.Error("通知发送异常: %v", r)
		}
	}()

	if err := s.notifier.Send(rule, alert); err != nil {
		logs.Error("发送告警通知失败: %v", err)
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

// GetAlertRuleByID 根据ID获取告警规则
func (s *AlertService) GetAlertRuleByID(id int64) (*models.AlertRule, error) {
	o := orm.NewOrm()
	rule := models.AlertRule{Id: id} // 使用值而非指针

	// 从数据库查询
	err := o.Read(&rule)
	if err != nil {
		if err == orm.ErrNoRows {
			return nil, fmt.Errorf("告警规则不存在")
		}
		return nil, fmt.Errorf("查询告警规则失败: %v", err)
	}

	return &rule, nil
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
