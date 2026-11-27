package httphelper

import (
	"context"
	"net/http"
)

// rawWriterKey 保存 gin 包装之前的底层 http.ResponseWriter。
type rawWriterKey struct{}

// WithRawResponseWriter 把底层 http.ResponseWriter 放进 request context。
//
// gin 会把 http.ResponseWriter 再包一层，gin 1.8 的包装类型没有实现 Unwrap，
// 因此 http.ResponseController 拿不到底层连接。SSE 长连接需要把全局 WriteTimeout
// 清掉时，通过这里保存的原始 writer 调用 SetWriteDeadline。
func WithRawResponseWriter(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := context.WithValue(r.Context(), rawWriterKey{}, w)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RawResponseWriter 返回底层 http.ResponseWriter。
func RawResponseWriter(ctx context.Context) (http.ResponseWriter, bool) {
	if ctx == nil {
		return nil, false
	}
	w, ok := ctx.Value(rawWriterKey{}).(http.ResponseWriter)
	return w, ok
}
