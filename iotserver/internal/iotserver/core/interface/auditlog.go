package interfaces

import (
	"context"

	"iotserver/internal/models"
	"iotserver/internal/pkg/tenant"
)

// AuditLogApp 审查日志写入与清理。
//
// 业务链路只调用 Enqueue：入队失败（队列满）只告警并丢弃，绝不阻塞业务；
// 真正的落库由后台协程批量完成。
type AuditLogApp interface {
	// Enqueue 非阻塞入队一条审查日志，tc 为请求捕获到的租户上下文
	Enqueue(tc tenant.Context, log models.AuditLog)
	// Flush 立即落库当前队列中已入队的记录（测试与关停使用）
	Flush()
	// CleanupExpired 按 data.retentionDays 清理过期记录，返回删除条数（0 表示永久保留）
	CleanupExpired(ctx context.Context) (int64, error)
	// Close 停止后台协程并落盘剩余记录
	Close()
}
