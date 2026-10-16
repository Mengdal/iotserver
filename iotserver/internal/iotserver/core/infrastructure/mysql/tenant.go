package mysql

import (
	"context"

	interfaces "iotserver/internal/iotserver/core/interface"
)

// WithCtx 返回绑定上下文的客户端副本。
//
// 租户隔离开关在这里：GORM 租户插件（internal/tools/sqldb/tenant）从
// Statement.Context 读取租户上下文，因此业务代码必须使用本方法得到的实例，
// 否则租户表的查询在 enforce 策略下会返回空集。
func (c *Client) WithCtx(ctx context.Context) interfaces.DBClient {
	if ctx == nil {
		return c
	}
	cp := *c
	scoped := c.Pool.WithContext(ctx)
	cp.Pool = scoped
	// 关键：client 包装体（ClientSQLite）持有自己的 Pool，
	// 只替换 cp.Pool 会让走 c.client.* 的查询绕过租户过滤。
	cp.client = c.client.WithPool(scoped)
	return &cp
}
