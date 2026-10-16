package constants

import (
	"iotserver/internal/pkg/logger"
)

var (
	//物模型能力类型
	Property = "property"
	Event    = "event"
	Action   = "action"
)

const (
	//gateway
	JwtParsedInfo          = "jwt-parsed-info"
	DefaultAgentReqTimeout = 10

	// 租户上下文：gin 上下文里的 key
	TenantCtxInfo = "tenant-ctx-info"
	// 平台管理员代管租户时使用的请求头
	TenantHeader = "x-tenant-id"
)

// 平台级（非租户）数据归属：内置模板、系统菜单、平台管理员角色等
const PlatformTenantId = ""

// 日志级别
type LogLevel int32

const (
	DebugLevel LogLevel = 0
	InfoLevel           = 1
	WarnLevel           = 2
	ErrorLevel          = 3
)

var LogMap = map[LogLevel]string{
	DebugLevel: logger.DebugLog,
	InfoLevel:  logger.InfoLog,
	WarnLevel:  logger.WarnLog,
	ErrorLevel: logger.ErrorLog,
}

type DeviceServiceStatusType int32

const (
	// 驱动运行状态
	RunStatusStarted = iota + 1
	RunStatusStopped
	RunStatusStarting
	RunStatusStopping
)

// 驱动实例日志
const (
	StatusRead = iota + 1
	StatusStop
)

const (
	// 驱动库操作状态
	OperateStatusDefault    = "default"    // 默认（未安装）
	OperateStatusInstalling = "installing" // 安装中
	OperateStatusInstalled  = "installed"  // 已安装
	OperateStatusUninstall  = "uninstall"  // 未安装
)
