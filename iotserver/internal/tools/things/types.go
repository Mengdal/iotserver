package things

import (
	"context"
	"errors"
	"time"
)

const (
	// DefaultBaseURL things 平台默认地址。
	DefaultBaseURL = "https://things.iotddc.com/api"
	// DefaultTimeout 单次 HTTP 请求默认超时。
	DefaultTimeout = 10 * time.Second
	// DefaultRefreshBefore token 空闲超过该时长后主动重新登录。
	// things 的 api token 空闲 2 小时失效，提前刷新可避免请求时才发现过期。
	DefaultRefreshBefore = 90 * time.Minute
	// MaskedValue 敏感配置回显给前端时使用的掩码。
	MaskedValue = "******"
)

// ErrDisabled things 集成未启用。
var ErrDisabled = errors.New("things integration is disabled")

// Config 是一次 things 调用所需的配置快照。
//
// Password 只在内存中用于登录和删除操作，不写入日志、不返回接口。
type Config struct {
	Enabled       bool
	BaseURL       string
	Username      string
	Password      string
	Timeout       time.Duration
	RefreshBefore time.Duration
}

// Normalize 补齐默认值并去掉地址尾部斜杠。
func (c Config) Normalize() Config {
	c.BaseURL = trimRightSlash(c.BaseURL)
	if c.BaseURL == "" {
		c.BaseURL = DefaultBaseURL
	}
	if c.Timeout <= 0 {
		c.Timeout = DefaultTimeout
	}
	if c.RefreshBefore <= 0 {
		c.RefreshBefore = DefaultRefreshBefore
	}
	return c
}

// ConfigProvider 返回当前配置。允许运行时从 sys_config 读取，配置变更无需重启。
type ConfigProvider func(ctx context.Context) (Config, error)

// LoginInfo 登录响应中的非敏感字段。
type LoginInfo struct {
	Token    string
	UserID   int64
	Username string
	RoleID   int64
}

// Project things 项目（edge_project）。
type Project struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Description string `json:"memo"`
}

// Gateway things 网关（edge_gateway）。
//
// 这里**故意不定义** secret / license 字段：即使 things 列表返回了它们，
// JSON 解析也会忽略，避免敏感字段进入本地模型、日志或接口响应。
// 只保留本地镜像需要的字段。things 的 status 是 bool，且本地在线状态由
// /edge 数据面维护，因此不在这里声明；未声明的字段会被 JSON 解析忽略。
type Gateway struct {
	ID        int64  `json:"id"`
	ProjectID int64  `json:"projectId"`
	SN        string `json:"sn"`
	PK        string `json:"pk"`
	Name      string `json:"name"`
	Model     string `json:"model"`
}

// CreateGatewayRequest 直接对应 things gateway/create 的参数，不做业务加工。
//
// Secret / License 由公司或用户提供，IoTServer 只负责透传。
type CreateGatewayRequest struct {
	ProjectID   int64
	SN          string
	Secret      string
	Name        string
	Model       string
	LicenseType string
	License     string
}

// UpdateGatewayRequest 对应 things gateway/modify 的可改字段。
type UpdateGatewayRequest struct {
	ProjectID   int64
	GatewayID   int64
	Name        string
	LicenseType string
	License     string
}

func trimRightSlash(s string) string {
	for len(s) > 0 && s[len(s)-1] == '/' {
		s = s[:len(s)-1]
	}
	return s
}
