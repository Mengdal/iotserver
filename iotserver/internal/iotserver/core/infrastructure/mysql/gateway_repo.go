package mysql

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

/******************************* 网关（第三方工业网关接入） *******************************/

func addGateway(c *Client, g models.Gateway) (models.Gateway, error) {
	if len(g.Id) == 0 {
		g.Id = utils.RandomNum()
	}
	g.Sn = strings.TrimSpace(g.Sn)
	if g.Sn == "" {
		return g, errort.NewCommonErr(errort.DefaultReqParamsError, fmt.Errorf("gateway sn is required"))
	}
	if g.Secret == "" {
		g.Secret = utils.GenerateDeviceSecret(16)
	}
	if g.Status == 0 && g.LastOnlineAt == 0 {
		g.Status = models.GatewayStatusNever
	}
	var count int64
	if err := c.Pool.Table(models.TableGateway).Where("sn = ?", g.Sn).Count(&count).Error; err != nil {
		return g, errort.NewCommonEdgeX(errort.DefaultSystemError, "gateway query failed", err)
	}
	if count > 0 {
		return g, errort.NewCommonEdgeX(errort.DefaultNameRepeat,
			fmt.Sprintf("gateway sn %s already exists", g.Sn), nil)
	}
	now := time.Now().UnixMilli()
	g.Timestamps = models.Timestamps{Created: now, Modified: now}
	if err := c.Pool.Table(g.TableName()).Create(&g).Error; err != nil {
		return g, errort.NewCommonEdgeX(errort.DefaultSystemError, "gateway creation failed", err)
	}
	return g, nil
}

func gatewayById(c *Client, id string) (models.Gateway, error) {
	var g models.Gateway
	if err := c.Pool.Table(models.TableGateway).Where("id = ?", id).First(&g).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return g, errort.NewCommonErr(errort.DefaultResourcesNotFound, fmt.Errorf("gateway %s not found", id))
		}
		return g, errort.NewCommonEdgeX(errort.DefaultSystemError, "gateway query failed", err)
	}
	return g, nil
}

// gatewayBySn 按 SN 查询。上行报文先靠 SN 定位网关（进而确定租户），
// 调用方需要显式使用平台作用域（见 gatewayingest），因为此时还没有租户上下文。
func gatewayBySn(c *Client, sn string) (models.Gateway, error) {
	var g models.Gateway
	if err := c.Pool.Table(models.TableGateway).Where("sn = ?", sn).First(&g).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return g, errort.NewCommonErr(errort.DefaultResourcesNotFound, fmt.Errorf("gateway %s not found", sn))
		}
		return g, errort.NewCommonEdgeX(errort.DefaultSystemError, "gateway query failed", err)
	}
	return g, nil
}

func gatewaySearch(c *Client, offset, limit int, req dtos.GatewaySearchQueryRequest) ([]models.Gateway, uint32, error) {
	var (
		list  []models.Gateway
		total int64
	)
	query := c.Pool.Table(models.TableGateway)
	if req.Name != "" {
		query = query.Where("name LIKE ?", "%"+escapeLike(req.Name)+"%")
	}
	if req.Sn != "" {
		query = query.Where("sn = ?", req.Sn)
	}
	if req.DeptId != "" {
		query = query.Where("dept_id = ?", req.DeptId)
	}
	if req.Status != nil {
		query = query.Where("status = ?", *req.Status)
	}
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, errort.NewCommonEdgeX(errort.DefaultSystemError, "gateway count failed", err)
	}
	if err := query.Order("created desc").Offset(offset).Limit(limit).Find(&list).Error; err != nil {
		return nil, 0, errort.NewCommonEdgeX(errort.DefaultSystemError, "gateway search failed", err)
	}
	return list, uint32(total), nil
}

func updateGateway(c *Client, g models.Gateway) error {
	old, err := gatewayById(c, g.Id)
	if err != nil {
		return err
	}
	return c.Pool.Table(models.TableGateway).Where("id = ?", g.Id).Updates(map[string]interface{}{
		"name":     g.Name,
		"dept_id":  g.DeptId,
		"vendor":   g.Vendor,
		"model":    g.Model,
		"remark":   g.Remark,
		"modified": time.Now().UnixMilli(),
		"created":  old.Created,
	}).Error
}

func deleteGateway(c *Client, id string) error {
	res := c.Pool.Table(models.TableGateway).Where("id = ?", id).Delete(&models.Gateway{})
	if res.Error != nil {
		return errort.NewCommonEdgeX(errort.DefaultSystemError, "gateway delete failed", res.Error)
	}
	if res.RowsAffected == 0 {
		return errort.NewCommonErr(errort.DefaultResourcesNotFound, fmt.Errorf("gateway %s not found", id))
	}
	return nil
}

// updateGatewayStatus 更新在线状态与最近时间
func updateGatewayStatus(c *Client, id string, status int, ip string, onlineAt int64) error {
	updates := map[string]interface{}{
		"status":   status,
		"modified": time.Now().UnixMilli(),
	}
	if onlineAt > 0 {
		updates["last_online_at"] = onlineAt
	}
	if ip != "" {
		updates["ip"] = ip
	}
	return c.Pool.Table(models.TableGateway).Where("id = ?", id).Updates(updates).Error
}

// touchGatewayReport 记录最近上报时间（不改变在线状态）
func touchGatewayReport(c *Client, id string, ts int64) error {
	if ts <= 0 {
		ts = time.Now().UnixMilli()
	}
	return c.Pool.Table(models.TableGateway).Where("id = ?", id).Updates(map[string]interface{}{
		"last_report_at": ts,
	}).Error
}

// updateGatewayConfig 写入待下发配置（版本 +1）
func updateGatewayConfig(c *Client, id, config string, version int) error {
	return c.Pool.Table(models.TableGateway).Where("id = ?", id).Updates(map[string]interface{}{
		"config":         config,
		"config_version": version,
		"modified":       time.Now().UnixMilli(),
	}).Error
}

// gatewayIngestLogExists 判断该网关的 seq 是否已处理过（断点续传幂等）
func gatewayIngestLogExists(c *Client, gatewayId string, seq int64) (bool, error) {
	var count int64
	if err := c.Pool.Table(models.TableGatewayIngestLog).
		Where("gateway_id = ? AND seq = ?", gatewayId, seq).Count(&count).Error; err != nil {
		return false, errort.NewCommonEdgeX(errort.DefaultSystemError, "ingest log query failed", err)
	}
	return count > 0, nil
}

func addGatewayIngestLog(c *Client, log models.GatewayIngestLog) error {
	if len(log.Id) == 0 {
		log.Id = utils.RandomNum()
	}
	if log.ReceivedAt == 0 {
		log.ReceivedAt = time.Now().UnixMilli()
	}
	return c.Pool.Table(models.TableGatewayIngestLog).Create(&log).Error
}

// gatewayDeviceCount / gatewayOnlineCount 列表页统计
func gatewayDeviceCount(c *Client, gatewayId string) (int64, error) {
	var count int64
	if err := c.Pool.Table("device").Where("gateway_id = ?", gatewayId).Count(&count).Error; err != nil {
		return 0, errort.NewCommonEdgeX(errort.DefaultSystemError, "device count failed", err)
	}
	return count, nil
}

func (c *Client) AddGateway(g models.Gateway) (models.Gateway, error) {
	return addGateway(c, g)
}

func (c *Client) GatewayById(id string) (models.Gateway, error) {
	return gatewayById(c, id)
}

func (c *Client) GatewayBySn(sn string) (models.Gateway, error) {
	return gatewayBySn(c, sn)
}

func (c *Client) GatewaySearch(offset, limit int, req dtos.GatewaySearchQueryRequest) ([]models.Gateway, uint32, error) {
	return gatewaySearch(c, offset, limit, req)
}

func (c *Client) UpdateGateway(g models.Gateway) error {
	return updateGateway(c, g)
}

func (c *Client) DeleteGateway(id string) error {
	return deleteGateway(c, id)
}

func (c *Client) UpdateGatewayStatus(id string, status int, ip string, onlineAt int64) error {
	return updateGatewayStatus(c, id, status, ip, onlineAt)
}
func (c *Client) MarkStaleGatewaysOffline(beforeMs int64) (int64, error) {
	return markStaleGatewaysOffline(c, beforeMs)
}

func (c *Client) TouchGatewayReport(id string, ts int64) error {
	return touchGatewayReport(c, id, ts)
}

func (c *Client) UpdateGatewayConfig(id, config string, version int) error {
	return updateGatewayConfig(c, id, config, version)
}

func (c *Client) GatewayIngestLogExists(gatewayId string, seq int64) (bool, error) {
	return gatewayIngestLogExists(c, gatewayId, seq)
}

func (c *Client) AddGatewayIngestLog(log models.GatewayIngestLog) error {
	return addGatewayIngestLog(c, log)
}

func (c *Client) GatewayDeviceCount(gatewayId string) (int64, error) {
	return gatewayDeviceCount(c, gatewayId)
}

// markStaleGatewaysOffline 把「最后在线时间早于 beforeMs」的在线网关批量置为离线。
//
// last_online_at 在「数据上报」与「状态心跳」两条路上都会更新（见 gatewayingest），
// 因此它就是网关的通用存活信号。
func markStaleGatewaysOffline(c *Client, beforeMs int64) (int64, error) {
	if beforeMs <= 0 {
		return 0, nil
	}
	tx := c.Pool.Table(models.TableGateway).
		Where("status = ? AND last_online_at > 0 AND last_online_at < ?",
			models.GatewayStatusOnline, beforeMs).
		Updates(map[string]interface{}{
			"status":   models.GatewayStatusOffline,
			"modified": time.Now().UnixMilli(),
		})
	if tx.Error != nil {
		return 0, errort.NewCommonEdgeX(errort.DefaultSystemError, "mark stale gateways offline failed", tx.Error)
	}
	return tx.RowsAffected, nil
}
