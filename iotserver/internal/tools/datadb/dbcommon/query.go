// Package dbcommon 存放各时序后端共用的查询参数处理。
package dbcommon

import "time"

const (
	// DefaultPageSize 默认每页条数。
	DefaultPageSize = 10
	// MaxPageSize 单页上限。
	MaxPageSize = 1000
)

// TimeWindow 计算查询时间窗（毫秒）。
// rng 有两个元素时取区间；否则 last 时取最近 30 分钟；都没有时给近 1 年的默认范围。
func TimeWindow(rng []int64, last bool) (start, end int64) {
	if len(rng) == 2 {
		if rng[0] < rng[1] {
			return rng[0], rng[1]
		}
		return rng[1], rng[0]
	}
	now := time.Now()
	if last {
		return now.Add(-30*time.Minute).UnixNano() / int64(time.Millisecond), NowMillis()
	}
	return now.AddDate(-1, 0, 0).UnixNano() / int64(time.Millisecond), NowMillis()
}

// NormalizePage 归一化分页参数；IsAll 时取单页上限。
func NormalizePage(page, pageSize int, isAll bool) (int, int) {
	if isAll {
		return 1, MaxPageSize
	}
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = DefaultPageSize
	}
	if pageSize > MaxPageSize {
		pageSize = MaxPageSize
	}
	return page, pageSize
}

// NowMillis 当前毫秒时间戳。
func NowMillis() int64 {
	return time.Now().UnixNano() / int64(time.Millisecond)
}
