package models

import (
	"github.com/beego/beego/v2/client/orm"
	"time"
)

// AlertRule 告警规则
type AlertRule struct {
	Id          int64  `orm:"auto;pk" json:"id"`
	Created     int64  `orm:"column(created);null" json:"created"`
	Modified    int64  `orm:"column(modified);null" json:"modified"`
	Name        string `orm:"size(255);null;unique" json:"name"`
	DeviceId    string `orm:"size(255);null;index" json:"device_id"`
	AlertType   string `orm:"size(64);null" json:"alert_type"`               // 告警类型
	AlertLevel  string `orm:"size(64);null" json:"alert_level"`              // 告警级别
	Status      string `orm:"size(64);null" json:"status"`                   // 状态：running/stopped
	Condition   string `orm:"type(text);null" json:"condition"`              // 执行条件：anyone/all
	SubRule     string `orm:"type(text);null" json:"sub_rule"`               // 子规则配置(JSON字符串)
	Notify      string `orm:"type(text);null" json:"notify"`                 // 通知配置(JSON字符串)
	SilenceTime int64  `orm:"column(silence_time);null" json:"silence_time"` // 静默时间(毫秒)
	Description string `orm:"type(text);null" json:"description"`

	// 反向关联 alert_list
	AlertLists []*AlertList `orm:"reverse(many);null" json:"alert_lists,omitempty"`
}

// AlertList 告警记录
type AlertList struct {
	Id          int64  `orm:"auto;pk" json:"id"`
	Created     int64  `orm:"column(created);null" json:"created"`
	Modified    int64  `orm:"column(modified);null" json:"modified"`
	TriggerTime int64  `orm:"column(trigger_time);null" json:"trigger_time"` // 触发时间
	AlertResult string `orm:"type(text);null" json:"alert_result"`           // 告警内容(JSON)
	Status      string `orm:"size(64);null" json:"status"`                   // 状态：未处理/已处理/忽略
	TreatedTime int64  `orm:"column(treated_time);null" json:"treated_time"` // 处理时间
	Message     string `orm:"type(text);null" json:"message"`                // 处理意见
	IsSend      bool   `orm:"column(is_send);null" json:"is_send"`           // 是否发送通知

	AlertRule *AlertRule `orm:"rel(fk);column(alert_rule_id);on_delete(do_nothing);on_update(do_nothing);null" json:"alert_rule,omitempty"`
}

// SubRule 规则
type SubRule struct {
	Trigger   string            `json:"trigger"   example:"设备数据触发"`                                                                                                                    //触发方式：设备数据触发/设备事件触发/设备状态触发
	ProductId int64             `json:"productId" example:"83026097"`                                                                                                                  //产品ID
	DeviceId  []string          `json:"deviceId"  example:"YT_test"`                                                                                                                   //edgeDB中的deviceName
	Option    map[string]string `json:"option"    example:"{\"code\": \"CurrentHumidity\", \"value_type\": \"original\",\"value_cycle\": \"60分钟周期\", \"decide_condition\": \"> 39\"}"` //code属性KEY，value_type枚举Trigger，decide_condition枚举ValueTypes
}

// Notify 通知
type Notify struct {
	Name            string            `json:"name" example:"企业微信机器人"`                                                  // 通知方式
	Option          map[string]string `json:"option" example:"{\"webhook\": \"https://big-exabyte-28.webhook.cool\"}"` // 通知参数
	StartEffectTime string            `json:"start_effect_time" example:"00:00:00"`                                    // 生效开始时间
	EndEffectTime   string            `json:"end_effect_time" example:"23:59:59"`                                      // 生效结束时间
}

// BeforeInsert 插入前钩子
func (a *AlertRule) BeforeInsert() error {
	now := time.Now().Unix()
	if a.Created == 0 {
		a.Created = now
	}
	a.Modified = now
	return nil
}

// BeforeUpdate 更新前钩子
func (a *AlertRule) BeforeUpdate() error {
	a.Modified = time.Now().Unix()
	return nil
}

// BeforeInsert 插入前钩子
func (a *AlertList) BeforeInsert() error {
	now := time.Now().Unix()
	if a.Created == 0 {
		a.Created = now
	}
	a.Modified = now
	return nil
}

// BeforeUpdate 更新前钩子
func (a *AlertList) BeforeUpdate() error {
	a.Modified = time.Now().Unix()
	return nil
}

func init() {
	// 注册模型
	orm.RegisterModel(new(AlertRule))
	orm.RegisterModel(new(AlertList))
}
