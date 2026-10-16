// Package auditlogapp 审查日志（登录 + 操作）的异步写入与过期清理。
//
// 写入：缓冲 channel + 后台协程批量落库（满 100 条或每 1 秒 flush），
// 队列满时丢弃并告警，业务主链路绝不阻塞。
//
// 清理：ticker 每 6 小时的平台作用域任务，按系统配置 data.retentionDays
// 删除 created 超期的记录（0 = 永久保留）。
package auditlogapp

import (
	"context"
	"strconv"
	"strings"
	"sync"
	"time"

	"iotserver/internal/iotserver/core/container"
	interfaces "iotserver/internal/iotserver/core/interface"
	"iotserver/internal/models"
	pkgcontainer "iotserver/internal/pkg/container"
	"iotserver/internal/pkg/di"
	"iotserver/internal/pkg/logger"
	"iotserver/internal/pkg/seed"
	"iotserver/internal/pkg/tenant"
)

const (
	// queueSize 缓冲队列容量，满则丢弃并告警
	queueSize = 4096
	// batchSize 满该条数立即落库
	batchSize = 100
	// flushInterval 不足一批时的最大等待时间
	flushInterval = time.Second
	// cleanupInterval 过期清理周期
	cleanupInterval = 6 * time.Hour
	// defaultRetentionDays 配置缺失/非法时的兜底保留天数
	defaultRetentionDays = 30
)

type entry struct {
	tc  tenant.Context
	log models.AuditLog
}

type auditLogApp struct {
	dbClient interfaces.DBClient
	lc       logger.LoggingClient

	queue    chan entry
	flushReq chan chan struct{}
	exitCh   chan struct{}

	closeOnce sync.Once
	wg        sync.WaitGroup
}

var _ interfaces.AuditLogApp = (*auditLogApp)(nil)

// New 构造审查日志应用并启动写入/清理后台协程
func New(dic *di.Container) *auditLogApp {
	app := &auditLogApp{
		dbClient: container.DBClientFrom(dic.Get),
		lc:       pkgcontainer.LoggingClientFrom(dic.Get),
		queue:    make(chan entry, queueSize),
		flushReq: make(chan chan struct{}),
		exitCh:   make(chan struct{}),
	}
	app.wg.Add(2)
	go app.flushLoop()
	go app.cleanupLoop()
	return app
}

// Enqueue 非阻塞入队，队列满时丢弃并 WARN
func (a *auditLogApp) Enqueue(tc tenant.Context, log models.AuditLog) {
	select {
	case a.queue <- entry{tc: tc, log: log}:
	default:
		if a.lc != nil {
			a.lc.Warnf("审查日志队列已满（容量 %d），丢弃一条 %s 记录: %s %s",
				queueSize, log.Type, log.Method, log.Path)
		}
	}
}

// Flush 等待后台协程把「当前已入队」的记录全部落库
func (a *auditLogApp) Flush() {
	done := make(chan struct{})
	select {
	case a.flushReq <- done:
		<-done
	case <-a.exitCh:
	}
}

// Close 停止后台协程（剩余记录会先落库）
func (a *auditLogApp) Close() {
	a.closeOnce.Do(func() { close(a.exitCh) })
	a.wg.Wait()
}

// flushLoop 批量写入循环：满 100 条或每 1 秒 flush，退出前落盘剩余记录
func (a *auditLogApp) flushLoop() {
	defer a.wg.Done()
	ticker := time.NewTicker(flushInterval)
	defer ticker.Stop()

	batch := make([]entry, 0, batchSize)
	for {
		select {
		case e := <-a.queue:
			batch = append(batch, e)
			if len(batch) >= batchSize {
				a.flush(batch)
				batch = batch[:0]
			}
		case done := <-a.flushReq:
			batch = a.drain(batch)
			a.flush(batch)
			batch = batch[:0]
			close(done)
		case <-ticker.C:
			if len(batch) > 0 {
				a.flush(batch)
				batch = batch[:0]
			}
		case <-a.exitCh:
			batch = a.drain(batch)
			a.flush(batch)
			return
		}
	}
}

// drain 把队列里当前积压的记录全部取出
func (a *auditLogApp) drain(batch []entry) []entry {
	for {
		select {
		case e := <-a.queue:
			batch = append(batch, e)
		default:
			return batch
		}
	}
}

// flush 按租户作用域分组批量落库（租户插件从 ctx 读取作用域并自动写 tenant_id）
func (a *auditLogApp) flush(batch []entry) {
	if len(batch) == 0 {
		return
	}
	groups := make(map[string][]models.AuditLog, 4)
	scopes := make(map[string]tenant.Context, 4)
	order := make([]string, 0, 4)
	for _, e := range batch {
		key := e.tc.TenantID
		if e.tc.Platform {
			key = "\x00platform"
		}
		if _, ok := groups[key]; !ok {
			order = append(order, key)
			scopes[key] = e.tc
		}
		groups[key] = append(groups[key], e.log)
	}
	for _, key := range order {
		scopeCtx := tenant.With(context.Background(), scopes[key])
		if err := a.dbClient.WithCtx(scopeCtx).AddSysAuditLogs(groups[key]); err != nil && a.lc != nil {
			a.lc.Errorf("审查日志批量写入失败（%d 条）: %v", len(groups[key]), err)
		}
	}
}

// cleanupLoop 过期清理循环
func (a *auditLogApp) cleanupLoop() {
	defer a.wg.Done()
	ticker := time.NewTicker(cleanupInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			_, _ = a.CleanupExpired(context.Background())
		case <-a.exitCh:
			return
		}
	}
}

// CleanupExpired 平台作用域跨租户删除 created < now - retentionDays 的记录。
// retentionDays 为 0 时永久保留（一条不删）。
func (a *auditLogApp) CleanupExpired(ctx context.Context) (int64, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	db := a.dbClient.WithCtx(tenant.Platform(ctx))

	days := defaultRetentionDays
	if cfg, err := db.SysConfigByKey(seed.ConfigKeyRetentionDays); err == nil {
		value := strings.TrimSpace(cfg.ConfigValue)
		if value != "" {
			if v, convErr := strconv.Atoi(value); convErr == nil && v >= 0 {
				days = v
			} else if a.lc != nil {
				a.lc.Warnf("审查日志清理: 保留天数配置非法(%q)，按 %d 天处理", cfg.ConfigValue, defaultRetentionDays)
			}
		}
	}
	if days <= 0 {
		if a.lc != nil {
			a.lc.Info("审查日志清理: 保留天数为 0（永久保留），本次跳过")
		}
		return 0, nil
	}

	cutoff := time.Now().AddDate(0, 0, -days).UnixMilli()
	removed, err := db.DeleteSysAuditLogBefore(cutoff)
	if err != nil {
		if a.lc != nil {
			a.lc.Errorf("审查日志清理失败: %v", err)
		}
		return 0, err
	}
	if a.lc != nil {
		a.lc.Infof("审查日志清理: 保留 %d 天，删除 %d 条 %d 之前的记录", days, removed, cutoff)
	}
	return removed, nil
}
