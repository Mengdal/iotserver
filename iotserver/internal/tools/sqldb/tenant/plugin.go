// Package tenant 提供 GORM 层的行级租户隔离。
//
// 工作方式：在 sqlite 的 gorm.DB 上注册 Query/Update/Delete/Create 回调，
// 对 models.TenantScopedTableNames() 里的表自动追加 tenant_id 条件（查询/更新/删除）
// 或强制写入当前租户（新增）。
//
// 租户身份来自 Statement.Context（见 internal/pkg/tenant），由应用层通过
// DBClient.WithCtx(ctx) 注入。未注入租户上下文的调用走 Policy：
//   - PolicyOff：完全不过滤（等价改造前行为，仅用于紧急回退）
//   - PolicyWarn：不过滤，但打告警日志（迁移期默认，便于排查未改造的调用点）
//   - PolicyEnforce：fail-closed，只允许 tenant_id = ” 的数据（生产与隔离测试）
package tenant

import (
	"context"
	"errors"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"iotserver/internal/models"
	"iotserver/internal/pkg/constants"
	"iotserver/internal/pkg/logger"
	tenantctx "iotserver/internal/pkg/tenant"
)

// errNoTenant 策略为 enforce 且缺少租户上下文时拒绝写入
var errNoTenant = errors.New("缺少租户上下文：拒绝写入租户隔离表")

// Policy 无租户上下文时的处理策略
type Policy int

const (
	// PolicyOff 不过滤
	PolicyOff Policy = iota
	// PolicyWarn 不过滤但告警（默认，迁移期）
	PolicyWarn
	// PolicyEnforce fail-closed
	PolicyEnforce
)

var (
	currentPolicy = PolicyWarn
	loggingClient logger.LoggingClient
)

// SetPolicy 设置策略（启动时调用一次）
func SetPolicy(p Policy, lc logger.LoggingClient) {
	currentPolicy = p
	loggingClient = lc
}

// CurrentPolicy 返回当前策略
func CurrentPolicy() Policy {
	return currentPolicy
}

// warnf 输出迁移期告警
func warnf(format string, args ...interface{}) {
	if loggingClient != nil {
		loggingClient.Warnf(format, args...)
	}
}

// Register 注册租户回调
func Register(db *gorm.DB) error {
	if err := db.Callback().Query().Before("gorm:query").
		Register("iotserver:tenant_query", tenantQueryCallback); err != nil {
		return err
	}
	if err := db.Callback().Update().Before("gorm:update").
		Register("iotserver:tenant_update", tenantWriteCallback); err != nil {
		return err
	}
	if err := db.Callback().Delete().Before("gorm:delete").
		Register("iotserver:tenant_delete", tenantWriteCallback); err != nil {
		return err
	}
	if err := db.Callback().Create().Before("gorm:create").
		Register("iotserver:tenant_create", tenantCreateCallback); err != nil {
		return err
	}
	return nil
}

// tenantQueryCallback 查询与更新/删除：追加租户条件
func tenantQueryCallback(tx *gorm.DB) {
	tenantWriteCallback(tx)
}

// tenantWriteCallback 给 Query/Update/Delete 追加 tenant_id 条件
func tenantWriteCallback(tx *gorm.DB) {
	table := resolveTable(tx)
	if !models.IsTenantScopedTable(table) {
		return
	}
	tc := tenantctx.FromContext(stmtContext(tx))
	if tc.Platform {
		// 平台作用域（平台运营 / 后台任务 / 系统初始化）：不加限制
		return
	}
	if tc.TenantID == "" {
		switch currentPolicy {
		case PolicyEnforce:
			// fail-closed：无租户上下文只能看到平台级数据
			addTenantWhere(tx, table, constants.PlatformTenantId)
		case PolicyWarn:
			warnf("租户隔离: 表 %s 的查询缺少租户上下文（策略=warn，本次不过滤；上线前需改为 WithCtx 并切换 enforce）", table)
		}
		return
	}
	addTenantWhere(tx, table, tc.TenantID)
}

// tenantCreateCallback 新增：强制写入当前租户
func tenantCreateCallback(tx *gorm.DB) {
	table := resolveTable(tx)
	if !models.IsTenantScopedTable(table) {
		return
	}
	tc := tenantctx.FromContext(stmtContext(tx))
	if tc.Platform {
		return
	}
	if tc.TenantID == "" {
		switch currentPolicy {
		case PolicyEnforce:
			_ = tx.AddError(errNoTenant)
		case PolicyWarn:
			warnf("租户隔离: 表 %s 的新增缺少租户上下文（策略=warn，tenant_id 将为空）", table)
		}
		return
	}
	// 强制覆盖，避免请求体伪造 tenantId
	tx.Statement.SetColumn("tenant_id", tc.TenantID)
}

func addTenantWhere(tx *gorm.DB, table, tenantID string) {
	if tx.Statement == nil {
		return
	}
	tx.Statement.AddClause(clause.Where{
		Exprs: []clause.Expression{
			clause.Eq{
				Column: clause.Column{Table: table, Name: "tenant_id"},
				Value:  tenantID,
			},
		},
	})
}

func resolveTable(tx *gorm.DB) string {
	if tx == nil || tx.Statement == nil {
		return ""
	}
	if tx.Statement.Table != "" {
		return tx.Statement.Table
	}
	if tx.Statement.Schema != nil {
		return tx.Statement.Schema.Table
	}
	return ""
}

func stmtContext(tx *gorm.DB) context.Context {
	if tx != nil && tx.Statement != nil && tx.Statement.Context != nil {
		return tx.Statement.Context
	}
	return context.Background()
}
