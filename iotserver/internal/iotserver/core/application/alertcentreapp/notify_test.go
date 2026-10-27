package alertcentreapp

import (
	"strings"
	"testing"

	"iotserver/internal/dtos"
	"iotserver/internal/models"
	"iotserver/internal/pkg/constants"
	"iotserver/internal/pkg/i18n"
)

func testAlertRule() models.AlertRule {
	return models.AlertRule{
		Name:       "Chiller high temperature",
		AlertLevel: constants.Urgent,
		SubRule: models.SubRule{{
			Trigger: constants.DeviceDataTrigger,
			Option: models.MapStringString{
				"code":             "room_temp",
				"decide_condition": "> 8",
				"value_cycle":      "1m",
			},
		}},
	}
}

func TestBuildAlertContentEnglish(t *testing.T) {
	rule := testAlertRule()
	device := models.Device{Name: "Chiller Room A"}
	product := models.Product{
		Name: "Chiller room",
		Properties: []models.Properties{{
			Code: "room_temp",
			Name: "Room temperature",
		}},
	}

	content := buildAlertContent(rule, device, product, i18n.LangEnUS)
	for _, want := range []string{
		"Device alarm notification",
		"Chiller Room A",
		"Chiller room",
		"Chiller high temperature",
		"Critical",
		"Room temperature[room_temp] > 8 1m",
	} {
		if !strings.Contains(content.Body, want) {
			t.Errorf("body missing %q:\n%s", want, content.Body)
		}
	}
	if !strings.Contains(content.Subject, "Device alarm: Chiller Room A - Chiller high temperature") {
		t.Errorf("unexpected subject: %q", content.Subject)
	}
}

func TestBuildAlertContentChinese(t *testing.T) {
	content := buildAlertContent(testAlertRule(), models.Device{Name: "冷藏库 A"}, models.Product{Name: "冷藏库"}, i18n.LangZhCN)
	if !strings.Contains(content.Body, "设备告警通知") || !strings.Contains(content.Body, "紧急") {
		t.Fatalf("unexpected Chinese body:\n%s", content.Body)
	}
}

func TestCheckNotifyParamAcceptsNewWays(t *testing.T) {
	base := func(name constants.AlertWay) []dtos.Notify {
		return []dtos.Notify{{
			Name:            name,
			StartEffectTime: "00:00:00",
			EndEffectTime:   "23:59:59",
		}}
	}
	for _, way := range []constants.AlertWay{constants.Email, constants.TwilioEmail, constants.SMS, constants.WhatsApp, constants.WEBAPI} {
		if err := checkNotifyParam(base(way)); err != nil {
			t.Errorf("%s should be accepted: %v", way, err)
		}
	}
	if err := checkNotifyParam(base(constants.AlertWay("企业微信机器人"))); err == nil {
		t.Fatal("removed channel should be rejected")
	}
}

func TestEmailProvider(t *testing.T) {
	cases := []struct {
		name string
		opt  models.MapStringString
		want string
	}{
		{"default", nil, "smtp"},
		{"smtp", models.MapStringString{"provider": "smtp"}, "smtp"},
		{"twilio", models.MapStringString{"provider": "twilio"}, "twilio"},
		{"legacy", models.MapStringString{"provider": "twilio_email"}, "twilio"},
	}
	for _, tc := range cases {
		got := emailProvider(models.SubNotify{Name: constants.Email, Option: tc.opt})
		if got != tc.want {
			t.Errorf("%s: got %s want %s", tc.name, got, tc.want)
		}
	}
}

func TestEffectTimeOptional(t *testing.T) {
	if err := checkNotifyParam([]dtos.Notify{{
		Name:            constants.Email,
		StartEffectTime: "",
		EndEffectTime:   "",
	}}); err != nil {
		t.Fatalf("empty effect time should mean all day: %v", err)
	}
	if err := checkNotifyParam([]dtos.Notify{{
		Name:            constants.Email,
		StartEffectTime: "08:00:00",
		EndEffectTime:   "",
	}}); err == nil {
		t.Fatal("only one effect time should be rejected")
	}
	if !checkEffectTime("", "") {
		t.Fatal("empty effect time should always send")
	}
}
