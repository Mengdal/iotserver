package sqlite

import (
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	"iotserver/internal/dtos"
	"iotserver/internal/models"
	"iotserver/internal/pkg/errort"
	"iotserver/internal/pkg/utils"
)

/******************************* 租户 *******************************/

func addTenant(c *Client, t models.Tenant) (models.Tenant, error) {
	if len(t.Id) == 0 {
		t.Id = utils.RandomNum()
	}
	t.Code = strings.TrimSpace(t.Code)
	if t.Code == "" {
		return t, errort.NewCommonErr(errort.DefaultReqParamsError, fmt.Errorf("tenant code is required"))
	}
	var count int64
	if err := c.Pool.Table(models.TableTenant).Where("code = ?", t.Code).Count(&count).Error; err != nil {
		return t, errort.NewCommonEdgeX(errort.DefaultSystemError, "tenant query failed", err)
	}
	if count > 0 {
		return t, errort.NewCommonEdgeX(errort.DefaultNameRepeat,
			fmt.Sprintf("tenant code %s already exists", t.Code), nil)
	}
	now := time.Now().UnixMilli()
	t.Status = 1
	t.Timestamps = models.Timestamps{Created: now, Modified: now}
	if err := c.Pool.Table(t.TableName()).Create(&t).Error; err != nil {
		return t, errort.NewCommonEdgeX(errort.DefaultSystemError, "tenant creation failed", err)
	}
	return t, nil
}

func tenantById(c *Client, id string) (models.Tenant, error) {
	var t models.Tenant
	if err := c.Pool.Table(models.TableTenant).Where("id = ?", id).First(&t).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return t, errort.NewCommonErr(errort.DefaultResourcesNotFound, fmt.Errorf("tenant %s not found", id))
		}
		return t, errort.NewCommonEdgeX(errort.DefaultSystemError, "tenant query failed", err)
	}
	return t, nil
}

func tenantByCode(c *Client, code string) (models.Tenant, error) {
	var t models.Tenant
	if err := c.Pool.Table(models.TableTenant).Where("code = ?", code).First(&t).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return t, errort.NewCommonErr(errort.DefaultResourcesNotFound, fmt.Errorf("tenant %s not found", code))
		}
		return t, errort.NewCommonEdgeX(errort.DefaultSystemError, "tenant query failed", err)
	}
	return t, nil
}

func tenantSearch(c *Client, offset, limit int, req dtos.TenantSearchQueryRequest) ([]models.Tenant, uint32, error) {
	var (
		list  []models.Tenant
		total int64
	)
	query := c.Pool.Table(models.TableTenant)
	if req.Name != "" {
		query = query.Where("name LIKE ?", "%"+escapeLike(req.Name)+"%")
	}
	if req.Code != "" {
		query = query.Where("code = ?", req.Code)
	}
	if req.Status == 0 || req.Status == 1 {
		query = query.Where("status = ?", req.Status)
	}
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, errort.NewCommonEdgeX(errort.DefaultSystemError, "tenant count failed", err)
	}
	if err := query.Order("created desc").Offset(offset).Limit(limit).Find(&list).Error; err != nil {
		return nil, 0, errort.NewCommonEdgeX(errort.DefaultSystemError, "tenant search failed", err)
	}
	return list, uint32(total), nil
}

func updateTenant(c *Client, t models.Tenant) error {
	old, err := tenantById(c, t.Id)
	if err != nil {
		return err
	}
	updates := map[string]interface{}{
		"name":          t.Name,
		"contact_name":  t.ContactName,
		"contact_phone": t.ContactPhone,
		"contact_email": t.ContactEmail,
		"expire_at":     t.ExpireAt,
		"max_devices":   t.MaxDevices,
		"max_gateways":  t.MaxGateways,
		"max_users":     t.MaxUsers,
		"remark":        t.Remark,
		"modified":      time.Now().UnixMilli(),
		"created":       old.Created,
	}
	if t.Status == 0 || t.Status == 1 {
		updates["status"] = t.Status
	}
	return c.Pool.Table(models.TableTenant).Where("id = ?", t.Id).Updates(updates).Error
}

func deleteTenant(c *Client, id string) error {
	res := c.Pool.Table(models.TableTenant).Where("id = ?", id).Delete(&models.Tenant{})
	if res.Error != nil {
		return errort.NewCommonEdgeX(errort.DefaultSystemError, "tenant delete failed", res.Error)
	}
	if res.RowsAffected == 0 {
		return errort.NewCommonErr(errort.DefaultResourcesNotFound, fmt.Errorf("tenant %s not found", id))
	}
	return nil
}

func (c *Client) AddTenant(t models.Tenant) (models.Tenant, error) {
	return addTenant(c, t)
}

func (c *Client) TenantById(id string) (models.Tenant, error) {
	return tenantById(c, id)
}

func (c *Client) TenantByCode(code string) (models.Tenant, error) {
	return tenantByCode(c, code)
}

func (c *Client) TenantSearch(offset, limit int, req dtos.TenantSearchQueryRequest) ([]models.Tenant, uint32, error) {
	return tenantSearch(c, offset, limit, req)
}

func (c *Client) UpdateTenant(t models.Tenant) error {
	return updateTenant(c, t)
}

func (c *Client) DeleteTenant(id string) error {
	return deleteTenant(c, id)
}

/******************************* 租户资源统计（配额 / 删租户保护） *******************************/

// countTenantRows 统计指定租户在某张表下的记录数。
//
// 显式带 tenant_id 条件：平台作用域下 GORM 租户插件不加过滤，
// 只有显式条件才能把计数限定在目标租户。调用方应同时用 WithCtx 把作用域
// 切到同一租户，防止插件与显式条件叠加成互斥的 AND。
func countTenantRows(c *Client, model interface{}, tenantId string) (int64, error) {
	var n int64
	if err := c.Pool.Model(model).Where("tenant_id = ?", tenantId).Count(&n).Error; err != nil {
		return 0, errort.NewCommonEdgeX(errort.DefaultSystemError, "tenant resource count failed", err)
	}
	return n, nil
}

// TenantUserCount 统计租户下的用户数
func (c *Client) TenantUserCount(tenantId string) (int64, error) {
	return countTenantRows(c, &models.User{}, tenantId)
}

// TenantDeviceCount 统计租户下的设备数
func (c *Client) TenantDeviceCount(tenantId string) (int64, error) {
	return countTenantRows(c, &models.Device{}, tenantId)
}

// TenantGatewayCount 统计租户下的网关数
func (c *Client) TenantGatewayCount(tenantId string) (int64, error) {
	return countTenantRows(c, &models.Gateway{}, tenantId)
}

// TenantProductCount 统计租户下的产品数
func (c *Client) TenantProductCount(tenantId string) (int64, error) {
	return countTenantRows(c, &models.Product{}, tenantId)
}

// TenantVizBoardCount 统计租户下的看板数
func (c *Client) TenantVizBoardCount(tenantId string) (int64, error) {
	return countTenantRows(c, &models.VizBoard{}, tenantId)
}

// CleanupTenantMenuRecords 清理租户的菜单定制记录。
//
// 菜单定制表（tenant_menu / tenant_menu_pref）由菜单任务引入，本任务不依赖其表结构：
// 表不存在时静默跳过，避免删租户因为「表还没建」而失败。
func (c *Client) CleanupTenantMenuRecords(tenantId string) error {
	for _, table := range []string{"tenant_menu", "tenant_menu_pref"} {
		if !c.Pool.Migrator().HasTable(table) {
			continue
		}
		if err := c.Pool.Exec("DELETE FROM "+table+" WHERE tenant_id = ?", tenantId).Error; err != nil {
			return errort.NewCommonEdgeX(errort.DefaultSystemError, "tenant menu data cleanup failed", err)
		}
	}
	return nil
}
