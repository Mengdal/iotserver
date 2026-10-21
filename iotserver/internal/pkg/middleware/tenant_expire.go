package middleware

import (
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"iotserver/internal/iotserver/core/container"
	pkgcontainer "iotserver/internal/pkg/container"
	"iotserver/internal/pkg/di"
	"iotserver/internal/pkg/errort"
	"iotserver/internal/pkg/httphelper"
	"iotserver/internal/pkg/logger"
	"iotserver/internal/pkg/tenant"
)

// TenantExpireCheck 租户可用性校验（停用 / 到期），防止「登录后才停用或到期」的会话继续使用。
//
// 规则（设计文档 §3.3）：
//   - 只对租户用户生效：平台视图（tenant_id=”，Platform=true）直接放行 —— 平台管理员不受影响；
//   - expire_at <= 0 表示永久，放行；
//   - 租户查不到（如租户刚被删除）时**不阻碍**并记警告，避免把删除竞态变成 500；
//   - 命中停用返回 AppTenantDisabled（“租户已停用，请联系平台管理员”）；
//   - 命中到期返回 AppTenantExpired（“租户已到期，请联系平台管理员”）。
//
// 必须放在 TenantContext() 之后；只挂在已鉴权的路由组上，
// 因此登录 / 初始化 / 回调等白名单路径天然不受影响。
func TenantExpireCheck(dic *di.Container) gin.HandlerFunc {
	return func(c *gin.Context) {
		tc := tenant.FromContext(c)
		if tc.Platform || tc.TenantID == "" {
			c.Next()
			return
		}
		// 延迟解析依赖：路由注册阶段（如只注册路由的单测）容器里可能还没有 DBClient
		dbClient := container.DBClientFrom(dic.Get)
		lc := pkgcontainer.LoggingClientFrom(dic.Get)
		t, err := dbClient.WithCtx(c.Request.Context()).TenantById(tc.TenantID)
		if err != nil {
			if lc != nil {
				lc.Warnf("到期校验: 租户 %s 查询失败，按不阻碍处理: %v", tc.TenantID, err)
			}
			c.Next()
			return
		}
		if t.Status == 0 && !tc.IsPlatformAdmin {
			renderTenantDisabled(c, lc, t.Id)
			return
		}
		if expired(t.ExpireAt) {
			renderTenantExpired(c, lc, t.Id, t.ExpireAt)
			return
		}
		c.Next()
	}
}

// expired 判断到期时间是否已过（<=0 表示永久）
func expired(expireAt int64) bool {
	return expireAt > 0 && time.Now().UnixMilli() > expireAt
}

// renderTenantExpired 统一输出到期错误并中断请求
func renderTenantExpired(c *gin.Context, lc logger.LoggingClient, tenantId string, expireAt int64) {
	err := errort.NewCommonErr(errort.AppTenantExpired,
		fmt.Errorf("tenant %s expired at %d", tenantId, expireAt))
	httphelper.RenderFail(c, err, c.Writer, lc)
	c.Abort()
}

// renderTenantDisabled 租户被平台停用，统一输出错误并中断请求
func renderTenantDisabled(c *gin.Context, lc logger.LoggingClient, tenantId string) {
	err := errort.NewCommonErr(errort.AppTenantDisabled,
		fmt.Errorf("tenant %s is disabled", tenantId))
	httphelper.RenderFail(c, err, c.Writer, lc)
	c.Abort()
}
