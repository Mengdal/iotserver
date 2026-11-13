package seed

import (
	"time"

	"gorm.io/gorm"

	"iotserver/internal/models"
)

// 内置系统组件 ID。
const (
	SystemComponentMosquittoId = "sys-mosquitto"
	SystemComponentEKuiperId   = "sys-ekuiper"
	SystemComponentIotEdgeDBId = "sys-iotedgedb"
	SystemComponentIotpGWId    = "sys-iotpgw"
)

// 驱动市场分类：系统组件。
// 系统组件和协议驱动一样出现在驱动市场里，用分类区分，不另开菜单/页签。
const (
	SystemComponentClassifyId   = "2"
	SystemComponentClassifyID   = 2
	SystemComponentClassifyName = "系统组件"
)

// MigrateDriverPluginFields 把存量驱动数据补齐到插件模型：
//   - 驱动库回填 category/runtime/arch/source
//   - 驱动实例回填 instance_name/runtime
//   - 去掉「一个驱动库只能一个实例」的唯一索引，改成 (库, 实例名) 唯一
//
// 幂等：字段为空才回填，索引先删后建。
func MigrateDriverPluginFields(tx *gorm.DB) error {
	if !tx.Migrator().HasTable("device_library") {
		return nil
	}
	if err := tx.Exec(
		"UPDATE device_library SET category = ? WHERE COALESCE(category, '') = ''",
		models.PluginCategoryDriver,
	).Error; err != nil {
		return err
	}
	if err := tx.Exec(
		"UPDATE device_library SET runtime = ? WHERE COALESCE(runtime, '') = ''",
		models.PluginRuntimeDocker,
	).Error; err != nil {
		return err
	}
	if err := tx.Exec(
		"UPDATE device_library SET arch = 'amd64' WHERE COALESCE(arch, '') = ''",
	).Error; err != nil {
		return err
	}
	if err := tx.Exec(
		"UPDATE device_library SET source = CASE WHEN is_internal = 1 THEN 'builtin' ELSE 'market' END " +
			"WHERE COALESCE(source, '') = ''",
	).Error; err != nil {
		return err
	}
	// AutoMigrate 给存量行填过默认值 'market'，内置驱动需要纠正回 builtin。
	if err := tx.Exec(
		"UPDATE device_library SET source = 'builtin' WHERE is_internal = 1 AND source = 'market'",
	).Error; err != nil {
		return err
	}

	if !tx.Migrator().HasTable("device_service") {
		return nil
	}
	// 旧版本的实例没有单独名称，先用展示名或 ID 补齐。
	if err := tx.Exec(
		"UPDATE device_service SET instance_name = COALESCE(NULLIF(name, ''), id) " +
			"WHERE COALESCE(instance_name, '') = ''",
	).Error; err != nil {
		return err
	}
	if err := tx.Exec(
		"UPDATE device_service SET runtime = ? WHERE COALESCE(runtime, '') = ''",
		models.PluginRuntimeDocker,
	).Error; err != nil {
		return err
	}

	// 旧唯一索引限制「一个驱动库只能一个实例」，多实例需要去掉。
	if err := tx.Exec("DROP INDEX IF EXISTS idx_device_service_device_library_id").Error; err != nil {
		return err
	}
	var duplicates int64
	if err := tx.Raw(
		"SELECT COUNT(*) FROM (" +
			"SELECT device_library_id, instance_name FROM device_service " +
			"GROUP BY device_library_id, instance_name HAVING COUNT(*) > 1" +
			")",
	).Scan(&duplicates).Error; err != nil {
		return err
	}
	if duplicates == 0 {
		return tx.Exec(
			"CREATE UNIQUE INDEX IF NOT EXISTS uk_device_service_library_instance " +
				"ON device_service(device_library_id, instance_name)",
		).Error
	}
	// 存量数据已经重名时先建普通索引，避免升级失败；后续由实例管理接口保证唯一。
	return tx.Exec(
		"CREATE INDEX IF NOT EXISTS idx_device_service_library_instance " +
			"ON device_service(device_library_id, instance_name)",
	).Error
}

// EnsureSystemComponents 种入内置系统组件（Mosquitto / eKuiper / IotEdgeDB）。
// 已存在时保持用户配置不变。
func EnsureSystemComponents(tx *gorm.DB) error {
	if !tx.Migrator().HasTable("device_library") {
		return nil
	}
	now := time.Now().UnixMilli()
	// 驱动市场的「内置组件」分类：系统组件通过该分类直接出现在驱动市场列表里。
	if tx.Migrator().HasTable("driver_classify") {
		var count int64
		if err := tx.Table("driver_classify").
			Where("id = ?", SystemComponentClassifyId).Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			classify := models.DriverClassify{Id: SystemComponentClassifyId, Name: SystemComponentClassifyName}
			classify.Timestamps = models.Timestamps{Created: now, Modified: now}
			if err := tx.Table("driver_classify").Create(&classify).Error; err != nil {
				return err
			}
		} else if err := tx.Table("driver_classify").Where("id = ?", SystemComponentClassifyId).
			Update("name", SystemComponentClassifyName).Error; err != nil {
			return err
		}
	}
	// 已存在的系统组件补齐分类，旧数据也能出现在市场里。
	if err := tx.Table("device_library").
		Where("id IN ? AND (classify_id IS NULL OR classify_id = 0)",
			[]string{SystemComponentMosquittoId, SystemComponentEKuiperId, SystemComponentIotEdgeDBId, SystemComponentIotpGWId}).
		Update("classify_id", SystemComponentClassifyID).Error; err != nil {
		return err
	}
	components := []models.DeviceLibrary{
		{
			Id:              SystemComponentMosquittoId,
			Name:            "Mosquitto",
			Description:     "内置 MQTT 消息总线，为规则引擎、告警和事件链路提供本机消息通道。",
			Protocol:        "MQTT",
			Version:         "builtin",
			SupportVersions: models.SupportVersions{},
			IsInternal:      true,
			Category:        models.PluginCategorySystem,
			Runtime:         models.PluginRuntimeBuiltin,
			Arch:            "any",
			Essential:       false,
			Source:          "builtin",
			DeviceMarket:    models.DeviceMarket{ClassifyId: SystemComponentClassifyID, Icon: "/assets/mosquitto.png", Manual: "https://mosquitto.org/"},
		},
		{
			Id:              SystemComponentEKuiperId,
			Name:            "eKuiper",
			Description:     "内置流处理引擎，订阅本机 MQTT 消息并按规则做窗口计算、过滤和告警触发。",
			Protocol:        "MQTT",
			Version:         "builtin",
			SupportVersions: models.SupportVersions{},
			IsInternal:      true,
			Category:        models.PluginCategorySystem,
			Runtime:         models.PluginRuntimeBuiltin,
			Arch:            "any",
			Essential:       false,
			Source:          "builtin",
			DeviceMarket:    models.DeviceMarket{ClassifyId: SystemComponentClassifyID, Icon: "/assets/ekuiper.png", Manual: "https://ekuiper.org/"},
		},
		{
			Id:              SystemComponentIotEdgeDBId,
			Name:            "IotEdgeDB",
			Description:     "外部时序数据库（IotEdgeDB），通过 [Databases.Data] 的 BaseURL 接入，负责采集数据的持久化与查询。",
			Protocol:        "HTTP",
			Version:         "external",
			SupportVersions: models.SupportVersions{},
			IsInternal:      true,
			Category:        models.PluginCategorySystem,
			Runtime:         models.PluginRuntimeBuiltin,
			Arch:            "any",
			Essential:       false,
			Source:          "builtin",
			DeviceMarket: models.DeviceMarket{
				ClassifyId: SystemComponentClassifyID,
				Icon:       "/assets/iotedgedb.png",
				Manual:     "http://docs.lmgateway.com/",
			},
		},
		{
			Id:              SystemComponentIotpGWId,
			Name:            "iotpGW",
			Description:     "软网关采集引擎，负责通道、设备与点位的采集，并把数据写入 iotp 时序库。",
			Protocol:        "HTTP",
			Version:         "builtin",
			SupportVersions: models.SupportVersions{},
			IsInternal:      true,
			Category:        models.PluginCategorySystem,
			Runtime:         models.PluginRuntimeBuiltin,
			Arch:            "any",
			Essential:       false,
			Source:          "builtin",
			DeviceMarket:    models.DeviceMarket{ClassifyId: SystemComponentClassifyID, Icon: "/assets/iotpgw.png"},
		},
	}
	for i := range components {
		var count int64
		if err := tx.Table("device_library").
			Where("id = ?", components[i].Id).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			// 内置组件的图标/手册地址跟随代码更新。
			if err := tx.Table("device_library").Where("id = ?", components[i].Id).
				Updates(map[string]interface{}{
					"icon":      components[i].Icon,
					"manual":    components[i].Manual,
					"essential": components[i].Essential,
				}).Error; err != nil {
				return err
			}
			continue
		}
		components[i].Timestamps = models.Timestamps{Created: now, Modified: now}
		if err := tx.Table("device_library").Create(&components[i]).Error; err != nil {
			return err
		}
	}
	return nil
}
