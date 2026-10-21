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
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"iotserver/internal/iotserver/core/container"
	interfaces "iotserver/internal/iotserver/core/interface"
	"iotserver/internal/models"
	"iotserver/internal/pkg/errort"
	"iotserver/internal/pkg/httphelper"
	"iotserver/internal/pkg/seed"
	"iotserver/internal/pkg/tenant"
)

// 看板管理（画布看板 CRUD）
//
// 看板只有一种形态：平台自己渲染的画布（列数/行高 + 若干组件，见 viz_canvas.go），
// 落库在 viz_board 表（按租户隔离）。租户还没有任何看板时，自动种入内置看板
// （首页看板 / 组件演示看板 / 系统监控看板，编译期内嵌，见 pkg/seed/boards.go）。
type vizBoardRecord struct {
	// Id 平台内看板 ID
	Id   string `json:"id"`
	Name string `json:"name"`
	// Icon 看板图标名（前端图标选择器给的值，列表里显示）
	Icon string `json:"icon,omitempty"`
	// Canvas 画布内容：布局 + 每个组件
	Canvas    *vizCanvas `json:"canvas,omitempty"`
	IsDefault bool       `json:"isDefault"`
	IsHome    bool       `json:"isHome"`
	UpdateAt  int64      `json:"updateAt"`
}

// matchBoardId 是否匹配前端传来的看板 ID
func (r vizBoardRecord) matchBoardId(id string) bool {
	id = strings.TrimSpace(id)
	return id != "" && r.Id == id
}

// vizBoardsMutex 串行化「读全量 → 改动 → 写全量」的读改写（管理接口并发很低，简单加锁即可）
var vizBoardsMutex sync.Mutex

// db 当前请求的数据库客户端（租户作用域由 WithCtx 携带，看板表按租户隔离）
func (ctl *controller) db(ctx context.Context) interfaces.DBClient {
	return container.DBClientFrom(ctl.dic.Get).WithCtx(ctx)
}

// boardTenantId 看板数据的有效租户：
// 平台管理员在「平台视图」（未代管租户）时是空串，即平台自己的看板；
// 代管租户 / 普通租户时是该租户 ID。看板属于租户配置，不能跨租户混在一起
// （同名看板会重复，且「设为首页/默认」会互相覆盖）。
func boardTenantId(ctx context.Context) string {
	return tenant.FromContext(ctx).TenantID
}

// boardDB 看板读写使用的 DB 客户端。
// 用平台作用域绕过租户插件的隐式过滤，再由仓储方法显式传 tenant_id：
// 空租户在 Enforce 策略下会被插件判为「无上下文」而返回空集。
func (ctl *controller) boardDB(ctx context.Context) interfaces.DBClient {
	return container.DBClientFrom(ctl.dic.Get).WithCtx(tenant.Platform(ctx))
}

// loadVizBoards 读看板列表（落库，按租户隔离）。
//
// 种入内置看板的两个时机（新建/删除都不影响）：
//   - 该租户一块看板都没有 → 种全套（新版部署自带首页/演示/监控）；
//   - 有看板、但一块都没设成「首页看板」 → 只补一块内置首页看板
//     （首页整页渲染首页看板，这个槽位不能空，存量安装靠它自愈）。
func (ctl *controller) loadVizBoards(ctx context.Context) []vizBoardRecord {
	tenantId := boardTenantId(ctx)
	rows, err := ctl.boardDB(ctx).VizBoardsByTenant(tenantId)
	if err != nil {
		ctl.lc.Warn(fmt.Sprintf("查询看板失败: %s", err.Error()))
		return nil
	}
	needSeed, onlyHome := false, false
	switch {
	case len(rows) == 0:
		needSeed = true
	case !vizHasHome(rows):
		needSeed, onlyHome = true, true
	}
	if needSeed {
		ctl.seedDefaultBoards(ctx, onlyHome)
		if rows, err = ctl.boardDB(ctx).VizBoardsByTenant(tenantId); err != nil {
			return nil
		}
	}
	return vizRecordsFromRows(rows)
}

// vizRecordsFromRows 数据库行 → 业务记录（画布 JSON 反序列化）
func vizRecordsFromRows(rows []models.VizBoard) []vizBoardRecord {
	out := make([]vizBoardRecord, 0, len(rows))
	for _, r := range rows {
		rec := vizBoardRecord{
			Id:        r.Id,
			Name:      r.Name,
			Icon:      r.Icon,
			IsDefault: r.IsDefault == 1,
			IsHome:    r.IsHome == 1,
			UpdateAt:  r.UpdateAt,
		}
		if strings.TrimSpace(r.Canvas) != "" {
			var cv vizCanvas
			if err := json.Unmarshal([]byte(r.Canvas), &cv); err == nil {
				rec.Canvas = &cv
			}
		}
		out = append(out, rec)
	}
	return out
}

// saveVizBoards 覆盖式保存看板
func (ctl *controller) saveVizBoards(ctx context.Context, boards []vizBoardRecord) error {
	list := make([]models.VizBoard, 0, len(boards))
	for _, b := range boards {
		canvas := ""
		if b.Canvas != nil {
			raw, err := json.Marshal(b.Canvas)
			if err != nil {
				return err
			}
			canvas = string(raw)
		}
		isDefault, isHome := 0, 0
		if b.IsDefault {
			isDefault = 1
		}
		if b.IsHome {
			isHome = 1
		}
		list = append(list, models.VizBoard{
			Id:        b.Id,
			Name:      b.Name,
			Icon:      b.Icon,
			Canvas:    canvas,
			IsDefault: isDefault,
			IsHome:    isHome,
			UpdateAt:  b.UpdateAt,
		})
	}
	return ctl.boardDB(ctx).VizBoardsReplaceByTenant(boardTenantId(ctx), list)
}

// vizHasHome 是否已有「首页看板」（首页整页渲染它，不能空）
func vizHasHome(rows []models.VizBoard) bool {
	for _, r := range rows {
		if r.IsHome == 1 {
			return true
		}
	}
	return false
}

// seedDefaultBoards 种入内置看板（部署自带）。
//
// 平台租户得到首页/演示/监控三块，普通租户得到首页/演示两块（见 pkg/seed/boards.go）；
// onlyHome = true 时只补首页看板（存量安装"有看板但没设首页"的自愈场景）。
// 画布走一遍 normalize 校验，保证内置 JSON 与编辑器保存的走同一套规则；id 稳定。
//
// 注意：viz_board 的 id 是全局主键（跨租户不重复），而内置看板的种子 id 是固定的
// （builtin-home 等），所以给非平台租户的 id 拼上租户后缀，避免不同租户种入时主键冲突。
func (ctl *controller) seedDefaultBoards(ctx context.Context, onlyHome bool) {
	scope := seed.BoardScopeTenant
	idSuffix := "-" + tenant.FromContext(ctx).TenantID
	if tenant.IsPlatformScope(ctx) {
		scope = seed.BoardScopePlatform
		idSuffix = ""
	}
	presets := seed.BoardPresets(scope)
	records := make([]vizBoardRecord, 0, len(presets))
	for _, p := range presets {
		if onlyHome && !p.IsHome {
			continue
		}
		var cv vizCanvas
		if err := json.Unmarshal(p.Canvas, &cv); err != nil {
			ctl.lc.Warnf("内置看板 %s 的画布解析失败: %s", p.Id, err.Error())
			continue
		}
		if err := cv.normalize(p.Name); err != nil {
			ctl.lc.Warnf("内置看板 %s 校验失败: %s", p.Id, err.Error())
			continue
		}
		rec := vizBoardRecord{
			Id:        p.Id + idSuffix,
			Name:      p.Name,
			Icon:      p.Icon,
			Canvas:    &cv,
			IsHome:    p.IsHome,
			IsDefault: p.IsDefault,
		}
		touchVizBoard(&rec)
		records = append(records, rec)
	}
	if len(records) == 0 {
		return
	}
	if err := ctl.mergeSaveBoards(ctx, records); err != nil {
		ctl.lc.Warnf("种入内置看板失败: %s", err.Error())
		return
	}
	ctl.lc.Infof("已种入 %d 块内置看板（scope=%s onlyHome=%v）", len(records), scope, onlyHome)
}

// mergeSaveBoards 把内置看板合并进该租户现有列表后整体保存。
//
// 不能直接 saveVizBoards(seeded)：那是「整体替换」，只补一块首页看板的自愈场景下
// 会把该租户其它看板全部删掉。这里按 id 合并：内置覆盖同 id 项，其余原样保留。
func (ctl *controller) mergeSaveBoards(ctx context.Context, seeded []vizBoardRecord) error {
	rows, err := ctl.boardDB(ctx).VizBoardsByTenant(boardTenantId(ctx))
	if err != nil {
		return err
	}
	byId := make(map[string]vizBoardRecord, len(rows)+len(seeded))
	for _, r := range vizRecordsFromRows(rows) {
		byId[r.Id] = r
	}
	for _, r := range seeded {
		byId[r.Id] = r
	}
	merged := make([]vizBoardRecord, 0, len(byId))
	for _, r := range byId {
		merged = append(merged, r)
	}
	return ctl.saveVizBoards(ctx, merged)
}

// vizBoardResponse 看板列表项
type vizBoardResponse struct {
	Id          string     `json:"id"`
	Name        string     `json:"name"`
	Icon        string     `json:"icon,omitempty"`
	Canvas      *vizCanvas `json:"canvas,omitempty"`
	WidgetCount int        `json:"widgetCount"`
	IsDefault   bool       `json:"isDefault"`
	IsHome      bool       `json:"isHome"`
	UpdateAt    int64      `json:"updateAt"`
}

// VizBoards 查询看板列表
// @Tags    可视化-看板管理
// @Summary 查询看板列表
// @Produce json
// @Success 200 {object} httphelper.CommonResponse
// @Router  /api/v1/viz/boards [get]
// @Security ApiKeyAuth
func (ctl *controller) VizBoards(c *gin.Context) {
	records := ctl.loadVizBoards(c)

	list := make([]vizBoardResponse, 0, len(records))
	for _, rec := range records {
		count := 0
		if rec.Canvas != nil {
			count = len(rec.Canvas.allWidgets())
		}
		list = append(list, vizBoardResponse{
			Id: rec.Id, Name: rec.Name, Icon: rec.Icon, Canvas: rec.Canvas,
			WidgetCount: count, IsDefault: rec.IsDefault, IsHome: rec.IsHome, UpdateAt: rec.UpdateAt,
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
	// Icon 图标名（可选）
	Icon   string     `json:"icon"`
	Canvas *vizCanvas `json:"canvas"`
}

// VizBoardCreate 新建看板
// @Tags    可视化-看板管理
// @Summary 新建看板
// @Accept  json
// @Produce json
// @Success 200 {object} httphelper.CommonResponse
// @Router  /api/v1/viz/boards [post]
// @Security ApiKeyAuth
func (ctl *controller) VizBoardCreate(c *gin.Context) {
	var req vizBoardSaveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultReqParamsError, err), c.Writer, ctl.lc)
		return
	}
	ctl.createCanvasBoard(c, req)
}

// VizBoardUpdate 更新看板（名称 / 图标 / 布局整体覆盖）
// @Tags    可视化-看板管理
// @Summary 更新看板
// @Accept  json
// @Produce json
// @Success 200 {object} httphelper.CommonResponse
// @Router  /api/v1/viz/boards/{boardId} [put]
// @Security ApiKeyAuth
func (ctl *controller) VizBoardUpdate(c *gin.Context) {
	boardId := strings.TrimSpace(c.Param("boardId"))
	if boardId == "" {
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultReqParamsError, errort.NewMessageErr("boardId 不能为空", nil)), c.Writer, ctl.lc)
		return
	}
	var req vizBoardSaveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultReqParamsError, err), c.Writer, ctl.lc)
		return
	}
	ctl.updateCanvasBoard(c, boardId, req)
}

// VizBoardDelete 删除看板
// @Tags    可视化-看板管理
// @Summary 删除看板
// @Produce json
// @Success 200 {object} httphelper.CommonResponse
// @Router  /api/v1/viz/boards/{boardId} [delete]
// @Security ApiKeyAuth
func (ctl *controller) VizBoardDelete(c *gin.Context) {
	boardId := strings.TrimSpace(c.Param("boardId"))
	if boardId == "" {
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultReqParamsError, errort.NewMessageErr("boardId 不能为空", nil)), c.Writer, ctl.lc)
		return
	}
	if !ctl.removeCanvasBoard(c, boardId) {
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultSystemError,
			errort.NewMessageErr("看板不存在：{{.p1}}", map[string]interface{}{"p1": boardId})), c.Writer, ctl.lc)
	}
}

type vizBoardActiveRequest struct {
	// Slot: default（数据看板整页）/ home（首页卡片）
	Slot  string `json:"slot"`
	Value bool   `json:"value"`
}

// VizBoardActive 把某块看板设为默认/首页看板（写登记表）
// @Tags    可视化-看板管理
// @Summary 设置默认/首页看板
// @Accept  json
// @Produce json
// @Success 200 {object} httphelper.CommonResponse
// @Router  /api/v1/viz/boards/{boardId}/active [post]
// @Security ApiKeyAuth
func (ctl *controller) VizBoardActive(c *gin.Context) {
	boardId := strings.TrimSpace(c.Param("boardId"))
	var req vizBoardActiveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultReqParamsError, err), c.Writer, ctl.lc)
		return
	}
	if req.Slot != "home" && req.Slot != "default" {
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultReqParamsError, errort.NewMessageErr("slot 只能是 home 或 default", nil)), c.Writer, ctl.lc)
		return
	}

	vizBoardsMutex.Lock()
	records := ctl.loadVizBoards(c)
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
	saveErr := ctl.saveVizBoards(c, records)
	vizBoardsMutex.Unlock()
	if saveErr != nil {
		httphelper.RenderFail(c, errort.NewCommonErr(errort.DefaultSystemError, saveErr), c.Writer, ctl.lc)
		return
	}
	httphelper.ResultSuccess(gin.H{"id": boardId, "slot": req.Slot, "value": req.Value, "hit": hit}, c.Writer, ctl.lc)
}
