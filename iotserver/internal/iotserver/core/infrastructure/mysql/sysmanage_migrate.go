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

package mysql

import (
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"iotserver/internal/models"
	"iotserver/internal/pkg/seed"
)

// DefaultAdminPassword 内置管理员初始密码
const DefaultAdminPassword = "admin123"

// MigrateSystemTables 自动创建系统管理相关表并写入初始化数据
func (c *Client) MigrateSystemTables() error {
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
		&models.LoginLog{},
		&models.SystemConfig{},
		// ===== 设备接入 =====
		&models.DeviceLibrary{},
		&models.DeviceService{},
		&models.Device{},
		&models.DockerConfig{},
		&models.AdvanceConfig{},
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
	); err != nil {
		return err
	}
	if err := c.ensureUniqueIndexes(); err != nil {
		return err
	}
	return c.seedSystemData()
}

func (c *Client) seedSystemData() error {
	lc := c.loggingClient

	return c.Pool.Transaction(func(tx *gorm.DB) error {
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
		defaultMenus := seed.DefaultMenus()
		seededMenuIds := make(map[string]struct{}, len(defaultMenus))
		for _, menu := range defaultMenus {
			seededMenuIds[menu.Id] = struct{}{}
		}
		var existingMenus []models.Menu
		if err := tx.Table("sys_menu").Find(&existingMenus).Error; err != nil {
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
			if err := tx.Table("sys_menu").Where("id IN ?", staleIds).Delete(&models.Menu{}).Error; err != nil {
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
				updates := map[string]interface{}{
					"parent_id": menu.ParentId,
					"name":      menu.Name,
					"path":      menu.Path,
					"icon":      menu.Icon,
					"menu_type": menu.MenuType,
					"perms":     menu.Perms,
					"sort":      menu.Sort,
					"visible":   menu.Visible,
					"status":    menu.Status,
					"is_frame":  menu.IsFrame,
					"modified":  now,
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

		/******* 超管角色绑定全部菜单 *******/
		var menuIds []string
		if err := tx.Table("sys_menu").Pluck("id", &menuIds).Error; err != nil {
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
		// 平台名称/描述跟随代码中的默认值刷新，其余配置仅在缺失时写入，避免覆盖用户修改
		alwaysRefresh := map[string]struct{}{
			"platform.name":      {},
			"platform.copyright": {},
		}
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
				// 名称/分组/类型始终跟随代码；值只对 alwaysRefresh 的品牌项刷新，避免覆盖用户修改
				updates := map[string]interface{}{
					"name":       item.Name,
					"group_name": item.Group,
					"type":       item.Type,
					"sort":       item.Sort,
					"modified":   now,
				}
				if _, ok := alwaysRefresh[item.Key]; ok {
					updates["config_value"] = item.Value
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
	})
}

// ensureUniqueIndexes 为「同类型内名称唯一」补数据库唯一索引（应用层已校验，这里是兜底）。
// 注意：不使用模型上的 uniqueIndex 标签——那样 AutoMigrate 会在历史库存在重复数据时直接失败、
// 导致服务起不来。这里先探测重复：干净才建索引，存在重复则只告警并跳过，由运维清理后再建。
func (c *Client) ensureUniqueIndexes() error {
	indexes := []struct {
		table   string
		name    string
		columns []string
		dupKeys string
	}{
		{models.TableDataResource, "uk_data_resource_type_name", []string{"type", "name"}, "type, name"},
		{models.TableRuleEngine, "uk_rule_engine_name", []string{"name"}, "name"},
		{models.TableScene, "uk_scene_name", []string{"name"}, "name"},
	}
	for _, idx := range indexes {
		if c.Pool.Migrator().HasIndex(idx.table, idx.name) {
			continue
		}
		var dupCount int64
		if err := c.Pool.Table(idx.table).
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
		if err := c.Pool.Exec(stmt).Error; err != nil {
			c.loggingClient.Warnf("create unique index %s on %s failed: %v", idx.name, idx.table, err)
		}
	}
	return nil
}
