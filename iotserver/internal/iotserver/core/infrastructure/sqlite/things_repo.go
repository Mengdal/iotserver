package sqlite

import (
	"fmt"
	"time"

	"iotserver/internal/models"
	"iotserver/internal/pkg/errort"
	"iotserver/internal/pkg/utils"
)

/******************************* 租户 ↔ things 项目绑定 *******************************/

func addOrUpdateTenantThingsBinding(c *Client, b models.TenantThingsBinding) (models.TenantThingsBinding, error) {
	if b.TenantId == "" {
		return b, errort.NewCommonErr(errort.DefaultReqParamsError, fmt.Errorf("tenant id is required"))
	}
	if b.ThingsProjectId <= 0 {
		return b, errort.NewCommonErr(errort.DefaultReqParamsError, fmt.Errorf("things project id is required"))
	}
	now := time.Now().UnixMilli()
	var old models.TenantThingsBinding
	// 用 Limit(1).Find 而不是 First：绑定不存在是正常分支，
	// GORM 的 First 会把 ErrRecordNotFound 打成 error 日志。
	err := c.Pool.Table(models.TableTenantThingsBinding).Where("tenant_id = ?", b.TenantId).Limit(1).Find(&old).Error
	if err != nil {
		return b, errort.NewCommonEdgeX(errort.DefaultSystemError, "tenant things binding query failed", err)
	}
	if old.Id == "" {
		if b.Id == "" {
			b.Id = utils.GenUUID()
		}
		if b.Status == "" {
			b.Status = models.TenantThingsStatusBound
		}
		if b.LastSyncAt == 0 {
			b.LastSyncAt = now
		}
		b.Timestamps = models.Timestamps{Created: now, Modified: now}
		if err = c.Pool.Table(b.TableName()).Create(&b).Error; err != nil {
			return b, errort.NewCommonEdgeX(errort.DefaultSystemError, "tenant things binding creation failed", err)
		}
		return b, nil
	}
	// 同一项目不能绑定到两个租户。显式查一次，避免依赖数据库唯一索引报错文案。
	var occupied models.TenantThingsBinding
	if err = c.Pool.Table(models.TableTenantThingsBinding).
		Where("things_project_id = ? AND tenant_id <> ?", b.ThingsProjectId, b.TenantId).
		Limit(1).Find(&occupied).Error; err != nil {
		return b, errort.NewCommonEdgeX(errort.DefaultSystemError, "tenant things binding query failed", err)
	}
	if occupied.Id != "" {
		return b, errort.NewCommonEdgeX(errort.DefaultNameRepeat,
			fmt.Sprintf("things project %d already bound to tenant %s", b.ThingsProjectId, occupied.TenantId), nil)
	}
	updates := map[string]interface{}{
		"things_project_id":   b.ThingsProjectId,
		"things_project_name": b.ThingsProjectName,
		"status":              b.Status,
		"last_sync_at":        b.LastSyncAt,
		"last_error":          b.LastError,
		"modified":            now,
	}
	if b.Status == "" {
		updates["status"] = models.TenantThingsStatusBound
	}
	if b.LastSyncAt == 0 {
		updates["last_sync_at"] = now
	}
	if err = c.Pool.Table(models.TableTenantThingsBinding).Where("id = ?", old.Id).Updates(updates).Error; err != nil {
		return b, errort.NewCommonEdgeX(errort.DefaultSystemError, "tenant things binding update failed", err)
	}
	b.Id = old.Id
	b.Created = old.Created
	b.Modified = now
	return b, nil
}

func tenantThingsBindingByTenant(c *Client, tenantID string) (models.TenantThingsBinding, error) {
	var b models.TenantThingsBinding
	err := c.Pool.Table(models.TableTenantThingsBinding).Where("tenant_id = ?", tenantID).Limit(1).Find(&b).Error
	if err != nil {
		return b, errort.NewCommonEdgeX(errort.DefaultSystemError, "tenant things binding query failed", err)
	}
	if b.Id == "" {
		return b, errort.NewCommonErr(errort.DefaultResourcesNotFound,
			fmt.Errorf("things binding for tenant %s not found", tenantID))
	}
	return b, nil
}

func tenantThingsBindingByProject(c *Client, projectID int64) (models.TenantThingsBinding, error) {
	var b models.TenantThingsBinding
	err := c.Pool.Table(models.TableTenantThingsBinding).Where("things_project_id = ?", projectID).Limit(1).Find(&b).Error
	if err != nil {
		return b, errort.NewCommonEdgeX(errort.DefaultSystemError, "tenant things binding query failed", err)
	}
	if b.Id == "" {
		return b, errort.NewCommonErr(errort.DefaultResourcesNotFound,
			fmt.Errorf("things project %d is not bound", projectID))
	}
	return b, nil
}

func tenantThingsBindingsByTenantIDs(c *Client, tenantIDs []string) (map[string]models.TenantThingsBinding, error) {
	out := make(map[string]models.TenantThingsBinding, len(tenantIDs))
	if len(tenantIDs) == 0 {
		return out, nil
	}
	var list []models.TenantThingsBinding
	if err := c.Pool.Table(models.TableTenantThingsBinding).
		Where("tenant_id IN ?", tenantIDs).Find(&list).Error; err != nil {
		return nil, errort.NewCommonEdgeX(errort.DefaultSystemError, "tenant things binding query failed", err)
	}
	for _, b := range list {
		out[b.TenantId] = b
	}
	return out, nil
}

func tenantThingsBindings(c *Client) ([]models.TenantThingsBinding, error) {
	var list []models.TenantThingsBinding
	if err := c.Pool.Table(models.TableTenantThingsBinding).Order("tenant_id").Find(&list).Error; err != nil {
		return nil, errort.NewCommonEdgeX(errort.DefaultSystemError, "tenant things binding query failed", err)
	}
	return list, nil
}

func updateTenantThingsBindingStatus(c *Client, tenantID, status, lastError string, lastSyncAt int64) error {
	updates := map[string]interface{}{
		"status":     status,
		"last_error": lastError,
		"modified":   time.Now().UnixMilli(),
	}
	if lastSyncAt > 0 {
		updates["last_sync_at"] = lastSyncAt
	}
	return c.Pool.Table(models.TableTenantThingsBinding).Where("tenant_id = ?", tenantID).Updates(updates).Error
}

func deleteTenantThingsBinding(c *Client, tenantID string) error {
	return c.Pool.Table(models.TableTenantThingsBinding).Where("tenant_id = ?", tenantID).Delete(&models.TenantThingsBinding{}).Error
}

/******************************* 网关 things 同步 *******************************/

func gatewayByThingsID(c *Client, thingsGatewayID int64) (models.Gateway, error) {
	var g models.Gateway
	err := c.Pool.Table(models.TableGateway).Where("things_gateway_id = ?", thingsGatewayID).Limit(1).Find(&g).Error
	if err != nil {
		return g, errort.NewCommonEdgeX(errort.DefaultSystemError, "gateway query failed", err)
	}
	if g.Id == "" {
		return g, errort.NewCommonErr(errort.DefaultResourcesNotFound,
			fmt.Errorf("gateway with things id %d not found", thingsGatewayID))
	}
	return g, nil
}

// updateGatewayThingsSync 用 things 的展示字段刷新本地镜像。
// 不更新 status/last_online_at/config 等数据面/本地运行字段。
func updateGatewayThingsSync(c *Client, g models.Gateway) error {
	now := time.Now().UnixMilli()
	return c.Pool.Table(models.TableGateway).Where("id = ?", g.Id).Updates(map[string]interface{}{
		"name":                g.Name,
		"pk":                  g.Pk,
		"model":               g.Model,
		"source":              models.GatewaySourceThings,
		"things_gateway_id":   g.ThingsGatewayId,
		"things_project_id":   g.ThingsProjectId,
		"things_sync_status":  g.ThingsSyncStatus,
		"things_last_sync_at": now,
		"things_last_error":   g.ThingsLastError,
		"modified":            now,
	}).Error
}

func updateGatewayThingsStatus(c *Client, id, status, lastError string, lastSyncAt int64) error {
	updates := map[string]interface{}{
		"things_sync_status": status,
		"things_last_error":  lastError,
		"modified":           time.Now().UnixMilli(),
	}
	if lastSyncAt > 0 {
		updates["things_last_sync_at"] = lastSyncAt
	}
	return c.Pool.Table(models.TableGateway).Where("id = ?", id).Updates(updates).Error
}

func (c *Client) AddOrUpdateTenantThingsBinding(b models.TenantThingsBinding) (models.TenantThingsBinding, error) {
	return addOrUpdateTenantThingsBinding(c, b)
}

func (c *Client) TenantThingsBindingByTenant(tenantID string) (models.TenantThingsBinding, error) {
	return tenantThingsBindingByTenant(c, tenantID)
}

func (c *Client) TenantThingsBindingByProject(projectID int64) (models.TenantThingsBinding, error) {
	return tenantThingsBindingByProject(c, projectID)
}

func (c *Client) TenantThingsBindingsByTenantIDs(tenantIDs []string) (map[string]models.TenantThingsBinding, error) {
	return tenantThingsBindingsByTenantIDs(c, tenantIDs)
}

func (c *Client) TenantThingsBindings() ([]models.TenantThingsBinding, error) {
	return tenantThingsBindings(c)
}

func (c *Client) UpdateTenantThingsBindingStatus(tenantID, status, lastError string, lastSyncAt int64) error {
	return updateTenantThingsBindingStatus(c, tenantID, status, lastError, lastSyncAt)
}

func (c *Client) DeleteTenantThingsBinding(tenantID string) error {
	return deleteTenantThingsBinding(c, tenantID)
}

func (c *Client) GatewayByThingsID(thingsGatewayID int64) (models.Gateway, error) {
	return gatewayByThingsID(c, thingsGatewayID)
}

func (c *Client) UpdateGatewayThingsSync(g models.Gateway) error {
	return updateGatewayThingsSync(c, g)
}

func (c *Client) UpdateGatewayThingsStatus(id, status, lastError string, lastSyncAt int64) error {
	return updateGatewayThingsStatus(c, id, status, lastError, lastSyncAt)
}
