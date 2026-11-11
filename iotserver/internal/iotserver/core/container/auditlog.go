package container

import (
	interfaces "iotserver/internal/iotserver/core/interface"
	"iotserver/internal/pkg/di"
)

// AuditLogAppName 审查日志应用（异步写入 + 过期清理）
var AuditLogAppName = di.TypeInstanceToName((*interfaces.AuditLogApp)(nil))

