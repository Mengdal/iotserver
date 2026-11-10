package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"

	"iotserver/internal/pkg/collectgateway"
)

// CollectGatewayContext 读取采集配置所属软网关 ID（query 或 header），写入请求上下文。
// 采集引擎按网关隔离，后续 collectapp 通过 collectgateway.From(ctx) 取用。
func CollectGatewayContext() gin.HandlerFunc {
	return func(c *gin.Context) {
		gatewayID := strings.TrimSpace(c.Query("gatewayId"))
		if gatewayID == "" {
			gatewayID = strings.TrimSpace(c.GetHeader("x-collect-gateway"))
		}
		if gatewayID != "" {
			c.Set(collectgateway.ContextKey, gatewayID)
			c.Request = c.Request.WithContext(collectgateway.With(c.Request.Context(), gatewayID))
		}
		c.Next()
	}
}
