package seed

import (
	"encoding/json"
	"strings"

	"gorm.io/gorm"

	"iotserver/internal/models"
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
		"DELETE FROM viz_board WHERE tenant_id <> '' AND (name = ? OR id LIKE ? OR id LIKE ?)",
		"系统监控看板", "builtin-ops%", "bd-tenant-monitor%",
	).Error; err != nil {
		return err
	}
	var boards []models.VizBoard
	if err := tx.Table("viz_board").Where("tenant_id = '' AND name = ?", "系统监控看板").
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
// SQLite / MySQL 两套迁移共用这一份实现，避免逻辑各改一遍后漂移。
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
