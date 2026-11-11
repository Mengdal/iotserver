// Package ingestcommon 存放网关/参考系统接入服务共用的小逻辑。
package ingestcommon

import (
	"sync"
	"time"
)

// OnlineThrottle 同一设备最多多久触发一次「置在线」写入。
const OnlineThrottle = 30 * time.Second

// OnlineTracker 对设备置在线做节流：按消息批量上报时，同一设备不必每批都写元数据库。
type OnlineTracker struct {
	mu   sync.Mutex
	seen map[string]time.Time
}

// Allow 判断本次是否需要写入在线状态；同一设备在节流窗口内只放行一次。
func (t *OnlineTracker) Allow(deviceID string) bool {
	if deviceID == "" {
		return false
	}
	now := time.Now()
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.seen == nil {
		t.seen = make(map[string]time.Time)
	}
	if last, ok := t.seen[deviceID]; ok && now.Sub(last) < OnlineThrottle {
		return false
	}
	t.seen[deviceID] = now
	// 防止 map 无界增长：条目过多时清理已过节流窗口的项。
	if len(t.seen) > 10000 {
		for id, seenAt := range t.seen {
			if now.Sub(seenAt) >= OnlineThrottle {
				delete(t.seen, id)
			}
		}
	}
	return true
}
