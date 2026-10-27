package things

import (
	"errors"
	"fmt"
	"strings"
)

// APIError things 返回的业务错误。
//
// things 网关的鉴权失败会用 HTTP 200 + code=401 返回，因此错误码必须以 envelope
// 里的 Code 为准，不能只看 HTTP status。
type APIError struct {
	Code       int
	Message    string
	HTTPStatus int
	Path       string
}

func (e *APIError) Error() string {
	if e == nil {
		return "things api error"
	}
	if e.Message == "" {
		return fmt.Sprintf("things api %s failed (code=%d, http=%d)", e.Path, e.Code, e.HTTPStatus)
	}
	return fmt.Sprintf("things api %s failed (code=%d, http=%d): %s", e.Path, e.Code, e.HTTPStatus, e.Message)
}

// IsAuthError 是否为 token 失效/未授权。
func IsAuthError(err error) bool {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.Code == 401
	}
	return false
}

// IsNotFound 是否为资源不存在。
//
// things 不同接口的 not found 文案不统一，这里同时看 code 与 message，
// 便于删除操作做幂等处理。
func IsNotFound(err error) bool {
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		return false
	}
	if apiErr.Code == 404 {
		return true
	}
	msg := strings.ToLower(apiErr.Message)
	return strings.Contains(msg, "not exist") ||
		strings.Contains(msg, "not found") ||
		strings.Contains(msg, "不存在")
}

// IsUnauthorizedMessage 兼容网关只返回 message=UNAUTHORIZED 的情况。
func IsUnauthorizedMessage(message string) bool {
	return strings.EqualFold(strings.TrimSpace(message), "UNAUTHORIZED")
}
