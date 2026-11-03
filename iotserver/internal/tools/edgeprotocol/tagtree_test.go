package edgeprotocol

import "testing"

// 取自真实网关（260112801）的 scadaGetTagsTree 回执，裁剪到最小可复现结构：
// 分组节点 → 通道 → 设备 → 测点；另外两个分组自己就是设备（deviceCode）。
const sampleTagTree = `{
  "status": true,
  "requestId": "sync-1",
  "data": [
    {"name": "I/O点", "children": [
      {"name": "HAVC", "channelCode": "HAVC", "children": [
        {"name": "THmeter", "description": "温湿度仪", "children": [
          {"name": "AirRoomTemp1", "id": "THmeter.AirRoomTemp1", "description": "空调房温度1"},
          {"name": "Switch1", "id": "THmeter.Switch1", "description": "开关1"}
        ]}
      ]},
      {"name": "Channel1", "channelCode": "Channel1", "children": [
        {"name": "Device1", "children": [
          {"name": "tag0001", "id": "Device1.tag0001"}
        ]}
      ]}
    ]},
    {"name": "用户点", "deviceCode": "user", "children": [
      {"name": "tag0001", "id": "user.tag0001", "description": "K1"}
    ]},
    {"name": "系统点", "deviceCode": "system", "children": [
      {"name": "CPU_USAGE", "id": "system.CPU_USAGE", "description": "CPU使用率(%)"}
    ]}
  ]
}`

func TestParseTagTreeGroupsByDevicePrefix(t *testing.T) {
	tags, err := ParseTagTree([]byte(sampleTagTree))
	if err != nil {
		t.Fatalf("ParseTagTree: %v", err)
	}
	byDevice := map[string][]Tag{}
	for _, tag := range tags {
		byDevice[tag.DeviceCode] = append(byDevice[tag.DeviceCode], tag)
	}

	// 通道名「I/O点」「HAVC」不能作为设备标识，否则下发控制会被网关拒绝
	for _, wrong := range []string{"I/O点", "HAVC", "用户点", "系统点"} {
		if _, ok := byDevice[wrong]; ok {
			t.Errorf("设备分组不应包含通道/分组名 %q", wrong)
		}
	}
	if got := len(byDevice["THmeter"]); got != 2 {
		t.Errorf("THmeter 测点数 = %d, want 2", got)
	}
	if desc := byDevice["THmeter"][0].DeviceDescription; desc != "温湿度仪" {
		t.Errorf("THmeter 设备描述 = %q, want 温湿度仪", desc)
	}
	if got := byDevice["Device1"]; len(got) != 1 || got[0].TagCode != "tag0001" {
		t.Errorf("Device1 解析结果 = %+v", got)
	}
	if got := byDevice["user"]; len(got) != 1 || got[0].TagCode != "tag0001" || got[0].Description != "K1" {
		t.Errorf("user 解析结果 = %+v", got)
	}
	if got := byDevice["system"]; len(got) != 1 || got[0].TagCode != "CPU_USAGE" {
		t.Errorf("system 解析结果 = %+v", got)
	}
}

func TestParseTagTreeAcceptsBareArray(t *testing.T) {
	tags, err := ParseTagTree([]byte(`[{"name":"D1","children":[{"name":"t1","id":"D1.t1"}]}]`))
	if err != nil {
		t.Fatalf("ParseTagTree: %v", err)
	}
	if len(tags) != 1 || tags[0].DeviceCode != "D1" || tags[0].TagCode != "t1" {
		t.Fatalf("解析结果 = %+v", tags)
	}
}

func TestParseTagTreeRejectsGarbage(t *testing.T) {
	if _, err := ParseTagTree([]byte(`not-json`)); err == nil {
		t.Fatal("非法报文应当报错")
	}
}
