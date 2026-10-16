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
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"iotserver/internal/pkg/errort"
	"iotserver/internal/pkg/httphelper"
)

// 画布看板
//
// 看板 = 一张画布，画布上是若干可拖拽/缩放的组件：文本、容器（装子网格）。
// 组件位置与嵌套关系都由平台自己保存与渲染，不再依赖外部查询引擎。
const (
	vizDefaultCols      = 12
	vizDefaultRowHeight = 40
	vizMaxWidgets       = 60
	// 子网格嵌套深度上限（顶层算第 1 层）：容器里再放容器就太挤了
	vizMaxNestDepth = 2

	// 组件类型
	vizWidgetText      = "text"
	vizWidgetContainer = "container"
	// vizWidgetChart 早期图表组件：已不再渲染，读旧记录时按它识别并给前端占位提示
	vizWidgetChart = "chart"
	// 数据类组件：绑定设备属性，配置存在 Config 里（渲染与配置表单由前端注册表负责）
	vizWidgetKpi   = "kpi"
	vizWidgetGauge = "gauge"
	vizWidgetLine  = "line"
	vizWidgetTable = "table"
	vizWidgetStats = "stats"
	// 文本组件正文长度上限
	vizTextMaxLen = 4000
	// 组件 config 序列化后的字节上限（避免把大 JSON 塞进看板文件）
	vizConfigMaxLen = 16 * 1024
	// 单个数据类组件最多绑定多少条数据序列
	vizMaxSeriesPerWidget = 20
	// 看板自动刷新间隔的下限 / 上限（秒）
	vizMinRefreshSeconds = 5
	vizMaxRefreshSeconds = 3600
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
	// Type 组件类型，取值见 vizWidgetRegistry。
	// 未知类型原样保留（新版本前端建的组件在旧后端上不会被改写成文本而丢内容）
	Type   string    `json:"type,omitempty"`
	Text   string    `json:"text,omitempty"`
	Layout vizLayout `json:"layout"`
	// Config 数据类组件的配置：每个类型自己的 schema，后端只负责补默认值与大小校验
	Config map[string]interface{} `json:"config,omitempty"`
	// RefreshInterval 组件自己的刷新间隔（秒）：0/缺省 = 跟随看板；
	// 只有数据类组件需要它（看板里一个"秒级"的数值卡和一个"小时级"的曲线可以并存）
	RefreshInterval int `json:"refreshInterval,omitempty"`
	// Children 容器组件的子网格内容（同一套 12 列坐标系，渲染时套一层子网格）
	Children []vizWidget `json:"children,omitempty"`
}

// vizWidgetSpec 后端对一种组件类型的登记：默认标题、默认配置、能否装子组件。
//
// 与 OpenRemote 的 WidgetManifest 同一个思路，但职责切分不同：
//   - 后端（这里）只管存储契约：类型登记、默认值合并、大小与结构校验；
//   - 前端 src/components/viz/widgets 的注册表管渲染与配置表单。
//
// 两边都不需要知道对方新增了什么类型：未知类型在保存时原样保留。
type vizWidgetSpec struct {
	DefaultTitle string
	Defaults     map[string]interface{}
	// HasChildren 容器型组件（装子网格）
	HasChildren bool
	// Data 数据类组件：绑定设备属性，必须有 Config
	Data bool
}

var vizWidgetRegistry = map[string]vizWidgetSpec{
	vizWidgetText:      {DefaultTitle: "文本"},
	vizWidgetContainer: {DefaultTitle: "容器", HasChildren: true},
	// 早期图表组件：保留登记只为识别旧记录，前端会渲染成占位提示
	vizWidgetChart: {DefaultTitle: "图表"},
	vizWidgetKpi: {
		DefaultTitle: "数值",
		Data:         true,
		Defaults: map[string]interface{}{
			"series": []interface{}{},
			// metrics 指标类数据源（平台/租户统计指标，见 vizmetricsapp）：
			// 与 series 并列，非空时按指标取数与渲染；旧看板没有这个键时走 series
			"metrics":  []interface{}{},
			"decimals": 0,
			"unit":     "",
			"compare":  "none",
		},
	},
	vizWidgetGauge: {
		DefaultTitle: "仪表盘",
		Data:         true,
		Defaults: map[string]interface{}{
			"series":   []interface{}{},
			"metrics":  []interface{}{},
			"min":      0,
			"max":      100,
			"decimals": 0,
			"unit":     "",
			"thresholds": []interface{}{
				[]interface{}{0, "#4caf50"},
				[]interface{}{75, "#ff9800"},
				[]interface{}{90, "#ef5350"},
			},
		},
	},
	vizWidgetLine: {
		DefaultTitle: "折线图",
		Data:         true,
		Defaults: map[string]interface{}{
			"series": []interface{}{},
			// metrics 时序指标（如每日消息量 report.daily）：与 series 二选一，
			// 取数时后端把它补进 series，前端按普通序列画
			"metrics":     []interface{}{},
			"timeRange":   "24h",
			"aggInterval": 300,
			"aggMethod":   "avg",
			"showLegend":  true,
			"smooth":      false,
			"fill":        false,
			"stacked":     false,
		},
	},
	vizWidgetTable: {
		DefaultTitle: "表格",
		Data:         true,
		Defaults: map[string]interface{}{
			"series":    []interface{}{},
			"metrics":   []interface{}{},
			"timeRange": "24h",
			"latest":    true,
			"pageSize":  10,
		},
	},
	// 统计卡：一卡多个指标块（标题 + 数值 + 子项），子项同样是指标 key
	vizWidgetStats: {
		DefaultTitle: "统计卡",
		Data:         true,
		Defaults: map[string]interface{}{
			"tiles": []interface{}{},
		},
	},
}

// vizSeriesRef 数据类组件绑定的一条数据序列：设备 + 属性
type vizSeriesRef struct {
	DeviceId string `json:"deviceId"`
	Code     string `json:"code"`
	Name     string `json:"name,omitempty"`
}

// widgetType 归一化后的组件类型：已知类型返回自身，未知类型原样保留，空值按文本处理
func (w vizWidget) widgetType() string {
	t := strings.ToLower(strings.TrimSpace(w.Type))
	if t == "" {
		return vizWidgetText
	}
	return t
}

// spec 该类型的登记信息；未知类型返回 false
func (w vizWidget) spec() (vizWidgetSpec, bool) {
	s, ok := vizWidgetRegistry[w.widgetType()]
	return s, ok
}

// normalizeConfig 合并默认配置并做基本校验。
// 只补缺失的键、不动已有值（与 OpenRemote 的 correctToConfigSpec 同思路），
// 所以给组件类型新增配置项时，存量看板读出来就自带默认值，不需要迁移脚本。
func (w *vizWidget) normalizeConfig(spec vizWidgetSpec) error {
	if !spec.Data {
		w.Config = nil
		return nil
	}
	if w.Config == nil {
		w.Config = make(map[string]interface{}, len(spec.Defaults))
	}
	for k, v := range spec.Defaults {
		if _, exists := w.Config[k]; !exists {
			w.Config[k] = v
		}
	}
	series, err := vizSeriesFromConfig(w.Config)
	if err != nil {
		return err
	}
	for i, s := range series {
		if strings.TrimSpace(s.DeviceId) == "" || strings.TrimSpace(s.Code) == "" {
			return fmt.Errorf("第 %d 条数据序列缺少设备或属性", i+1)
		}
	}
	raw, err := json.Marshal(w.Config)
	if err != nil {
		return fmt.Errorf("配置无法序列化：%s", err.Error())
	}
	if len(raw) > vizConfigMaxLen {
		return fmt.Errorf("配置过大（上限 %d 字节）", vizConfigMaxLen)
	}
	return nil
}

// vizSeriesFromConfig 取 config.series 里的序列列表。
// 存进 JSON 文件再读回来是 []interface{}，刚构造出来可能是 []vizSeriesRef，两种都支持。
func vizSeriesFromConfig(config map[string]interface{}) ([]vizSeriesRef, error) {
	if config == nil {
		return nil, nil
	}
	raw, ok := config["series"]
	if !ok || raw == nil {
		return nil, nil
	}
	if list, ok := raw.([]vizSeriesRef); ok {
		if len(list) > vizMaxSeriesPerWidget {
			return nil, fmt.Errorf("最多绑定 %d 条数据序列", vizMaxSeriesPerWidget)
		}
		return list, nil
	}
	buf, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("series 格式有误")
	}
	var list []vizSeriesRef
	if err := json.Unmarshal(buf, &list); err != nil {
		return nil, fmt.Errorf("series 格式有误")
	}
	if len(list) > vizMaxSeriesPerWidget {
		return nil, fmt.Errorf("最多绑定 %d 条数据序列", vizMaxSeriesPerWidget)
	}
	return list, nil
}

// isContainer 容器组件：不放自己的内容，只装子网格
func (w vizWidget) isContainer() bool {
	return w.widgetType() == vizWidgetContainer
}

type vizCanvas struct {
	Cols      int `json:"cols"`
	RowHeight int `json:"rowHeight"`
	// RefreshInterval 整页自动刷新间隔（秒）：0 = 不自动刷新。
	// 只影响展示（数据看板页 / 只读模式），编辑态强制不刷新。
	RefreshInterval int         `json:"refreshInterval,omitempty"`
	Widgets         []vizWidget `json:"widgets"`
}

// allWidgets 前序展开所有层级的组件（统计数量用）
func (cv *vizCanvas) allWidgets() []vizWidget {
	var out []vizWidget
	var walk func(list []vizWidget)
	walk = func(list []vizWidget) {
		for _, w := range list {
			out = append(out, w)
			walk(w.Children)
		}
	}
	walk(cv.Widgets)
	return out
}

// normalize 补默认值、收敛越界、校验每个组件的文本（保存前调用）
func (cv *vizCanvas) normalize(boardName string) error {
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
	// 自动刷新间隔：只允许 0（关）或 [5s, 1h]，避免把前端轮询设成 1 秒打爆数据层
	cv.RefreshInterval = clampVizRefresh(cv.RefreshInterval)

	seen := make(map[string]struct{}, vizMaxWidgets)
	total := 0

	// 逐层处理：子网格与顶层用同一套 12 列坐标系（渲染时子网格列数固定 12），
	// 所以位置收敛逻辑可以整棵树复用，只是 id 唯一 / 总数 / 深度是全局约束。
	var walk func(list []vizWidget, depth int) error
	walk = func(list []vizWidget, depth int) error {
		if len(list) > 0 && depth > vizMaxNestDepth {
			return fmt.Errorf("组件嵌套最多 %d 层", vizMaxNestDepth)
		}
		for i := range list {
			w := &list[i]
			total++
			if total > vizMaxWidgets {
				return fmt.Errorf("一块看板最多 %d 个组件", vizMaxWidgets)
			}
			if strings.TrimSpace(w.Id) == "" {
				w.Id = uuid.NewString()
			}
			if _, dup := seen[w.Id]; dup {
				return fmt.Errorf("组件 id 重复：%s", w.Id)
			}
			seen[w.Id] = struct{}{}

			w.Type = w.widgetType()
			spec, known := w.spec()
			switch {
			case w.Type == vizWidgetText:
				if len([]rune(strings.TrimSpace(w.Text))) > vizTextMaxLen {
					return fmt.Errorf("文本组件超过 %d 字", vizTextMaxLen)
				}
				w.Config = nil
			case !known:
				// 未知类型：数据原样保留，不做校验（可能是新版本前端建的组件）
			default:
				// 容器与数据类组件都不带正文
				w.Text = ""
				if err := w.normalizeConfig(spec); err != nil {
					return fmt.Errorf("组件「%s」%s", vizWidgetTitle(*w, boardName), err.Error())
				}
				if !spec.HasChildren && len(w.Children) > 0 {
					return fmt.Errorf("组件「%s」不能包含子组件", vizWidgetTitle(*w, boardName))
				}
			}

			// 组件级刷新间隔：只有已知的数据类组件会收敛；文本/容器清掉；未知类型原样保留
			switch {
			case !known:
				// 未知类型可能是新版本的数据组件，保留它的设置
			case spec.Data:
				w.RefreshInterval = clampVizRefresh(w.RefreshInterval)
			default:
				w.RefreshInterval = 0
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

			if len(w.Children) > 0 {
				if err := walk(w.Children, depth+1); err != nil {
					return err
				}
			}
		}
		return nil
	}
	return walk(cv.Widgets, 1)
}

// clampVizRefresh 刷新间隔归一化：0（含负值）= 不自动刷新 / 跟随看板，其余收敛到 [5s, 1h]
func clampVizRefresh(seconds int) int {
	if seconds <= 0 {
		return 0
	}
	if seconds < vizMinRefreshSeconds {
		return vizMinRefreshSeconds
	}
	if seconds > vizMaxRefreshSeconds {
		return vizMaxRefreshSeconds
	}
	return seconds
}

// vizWidgetTitle 组件默认标题：优先用类型登记里的默认标题
func vizWidgetTitle(w vizWidget, boardName string) string {
	if w.widgetType() == vizWidgetText {
		if strings.TrimSpace(boardName) != "" {
			return strings.TrimSpace(boardName) + "·文本"
		}
		return "文本"
	}
	if spec, ok := w.spec(); ok && spec.DefaultTitle != "" {
		return spec.DefaultTitle
	}
	// 未知类型直接用类型名，便于排查
	return w.widgetType()
}

// ===== 单个看板详情 =====

type vizBoardDetailResponse struct {
	Id        string     `json:"id"`
	Name      string     `json:"name"`
	Icon      string     `json:"icon,omitempty"`
	Canvas    *vizCanvas `json:"canvas,omitempty"`
	IsDefault bool       `json:"isDefault"`
	IsHome    bool       `json:"isHome"`
	UpdateAt  int64      `json:"updateAt"`
}

// VizBoardDetail 取单个看板（画布编辑器加载用）
// @Tags    可视化-看板管理
// @Summary 查询单个看板
// @Produce json
// @Success 200 {object} httphelper.CommonResponse
// @Router  /api/v1/viz/boards/{boardId} [get]
// @Security ApiKeyAuth
func (ctl *controller) VizBoardDetail(c *gin.Context) {
	boardId := strings.TrimSpace(c.Param("boardId"))
	if boardId == "" {
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultReqParamsError, fmt.Errorf("boardId 不能为空")), c.Writer, ctl.lc)
		return
	}
	for _, rec := range ctl.loadVizBoards(c) {
		if !rec.matchBoardId(boardId) {
			continue
		}
		httphelper.ResultSuccess(vizBoardDetailResponse{
			Id:        rec.Id,
			Name:      rec.Name,
			Icon:      rec.Icon,
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

// newVizBoardId 平台内看板 ID
func newVizBoardId() string {
	return "cv" + strings.ReplaceAll(uuid.NewString(), "-", "")[:20]
}

// touchVizBoard 更新时间戳
func touchVizBoard(rec *vizBoardRecord) {
	rec.UpdateAt = time.Now().UnixMilli()
}

// ===== 画布看板的登记 =====

// createCanvasBoard 新建画布看板
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
	if err := cv.normalize(name); err != nil {
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultReqParamsError, err), c.Writer, ctl.lc)
		return
	}

	rec := vizBoardRecord{Id: newVizBoardId(), Name: name, Icon: strings.TrimSpace(req.Icon), Canvas: cv}
	touchVizBoard(&rec)

	vizBoardsMutex.Lock()
	records := ctl.loadVizBoards(c)
	records = append(records, rec)
	saveErr := ctl.saveVizBoards(c, records)
	vizBoardsMutex.Unlock()
	if saveErr != nil {
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultSystemError, saveErr), c.Writer, ctl.lc)
		return
	}

	httphelper.ResultSuccess(gin.H{
		"id": rec.Id, "name": name, "widgets": len(cv.allWidgets()),
	}, c.Writer, ctl.lc)
}

// updateCanvasBoard 更新画布看板（名称 / 图标 / 布局整体覆盖，保留首页·整页标记）
func (ctl *controller) updateCanvasBoard(c *gin.Context, boardId string, req vizBoardSaveRequest) {
	vizBoardsMutex.Lock()
	records := ctl.loadVizBoards(c)
	index := -1
	for i := range records {
		if records[i].matchBoardId(boardId) {
			index = i
			break
		}
	}
	if index < 0 {
		vizBoardsMutex.Unlock()
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultSystemError,
			fmt.Errorf("看板不存在：%s", boardId)), c.Writer, ctl.lc)
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
	if err := cv.normalize(name); err != nil {
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
	saveErr := ctl.saveVizBoards(c, records)
	vizBoardsMutex.Unlock()
	if saveErr != nil {
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultSystemError, saveErr), c.Writer, ctl.lc)
		return
	}

	httphelper.ResultSuccess(gin.H{
		"id": updated.Id, "name": name,
		"widgets": len(cv.allWidgets()), "updateAt": updated.UpdateAt,
	}, c.Writer, ctl.lc)
}

// removeCanvasBoard 删除画布看板；返回 false 表示看板不存在
func (ctl *controller) removeCanvasBoard(c *gin.Context, boardId string) bool {
	vizBoardsMutex.Lock()
	records := ctl.loadVizBoards(c)
	index := -1
	for i := range records {
		if records[i].matchBoardId(boardId) {
			index = i
			break
		}
	}
	if index < 0 {
		vizBoardsMutex.Unlock()
		return false
	}

	next := make([]vizBoardRecord, 0, len(records)-1)
	next = append(next, records[:index]...)
	next = append(next, records[index+1:]...)
	saveErr := ctl.saveVizBoards(c, next)
	vizBoardsMutex.Unlock()
	if saveErr != nil {
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultSystemError, saveErr), c.Writer, ctl.lc)
		return true
	}
	httphelper.ResultSuccess(gin.H{"id": boardId, "deleted": true}, c.Writer, ctl.lc)
	return true
}
