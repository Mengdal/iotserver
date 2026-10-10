package aggregate

import (
	"math"
	"testing"

	"iotserver/internal/dtos"
)

const minute = 60 // 秒

func rows(pairs ...interface{}) []dtos.ReportData {
	out := make([]dtos.ReportData, 0, len(pairs)/2)
	for i := 0; i+1 < len(pairs); i += 2 {
		out = append(out, dtos.ReportData{Time: int64(pairs[i].(int)), Value: pairs[i+1]})
	}
	return out
}

func TestBucketizeNumericMethods(t *testing.T) {
	// 两个桶：桶 0（ts 1000/3000/59000 → 对齐后都不是 0，改用 60s 秒对齐易读的值）
	// 用 interval=60s=60000ms，桶起点为 0 与 60000
	data := rows(
		1000, 10.0,
		2000, 20.0,
		3000, 30.0,
		61000, 100.0,
		62000, 200.0,
	)

	tests := []struct {
		method string
		want   map[int64]float64
	}{
		{MethodAvg, map[int64]float64{0: 20, 60000: 150}},
		{MethodSum, map[int64]float64{0: 60, 60000: 300}},
		{MethodMin, map[int64]float64{0: 10, 60000: 100}},
		{MethodMax, map[int64]float64{0: 30, 60000: 200}},
		{MethodCount, map[int64]float64{0: 3, 60000: 2}},
	}

	for _, tt := range tests {
		got := Bucketize(data, minute, tt.method)
		if len(got) != len(tt.want) {
			t.Fatalf("%s: 桶数 = %d, want %d", tt.method, len(got), len(tt.want))
		}
		for _, item := range got {
			want, ok := tt.want[item.Time]
			if !ok {
				t.Fatalf("%s: 出现意外桶 %d", tt.method, item.Time)
			}
			v, _ := item.Value.(float64)
			if math.Abs(v-want) > 1e-9 {
				t.Fatalf("%s: 桶 %d 值 = %v, want %v", tt.method, item.Time, v, want)
			}
		}
	}
}

func TestBucketizeSortsDesc(t *testing.T) {
	data := rows(1000, 1.0, 61000, 2.0, 121000, 3.0)
	got := Bucketize(data, minute, MethodAvg)
	if len(got) != 3 {
		t.Fatalf("桶数 = %d, want 3", len(got))
	}
	if !(got[0].Time > got[1].Time && got[1].Time > got[2].Time) {
		t.Fatalf("结果未按时间降序: %+v", got)
	}
}

func TestBucketizeAlignment(t *testing.T) {
	// 间隔 60 秒：1000/60000/119999 应落在 0 / 60000 / 60000
	got := Bucketize(rows(1000, 1.0, 60000, 2.0, 119999, 3.0), minute, MethodCount)
	want := map[int64]float64{0: 1, 60000: 2}
	if len(got) != len(want) {
		t.Fatalf("桶数 = %d, want %d (%+v)", len(got), len(want), got)
	}
	for _, item := range got {
		v, _ := item.Value.(float64)
		if want[item.Time] != v {
			t.Fatalf("桶 %d = %v, want %v", item.Time, v, want[item.Time])
		}
	}
}

func TestBucketizeTextMethods(t *testing.T) {
	data := []dtos.ReportData{
		{Time: 5000, Value: "auto"},
		{Time: 1000, Value: "off"},
		{Time: 9000, Value: "manual"},
		{Time: 65000, Value: "idle"},
	}
	// 文本值：count 可用，first/last 保留原始字符串
	got := Bucketize(data, minute, MethodCount)
	if len(got) != 2 {
		t.Fatalf("count 桶数 = %d, want 2", len(got))
	}
	for _, item := range got {
		v, _ := item.Value.(float64)
		switch item.Time {
		case 0:
			if v != 3 {
				t.Fatalf("桶 0 count = %v, want 3", v)
			}
		case 60000:
			if v != 1 {
				t.Fatalf("桶 60000 count = %v, want 1", v)
			}
		}
	}

	first := Bucketize(data, minute, MethodFirst)
	for _, item := range first {
		if item.Time == 0 {
			if item.Value != "off" {
				t.Fatalf("桶 0 first = %v, want off（时间最小那行）", item.Value)
			}
		}
		if item.Time == 60000 {
			if item.Value != "idle" {
				t.Fatalf("桶 60000 first = %v, want idle", item.Value)
			}
		}
	}

	last := Bucketize(data, minute, MethodLast)
	for _, item := range last {
		if item.Time == 0 {
			if item.Value != "manual" {
				t.Fatalf("桶 0 last = %v, want manual（时间最大那行）", item.Value)
			}
		}
	}
}

func TestBucketizeSkipsBucketsWithoutNumeric(t *testing.T) {
	// 桶 0 只有文本，桶 60000 有数值 → 数值方法只应输出桶 60000
	data := []dtos.ReportData{
		{Time: 1000, Value: "text-only"},
		{Time: 61000, Value: 42.0},
	}
	got := Bucketize(data, minute, MethodAvg)
	if len(got) != 1 {
		t.Fatalf("avg 桶数 = %d, want 1（无数值行的桶应跳过）", len(got))
	}
	if got[0].Time != 60000 {
		t.Fatalf("avg 桶 = %d, want 60000", got[0].Time)
	}
}

func TestBucketizeBoolAndInt(t *testing.T) {
	data := []dtos.ReportData{
		{Time: 1000, Value: true},
		{Time: 2000, Value: false},
		{Time: 3000, Value: 2},
	}
	got := Bucketize(data, minute, MethodSum)
	if len(got) != 1 {
		t.Fatalf("桶数 = %d, want 1", len(got))
	}
	v, _ := got[0].Value.(float64)
	if v != 3 { // true=1 + false=0 + 2
		t.Fatalf("sum = %v, want 3（bool 按 1/0 处理）", v)
	}
}

func TestBucketizeEmptyAndNoInterval(t *testing.T) {
	if got := Bucketize(nil, minute, MethodAvg); len(got) != 0 {
		t.Fatalf("空输入应返回空, got %+v", got)
	}
	data := rows(1000, 1.0)
	if got := Bucketize(data, 0, MethodAvg); len(got) != 1 {
		t.Fatalf("interval<=0 应原样返回, got %+v", got)
	}
}

func TestValidate(t *testing.T) {
	// 不聚合：不做任何校验
	if _, _, err := Validate(0, "whatever", nil); err != nil {
		t.Fatalf("不聚合不应校验: %v", err)
	}
	// 聚合但没给时间范围
	if _, _, err := Validate(60, MethodAvg, nil); err == nil {
		t.Fatal("缺时间范围应报错")
	}
	// 缺省方法 → avg
	interval, method, err := Validate(60, "", []int64{0, 600000})
	if err != nil || method != MethodAvg || interval != 60 {
		t.Fatalf("归一化失败: interval=%d method=%s err=%v", interval, method, err)
	}
	// 非法方法
	if _, _, err := Validate(60, "median", []int64{0, 600000}); err == nil {
		t.Fatal("非法方法应报错")
	}
	// 桶数超限：30 秒 × 30 天 = 86400 个桶
	span := int64(30 * 24 * 3600 * 1000)
	if _, _, err := Validate(30, MethodAvg, []int64{0, span}); err == nil {
		t.Fatal("桶数超限应报错")
	}
	// 边界内
	if _, _, err := Validate(60, MethodMax, []int64{0, 600000}); err != nil {
		t.Fatalf("正常参数不应报错: %v", err)
	}
	// 区间逆序也能处理
	if _, _, err := Validate(60, MethodAvg, []int64{600000, 0}); err != nil {
		t.Fatalf("逆序区间应可处理: %v", err)
	}
}

func TestPaginate(t *testing.T) {
	data := []dtos.ReportData{{Time: 300}, {Time: 200}, {Time: 100}}
	page, total := Paginate(data, 1, 2)
	if total != 3 || len(page) != 2 {
		t.Fatalf("第一页: total=%d len=%d, want 3/2", total, len(page))
	}
	page, total = Paginate(data, 2, 2)
	if total != 3 || len(page) != 1 {
		t.Fatalf("第二页: total=%d len=%d, want 3/1", total, len(page))
	}
	page, total = Paginate(data, 9, 2)
	if total != 3 || len(page) != 0 {
		t.Fatalf("越界页: total=%d len=%d, want 3/0", total, len(page))
	}
}
