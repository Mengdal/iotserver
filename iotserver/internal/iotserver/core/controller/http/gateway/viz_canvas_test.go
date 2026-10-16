//
// Copyright (c) 2024 IOTServer
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//

package gateway

import (
	"testing"
	"time"

	"iotserver/internal/dtos"
)

/* ------------------------- 组件配置：默认值合并与校验 ------------------------- */

func TestNormalizeMergesWidgetDefaults(t *testing.T) {
	cv := &vizCanvas{Widgets: []vizWidget{
		{Id: "k1", Type: vizWidgetKpi, Layout: vizLayout{W: 3, H: 4}},
		{Id: "g1", Type: vizWidgetGauge, Layout: vizLayout{W: 3, H: 6}},
	}}
	if err := cv.normalize("看板A"); err != nil {
		t.Fatalf("normalize 不该报错: %v", err)
	}
	kpi := cv.Widgets[0]
	if kpi.Title != "数值" {
		t.Errorf("kpi 默认标题应为「数值」，实际 %q", kpi.Title)
	}
	if kpi.Config["decimals"] != 0 {
		t.Errorf("kpi 应补上 decimals=0，实际 %v", kpi.Config["decimals"])
	}
	if _, ok := kpi.Config["series"]; !ok {
		t.Errorf("kpi 应补上空 series")
	}
	gauge := cv.Widgets[1]
	if gauge.Title != "仪表盘" {
		t.Errorf("gauge 默认标题应为「仪表盘」，实际 %q", gauge.Title)
	}
	if gauge.Config["max"] != 100 {
		t.Errorf("gauge 应补上 max=100，实际 %v", gauge.Config["max"])
	}
}

// 已有配置项不被默认值覆盖（= OpenRemote 的 correctToConfigSpec 语义）
func TestNormalizeKeepsExistingConfigValues(t *testing.T) {
	cv := &vizCanvas{Widgets: []vizWidget{{
		Id: "k1", Type: vizWidgetKpi, Layout: vizLayout{W: 3, H: 4},
		Config: map[string]interface{}{"decimals": 2, "unit": "℃"},
	}}}
	if err := cv.normalize("看板A"); err != nil {
		t.Fatalf("normalize 不该报错: %v", err)
	}
	cfg := cv.Widgets[0].Config
	if cfg["decimals"] != 2 {
		t.Errorf("decimals 应保持 2，实际 %v", cfg["decimals"])
	}
	if cfg["unit"] != "℃" {
		t.Errorf("unit 应保持 ℃，实际 %v", cfg["unit"])
	}
	// 缺的键仍然补默认值
	if _, ok := cfg["compare"]; !ok {
		t.Errorf("缺失的 compare 应被补上默认值")
	}
}

func TestNormalizeRejectsIncompleteSeries(t *testing.T) {
	cv := &vizCanvas{Widgets: []vizWidget{{
		Id: "k1", Type: vizWidgetKpi, Layout: vizLayout{W: 3, H: 4},
		Config: map[string]interface{}{
			"series": []interface{}{map[string]interface{}{"deviceId": "d1"}},
		},
	}}}
	if err := cv.normalize("看板A"); err == nil {
		t.Fatal("缺少属性 code 的序列应当被拒绝")
	}
}

func TestNormalizeAcceptsCompleteSeries(t *testing.T) {
	cv := &vizCanvas{Widgets: []vizWidget{{
		Id: "l1", Type: vizWidgetLine, Layout: vizLayout{W: 8, H: 8},
		Config: map[string]interface{}{
			"series": []interface{}{map[string]interface{}{"deviceId": "d1", "code": "temperature"}},
		},
	}}}
	if err := cv.normalize("看板A"); err != nil {
		t.Fatalf("完整序列不该被拒绝: %v", err)
	}
	series, err := vizSeriesFromConfig(cv.Widgets[0].Config)
	if err != nil || len(series) != 1 || series[0].Code != "temperature" {
		t.Fatalf("序列解析结果不对: %+v, err=%v", series, err)
	}
}

// 未知类型原样保留：新版本前端建的组件在旧后端上不能被改写或清空
func TestNormalizeKeepsUnknownWidgetType(t *testing.T) {
	cv := &vizCanvas{Widgets: []vizWidget{{
		Id: "x1", Type: "FutureWidget", Layout: vizLayout{W: 4, H: 4},
		Config: map[string]interface{}{"anything": "keep-me"},
	}}}
	if err := cv.normalize("看板A"); err != nil {
		t.Fatalf("未知类型不该导致保存失败: %v", err)
	}
	w := cv.Widgets[0]
	if w.Type != "futurewidget" {
		t.Errorf("类型应被规范化保留，实际 %q", w.Type)
	}
	if w.Config["anything"] != "keep-me" {
		t.Errorf("未知类型的 config 不该被清空: %v", w.Config)
	}
}

func TestNormalizeRejectsChildrenOnDataWidget(t *testing.T) {
	cv := &vizCanvas{Widgets: []vizWidget{{
		Id: "k1", Type: vizWidgetKpi, Layout: vizLayout{W: 3, H: 4},
		Children: []vizWidget{{Id: "c1", Type: vizWidgetText, Layout: vizLayout{W: 2, H: 2}}},
	}}}
	if err := cv.normalize("看板A"); err == nil {
		t.Fatal("数据类组件不应允许带子组件")
	}
}

func TestNormalizeClampsRefreshInterval(t *testing.T) {
	cv := &vizCanvas{RefreshInterval: 1}
	if err := cv.normalize("看板A"); err != nil {
		t.Fatalf("normalize 不该报错: %v", err)
	}
	if cv.RefreshInterval != vizMinRefreshSeconds {
		t.Errorf("刷新间隔应被抬到下限 %d，实际 %d", vizMinRefreshSeconds, cv.RefreshInterval)
	}

	cv2 := &vizCanvas{RefreshInterval: 99999}
	if err := cv2.normalize("看板A"); err != nil {
		t.Fatalf("normalize 不该报错: %v", err)
	}
	if cv2.RefreshInterval != vizMaxRefreshSeconds {
		t.Errorf("刷新间隔应被压到上限 %d，实际 %d", vizMaxRefreshSeconds, cv2.RefreshInterval)
	}
}

// 组件级刷新间隔：0 = 跟随看板；非数据类组件的间隔会被清掉
func TestNormalizeClampsWidgetRefreshInterval(t *testing.T) {
	cv := &vizCanvas{Widgets: []vizWidget{
		{Id: "k1", Type: vizWidgetKpi, RefreshInterval: 1, Layout: vizLayout{W: 3, H: 4}},
		{Id: "k2", Type: vizWidgetKpi, RefreshInterval: 99999, Layout: vizLayout{W: 3, H: 4}},
		{Id: "t1", Type: vizWidgetText, RefreshInterval: 30, Layout: vizLayout{W: 3, H: 4}},
		{Id: "k3", Type: vizWidgetKpi, Layout: vizLayout{W: 3, H: 4}},
	}}
	if err := cv.normalize("看板A"); err != nil {
		t.Fatalf("normalize 不该报错: %v", err)
	}
	if cv.Widgets[0].RefreshInterval != vizMinRefreshSeconds {
		t.Errorf("组件的刷新间隔应被抬到下限，实际 %d", cv.Widgets[0].RefreshInterval)
	}
	if cv.Widgets[1].RefreshInterval != vizMaxRefreshSeconds {
		t.Errorf("组件的刷新间隔应被压到上限，实际 %d", cv.Widgets[1].RefreshInterval)
	}
	if cv.Widgets[2].RefreshInterval != 0 {
		t.Errorf("文本组件不该带刷新间隔，实际 %d", cv.Widgets[2].RefreshInterval)
	}
	if cv.Widgets[3].RefreshInterval != 0 {
		t.Errorf("缺省应表示跟随看板（0），实际 %d", cv.Widgets[3].RefreshInterval)
	}
}

/* ------------------------------- 取数参数处理 ------------------------------- */

func TestVizTimeWindow(t *testing.T) {
	// 显式范围优先，且自动纠正倒序
	from, to := vizTimeWindow([]int64{2000, 1000}, "1h")
	if from != 1000 || to != 2000 {
		t.Errorf("显式范围应原样（并纠正顺序），实际 [%d,%d]", from, to)
	}

	now := time.Now().UnixMilli()
	for _, tc := range []struct{ key string; span int64 }{
		{"1h", 3600 * 1000},
		{"24h", 24 * 3600 * 1000},
		{"7d", 7 * 24 * 3600 * 1000},
		{"30d", 30 * 24 * 3600 * 1000},
	} {
		start, end := vizTimeWindow(nil, tc.key)
		if end-start != tc.span {
			t.Errorf("%s 的时间窗应为 %d ms，实际 %d ms", tc.key, tc.span, end-start)
		}
		if end < now-2000 || end > now+2000 {
			t.Errorf("%s 的结束时间应接近当前时间", tc.key)
		}
	}

	// 缺省按 24h
	start, end := vizTimeWindow(nil, "")
	if end-start != 24*3600*1000 {
		t.Errorf("缺省应为 24h，实际跨度 %d ms", end-start)
	}

	// all：从 0 开始
	start, _ = vizTimeWindow(nil, "all")
	if start != 0 {
		t.Errorf("all 应从 0 开始，实际 %d", start)
	}
}

// 数据层按时间倒序返回，折线图需要正序；latest 只取第一条
func TestVizReportDataToPoints(t *testing.T) {
	rows := []dtos.ReportData{
		{Time: 300, Value: 3.0},
		{Time: 200, Value: 2.0},
		{Time: 100, Value: 1.0},
	}
	asc := vizReportDataToPoints(rows, false)
	if len(asc) != 3 || asc[0].Time != 100 || asc[2].Time != 300 {
		t.Fatalf("历史点应翻转为正序，实际 %+v", asc)
	}

	latest := vizReportDataToPoints(rows, true)
	if len(latest) != 1 || latest[0].Time != 300 {
		t.Fatalf("latest 应只取第一条（最新），实际 %+v", latest)
	}

	// 类型不符时不 panic，返回空
	if got := vizReportDataToPoints("not-a-row-list", false); len(got) != 0 {
		t.Fatalf("类型不符时应返回空，实际 %+v", got)
	}
}

func TestVizSeriesFromConfigRejectsTooMany(t *testing.T) {
	list := make([]interface{}, 0, vizMaxSeriesPerWidget+1)
	for i := 0; i <= vizMaxSeriesPerWidget; i++ {
		list = append(list, map[string]interface{}{"deviceId": "d", "code": "c"})
	}
	if _, err := vizSeriesFromConfig(map[string]interface{}{"series": list}); err == nil {
		t.Fatal("超过上限的序列数应当报错")
	}
}

func TestVizSeriesNameAndUnit(t *testing.T) {
	if got := vizSeriesName("1号机", "温度", "temp"); got != "1号机·温度" {
		t.Errorf("图例名拼接不对: %q", got)
	}
	if got := vizSeriesName("", "", "temp"); got != "temp" {
		t.Errorf("没有名称时应回退 code: %q", got)
	}
	if got := vizSeriesName("1号机", "", "temp"); got != "1号机·temp" {
		t.Errorf("属性无中文名时应回退 code: %q", got)
	}
}
