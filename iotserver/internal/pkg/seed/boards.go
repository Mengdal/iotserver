package seed

import (
	"embed"
	"encoding/json"
	"strings"
)

// 内置看板（部署自带的初始内容）：首页看板、组件演示看板、系统监控看板。
// 普通租户种入首页与组件演示两块；系统监控看板只给平台管理员。
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
	// BoardScopeTenant 普通租户：首页 + 组件演示（系统监控看板是平台自用的）
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
	Id   string `json:"id"`
	Name string `json:"name"`
	Icon string `json:"icon"`
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

// BoardPresetById 按 id 取内置看板定义（存量看板升级时用）。
// 用平台作用域取，平台与租户的内置看板共用同一份定义。
func BoardPresetById(id string) (BoardPreset, bool) {
	for _, p := range BoardPresets(BoardScopePlatform) {
		if p.Id == id {
			return p, true
		}
	}
	return BoardPreset{}, false
}

// UpgradeBoardWidgets 用内置定义升级存量看板里的「占位文本」组件。
//
// 只替换同时满足以下条件的组件：
//   - 组件 id 与内置定义一致；
//   - 当前类型还是 text（说明还是种子里的占位文案，缺省类型也按 text 处理），
//     或上一版临时新增过的 rank 组件（现已改为复用 table）；
//   - 内置定义里该组件已经是别的类型。
//
// 用户改过类型的组件、自定义组件都不会被动；返回新的画布 JSON 与是否发生变化。
func UpgradeBoardWidgets(canvas string, presetCanvas json.RawMessage) (string, bool) {
	var stored, preset map[string]interface{}
	if json.Unmarshal([]byte(canvas), &stored) != nil || json.Unmarshal(presetCanvas, &preset) != nil {
		return canvas, false
	}
	storedWidgets, ok := stored["widgets"].([]interface{})
	if !ok {
		return canvas, false
	}
	presetWidgets, ok := preset["widgets"].([]interface{})
	if !ok {
		return canvas, false
	}
	byID := make(map[string]map[string]interface{}, len(presetWidgets))
	for _, raw := range presetWidgets {
		w, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		if id, _ := w["id"].(string); id != "" {
			byID[id] = w
		}
	}
	changed := false
	for i, raw := range storedWidgets {
		w, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		id, _ := w["id"].(string)
		next, ok := byID[id]
		if !ok {
			continue
		}
		curType, _ := w["type"].(string)
		nextType, _ := next["type"].(string)
		if nextType == "" || nextType == curType {
			continue
		}
		// 只升级内置占位组件（text）和上一版临时新增的 rank 组件；
		// 用户自己换成别的类型时保持不动。
		if curType != "" && curType != "text" && curType != "rank" {
			continue
		}
		// 只换类型与内容：布局和标题沿用存量看板里的值，
		// 否则升级会把用户调好的位置/大小重置成种子里的默认值。
		merged := make(map[string]interface{}, len(next))
		for k, v := range next {
			merged[k] = v
		}
		if layout, ok := w["layout"]; ok {
			merged["layout"] = layout
		}
		if title, _ := w["title"].(string); title != "" {
			merged["title"] = title
		}
		storedWidgets[i] = merged
		changed = true
	}
	if !changed {
		return canvas, false
	}
	stored["widgets"] = storedWidgets
	out, err := json.Marshal(stored)
	if err != nil {
		return canvas, false
	}
	return string(out), true
}
