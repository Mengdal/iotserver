package duckdb

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

// 本文件负责批量写入与保留策略。
//
// DuckDB 的每个事务不产生 Parquet 文件，所以
// flush 间隔可以保持很短（默认 2 秒）而不必担心文件数膨胀；保留策略也不再需要
// 「合并小文件 + 过期快照 + 回收无引用文件」三步，DELETE 之后 CHECKPOINT 即可。
//
// 关于持久性：缓冲区内尚未落盘的数据在进程被强杀时会丢失，
// 默认 2 秒 / 5000 行触发一次落盘，可据此评估可接受的丢失窗口。

const (
	// defaultFlushRows 攒够多少行立即落盘
	defaultFlushRows = 5000
	// defaultFlushInterval 最长多久落盘一次
	defaultFlushInterval = 2 * time.Second
	// defaultRetentionDays 默认保留天数（0 表示永久保留）
	defaultRetentionDays = 30
	// defaultMaxOpenConns 数据库连接上限（DuckDB 为进程内 OLAP 引擎，不宜放开）
	defaultMaxOpenConns = 8
	// insertChunkRows 单条 INSERT 语句的最大行数（多行 VALUES 提升批量写入吞吐）
	insertChunkRows = 500
	// defaultMaxBufferRows 缓冲区行数上限。落盘持续失败时缓冲会回灌重试，
	// 若无上限则数据库长时间不可用会把内存吃光；到顶后拒绝新写入并返回错误，
	// 由调用方决定丢弃还是重试（宁可明确报错，也不静默涨到 OOM）。
	defaultMaxBufferRows = 200_000
	// flushBackoffBase / flushBackoffMax 落盘失败后的退避：连续失败按 2 的幂退避，
	// 避免数据库不可用时每秒空转重试（查询前与关闭前的落盘不受退避限制）。
	flushBackoffBase = 2 * time.Second
	flushBackoffMax  = 5 * time.Minute
)

// errBufferFull 缓冲区已满，拒绝新写入
var errBufferFull = errors.New("duckdb: 写入缓冲区已满（落盘持续失败），本次写入被拒绝")

// propRow 待写入的属性行
type propRow struct {
	tenantID string
	deviceID string
	code     string
	ts       int64
	num      interface{}
	text     interface{}
}

// payloadRow 待写入的事件/服务行
type payloadRow struct {
	tenantID string
	deviceID string
	code     string
	ts       int64
	payload  string
}

/** ***************************** 缓冲写入 ***************************** **/

// bufferedLocked 当前缓冲总行数（调用方需持锁）
func (c *Client) bufferedLocked() int {
	return len(c.propBuf) + len(c.eventBuf) + len(c.svcBuf)
}

// enqueueProperty 属性入缓冲；缓冲已达上限时返回 errBufferFull
func (c *Client) enqueueProperty(row propRow) error {
	c.mu.Lock()
	if c.bufferedLocked() >= c.maxBufferRows {
		c.mu.Unlock()
		return errBufferFull
	}
	c.propBuf = append(c.propBuf, row)
	full := len(c.propBuf) >= c.flushRows
	c.mu.Unlock()
	if full {
		c.flushPending()
	}
	return nil
}

// enqueueEvent 事件入缓冲
func (c *Client) enqueueEvent(row payloadRow) error {
	c.mu.Lock()
	if c.bufferedLocked() >= c.maxBufferRows {
		c.mu.Unlock()
		return errBufferFull
	}
	c.eventBuf = append(c.eventBuf, row)
	full := len(c.eventBuf) >= c.flushRows
	c.mu.Unlock()
	if full {
		c.flushPending()
	}
	return nil
}

// enqueueService 服务调用入缓冲
func (c *Client) enqueueService(row payloadRow) error {
	c.mu.Lock()
	if c.bufferedLocked() >= c.maxBufferRows {
		c.mu.Unlock()
		return errBufferFull
	}
	c.svcBuf = append(c.svcBuf, row)
	full := len(c.svcBuf) >= c.flushRows
	c.mu.Unlock()
	if full {
		c.flushPending()
	}
	return nil
}

// flushPending 把三类缓冲一次性落盘（单个事务），查询前与关闭前调用。
// 这条路径**不受退避限制**：必须保证「写入后立刻查询」与「退出前落盘」。
func (c *Client) flushPending() {
	_ = c.flush(true)
}

// flushIdle 供定时器调用：处于退避窗口内则跳过，避免数据库不可用时空转重试。
func (c *Client) flushIdle() {
	_ = c.flush(false)
}

// flush 执行一次落盘。force=false 时若处于失败退避窗口内直接跳过。
func (c *Client) flush(force bool) error {
	c.mu.Lock()
	if !force && !c.nextFlushAt.IsZero() && time.Now().Before(c.nextFlushAt) {
		c.mu.Unlock()
		return nil
	}
	if len(c.propBuf) == 0 && len(c.eventBuf) == 0 && len(c.svcBuf) == 0 {
		c.mu.Unlock()
		return nil
	}
	props := c.propBuf
	events := c.eventBuf
	services := c.svcBuf
	c.propBuf = nil
	c.eventBuf = nil
	c.svcBuf = nil
	c.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	err := c.writeBatch(ctx, props, events, services)

	c.mu.Lock()
	defer c.mu.Unlock()
	if err != nil {
		// 失败的行重新放回缓冲，等下次重试，避免静默丢数据
		c.propBuf = append(props, c.propBuf...)
		c.eventBuf = append(events, c.eventBuf...)
		c.svcBuf = append(services, c.svcBuf...)
		c.failStreak++
		backoff := flushBackoffBase << (c.failStreak - 1)
		if backoff > flushBackoffMax || backoff <= 0 {
			backoff = flushBackoffMax
		}
		c.nextFlushAt = time.Now().Add(backoff)
		buffered := c.bufferedLocked()
		c.lc.Errorf("duckdb: flush batch failed (%d props / %d events / %d services), 第 %d 次连续失败，%s 后重试，缓冲 %d 行: %v",
			len(props), len(events), len(services), c.failStreak, backoff, buffered, err)
		return err
	}
	if c.failStreak > 0 {
		c.lc.Infof("duckdb: flush 已恢复正常（此前连续失败 %d 次）", c.failStreak)
	}
	c.failStreak = 0
	c.nextFlushAt = time.Time{}
	return nil
}

// writeBatch 单事务批量写入，按 insertChunkRows 拆成多条多行 INSERT。
//
// 整个事务由 writeMu 串行化（DuckDB 是单写者引擎，写入串行是有意为之，
// 详见 Client.writeMu 的说明）。
func (c *Client) writeBatch(ctx context.Context, props []propRow, events, services []payloadRow) error {
	if len(props) == 0 && len(events) == 0 && len(services) == 0 {
		return nil
	}
	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	tx, err := c.db.BeginTx(normalizeCtx(ctx), nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if len(props) > 0 {
		const cols = 5
		for start := 0; start < len(props); start += insertChunkRows {
			end := start + insertChunkRows
			if end > len(props) {
				end = len(props)
			}
			var sb strings.Builder
			sb.WriteString("INSERT INTO " + tableProperty +
				" (tenant_id, device_id, code, ts_ms, num_value, text_value) VALUES ")
			args := make([]interface{}, 0, (end-start)*cols)
			for i := start; i < end; i++ {
				if i > start {
					sb.WriteString(",")
				}
				sb.WriteString("(?,?,?,?,?,?)")
				r := props[i]
				args = append(args, r.tenantID, r.deviceID, r.code, r.ts, r.num, r.text)
			}
			if _, err = tx.ExecContext(ctx, sb.String(), args...); err != nil {
				return err
			}
		}
	}

	if len(events) > 0 {
		for start := 0; start < len(events); start += insertChunkRows {
			end := start + insertChunkRows
			if end > len(events) {
				end = len(events)
			}
			var sb strings.Builder
			sb.WriteString("INSERT INTO " + tableEvent +
				" (tenant_id, device_id, event_code, ts_ms, payload) VALUES ")
			args := make([]interface{}, 0, (end-start)*4)
			for i := start; i < end; i++ {
				if i > start {
					sb.WriteString(",")
				}
				sb.WriteString("(?,?,?,?,?)")
				r := events[i]
				args = append(args, r.tenantID, r.deviceID, r.code, r.ts, r.payload)
			}
			if _, err = tx.ExecContext(ctx, sb.String(), args...); err != nil {
				return err
			}
		}
	}

	if len(services) > 0 {
		for start := 0; start < len(services); start += insertChunkRows {
			end := start + insertChunkRows
			if end > len(services) {
				end = len(services)
			}
			var sb strings.Builder
			sb.WriteString("INSERT INTO " + tableService +
				" (tenant_id, device_id, code, ts_ms, payload) VALUES ")
			args := make([]interface{}, 0, (end-start)*4)
			for i := start; i < end; i++ {
				if i > start {
					sb.WriteString(",")
				}
				sb.WriteString("(?,?,?,?,?)")
				r := services[i]
				args = append(args, r.tenantID, r.deviceID, r.code, r.ts, r.payload)
			}
			if _, err = tx.ExecContext(ctx, sb.String(), args...); err != nil {
				return err
			}
		}
	}

	return tx.Commit()
}

/** ***************************** 保留策略 ***************************** **/

// startFlusher 启动定时落盘（兜住「长时间没攒够行数」的情况）
func (c *Client) startFlusher() {
	if c.flushInterval <= 0 {
		return
	}
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		ticker := time.NewTicker(c.flushInterval)
		defer ticker.Stop()
		for {
			select {
			case <-c.stopCh:
				return
			case <-ticker.C:
				// 定时落盘走带退避的路径：数据库不可用时不空转重试
				c.flushIdle()
			}
		}
	}()
}

// 保留策略不再由数据层自己定时执行：
// 应用层（application/retention.go）按租户读取各自的保留天数后调用
// PurgeBefore / PurgeTenantBefore，这样改配置不必重启，也才能做到分级保留。

// PurgeBefore 删除 cutoff（毫秒）之前的所有租户数据，返回删除的行数
func (c *Client) PurgeBefore(ctx context.Context, cutoffMillis int64) (int64, error) {
	return c.purge(ctx, "", cutoffMillis)
}

// PurgeTenantBefore 只删除指定租户 cutoff（毫秒）之前的数据（分级保留）
func (c *Client) PurgeTenantBefore(ctx context.Context, tenantID string, cutoffMillis int64) (int64, error) {
	if tenantID == "" {
		return 0, fmt.Errorf("duckdb: tenantID is required for per-tenant purge")
	}
	return c.purge(ctx, tenantID, cutoffMillis)
}

func (c *Client) purge(ctx context.Context, tenantID string, cutoffMillis int64) (int64, error) {
	ctx = normalizeCtx(ctx)
	// 先把缓冲落盘，避免刚删完又被缓冲里的旧数据写回来
	c.flushPending()

	var deleted int64
	for _, table := range []string{tableProperty, tableEvent, tableService} {
		sqlText := fmt.Sprintf("DELETE FROM %s WHERE ts_ms < ?", table)
		args := []interface{}{cutoffMillis}
		if tenantID != "" {
			sqlText = fmt.Sprintf("DELETE FROM %s WHERE tenant_id = ? AND ts_ms < ?", table)
			args = []interface{}{tenantID, cutoffMillis}
		}
		res, err := c.db.ExecContext(ctx, sqlText, args...)
		if err != nil {
			return deleted, err
		}
		if n, err := res.RowsAffected(); err == nil {
			deleted += n
		}
	}
	if deleted == 0 {
		return 0, nil
	}
	// 把 WAL 合并进主文件，让删除后的空间可以被复用
	if _, err := c.db.ExecContext(ctx, "CHECKPOINT"); err != nil {
		c.lc.Warnf("duckdb: checkpoint after purge failed: %v", err)
	}
	return deleted, nil
}
