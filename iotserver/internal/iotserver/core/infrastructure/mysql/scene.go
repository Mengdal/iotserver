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

/******************************* 高级能力 - 场景联动 *******************************/

func addScene(c *Client, s models.Scene) (models.Scene, error) {
	if len(s.Id) == 0 {
		s.Id = utils.RandomNum()
	}
	var count int64
	if err := c.Pool.Table(models.TableScene).Where("name = ?", s.Name).Count(&count).Error; err != nil {
		return s, errort.NewCommonEdgeX(errort.DefaultSystemError, "scene query failed", err)
	}
	if count > 0 {
		return s, errort.NewCommonEdgeX(errort.DefaultNameRepeat, fmt.Sprintf("scene %s already exists", s.Name), nil)
	}
	if s.Status == "" {
		s.Status = string(constants.SceneStop)
	}
	if s.Conditions == nil {
		s.Conditions = models.SceneConditions{}
	}
	if s.Actions == nil {
		s.Actions = models.SceneActions{}
	}
	now := time.Now().UnixMilli()
	s.Timestamps = models.Timestamps{Created: now, Modified: now}
	if err := c.Pool.Table(s.TableName()).Create(&s).Error; err != nil {
		return s, errort.NewCommonEdgeX(errort.DefaultSystemError, "scene creation failed", err)
	}
	return s, nil
}

func sceneById(c *Client, id string) (models.Scene, error) {
	var s models.Scene
	if err := c.Pool.Table(models.TableScene).Where("id = ?", id).First(&s).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return s, errort.NewCommonErr(errort.DefaultResourcesNotFound, fmt.Errorf("scene %s not found", id))
		}
		return s, errort.NewCommonEdgeX(errort.DefaultSystemError, "scene query failed", err)
	}
	return s, nil
}

func sceneSearch(c *Client, offset, limit int, req dtos.SceneSearchQueryRequest) ([]models.Scene, uint32, error) {
	var (
		list  []models.Scene
		total int64
	)
	query := c.Pool.Table(models.TableScene)
	if req.Status != "" {
		query = query.Where("status = ?", req.Status)
	}
	if req.Name != "" {
		query = query.Where("name LIKE ?", "%"+escapeLike(req.Name)+"%")
	}
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, errort.NewCommonEdgeX(errort.DefaultSystemError, "scene count failed", err)
	}
	if err := query.Order("created desc").Offset(offset).Limit(limit).Find(&list).Error; err != nil {
		return nil, 0, errort.NewCommonEdgeX(errort.DefaultSystemError, "scene search failed", err)
	}
	return list, uint32(total), nil
}

func updateScene(c *Client, s models.Scene) error {
	var count int64
	if err := c.Pool.Table(models.TableScene).
		Where("name = ? AND id <> ?", s.Name, s.Id).Count(&count).Error; err != nil {
		return errort.NewCommonEdgeX(errort.DefaultSystemError, "scene query failed", err)
	}
	if count > 0 {
		return errort.NewCommonEdgeX(errort.DefaultNameRepeat, fmt.Sprintf("scene %s already exists", s.Name), nil)
	}
	return c.Pool.Table(models.TableScene).Where("id = ?", s.Id).Updates(map[string]interface{}{
		"name":        s.Name,
		"description": s.Describe,
		"conditions":  s.Conditions,
		"actions":     s.Actions,
		"modified":    time.Now().UnixMilli(),
	}).Error
}

func deleteScene(c *Client, id string) error {
	res := c.Pool.Table(models.TableScene).Where("id = ?", id).Delete(&models.Scene{})
	if res.Error != nil {
		return errort.NewCommonEdgeX(errort.DefaultSystemError, "scene delete failed", res.Error)
	}
	if res.RowsAffected == 0 {
		return errort.NewCommonErr(errort.DefaultResourcesNotFound, fmt.Errorf("scene %s not found", id))
	}
	return nil
}

func updateSceneStatus(c *Client, id, status string) error {
	res := c.Pool.Table(models.TableScene).Where("id = ?", id).Updates(map[string]interface{}{
		"status":   status,
		"modified": time.Now().UnixMilli(),
	})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errort.NewCommonErr(errort.DefaultResourcesNotFound, fmt.Errorf("scene %s not found", id))
	}
	return nil
}

func sceneRunningTimers(c *Client) ([]models.Scene, error) {
	var list []models.Scene
	if err := c.Pool.Table(models.TableScene).
		Where("status = ?", string(constants.SceneStart)).Find(&list).Error; err != nil {
		return nil, errort.NewCommonEdgeX(errort.DefaultSystemError, "scene query failed", err)
	}
	return list, nil
}

/** 场景日志 **/

func addSceneLog(c *Client, log models.SceneLog) error {
	if len(log.Id) == 0 {
		log.Id = utils.RandomNum()
	}
	now := time.Now().UnixMilli()
	if log.TriggerTime == 0 {
		log.TriggerTime = now
	}
	log.Timestamps = models.Timestamps{Created: now, Modified: now}
	return c.Pool.Table(models.TableSceneLog).Create(&log).Error
}

func sceneLogSearch(c *Client, offset, limit int, req dtos.SceneLogSearchQueryRequest) ([]models.SceneLog, uint32, error) {
	var (
		list  []models.SceneLog
		total int64
	)
	query := c.Pool.Table(models.TableSceneLog).Where("scene_id = ?", req.SceneId)
	if req.StartTime > 0 {
		query = query.Where("created >= ?", req.StartTime)
	}
	if req.EndTime > 0 {
		query = query.Where("created <= ?", req.EndTime)
	}
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, errort.NewCommonEdgeX(errort.DefaultSystemError, "scene log count failed", err)
	}
	if err := query.Order("created desc").Offset(offset).Limit(limit).Find(&list).Error; err != nil {
		return nil, 0, errort.NewCommonEdgeX(errort.DefaultSystemError, "scene log search failed", err)
	}
	return list, uint32(total), nil
}

func deleteSceneLogsBySceneId(c *Client, sceneId string) error {
	return c.Pool.Table(models.TableSceneLog).Where("scene_id = ?", sceneId).Delete(&models.SceneLog{}).Error
}

func (c *Client) AddScene(s models.Scene) (models.Scene, error) {
	return addScene(c, s)
}

func (c *Client) SceneById(id string) (models.Scene, error) {
	return sceneById(c, id)
}

func (c *Client) SceneSearch(offset, limit int, req dtos.SceneSearchQueryRequest) ([]models.Scene, uint32, error) {
	return sceneSearch(c, offset, limit, req)
}

func (c *Client) UpdateScene(s models.Scene) error {
	return updateScene(c, s)
}

func (c *Client) DeleteScene(id string) error {
	return deleteScene(c, id)
}

func (c *Client) UpdateSceneStatus(id, status string) error {
	return updateSceneStatus(c, id, status)
}

func (c *Client) SceneRunningTimers() ([]models.Scene, error) {
	return sceneRunningTimers(c)
}

func (c *Client) AddSceneLog(log models.SceneLog) error {
	return addSceneLog(c, log)
}

func (c *Client) SceneLogSearch(offset, limit int, req dtos.SceneLogSearchQueryRequest) ([]models.SceneLog, uint32, error) {
	return sceneLogSearch(c, offset, limit, req)
}

func (c *Client) DeleteSceneLogsBySceneId(sceneId string) error {
	return deleteSceneLogsBySceneId(c, sceneId)
}
