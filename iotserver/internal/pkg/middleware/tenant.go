package middleware

import (
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"

	"iotserver/internal/pkg/constants"
	"iotserver/internal/pkg/errort"
	"iotserver/internal/pkg/httphelper"
	"iotserver/internal/pkg/tenant"
)

// TenantContext 把 JWT 中的租户信息组装成租户上下文，供后续仓储层做行级隔离。
//
// 规则：
//   - 普通用户：租户固定取 token 里的 tenantId，忽略请求头的 x-tenant-id（防越权伪造）；
//   - 平台管理员（PlatformAdmin）：允许用 x-tenant-id 代管某个租户，便于 ROBX 运营跨租户排障。
//
// 必须放在 JWTAuth 之后。
func TenantContext() gin.HandlerFunc {
	return func(c *gin.Context) {
		tc := tenant.Context{}
		if v, ok := c.Get(constants.JwtParsedInfo); ok {
			if claims, ok := v.(*CustomClaims); ok && claims != nil {
				tc.TenantID = claims.TenantId
				tc.DeptID = claims.DeptId
				tc.DataScope = claims.DataScope
				tc.IsPlatformAdmin = claims.PlatformAdmin
			}
		}
		if tc.IsPlatformAdmin {
			if header := strings.TrimSpace(c.GetHeader(constants.TenantHeader)); header != "" {
				// 代管某个租户：只看该租户的数据
				tc.TenantID = header
			} else {
				// 平台视图：ROBX 运营需要跨租户查看与排障
				tc.Platform = true
			}
		}
		c.Set(constants.TenantCtxInfo, tc)
		if c.Request != nil {
			c.Request = c.Request.WithContext(tenant.With(c.Request.Context(), tc))
		}
		c.Next()
	}
}

// PlatformOnly 只允许平台管理员（ROBX 运营账号）访问。
//
// 用于租户管理等平台级接口：即使某个租户角色被误授予了 platform:* 权限，
// 也会在这里被拦住，避免跨租户读取平台数据。必须放在 TenantContext 之后。
func PlatformOnly() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !tenant.FromContext(c).IsPlatformAdmin {
			err := errort.NewCommonErr(errort.AppPermissionDenied, fmt.Errorf("platform admin only"))
			httphelper.RenderFailNoLog(c, err, c.Writer)
			c.Abort()
			return
		}
		c.Next()
	}
}
