package middleware

import "github.com/dgrijalva/jwt-go"

type CustomClaims struct {
	ID       uint
	Username string
	// TenantId 所属租户；空串=平台级账号（ROBX 运营）
	TenantId string
	// DeptId 所属组织节点（工厂/车间/产线）
	DeptId string
	// DataScope 数据范围：1全部 2本部门 3本部门及以下 4仅本人
	DataScope string
	// PlatformAdmin 平台（ROBX）管理员：可切换/代管租户
	PlatformAdmin bool
	jwt.StandardClaims
}
