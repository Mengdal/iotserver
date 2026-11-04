package jobs

import "testing"

// 时区前缀缺少空格时不能 panic，应返回参数错误。
func TestParseTimezoneWithoutSpace(t *testing.T) {
	for _, spec := range []string{"TZ=UTC", "CRON_TZ=Asia/Shanghai"} {
		if _, err := NewParser(Second | Minute | Hour | Dom | Month | Dow | Descriptor).Parse(spec); err == nil {
			t.Fatalf("%q 应返回错误", spec)
		}
	}
}

func TestParseTimezoneWithSpace(t *testing.T) {
	schedule, err := NewParser(Minute | Hour | Dom | Month | Dow | Descriptor).Parse("TZ=UTC */5 * * * *")
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if schedule == nil || schedule.Location == nil || schedule.Location.String() != "UTC" {
		t.Fatalf("时区解析错误: %+v", schedule)
	}
}
