package userapp

import (
	"testing"

	"iotserver/internal/models"
	"iotserver/internal/tools/things"
)

func TestToConfigResponseMasksPassword(t *testing.T) {
	res := toConfigResponse(models.SystemConfig{
		ConfigKey:   "things.password",
		ConfigValue: "encrypted-value",
		Type:        "password",
	})
	if res.ConfigValue != things.MaskedValue {
		t.Fatalf("password config must be masked, got %q", res.ConfigValue)
	}

	res = toConfigResponse(models.SystemConfig{
		ConfigKey:   "things.username",
		ConfigValue: "svc",
		Type:        "string",
	})
	if res.ConfigValue != "svc" {
		t.Fatalf("non-password config must not be masked, got %q", res.ConfigValue)
	}
}
