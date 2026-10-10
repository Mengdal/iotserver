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
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"iotserver/internal/pkg/errort"
	"iotserver/internal/pkg/httphelper"
)

// 看板管理（Shaper 看板 CRUD）
//
// Shaper 的看板接口只认真实管理 JWT：API Key（哪怕带 dashboard:read）换来的 token 只能渲染看板，
// 建/删会返回 401；本地模式下调 POST /api/auth/token（不带任何凭据）即可拿到管理 JWT。
// 另外 GET /api/dashboards/{id} 不返回 SQL 原文，所以看板的 SQL 副本由平台自己存一份
// （Shaper.BoardsFile，默认 .shaper-boards.json，不进仓库），用于「编辑」时回填。
type vizBoardRecord struct {
	// Id 平台内看板 ID：canvas 类型必填；legacy（shaper）记录为空，用 ShaperId 兼容
	Id string `json:"id,omitempty"`
	// Kind canvas（平台渲染的画布）| 空/shaper（Shaper DSL 看板，走 embed）
	Kind     string `json:"kind,omitempty"`
	ShaperId string `json:"shaperId,omitempty"`
	Name     string `json:"name"`
	// Icon 看板图标名（前端图标选择器给的值，列表里显示）
	Icon string `json:"icon,omitempty"`
	// Sql 由「可视化配置」生成（也可由老接口直接传入），保留它便于排查与展示
	Sql string `json:"sql,omitempty"`
	// Spec 单看板可视化配置（legacy/shaper 类型用）
	Spec *vizBoardSpec `json:"spec,omitempty"`
	// Canvas 画布看板（kind=canvas）：布局 + 每个 widget 的配置
	Canvas    *vizCanvas `json:"canvas,omitempty"`
	IsDefault bool       `json:"isDefault"`
	IsHome    bool       `json:"isHome"`
	UpdateAt  int64      `json:"updateAt"`
}

// kind 看板类型（旧记录缺省视为 shaper）
func (r vizBoardRecord) kind() string {
	if strings.EqualFold(strings.TrimSpace(r.Kind), vizKindCanvas) {
		return vizKindCanvas
	}
	return vizKindShaper
}

// boardId 平台内唯一标识（canvas 用 Id，legacy 用 ShaperId）
func (r vizBoardRecord) boardId() string {
	if strings.TrimSpace(r.Id) != "" {
		return r.Id
	}
	return r.ShaperId
}

// matchBoardId 是否匹配前端传来的看板 ID（Id 或 ShaperId 都认，便于前后兼容）
func (r vizBoardRecord) matchBoardId(id string) bool {
	id = strings.TrimSpace(id)
	return id != "" && (r.Id == id || r.ShaperId == id)
}

// ===== 可视化看板配置：用户在界面上选的东西，SQL 由平台生成 =====

type vizMeasure struct {
	// Column: 列名；"*" 表示计数
	Column string `json:"column"`
	// Agg: count / sum / avg / max / min
	Agg string `json:"agg"`
	// Alias: 展示名（也是 SQL 里的列别名）
	Alias string `json:"alias"`
}

type vizBoardSpec struct {
	// View: 数据视图，如 v_login_log / v_system_metrics / v_asset_count
	View string `json:"view"`
	// Chart: value（数值卡）/ line / bar / pie / table
	Chart string `json:"chart"`
	// Dimension: 维度列（X 轴）；数值卡可为空
	Dimension string `json:"dimension"`
	// Measures: 指标（聚合列）
	Measures []vizMeasure `json:"measures"`
	// TimeColumn / TimeRange: 时间过滤（TimeRange: 1h / 24h / 7d / 30d / all）
	TimeColumn string `json:"timeColumn"`
	TimeRange  string `json:"timeRange"`
	// SortDesc: 是否按第一个指标倒序（排行榜）
	SortDesc bool `json:"sortDesc"`
	// Limit: 最多几个分组/几行
	Limit int `json:"limit"`
	// Section: 段落标题（默认用看板名）
	Section string `json:"section"`
}

type vizBoardFile struct {
	Boards []vizBoardRecord `json:"boards"`
}

// vizBoardsMutex 串行化登记文件的读写（管理接口并发很低，简单加锁即可）
var vizBoardsMutex sync.Mutex

// shaperMgmtToken 取管理 JWT（本地模式无凭据即可；开启鉴权的部署会失败并返回错误）
func (ctl *controller) shaperMgmtToken(baseUrl string) (string, error) {
	body, _ := json.Marshal(map[string]interface{}{})
	req, err := http.NewRequest(http.MethodPost, baseUrl+"/api/auth/token", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	raw, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("shaper auth returned %s: %s", res.Status, strings.TrimSpace(string(raw)))
	}
	var out struct {
		Jwt string `json:"jwt"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", err
	}
	if out.Jwt == "" {
		return "", fmt.Errorf("shaper returned empty management jwt")
	}
	return out.Jwt, nil
}

// shaperRequest 带管理 JWT 调 Shaper 的 HTTP 接口
func (ctl *controller) shaperRequest(baseUrl, method, path, jwt string, payload interface{}, out interface{}) error {
	var reader io.Reader
	if payload != nil {
		body, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequest(method, baseUrl+path, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+jwt)

	client := &http.Client{Timeout: 20 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	raw, _ := io.ReadAll(io.LimitReader(res.Body, 4<<20))
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("shaper %s %s returned %s: %s", method, path, res.Status, strings.TrimSpace(string(raw)))
	}
	if out != nil && len(raw) > 0 {
		if err := json.Unmarshal(raw, out); err != nil {
			return err
		}
	}
	return nil
}

// boardsFilePath 登记文件路径（相对后端工作目录）
func (ctl *controller) boardsFilePath() string {
	path := strings.TrimSpace(ctl.cfg.Shaper.BoardsFile)
	if path == "" {
		path = ".shaper-boards.json"
	}
	return path
}

// loadVizBoards 读登记表
func (ctl *controller) loadVizBoards() []vizBoardRecord {
	raw, err := os.ReadFile(ctl.boardsFilePath())
	if err != nil {
		return nil
	}
	var f vizBoardFile
	if err := json.Unmarshal(raw, &f); err != nil {
		ctl.lc.Warn(fmt.Sprintf("parse %s failed: %s", ctl.boardsFilePath(), err.Error()))
		return nil
	}
	return f.Boards
}

// saveVizBoards 写登记表
func (ctl *controller) saveVizBoards(boards []vizBoardRecord) error {
	path := ctl.boardsFilePath()
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	raw, err := json.MarshalIndent(vizBoardFile{Boards: boards}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o600)
}

// vizBoardResponse 合并 Shaper 在线看板与平台登记信息后返回给前端
type vizBoardResponse struct {
	Id          string        `json:"id"`
	Kind        string        `json:"kind"`
	ShaperId    string        `json:"shaperId,omitempty"`
	Name        string        `json:"name"`
	Icon        string        `json:"icon,omitempty"`
	Sql         string        `json:"sql,omitempty"`
	Spec        *vizBoardSpec `json:"spec,omitempty"`
	Canvas      *vizCanvas    `json:"canvas,omitempty"`
	WidgetCount int           `json:"widgetCount"`
	Registered  bool          `json:"registered"`
	IsDefault   bool          `json:"isDefault"`
	IsHome      bool          `json:"isHome"`
	UpdateAt    int64         `json:"updateAt"`
	CreatedAt   string        `json:"createdAt"`
}

// shaperBaseUrlOrFail 取服务端用的 Shaper 地址（gin 版本，失败时直接返回错误响应）
func (ctl *controller) shaperBaseUrlOrFail(c *gin.Context) (string, string, bool) {
	baseUrl, apiKey, err := ctl.shaperBaseUrl()
	if err != nil {
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultSystemError, err), c.Writer, ctl.lc)
		return "", "", false
	}
	return baseUrl, apiKey, true
}

// ShaperBoards 列出 Shaper 里的看板，并合并平台登记信息（SQL 副本、首页/默认标记）
// @Tags    可视化-看板管理
// @Summary 查询看板列表
// @Produce json
// @Success 200 {object} httphelper.CommonResponse
// @Router  /api/v1/viz/boards [get]
// @Security ApiKeyAuth
func (ctl *controller) ShaperBoards(c *gin.Context) {
	records := ctl.loadVizBoards()
	byShaperId := make(map[string]vizBoardRecord, len(records))
	for _, r := range records {
		if r.ShaperId != "" {
			byShaperId[r.ShaperId] = r
		}
	}

	list := make([]vizBoardResponse, 0, len(records)+8)

	// Shaper 里的看板（legacy）。Shaper 不可用时降级：只列平台内的 canvas 看板，不影响管理页。
	apps, err := ctl.shaperApps()
	if err != nil {
		ctl.lc.Warn(fmt.Sprintf("list shaper dashboards failed: %s", err.Error()))
	}
	for _, app := range apps {
		item := vizBoardResponse{
			Id: app.Id, Kind: vizKindShaper, ShaperId: app.Id,
			Name: app.Name, CreatedAt: app.CreatedAt,
		}
		if rec, hit := byShaperId[app.Id]; hit {
			item.Registered = true
			item.Icon = rec.Icon
			item.Sql = rec.Sql
			item.Spec = rec.Spec
			item.IsDefault = rec.IsDefault
			item.IsHome = rec.IsHome
			item.UpdateAt = rec.UpdateAt
			if rec.Name != "" {
				item.Name = rec.Name
			}
		}
		list = append(list, item)
	}

	// platform 画布看板（不在 Shaper 里，由平台渲染）
	for _, rec := range records {
		if rec.kind() != vizKindCanvas {
			continue
		}
		count := 0
		if rec.Canvas != nil {
			count = len(rec.Canvas.Widgets)
		}
		list = append(list, vizBoardResponse{
			Id: rec.boardId(), Kind: vizKindCanvas, Name: rec.Name, Icon: rec.Icon,
			Canvas: rec.Canvas, WidgetCount: count, Registered: true,
			IsDefault: rec.IsDefault, IsHome: rec.IsHome, UpdateAt: rec.UpdateAt,
		})
	}

	sort.SliceStable(list, func(i, j int) bool {
		if list[i].UpdateAt != list[j].UpdateAt {
			return list[i].UpdateAt > list[j].UpdateAt
		}
		return list[i].Name < list[j].Name
	})

	httphelper.ResultSuccess(gin.H{"list": list, "total": len(list)}, c.Writer, ctl.lc)
}

type vizBoardSaveRequest struct {
	Name string `json:"name"`
	// Kind: canvas 时走画布分支（只登记，不建 Shaper 看板）
	Kind string `json:"kind"`
	// Icon 图标名（画布看板用）
	Icon string `json:"icon"`
	// Sql 与 Spec 二选一：传了 Spec 就由平台按可视化配置生成 SQL
	Sql    string        `json:"sql"`
	Spec   *vizBoardSpec `json:"spec"`
	Canvas *vizCanvas    `json:"canvas"`
}

// resolveVizSave 统一处理「可视化配置 / 直接给 SQL」两种入参
func (ctl *controller) resolveVizSave(req *vizBoardSaveRequest) (string, string, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return "", "", fmt.Errorf("name 不能为空")
	}
	if req.Spec != nil {
		sql, err := ctl.buildVizSQL(name, *req.Spec)
		if err != nil {
			return "", "", err
		}
		return name, sql, nil
	}
	sql := strings.TrimSpace(req.Sql)
	if sql == "" {
		return "", "", fmt.Errorf("可视化配置(spec)与 sql 不能同时为空")
	}
	return name, sql, nil
}

// ShaperBoardCreate 新建看板：在 Shaper 里建，同时把 SQL 副本登记到平台
// @Tags    可视化-看板管理
// @Summary 新建看板
// @Accept  json
// @Produce json
// @Success 200 {object} httphelper.CommonResponse
// @Router  /api/v1/viz/boards [post]
// @Security ApiKeyAuth
func (ctl *controller) ShaperBoardCreate(c *gin.Context) {
	baseUrl, _, ok := ctl.shaperBaseUrlOrFail(c)
	if !ok {
		return
	}
	var req vizBoardSaveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultReqParamsError, err), c.Writer, ctl.lc)
		return
	}
	// canvas 看板由平台渲染，不需要在 Shaper 建对象
	if strings.EqualFold(strings.TrimSpace(req.Kind), vizKindCanvas) {
		ctl.createCanvasBoard(c, req)
		return
	}

	name, sql, err := ctl.resolveVizSave(&req)
	if err != nil {
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultReqParamsError, err), c.Writer, ctl.lc)
		return
	}

	created, err := ctl.shaperCreateBoard(baseUrl, name, sql)
	if err != nil {
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultSystemError, err), c.Writer, ctl.lc)
		return
	}

	// 登记 SQL 副本
	vizBoardsMutex.Lock()
	records := ctl.loadVizBoards()
	records = append(records, vizBoardRecord{
		ShaperId: created, Name: name, Sql: sql, Spec: req.Spec, UpdateAt: time.Now().UnixMilli(),
	})
	saveErr := ctl.saveVizBoards(records)
	vizBoardsMutex.Unlock()
	if saveErr != nil {
		ctl.lc.Warn(fmt.Sprintf("save boards registry failed: %s", saveErr.Error()))
	}

	httphelper.ResultSuccess(gin.H{"shaperId": created, "name": name, "sql": sql}, c.Writer, ctl.lc)
}

// shaperCreateBoard 在 Shaper 里创建看板，返回看板 ID
func (ctl *controller) shaperCreateBoard(baseUrl, name, sql string) (string, error) {
	mgmtJwt, err := ctl.shaperMgmtToken(baseUrl)
	if err != nil {
		return "", fmt.Errorf("获取 Shaper 管理凭据失败：%s", err.Error())
	}
	var created struct {
		Id string `json:"id"`
	}
	payload := map[string]interface{}{"name": name, "content": sql}
	if err := ctl.shaperRequest(baseUrl, http.MethodPost, "/api/dashboards", mgmtJwt, payload, &created); err != nil {
		return "", err
	}
	if created.Id == "" {
		return "", fmt.Errorf("shaper 未返回看板 ID")
	}
	return created.Id, nil
}

// ShaperBoardUpdate 更新看板：Shaper 没有更新接口（PUT/PATCH 都是 404），
// 所以实现为「删除旧看板 + 用新 SQL 建一个」，并保留原来的首页/默认标记。
// @Tags    可视化-看板管理
// @Summary 更新看板
// @Accept  json
// @Produce json
// @Success 200 {object} httphelper.CommonResponse
// @Router  /api/v1/viz/boards/{shaperId} [put]
// @Security ApiKeyAuth
func (ctl *controller) ShaperBoardUpdate(c *gin.Context) {
	baseUrl, _, ok := ctl.shaperBaseUrlOrFail(c)
	if !ok {
		return
	}
	boardId := strings.TrimSpace(c.Param("boardId"))
	var req vizBoardSaveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultReqParamsError, err), c.Writer, ctl.lc)
		return
	}
	if boardId == "" {
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultReqParamsError, fmt.Errorf("boardId 不能为空")), c.Writer, ctl.lc)
		return
	}
	// canvas 看板：只更新平台登记记录（改名 / 改布局 / 改组件配置）
	if req.Canvas != nil || strings.EqualFold(strings.TrimSpace(req.Kind), vizKindCanvas) {
		ctl.updateCanvasBoard(c, boardId, req)
		return
	}
	name, sql, err := ctl.resolveVizSave(&req)
	if err != nil {
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultReqParamsError, err), c.Writer, ctl.lc)
		return
	}

	shaperId := boardId
	// 先记住原记录（保留标记），再删旧建新
	vizBoardsMutex.Lock()
	records := ctl.loadVizBoards()
	var old vizBoardRecord
	hasOld := false
	for _, r := range records {
		if r.matchBoardId(boardId) {
			old, hasOld = r, true
			shaperId = r.ShaperId
			break
		}
	}
	vizBoardsMutex.Unlock()

	// Shaper 不允许看板重名，所以顺序取决于是否改名：
	//   改名了 -> 先建新的（名字不冲突），再删旧的，中途失败也不会丢看板
	//   没改名 -> 只能先删旧的再建新的（SQL 副本已在平台侧，建失败可重试）
	deleteOld := func() {
		if mgmtJwt, err := ctl.shaperMgmtToken(baseUrl); err == nil {
			if err := ctl.shaperRequest(baseUrl, http.MethodDelete, "/api/dashboards/"+shaperId, mgmtJwt, nil, nil); err != nil {
				ctl.lc.Warn(fmt.Sprintf("delete old shaper dashboard %s failed: %s", shaperId, err.Error()))
			}
		}
	}

	var newId string
	if hasOld && old.Name == name {
		deleteOld()
		created, err := ctl.shaperCreateBoard(baseUrl, name, sql)
		if err != nil {
			httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultSystemError, err), c.Writer, ctl.lc)
			return
		}
		newId = created
	} else {
		created, err := ctl.shaperCreateBoard(baseUrl, name, sql)
		if err != nil {
			httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultSystemError, err), c.Writer, ctl.lc)
			return
		}
		newId = created
		deleteOld()
	}

	vizBoardsMutex.Lock()
	records = ctl.loadVizBoards()
	next := make([]vizBoardRecord, 0, len(records)+1)
	for _, r := range records {
		if r.ShaperId != shaperId {
			next = append(next, r)
		}
	}
	rec := vizBoardRecord{ShaperId: newId, Name: name, Sql: sql, Spec: req.Spec, UpdateAt: time.Now().UnixMilli()}
	if hasOld {
		rec.IsDefault, rec.IsHome = old.IsDefault, old.IsHome
	}
	next = append(next, rec)
	saveErr := ctl.saveVizBoards(next)
	vizBoardsMutex.Unlock()
	if saveErr != nil {
		ctl.lc.Warn(fmt.Sprintf("save boards registry failed: %s", saveErr.Error()))
	}

	httphelper.ResultSuccess(gin.H{"shaperId": newId, "replacedId": shaperId, "sql": sql}, c.Writer, ctl.lc)
}

// ShaperBoardDelete 删除看板（Shaper + 平台登记）
// @Tags    可视化-看板管理
// @Summary 删除看板
// @Produce json
// @Success 200 {object} httphelper.CommonResponse
// @Router  /api/v1/viz/boards/{shaperId} [delete]
// @Security ApiKeyAuth
func (ctl *controller) ShaperBoardDelete(c *gin.Context) {
	baseUrl, _, ok := ctl.shaperBaseUrlOrFail(c)
	if !ok {
		return
	}
	boardId := strings.TrimSpace(c.Param("boardId"))
	if boardId == "" {
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultReqParamsError, fmt.Errorf("boardId 不能为空")), c.Writer, ctl.lc)
		return
	}
	// canvas 看板：只删平台登记记录
	if ctl.removeCanvasBoard(c, boardId) {
		return
	}
	shaperId := boardId
	mgmtJwt, err := ctl.shaperMgmtToken(baseUrl)
	if err != nil {
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultSystemError, err), c.Writer, ctl.lc)
		return
	}
	if err := ctl.shaperRequest(baseUrl, http.MethodDelete, "/api/dashboards/"+shaperId, mgmtJwt, nil, nil); err != nil {
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultSystemError, err), c.Writer, ctl.lc)
		return
	}

	vizBoardsMutex.Lock()
	records := ctl.loadVizBoards()
	next := make([]vizBoardRecord, 0, len(records))
	for _, r := range records {
		if !r.matchBoardId(shaperId) {
			next = append(next, r)
		}
	}
	saveErr := ctl.saveVizBoards(next)
	vizBoardsMutex.Unlock()
	if saveErr != nil {
		ctl.lc.Warn(fmt.Sprintf("save boards registry failed: %s", saveErr.Error()))
	}

	httphelper.ResultSuccess(gin.H{"shaperId": shaperId}, c.Writer, ctl.lc)
}

type vizBoardActiveRequest struct {
	// Slot: default（数据看板整页）/ home（首页卡片）
	Slot  string `json:"slot"`
	Value bool   `json:"value"`
}

// ShaperBoardActive 把某块看板设为默认/首页看板（写平台登记表，嵌入接口优先用它）
// @Tags    可视化-看板管理
// @Summary 设置默认/首页看板
// @Accept  json
// @Produce json
// @Success 200 {object} httphelper.CommonResponse
// @Router  /api/v1/viz/boards/{shaperId}/active [post]
// @Security ApiKeyAuth
func (ctl *controller) ShaperBoardActive(c *gin.Context) {
	boardId := strings.TrimSpace(c.Param("boardId"))
	var req vizBoardActiveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultReqParamsError, err), c.Writer, ctl.lc)
		return
	}
	if req.Slot != "home" && req.Slot != "default" {
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultReqParamsError, fmt.Errorf("slot 只能是 home 或 default")), c.Writer, ctl.lc)
		return
	}

	vizBoardsMutex.Lock()
	records := ctl.loadVizBoards()
	hit := false
	for i := range records {
		match := records[i].matchBoardId(boardId)
		if req.Slot == "home" {
			records[i].IsHome = req.Value && match
		} else {
			records[i].IsDefault = req.Value && match
		}
		if match {
			hit = true
			records[i].UpdateAt = time.Now().UnixMilli()
		}
	}
	saveErr := ctl.saveVizBoards(records)
	vizBoardsMutex.Unlock()
	if saveErr != nil {
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultSystemError, saveErr), c.Writer, ctl.lc)
		return
	}
	httphelper.ResultSuccess(gin.H{"id": boardId, "slot": req.Slot, "value": req.Value, "hit": hit}, c.Writer, ctl.lc)
}

// activeVizBoardId 取登记表里被标记为默认/首页的看板 ID（给嵌入接口用）
func (ctl *controller) activeVizBoardId(slot string) string {
	for _, r := range ctl.loadVizBoards() {
		// 只服务 legacy shaper 看板的嵌入；画布看板由平台渲染，前端从列表里挑 isDefault
		if r.kind() != vizKindShaper || r.ShaperId == "" {
			continue
		}
		if slot == "home" && r.IsHome {
			return r.ShaperId
		}
		if slot == "default" && r.IsDefault {
			return r.ShaperId
		}
	}
	return ""
}

// ===== 可视化配置 -> Shaper SQL（用户只操作界面，SQL 由平台生成） =====
//
// 列表达式来自数据目录（shaper_catalog.go）：普通列直接用；JSON 派生列（data.cpu_used_percent）
// 用 json_extract 取值；epoch 毫秒时间列在过滤/分组时做相应换算。

// vizJsonMeasureExpr / vizJsonValueExpr 处理 JSON 派生列
func vizJsonMeasureExpr(col string) string {
	if base, key, ok := strings.Cut(col, "."); ok && base != "" && key != "" {
		return "CAST(json_extract(\"" + base + "\", '$." + key + "') AS DOUBLE)"
	}
	return col
}

func vizJsonValueExpr(col string) string {
	if base, key, ok := strings.Cut(col, "."); ok && base != "" && key != "" {
		return "json_extract(\"" + base + "\", '$." + key + "')"
	}
	return col
}

// vizTimeFormat 取时间列的格式（epoch_ms 表示毫秒整数时间戳）
func (ctl *controller) vizTimeFormat(table, col string) string {
	for _, t := range ctl.vizCatalogTables() {
		if t.View != table {
			continue
		}
		for _, c := range t.Columns {
			if c.Name == col {
				return c.Format
			}
		}
	}
	return ""
}

func (ctl *controller) vizAggExpr(table string, m vizMeasure) string {
	col := strings.TrimSpace(m.Column)
	agg := strings.ToLower(strings.TrimSpace(m.Agg))
	if col == "" || col == "*" || agg == "" || agg == "count" {
		return "COUNT(*)"
	}
	if !ctl.vizColumnAllowed(table, col) {
		return "COUNT(*)"
	}
	expr := vizJsonMeasureExpr(col)
	switch agg {
	case "sum":
		return "SUM(" + expr + ")"
	case "avg":
		return "ROUND(AVG(" + expr + "), 2)"
	case "max":
		return "MAX(" + expr + ")"
	case "min":
		return "MIN(" + expr + ")"
	}
	return "COUNT(*)"
}

func vizMeasureAlias(m vizMeasure, index int) string {
	if alias := strings.TrimSpace(m.Alias); alias != "" {
		return alias
	}
	agg := strings.ToLower(strings.TrimSpace(m.Agg))
	col := strings.TrimSpace(m.Column)
	if col == "" || col == "*" || agg == "" {
		return "数量"
	}
	names := map[string]string{"count": "数量", "sum": "合计", "avg": "平均", "max": "最大", "min": "最小"}
	prefix := names[agg]
	if prefix == "" {
		prefix = "数量"
	}
	return prefix + "·" + col
}

// vizInterval 时间范围 -> SQL 间隔与维度时间格式
func vizInterval(timeRange string) (string, string) {
	switch strings.ToLower(strings.TrimSpace(timeRange)) {
	case "1h":
		return "1 HOUR", "%H:%M"
	case "24h":
		return "24 HOUR", "%m-%d %H:00"
	case "7d":
		return "7 DAY", "%m-%d"
	case "30d":
		return "30 DAY", "%m-%d"
	}
	return "", "%Y-%m-%d"
}

// specTimeColumn 时间列：spec 指定了就用它，否则用表目录里的默认时间列（用户不用选）
func (ctl *controller) specTimeColumn(spec vizBoardSpec) string {
	col := strings.TrimSpace(spec.TimeColumn)
	if col != "" && ctl.vizColumnAllowed(spec.View, col) {
		return col
	}
	return ctl.vizCatalogTimeColumn(spec.View)
}

// vizTimeCondition 时间过滤条件（整数毫秒时间戳用 epoch_ms 比较）
func (ctl *controller) vizTimeCondition(spec vizBoardSpec) string {
	interval, _ := vizInterval(spec.TimeRange)
	if interval == "" {
		return ""
	}
	timeCol := ctl.specTimeColumn(spec)
	if timeCol == "" {
		return ""
	}
	if ctl.vizTimeFormat(spec.View, timeCol) == vizFormatEpochMs {
		return timeCol + " > epoch_ms(CAST(now() AS TIMESTAMP) - INTERVAL " + interval + ")"
	}
	return timeCol + " > now() - INTERVAL " + interval
}

// vizWhere 组装 WHERE（目前只有时间范围条件）
func (ctl *controller) vizWhere(spec vizBoardSpec) string {
	condition := ctl.vizTimeCondition(spec)
	if condition == "" {
		return ""
	}
	return " WHERE " + condition
}

// vizDimensionExpr 维度表达式：时间列按范围取粒度，JSON 派生列用 json_extract。
// 没选维度时自动按"记录时间"分桶（默认按天），用户不需要理解"时间字段"这个概念。
func (ctl *controller) vizDimensionExpr(spec vizBoardSpec) string {
	dim := strings.TrimSpace(spec.Dimension)
	if dim == "" {
		auto := ctl.specTimeColumn(spec)
		if auto == "" {
			return ""
		}
		_, format := vizInterval(spec.TimeRange)
		if format == "" {
			format = "%Y-%m-%d"
		}
		if ctl.vizTimeFormat(spec.View, auto) == vizFormatEpochMs {
			return "strftime(to_timestamp(CAST(" + auto + " AS BIGINT) / 1000), '" + format + "')"
		}
		return "strftime(" + auto + ", '" + format + "')"
	}
	if !ctl.vizColumnAllowed(spec.View, dim) {
		return ""
	}
	if dim == ctl.specTimeColumn(spec) {
		_, format := vizInterval(spec.TimeRange)
		if ctl.vizTimeFormat(spec.View, dim) == vizFormatEpochMs {
			return "strftime(to_timestamp(CAST(" + dim + " AS BIGINT) / 1000), '" + format + "')"
		}
		return "strftime(" + dim + ", '" + format + "')"
	}
	return vizJsonValueExpr(dim)
}

func vizDirective(chart string) string {
	switch strings.ToLower(strings.TrimSpace(chart)) {
	case "line":
		return "LINECHART"
	case "bar":
		return "BARCHART"
	case "pie":
		return "PIECHART"
	}
	return ""
}

// validateVizSpec 校验可视化配置（保存与查询共用；非法直接拒绝，避免生成坏 SQL）
func (ctl *controller) validateVizSpec(spec vizBoardSpec) error {
	table := strings.TrimSpace(spec.View)
	if !ctl.vizTableAllowed(table) {
		return fmt.Errorf("未知的数据表：%s", spec.View)
	}
	chart := strings.ToLower(strings.TrimSpace(spec.Chart))
	if chart == "" {
		chart = "bar"
	}
	switch chart {
	case "value", "line", "bar", "pie", "table":
	default:
		return fmt.Errorf("不支持的图表类型：%s", spec.Chart)
	}
	if len(spec.Measures) == 0 {
		return fmt.Errorf("至少需要一个指标")
	}
	for _, m := range spec.Measures {
		col := strings.TrimSpace(m.Column)
		if col == "" || col == "*" {
			continue
		}
		if !ctl.vizColumnAllowed(table, col) {
			return fmt.Errorf("未知的指标列：%s", col)
		}
	}
	if chart != "value" {
		dim := strings.TrimSpace(spec.Dimension)
		if dim != "" && !ctl.vizColumnAllowed(table, dim) {
			return fmt.Errorf("未知的维度列：%s", dim)
		}
		// 没选维度也可以：后端会用表的默认时间列自动按天分桶
		if dim == "" && ctl.vizCatalogTimeColumn(table) == "" {
			return fmt.Errorf("这个图表类型需要选择维度")
		}
	}
	timeRange := strings.ToLower(strings.TrimSpace(spec.TimeRange))
	if timeRange != "" && timeRange != "all" {
		switch timeRange {
		case "1h", "24h", "7d", "30d":
		default:
			return fmt.Errorf("不支持的时间范围：%s", spec.TimeRange)
		}
		tc := strings.TrimSpace(spec.TimeColumn)
		if tc != "" && !ctl.vizColumnAllowed(table, tc) {
			return fmt.Errorf("未知的时间列：%s", tc)
		}
	}
	return nil
}

// buildVizSQL 由可视化配置生成 Shaper 看板 SQL（DSL）
func (ctl *controller) buildVizSQL(name string, spec vizBoardSpec) (string, error) {
	if err := ctl.validateVizSpec(spec); err != nil {
		return "", err
	}
	// 平台表挂在 attach 的 sqlite 目录下（见 shaper/data/init.sql: ATTACH ... AS core）
	// 注意：列校验用不带前缀的表名，只有 FROM 用带前缀的引用
	table := strings.TrimSpace(spec.View)
	view := vizTableRef(table)
	section := strings.TrimSpace(spec.Section)
	if section == "" {
		section = name
	}
	lines := []string{"SELECT '" + vizEscape(section) + "'::SECTION;"}

	where := ctl.vizWhere(spec)
	limit := spec.Limit
	if limit <= 0 {
		limit = 20
	}
	dimExpr := ctl.vizDimensionExpr(spec)
	chart := strings.ToLower(strings.TrimSpace(spec.Chart))
	directive := vizDirective(chart)

	// 数值卡：每个指标一个数字块
	if chart == "value" || dimExpr == "" {
		for i, m := range spec.Measures {
			alias := vizMeasureAlias(m, i)
			lines = append(lines, "SELECT '"+vizEscape(alias)+"'::LABEL;")
			lines = append(lines, "SELECT "+ctl.vizAggExpr(table, m)+" AS \""+vizEscape(alias)+"\" FROM "+view+where+";")
		}
		if len(spec.Measures) == 0 {
			lines = append(lines, "SELECT COUNT(*) AS \"记录数\" FROM "+view+where+";")
		}
		return strings.Join(lines, "\n"), nil
	}

	// 表格
	if chart == "table" {
		cols := []string{dimExpr}
		for i, m := range spec.Measures {
			cols = append(cols, ctl.vizAggExpr(table, m)+" AS \""+vizEscape(vizMeasureAlias(m, i))+"\"")
		}
		sql := "SELECT " + strings.Join(cols, ", ") + " FROM " + view + where + " GROUP BY 1"
		if spec.SortDesc && len(spec.Measures) > 0 {
			sql += " ORDER BY 2 DESC"
		} else {
			sql += " ORDER BY 1"
		}
		sql += fmt.Sprintf(" LIMIT %d;", limit)
		lines = append(lines, sql)
		return strings.Join(lines, "\n"), nil
	}

	// 折线/柱状/饼图：一个指标一张图（Shaper 一张图只渲染第一个图表指令列）
	for i, m := range spec.Measures {
		alias := vizMeasureAlias(m, i)
		if len(spec.Measures) > 1 {
			lines = append(lines, "SELECT '"+vizEscape(alias)+"'::LABEL;")
		}
		sql := "SELECT " + dimExpr + "::XAXIS, " + ctl.vizAggExpr(table, m) + "::" + directive + " FROM " + view + where + " GROUP BY 1"
		if spec.SortDesc {
			sql += " ORDER BY 2 DESC"
		} else {
			sql += " ORDER BY 1"
		}
		sql += fmt.Sprintf(" LIMIT %d;", limit)
		lines = append(lines, sql)
	}
	return strings.Join(lines, "\n"), nil
}

// shaperSqlQuery 调 Shaper 的 /api/sql（只允许单条查询，返回 CSV 文本）并解析成表
func (ctl *controller) shaperSqlQuery(baseUrl, sql string) ([]string, [][]string, error) {
	mgmtJwt, err := ctl.shaperMgmtToken(baseUrl)
	if err != nil {
		return nil, nil, err
	}
	return ctl.shaperSqlQueryWithToken(baseUrl, mgmtJwt, sql)
}

// shaperSqlQueryWithToken 用已有 JWT 执行查询（批量取数时只 mint 一次 token）
func (ctl *controller) shaperSqlQueryWithToken(baseUrl, mgmtJwt, sql string) ([]string, [][]string, error) {
	body, err := json.Marshal(map[string]string{"sql": sql})
	if err != nil {
		return nil, nil, err
	}
	req, err := http.NewRequest(http.MethodPost, baseUrl+"/api/sql", bytes.NewReader(body))
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+mgmtJwt)

	client := &http.Client{Timeout: 30 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(res.Body, 4<<20))
	if res.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("shaper sql returned %s: %s", res.Status, strings.TrimSpace(string(raw)))
	}

	records, err := csv.NewReader(strings.NewReader(string(raw))).ReadAll()
	if err != nil {
		return nil, nil, fmt.Errorf("解析 Shaper 返回失败: %s", err.Error())
	}
	if len(records) == 0 {
		return []string{}, [][]string{}, nil
	}
	return records[0], records[1:], nil
}

// firstVizDataQuery 从生成的看板 SQL 里取出第一条数据查询（跳过标题类语句）
func firstVizDataQuery(sql string) string {
	for _, part := range strings.Split(sql, ";\n") {
		stmt := strings.TrimSpace(part)
		stmt = strings.TrimSuffix(stmt, ";")
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}
		if strings.Contains(stmt, "::SECTION") || strings.Contains(stmt, "::LABEL") {
			continue
		}
		return stmt + ";"
	}
	return ""
}

func vizEscape(s string) string { return strings.ReplaceAll(strings.TrimSpace(s), "'", "''") }

// ShaperVizDatasources 返回可用于可视化配置的数据视图与列
// @Tags    可视化-看板管理
// @Summary 可视化看板可用的数据视图
// @Produce json
// @Success 200 {object} httphelper.CommonResponse
// @Router  /api/v1/viz/datasources [get]
// @Security ApiKeyAuth
func (ctl *controller) ShaperVizDatasources(c *gin.Context) {
	tables, err := ctl.vizCatalog()
	if err != nil {
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultSystemError, err), c.Writer, ctl.lc)
		return
	}
	httphelper.ResultSuccess(gin.H{"views": tables}, c.Writer, ctl.lc)
}

type vizPreviewRequest struct {
	Name string       `json:"name"`
	Spec vizBoardSpec `json:"spec"`
}

// ShaperVizPreview 预览：按可视化配置生成 SQL 并取前几行数据（不落看板）
// @Tags    可视化-看板管理
// @Summary 预览可视化配置生成的数据
// @Accept  json
// @Produce json
// @Success 200 {object} httphelper.CommonResponse
// @Router  /api/v1/viz/boards/preview [post]
// @Security ApiKeyAuth
func (ctl *controller) ShaperVizPreview(c *gin.Context) {
	baseUrl, _, ok := ctl.shaperBaseUrlOrFail(c)
	if !ok {
		return
	}
	var req vizPreviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultReqParamsError, err), c.Writer, ctl.lc)
		return
	}
	sql, err := ctl.buildVizSQL(req.Name, req.Spec)
	if err != nil {
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultReqParamsError, err), c.Writer, ctl.lc)
		return
	}
	// 预览跑第一条「数据」查询（跳过 ::SECTION / ::LABEL 这类标题语句）
	first := firstVizDataQuery(sql)
	if first == "" {
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultReqParamsError, fmt.Errorf("没有可预览的查询")), c.Writer, ctl.lc)
		return
	}

	columns, rows, err := ctl.shaperSqlQuery(baseUrl, first)
	if err != nil {
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultSystemError, err), c.Writer, ctl.lc)
		return
	}
	// 只回前 20 行给界面预览
	if len(rows) > 20 {
		rows = rows[:20]
	}
	httphelper.ResultSuccess(gin.H{"sql": sql, "previewSql": first, "columns": columns, "rows": rows}, c.Writer, ctl.lc)
}
