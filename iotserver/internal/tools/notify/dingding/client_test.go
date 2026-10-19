package dingding

import (
	"strings"
	"testing"

	"iotserver/internal/pkg/constants"
	"iotserver/internal/pkg/i18n"
)

// 通知正文要跟请求语言走：中文（默认）与英文各验一遍
func TestDingDingTemplateLocalized(t *testing.T) {
	zh := dingDingTemplate(constants.DeviceDataTrigger, "车间网关", "高温告警", i18n.LangZhCN)
	en := dingDingTemplate(constants.DeviceDataTrigger, "workshop-gw", "high-temp", i18n.LangEnUS)

	if !strings.Contains(zh, "设备名称：车间网关") || !strings.Contains(zh, "触发方式：设备数据触发") {
		t.Errorf("中文通知内容不对：%q", zh)
	}
	if strings.Contains(zh, "Device name") {
		t.Errorf("中文通知里混进了英文：%q", zh)
	}
	if !strings.Contains(en, "Device name: workshop-gw") ||
		!strings.Contains(en, "Alarm rule: high-temp") ||
		!strings.Contains(en, "Trigger: Device data trigger") {
		t.Errorf("英文通知内容不对：%q", en)
	}
	if strings.Contains(en, "设备") {
		t.Errorf("英文通知里混进了中文：%q", en)
	}
}
