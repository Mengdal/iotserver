package persistence

import (
	"sync"
	"time"
)

// deviceTenantCache 设备 -> 租户 的本地缓存。
//
// 背景：上行消息（MQTT/网关）本身不带租户，写入时序库时需要的 tenant_id
// 只能从设备归属推导。每条消息都查一次设备表代价太高，因此缓存映射关系。
//
// 设备归属在正常业务里不会频繁变更；TTL 只用于兜底（设备迁移租户后最多滞后一个 TTL）。
type deviceTenantCache struct {
	mu  sync.RWMutex
	m   map[string]deviceTenantEntry
	ttl time.Duration
	// maxEntries 容量上限，超过后先清理过期项，仍然超限则整体重置，避免无界增长
	maxEntries int
}

type deviceTenantEntry struct {
	tenantID string
	expireAt time.Time
}

func newDeviceTenantCache(ttl time.Duration, maxEntries int) *deviceTenantCache {
	if ttl <= 0 {
		ttl = 10 * time.Minute
	}
	if maxEntries <= 0 {
		maxEntries = 100_000
	}
	return &deviceTenantCache{
		m:          make(map[string]deviceTenantEntry),
		ttl:        ttl,
		maxEntries: maxEntries,
	}
}

// get 读取缓存（过期即视为未命中）
func (c *deviceTenantCache) get(deviceID string) (string, bool) {
	if c == nil || deviceID == "" {
		return "", false
	}
	c.mu.RLock()
	e, ok := c.m[deviceID]
	c.mu.RUnlock()
	if !ok {
		return "", false
	}
	if time.Now().After(e.expireAt) {
		c.mu.Lock()
		delete(c.m, deviceID)
		c.mu.Unlock()
		return "", false
	}
	return e.tenantID, true
}

// set 写入缓存
func (c *deviceTenantCache) set(deviceID, tenantID string) {
	if c == nil || deviceID == "" {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.m) >= c.maxEntries {
		c.purgeExpiredLocked()
		if len(c.m) >= c.maxEntries {
			// 仍然超限：整体重置比无界增长更安全（下次访问重新回填）
			c.m = make(map[string]deviceTenantEntry)
		}
	}
	c.m[deviceID] = deviceTenantEntry{tenantID: tenantID, expireAt: time.Now().Add(c.ttl)}
}

// invalidate 使某个设备的缓存失效（设备改属租户时调用）
func (c *deviceTenantCache) invalidate(deviceID string) {
	if c == nil || deviceID == "" {
		return
	}
	c.mu.Lock()
	delete(c.m, deviceID)
	c.mu.Unlock()
}

func (c *deviceTenantCache) purgeExpiredLocked() {
	now := time.Now()
	for k, v := range c.m {
		if now.After(v.expireAt) {
			delete(c.m, k)
		}
	}
}

// size 当前缓存条目数（测试用）
func (c *deviceTenantCache) size() int {
	if c == nil {
		return 0
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.m)
}
