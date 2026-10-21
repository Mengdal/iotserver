/*******************************************************************************
 * Copyright 2017.
 *
 * Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software distributed under the License
 * is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express
 * or implied. See the License for the specific language governing permissions and limitations under
 * the License.
 *******************************************************************************/

package sqlite

import (
	"context"
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"iotserver/internal/models"
	"iotserver/internal/pkg/seed"
	"iotserver/internal/pkg/tenant"
)

// DefaultAdminPassword 内置管理员初始密码
const DefaultAdminPassword = "admin123"

// MigrateSystemTables 自动创建系统管理相关表并写入初始化数据
func (c *Client) MigrateSystemTables() error {
	// 审查日志表改名必须在 AutoMigrate 之前：否则新表会先被建出来，
	// 旧表 sys_login_log 的历史数据就搬不过去了。
	c.migrateAuditLogTable()

	// 建表清单：系统管理 + 设备接入 + 运维监控 + 工具与文档 + 高级能力。
	// 注意：全新库必须在这里建齐所有业务表——早期版本把业务表放在「连接时 InitTable」里，
	// 那段代码被注释后新建库会缺表（产品/设备/驱动/物模型/告警等接口报 no such table）。
	if err := c.Pool.AutoMigrate(
		// ===== 系统管理 =====
		&models.User{},
		&models.Role{},
		&models.Menu{},
		&models.Dept{},
		&models.UserRole{},
		&models.RoleMenu{},
		&models.AuditLog{},
		&models.SystemConfig{},
		// 租户菜单展示层覆盖（隐藏/改名/图标/排序）
		&models.TenantMenuPref{},
		// ===== 设备接入 =====
		&models.DeviceLibrary{},
		&models.DeviceService{},
		&models.Device{},
		&models.DockerConfig{},
		&models.CategoryTemplate{},
		&models.ThingModelTemplate{},
		&models.DriverClassify{},
		&models.Product{},
		&models.Properties{},
		&models.Actions{},
		&models.Events{},
		&models.Unit{},
		&models.LanguageSdk{},
		&models.MqttAuth{},
		// ===== 运维监控（监控指标 + 告警）=====
		&models.Metrics{},
		&models.SystemMetrics{},
		&models.ServiceStats{},
		&models.AlertRule{},
		&models.AlertList{},
		&models.AlertContent{},
		// ===== 工具与文档 / 首页 =====
		&models.QuickNavigation{},
		&models.Doc{},
		&models.MsgGather{},
		// ===== 高级能力：资源管理 / 规则引擎 / 场景联动 =====
		&models.DataResource{},
		&models.RuleEngine{},
		&models.Scene{},
		&models.SceneLog{},
		// ===== 多租户 =====
		&models.Tenant{},
		// ===== 网关接入（第三方工业网关 MQTT 上行）=====
		&models.Gateway{},
		&models.GatewayIngestLog{},
		// 参考系统（/edge 协议）接入设备的设备级测点（无物模型时的读取侧来源）
		&models.DeviceTag{},
		// ===== 行业方案 =====
		&models.Solution{},
		// ===== 画布看板（落库）=====
		&models.VizBoard{},
	); err != nil {
		return err
	}
	if err := c.migrateTenantColumns(); err != nil {
		return err
	}
	if err := c.migrateAuditLogColumns(); err != nil {
		return err
	}
	if err := c.ensureUniqueIndexes(); err != nil {
		return err
	}
	if err := c.ensurePartialUniqueIndexes(); err != nil {
		return err
	}
	return c.seedSystemData()
}

// ensurePartialUniqueIndexes 建「只对部分行成立」的唯一索引。
//
// 为什么不能放进 ensureUniqueIndexes：那里的索引对整表成立。
// device 表的 (gateway_id, device_key) 只对**网关接入的设备**唯一——
// 平台手工创建与驱动接入的设备这两列都是空串，普通唯一索引会立刻冲突。
//
// SQLite 支持带 WHERE 的部分索引；MySQL 不支持（其版本见 mysql 包，为显式空实现）。
func (c *Client) ensurePartialUniqueIndexes() error {
	pool := c.Pool.WithContext(tenant.Platform(context.Background()))
	const (
		name  = "uk_device_gateway_key"
		table = models.TableDevice
	)
	if pool.Migrator().HasIndex(table, name) {
		return nil
	}
	// 先探测重复：直接建索引会在历史脏数据上失败，导致服务起不来（与 ensureUniqueIndexes 同理）
	var dupCount int64
	if err := pool.Table(table).
		Select("COUNT(*)").
		Where("gateway_id <> '' AND device_key <> ''").
		Group("gateway_id, device_key").
		Having("COUNT(*) > 1").
		Count(&dupCount).Error; err != nil {
		return fmt.Errorf("check duplicate of %s failed: %w", table, err)
	}
	if dupCount > 0 {
		c.loggingClient.Warnf("table %s has %d duplicated (gateway_id, device_key), skip creating %s",
			table, dupCount, name)
		return nil
	}
	stmt := fmt.Sprintf(
		"CREATE UNIQUE INDEX %s ON %s(gateway_id, device_key) WHERE gateway_id <> '' AND device_key <> ''",
		name, table)
	if err := pool.Exec(stmt).Error; err != nil {
		c.loggingClient.Warnf("create partial unique index %s on %s failed: %v", name, table, err)
	}
	return nil
}

// migrateTenantColumns 多租户改造的数据迁移：
//  1. 历史数据的 tenant_id 回填为平台级（空串）——租户列是后加的，老数据都属于平台/单一部署；
//  2. 把角色名的全局唯一索引改成 (tenant_id, name) 唯一，使不同租户可以有同名角色。
func (c *Client) migrateTenantColumns() error {
	// 表清单直接取自「租户表登记」，避免与 models 里的定义漂移
	for t := range models.TenantScopedTableNames() {
		if !c.Pool.Migrator().HasTable(t) || !c.Pool.Migrator().HasColumn(t, "tenant_id") {
			continue
		}
		if err := c.Pool.Exec(fmt.Sprintf(
			"UPDATE %s SET tenant_id = '' WHERE tenant_id IS NULL", t)).Error; err != nil {
			c.loggingClient.Warnf("backfill tenant_id of %s failed: %v", t, err)
		}
	}

	// 角色名唯一索引：全局唯一 → 租户内唯一
	if c.Pool.Migrator().HasIndex("sys_role", "idx_sys_role_name") {
		if err := c.Pool.Exec("DROP INDEX IF EXISTS idx_sys_role_name").Error; err != nil {
			c.loggingClient.Warnf("drop legacy unique index idx_sys_role_name failed: %v", err)
		}
	}
	if !c.Pool.Migrator().HasIndex("sys_role", "uk_sys_role_tenant_name") {
		stmt := "CREATE UNIQUE INDEX uk_sys_role_tenant_name ON sys_role(tenant_id, name)"
		if err := c.Pool.Exec(stmt).Error; err != nil {
			c.loggingClient.Warnf("create unique index uk_sys_role_tenant_name failed: %v", err)
		}
	}

	// 权限字符同样只在租户内唯一（不同租户可以都有 admin / factory_ops）
	if !c.Pool.Migrator().HasIndex("sys_role", "uk_sys_role_tenant_key") {
		stmt := "CREATE UNIQUE INDEX uk_sys_role_tenant_key ON sys_role(tenant_id, role_key)"
		if err := c.Pool.Exec(stmt).Error; err != nil {
			c.loggingClient.Warnf("create unique index uk_sys_role_tenant_key failed: %v", err)
		}
	}

	// 菜单归属列：历史菜单全部属于平台（tenant_id=''），回填后才能按「全局 + 租户」过滤。
	// sys_menu 没有登记进 tenantScopedModels，所以上面的统一 backfill 不会覆盖它。
	if c.Pool.Migrator().HasTable(models.TableMenu) {
		if !c.Pool.Migrator().HasColumn(&models.Menu{}, "tenant_id") {
			if err := c.Pool.Migrator().AddColumn(&models.Menu{}, "tenant_id"); err != nil {
				c.loggingClient.Warnf("add sys_menu.tenant_id failed: %v", err)
			}
		}
		if err := c.Pool.Exec(
			"UPDATE sys_menu SET tenant_id = '' WHERE tenant_id IS NULL").Error; err != nil {
			c.loggingClient.Warnf("backfill tenant_id of sys_menu failed: %v", err)
		}
	}

	// 租户菜单覆盖：同一租户同一菜单只允许一条
	if c.Pool.Migrator().HasTable(models.TableTenantMenuPref) &&
		!c.Pool.Migrator().HasIndex(models.TableTenantMenuPref, "uk_tenant_menu_pref") {
		stmt := "CREATE UNIQUE INDEX uk_tenant_menu_pref ON tenant_menu_pref(tenant_id, menu_id)"
		if err := c.Pool.Exec(stmt).Error; err != nil {
			c.loggingClient.Warnf("create unique index uk_tenant_menu_pref failed: %v", err)
		}
	}
	return nil
}

// migrateAuditLogTable 审查日志表改名（幂等）：
// 只有「旧表在、新表不在」时才执行 sys_login_log → sys_audit_log，绝不重复改名或丢数据。
func (c *Client) migrateAuditLogTable() {
	if !c.Pool.Migrator().HasTable("sys_login_log") || c.Pool.Migrator().HasTable(models.TableAuditLog) {
		return
	}
	if err := c.Pool.Exec("ALTER TABLE sys_login_log RENAME TO sys_audit_log").Error; err != nil {
		if c.loggingClient != nil {
			c.loggingClient.Warnf("rename sys_login_log to sys_audit_log failed: %v", err)
		}
		return
	}
	if c.loggingClient != nil {
		c.loggingClient.Infof("审查日志: 表 sys_login_log 已改名为 sys_audit_log（保留全部历史数据）")
	}
}

// migrateAuditLogColumns 审查日志补列后的数据回填与索引（幂等）：
//  1. 历史登录记录回填 type='login'、module='认证'；
//  2. created 缺失的历史记录用 login_time 兜底（避免被保留策略误判为过期）；
//  3. 建 (tenant_id,created)、(tenant_id,type,created)、(username)、(status) 索引。
func (c *Client) migrateAuditLogColumns() error {
	if !c.Pool.Migrator().HasTable(models.TableAuditLog) {
		return nil
	}
	if err := c.Pool.Exec("UPDATE sys_audit_log SET type = ? WHERE type IS NULL OR type = ''",
		models.AuditTypeLogin).Error; err != nil {
		c.loggingClient.Warnf("backfill audit log type failed: %v", err)
	}
	if err := c.Pool.Exec("UPDATE sys_audit_log SET module = ? WHERE module IS NULL OR module = ''",
		models.AuditModuleAuth).Error; err != nil {
		c.loggingClient.Warnf("backfill audit log module failed: %v", err)
	}
	if err := c.Pool.Exec(
		"UPDATE sys_audit_log SET created = login_time WHERE (created IS NULL OR created = 0) AND login_time > 0").Error; err != nil {
		c.loggingClient.Warnf("backfill audit log created failed: %v", err)
	}

	// alternates：模型 index 标签在 AutoMigrate 时生成的等价索引名，存在即跳过，避免重复建索引
	indexes := []struct {
		name       string
		columns    string
		alternates []string
	}{
		{"idx_audit_log_tenant_created", "tenant_id, created", nil},
		{"idx_audit_log_tenant_type_created", "tenant_id, type, created", nil},
		{"idx_audit_log_username", "username",
			[]string{"idx_sys_audit_log_username", "idx_sys_login_log_username"}},
		{"idx_audit_log_status", "status",
			[]string{"idx_sys_audit_log_status", "idx_sys_login_log_status"}},
	}
	for _, idx := range indexes {
		exists := c.Pool.Migrator().HasIndex(models.TableAuditLog, idx.name)
		for _, alt := range idx.alternates {
			exists = exists || c.Pool.Migrator().HasIndex(models.TableAuditLog, alt)
		}
		if exists {
			continue
		}
		stmt := fmt.Sprintf("CREATE INDEX %s ON sys_audit_log(%s)", idx.name, idx.columns)
		if err := c.Pool.Exec(stmt).Error; err != nil {
			c.loggingClient.Warnf("create index %s on sys_audit_log failed: %v", idx.name, err)
		}
	}
	return nil
}

func (c *Client) seedSystemData() error {
	lc := c.loggingClient

	// 平台作用域：初始化/种子数据属于平台级数据（tenant_id 为空），
	// 必须显式声明平台作用域，否则在 enforce 策略下写租户表会被拒绝。
	if err := c.Pool.WithContext(tenant.Platform(context.Background())).Transaction(func(tx *gorm.DB) error {
		now := time.Now().UnixMilli()

		/******* 部门 *******/
		// 内置根部门的名称跟随代码刷新（用户自建部门不受影响）
		for _, dept := range seed.DefaultDepts() {
			var count int64
			if err := tx.Table("sys_dept").Where("id = ?", dept.Id).Count(&count).Error; err != nil {
				return err
			}
			if count > 0 {
				if err := tx.Table("sys_dept").Where("id = ?", dept.Id).
					Updates(map[string]interface{}{"name": dept.Name, "modified": now}).Error; err != nil {
					return err
				}
				continue
			}
			dept.Timestamps = models.Timestamps{Created: now, Modified: now}
			if err := tx.Table("sys_dept").Create(&dept).Error; err != nil {
				return err
			}
		}

		/******* 菜单：先清理遗留菜单，再按 ID 插入或更新（保证权限标识与路径始终跟随代码） *******/
		// 只处理平台菜单（tenant_id=''）：租户自建菜单不属于 seed 的管理范围，
		// 否则启动一次就会把租户的菜单当「遗留菜单」删掉。
		defaultMenus := seed.DefaultMenus()
		seededMenuIds := make(map[string]struct{}, len(defaultMenus))
		for _, menu := range defaultMenus {
			seededMenuIds[menu.Id] = struct{}{}
		}
		var existingMenus []models.Menu
		if err := tx.Table("sys_menu").Where("tenant_id = ''").Find(&existingMenus).Error; err != nil {
			return err
		}
		staleIds := make([]string, 0)
		for _, exist := range existingMenus {
			if _, ok := seededMenuIds[exist.Id]; !ok {
				staleIds = append(staleIds, exist.Id)
			}
		}
		if len(staleIds) > 0 {
			if err := tx.Table("sys_role_menu").Where("menu_id IN ?", staleIds).Delete(&models.RoleMenu{}).Error; err != nil {
				return err
			}
			if err := tx.Table("sys_menu").Where("id IN ? AND tenant_id = ''", staleIds).Delete(&models.Menu{}).Error; err != nil {
				return err
			}
			if lc != nil {
				lc.Infof("清理遗留菜单 %d 个", len(staleIds))
			}
		}
		for _, menu := range defaultMenus {
			var count int64
			if err := tx.Table("sys_menu").Where("id = ?", menu.Id).Count(&count).Error; err != nil {
				return err
			}
			if count > 0 {
				// 已存在则更新可维护字段（parent/name/path/icon/type/perms/sort/visible/status）
				// tenant_id 固定回平台：seed 的 ID 都是平台菜单，顺带纠正历史脏数据；
				// 租户的展示层覆盖在独立的 tenant_menu_pref 表里，不会被子 seed 覆盖。
				updates := map[string]interface{}{
					"tenant_id":     "",
					"parent_id":     menu.ParentId,
					"name":          menu.Name,
					"path":          menu.Path,
					"icon":          menu.Icon,
					"menu_type":     menu.MenuType,
					"perms":         menu.Perms,
					"sort":          menu.Sort,
					"visible":       menu.Visible,
					"status":        menu.Status,
					"is_frame":      menu.IsFrame,
					"platform_only": menu.PlatformOnly,
					"modified":      now,
				}
				if err := tx.Table("sys_menu").Where("id = ?", menu.Id).Updates(updates).Error; err != nil {
					return err
				}
				continue
			}
			menu.Timestamps = models.Timestamps{Created: now, Modified: now}
			if err := tx.Table("sys_menu").Create(&menu).Error; err != nil {
				return err
			}
		}

		/******* 角色 *******/
		for _, role := range seed.DefaultRoles() {
			var count int64
			if err := tx.Table("sys_role").Where("id = ?", role.Id).Count(&count).Error; err != nil {
				return err
			}
			if count > 0 {
				continue
			}
			role.Timestamps = models.Timestamps{Created: now, Modified: now}
			if err := tx.Table("sys_role").Create(&role).Error; err != nil {
				return err
			}
		}

		/******* 超管角色绑定全部菜单（只绑平台菜单，租户自建菜单由租户自己授权） *******/
		var menuIds []string
		if err := tx.Table("sys_menu").Where("tenant_id = ''").Pluck("id", &menuIds).Error; err != nil {
			return err
		}
		for _, menuId := range menuIds {
			var count int64
			if err := tx.Table("sys_role_menu").
				Where("role_id = ? AND menu_id = ?", seed.RoleAdminId, menuId).Count(&count).Error; err != nil {
				return err
			}
			if count > 0 {
				continue
			}
			if err := tx.Table("sys_role_menu").
				Create(&models.RoleMenu{RoleId: seed.RoleAdminId, MenuId: menuId}).Error; err != nil {
				return err
			}
		}

		/******* 系统配置 *******/
		// 清理历史遗留的「无代码消费」配置项
		if err := tx.Table("sys_config").Where("config_key IN ?", seed.UnusedConfigKeys()).
			Delete(&models.SystemConfig{}).Error; err != nil {
			return err
		}
		for _, item := range seed.DefaultConfigs() {
			var count int64
			if err := tx.Table("sys_config").
				Where("config_key = ?", item.Key).Count(&count).Error; err != nil {
				return err
			}
			if count > 0 {
				// 名称/分组/类型跟随代码；配置值一律保留用户修改后的值
				updates := map[string]interface{}{
					"name":       item.Name,
					"group_name": item.Group,
					"type":       item.Type,
					"sort":       item.Sort,
					"modified":   now,
				}
				if err := tx.Table("sys_config").Where("config_key = ?", item.Key).Updates(updates).Error; err != nil {
					return err
				}
				continue
			}
			cfg := models.SystemConfig{
				Id:          "600000000000000" + item.Key,
				ConfigKey:   item.Key,
				ConfigValue: item.Value,
				Name:        item.Name,
				GroupName:   item.Group,
				Type:        item.Type,
				Sort:        item.Sort,
				Timestamps:  models.Timestamps{Created: now, Modified: now},
			}
			if err := tx.Table("sys_config").Create(&cfg).Error; err != nil {
				return err
			}
		}

		/******* 存量平台值兼容 *******/
		// 参考系统平台的取值从历史名称改为 things platform，存量产品/设备要一起迁移，
		// 否则按平台筛选与驱动同步都会查不到旧数据。
		if err := seed.MigrateLegacyPlatform(tx); err != nil {
			return err
		}
		// 设备状态改为稳定代码（online/offline/...），存量中文值一并迁移
		if err := seed.MigrateLegacyDeviceStatus(tx); err != nil {
			return err
		}
		// 驱动名称去掉「协议驱动」后缀（驱动库 + 我的驱动实例名）
		if err := seed.MigrateDriverNames(tx); err != nil {
			return err
		}
		// 内置驱动市场（MQTT / TCP / HTTP）：空库启动时补种，避免「暂无数据」
		if err := seed.EnsureBuiltinDriverLibraries(tx); err != nil {
			return err
		}
		// 参考系统 /edge 网关的 protocol 落到 edge（历史数据回填）
		if err := seed.MigrateEdgeGatewayProtocol(tx); err != nil {
			return err
		}
		// 系统监控看板只保留给平台管理员，并去掉其中的业务组件
		if err := seed.MigrateSystemBoardPlatformOnly(tx); err != nil {
			return err
		}

		/******* 内置看板升级 *******/
		// 存量看板里的占位组件（如首页看板的「设备消息量排名」文本）按内置定义升级成真实组件；
		// 只替换组件 id 一致、类型仍然是 text 的项，用户改过类型的组件不会被动。
		if err := seed.UpgradeBuiltinBoards(tx); err != nil {
			return err
		}
		// 内置看板名称统一英文；存量演示看板补英文正文
		if err := seed.MigrateBuiltinBoardNames(tx); err != nil {
			return err
		}
		if err := seed.UpgradeBuiltinBoardTexts(tx); err != nil {
			return err
		}
		// 存量租户补齐组件演示看板（新版本普通租户内置首页 + 演示两块）
		if err := seed.EnsureTenantBuiltinBoards(tx); err != nil {
			return err
		}

		/******* 内置管理员 *******/
		var adminCount int64
		if err := tx.Table("user").Where("username = ?", "admin").Count(&adminCount).Error; err != nil {
			return err
		}
		var adminId string
		if adminCount == 0 {
			hash, err := bcrypt.GenerateFromPassword([]byte(DefaultAdminPassword), bcrypt.DefaultCost)
			if err != nil {
				return err
			}
			adminId = "5000000000000000001"
			admin := models.User{
				Id:         adminId,
				Username:   "admin",
				Password:   string(hash),
				Nickname:   "超级管理员",
				DeptId:     seed.DeptRootId,
				Status:     1,
				Lang:       "zh",
				Remark:     "系统内置管理员",
				Timestamps: models.Timestamps{Created: now, Modified: now},
			}
			if err := tx.Table("user").Create(&admin).Error; err != nil {
				return err
			}
			if lc != nil {
				lc.Infof("初始化系统管理员账号: admin / %s", DefaultAdminPassword)
			}
		} else {
			var admin models.User
			if err := tx.Table("user").Where("username = ?", "admin").First(&admin).Error; err != nil {
				return err
			}
			adminId = admin.Id
			if len(adminId) == 0 {
				adminId = "5000000000000000001"
				if err := tx.Table("user").Where("username = ?", "admin").
					Update("id", adminId).Error; err != nil {
					return err
				}
			}
		}

		/******* 管理员绑定超管角色 *******/
		if len(adminId) > 0 {
			var count int64
			if err := tx.Table("sys_user_role").
				Where("user_id = ? AND role_id = ?", adminId, seed.RoleAdminId).Count(&count).Error; err != nil {
				return err
			}
			if count == 0 {
				if err := tx.Table("sys_user_role").
					Create(&models.UserRole{UserId: adminId, RoleId: seed.RoleAdminId}).Error; err != nil {
					return err
				}
			}
		}

		if lc != nil {
			lc.Info("系统管理表初始化完成")
		}
		return nil
	}); err != nil {
		return err
	}
	// 内置工业方案：幂等写入（品类模板 + 物模型模板 + 方案包）
	return c.seedSolutions()
}

// seedSolutions 写入内置工业方案（内置行业模板）。
//
// 同时写三张表：
//   - category_template / thing_model_template：让前端「一键建产品」直接选到工业品类（零改动）；
//   - solution：模板包（告警预设 + 看板预设 + OEE 指标映射），供一键应用使用。
func (c *Client) seedSolutions() error {
	templates, categories, thingModels, err := seed.DefaultSolutions()
	if err != nil {
		return err
	}
	if _, err = c.BatchUpsertCategoryTemplate(categories); err != nil {
		return fmt.Errorf("写入内置品类模板失败: %w", err)
	}
	if _, err = c.BatchUpsertThingModelTemplate(thingModels); err != nil {
		return fmt.Errorf("写入内置物模型模板失败: %w", err)
	}
	if _, err = c.BatchUpsertSolution(templates); err != nil {
		return fmt.Errorf("写入内置方案失败: %w", err)
	}
	// 清理旧命名遗留（machine_template → solution、品类 key 前缀 mt_ → sol_）
	c.cleanupLegacySolutionData()
	if c.loggingClient != nil {
		c.loggingClient.Infof("内置方案已就绪: %d 个", len(templates))
	}
	return nil
}

// cleanupLegacySolutionData 清理改名前的遗留数据（幂等，可反复执行）：
//   - 旧表 machine_template（数据已迁移到 solution，由本函数之前的 upsert 保证）
//   - 旧品类 key 前缀 mt_ 对应的 category_template / thing_model_template 行
func (c *Client) cleanupLegacySolutionData() {
	if c.Pool.Migrator().HasTable("machine_template") {
		if err := c.Pool.Migrator().DropTable("machine_template"); err != nil {
			c.loggingClient.Warnf("删除旧表 machine_template 失败: %v", err)
		} else {
			c.loggingClient.Infof("已删除改名前遗留的 machine_template 表")
		}
	}
	for _, t := range []string{"category_template", "thing_model_template"} {
		if !c.Pool.Migrator().HasTable(t) {
			continue
		}
		res := c.Pool.Exec("DELETE FROM " + t + " WHERE category_key LIKE 'mt\\_%' ESCAPE '\\'")
		if res.Error != nil {
			c.loggingClient.Warnf("清理 %s 中旧前缀数据失败: %v", t, res.Error)
			continue
		}
		if res.RowsAffected > 0 {
			c.loggingClient.Infof("已清理 %s 中 %d 条旧前缀(mt_)方案数据", t, res.RowsAffected)
		}
	}
}

// ensureUniqueIndexes 为「同类型内名称唯一」补数据库唯一索引（应用层已校验，这里是兜底）。
// 注意：不使用模型上的 uniqueIndex 标签——那样 AutoMigrate 会在历史库存在重复数据时直接失败、
// 导致服务起不来。这里先探测重复：干净才建索引，存在重复则只告警并跳过，由运维清理后再建。
func (c *Client) ensureUniqueIndexes() error {
	pool := c.Pool.WithContext(tenant.Platform(context.Background()))
	indexes := []struct {
		table   string
		name    string
		columns []string
		dupKeys string
	}{
		{models.TableDataResource, "uk_data_resource_type_name", []string{"type", "name"}, "type, name"},
		{models.TableRuleEngine, "uk_rule_engine_name", []string{"name"}, "name"},
		{models.TableScene, "uk_scene_name", []string{"name"}, "name"},
		// 网关 SN 作为 MQTT clientId/username，必须全局唯一（上行报文靠它定位网关与租户）
		{models.TableGateway, "uk_gateway_sn", []string{"sn"}, "sn"},
		// 断点续传幂等：同一网关同一 seq 只落一条
		{models.TableGatewayIngestLog, "uk_gateway_ingest_seq", []string{"gateway_id", "seq"}, "gateway_id, seq"},
	}
	for _, idx := range indexes {
		if pool.Migrator().HasIndex(idx.table, idx.name) {
			continue
		}
		var dupCount int64
		if err := pool.Table(idx.table).
			Select("COUNT(*)").Group(idx.dupKeys).Having("COUNT(*) > 1").
			Count(&dupCount).Error; err != nil {
			return fmt.Errorf("check duplicate of %s failed: %w", idx.table, err)
		}
		if dupCount > 0 {
			c.loggingClient.Warnf("table %s has %d duplicated keys(%s), skip creating unique index %s",
				idx.table, dupCount, idx.dupKeys, idx.name)
			continue
		}
		stmt := fmt.Sprintf("CREATE UNIQUE INDEX %s ON %s(%s)",
			idx.name, idx.table, strings.Join(idx.columns, ", "))
		if err := pool.Exec(stmt).Error; err != nil {
			c.loggingClient.Warnf("create unique index %s on %s failed: %v", idx.name, idx.table, err)
		}
	}
	return nil
}
