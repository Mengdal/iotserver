// Package aggregate 提供历史数据的聚合（时间分桶 + 聚合方法）能力。
//
// 该实现是后端无关的纯函数：SQL 型数据层（如 DuckLake）在 SQL 里分桶，
// 本地 KV / 内存型数据层（LevelDB、TStorage）先取回范围内全部原始行，再调用这里的 Bucketize。
//
// 语义约定（有测试覆盖）：
//   - 桶按 epoch 对齐：bucketStart = ts - (ts % intervalMs)，单位毫秒、与时区无关；
//   - avg/sum/max/min 只统计数值行；桶内没有任何数值行时该桶不输出（避免返回 0 造成误读）；
//   - count 统计桶内全部行数（任意类型）；
//   - first/last 取桶内时间最小/最大那一行的原始值（保留数值/文本类型）；
//   - 返回结果按桶起始时间降序，与既有数据层的返回顺序保持一致。
package aggregate

import (
	"fmt"
	"math"
	"sort"

	"iotserver/internal/dtos"
)

// 聚合方法
const (
	MethodAvg   = "avg"
	MethodMax   = "max"
	MethodMin   = "min"
	MethodSum   = "sum"
	MethodCount = "count"
	MethodFirst = "first"
	MethodLast  = "last"
)

// MaxBuckets 单次聚合允许的最大桶数（防止 30 秒 × 30 天这类组合打爆查询）
const MaxBuckets = 20000

var validMethods = map[string]struct{}{
	MethodAvg:   {},
	MethodMax:   {},
	MethodMin:   {},
	MethodSum:   {},
	MethodCount: {},
	MethodFirst: {},
	MethodLast:  {},
}

var numericMethods = map[string]struct{}{
	MethodAvg: {},
	MethodMax: {},
	MethodMin: {},
	MethodSum: {},
}

// IsValidMethod 是否为受支持的聚合方法
func IsValidMethod(method string) bool {
	_, ok := validMethods[method]
	return ok
}

// IsNumericMethod 是否需要数值（avg/sum/max/min）
func IsNumericMethod(method string) bool {
	_, ok := numericMethods[method]
	return ok
}

// Validate 校验并归一化聚合参数。
// interval <= 0 表示「不聚合」，此时原样返回不做校验。
// 返回归一化后的 interval 与 method（method 为空时默认 avg）。
func Validate(interval int, method string, rng []int64) (int, string, error) {
	if interval <= 0 {
		return 0, "", nil
	}
	if len(rng) != 2 {
		return 0, "", fmt.Errorf("聚合查询必须指定时间范围")
	}
	if method == "" {
		method = MethodAvg
	}
	if !IsValidMethod(method) {
		return 0, "", fmt.Errorf("不支持的聚合方法: %s", method)
	}

	start, end := rng[0], rng[1]
	if start > end {
		start, end = end, start
	}
	intervalMs := int64(interval) * 1000
	buckets := (end-start)/intervalMs + 1
	if buckets > MaxBuckets {
		return 0, "", fmt.Errorf("聚合时间过小：预计 %d 个桶，超过上限 %d，请增大聚合间隔或缩小时间范围",
			buckets, MaxBuckets)
	}
	return interval, method, nil
}

// Bucketize 把同一设备同一属性的原始数据按 intervalSeconds 分桶聚合。
// intervalSeconds <= 0 时原样返回（调用方通常已提前分支）。
func Bucketize(rows []dtos.ReportData, intervalSeconds int, method string) []dtos.ReportData {
	if intervalSeconds <= 0 || len(rows) == 0 {
		return rows
	}
	if method == "" {
		method = MethodAvg
	}
	intervalMs := int64(intervalSeconds) * 1000

	type bucket struct {
		start  int64
		count  int
		sum    float64
		numN   int
		minV   float64
		maxV   float64
		firstV interface{}
		firstT int64
		lastV  interface{}
		lastT  int64
	}

	buckets := make(map[int64]*bucket, len(rows))
	for _, row := range rows {
		start := row.Time - row.Time%intervalMs
		b := buckets[start]
		if b == nil {
			b = &bucket{start: start, minV: math.Inf(1), maxV: math.Inf(-1)}
			buckets[start] = b
		}
		b.count++
		if v, ok := toFloat(row.Value); ok {
			b.sum += v
			b.numN++
			if v < b.minV {
				b.minV = v
			}
			if v > b.maxV {
				b.maxV = v
			}
		}
		if b.firstV == nil || row.Time < b.firstT {
			b.firstV, b.firstT = row.Value, row.Time
		}
		if b.lastV == nil || row.Time > b.lastT {
			b.lastV, b.lastT = row.Value, row.Time
		}
	}

	out := make([]dtos.ReportData, 0, len(buckets))
	for _, b := range buckets {
		switch method {
		case MethodCount:
			out = append(out, dtos.ReportData{Value: float64(b.count), Time: b.start})
		case MethodFirst:
			out = append(out, dtos.ReportData{Value: b.firstV, Time: b.start})
		case MethodLast:
			out = append(out, dtos.ReportData{Value: b.lastV, Time: b.start})
		default:
			if b.numN == 0 {
				continue // 无数值行，该桶不输出
			}
			var v float64
			switch method {
			case MethodSum:
				v = b.sum
			case MethodMin:
				v = b.minV
			case MethodMax:
				v = b.maxV
			default: // avg
				v = b.sum / float64(b.numN)
			}
			out = append(out, dtos.ReportData{Value: v, Time: b.start})
		}
	}

	sort.Slice(out, func(i, j int) bool { return out[i].Time > out[j].Time })
	return out
}

// Paginate 对聚合结果分页（桶分页），返回当前页与桶总数
func Paginate(buckets []dtos.ReportData, page, pageSize int) ([]dtos.ReportData, int) {
	total := len(buckets)
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}
	start := (page - 1) * pageSize
	if start >= total {
		return []dtos.ReportData{}, total
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	return buckets[start:end], total
}

// toFloat 判断值是否可参与数值聚合（bool 按 1/0 处理）
func toFloat(value interface{}) (float64, bool) {
	switch v := value.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int8:
		return float64(v), true
	case int16:
		return float64(v), true
	case int32:
		return float64(v), true
	case int64:
		return float64(v), true
	case uint:
		return float64(v), true
	case uint8:
		return float64(v), true
	case uint16:
		return float64(v), true
	case uint32:
		return float64(v), true
	case uint64:
		return float64(v), true
	case bool:
		if v {
			return 1, true
		}
		return 0, true
	default:
		return 0, false
	}
}
