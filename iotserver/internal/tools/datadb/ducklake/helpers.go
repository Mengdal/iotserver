package ducklake

import (
	"context"
	"encoding/json"
	"reflect"
	"time"
)

// normalizeCtx 兜底：部分调用方会传 nil（leveldb/tstorage 忽略 ctx），这里统一替换为 Background
func normalizeCtx(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

// nowMillis 当前毫秒时间戳
func nowMillis() int64 {
	return time.Now().UnixNano() / int64(time.Millisecond)
}

// splitValue 把上报值拆成「数值列 / 文本列」，非数值类型统一落文本列
func splitValue(value interface{}) (num interface{}, text interface{}) {
	switch v := value.(type) {
	case nil:
		return nil, nil
	case float64:
		return v, nil
	case float32:
		return float64(v), nil
	case int:
		return float64(v), nil
	case int32:
		return float64(v), nil
	case int64:
		return float64(v), nil
	case uint64:
		return float64(v), nil
	case bool:
		if v {
			return float64(1), nil
		}
		return float64(0), nil
	case string:
		return nil, v
	default:
		if b, err := json.Marshal(v); err == nil {
			return nil, string(b)
		}
		return nil, ""
	}
}

// marshalPayload 序列化事件/服务载荷，并尽量取出其中的上报时间
func marshalPayload(payload interface{}) (body string, eventTime int64) {
	switch v := payload.(type) {
	case nil:
		return "", 0
	case string:
		eventTime = timeFromJSON([]byte(v))
		return v, eventTime
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return "", 0
	}
	return string(b), timeFromJSON(b)
}

// timeFromJSON 从载荷 JSON 中读取可能的时间字段（毫秒）
func timeFromJSON(b []byte) int64 {
	if len(b) == 0 {
		return 0
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(b, &raw); err != nil {
		return 0
	}
	for _, key := range []string{"time", "eventTime", "event_time", "reportTime", "report_time"} {
		if v, ok := raw[key]; ok {
			switch t := v.(type) {
			case float64:
				return normalizeMillis(int64(t))
			case int64:
				return normalizeMillis(t)
			case json.Number:
				if n, err := t.Int64(); err == nil {
					return normalizeMillis(n)
				}
			}
		}
	}
	return 0
}

// normalizeMillis 秒级时间戳统一为毫秒
func normalizeMillis(ts int64) int64 {
	if ts > 0 && ts < 1e12 {
		return ts * 1000
	}
	return ts
}

// unmarshalInto 把载荷 JSON 反序列化到目标结构
func unmarshalInto(payload string, target interface{}) {
	if payload == "" {
		return
	}
	_ = json.Unmarshal([]byte(payload), target)
}

// timeWindow 计算查询时间窗（毫秒）。
// range 有两个元素时取区间；否则 last 时取最近 30 分钟，与 tstorage 行为一致。
func timeWindow(rng []int64, last bool) (start, end int64) {
	if len(rng) == 2 {
		if rng[0] < rng[1] {
			return rng[0], rng[1]
		}
		return rng[1], rng[0]
	}
	now := time.Now()
	if last {
		return now.Add(-30*time.Minute).UnixNano() / int64(time.Millisecond), nowMillis()
	}
	// 未指定时间窗：给一个足够大的默认范围（近 1 年）
	return now.AddDate(-1, 0, 0).UnixNano() / int64(time.Millisecond), nowMillis()
}

// normalizePage 归一化分页参数；isAll 时取单页上限
func normalizePage(page, pageSize int, isAll bool) (int, int) {
	if isAll {
		return 1, maxPageSize
	}
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = defaultPageSize
	}
	if pageSize > maxPageSize {
		pageSize = maxPageSize
	}
	return page, pageSize
}

// isNilValue 便于测试断言
func isNilValue(v interface{}) bool {
	if v == nil {
		return true
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Ptr, reflect.Interface, reflect.Slice, reflect.Map:
		return rv.IsNil()
	default:
		return false
	}
}
