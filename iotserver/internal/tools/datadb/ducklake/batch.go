package ducklake

import (
	"context"
	"fmt"
	"time"
)

// 本文件负责两件事：
//  1. 批量写入：上报先入内存缓冲，按行数/时间攒批，一个事务写一次（显著减少 DuckLake 快照数）；
//  2. 保留策略：定期清理超过保留期的数据，并回收过期快照与孤儿 Parquet 文件。
//
// 关于持久性：缓冲区内尚未落盘的数据在进程被强杀时会丢失，
// 默认 2 秒 / 500 行触发一次落盘，可据此评估可接受的丢失窗口。

const (
	// defaultFlushRows 攒够多少行立即落盘
	defaultFlushRows = 500
	// defaultFlushInterval 最长多久落盘一次
	defaultFlushInterval = 2 * time.Second
	// defaultRetentionDays 默认保留天数（0 表示永久保留）
	defaultRetentionDays = 30
	// maintenanceInterval 保留策略执行周期
	maintenanceInterval = 6 * time.Hour
	// maintenanceInitialDelay 启动后多久做第一次清理（避开启动高峰）
	maintenanceInitialDelay = 1 * time.Minute
)

// propRow 待写入的属性行
type propRow struct {
	deviceId string
	code     string
	ts       int64
	num      interface{}
	text     interface{}
}

// payloadRow 待写入的事件/服务行
type payloadRow struct {
	deviceId string
	code     string
	ts       int64
	payload  string
}

/** ***************************** 缓冲写入 ***************************** **/

// enqueueProperty 属性入缓冲
func (c *Client) enqueueProperty(row propRow) {
	c.mu.Lock()
	c.propBuf = append(c.propBuf, row)
	full := len(c.propBuf) >= c.flushRows
	c.mu.Unlock()
	if full {
		c.flushPending()
	}
}

// enqueueEvent 事件入缓冲
func (c *Client) enqueueEvent(row payloadRow) {
	c.mu.Lock()
	c.eventBuf = append(c.eventBuf, row)
	full := len(c.eventBuf) >= c.flushRows
	c.mu.Unlock()
	if full {
		c.flushPending()
	}
}

// enqueueService 服务调用入缓冲
func (c *Client) enqueueService(row payloadRow) {
	c.mu.Lock()
	c.svcBuf = append(c.svcBuf, row)
	full := len(c.svcBuf) >= c.flushRows
	c.mu.Unlock()
	if full {
		c.flushPending()
	}
}

// flushPending 把三类缓冲一次性落盘（单个事务 = 单个快照）。
// 查询前会调用它，保证「写入后立刻查询」能读到数据。
func (c *Client) flushPending() {
	c.mu.Lock()
	if len(c.propBuf) == 0 && len(c.eventBuf) == 0 && len(c.svcBuf) == 0 {
		c.mu.Unlock()
		return
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
	if err := c.writeBatch(ctx, props, events, services); err != nil {
		c.lc.Errorf("ducklake: flush batch failed (%d props / %d events / %d services): %v",
			len(props), len(events), len(services), err)
		// 失败的行重新放回缓冲，等下次重试，避免静默丢数据
		c.mu.Lock()
		c.propBuf = append(props, c.propBuf...)
		c.eventBuf = append(events, c.eventBuf...)
		c.svcBuf = append(services, c.svcBuf...)
		c.mu.Unlock()
	}
}

// writeBatch 单事务批量写入
func (c *Client) writeBatch(ctx context.Context, props []propRow, events, services []payloadRow) error {
	tx, err := c.db.BeginTx(normalizeCtx(ctx), nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if len(props) > 0 {
		stmt, err := tx.PrepareContext(ctx, fmt.Sprintf(
			"INSERT INTO %s.%s (device_id, code, ts_ms, num_value, text_value) VALUES (?, ?, ?, ?, ?)",
			lakeAlias, tableProperty))
		if err != nil {
			return err
		}
		for _, row := range props {
			if _, err = stmt.ExecContext(ctx, row.deviceId, row.code, row.ts, row.num, row.text); err != nil {
				_ = stmt.Close()
				return err
			}
		}
		_ = stmt.Close()
	}

	if len(events) > 0 {
		stmt, err := tx.PrepareContext(ctx, fmt.Sprintf(
			"INSERT INTO %s.%s (device_id, event_code, ts_ms, payload) VALUES (?, ?, ?, ?)",
			lakeAlias, tableEvent))
		if err != nil {
			return err
		}
		for _, row := range events {
			if _, err = stmt.ExecContext(ctx, row.deviceId, row.code, row.ts, row.payload); err != nil {
				_ = stmt.Close()
				return err
			}
		}
		_ = stmt.Close()
	}

	if len(services) > 0 {
		stmt, err := tx.PrepareContext(ctx, fmt.Sprintf(
			"INSERT INTO %s.%s (device_id, code, ts_ms, payload) VALUES (?, ?, ?, ?)",
			lakeAlias, tableService))
		if err != nil {
			return err
		}
		for _, row := range services {
			if _, err = stmt.ExecContext(ctx, row.deviceId, row.code, row.ts, row.payload); err != nil {
				_ = stmt.Close()
				return err
			}
		}
		_ = stmt.Close()
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
				c.flushPending()
			}
		}
	}()
}

// startMaintenance 启动后台维护（清理超期数据 + 回收快照/文件）
func (c *Client) startMaintenance() {
	if c.retentionDays <= 0 {
		return
	}
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		timer := time.NewTimer(maintenanceInitialDelay)
		defer timer.Stop()
		for {
			select {
			case <-c.stopCh:
				return
			case <-timer.C:
				c.runMaintenance()
				timer.Reset(maintenanceInterval)
			}
		}
	}()
}

func (c *Client) runMaintenance() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	cutoff := time.Now().AddDate(0, 0, -c.retentionDays).UnixNano() / int64(time.Millisecond)
	deleted, err := c.PurgeBefore(ctx, cutoff)
	if err != nil {
		c.lc.Errorf("ducklake: retention purge failed: %v", err)
		return
	}
	c.lc.Infof("ducklake: retention applied (keep %d days), purged %d rows", c.retentionDays, deleted)
}

// PurgeBefore 删除 cutoff（毫秒）之前的数据，并回收过期快照与孤儿文件。
// 返回删除的行数。过期快照被回收后，那部分历史数据版本将无法再读取（用于回收磁盘）。
func (c *Client) PurgeBefore(ctx context.Context, cutoffMillis int64) (int64, error) {
	ctx = normalizeCtx(ctx)
	// 先把缓冲落盘，避免刚删完又被缓冲里的旧数据写回来
	c.flushPending()

	var deleted int64
	for _, table := range []string{tableProperty, tableEvent, tableService} {
		res, err := c.db.ExecContext(ctx,
			fmt.Sprintf("DELETE FROM %s.%s WHERE ts_ms < ?", lakeAlias, table), cutoffMillis)
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

	// 物化 + 回收过期快照 + 清理无引用文件
	if err := c.FlushInlinedData(ctx); err != nil {
		c.lc.Warnf("ducklake: flush after purge failed: %v", err)
	}
	if c.retentionDays > 0 {
		expire := fmt.Sprintf("CALL ducklake_expire_snapshots(%s, older_than => now() - INTERVAL '%d days')",
			quote(lakeAlias), c.retentionDays)
		if _, err := c.db.ExecContext(ctx, expire); err != nil {
			c.lc.Warnf("ducklake: expire snapshots failed: %v", err)
		}
	}
	if _, err := c.db.ExecContext(ctx,
		fmt.Sprintf("CALL ducklake_cleanup_old_files(%s, cleanup_all => true)", quote(lakeAlias))); err != nil {
		c.lc.Warnf("ducklake: cleanup old files failed: %v", err)
	}
	return deleted, nil
}

// Compact 物化内联数据、回收过期快照与无引用文件（不删除任何数据行）。
// 适合在 drop 设备数据、或想立刻回收磁盘空间时手动调用。
func (c *Client) Compact(ctx context.Context, keepDays int) error {
	ctx = normalizeCtx(ctx)
	c.flushPending()
	if err := c.FlushInlinedData(ctx); err != nil {
		return err
	}
	if keepDays > 0 {
		if _, err := c.db.ExecContext(ctx, fmt.Sprintf(
			"CALL ducklake_expire_snapshots(%s, older_than => now() - INTERVAL '%d days')",
			quote(lakeAlias), keepDays)); err != nil {
			c.lc.Warnf("ducklake: expire snapshots failed: %v", err)
		}
	}
	if _, err := c.db.ExecContext(ctx, fmt.Sprintf(
		"CALL ducklake_cleanup_old_files(%s, cleanup_all => true)", quote(lakeAlias))); err != nil {
		c.lc.Warnf("ducklake: cleanup old files failed: %v", err)
	}
	return nil
}

// RetentionDays 当前保留天数（0 = 永久保留）
func (c *Client) RetentionDays() int { return c.retentionDays }
