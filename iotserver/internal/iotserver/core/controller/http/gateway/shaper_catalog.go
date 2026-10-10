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
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3" // 只读内省平台 sqlite 表结构
)

// 数据目录（对标 Dash 的 Catalog）：直接内省平台自己的关系库，
// 列出**真实存在的表/视图**与列，供图形化配置选择；不再硬编码内置指标视图。
//
// 两个额外能力，让平台的原始表可以直接出图：
//   1. JSON 派生列：TEXT 列里存 JSON 时（如 system_metrics.data），把顶层数值/字符串键
//      派生成 `data.cpu_used_percent` 这样的虚拟列，SQL 里用 json_extract 取值；
//   2. epoch 毫秒时间列：INTEGER 的 created/login_time/timestamp 等，按毫秒时间处理
//      （过滤用 epoch_ms(...)，分组用 to_timestamp(col/1000)）。

const (
	vizCatalogTTL = 30 * time.Second
	vizRoleTime   = "time"

	vizFormatEpochMs = "epoch_ms"
)

// 内置指标视图不再对外提供（数据目录只用平台真实表）
var vizHiddenTables = map[string]struct{}{
	"v_system_metrics": {},
	"v_login_log":      {},
	"v_asset_count":    {},
}

// 界面上不展示的列：技术字段、审计字段、密钥、大 JSON。
// 目标：用户只看到"能用来做图表"的字段，少做选择。
var vizHiddenColumns = map[string]struct{}{
	// 密钥/敏感
	"password": {}, "secret": {}, "open_api_key": {}, "gateway_key": {},
	// 审计/技术
	"id": {}, "created": {}, "modified": {}, "sort": {}, "remark": {},
	"parent_id": {}, "ancestors": {}, "path": {}, "component": {}, "perms": {},
	"is_frame": {}, "visible": {}, "menu_type": {}, "icon": {},
	"container_name": {}, "docker_repo_name": {}, "docker_image_id": {},
	"support_versions": {}, "language": {}, "jump_link": {},
	"config_key": {}, "config_value": {},
	// 大 JSON 原串（它的字段会以 data.xxx 派生列出现）
	"thing_model_json": {}, "data": {},
}

// vizJsonKeyLabels JSON 派生列的中文名（用户看到的就是这些，而不是原始 key）
var vizJsonKeyLabels = map[string]string{
	"cpu_used_percent":    "CPU 使用率(%)",
	"cpu_avg":             "CPU 平均使用率(%)",
	"memory.used_percent": "内存使用率(%)",
	"disk.used_percent":   "磁盘使用率(%)",
	"memory.used":         "内存已用",
	"memory.total":        "内存总量",
	"disk.used":           "磁盘已用",
	"disk.total":          "磁盘总容量",
}

// vizJsonKeyLabel 派生列显示名：常见指标用中文，网卡流量按后缀拼
func vizJsonKeyLabel(key string) string {
	if label, ok := vizJsonKeyLabels[key]; ok {
		return label
	}
	lower := strings.ToLower(key)
	if strings.Contains(lower, "bytes_recv") {
		return "网卡接收(字节)"
	}
	if strings.Contains(lower, "bytes_sent") {
		return "网卡发送(字节)"
	}
	return key
}

// vizColumnHidden 列是否不展示（含后缀规则：外键 / 编码 / 密钥 / JSON）
func vizColumnHidden(name string) bool {
	lower := strings.ToLower(strings.TrimSpace(name))
	if _, hidden := vizHiddenColumns[lower]; hidden {
		return true
	}
	for _, suffix := range []string{"_id", "_key", "_json", "_token", "_secret", "_password"} {
		if strings.HasSuffix(lower, suffix) {
			return true
		}
	}
	return false
}

// vizJsonKeyUseful JSON 派生列只保留能看懂的指标（占比/用量/总量/流量）
func vizJsonKeyUseful(key string) bool {
	lower := strings.ToLower(key)
	if strings.Contains(lower, "percent") {
		return true
	}
	for _, suffix := range []string{"used", "total"} {
		if strings.HasSuffix(lower, suffix) {
			return true
		}
	}
	return strings.Contains(lower, "bytes")
}

// 常用表排序（数字小的排前面）
var vizTablePriority = map[string]int{
	"product":              1,
	"device":               2,
	"thing_model_template": 3,
	"category_template":    4,
	"device_library":       5,
	"driver_classify":      6,
	"sys_login_log":        7,
	"system_metrics":       8,
	"user":                 9,
	"sys_role":             10,
	"sys_dept":             11,
	"sys_menu":             12,
	"alert_list":           13,
	"alert_rule":           14,
}

// 常见表的中文名（没有就用原表名）
var vizTableLabels = map[string]string{
	"product":              "产品",
	"device":               "设备",
	"device_library":       "驱动库",
	"driver_classify":      "驱动分类",
	"thing_model_template": "物模型模板",
	"category_template":    "分类模板",
	"sys_login_log":        "登录日志",
	"system_metrics":       "系统指标快照",
	"user":                 "用户",
	"sys_role":             "角色",
	"sys_menu":             "菜单",
	"sys_dept":             "部门",
	"sys_config":           "系统参数",
	"quick_navigation":     "快捷入口",
	"properties":           "物模型属性",
	"mqtt_auth":            "MQTT 认证",
	"language_sdk":         "语言 SDK",
	"data_resource":        "数据资源",
	"docker_config":        "镜像仓库配置",
	"alert_rule":           "告警规则",
	"alert_list":           "告警记录",
	"scene":                "场景",
	"scene_log":            "场景日志",
	"rule_engine":          "规则引擎",
	"device_service":       "设备服务",
	"actions":              "物模型动作",
	"events":               "物模型事件",
	"unit":                 "单位",
	"doc":                  "文档",
	"advance_config":       "高级配置",
	"msg_gather":           "消息汇总",
}

type vizCatalogColumn struct {
	Name   string `json:"name"`
	Role   string `json:"role"` // time / dimension / measure
	Label  string `json:"label,omitempty"`
	Format string `json:"format,omitempty"` // int / float / percent / epoch_ms
	// Json JSON 派生列的键（列名形如 data.cpu_used_percent）
	Json string `json:"json,omitempty"`
	// Hidden 界面上不展示（技术字段/外键/密钥/大 JSON），但服务端仍可用于默认时间列等
	Hidden bool `json:"hidden,omitempty"`
}

type vizCatalogTable struct {
	// View 表名（前端字段名沿用 view）
	View  string `json:"view"`
	Label string `json:"label"`
	Kind  string `json:"kind"` // table / view
	// TimeColumn 默认时间列：设时间范围 / 画趋势时自动用它，用户不用选
	TimeColumn string             `json:"timeColumn,omitempty"`
	Columns    []vizCatalogColumn `json:"columns"`
}

var vizCatalogCache struct {
	sync.Mutex
	at     time.Time
	tables []vizCatalogTable
	err    error
}

// metadataSqlitePath 平台关系库（sqlite）路径
func (ctl *controller) metadataSqlitePath() (string, bool) {
	byType, ok := ctl.cfg.Databases["Metadata"]
	if !ok {
		return "", false
	}
	primary, ok := byType["Primary"]
	if !ok {
		return "", false
	}
	if !strings.EqualFold(strings.TrimSpace(primary.Type), "sqlite") {
		return "", false
	}
	dsn := strings.TrimSpace(primary.DataSource)
	if dsn == "" {
		return "", false
	}
	if idx := strings.Index(dsn, "?"); idx >= 0 {
		dsn = dsn[:idx]
	}
	return dsn, dsn != ""
}

// vizCatalog 取数据目录（带短 TTL 缓存，表结构很少变）
func (ctl *controller) vizCatalog() ([]vizCatalogTable, error) {
	vizCatalogCache.Lock()
	defer vizCatalogCache.Unlock()
	if time.Since(vizCatalogCache.at) < vizCatalogTTL && vizCatalogCache.tables != nil {
		return vizCatalogCache.tables, vizCatalogCache.err
	}
	tables, err := ctl.loadVizCatalog()
	if err == nil {
		vizCatalogCache.tables, vizCatalogCache.err = tables, nil
	} else {
		ctl.lc.Warn(fmt.Sprintf("load viz catalog failed: %s", err.Error()))
		vizCatalogCache.tables, vizCatalogCache.err = nil, err
	}
	vizCatalogCache.at = time.Now()
	return vizCatalogCache.tables, err
}

// loadVizCatalog 读取 sqlite 表结构（只读打开）
func (ctl *controller) loadVizCatalog() ([]vizCatalogTable, error) {
	path, ok := ctl.metadataSqlitePath()
	if !ok {
		return nil, fmt.Errorf("当前部署不是 sqlite，暂不支持自动读取数据目录")
	}
	db, err := sql.Open("sqlite3", "file:"+path+"?mode=ro&_busy_timeout=3000")
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rows, err := db.Query("SELECT name, type FROM sqlite_master WHERE type IN ('table','view') AND name NOT LIKE 'sqlite_%' ORDER BY name")
	if err != nil {
		return nil, err
	}
	type rawTable struct{ name, kind string }
	var raw []rawTable
	for rows.Next() {
		var name, kind string
		if err := rows.Scan(&name, &kind); err != nil {
			continue
		}
		if _, hidden := vizHiddenTables[name]; hidden {
			continue
		}
		raw = append(raw, rawTable{name: name, kind: kind})
	}
	rows.Close()

	out := make([]vizCatalogTable, 0, len(raw))
	for _, item := range raw {
		cols, err := ctl.loadVizTableColumns(db, item.name)
		if err != nil || len(cols) == 0 {
			continue
		}
		label := vizTableLabels[item.name]
		if label == "" {
			label = item.name
		}
		out = append(out, vizCatalogTable{
			View: item.name, Label: label, Kind: item.kind,
			TimeColumn: vizDefaultTimeColumn(cols), Columns: cols,
		})
	}
	// 常用表排前面（默认视图更好用），其余按表名
	sort.SliceStable(out, func(i, j int) bool {
		pi, iok := vizTablePriority[out[i].View]
		pj, jok := vizTablePriority[out[j].View]
		if iok != jok {
			return iok
		}
		if iok && pi != pj {
			return pi < pj
		}
		return out[i].View < out[j].View
	})
	return out, nil
}

// loadVizTableColumns 读一张表的列（含 JSON 派生列）
func (ctl *controller) loadVizTableColumns(db *sql.DB, table string) ([]vizCatalogColumn, error) {
	rows, err := db.Query(fmt.Sprintf("PRAGMA table_info(%q)", table))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	cols := make([]vizCatalogColumn, 0, 16)
	textCols := make([]string, 0, 4)
	for rows.Next() {
		var cid int
		var name, declType string
		var notNull, pk int
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &declType, &notNull, &dflt, &pk); err != nil {
			continue
		}
		hidden := vizColumnHidden(name)
		role, format := vizColumnRole(name, declType)
		cols = append(cols, vizCatalogColumn{Name: name, Role: role, Format: format, Hidden: hidden})
		if role != vizRoleTime && strings.Contains(strings.ToUpper(declType), "TEXT") {
			textCols = append(textCols, name)
		}
	}

	// JSON 派生列（只取有值且像 JSON 对象的那一列）
	for _, col := range textCols {
		derived := ctl.loadVizJsonColumns(db, table, col)
		if len(derived) > 0 {
			cols = append(cols, derived...)
			break
		}
	}
	return cols, nil
}

// loadVizJsonColumns 采样多行 JSON，把出现过的**标量路径**（含嵌套，最多 3 层）派生成虚拟列
// 例：system_metrics.data -> data.cpu_used_percent / data.memory.used_percent / data.disk.used_percent
func (ctl *controller) loadVizJsonColumns(db *sql.DB, table, col string) []vizCatalogColumn {
	query := fmt.Sprintf("SELECT %q FROM %q WHERE %q IS NOT NULL AND %q LIKE '{%%' LIMIT 80", col, table, col, col)
	rows, err := db.Query(query)
	if err != nil {
		return nil
	}
	defer rows.Close()

	kinds := make(map[string]string) // path -> number / string / bool
	order := make([]string, 0, 32)
	for rows.Next() {
		var raw sql.NullString
		if err := rows.Scan(&raw); err != nil || !raw.Valid {
			continue
		}
		var parsed map[string]interface{}
		if err := json.Unmarshal([]byte(raw.String), &parsed); err != nil {
			continue
		}
		for path, kind := range vizFlattenJson(parsed, "", 0) {
			if _, seen := kinds[path]; !seen {
				kinds[path] = kind
				order = append(order, path)
			}
		}
	}
	sort.Slice(order, func(i, j int) bool {
		// 使用率（percent）优先，其次浅层路径，最后按名字
		pi := strings.Contains(strings.ToLower(order[i]), "percent")
		pj := strings.Contains(strings.ToLower(order[j]), "percent")
		if pi != pj {
			return pi
		}
		di, dj := strings.Count(order[i], "."), strings.Count(order[j], ".")
		if di != dj {
			return di < dj
		}
		return order[i] < order[j]
	})

	out := make([]vizCatalogColumn, 0, len(order))
	for _, path := range order {
		// 只保留能看懂的指标（占比/用量/总量/流量），别的派生列不展示
		if !vizJsonKeyUseful(path) {
			continue
		}
		if kinds[path] == "number" {
			lower := strings.ToLower(path)
			format := "float"
			switch {
			case strings.Contains(lower, "percent") || strings.HasSuffix(lower, "_avg"):
				format = "percent"
			case strings.Contains(lower, "bytes") || strings.HasSuffix(lower, "used") ||
				strings.HasSuffix(lower, "total"):
				format = "bytes"
			}
			out = append(out, vizCatalogColumn{
				Name: col + "." + path, Role: "measure",
				Label: vizJsonKeyLabel(path), Format: format, Json: path,
			})
		} else {
			out = append(out, vizCatalogColumn{
				Name: col + "." + path, Role: "dimension", Label: path, Json: path,
			})
		}
	}
	if len(out) > 12 {
		out = out[:12]
	}
	return out
}

// vizFlattenJson 递归收集标量路径（对象最多下探 3 层，数组跳过）
func vizFlattenJson(value map[string]interface{}, prefix string, depth int) map[string]string {
	out := make(map[string]string)
	if depth > 3 {
		return out
	}
	for key, item := range value {
		path := key
		if prefix != "" {
			path = prefix + "." + key
		}
		switch typed := item.(type) {
		case float64:
			out[path] = "number"
		case string:
			out[path] = "string"
		case bool:
			out[path] = "bool"
		case map[string]interface{}:
			for p, kind := range vizFlattenJson(typed, path, depth+1) {
				out[p] = kind
			}
		}
	}
	return out
}

// vizColumnRole 按列名与声明类型判断角色（以及时间列的存储格式）
func vizColumnRole(name, declType string) (string, string) {
	lower := strings.ToLower(strings.TrimSpace(name))
	upper := strings.ToUpper(declType)
	numeric := strings.Contains(upper, "INT") || strings.Contains(upper, "REAL") ||
		strings.Contains(upper, "NUMERIC") || strings.Contains(upper, "FLOAT") ||
		strings.Contains(upper, "DOUBLE") || strings.Contains(upper, "DECIMAL")

	isTimeName := lower == "created" || lower == "modified" || lower == "timestamp" || lower == "ts" ||
		strings.HasSuffix(lower, "_time") || strings.HasSuffix(lower, "_at") ||
		strings.Contains(upper, "DATE") || strings.Contains(upper, "TIME")
	if isTimeName {
		// 整型时间戳按 epoch 毫秒处理（平台里 created/login_time/timestamp 都是毫秒）
		if numeric {
			return vizRoleTime, vizFormatEpochMs
		}
		return vizRoleTime, ""
	}
	if numeric {
		format := "float"
		if strings.Contains(lower, "percent") {
			format = "percent"
		} else if strings.Contains(upper, "INT") {
			format = "int"
		}
		return "measure", format
	}
	return "dimension", ""
}

// vizDefaultTimeColumn 表的默认时间列：优先非隐藏的，其次隐藏的（created/login_time/timestamp…）
func vizDefaultTimeColumn(cols []vizCatalogColumn) string {
	fallback := ""
	for _, c := range cols {
		if c.Role != vizRoleTime {
			continue
		}
		if !c.Hidden {
			return c.Name
		}
		if fallback == "" {
			fallback = c.Name
		}
	}
	return fallback
}

// vizResultFormats 按 spec 推导查询结果每一列的格式（顺序与 SQL 输出列一致）
func (ctl *controller) vizResultFormats(spec vizBoardSpec, columns []string) []string {
	// 目录里查一列的角色/格式
	lookup := func(col string) (string, string) {
		for _, t := range ctl.vizCatalogTables() {
			if t.View != strings.TrimSpace(spec.View) {
				continue
			}
			for _, c := range t.Columns {
				if c.Name == col {
					return c.Role, c.Format
				}
			}
		}
		return "", ""
	}
	chart := strings.ToLower(strings.TrimSpace(spec.Chart))
	formats := make([]string, 0, len(columns)+1)
	if chart != "value" {
		// 第一列是维度（或自动按时间分桶）
		dim := strings.TrimSpace(spec.Dimension)
		if dim == "" {
			formats = append(formats, "date")
		} else if dim == ctl.specTimeColumn(spec) {
			formats = append(formats, "date")
		} else {
			formats = append(formats, "text")
		}
	}
	for _, m := range spec.Measures {
		if strings.TrimSpace(m.Column) == "" || strings.TrimSpace(m.Column) == "*" {
			formats = append(formats, "int")
			continue
		}
		_, format := lookup(m.Column)
		if format == "" {
			format = "float"
		}
		formats = append(formats, format)
	}
	if len(formats) < len(columns) {
		for i := len(formats); i < len(columns); i++ {
			formats = append(formats, "text")
		}
	}
	if len(formats) > len(columns) {
		formats = formats[:len(columns)]
	}
	return formats
}

// vizTableRef 生成 SQL 里的表引用：平台表都在 attach 的 core 目录下
func vizTableRef(table string) string {
	name := strings.TrimSpace(table)
	if name == "" {
		return name
	}
	if strings.Contains(name, ".") {
		return name
	}
	return "core." + name
}

// vizCatalogTimeColumn 取某表的默认时间列（目录里声明）
func (ctl *controller) vizCatalogTimeColumn(view string) string {
	for _, t := range ctl.vizCatalogTables() {
		if t.View == strings.TrimSpace(view) {
			return t.TimeColumn
		}
	}
	return ""
}

// vizTableAllowed 表是否在数据目录里
func (ctl *controller) vizTableAllowed(table string) bool {
	for _, t := range ctl.vizCatalogTables() {
		if t.View == strings.TrimSpace(table) {
			return true
		}
	}
	return false
}

// vizColumnAllowed 列是否属于该表（含 JSON 派生列）
func (ctl *controller) vizColumnAllowed(table, col string) bool {
	table = strings.TrimSpace(table)
	col = strings.TrimSpace(col)
	for _, t := range ctl.vizCatalogTables() {
		if t.View != table {
			continue
		}
		for _, c := range t.Columns {
			if c.Name == col {
				return true
			}
		}
	}
	return false
}

// vizCatalogTables 目录（出错时返回空，校验自然失败并给出提示）
func (ctl *controller) vizCatalogTables() []vizCatalogTable {
	tables, err := ctl.vizCatalog()
	if err != nil {
		return nil
	}
	return tables
}
