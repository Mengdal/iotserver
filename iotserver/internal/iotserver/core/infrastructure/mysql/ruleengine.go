package mysql

import (
	"fmt"
	"time"

	"gorm.io/gorm"

	"iotserver/internal/dtos"
	"iotserver/internal/models"
	"iotserver/internal/pkg/constants"
	"iotserver/internal/pkg/errort"
	"iotserver/internal/pkg/utils"
)

/******************************* 高级能力 - 规则引擎 *******************************/

func addRuleEngine(c *Client, r models.RuleEngine) (models.RuleEngine, error) {
	if len(r.Id) == 0 {
		r.Id = utils.RandomNum()
	}
	var count int64
	if err := c.Pool.Table(models.TableRuleEngine).Where("name = ?", r.Name).Count(&count).Error; err != nil {
		return r, errort.NewCommonEdgeX(errort.DefaultSystemError, "rule engine query failed", err)
	}
	if count > 0 {
		return r, errort.NewCommonEdgeX(errort.DefaultNameRepeat, fmt.Sprintf("rule engine %s already exists", r.Name), nil)
	}
	if r.Status == "" {
		r.Status = string(constants.RuleEngineStop)
	}
	now := time.Now().UnixMilli()
	r.Timestamps = models.Timestamps{Created: now, Modified: now}
	if err := c.Pool.Table(r.TableName()).Create(&r).Error; err != nil {
		return r, errort.NewCommonEdgeX(errort.DefaultSystemError, "rule engine creation failed", err)
	}
	return r, nil
}

func ruleEngineById(c *Client, id string) (models.RuleEngine, error) {
	var r models.RuleEngine
	if err := c.Pool.Table(models.TableRuleEngine).Where("id = ?", id).First(&r).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return r, errort.NewCommonErr(errort.DefaultResourcesNotFound, fmt.Errorf("rule engine %s not found", id))
		}
		return r, errort.NewCommonEdgeX(errort.DefaultSystemError, "rule engine query failed", err)
	}
	return r, nil
}

func ruleEngineSearch(c *Client, offset, limit int, req dtos.RuleEngineSearchQueryRequest) ([]models.RuleEngine, uint32, error) {
	var (
		list  []models.RuleEngine
		total int64
	)
	query := c.Pool.Table(models.TableRuleEngine)
	if req.Status != "" {
		query = query.Where("status = ?", req.Status)
	}
	// 前端 name 是模糊搜索
	if req.NameLike != "" {
		query = query.Where("name LIKE ?", "%"+escapeLike(req.NameLike)+"%")
	} else if req.Name != "" {
		query = query.Where("name LIKE ?", "%"+escapeLike(req.Name)+"%")
	}
	if req.Id != "" {
		query = query.Where("id = ?", req.Id)
	}
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, errort.NewCommonEdgeX(errort.DefaultSystemError, "rule engine count failed", err)
	}
	if err := query.Order("created desc").Offset(offset).Limit(limit).Find(&list).Error; err != nil {
		return nil, 0, errort.NewCommonEdgeX(errort.DefaultSystemError, "rule engine search failed", err)
	}
	return list, uint32(total), nil
}

func updateRuleEngine(c *Client, r models.RuleEngine) error {
	var count int64
	if err := c.Pool.Table(models.TableRuleEngine).
		Where("name = ? AND id <> ?", r.Name, r.Id).Count(&count).Error; err != nil {
		return errort.NewCommonEdgeX(errort.DefaultSystemError, "rule engine query failed", err)
	}
	if count > 0 {
		return errort.NewCommonEdgeX(errort.DefaultNameRepeat, fmt.Sprintf("rule engine %s already exists", r.Name), nil)
	}
	return c.Pool.Table(models.TableRuleEngine).Where("id = ?", r.Id).Updates(map[string]interface{}{
		"name":             r.Name,
		"description":      r.Describe,
		"data_resource_id": r.DataResourceId,
		"resource_type":    r.ResourceType,
		"filter":           r.Filter,
		"modified":         time.Now().UnixMilli(),
	}).Error
}

func deleteRuleEngine(c *Client, id string) error {
	res := c.Pool.Table(models.TableRuleEngine).Where("id = ?", id).Delete(&models.RuleEngine{})
	if res.Error != nil {
		return errort.NewCommonEdgeX(errort.DefaultSystemError, "rule engine delete failed", res.Error)
	}
	if res.RowsAffected == 0 {
		return errort.NewCommonErr(errort.DefaultResourcesNotFound, fmt.Errorf("rule engine %s not found", id))
	}
	return nil
}

func updateRuleEngineStatus(c *Client, id, status string) error {
	res := c.Pool.Table(models.TableRuleEngine).Where("id = ?", id).Updates(map[string]interface{}{
		"status":   status,
		"modified": time.Now().UnixMilli(),
	})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errort.NewCommonErr(errort.DefaultResourcesNotFound, fmt.Errorf("rule engine %s not found", id))
	}
	return nil
}

func (c *Client) AddRuleEngine(r models.RuleEngine) (models.RuleEngine, error) {
	return addRuleEngine(c, r)
}

func (c *Client) RuleEngineById(id string) (models.RuleEngine, error) {
	return ruleEngineById(c, id)
}

func (c *Client) RuleEngineSearch(offset, limit int, req dtos.RuleEngineSearchQueryRequest) ([]models.RuleEngine, uint32, error) {
	return ruleEngineSearch(c, offset, limit, req)
}

func (c *Client) UpdateRuleEngine(r models.RuleEngine) error {
	return updateRuleEngine(c, r)
}

func (c *Client) DeleteRuleEngine(id string) error {
	return deleteRuleEngine(c, id)
}

func (c *Client) UpdateRuleEngineStatus(id, status string) error {
	return updateRuleEngineStatus(c, id, status)
}
