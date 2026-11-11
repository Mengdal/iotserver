package iotp

import (
	"encoding/json"
	"strings"
	"unicode"

	"iotserver/internal/pkg/constants"
)

// DefaultTable 无产品设备（网关子设备等）统一写入的兜底表。
const DefaultTable = "default"

// TableName 产品表名：直接使用产品 ID，不再加 p_ 前缀。
func TableName(productID string) string {
	return FieldName(productID)
}

// DeviceTable 设备应写入的表：有产品用产品表，无产品设备落到 default。
func DeviceTable(productID string) string {
	if strings.TrimSpace(productID) != "" {
		return TableName(productID)
	}
	return DefaultTable
}

// FieldName 把物模型 code 转成 iotp 合法字段名：
// 只保留字母/数字/下划线，保留名加 f_ 前缀。
func FieldName(code string) string {
	var builder strings.Builder
	for _, r := range strings.TrimSpace(code) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' {
			builder.WriteRune(r)
		} else {
			builder.WriteRune('_')
		}
	}
	name := builder.String()
	if name == "" {
		return "f_unknown"
	}
	switch strings.ToLower(name) {
	case "time", "device_id", "table", "name", "gateway",
		"tenant_id", "product_id", "device_sn", "device_key", "dept_id":
		return "f_" + name
	}
	return name
}

// FieldType 平台物模型类型 → iotp 字段类型。
func FieldType(spec constants.SpecsType) string {
	switch spec {
	case constants.SpecsTypeInt:
		return TypeI64
	case constants.SpecsTypeFloat:
		return TypeF64
	case constants.SpecsTypeBool:
		return TypeBool
	default:
		return TypeString
	}
}

// BaseFields 每张表都带的基础标签字段（与设备实例的 tags 对应）。
func BaseFields() []Field {
	return []Field{
		{Name: "tenant_id", Type: TypeString, IsTag: true},
		{Name: "product_id", Type: TypeString, IsTag: true},
		{Name: "device_sn", Type: TypeString, IsTag: true},
		{Name: "device_key", Type: TypeString, IsTag: true},
		{Name: "dept_id", Type: TypeString, IsTag: true},
	}
}

// DedupeFields 按字段名去重，保留首次出现的定义。
func DedupeFields(fields []Field) []Field {
	seen := make(map[string]struct{}, len(fields))
	out := make([]Field, 0, len(fields))
	for _, field := range fields {
		if _, ok := seen[field.Name]; ok {
			continue
		}
		seen[field.Name] = struct{}{}
		out = append(out, field)
	}
	return out
}

// InferType 按运行时值推断 iotp 字段类型（用于无物模型的首包动态字段）。
func InferType(value any) string {
	switch v := value.(type) {
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return TypeI64
	case float32, float64:
		return TypeF64
	case bool:
		return TypeBool
	case json.Number:
		if strings.ContainsAny(string(v), ".eE") {
			return TypeF64
		}
		return TypeI64
	default:
		return TypeString
	}
}
