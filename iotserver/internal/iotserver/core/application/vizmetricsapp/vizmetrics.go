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

// Package vizmetricsapp 看板「指标类数据源」（P0）。
//
// 背景：看板上的数据类组件（kpi/gauge/table）过去只能绑「设备 + 测点」，
// 「产品数 / 设备在线数 / 未处理告警」这类平台或租户维度的聚合值无法表达。
// 这里把这些聚合值登记成带 key 的指标，取数与设备属性同路
// （POST /api/v1/viz/device-query 的 items[].metrics）。
//
// 约定：
//   - 指标清单只在本文件定义（GET /api/v1/viz/metrics 直接返回），前后端不各写一份；
//   - 统计一律走调用方传入的 DBClient（调用方需已 WithCtx），因此 GORM 租户插件
//     自动生效：租户视图只算本租户的行，平台视图算全平台（与
//     infrastructure/{sqlite,mysql}/tenant_repo.go 的计数口径一致）；
//   - PlatformOnly 指标只在平台视图返回：租户视图直接省略该项，不报错；
//   - 未登记的 key 直接忽略，不报错、不返回；
//   - 值为 0 也照常返回（调用方不要因为 0 而省略）；
//   - 指标分两种 kind：瞬时值（缺省，走 Stats.Query）与时序值（KindSeries，
//     按天出点，走 QuerySeries，请求方把它当一条数据序列用）。
package vizmetricsapp

import (
	"context"
	"iotserver/internal/pkg/errort"
	"math"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"

	"iotserver/internal/dtos"
	interfaces "iotserver/internal/iotserver/core/interface"
	"iotserver/internal/models"
	"iotserver/internal/pkg/constants"
	"iotserver/internal/pkg/tenant"
)

// KindSeries 时序指标：值是"序列"而不是单个数字（按天或按采样桶出点）。
// 这类指标不进 Stats.Query（瞬时统计），由 QuerySeries 取点；请求方（看板取数）
// 把它落到响应的 series 里，折线图这类按序列渲染的组件就能直接接住。
const KindSeries = "series"

// Metric 指标元信息（GET /api/v1/viz/metrics 列表项，前端下拉用）
type Metric struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Unit  string `json:"unit"`
	// Precision 小数位（缺省 0）：计数类 0 位，百分比/容量 1 位；前端按它格式化
	Precision int `json:"precision,omitempty"`
	// Kind 指标类别：空 = 瞬时值；series = 时序值（见 KindSeries）
	Kind         string `json:"kind,omitempty"`
	PlatformOnly bool   `json:"platformOnly"`
}

// SeriesPoint 时序指标的一个点
type SeriesPoint struct {
	// Time 点的时间（毫秒时间戳）：按天口径是当天 0 点，系统类是按桶起点
	Time  int64   `json:"time"`
	Value float64 `json:"value"`
}

// Result 指标取值结果（device-query 响应的 metrics 列表项）
type Result struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	// Value 指标值：业务计数为整数，系统类指标（CPU/内存）是小数
	Value float64 `json:"value"`
	// Precision 小数位（缺省 0），与 Metric.Precision 一致
	Precision int    `json:"precision,omitempty"`
	Unit      string `json:"unit"`
}

// counts 一次统计出来的全部原始计数（各指标从中取值，避免每个 key 各查一次）
type counts struct {
	products       int64
	productsSelf   int64
	devices        int64
	devicesSelf    int64
	devicesOnline  int64
	devicesOffline int64
	devicesNever   int64
	gateways       int64
	gatewaysOnline int64
	gatewayOffline int64
	gatewayNever   int64
	alarmsPending  int64
	reportToday    int64
	users          int64
	tenants        int64
}

// metricDef 注册项：元信息 + 取值方式（value / instant / series 三选一）。
//   - value   业务计数类：从一次统计快照（counts）里取；
//   - instant 系统资源类：从最近一条系统采样里取（见 monitor/systemmonitor.go）；
//   - series  时序类：按时间窗取点（见 QuerySeries）。
type metricDef struct {
	Metric
	value   func(counts) int64
	instant func(sys *dtos.SystemMetrics) float64
	series  func(db interfaces.DBClient, q SeriesQuery) ([]SeriesPoint, error)
}

// SeriesQuery 时序指标取数参数
type SeriesQuery struct {
	// From / To 时间窗（毫秒时间戳）；From 缺省按各指标自己的兜底窗口
	From, To int64
	// AggInterval 聚合间隔（秒），0 = 按指标自己的采样粒度
	AggInterval int
	// MaxPoints 返回点数上限：窗口太大时自动加粗聚合桶，保证不超
	MaxPoints int
}

// registry P0 指标清单。顺序即前端下拉顺序。
//
// 计数口径：
//   - 本地/第三方产品（设备）= product.platform（device.platform）是否本地接入
//     （constants.IotPlatform_LocalIot，与首页「平台概述」的本地/第三方同口径）；
//   - 设备在线/离线/从未上线 = device.status（online/offline/inactive，见 constants.DeviceStatus）；
//   - 网关在线/离线/从未上线 = gateway.status（1/0/2，见 models.GatewayStatus*）；
//   - 未处理告警 = alert_list.status = 未处理；
//   - 今日上报量 = msg_gather 中 date = 今天的 count 之和（该表按天落一条汇总，
//     由每日任务写入，当天记录未生成时值为 0）；
//   - 每日消息量 = msg_gather 按天的 count 序列（时序指标，缺的天补 0，见 queryMsgGatherDaily）；
//   - 系统资源（system.*）= system_metrics 最近一条采样；趋势是窗口内按桶求平均
//     （保留期见 monitor/systemmonitor.go，默认 7 天）；
//   - tenant.total 与 system.* 为平台专属：租户视图返回空，用来验证视图隔离。
var registry = []metricDef{
	{Metric: Metric{Key: "product.total", Label: "产品总数"}, value: func(c counts) int64 { return c.products }},
	{Metric: Metric{Key: "product.self", Label: "本地产品"}, value: func(c counts) int64 { return c.productsSelf }},
	{Metric: Metric{Key: "product.other", Label: "第三方产品"}, value: func(c counts) int64 { return c.products - c.productsSelf }},
	{Metric: Metric{Key: "device.total", Label: "设备总数"}, value: func(c counts) int64 { return c.devices }},
	{Metric: Metric{Key: "device.self", Label: "本地设备"}, value: func(c counts) int64 { return c.devicesSelf }},
	{Metric: Metric{Key: "device.other", Label: "第三方设备"}, value: func(c counts) int64 { return c.devices - c.devicesSelf }},
	{Metric: Metric{Key: "device.online", Label: "在线设备"}, value: func(c counts) int64 { return c.devicesOnline }},
	{Metric: Metric{Key: "device.offline", Label: "离线设备"}, value: func(c counts) int64 { return c.devicesOffline }},
	{Metric: Metric{Key: "device.never", Label: "从未上线设备"}, value: func(c counts) int64 { return c.devicesNever }},
	{Metric: Metric{Key: "gateway.total", Label: "网关总数"}, value: func(c counts) int64 { return c.gateways }},
	{Metric: Metric{Key: "gateway.online", Label: "在线网关"}, value: func(c counts) int64 { return c.gatewaysOnline }},
	{Metric: Metric{Key: "gateway.offline", Label: "离线网关"}, value: func(c counts) int64 { return c.gatewayOffline }},
	{Metric: Metric{Key: "gateway.never", Label: "从未上线网关"}, value: func(c counts) int64 { return c.gatewayNever }},
	{Metric: Metric{Key: "alarm.pending", Label: "未处理告警"}, value: func(c counts) int64 { return c.alarmsPending }},
	{Metric: Metric{Key: "report.today", Label: "今日上报量"}, value: func(c counts) int64 { return c.reportToday }},
	{
		// 时序指标：每日消息量。数据源是 msg_gather（一天一条汇总，每日任务写入），
		// 首页「设备消息总数」面积图用的就是它。
		Metric: Metric{Key: "report.daily", Label: "每日消息量", Kind: KindSeries},
		series: queryMsgGatherDaily,
	},
	{Metric: Metric{Key: "user.total", Label: "用户总数"}, value: func(c counts) int64 { return c.users }},
	{
		Metric: Metric{Key: "tenant.total", Label: "租户总数", PlatformOnly: true},
		value:  func(c counts) int64 { return c.tenants },
	},
	// ── 平台系统资源（监控采集器每分钟采样一条，见 monitor/systemmonitor.go）──
	// 全部 PlatformOnly：系统指标是"这台服务自己"的运行情况，租户视图下不下发、取不到
	// （看板里绑这些指标的块/组件在租户视图下会自动隐藏，见前端 platformScope）。
	{
		Metric:  Metric{Key: "system.cpu.usage", Label: "CPU使用率", Unit: "%", Precision: 1, PlatformOnly: true},
		instant: func(sys *dtos.SystemMetrics) float64 { return sys.CpuUsedPercent },
	},
	{
		Metric:  Metric{Key: "system.cpu.load", Label: "系统负载率", Unit: "%", Precision: 1, PlatformOnly: true},
		instant: func(sys *dtos.SystemMetrics) float64 { return systemLoadPercent(sys) },
	},
	{
		Metric:  Metric{Key: "system.memory.used", Label: "内存占用", Unit: "GB", Precision: 1, PlatformOnly: true},
		instant: func(sys *dtos.SystemMetrics) float64 { return bytesToGB(sys.Memory.Used) },
	},
	{
		Metric:  Metric{Key: "system.memory.total", Label: "内存总量", Unit: "GB", Precision: 1, PlatformOnly: true},
		instant: func(sys *dtos.SystemMetrics) float64 { return bytesToGB(sys.Memory.Total) },
	},
	{
		Metric:  Metric{Key: "system.disk.used", Label: "磁盘占用", Unit: "GB", Precision: 1, PlatformOnly: true},
		instant: func(sys *dtos.SystemMetrics) float64 { return bytesToGB(sys.Disk.Used) },
	},
	{
		Metric:  Metric{Key: "system.disk.total", Label: "磁盘总量", Unit: "GB", Precision: 1, PlatformOnly: true},
		instant: func(sys *dtos.SystemMetrics) float64 { return bytesToGB(sys.Disk.Total) },
	},
	{
		// 趋势：按请求的聚合间隔分桶求平均（窗口太大时自动加粗桶，见 systemTrend）
		Metric: Metric{Key: "system.cpu.trend", Label: "CPU使用率趋势", Unit: "%", Precision: 1, Kind: KindSeries, PlatformOnly: true},
		series: systemTrend(func(sys dtos.SystemMetrics) float64 { return sys.CpuUsedPercent }),
	},
	{
		Metric: Metric{Key: "system.memory.trend", Label: "内存使用率趋势", Unit: "%", Precision: 1, Kind: KindSeries, PlatformOnly: true},
		series: systemTrend(func(sys dtos.SystemMetrics) float64 { return sys.Memory.UsedPercent }),
	},
}

// List 全部指标元信息（含平台专属项；视图过滤由调用方按 PlatformOnly 决定）
func List() []Metric {
	out := make([]Metric, 0, len(registry))
	for _, def := range registry {
		out = append(out, def.Metric)
	}
	return out
}

// Find 按 key 查指标元信息（未登记返回 false）
func Find(key string) (Metric, bool) {
	def, ok := findDef(key)
	if !ok {
		return Metric{}, false
	}
	return def.Metric, true
}

func findDef(key string) (metricDef, bool) {
	key = strings.TrimSpace(key)
	if key == "" {
		return metricDef{}, false
	}
	for _, def := range registry {
		if def.Key == key {
			return def, true
		}
	}
	return metricDef{}, false
}

/* ---------------------------------- 取数 ---------------------------------- */

// Stats 一次取数请求内的指标统计缓存。
//
// 同一块画布上多个组件往往要同一批指标（产品数/设备数…），这里只统计一次，
// 组件各取所需，避免每个组件都重复 count。
type Stats struct {
	db       interfaces.DBClient
	platform bool
	once     sync.Once
	counts   counts
	err      error
	// 系统类指标的最新采样：只有请求里带了 system.* 才会去查（同样只查一次）
	systemOnce sync.Once
	system     *dtos.SystemMetrics
	systemErr  error
}

// systemRow 最近一条系统采样；采集器还没写出数据时返回 (nil, nil)
func (s *Stats) systemRow() (*dtos.SystemMetrics, error) {
	s.systemOnce.Do(func() {
		s.system, s.systemErr = latestSystemMetrics(s.db)
	})
	return s.system, s.systemErr
}

// NewStats ctx 决定视图（平台/租户），db 需要是已 WithCtx 的客户端
func NewStats(ctx context.Context, db interfaces.DBClient) *Stats {
	return &Stats{db: db, platform: tenant.IsPlatformScope(ctx)}
}

// Query 按请求的 key 返回指标值。
//
//   - 未登记 / 空 key 直接忽略；时序指标（KindSeries）也不在这里出数（走 QuerySeries）；
//   - PlatformOnly 指标在租户视图下省略；
//   - 值为 0 照常返回；重复 key 只返回一次；
//   - 统计失败时返回错误，调用方按「取不到不影响出数」处理。
func (s *Stats) Query(ctx context.Context, keys []string) ([]Result, error) {
	if len(keys) == 0 {
		return nil, nil
	}
	// 先按登记表过滤/去重，再决定要准备哪份快照（计数 / 系统采样）
	defs := make([]metricDef, 0, len(keys))
	seen := make(map[string]struct{}, len(keys))
	needCounts := false
	for _, key := range keys {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		if _, dup := seen[key]; dup {
			continue
		}
		def, ok := findDef(key)
		if !ok || (def.PlatformOnly && !s.platform) {
			continue
		}
		if def.value == nil && def.instant == nil {
			// 时序指标不在这里出数（走 QuerySeries）
			continue
		}
		seen[key] = struct{}{}
		if def.value != nil {
			needCounts = true
		}
		defs = append(defs, def)
	}
	if needCounts {
		s.once.Do(func() {
			s.counts, s.err = collect(ctx, s.db, s.platform)
		})
		if s.err != nil {
			return nil, s.err
		}
	}
	out := make([]Result, 0, len(defs))
	for _, def := range defs {
		res := Result{Key: def.Key, Label: def.Label, Unit: def.Unit, Precision: def.Precision}
		switch {
		case def.value != nil:
			res.Value = float64(def.value(s.counts))
		case def.instant != nil:
			system, err := s.systemRow()
			if err != nil {
				return nil, err
			}
			if system == nil {
				// 采集器还没有数据：这一项不发（前端按"取不到"显示 --）
				continue
			}
			res.Value = def.instant(system)
		}
		out = append(out, res)
	}
	return out, nil
}

/* ------------------------------ 时序指标取数 ------------------------------ */

// 时序指标最多回溯天数：msg_gather 一天一行，90 天足够看趋势，
// 也兜住 timeRange=all 这种没有起点的窗口，避免把整表拉出来。
const dailySeriesMaxDays = 90

// QuerySeries 取时序指标的点序列。
//
// db 需是已 WithCtx 的客户端（租户过滤交给 GORM 租户插件）。
// 未登记或非时序指标返回错误；窗口/聚合参数见 SeriesQuery。
func QuerySeries(db interfaces.DBClient, key string, q SeriesQuery) ([]SeriesPoint, error) {
	def, ok := findDef(key)
	if !ok {
		return nil, errort.NewMessageErr("未登记的指标：{{.p1}}", map[string]interface{}{"p1": strings.TrimSpace(key)})
	}
	if def.series == nil {
		return nil, errort.NewMessageErr("指标 {{.p1}} 不是时序指标", map[string]interface{}{"p1": def.Key})
	}
	return def.series(db, q)
}

// queryMsgGatherDaily 每日消息量：msg_gather 一天一条汇总，缺的天补 0
// （与首页「设备消息总数」同口径：没记录 = 当天 0 条，而不是断点）。
//
// 按整天出点：一天的汇总没法按小时切，所以窗口只用来算"要几天"——
// 近 24h = 今天一天，近 7 天 = 含今天往前 7 天（不是 7×24h 覆盖到的 8 个自然日）。
// 窗口内的聚合参数（AggInterval/MaxPoints）对按天口径无意义，忽略。
func queryMsgGatherDaily(db interfaces.DBClient, q SeriesQuery) ([]SeriesPoint, error) {
	if db == nil {
		return nil, errort.NewMessageErr("数据库客户端为空", nil)
	}
	endTs := time.Now().UnixMilli()
	if q.To > 0 {
		endTs = q.To
	}
	days := dailySeriesMaxDays
	if q.From > 0 {
		if q.From >= endTs {
			days = 1
		} else {
			days = int(math.Ceil(float64(endTs-q.From) / float64(24*time.Hour/time.Millisecond)))
		}
	}
	if days < 1 {
		days = 1
	}
	if days > dailySeriesMaxDays {
		days = dailySeriesMaxDays
	}

	end := time.UnixMilli(endTs)
	end = time.Date(end.Year(), end.Month(), end.Day(), 0, 0, 0, 0, time.Local)
	start := end.AddDate(0, 0, -(days - 1))

	dates := make([]string, 0, days)
	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		dates = append(dates, d.Format("2006-01-02"))
	}

	rows, _, err := db.MsgGatherSearch(0, -1, dtos.MsgGatherSearchQueryRequest{Date: dates})
	if err != nil {
		return nil, err
	}
	byDate := make(map[string]int64, len(rows))
	for _, r := range rows {
		byDate[r.Date] += int64(r.Count)
	}
	out := make([]SeriesPoint, 0, len(dates))
	for i, date := range dates {
		out = append(out, SeriesPoint{
			Time:  start.AddDate(0, 0, i).UnixMilli(),
			Value: float64(byDate[date]),
		})
	}
	return out, nil
}

/* --------------------------- 系统资源指标取数 --------------------------- */

// latestSystemMetrics 最近一条系统采样。采集器每分钟写一条，往前找 10 分钟足够；
// 采集器还没出过数据（刚部署）时返回 (nil, nil)。
func latestSystemMetrics(db interfaces.DBClient) (*dtos.SystemMetrics, error) {
	if db == nil {
		return nil, errort.NewMessageErr("数据库客户端为空", nil)
	}
	now := time.Now().UnixMilli()
	rows, err := db.GetSystemMetrics(now-10*time.Minute.Milliseconds(), now)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	latest := &rows[0]
	for i := range rows {
		if rows[i].Timestamp > latest.Timestamp {
			latest = &rows[i]
		}
	}
	return latest, nil
}

// systemTrend 系统指标趋势：把窗口内的采样按桶求平均。
// 桶宽 = max(请求的聚合间隔, 窗口/点数上限)，保证点数不超（大窗口自动变粗）。
// 只输出有采样的桶：没数据 = 断点，而不是 0。
func systemTrend(pick func(dtos.SystemMetrics) float64) func(db interfaces.DBClient, q SeriesQuery) ([]SeriesPoint, error) {
	return func(db interfaces.DBClient, q SeriesQuery) ([]SeriesPoint, error) {
		if db == nil {
			return nil, errort.NewMessageErr("数据库客户端为空", nil)
		}
		to := q.To
		if to <= 0 {
			to = time.Now().UnixMilli()
		}
		from := q.From
		if from <= 0 || from >= to {
			from = to - time.Hour.Milliseconds() // 兜底近 1 小时
		}
		bucket := q.AggInterval
		if bucket <= 0 {
			bucket = 60 // 采集粒度：1 分钟
		}
		if q.MaxPoints > 0 {
			if minBucket := int(math.Ceil(float64(to-from) / float64(q.MaxPoints) / 1000)); minBucket > bucket {
				bucket = minBucket
			}
		}

		rows, err := db.GetSystemMetrics(from, to)
		if err != nil {
			return nil, err
		}
		sums := make(map[int64]float64, len(rows))
		counts := make(map[int64]int, len(rows))
		for _, r := range rows {
			if r.Timestamp < from || r.Timestamp > to {
				continue
			}
			key := r.Timestamp / (int64(bucket) * 1000)
			sums[key] += pick(r)
			counts[key]++
		}
		keys := make([]int64, 0, len(sums))
		for k := range sums {
			keys = append(keys, k)
		}
		sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
		out := make([]SeriesPoint, 0, len(keys))
		for _, k := range keys {
			out = append(out, SeriesPoint{
				Time:  k * int64(bucket) * 1000,
				Value: sums[k] / float64(counts[k]),
			})
		}
		return out, nil
	}
}

// bytesToGB 字节 → GB（保留 1 位小数的量级，具体小数位由 Metric.Precision 控制）
func bytesToGB(bytes uint64) float64 {
	return float64(bytes) / 1024 / 1024 / 1024
}

// systemLoadPercent 系统负载率：1 分钟平均负载 / 核数 × 100
// （负载 1.0 ≈ 单核跑满；多核机器按核数归一）
func systemLoadPercent(sys *dtos.SystemMetrics) float64 {
	cores := runtime.NumCPU()
	if cores <= 0 {
		return 0
	}
	return sys.CpuAvg / float64(cores) * 100
}

// collect 统计全部 P0 指标。db 是已 WithCtx 的客户端，租户插件按作用域过滤。
func collect(_ context.Context, db interfaces.DBClient, platform bool) (counts, error) {
	var c counts
	if db == nil {
		return c, errort.NewMessageErr("数据库客户端为空", nil)
	}
	gdb := db.GetDBInstance()
	if gdb == nil {
		return c, errort.NewMessageErr("数据库连接为空", nil)
	}

	var err error
	productTable := (&models.Product{}).TableName()
	if c.products, err = countTable(gdb, productTable, ""); err != nil {
		return c, err
	}
	// 「本地」= platform 是本地接入（与首页「本地/第三方」同口径，见 homepageapp）
	if c.productsSelf, err = countTable(gdb, productTable, "platform = ?", string(constants.IotPlatform_LocalIot)); err != nil {
		return c, err
	}
	deviceTable := (&models.Device{}).TableName()
	if c.devices, err = countTable(gdb, deviceTable, ""); err != nil {
		return c, err
	}
	if c.devicesSelf, err = countTable(gdb, deviceTable, "platform = ?", string(constants.IotPlatform_LocalIot)); err != nil {
		return c, err
	}
	if c.devicesOnline, err = countTable(gdb, deviceTable, "status = ?", string(constants.DeviceStatusOnline)); err != nil {
		return c, err
	}
	if c.devicesOffline, err = countTable(gdb, deviceTable, "status = ?", string(constants.DeviceStatusOffline)); err != nil {
		return c, err
	}
	if c.devicesNever, err = countTable(gdb, deviceTable, "status = ?", string(constants.DeviceStatusUnActive)); err != nil {
		return c, err
	}

	gatewayTable := (&models.Gateway{}).TableName()
	if c.gateways, err = countTable(gdb, gatewayTable, ""); err != nil {
		return c, err
	}
	if c.gatewaysOnline, err = countTable(gdb, gatewayTable, "status = ?", models.GatewayStatusOnline); err != nil {
		return c, err
	}
	if c.gatewayOffline, err = countTable(gdb, gatewayTable, "status = ?", models.GatewayStatusOffline); err != nil {
		return c, err
	}
	if c.gatewayNever, err = countTable(gdb, gatewayTable, "status = ?", models.GatewayStatusNever); err != nil {
		return c, err
	}

	if c.alarmsPending, err = countTable(gdb, (&models.AlertList{}).TableName(),
		"status = ?", string(constants.Untreated)); err != nil {
		return c, err
	}
	if c.reportToday, err = sumTable(gdb, (&models.MsgGather{}).TableName(), "`count`",
		"`date` = ?", time.Now().Format("2006-01-02")); err != nil {
		return c, err
	}
	if c.users, err = countTable(gdb, (&models.User{}).TableName(), ""); err != nil {
		return c, err
	}

	// 租户表不在租户隔离表清单里（平台级共享），只能在平台视图统计，
	// 否则租户视图会拿到全平台租户数（该指标也标记了 PlatformOnly，双重保护）。
	if platform {
		if c.tenants, err = countTable(gdb, models.TableTenant, ""); err != nil {
			return c, err
		}
	}
	return c, nil
}

func countTable(db *gorm.DB, table, where string, args ...interface{}) (int64, error) {
	var n int64
	q := db.Table(table)
	if where != "" {
		q = q.Where(where, args...)
	}
	if err := q.Count(&n).Error; err != nil {
		return 0, errort.NewMessageErr("统计 {{.p1}} 失败：{{.p2}}", map[string]interface{}{"p1": table, "p2": err})
	}
	return n, nil
}

func sumTable(db *gorm.DB, table, column, where string, args ...interface{}) (int64, error) {
	var row struct{ Total int64 }
	q := db.Table(table).Select("COALESCE(SUM(" + column + "), 0) AS total")
	if where != "" {
		q = q.Where(where, args...)
	}
	if err := q.Scan(&row).Error; err != nil {
		return 0, errort.NewMessageErr("统计 {{.p1}} 失败：{{.p2}}", map[string]interface{}{"p1": table, "p2": err})
	}
	return row.Total, nil
}
