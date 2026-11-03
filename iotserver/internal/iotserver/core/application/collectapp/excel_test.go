package collectapp

import "testing"

func TestParseCollectPointRowsSampleFormat(t *testing.T) {
	rows := [][]string{
		{"标签ID", "通道", "地址", "数据类型", "读写", "倍率/偏移", "存储", "使能"},
		{"D1.a", "C1", "slave=1;addr=400001", "float", "R", "0.1/2", "是", "是"},
		{"D1.b", "C1", "slave=1;addr=400002", "int16", "RW", "1/0", "否", "否"},
	}
	points, err := parseCollectPointRows(rows)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(points) != 2 {
		t.Fatalf("want 2 points, got %d", len(points))
	}
	first := points[0]
	if first.TagId != "D1.a" || first.Multiplier != 0.1 || first.Offset != 2 || !first.Storage || !first.Enabled {
		t.Fatalf("unexpected first point: %+v", first)
	}
	second := points[1]
	if second.AccessMode != "RW" || second.Storage || second.Enabled {
		t.Fatalf("unexpected second point: %+v", second)
	}
	if second.Sort != 1 {
		t.Fatalf("want sort 1, got %d", second.Sort)
	}
}

func TestParseCollectPointRowsSeparateScaleColumns(t *testing.T) {
	rows := [][]string{
		{"tagId", "address", "dataType", "读写", "倍率", "偏移", "存储", "启用"},
		{"temp", "40001", "int16", "", "10", "0.5", "", "false"},
	}
	points, err := parseCollectPointRows(rows)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(points) != 1 {
		t.Fatalf("want 1 point, got %d", len(points))
	}
	point := points[0]
	if point.AccessMode != "R" || point.Multiplier != 10 || point.Offset != 0.5 {
		t.Fatalf("unexpected point: %+v", point)
	}
	if !point.Storage {
		t.Fatalf("blank storage should default to true")
	}
	if point.Enabled {
		t.Fatalf("enabled=false should be parsed")
	}
}

func TestParseCollectPointRowsSkipsBlankRows(t *testing.T) {
	rows := [][]string{
		{"标签ID", "地址", "数据类型"},
		{"", "", ""},
		{"   ", "", ""},
		{"p1", "40001", "int16"},
	}
	points, err := parseCollectPointRows(rows)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(points) != 1 || points[0].TagId != "p1" {
		t.Fatalf("unexpected points: %+v", points)
	}
}

func TestParseCollectPointRowsMissingRequiredHeader(t *testing.T) {
	rows := [][]string{
		{"标签ID", "数据类型"},
		{"p1", "int16"},
	}
	if _, err := parseCollectPointRows(rows); err == nil {
		t.Fatal("want error for missing address column")
	}
}

func TestParseCollectPointRowsInvalidScale(t *testing.T) {
	rows := [][]string{
		{"标签ID", "地址", "数据类型", "倍率/偏移"},
		{"p1", "40001", "int16", "abc/0"},
	}
	if _, err := parseCollectPointRows(rows); err == nil {
		t.Fatal("want error for invalid scale")
	}
}

func TestParseCollectPointRowsEmpty(t *testing.T) {
	points, err := parseCollectPointRows(nil)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if points == nil || len(points) != 0 {
		t.Fatalf("want empty non-nil slice, got %#v", points)
	}
}
