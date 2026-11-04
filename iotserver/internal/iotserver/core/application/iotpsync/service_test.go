package iotpsync

import (
	"testing"

	"iotserver/internal/pkg/constants"
	"iotserver/internal/tools/iotp"
)

func TestFieldType(t *testing.T) {
	cases := map[constants.SpecsType]string{
		constants.SpecsTypeInt:    iotp.TypeI64,
		constants.SpecsTypeFloat:  iotp.TypeF64,
		constants.SpecsTypeBool:   iotp.TypeBool,
		constants.SpecsTypeText:   iotp.TypeString,
		constants.SpecsTypeDate:   iotp.TypeString,
		constants.SpecsTypeEnum:   iotp.TypeString,
		constants.SpecsTypeStruct: iotp.TypeString,
		constants.SpecsTypeArray:  iotp.TypeString,
	}
	for spec, want := range cases {
		if got := iotp.FieldType(spec); got != want {
			t.Fatalf("iotp.FieldType(%s) = %s, want %s", spec, got, want)
		}
	}
}

func TestFieldName(t *testing.T) {
	cases := map[string]string{
		"temperature": "temperature",
		"a-b c":       "a_b_c",
		"device_id":   "f_device_id",
		"tenant_id":   "f_tenant_id",
		"time":        "f_time",
		"":            "f_unknown",
	}
	for code, want := range cases {
		if got := iotp.FieldName(code); got != want {
			t.Fatalf("iotp.FieldName(%q) = %q, want %q", code, got, want)
		}
	}
}

func TestTableName(t *testing.T) {
	if got := iotp.TableName("81234567"); got != "p_81234567" {
		t.Fatalf("iotp.TableName = %q", got)
	}
}

func TestDeviceID(t *testing.T) {
	if got := iotp.DeviceID("81283089"); got != 81283089 {
		t.Fatalf("DeviceID numeric = %d", got)
	}
	first := iotp.DeviceID("device-abc")
	second := iotp.DeviceID("device-abc")
	if first <= 0 || first != second {
		t.Fatalf("DeviceID hash unstable: %d / %d", first, second)
	}
}
