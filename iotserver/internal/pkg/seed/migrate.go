package seed

import (
	"encoding/json"
	"strings"
	"time"

	"gorm.io/gorm"

	"iotserver/internal/models"
	"iotserver/internal/pkg/constants"
)

// ThingsPlatform 参考系统（iotddc / things）写入产品 / 设备 platform 字段的取值。
// 与 constants.IotPlatform_Things 保持一致；seed 不反向依赖 constants，避免包循环。
const ThingsPlatform = "things platform"

// legacyThingsPlatform 历史版本写入的参考系统平台值，仅用于存量数据迁移。
// 用 Unicode 转义保留原值，避免旧名称重新出现在仓库文本里。
const legacyThingsPlatform = "\u7f57\u7c73\u7269\u8054\u7f51\u5e73\u53f0"

// MigrateLegacyPlatform 把存量产品 / 设备上的旧平台值改成 things platform。
// 平台值会参与驱动同步与云平台筛选，不迁移会导致历史数据查不到。
func MigrateLegacyPlatform(tx *gorm.DB) error {
	for _, table := range []string{"product", "device"} {
		if err := tx.Table(table).
			Where("platform = ?", legacyThingsPlatform).
			Update("platform", ThingsPlatform).Error; err != nil {
			return err
		}
	}
	return nil
}

// MigrateLegacyDeviceStatus 把设备状态从历史的中文展示文案迁移为稳定代码。
// 状态入库后不再随界面语言变化，展示层负责翻译。
func MigrateLegacyDeviceStatus(tx *gorm.DB) error {
	legacy := map[string]string{
		"在线":  "online",
		"离线":  "offline",
		"未激活": "inactive",
		"禁用":  "disabled",
		"未知":  "unknown",
	}
	for old, next := range legacy {
		if err := tx.Table("device").Where("status = ?", old).
			Update("status", next).Error; err != nil {
			return err
		}
	}
	return nil
}

// MigrateEdgeGatewayProtocol 把参考系统（things / iotddc）网关的 protocol 回填为 edge。
//
// 早期网关没有 protocol 字段，只靠 vendor/model 识别；后来新增了显式协议值。
// 统一落成 edge 后，前端按「参考系统网关」展示与操作不再依赖厂商字符串。
// 自有协议网关（gatewaymqtt）不受影响。
func MigrateEdgeGatewayProtocol(tx *gorm.DB) error {
	if !tx.Migrator().HasTable("gateway") {
		return nil
	}
	return tx.Exec(
		"UPDATE gateway SET protocol = ? WHERE (vendor LIKE ? OR vendor LIKE ? OR model LIKE ? OR model LIKE ?) AND (protocol = '' OR protocol = ?)",
		models.GatewayProtocolEdge, "%things%", "%iotddc%", "%things%", "%iotddc%", models.GatewayProtocolNative,
	).Error
}

// EnsureBuiltinDriverLibraries 种入内置驱动市场（MQTT / TCP / HTTP）。
//
// 早期这些记录是手工写库的，新部署从空库启动时驱动市场会一直显示「暂无数据」。
// 这里按固定 id 幂等插入，已有记录不覆盖（保留用户下载后更新的版本/镜像信息）。
func EnsureBuiltinDriverLibraries(tx *gorm.DB) error {
	if !tx.Migrator().HasTable("device_library") {
		return nil
	}
	now := time.Now().UnixMilli()
	if tx.Migrator().HasTable("driver_classify") {
		var classifyCount int64
		if err := tx.Table("driver_classify").Where("id = ?", "1").Count(&classifyCount).Error; err != nil {
			return err
		}
		if classifyCount == 0 {
			classify := models.DriverClassify{Id: "1", Name: "协议驱动"}
			classify.Created, classify.Modified = now, now
			if err := tx.Table("driver_classify").Create(&classify).Error; err != nil {
				return err
			}
		}
	}

	libs := []models.DeviceLibrary{
		{
			Id: "dl-mqtt-driver", Name: "MQTT", Protocol: "MQTT", Version: "1.0.1",
			Description:    "驱动内嵌 MQTT broker，设备直连上报 JSON 报文。支持属性/事件上报与平台下发命令转发。",
			ContainerName:  "driver-mqtt",
			DockerRepoName: "lmgateways/iotserver-mqtt-driver",
			DockerImageId:  "ghcr.io/lmgateways/iotserver-mqtt-driver:1.0.1",
			IsInternal:     true, Language: "Go",
			SupportVersions: models.SupportVersions{
				{Version: "1.0.0"},
				{Version: "1.0.1", IsDefault: true},
			},
		},
		{
			Id: "dl-tcp-driver", Name: "TCP", Protocol: "TCP", Version: "1.0.1",
			Description:    "驱动监听 TCP 端口，设备以「按行分隔的 JSON」接入：先发 {\"sn\":\"...\"} 握手，再发属性/事件帧。",
			ContainerName:  "driver-tcp",
			DockerRepoName: "lmgateways/iotserver-tcp-driver",
			DockerImageId:  "ghcr.io/lmgateways/iotserver-tcp-driver:1.0.1",
			IsInternal:     true, Language: "Go",
			SupportVersions: models.SupportVersions{
				{Version: "1.0.0"},
				{Version: "1.0.1", IsDefault: true},
			},
		},
		{
			Id: "dl-http-driver", Name: "HTTP", Protocol: "HTTP", Version: "1.0.1",
			Description:    "设备用 HTTP POST 上报数据（/ingest/{sn}），通过轮询接口取平台下发的命令。",
			ContainerName:  "driver-http",
			DockerRepoName: "lmgateways/iotserver-http-driver",
			IsInternal:     true, Language: "Go",
			SupportVersions: models.SupportVersions{
				{Version: "1.0.0"},
				{Version: "1.0.1", IsDefault: true},
			},
		},
	}
	for i := range libs {
		// 内嵌 DeviceMarket 的提升字段不能在字面量里赋值（Go 1.24 限制），这里统一补
		repo := strings.ToLower(libs[i].Name)
		libs[i].ClassifyId = 1
		libs[i].Manual = "https://github.com/LMgateways/iotserver-" + repo + "-driver#readme"
		libs[i].Icon = "https://raw.githubusercontent.com/LMgateways/iotserver-" + repo + "-driver/main/docs/icon.svg"
	}
	for _, lib := range libs {
		var count int64
		if err := tx.Table("device_library").
			Where("id = ? OR (name = ? AND protocol = ?)", lib.Id, lib.Name, lib.Protocol).
			Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			continue
		}
		lib.Created, lib.Modified = now, now
		if err := tx.Table("device_library").Create(&lib).Error; err != nil {
			return err
		}
	}
	return nil
}

// MigrateDriverNames 去掉驱动名称里的「协议驱动」后缀。
// 驱动名称本身就是协议名（MQTT / HTTP / TCP），带后缀既冗余又要额外做国际化；
// 同步处理驱动库与「我的驱动」里的实例名，存量数据启动时一次性修正。
func MigrateDriverNames(tx *gorm.DB) error {
	for _, table := range []string{"device_library", "device_service"} {
		if !tx.Migrator().HasTable(table) {
			continue
		}
		if err := tx.Exec(
			"UPDATE " + table + " SET name = REPLACE(REPLACE(name, ' 协议驱动', ''), '协议驱动', '') WHERE name LIKE '%协议驱动%'",
		).Error; err != nil {
			return err
		}
	}
	return nil
}

// MigrateSystemBoardPlatformOnly 系统监控看板只留给平台管理员：
//   - 删除租户作用域下由旧版本种入的「系统监控看板」；
//   - 平台看板里去掉「设备与告警」「消息量趋势」两个业务组件，
//     只保留 CPU / 内存 / 磁盘等系统运行情况。
func MigrateSystemBoardPlatformOnly(tx *gorm.DB) error {
	if !tx.Migrator().HasTable("viz_board") {
		return nil
	}
	if err := tx.Exec(
		"DELETE FROM viz_board WHERE tenant_id <> '' AND (name = ? OR name = ? OR id LIKE ? OR id LIKE ?)",
		"系统监控看板", "System Monitoring", "builtin-ops%", "bd-tenant-monitor%",
	).Error; err != nil {
		return err
	}
	var boards []models.VizBoard
	if err := tx.Table("viz_board").Where("tenant_id = '' AND name IN ?", []string{"系统监控看板", "System Monitoring"}).
		Find(&boards).Error; err != nil {
		return err
	}
	preset, hasPreset := BoardPresetById("builtin-ops")
	for _, b := range boards {
		// 旧版本把「租户现场运行看板」也叫系统监控看板：整块换成平台系统运行看板
		if strings.Contains(b.Canvas, `"bd-tenant-`) {
			if !hasPreset {
				continue
			}
			if err := tx.Table("viz_board").Where("id = ?", b.Id).
				Update("canvas", string(preset.Canvas)).Error; err != nil {
				return err
			}
			continue
		}
		next, changed := removeSystemBoardBusinessWidgets(b.Canvas)
		if !changed {
			continue
		}
		if err := tx.Table("viz_board").Where("id = ?", b.Id).
			Update("canvas", next).Error; err != nil {
			return err
		}
	}
	return nil
}

// removeSystemBoardBusinessWidgets 去掉系统监控看板里的业务组件（保留其它组件与布局）
func removeSystemBoardBusinessWidgets(canvas string) (string, bool) {
	var cv map[string]interface{}
	if json.Unmarshal([]byte(canvas), &cv) != nil {
		return canvas, false
	}
	raw, ok := cv["widgets"].([]interface{})
	if !ok {
		return canvas, false
	}
	kept := make([]interface{}, 0, len(raw))
	removed := false
	for _, item := range raw {
		w, ok := item.(map[string]interface{})
		if !ok {
			kept = append(kept, item)
			continue
		}
		id, _ := w["id"].(string)
		if id == "ops-business" || id == "ops-msg-trend" {
			removed = true
			continue
		}
		kept = append(kept, item)
	}
	if !removed {
		return canvas, false
	}
	cv["widgets"] = kept
	out, err := json.Marshal(cv)
	if err != nil {
		return canvas, false
	}
	return string(out), true
}

// UpgradeBuiltinBoards 把存量内置看板里的占位组件按最新内置定义升级。
// 只替换组件 id 一致、类型仍然是 text 的项，用户改过类型的组件不会被动。
// SQLite 迁移共用这一份实现，避免逻辑各改一遍后漂移。
func UpgradeBuiltinBoards(tx *gorm.DB) error {
	preset, ok := BoardPresetById("builtin-home")
	if !ok {
		return nil
	}
	var boards []models.VizBoard
	if err := tx.Table((&models.VizBoard{}).TableName()).Find(&boards).Error; err != nil {
		return err
	}
	for _, b := range boards {
		next, changed := UpgradeBoardWidgets(b.Canvas, preset.Canvas)
		if !changed {
			continue
		}
		if err := tx.Table((&models.VizBoard{}).TableName()).Where("id = ?", b.Id).
			Update("canvas", next).Error; err != nil {
			return err
		}
	}
	return nil
}

// MigrateBuiltinBoardNames 把内置看板名称统一为英文。
//
// 看板名称属于展示层数据，内置看板在中文界面也使用同一套英文名称，
// 避免中英切换时名称与内容风格不一致。只迁移历史中文名；用户改过的名称不覆盖。
func MigrateBuiltinBoardNames(tx *gorm.DB) error {
	if !tx.Migrator().HasTable("viz_board") {
		return nil
	}
	renames := []struct {
		name       string
		conditions string
		args       []interface{}
	}{
		{"Component Demo", "name = ?", []interface{}{"组件演示看板"}},
		{"Home Board", "name = ?", []interface{}{"首页看板"}},
		{"System Monitoring", "name = ?", []interface{}{"系统监控看板"}},
	}
	for _, item := range renames {
		if err := tx.Table("viz_board").Where(item.conditions, item.args...).
			Update("name", item.name).Error; err != nil {
			return err
		}
	}
	return nil
}

// UpgradeBuiltinBoardTexts 给存量内置演示看板的文本组件补英文正文（textEn）。
//
// 只处理组件 id 与文本都未改过的内置内容；用户自己改过的正文不覆盖。
func UpgradeBuiltinBoardTexts(tx *gorm.DB) error {
	if !tx.Migrator().HasTable("viz_board") {
		return nil
	}
	preset, ok := BoardPresetById("builtin-demo")
	if !ok {
		return nil
	}
	var pcv map[string]interface{}
	if err := json.Unmarshal(preset.Canvas, &pcv); err != nil {
		return nil
	}
	type pair struct{ text, textEn string }
	want := map[string]pair{}
	var collect func(list []interface{})
	collect = func(list []interface{}) {
		for _, item := range list {
			w, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			if id, _ := w["id"].(string); id != "" {
				text, _ := w["text"].(string)
				textEn, _ := w["textEn"].(string)
				if text != "" && textEn != "" {
					want[id] = pair{text: text, textEn: textEn}
				}
			}
			if children, ok := w["children"].([]interface{}); ok {
				collect(children)
			}
		}
	}
	if widgets, ok := pcv["widgets"].([]interface{}); ok {
		collect(widgets)
	}
	if len(want) == 0 {
		return nil
	}

	var boards []models.VizBoard
	if err := tx.Table("viz_board").Where("id LIKE ?", "builtin-demo%").
		Find(&boards).Error; err != nil {
		return err
	}
	for _, b := range boards {
		var cv map[string]interface{}
		if err := json.Unmarshal([]byte(b.Canvas), &cv); err != nil {
			continue
		}
		changed := false
		var walk func(list []interface{})
		walk = func(list []interface{}) {
			for _, item := range list {
				w, ok := item.(map[string]interface{})
				if !ok {
					continue
				}
				id, _ := w["id"].(string)
				if p, ok := want[id]; ok {
					text, _ := w["text"].(string)
					textEn, _ := w["textEn"].(string)
					if text == p.text && textEn == "" {
						w["textEn"] = p.textEn
						changed = true
					}
				}
				if children, ok := w["children"].([]interface{}); ok {
					walk(children)
				}
			}
		}
		if widgets, ok := cv["widgets"].([]interface{}); ok {
			walk(widgets)
		}
		if !changed {
			continue
		}
		raw, err := json.Marshal(cv)
		if err != nil {
			continue
		}
		if err := tx.Table("viz_board").Where("id = ?", b.Id).
			Update("canvas", string(raw)).Error; err != nil {
			return err
		}
	}
	return nil
}

// EnsureTenantBuiltinBoards 给存量租户补齐内置「组件演示看板」。
//
// 新版本普通租户与平台一样自带首页 + 组件演示两块看板；旧版本只种了首页，
// 升级时在这里补齐。只在该租户还没有这块看板时插入，避免重复。
// 租户已有别的默认看板时，补入的这块不再标记为默认，保持「每租户一个默认看板」。
func EnsureTenantBuiltinBoards(tx *gorm.DB) error {
	if !tx.Migrator().HasTable("viz_board") || !tx.Migrator().HasTable("tenant") {
		return nil
	}
	preset, ok := BoardPresetById("builtin-demo")
	if !ok {
		return nil
	}
	var tenantIds []string
	if err := tx.Table("tenant").Pluck("id", &tenantIds).Error; err != nil {
		return err
	}
	now := time.Now().UnixMilli()
	// 平台自身的看板由首次访问时的种入逻辑统一处理；这里只补存量租户，
	// 避免提前插入一块「默认看板」导致平台无法再种入完整的一套（首页/演示/监控）。
	for _, tenantId := range tenantIds {
		boardId := preset.Id + "-" + tenantId
		// 按 id 或名称判重：看板管理里手工建的同名看板也算已有，避免升级后出现两块
		var count int64
		if err := tx.Table("viz_board").
			Where("id = ? OR (tenant_id = ? AND name = ?)", boardId, tenantId, preset.Name).
			Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			continue
		}
		var defaults int64
		if err := tx.Table("viz_board").Where("tenant_id = ? AND is_default = 1", tenantId).
			Count(&defaults).Error; err != nil {
			return err
		}
		board := models.VizBoard{
			Id:       boardId,
			Name:     preset.Name,
			Icon:     preset.Icon,
			Canvas:   string(preset.Canvas),
			UpdateAt: now,
		}
		board.TenantId = tenantId
		board.Created, board.Modified = now, now
		if defaults == 0 {
			board.IsDefault = 1
		}
		if err := tx.Table("viz_board").Create(&board).Error; err != nil {
			return err
		}
	}
	return nil
}

// MigrateAlertNotifyWays 清理已下线的通知渠道。
//
// 企业微信 / 钉钉 / 飞书 / 语音告警已移除；告警规则 JSON 里如果还保留这些
// 配置，编辑弹窗会凭空多出一条无法渲染的通知。启动时直接删掉，只保留
// 邮件、短信、WhatsApp 与 Webhook。
func MigrateAlertNotifyWays(tx *gorm.DB) error {
	if !tx.Migrator().HasTable("alert_rule") {
		return nil
	}
	allowed := map[string]bool{
		string(constants.Email):       true,
		string(constants.TwilioEmail): true,
		string(constants.SMS):         true,
		string(constants.WhatsApp):    true,
		string(constants.WEBAPI):      true,
	}
	var rows []struct {
		Id     string
		Notify string
	}
	if err := tx.Table("alert_rule").Select("id, notify").Find(&rows).Error; err != nil {
		return err
	}
	for _, row := range rows {
		if strings.TrimSpace(row.Notify) == "" {
			continue
		}
		var list models.Notify
		if err := json.Unmarshal([]byte(row.Notify), &list); err != nil {
			continue
		}
		kept := make(models.Notify, 0, len(list))
		changed := false
		for _, item := range list {
			if item.Name == constants.TwilioEmail {
				item.Name = constants.Email
				if item.Option == nil {
					item.Option = models.MapStringString{}
				}
				item.Option["provider"] = "twilio"
				changed = true
			} else if item.Name == constants.Email && item.Option != nil && item.Option["provider"] == "" {
				item.Option["provider"] = "smtp"
				changed = true
			}
			if allowed[string(item.Name)] {
				kept = append(kept, item)
			} else {
				changed = true
			}
		}
		if !changed && len(kept) == len(list) {
			continue
		}
		payload, err := json.Marshal(kept)
		if err != nil {
			return err
		}
		if err := tx.Table("alert_rule").Where("id = ?", row.Id).
			Update("notify", string(payload)).Error; err != nil {
			return err
		}
	}
	return nil
}
