package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"iotserver/internal/iotserver/core/container"
	interfaces "iotserver/internal/iotserver/core/interface"
	"iotserver/internal/models"
	"iotserver/internal/pkg/di"
	"iotserver/internal/pkg/tenant"
	"iotserver/internal/pkg/utils"
)

const (
	// auditMaxParamsBytes 入库参数上限（超出截断）
	auditMaxParamsBytes = 2048
	// auditMaxResponseBytes 响应体捕获上限（足够解析 success/errorCode/errorMsg 前缀）
	auditMaxResponseBytes = 8192
	// auditMaxReadBytes 超过该长度的请求体不记录参数（避免大文件上传进内存）
	auditMaxReadBytes = 64 << 10
	// auditMaxMessageLen 失败提示入库长度
	auditMaxMessageLen = 500
)

// auditWhitelist 不记录操作日志的路径片段（登录/刷新令牌/回调/静态/健康检查）
var auditWhitelist = []string{
	"auth/login",
	"auth/init-password",
	"auth/initinfo",
	"ekuiper/",
	"token/",
	"swagger",
	"/ws/",
	"site-info",
	"/health",
	"/healthz",
	"/ping",
	"/favicon",
	"/assets/",
	"/static/",
}

// auditSensitiveKeys 命中即打码的字段名片段（不区分大小写）
var auditSensitiveKeys = []string{
	"password", "secret", "token", "appkey", "privatekey", "accesskey", "credential",
}

// auditModuleByPrefix 路径首段 → 模块名；未命中回落原始路径
var auditModuleByPrefix = map[string]string{
	"product": "产品管理", "products": "产品管理",
	"thingmodel": "物模型管理",
	"device":     "设备管理", "devices": "设备管理", "device-mqtt": "设备管理",
	"gateway": "网关管理", "gateways": "网关管理",
	"sys-user": "用户管理", "user": "用户管理", "pwd": "用户管理",
	"role": "角色管理", "menurole": "角色管理",
	"menu": "菜单管理",
	"dept": "部门管理", "dept-tree": "部门管理",
	"system-setup": "系统配置",
	"tenant":       "租户管理",
	"solution":     "方案中心",
	"alert-rule":   "告警中心", "alert-list": "告警中心", "alert-plate": "告警中心",
	"alert-ignore": "告警中心", "alert-treated": "告警中心", "rule-field": "告警中心",
	"scene":        "场景联动",
	"rule-engine":  "规则引擎",
	"dataresource": "资源管理", "typeresource": "资源管理",
	"viz":              "数据看板",
	"docker-configs":   "驱动镜像",
	"device-libraries": "驱动管理", "device-servers": "驱动管理",
	"device-classify": "驱动管理", "driver-classify": "驱动管理",
	"sys-audit-log": "审查日志", "sys-login-log": "审查日志",
	"auth": "认证",
}

// AuditLog 审查日志（操作日志）中间件：挂在 JWT + 租户上下文之后。
//
//   - 只记非 GET（HEAD/OPTIONS 同样跳过）；
//   - 白名单路径不记（登录、令牌刷新、eKuiper 回调、swagger、静态与健康检查）；
//   - 包装 ResponseWriter 提取 success/errorCode/errorMsg 判断成败；
//   - 参数脱敏 + 截断 2KB；
//   - 通过 AuditLogApp 异步落库（用请求捕获的租户上下文写库），不阻塞业务。
func AuditLog(dic *di.Container) gin.HandlerFunc {
	var app interfaces.AuditLogApp
	if v := dic.Get(container.AuditLogAppName); v != nil {
		app, _ = v.(interfaces.AuditLogApp)
	}
	return func(c *gin.Context) {
		if app == nil || !shouldAuditOperation(c) {
			c.Next()
			return
		}

		start := time.Now()
		tc := tenant.FromContext(c)
		username, _ := UsernameFromContext(c)
		path := c.Request.URL.Path
		method := c.Request.Method
		params := captureAuditParams(c)

		writer := &auditResponseWriter{ResponseWriter: c.Writer, body: &bytes.Buffer{}}
		c.Writer = writer
		c.Next()

		status, code, msg := parseAuditResult(writer.body.Bytes(), c.Writer.Status())
		now := time.Now().UnixMilli()
		record := models.AuditLog{
			Id:         utils.RandomNum(),
			Type:       models.AuditTypeOperation,
			Username:   username,
			Module:     auditModule(path),
			Action:     auditAction(method, path),
			Target:     auditTarget(c, params),
			Method:     method,
			Path:       path,
			Ip:         c.ClientIP(),
			Browser:    utils.GetBrowser(c.Request.UserAgent()),
			Os:         utils.GetOs(c.Request.UserAgent()),
			Status:     status,
			ErrorCode:  code,
			Message:    msg,
			Params:     params,
			DurationMs: time.Since(start).Milliseconds(),
		}
		// 提升字段（内嵌 Timestamps）不能在复合字面量里赋值，构造后单独设置
		record.Created = now
		record.Modified = now
		app.Enqueue(tc, record)
	}
}

// shouldAuditOperation 只记录需要审计的写操作
func shouldAuditOperation(c *gin.Context) bool {
	if c.Request == nil || c.Request.URL == nil {
		return false
	}
	switch c.Request.Method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return false
	}
	path := strings.ToLower(c.Request.URL.Path)
	for _, w := range auditWhitelist {
		if strings.Contains(path, w) {
			return false
		}
	}
	return true
}

// auditResponseWriter 捕获响应体前缀（限长），不改变真实响应
type auditResponseWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

func (w *auditResponseWriter) capture(b []byte) {
	if w.body.Len() >= auditMaxResponseBytes {
		return
	}
	remain := auditMaxResponseBytes - w.body.Len()
	if len(b) > remain {
		b = b[:remain]
	}
	w.body.Write(b)
}

func (w *auditResponseWriter) Write(b []byte) (int, error) {
	w.capture(b)
	return w.ResponseWriter.Write(b)
}

func (w *auditResponseWriter) WriteString(s string) (int, error) {
	w.capture([]byte(s))
	return w.ResponseWriter.WriteString(s)
}

// parseAuditResult 从响应体提取成败/错误码/错误信息，解析失败时回落 HTTP 状态
func parseAuditResult(body []byte, httpStatus int) (int, string, string) {
	if len(body) == 0 {
		if httpStatus >= http.StatusBadRequest {
			return models.LoginStatusFail, "", ""
		}
		return models.LoginStatusSuccess, "", ""
	}
	// 同时兼容后台接口（success/errorCode/errorMsg）与 OpenAPI（success/code/msg）两种响应体
	var resp struct {
		Success   bool   `json:"success"`
		ErrorCode uint32 `json:"errorCode"`
		ErrorMsg  string `json:"errorMsg"`
		Code      uint32 `json:"code"`
		Msg       string `json:"msg"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		// 响应体被截断（大列表）时按前缀判断
		if bytes.Contains(body, []byte(`"success":true`)) && httpStatus < http.StatusBadRequest {
			return models.LoginStatusSuccess, "", ""
		}
		if httpStatus >= http.StatusBadRequest {
			return models.LoginStatusFail, "", ""
		}
		return models.LoginStatusSuccess, "", ""
	}
	if !resp.Success || httpStatus >= http.StatusBadRequest {
		code, msg := resp.ErrorCode, resp.ErrorMsg
		if code == 0 {
			code = resp.Code
		}
		if msg == "" {
			msg = resp.Msg
		}
		codeText := ""
		if code > 0 {
			codeText = strconv.FormatUint(uint64(code), 10)
		}
		return models.LoginStatusFail, codeText, truncateAuditString(msg, auditMaxMessageLen)
	}
	return models.LoginStatusSuccess, "", ""
}

// captureAuditParams 读取请求体并回填（只处理 JSON），返回脱敏 + 截断后的参数
func captureAuditParams(c *gin.Context) string {
	if c.Request == nil || c.Request.Body == nil {
		return ""
	}
	if !strings.Contains(strings.ToLower(c.GetHeader("Content-Type")), "json") {
		return ""
	}
	if c.Request.ContentLength > auditMaxReadBytes {
		return ""
	}
	raw, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return ""
	}
	c.Request.Body = io.NopCloser(bytes.NewReader(raw))
	if len(raw) == 0 {
		return ""
	}
	return sanitizeAuditParams(raw)
}

// sanitizeAuditParams 敏感字段打码 + 截断 2KB
func sanitizeAuditParams(raw []byte) string {
	var data interface{}
	if err := json.Unmarshal(raw, &data); err != nil {
		return truncateAuditString(string(raw), auditMaxParamsBytes)
	}
	out, err := json.Marshal(maskAuditSensitive(data))
	if err != nil {
		return truncateAuditString(string(raw), auditMaxParamsBytes)
	}
	return truncateAuditString(string(out), auditMaxParamsBytes)
}

// maskAuditSensitive 递归替换敏感字段值
func maskAuditSensitive(v interface{}) interface{} {
	switch val := v.(type) {
	case map[string]interface{}:
		out := make(map[string]interface{}, len(val))
		for k, item := range val {
			if isAuditSensitiveKey(k) {
				out[k] = "***"
				continue
			}
			out[k] = maskAuditSensitive(item)
		}
		return out
	case []interface{}:
		out := make([]interface{}, 0, len(val))
		for _, item := range val {
			out = append(out, maskAuditSensitive(item))
		}
		return out
	default:
		return v
	}
}

func isAuditSensitiveKey(key string) bool {
	k := strings.ToLower(key)
	for _, s := range auditSensitiveKeys {
		if strings.Contains(k, s) {
			return true
		}
	}
	return false
}

func truncateAuditString(s string, max int) string {
	if max <= 0 || len(s) <= max {
		return s
	}
	return strings.ToValidUTF8(s[:max], "") + "…(截断)"
}

// auditModule 路径首段 → 模块名
func auditModule(path string) string {
	seg := auditFirstSegment(path)
	if seg == "" {
		return path
	}
	if name, ok := auditModuleByPrefix[seg]; ok {
		return name
	}
	return seg
}

// auditFirstSegment 跳过 api/v1/v1.0/openapi 等前缀后的首段（小写）
func auditFirstSegment(path string) string {
	for _, part := range strings.Split(strings.Trim(path, "/"), "/") {
		part = strings.ToLower(part)
		switch part {
		case "", "api", "v1", "v1.0", "openapi":
			continue
		}
		return part
	}
	return ""
}

// auditAction 特征路径优先，其次按 HTTP 方法映射
func auditAction(method, path string) string {
	p := strings.ToLower(path)
	switch {
	case strings.Contains(p, "unrelease"):
		return "取消发布"
	case strings.Contains(p, "-release") || strings.HasSuffix(p, "/release"):
		return "发布"
	case strings.HasSuffix(p, "/start"):
		return "启动"
	case strings.HasSuffix(p, "/stop"):
		return "停止"
	case strings.HasSuffix(p, "/import"):
		return "导入"
	case strings.HasSuffix(p, "/apply"):
		return "应用"
	case strings.HasSuffix(p, "/clear"):
		return "清空"
	case strings.HasSuffix(p, "/command"):
		return "下发命令"
	case strings.HasSuffix(p, "/config"):
		return "下发配置"
	case strings.HasSuffix(p, "/sync"):
		return "同步"
	}
	switch method {
	case http.MethodPost:
		return "新增"
	case http.MethodPut, http.MethodPatch:
		return "修改"
	case http.MethodDelete:
		return "删除"
	}
	return method
}

// auditTarget 操作对象：优先路径参数，其次请求体顶层 name/id，最后回落路径
func auditTarget(c *gin.Context, params string) string {
	for _, p := range c.Params {
		if p.Value == "" {
			continue
		}
		if p.Key == "id" || strings.HasSuffix(p.Key, "Id") || strings.HasSuffix(p.Key, "id") {
			return p.Value
		}
	}
	if params != "" {
		var m map[string]interface{}
		if json.Unmarshal([]byte(params), &m) == nil {
			for _, key := range []string{"name", "id"} {
				if v, ok := m[key].(string); ok && v != "" {
					return v
				}
			}
		}
	}
	if c.Request != nil && c.Request.URL != nil {
		return c.Request.URL.Path
	}
	return ""
}
