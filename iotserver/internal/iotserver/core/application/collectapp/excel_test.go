package collectapp

import "testing"

func TestParseCollectPointRowsBulk(t *testing.T) {
	rows := [][]string{
		{"通道", "设备名称", "设备ID", "标签ID", "地址", "数据类型", "读写", "倍率", "偏移", "存储", "使能"},
		{"C1", "dev1", "17156557", "oil_temp", "40001", "float", "R", "0.1", "2", "是", "是"},
		{"C1", "dev1", "17156557", "oil_pressure", "40002", "int16", "RW", "", "", "否", "否"},
		{"C2", "D2", "", "water_temp", "COM1", "int16", "", "", "", "", ""},
	}
	items, errs, err := parseCollectPointRowsBulk(rows)
	if err != nil {
		t.Fatalf("parse bulk: %v", err)
	}
	if len(errs) != 0 {
		t.Fatalf("unexpected row errors: %+v", errs)
	}
	if len(items) != 3 {
		t.Fatalf("want 3 rows, got %d", len(items))
	}
	first := items[0]
	if first.deviceId != 17156557 || first.deviceName != "dev1" {
		t.Fatalf("unexpected device fields: %+v", first)
	}
	if first.point.TagId != "oil_temp" || first.point.Multiplier != 0.1 || first.point.Offset != 2 || !first.point.Enabled {
		t.Fatalf("unexpected point: %+v", first.point)
	}
	second := items[1]
	if second.point.Multiplier != 1 || second.point.Offset != 0 || second.point.Storage || !second.point.Enabled {
		t.Fatalf("blank scale/storage should use defaults: %+v", second.point)
	}
	third := items[2]
	if third.deviceId != 0 || third.deviceName != "D2" || third.point.AccessMode != "R" {
		t.Fatalf("unexpected third row: %+v", third)
	}
}

func TestParseCollectPointRowsBulkRowErrors(t *testing.T) {
	rows := [][]string{
		{"通道", "设备名称", "设备ID", "标签ID", "地址", "数据类型"},
		{"C1", "", "", "p1", "40001", "int16"},
		{"C1", "dev1", "abc", "p2", "40002", "int16"},
		{"C1", "dev1", "17156557", "p3", "", "int16"},
		{"C1", "dev1", "17156557", "p4", "40004", "int16"},
	}
	items, errs, err := parseCollectPointRowsBulk(rows)
	if err != nil {
		t.Fatalf("parse bulk: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("want 1 valid row, got %d", len(items))
	}
	if len(errs) != 3 {
		t.Fatalf("want 3 row errors, got %+v", errs)
	}
	if errs[0].Row != 2 || errs[1].Row != 3 || errs[2].Row != 4 {
		t.Fatalf("unexpected error rows: %+v", errs)
	}
}

func TestParseCollectPointRowsBulkMissingDeviceColumn(t *testing.T) {
	rows := [][]string{
		{"标签ID", "地址", "数据类型"},
		{"p1", "40001", "int16"},
	}
	if _, _, err := parseCollectPointRowsBulk(rows); err == nil {
		t.Fatal("want error for missing device column")
	}
}

func TestParseCollectPointRowsBulkCombinedScale(t *testing.T) {
	rows := [][]string{
		{"设备ID", "标签ID", "地址", "数据类型", "倍率/偏移", "存储", "使能"},
		{"17156557", "p1", "40001", "float", "0.1/2", "是", "否"},
	}
	items, errs, err := parseCollectPointRowsBulk(rows)
	if err != nil {
		t.Fatalf("parse bulk: %v", err)
	}
	if len(errs) != 0 || len(items) != 1 {
		t.Fatalf("unexpected result: items=%+v errs=%+v", items, errs)
	}
	if items[0].point.Multiplier != 0.1 || items[0].point.Offset != 2 {
		t.Fatalf("unexpected scale: %+v", items[0].point)
	}
}
