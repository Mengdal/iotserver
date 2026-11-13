package iotp

import "time"

// MillisToNanos 把平台毫秒时间转换成 iotp 的 time 列单位（纳秒）。
//
// 写入时 JSON 信封的 timestamp 用毫秒，iotp 落盘后会转成纳秒；
// 查询 SQL 里与 time 比较时必须用纳秒，否则时间窗内查不到数据。
func MillisToNanos(ms int64) int64 {
	if ms <= 0 {
		return 0
	}
	return ms * int64(time.Millisecond)
}

// NanosToMillis 把 iotp 返回的 time 归一化为毫秒。
// iotp 标准返回纳秒；这里兼容微秒/秒，避免不同版本部署出现数量级错误。
func NanosToMillis(value int64) int64 {
	switch {
	case value >= 1e17: // 纳秒
		return value / 1e6
	case value >= 1e14: // 微秒
		return value / 1e3
	case value > 0 && value < 1e11: // 秒
		return value * 1e3
	default: // 毫秒
		return value
	}
}
