// Package tenant 提供多租户上下文：谁在请求、作用域属于哪个租户、是否平台级操作。
//
// 设计要点（与 ROBX 改造方案一致）：
//   - 租户上下文随 context.Context 传递，仓储层通过 GORM 插件从 Statement.Context 读取；
//   - 默认 fail-closed：拿不到租户上下文、且不是平台作用域时，租户表的查询返回空集；
//   - 平台作用域（Platform=true）用于 ROBX 运营、后台任务与系统初始化，可跨租户读写。
package tenant

import (
	"context"

	"iotserver/internal/pkg/constants"
)

// Context 请求的租户上下文
type Context struct {
	// TenantID 租户ID；空串表示平台级数据（内置模板、系统菜单等）
	TenantID string
	// DeptID 当前用户所属组织节点（工厂/产线）
	DeptID string
	// DataScope 数据范围：1全部 2本部门 3本部门及以下 4仅本人
	DataScope string
	// IsPlatformAdmin 平台（ROBX）管理员：可切换/代管租户
	IsPlatformAdmin bool
	// Platform 平台作用域：不受租户过滤限制（运营、后台任务、初始化）
	Platform bool
}

// IsEmpty 是否没有任何租户归属信息
func (c Context) IsEmpty() bool {
	return c.TenantID == "" && !c.Platform
}

type ctxKey struct{}

// ginGetter 只依赖 gin.Context 的一个方法，避免本包反向依赖 gin。
type ginGetter interface {
	Get(key string) (interface{}, bool)
}

// With 把租户上下文放进 context.Context
func With(ctx context.Context, tc Context) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, ctxKey{}, tc)
}

// Platform 返回一个平台作用域的上下文（后台任务/系统初始化/ROBX 运营）
func Platform(ctx context.Context) context.Context {
	return With(ctx, Context{Platform: true})
}

// FromContext 读取租户上下文。
// 支持两种载体：gin.Context（业务请求，中间件写入）与普通 context.Context（后台任务）。
// 取不到时返回零值 —— 调用方应按「无租户」处理（fail-closed）。
func FromContext(ctx context.Context) Context {
	if ctx == nil {
		return Context{}
	}
	// gin.Context 优先：它的 Value() 会代理到 Request.Context()，必须先走 Get。
	if g, ok := ctx.(ginGetter); ok {
		if v, exists := g.Get(constants.TenantCtxInfo); exists {
			if tc, ok := v.(Context); ok {
				return tc
			}
			if tc, ok := v.(*Context); ok && tc != nil {
				return *tc
			}
		}
	}
	if v := ctx.Value(ctxKey{}); v != nil {
		if tc, ok := v.(Context); ok {
			return tc
		}
	}
	return Context{}
}

// IsPlatformScope 当前上下文是否平台作用域（不受租户过滤）
func IsPlatformScope(ctx context.Context) bool {
	return FromContext(ctx).Platform
}
