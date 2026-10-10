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

// escapeLike 转义 LIKE 的通配符，避免用户输入 % 或 _ 时匹配到全部记录
func escapeLike(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "%", "\\%")
	s = strings.ReplaceAll(s, "_", "\\_")
	return s
}

/******************************* 高级能力 - 资源管理 *******************************/

func addDataResource(c *Client, r models.DataResource) (models.DataResource, error) {
	if len(r.Id) == 0 {
		r.Id = utils.RandomNum()
	}
	var count int64
	if err := c.Pool.Table(models.TableDataResource).
		Where("type = ? AND name = ?", r.Type, r.Name).Count(&count).Error; err != nil {
		return r, errort.NewCommonEdgeX(errort.DefaultSystemError, "data resource query failed", err)
	}
	if count > 0 {
		return r, errort.NewCommonEdgeX(errort.DefaultNameRepeat,
			fmt.Sprintf("data resource %s already exists in type %s", r.Name, r.Type), nil)
	}
	now := time.Now().UnixMilli()
	r.Timestamps = models.Timestamps{Created: now, Modified: now}
	if err := c.Pool.Table(r.TableName()).Create(&r).Error; err != nil {
		return r, errort.NewCommonEdgeX(errort.DefaultSystemError, "data resource creation failed", err)
	}
	return r, nil
}

func dataResourceById(c *Client, id string) (models.DataResource, error) {
	var r models.DataResource
	if err := c.Pool.Table(models.TableDataResource).Where("id = ?", id).First(&r).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return r, errort.NewCommonErr(errort.DefaultResourcesNotFound, fmt.Errorf("data resource %s not found", id))
		}
		return r, errort.NewCommonEdgeX(errort.DefaultSystemError, "data resource query failed", err)
	}
	return r, nil
}

// dataResourceByIds 按 id 批量查询（规则列表关联资源用，避免 N+1）
func dataResourceByIds(c *Client, ids []string) ([]models.DataResource, error) {
	if len(ids) == 0 {
		return []models.DataResource{}, nil
	}
	var list []models.DataResource
	if err := c.Pool.Table(models.TableDataResource).Where("id IN ?", ids).Find(&list).Error; err != nil {
		return nil, errort.NewCommonEdgeX(errort.DefaultSystemError, "data resource query failed", err)
	}
	return list, nil
}

func dataResourceSearch(c *Client, offset, limit int, req dtos.DataResourceSearchRequest) ([]models.DataResource, uint32, error) {
	var (
		list  []models.DataResource
		total int64
	)
	query := c.Pool.Table(models.TableDataResource)
	if req.Type != "" {
		query = query.Where("type = ?", req.Type)
	}
	// 名称模糊搜索（前端资源下拉的 onSearch）
	if req.Name != "" {
		query = query.Where("name LIKE ?", "%"+escapeLike(req.Name)+"%")
	}
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, errort.NewCommonEdgeX(errort.DefaultSystemError, "data resource count failed", err)
	}
	if err := query.Order("created desc").Offset(offset).Limit(limit).Find(&list).Error; err != nil {
		return nil, 0, errort.NewCommonEdgeX(errort.DefaultSystemError, "data resource search failed", err)
	}
	return list, uint32(total), nil
}

func updateDataResource(c *Client, r models.DataResource) error {
	old, err := dataResourceById(c, r.Id)
	if err != nil {
		return err
	}
	// 同类型内名称唯一（排除自己）
	var count int64
	if err := c.Pool.Table(models.TableDataResource).
		Where("type = ? AND name = ? AND id <> ?", r.Type, r.Name, r.Id).Count(&count).Error; err != nil {
		return errort.NewCommonEdgeX(errort.DefaultSystemError, "data resource query failed", err)
	}
	if count > 0 {
		return errort.NewCommonEdgeX(errort.DefaultNameRepeat,
			fmt.Sprintf("data resource %s already exists in type %s", r.Name, r.Type), nil)
	}
	return c.Pool.Table(models.TableDataResource).Where("id = ?", r.Id).Updates(map[string]interface{}{
		"name":        r.Name,
		"type":        r.Type,
		"option_json": r.Option,
		"modified":    time.Now().UnixMilli(),
		"created":     old.Created,
	}).Error
}

func deleteDataResource(c *Client, id string) error {
	res := c.Pool.Table(models.TableDataResource).Where("id = ?", id).Delete(&models.DataResource{})
	if res.Error != nil {
		return errort.NewCommonEdgeX(errort.DefaultSystemError, "data resource delete failed", res.Error)
	}
	if res.RowsAffected == 0 {
		return errort.NewCommonErr(errort.DefaultResourcesNotFound, fmt.Errorf("data resource %s not found", id))
	}
	return nil
}

// updateDataResourceHealth 写回最近一次验证结果
func updateDataResourceHealth(c *Client, id string, health bool) error {
	res := c.Pool.Table(models.TableDataResource).Where("id = ?", id).Updates(map[string]interface{}{
		"health":   health,
		"modified": time.Now().UnixMilli(),
	})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errort.NewCommonErr(errort.DefaultResourcesNotFound, fmt.Errorf("data resource %s not found", id))
	}
	return nil
}

func (c *Client) AddDataResource(r models.DataResource) (models.DataResource, error) {
	return addDataResource(c, r)
}

func (c *Client) DataResourceById(id string) (models.DataResource, error) {
	return dataResourceById(c, id)
}

func (c *Client) DataResourceByIds(ids []string) ([]models.DataResource, error) {
	return dataResourceByIds(c, ids)
}

func (c *Client) DataResourceSearch(offset, limit int, req dtos.DataResourceSearchRequest) ([]models.DataResource, uint32, error) {
	return dataResourceSearch(c, offset, limit, req)
}

func (c *Client) UpdateDataResource(r models.DataResource) error {
	return updateDataResource(c, r)
}

func (c *Client) DeleteDataResource(id string) error {
	return deleteDataResource(c, id)
}

func (c *Client) UpdateDataResourceHealth(id string, health bool) error {
	return updateDataResourceHealth(c, id, health)
}
