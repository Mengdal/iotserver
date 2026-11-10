// Package collectgateway 在请求上下文里传递采集配置所属的软网关 ID。
package collectgateway

import "context"

// ContextKey 中间件写入 gin.Context 时使用的键。
const ContextKey = "collect-gateway"

type ctxKey struct{}

// ginGetter 与 tenant 包一致：gin.Context.Value 默认不回退到 Request.Context。
type ginGetter interface {
	Get(key string) (interface{}, bool)
}

// With 把当前软网关 ID 写入上下文。
func With(ctx context.Context, gatewayID string) context.Context {
	return context.WithValue(ctx, ctxKey{}, gatewayID)
}

// From 读取当前软网关 ID；没有设置时返回空串。
func From(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if g, ok := ctx.(ginGetter); ok {
		if value, exists := g.Get(ContextKey); exists {
			if text, ok := value.(string); ok {
				return text
			}
		}
	}
	value, _ := ctx.Value(ctxKey{}).(string)
	return value
}
