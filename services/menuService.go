package services

import (
	"fmt"
	"github.com/beego/beego/v2/client/orm"
	"iotServer/models"
)

type MenuService struct{}

// InitTenantMenus 为新租户初始化菜单
// newTenantId: 新创建的租户/部门 ID (例如 1001)
func (m *MenuService) InitTenantMenus(newTenantId int64) error {
	o := orm.NewOrm()

	// 1. 查询所有模板菜单 (TenantId = 0)
	// 关键：必须按 ParentId 排序，保证先处理根节点，再处理子节点
	var templates []*models.Menu

	// SQL 逻辑: SELECT * FROM menu WHERE tenant_id = 0 ORDER BY parent_id ASC
	// 注意：不同数据库对 NULL 的排序可能不同，通常 NULL 会排在最前
	qs := o.QueryTable(new(models.Menu)).Filter("Department__isnull", true)
	_, err := qs.All(&templates)
	if err != nil {
		return fmt.Errorf("查询模板菜单失败: %v", err)
	}

	if len(templates) == 0 {
		// 没有配置模板，直接返回，不算错误
		return nil
	}

	// 2. 定义 ID 映射表: map[旧模板ID] => 新生成的ID
	idMap := make(map[int64]int64)

	// 3. 开启事务
	txOrm, err := o.Begin()
	if err != nil {
		return fmt.Errorf("开启事务失败: %v", err)
	}

	// 4. 循环复制
	for _, t := range templates {
		// 记录旧 ID 用于后续映射
		oldId := t.Id

		// --- 准备新对象 ---
		// 注意：这里是指针引用，需要新建一个对象，否则会修改原切片
		newMenu := models.Menu{
			Name:           t.Name,
			Meta:           t.Meta,
			Component:      t.Component,
			Status:         t.Status,
			Path:           t.Path,
			Redirect:       t.Redirect,
			Type:           t.Type,
			PermissionList: t.PermissionList,
			Priority:       t.Priority,

			// 设置为新租户的 ID
			Department: &models.Department{Id: newTenantId},
		}

		// --- 核心逻辑：重建父子关系 ---
		if t.ParentId == nil || *t.ParentId == 0 {
			// 如果模板是根节点，新菜单也是根节点
			newMenu.ParentId = nil
		} else {
			// 如果模板是子节点，去 Map 里找它“爸爸”的新 ID
			oldParentId := *t.ParentId
			if newParentId, ok := idMap[oldParentId]; ok {
				newMenu.ParentId = &newParentId
			} else {
				// 异常情况：逻辑上不应该发生，因为我们按 ParentId 排序了
				// 如果发生了，说明父节点还没创建，或者数据完整性有问题
				_ = txOrm.Rollback()
				return fmt.Errorf("无法找到父级菜单 ID: %d (子菜单: %s)", oldParentId, t.Name)
			}
		}

		// --- 插入新记录 ---
		createdId, err := o.Insert(&newMenu)
		if err != nil {
			_ = txOrm.Rollback()
			return fmt.Errorf("创建菜单失败: %v", err)
		}

		// --- 记录映射 ---
		// 比如：模板里"系统管理"ID是1，新创建的ID是100。记录 1 -> 100
		idMap[oldId] = createdId
	}

	// 5. 提交事务
	if err := txOrm.Commit(); err != nil {
		return err
	}

	return nil
}
