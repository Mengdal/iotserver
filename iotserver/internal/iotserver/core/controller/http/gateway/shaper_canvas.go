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
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"iotserver/internal/pkg/errort"
	"iotserver/internal/pkg/httphelper"
)

// 画布看板（kind=canvas）
//
// 参考 app.dash.builders 的形态：看板 = 一张画布，画布上是若干可拖拽/缩放的 widget 卡片，
// 每个卡片有自己的可视化配置（视图/维度/指标/图表类型/时间范围），图形与布局都由平台渲染，
// Shaper 只当查询引擎（SQL 由平台按配置生成后交给它执行）。
const (
	vizKindCanvas = "canvas"
	vizKindShaper = "shaper"

	vizDefaultCols      = 12
	vizDefaultRowHeight = 40
	vizMaxWidgets       = 60
	vizQueryConcurrency = 4

	// 组件类型
	vizWidgetChart = "chart"
	vizWidgetText  = "text"
	// 文本组件正文长度上限
	vizTextMaxLen = 4000
)

type vizLayout struct {
	X int `json:"x"`
	Y int `json:"y"`
	W int `json:"w"`
	H int `json:"h"`
}

type vizWidget struct {
	Id        string `json:"id"`
	Title     string `json:"title"`
	ShowTitle bool   `json:"showTitle"`
	// Type chart（默认，图表组件）| text（文本组件，不需要查询）
	Type   string    `json:"type,omitempty"`
	Text   string    `json:"text,omitempty"`
	Layout vizLayout `json:"layout"`
	// Spec 图表组件的可视化配置（文本组件为空）
	Spec vizBoardSpec `json:"spec"`
}

// widgetType 组件类型（缺省按图表处理，兼容已有记录）
func (w vizWidget) widgetType() string {
	switch strings.ToLower(strings.TrimSpace(w.Type)) {
	case vizWidgetText:
		return vizWidgetText
	}
	return vizWidgetChart
}

type vizCanvas struct {
	Cols      int         `json:"cols"`
	RowHeight int         `json:"rowHeight"`
	Widgets   []vizWidget `json:"widgets"`
}

// normalize 补默认值、收敛越界、校验每个 widget 的配置（保存前调用）
func (cv *vizCanvas) normalize(boardName string, validate func(vizBoardSpec) error) error {
	if cv.Cols <= 0 {
		cv.Cols = vizDefaultCols
	}
	if cv.Cols < 6 {
		cv.Cols = 6
	}
	if cv.Cols > 24 {
		cv.Cols = 24
	}
	if cv.RowHeight <= 0 {
		cv.RowHeight = vizDefaultRowHeight
	}
	if len(cv.Widgets) > vizMaxWidgets {
		return fmt.Errorf("一块看板最多 %d 个组件", vizMaxWidgets)
	}

	seen := make(map[string]struct{}, len(cv.Widgets))
	for i := range cv.Widgets {
		w := &cv.Widgets[i]
		if strings.TrimSpace(w.Id) == "" {
			w.Id = uuid.NewString()
		}
		if _, dup := seen[w.Id]; dup {
			return fmt.Errorf("组件 id 重复：%s", w.Id)
		}
		seen[w.Id] = struct{}{}

		w.Type = w.widgetType()
		if w.Type == vizWidgetText {
			// 文本组件不需要查询，只校验正文长度
			if len([]rune(strings.TrimSpace(w.Text))) > vizTextMaxLen {
				return fmt.Errorf("文本组件超过 %d 字", vizTextMaxLen)
			}
		} else if validate != nil {
			if err := validate(w.Spec); err != nil {
				return fmt.Errorf("组件「%s」配置有误：%s", vizWidgetTitle(*w, boardName), err.Error())
			}
		}
		if strings.TrimSpace(w.Title) == "" {
			w.Title = vizWidgetTitle(*w, boardName)
		}

		l := &w.Layout
		if l.W < 1 {
			l.W = 4
		}
		if l.W > cv.Cols {
			l.W = cv.Cols
		}
		if l.H < 2 {
			l.H = 6
		}
		if l.H > 40 {
			l.H = 40
		}
		if l.X < 0 {
			l.X = 0
		}
		if l.X+l.W > cv.Cols {
			l.X = cv.Cols - l.W
			if l.X < 0 {
				l.X = 0
				l.W = cv.Cols
			}
		}
		if l.Y < 0 {
			l.Y = 0
		}
	}
	return nil
}

// vizWidgetTitle 组件默认标题：优先用第一个指标的别名
func vizWidgetTitle(w vizWidget, boardName string) string {
	if w.widgetType() == vizWidgetText {
		return "文本"
	}
	for i, m := range w.Spec.Measures {
		if alias := strings.TrimSpace(m.Alias); alias != "" {
			return alias
		}
		return vizMeasureAlias(m, i)
	}
	if d := strings.TrimSpace(w.Spec.Dimension); d != "" {
		return d
	}
	if strings.TrimSpace(boardName) != "" {
		return boardName
	}
	return "图表"
}

// ===== 查询：按可视化配置取数（供画布上的 widget 出图） =====

type vizQueryRequest struct {
	Name string       `json:"name"`
	Spec vizBoardSpec `json:"spec"`
}

type vizQueryItem struct {
	Id   string       `json:"id"`
	Name string       `json:"name"`
	Spec vizBoardSpec `json:"spec"`
}

type vizQueryBatchRequest struct {
	Items []vizQueryItem `json:"items"`
}

type vizQueryResult struct {
	Id      string     `json:"id,omitempty"`
	Sql     string     `json:"sql"`
	Columns []string   `json:"columns"`
	Rows    [][]string `json:"rows"`
	// Formats 与 Columns 一一对应：int / float / percent / bytes / date / text
	// 前端按它把原始值渲染成"37.43"、"29.54 GB"这样可读的格式
	Formats []string `json:"formats,omitempty"`
	Error   string   `json:"error,omitempty"`
}

// runVizSpec 生成 SQL 并取数（复用 firstVizDataQuery 只跑第一条数据查询）
func (ctl *controller) runVizSpec(baseUrl, jwt, name string, spec vizBoardSpec) vizQueryResult {
	sql, err := ctl.buildVizSQL(name, spec)
	if err != nil {
		return vizQueryResult{Error: err.Error()}
	}
	first := firstVizDataQuery(sql)
	if first == "" {
		return vizQueryResult{Sql: sql, Error: "没有可执行的查询"}
	}
	columns, rows, err := ctl.shaperSqlQueryWithToken(baseUrl, jwt, first)
	if err != nil {
		return vizQueryResult{Sql: sql, Error: err.Error()}
	}
	if len(rows) > 500 {
		rows = rows[:500]
	}
	return vizQueryResult{
		Sql: sql, Columns: columns, Rows: rows,
		Formats: ctl.vizResultFormats(spec, columns),
	}
}

// ShaperVizQuery 按可视化配置取数（单组件）
// @Tags    可视化-看板管理
// @Summary 按可视化配置查询数据
// @Accept  json
// @Produce json
// @Success 200 {object} httphelper.CommonResponse
// @Router  /api/v1/viz/query [post]
// @Security ApiKeyAuth
func (ctl *controller) ShaperVizQuery(c *gin.Context) {
	baseUrl, _, ok := ctl.shaperBaseUrlOrFail(c)
	if !ok {
		return
	}
	var req vizQueryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultReqParamsError, err), c.Writer, ctl.lc)
		return
	}
	if err := ctl.validateVizSpec(req.Spec); err != nil {
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultReqParamsError, err), c.Writer, ctl.lc)
		return
	}
	jwt, err := ctl.shaperMgmtToken(baseUrl)
	if err != nil {
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultSystemError, err), c.Writer, ctl.lc)
		return
	}
	res := ctl.runVizSpec(baseUrl, jwt, req.Name, req.Spec)
	if res.Error != "" {
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultSystemError, fmt.Errorf("%s", res.Error)), c.Writer, ctl.lc)
		return
	}
	httphelper.ResultSuccess(gin.H{"sql": res.Sql, "columns": res.Columns, "rows": res.Rows}, c.Writer, ctl.lc)
}

// ShaperVizQueryBatch 按可视化配置批量取数（一个画布一次请求，并发上限 4）
// @Tags    可视化-看板管理
// @Summary 批量按可视化配置查询数据
// @Accept  json
// @Produce json
// @Success 200 {object} httphelper.CommonResponse
// @Router  /api/v1/viz/query/batch [post]
// @Security ApiKeyAuth
func (ctl *controller) ShaperVizQueryBatch(c *gin.Context) {
	baseUrl, _, ok := ctl.shaperBaseUrlOrFail(c)
	if !ok {
		return
	}
	var req vizQueryBatchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultReqParamsError, err), c.Writer, ctl.lc)
		return
	}
	if len(req.Items) == 0 {
		httphelper.ResultSuccess(gin.H{"results": []vizQueryResult{}}, c.Writer, ctl.lc)
		return
	}
	if len(req.Items) > vizMaxWidgets {
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultReqParamsError,
			fmt.Errorf("一次最多查询 %d 个组件", vizMaxWidgets)), c.Writer, ctl.lc)
		return
	}
	// 每个组件单独校验：坏的只让自己报错，不影响其它组件（文本组件没有查询，跳过）
	for _, item := range req.Items {
		if strings.TrimSpace(item.Spec.View) == "" && len(item.Spec.Measures) == 0 {
			continue
		}
		if err := ctl.validateVizSpec(item.Spec); err != nil {
			httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultReqParamsError,
				fmt.Errorf("组件配置有误：%s", err.Error())), c.Writer, ctl.lc)
			return
		}
	}

	jwt, err := ctl.shaperMgmtToken(baseUrl)
	if err != nil {
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultSystemError, err), c.Writer, ctl.lc)
		return
	}

	results := make([]vizQueryResult, len(req.Items))
	sem := make(chan struct{}, vizQueryConcurrency)
	var wg sync.WaitGroup
	for i, item := range req.Items {
		wg.Add(1)
		go func(index int, it vizQueryItem) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			// 文本组件不查数据
			if strings.TrimSpace(it.Spec.View) == "" && len(it.Spec.Measures) == 0 {
				results[index] = vizQueryResult{Id: it.Id}
				return
			}
			name := strings.TrimSpace(it.Name)
			if name == "" {
				name = "看板组件"
			}
			res := ctl.runVizSpec(baseUrl, jwt, name, it.Spec)
			res.Id = it.Id
			results[index] = res
		}(i, item)
	}
	wg.Wait()

	httphelper.ResultSuccess(gin.H{"results": results}, c.Writer, ctl.lc)
}

// ===== 单个看板详情（画布编辑器加载用） =====

type vizBoardDetailResponse struct {
	Id        string        `json:"id"`
	Kind      string        `json:"kind"`
	ShaperId  string        `json:"shaperId,omitempty"`
	Name      string        `json:"name"`
	Icon      string        `json:"icon,omitempty"`
	Sql       string        `json:"sql,omitempty"`
	Spec      *vizBoardSpec `json:"spec,omitempty"`
	Canvas    *vizCanvas    `json:"canvas,omitempty"`
	IsDefault bool          `json:"isDefault"`
	IsHome    bool          `json:"isHome"`
	UpdateAt  int64         `json:"updateAt"`
}

// ShaperBoardDetail 取单个看板（canvas 类型返回画布与各组件配置）
// @Tags    可视化-看板管理
// @Summary 查询单个看板
// @Produce json
// @Success 200 {object} httphelper.CommonResponse
// @Router  /api/v1/viz/boards/{boardId} [get]
// @Security ApiKeyAuth
func (ctl *controller) ShaperBoardDetail(c *gin.Context) {
	boardId := strings.TrimSpace(c.Param("boardId"))
	if boardId == "" {
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultReqParamsError, fmt.Errorf("boardId 不能为空")), c.Writer, ctl.lc)
		return
	}
	for _, rec := range ctl.loadVizBoards() {
		if rec.boardId() != boardId && rec.ShaperId != boardId {
			continue
		}
		httphelper.ResultSuccess(vizBoardDetailResponse{
			Id:        rec.boardId(),
			Kind:      rec.kind(),
			ShaperId:  rec.ShaperId,
			Name:      rec.Name,
			Icon:      rec.Icon,
			Sql:       rec.Sql,
			Spec:      rec.Spec,
			Canvas:    rec.Canvas,
			IsDefault: rec.IsDefault,
			IsHome:    rec.IsHome,
			UpdateAt:  rec.UpdateAt,
		}, c.Writer, ctl.lc)
		return
	}
	httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultSystemError,
		fmt.Errorf("看板不存在：%s", boardId)), c.Writer, ctl.lc)
}

// ===== 依据校验 =====

// sortVizBoards 最近更新的排前面
func sortVizBoards(records []vizBoardRecord) {
	sort.SliceStable(records, func(i, j int) bool {
		if records[i].UpdateAt != records[j].UpdateAt {
			return records[i].UpdateAt > records[j].UpdateAt
		}
		return records[i].Name < records[j].Name
	})
}

// newVizBoardId 平台内看板 ID（canvas 类型不依赖 Shaper）
func newVizBoardId() string {
	return "cv" + strings.ReplaceAll(uuid.NewString(), "-", "")[:20]
}

// touchVizBoard 更新时间戳
func touchVizBoard(rec *vizBoardRecord) {
	rec.UpdateAt = time.Now().UnixMilli()
}

// ===== 画布看板的登记（不走 Shaper 的看板对象） =====

// shaperBaseUrl 取服务端用的 Shaper 地址与 API Key（不依赖 gin.Context，便于内部调用）
func (ctl *controller) shaperBaseUrl() (string, string, error) {
	cfg := ctl.cfg.Shaper
	baseUrl := strings.TrimRight(ctl.shaperSetting(shaperBaseUrlEnv, cfg.BaseUrl), "/")
	apiKey := ctl.shaperSetting(shaperApiKeyEnv, cfg.ApiKey)
	if !cfg.Enabled || baseUrl == "" {
		return "", "", fmt.Errorf("Shaper 未配置（BaseUrl 为空）")
	}
	return baseUrl, apiKey, nil
}

type shaperApp struct {
	Id        string `json:"id"`
	Name      string `json:"name"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

// shaperApps 取 Shaper 里的看板列表（legacy 看板）；失败由调用方降级处理
func (ctl *controller) shaperApps() ([]shaperApp, error) {
	baseUrl, _, err := ctl.shaperBaseUrl()
	if err != nil {
		return nil, err
	}
	jwt, err := ctl.shaperMgmtToken(baseUrl)
	if err != nil {
		return nil, err
	}
	var out struct {
		Apps []shaperApp `json:"apps"`
	}
	if err := ctl.shaperRequest(baseUrl, http.MethodGet, "/api/apps?pageSize=200", jwt, nil, &out); err != nil {
		return nil, err
	}
	return out.Apps, nil
}

// createCanvasBoard 新建画布看板（只登记，不建 Shaper 对象）
func (ctl *controller) createCanvasBoard(c *gin.Context, req vizBoardSaveRequest) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultReqParamsError, fmt.Errorf("name 不能为空")), c.Writer, ctl.lc)
		return
	}
	cv := req.Canvas
	if cv == nil {
		cv = &vizCanvas{}
	}
	if err := cv.normalize(name, ctl.validateVizSpec); err != nil {
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultReqParamsError, err), c.Writer, ctl.lc)
		return
	}

	rec := vizBoardRecord{Id: newVizBoardId(), Kind: vizKindCanvas, Name: name, Icon: strings.TrimSpace(req.Icon), Canvas: cv}
	touchVizBoard(&rec)

	vizBoardsMutex.Lock()
	records := ctl.loadVizBoards()
	records = append(records, rec)
	saveErr := ctl.saveVizBoards(records)
	vizBoardsMutex.Unlock()
	if saveErr != nil {
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultSystemError, saveErr), c.Writer, ctl.lc)
		return
	}

	httphelper.ResultSuccess(gin.H{
		"id": rec.Id, "kind": vizKindCanvas, "name": name, "widgets": len(cv.Widgets),
	}, c.Writer, ctl.lc)
}

// updateCanvasBoard 更新画布看板（名称 / 布局 / 组件配置整体覆盖，保留首页·整页标记）
func (ctl *controller) updateCanvasBoard(c *gin.Context, boardId string, req vizBoardSaveRequest) {
	vizBoardsMutex.Lock()
	records := ctl.loadVizBoards()
	index := -1
	for i := range records {
		if records[i].matchBoardId(boardId) {
			index = i
			break
		}
	}
	if index < 0 || records[index].kind() != vizKindCanvas {
		vizBoardsMutex.Unlock()
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultSystemError,
			fmt.Errorf("画布看板不存在：%s", boardId)), c.Writer, ctl.lc)
		return
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = records[index].Name
	}
	cv := req.Canvas
	if cv == nil {
		cv = records[index].Canvas
	}
	if cv == nil {
		cv = &vizCanvas{}
	}
	if err := cv.normalize(name, ctl.validateVizSpec); err != nil {
		vizBoardsMutex.Unlock()
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultReqParamsError, err), c.Writer, ctl.lc)
		return
	}

	records[index].Name = name
	if strings.TrimSpace(req.Icon) != "" {
		records[index].Icon = strings.TrimSpace(req.Icon)
	}
	records[index].Canvas = cv
	touchVizBoard(&records[index])
	updated := records[index]
	saveErr := ctl.saveVizBoards(records)
	vizBoardsMutex.Unlock()
	if saveErr != nil {
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultSystemError, saveErr), c.Writer, ctl.lc)
		return
	}

	httphelper.ResultSuccess(gin.H{
		"id": updated.boardId(), "kind": vizKindCanvas, "name": name,
		"widgets": len(cv.Widgets), "updateAt": updated.UpdateAt,
	}, c.Writer, ctl.lc)
}

// removeCanvasBoard 删除画布看板；返回 true 表示已处理（canvas 类型）
func (ctl *controller) removeCanvasBoard(c *gin.Context, boardId string) bool {
	vizBoardsMutex.Lock()
	records := ctl.loadVizBoards()
	index := -1
	for i := range records {
		if records[i].matchBoardId(boardId) {
			index = i
			break
		}
	}
	if index < 0 || records[index].kind() != vizKindCanvas {
		vizBoardsMutex.Unlock()
		return false
	}

	next := make([]vizBoardRecord, 0, len(records)-1)
	next = append(next, records[:index]...)
	next = append(next, records[index+1:]...)
	saveErr := ctl.saveVizBoards(next)
	vizBoardsMutex.Unlock()
	if saveErr != nil {
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultSystemError, saveErr), c.Writer, ctl.lc)
		return true
	}
	httphelper.ResultSuccess(gin.H{"id": boardId, "deleted": true}, c.Writer, ctl.lc)
	return true
}
