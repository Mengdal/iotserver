package seed

import (
	"embed"
	"encoding/json"
	"strings"
)

// 内置看板（部署自带的初始内容）：首页看板、组件演示看板、系统监控看板。
//
// 为什么用 go:embed 而不是读文件：运行镜像只带二进制与前端产物（见 Dockerfile），
// 早期"从 .viz-boards.json 迁移"的机制在容器部署里根本不生效——编译期内嵌才跟得上部署。
//
// 触发时机：某个租户的看板列表为空时由网关侧种入（gateway.seedDefaultBoards）；
// id 固定（builtin-*），所以重复种不会重复，把看板全部删掉后再访问会恢复内置内容。
//
//go:embed boards/*.json
var boardFS embed.FS

// BoardScope 看板种子的作用域
type BoardScope string

const (
	// BoardScopePlatform 平台租户：全部内置看板
	BoardScopePlatform BoardScope = "platform"
	// BoardScopeTenant 普通租户：首页 + 系统监控（组件演示看板是平台自用的）
	BoardScopeTenant BoardScope = "tenant"
)

// BoardPreset 一块内置看板
type BoardPreset struct {
	// Id 固定 id（builtin-home / builtin-demo / builtin-ops）
	Id        string
	Name      string
	Icon      string
	IsHome    bool
	IsDefault bool
	// Canvas 画布 JSON（由调用方按自己的类型反序列化）
	Canvas json.RawMessage
}

// boardFile 内置看板文件（boards/*.json）的顶层结构
type boardFile struct {
	Id        string `json:"id"`
	Name      string `json:"name"`
	Icon      string `json:"icon"`
	// Scope 适用作用域：platform（仅平台）或 all（平台与租户都种）
	Scope     string          `json:"scope"`
	IsHome    bool            `json:"isHome"`
	IsDefault bool            `json:"isDefault"`
	Canvas    json.RawMessage `json:"canvas"`
}

// BoardPresets 当前作用域下的内置看板。
// 文件名前缀（10-/20-/30-）即期望的组件库顺序，ReadDir 已按文件名排序。
func BoardPresets(scope BoardScope) []BoardPreset {
	entries, err := boardFS.ReadDir("boards")
	if err != nil {
		return nil
	}
	out := make([]BoardPreset, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		raw, err := boardFS.ReadFile("boards/" + entry.Name())
		if err != nil {
			continue
		}
		var f boardFile
		if err := json.Unmarshal(raw, &f); err != nil {
			continue
		}
		if f.Id == "" || f.Name == "" || len(f.Canvas) == 0 {
			continue
		}
		if f.Scope == "platform" && scope != BoardScopePlatform {
			continue
		}
		out = append(out, BoardPreset{
			Id:        f.Id,
			Name:      f.Name,
			Icon:      f.Icon,
			IsHome:    f.IsHome,
			IsDefault: f.IsDefault,
			Canvas:    f.Canvas,
		})
	}
	return out
}
