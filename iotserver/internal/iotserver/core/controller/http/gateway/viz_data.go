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
	"context"
	"encoding/json"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"iotserver/internal/dtos"
	"iotserver/internal/iotserver/core/application/vizmetricsapp"
	"iotserver/internal/models"
	"iotserver/internal/pkg/constants"
	"iotserver/internal/pkg/errort"
	"iotserver/internal/pkg/httphelper"
	"iotserver/internal/pkg/tenant"
)

// 看板取数（设备属性）
//
// 看板上的数据类组件（kpi/gauge/line/table）都绑在"设备 + 属性"上，
// 前端把整块画布的组件一次性发过来（POST /api/v1/viz/device-query），后端并发查完一起返回。
//
// 为什么不做成"一个组件一个请求"：
//   - 画布上十几个组件会打出十几个 HTTP 请求，浏览器连接数被占满、首屏抖动；
//   - DuckDB 为单连接，集中限流比让前端各自为战更可控。
//
// 取数统一走持久化层的 SearchDeviceThingModelHistoryPropertyData，
// 所以数据层（duckdb）自动支持。
const (
	// 一次请求最多多少个组件
	vizDataMaxItems = 60
	// 一次请求最多多少条数据序列（设备 × 属性）
	vizDataMaxSeries = 120
	// 一次请求最多多少个指标（指标是聚合值，P0 清单本身只有十几个）
	vizDataMaxMetrics = 30
	// 单条序列默认 / 最大返回点数（上限与数据层的 maxPageSize 对齐：1000）
	vizDataDefaultPoints = 500
	vizDataMaxPoints     = 1000
	// 查询并发（数据层多为单连接，别开太大）
	vizDataConcurrency = 4
	// 单次请求总超时
	vizDataTimeout = 20 * time.Second
	// latest 模式的回看窗口。
	//
	// ⚠️ 数据层在「不传时间范围 + Last」时只回溯最近 30 分钟（见 duckdb/helpers.go 的 timeWindow、
	// tstorage/client.go 的半小时窗口），设备半小时没上报就会查空。所以这里显式给一个回看窗口。
	vizDataLatestLookback = 30 * 24 * time.Hour
)

/* ------------------------------ 请求 / 响应 ------------------------------ */

type vizDataSeriesRequest struct {
	DeviceId string `json:"deviceId"`
	Code     string `json:"code"`
	// Name 图例名；为空时后端用"设备名·属性名"补上
	Name string `json:"name,omitempty"`
}

type vizDataItemRequest struct {
	// Id 组件 id，原样回显，前端按它把结果挂回对应组件
	Id     string                 `json:"id"`
	Series []vizDataSeriesRequest `json:"series"`
	// Metrics 指标 key 列表（平台/租户统计指标，与 Series 并列，同一项可以二选一）。
	// 未登记的 key 直接忽略：不报错、也不出现在响应的 metrics 里。
	// 时序指标（kind=series，如每日消息量）不落 metrics，而是作为一条数据序列
	// 补进响应的 series —— 折线图这类按序列渲染的组件不用改就能接住。
	Metrics []string `json:"metrics,omitempty"`
	// Range 毫秒时间戳 [from, to]；给了就以它为准
	Range []int64 `json:"range,omitempty"`
	// TimeRange 1h / 24h / 7d / 30d / all；Range 缺省时用它算时间窗（默认 24h）
	TimeRange string `json:"timeRange,omitempty"`
	// AggInterval 聚合间隔（秒），0 = 不聚合（原始点）
	AggInterval int `json:"aggInterval,omitempty"`
	// AggMethod avg / max / min / sum / count / first / last
	AggMethod string `json:"aggMethod,omitempty"`
	// Limit 单条序列最多返回点数，默认 500，上限 1000
	Limit int `json:"limit,omitempty"`
	// Latest 只取最新值（kpi / 仪表盘 / 表格的"当前值"用）
	Latest bool `json:"latest,omitempty"`
	// LookbackSeconds latest 模式的回看窗口（秒），缺省用 vizDataLatestLookback
	LookbackSeconds int `json:"lookbackSeconds,omitempty"`
}

type vizDataRequest struct {
	Items []vizDataItemRequest `json:"items"`
}

type vizDataPoint struct {
	Time  int64       `json:"time"`
	Value interface{} `json:"value"`
}

type vizDataSeriesResponse struct {
	DeviceId   string         `json:"deviceId"`
	DeviceName string         `json:"deviceName,omitempty"`
	Code       string         `json:"code"`
	Name       string         `json:"name,omitempty"`
	Unit       string         `json:"unit,omitempty"`
	DataType   string         `json:"dataType,omitempty"`
	Total      int            `json:"total,omitempty"`
	Points     []vizDataPoint `json:"points"`
	Error      string         `json:"error,omitempty"`
}

type vizDataItemResponse struct {
	Id     string                  `json:"id"`
	Series []vizDataSeriesResponse `json:"series"`
	// Metrics 指标取值：值为 0 也照常返回；未登记 / 租户视图下的平台专属指标不出现
	Metrics []vizmetricsapp.Result `json:"metrics"`
	Error   string                 `json:"error,omitempty"`
}

type vizDataResponse struct {
	Results []vizDataItemResponse `json:"results"`
}

/* ------------------------------------ 接口 ------------------------------------ */

// VizDeviceQuery 看板批量取数（设备属性）
// @Tags    可视化-看板取数
// @Summary 按设备与属性批量查询数据（画布一次请求取全部组件）
// @Accept  json
// @Produce json
// @Success 200 {object} httphelper.CommonResponse
// @Router  /api/v1/viz/device-query [post]
// @Security ApiKeyAuth
func (ctl *controller) VizDeviceQuery(c *gin.Context) {
	var req vizDataRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultReqParamsError,
			errort.NewMessageErr("请求体格式有误：{{.p1}}", map[string]interface{}{"p1": err.Error()})), c.Writer, ctl.lc)
		return
	}
	if len(req.Items) == 0 {
		httphelper.ResultSuccess(vizDataResponse{Results: []vizDataItemResponse{}}, c.Writer, ctl.lc)
		return
	}
	if len(req.Items) > vizDataMaxItems {
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultReqParamsError,
			errort.NewMessageErr("一次最多查询 {{.p1}} 个组件", map[string]interface{}{"p1": vizDataMaxItems})), c.Writer, ctl.lc)
		return
	}
	total := 0
	metricsTotal := 0
	for _, item := range req.Items {
		total += len(item.Series)
		metricsTotal += len(item.Metrics)
	}
	if total > vizDataMaxSeries {
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultReqParamsError,
			errort.NewMessageErr("一次最多查询 {{.p1}} 条数据序列", map[string]interface{}{"p1": vizDataMaxSeries})), c.Writer, ctl.lc)
		return
	}
	if metricsTotal > vizDataMaxMetrics {
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultReqParamsError,
			errort.NewMessageErr("一次最多查询 {{.p1}} 个指标", map[string]interface{}{"p1": vizDataMaxMetrics})), c.Writer, ctl.lc)
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), vizDataTimeout)
	defer cancel()

	meta := newVizMetaCache(ctl)
	// 指标统计：同一请求内多个组件共用一次统计（ctx 决定租户/平台视图作用域）
	metrics := vizmetricsapp.NewStats(ctx, ctl.db(ctx))
	results := make([]vizDataItemResponse, len(req.Items))

	// 组件之间并发；组件内部串行（同一组件里几条曲线的数据层调用排队，避免把单连接打爆）
	sem := make(chan struct{}, vizDataConcurrency)
	var wg sync.WaitGroup
	for i := range req.Items {
		wg.Add(1)
		sem <- struct{}{}
		go func(idx int) {
			defer wg.Done()
			defer func() { <-sem }()
			results[idx] = ctl.runVizDataItem(ctx, req.Items[idx], meta, metrics)
		}(i)
	}
	wg.Wait()

	httphelper.ResultSuccess(vizDataResponse{Results: results}, c.Writer, ctl.lc)
}

// VizMetrics 看板可用指标清单
//
// 前端下拉直接用它，避免前后端各维护一份指标清单。
// 租户视图下不下发平台专属指标（避免选到永远没有值的项）；取数时同样会忽略。
// @Tags    可视化-看板取数
// @Summary 查询可绑定的平台/租户统计指标清单
// @Produce json
// @Success 200 {object} httphelper.CommonResponse
// @Router  /api/v1/viz/metrics [get]
// @Security ApiKeyAuth
func (ctl *controller) VizMetrics(c *gin.Context) {
	list := vizmetricsapp.List()
	if !tenant.IsPlatformScope(c) {
		filtered := make([]vizmetricsapp.Metric, 0, len(list))
		for _, m := range list {
			if !m.PlatformOnly {
				filtered = append(filtered, m)
			}
		}
		list = filtered
	}
	httphelper.ResultSuccess(gin.H{"list": list}, c.Writer, ctl.lc)
}

// runVizDataItem 处理一个组件里的所有数据序列与指标
func (ctl *controller) runVizDataItem(ctx context.Context, item vizDataItemRequest, meta *vizMetaCache, metrics *vizmetricsapp.Stats) vizDataItemResponse {
	resp := vizDataItemResponse{
		Id:      item.Id,
		Series:  make([]vizDataSeriesResponse, 0, len(item.Series)),
		Metrics: make([]vizmetricsapp.Result, 0, len(item.Metrics)),
	}

	// 时间窗先算出来：时序指标与设备属性共用它（瞬时指标与窗口无关）
	from, to := vizTimeWindow(item.Range, item.TimeRange)
	if item.Latest {
		lookback := vizDataLatestLookback
		if item.LookbackSeconds > 0 {
			lookback = time.Duration(item.LookbackSeconds) * time.Second
		}
		from = time.Now().Add(-lookback).UnixMilli()
		to = time.Now().UnixMilli()
	}
	limit := item.Limit
	if limit <= 0 {
		limit = vizDataDefaultPoints
	}
	if limit > vizDataMaxPoints {
		limit = vizDataMaxPoints
	}

	// 指标分两类：瞬时值进 metrics（一次统计，与时间窗无关）；
	// 时序指标（每日消息量这类）按时间窗取点，落到 series 里。
	instantKeys := make([]string, 0, len(item.Metrics))
	for _, key := range item.Metrics {
		def, ok := vizmetricsapp.Find(key)
		if !ok || def.Kind != vizmetricsapp.KindSeries {
			instantKeys = append(instantKeys, key)
			continue
		}
		series := vizDataSeriesResponse{
			Code:   def.Key,
			Name:   def.Label,
			Unit:   def.Unit,
			Points: []vizDataPoint{},
		}
		rows, err := vizmetricsapp.QuerySeries(ctl.db(ctx), def.Key, vizmetricsapp.SeriesQuery{
			From: from, To: to, AggInterval: item.AggInterval, MaxPoints: limit,
		})
		if err != nil {
			// 取不到不影响其它序列出数：错误挂在序列上
			ctl.lc.Warnf("viz: 时序指标 %s 取数失败: %s", def.Key, err.Error())
			series.Error = err.Error()
			resp.Series = append(resp.Series, series)
			continue
		}
		points := make([]vizDataPoint, 0, len(rows))
		for _, p := range rows {
			points = append(points, vizDataPoint{Time: p.Time, Value: p.Value})
		}
		series.Points = points
		series.Total = len(points)
		resp.Series = append(resp.Series, series)
	}

	if len(instantKeys) > 0 {
		list, err := metrics.Query(ctx, instantKeys)
		if err != nil {
			// 统计失败不影响设备属性出数：记日志，指标留空
			ctl.lc.Warnf("viz: 统计指标失败: %s", err.Error())
		} else {
			resp.Metrics = list
		}
	}

	for _, s := range item.Series {
		deviceId := strings.TrimSpace(s.DeviceId)
		code := strings.TrimSpace(s.Code)
		series := vizDataSeriesResponse{
			DeviceId: deviceId,
			Code:     code,
			Name:     strings.TrimSpace(s.Name),
			Points:   []vizDataPoint{},
		}
		if deviceId == "" || code == "" {
			series.Error = "缺少设备或属性"
			resp.Series = append(resp.Series, series)
			continue
		}

		// 设备名 / 属性中文名 / 单位：图例与仪表盘要显示，取不到不影响出数
		if info := meta.lookup(ctx, deviceId); info != nil {
			series.DeviceName = info.deviceName
			if p, ok := info.props[code]; ok {
				if series.Name == "" {
					series.Name = vizSeriesName(info.deviceName, p.Name, code)
				}
				series.Unit = vizPropertyUnit(p)
				series.DataType = string(p.TypeSpec.Type)
			}
		}
		if series.Name == "" {
			series.Name = code
		}

		req := dtos.ThingModelPropertyDataRequest{
			DeviceId: deviceId,
			Code:     code,
		}
		if item.Latest {
			// latest 只取一个点：不做聚合（聚合必须带时间范围），靠 Last 让数据层走倒序取第一条
			req.Range = []int64{from, to}
			req.Last = true
			req.Page = 1
			req.PageSize = 1
		} else {
			req.Range = []int64{from, to}
			req.AggInterval = item.AggInterval
			req.AggMethod = item.AggMethod
			req.Page = 1
			req.PageSize = limit
		}

		data, count, err := ctl.getPersistApp().SearchDeviceThingModelHistoryPropertyData(ctx, req)
		if err != nil {
			series.Error = err.Error()
			resp.Series = append(resp.Series, series)
			continue
		}
		series.Total = count
		series.Points = vizReportDataToPoints(data, item.Latest)
		resp.Series = append(resp.Series, series)
	}
	return resp
}

/* ---------------------------------- 小工具 ---------------------------------- */

// vizTimeWindow 解析时间窗：优先用显式 Range，其次 TimeRange 关键字，兜底近 24 小时
func vizTimeWindow(rng []int64, timeRange string) (int64, int64) {
	now := time.Now()
	if len(rng) == 2 && rng[0] > 0 {
		from, to := rng[0], rng[1]
		if from > to {
			from, to = to, from
		}
		return from, to
	}
	switch strings.ToLower(strings.TrimSpace(timeRange)) {
	case "1h":
		return now.Add(-time.Hour).UnixMilli(), now.UnixMilli()
	case "7d":
		return now.Add(-7 * 24 * time.Hour).UnixMilli(), now.UnixMilli()
	case "30d":
		return now.Add(-30 * 24 * time.Hour).UnixMilli(), now.UnixMilli()
	case "all":
		// 从 0 开始：数据层会按自己的默认（近 1 年）处理
		return 0, now.UnixMilli()
	default: // 24h
		return now.Add(-24 * time.Hour).UnixMilli(), now.UnixMilli()
	}
}

// vizReportDataToPoints 数据层返回的是 {value, time} 列表，且按时间**倒序**。
// 折线图要正序，所以这里翻一下；latest 只要第一条。
func vizReportDataToPoints(data interface{}, latest bool) []vizDataPoint {
	rows, ok := data.([]dtos.ReportData)
	if !ok {
		return []vizDataPoint{}
	}
	if latest {
		if len(rows) == 0 {
			return []vizDataPoint{}
		}
		return []vizDataPoint{{Time: rows[0].Time, Value: rows[0].Value}}
	}
	points := make([]vizDataPoint, 0, len(rows))
	for i := len(rows) - 1; i >= 0; i-- {
		points = append(points, vizDataPoint{Time: rows[i].Time, Value: rows[i].Value})
	}
	return points
}

// vizSeriesName 图例名：设备名 · 属性名
func vizSeriesName(deviceName, propName, code string) string {
	name := strings.TrimSpace(propName)
	if name == "" {
		name = code
	}
	if strings.TrimSpace(deviceName) == "" {
		return name
	}
	return deviceName + "·" + name
}

// vizPropertyUnit 取出属性的单位（只有 int/float 的 specs 里有 unit）
func vizPropertyUnit(p models.Properties) string {
	if p.TypeSpec.Type != constants.SpecsTypeInt && p.TypeSpec.Type != constants.SpecsTypeFloat {
		return ""
	}
	var spec models.TypeSpecIntOrFloat
	if err := json.Unmarshal([]byte(p.TypeSpec.Specs), &spec); err != nil {
		return ""
	}
	return strings.TrimSpace(spec.Unit)
}

/* ------------------------------ 设备/产品元信息缓存 ------------------------------ */

// vizDeviceMeta 一台设备的展示元信息：设备名 + 属性表（按 code 索引）
type vizDeviceMeta struct {
	deviceName string
	props      map[string]models.Properties
}

// vizMetaCache 同一次请求内复用设备/产品查询结果（画布上同一台设备常被多个组件引用）
type vizMetaCache struct {
	ctl   *controller
	mu    sync.Mutex
	cache map[string]*vizDeviceMeta
}

func newVizMetaCache(ctl *controller) *vizMetaCache {
	return &vizMetaCache{ctl: ctl, cache: make(map[string]*vizDeviceMeta)}
}

func (m *vizMetaCache) lookup(ctx context.Context, deviceId string) *vizDeviceMeta {
	m.mu.Lock()
	if hit, ok := m.cache[deviceId]; ok {
		m.mu.Unlock()
		return hit
	}
	m.mu.Unlock()

	meta := &vizDeviceMeta{props: make(map[string]models.Properties)}
	device, err := m.ctl.getDeviceApp().DeviceById(ctx, deviceId)
	if err != nil {
		m.ctl.lc.Warnf("viz: 查询设备 %s 失败: %s", deviceId, err.Error())
		return nil
	}
	meta.deviceName = device.Name
	if device.ProductId != "" {
		product, perr := m.ctl.getProductApp().ProductById(ctx, device.ProductId)
		if perr != nil {
			m.ctl.lc.Warnf("viz: 查询产品 %s 失败: %s", device.ProductId, perr.Error())
		} else if props, ok := product.Properties.([]models.Properties); ok {
			for _, p := range props {
				meta.props[p.Code] = p
			}
		}
	}

	m.mu.Lock()
	m.cache[deviceId] = meta
	m.mu.Unlock()
	return meta
}
