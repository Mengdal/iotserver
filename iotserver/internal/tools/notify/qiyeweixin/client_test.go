package yiqiweixin

import (
	"strings"
	"testing"

	"iotserver/internal/models"
	"iotserver/internal/pkg/constants"
	"iotserver/internal/pkg/i18n"
)

func testClient() *WeixinClient { return &WeixinClient{} }

func TestStatusAlertLocalized(t *testing.T) {
	rule := models.Rule{Trigger: constants.DeviceStatusTrigger}
	device := models.Device{Name: "车间网关", Status: constants.DeviceStatusOnline}

	zh := testClient().generateWeiXinTemplate(rule, device, models.Product{}, "离线告警", i18n.LangZhCN)
	en := testClient().generateWeiXinTemplate(rule, device, models.Product{}, "offline-alarm", i18n.LangEnUS)

	if !strings.Contains(zh.Markdown.Content, "设备已触发状态告警") || !strings.Contains(zh.Markdown.Content, "在线") {
		t.Errorf("中文状态告警不对：%q", zh.Markdown.Content)
	}
	if !strings.Contains(en.Markdown.Content, "Device status alarm triggered") ||
		!strings.Contains(en.Markdown.Content, "Device name:") ||
		!strings.Contains(en.Markdown.Content, "Online") {
		t.Errorf("英文状态告警不对：%q", en.Markdown.Content)
	}
	if strings.Contains(en.Markdown.Content, "设备") || strings.Contains(en.Markdown.Content, "在线") {
		t.Errorf("英文状态告警里混进了中文：%q", en.Markdown.Content)
	}
}

func TestDataAlertLocalized(t *testing.T) {
	rule := models.Rule{
		Trigger: constants.DeviceDataTrigger,
		Option:  models.MapStringString{"code": "temp"},
	}
	product := models.Product{Properties: []models.Properties{{Code: "temp", Name: "温度"}}}
	device := models.Device{Name: "sensor-01"}

	en := testClient().generateWeiXinTemplate(rule, device, product, "high-temp", i18n.LangEnUS)
	if !strings.Contains(en.Markdown.Content, "Device data alarm triggered") ||
		!strings.Contains(en.Markdown.Content, "Property:") ||
		!strings.Contains(en.Markdown.Content, "Triggered at:") {
		t.Errorf("英文数据告警不对：%q", en.Markdown.Content)
	}
	if strings.Contains(en.Markdown.Content, "设备已触发") {
		t.Errorf("英文数据告警里混进了中文：%q", en.Markdown.Content)
	}
}
