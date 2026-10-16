package mysql

import (
	"time"

	"gorm.io/gorm"

	"iotserver/internal/models"
	"iotserver/internal/pkg/errort"
)

// 看板落库（多租户）：viz_board 已登记为租户隔离表，
// 查询/写入由 GORM 租户插件自动带上 tenant_id，这里无需手写租户条件。

func vizBoards(c *Client) ([]models.VizBoard, error) {
	var list []models.VizBoard
	if err := c.Pool.Table(models.TableVizBoard).Order("update_at desc").Find(&list).Error; err != nil {
		return nil, errort.NewCommonEdgeX(errort.DefaultSystemError, "viz board query failed", err)
	}
	return list, nil
}

// vizBoardsReplace 覆盖式保存当前租户的看板：列表内的 upsert，列表外（已删除）的删除。
//
// 管理接口并发很低，且前端是「读全量 → 改动 → 写全量」的模式，因此用事务整体替换最稳妥。
func vizBoardsReplace(c *Client, list []models.VizBoard) error {
	now := time.Now().UnixMilli()
	return c.Pool.Transaction(func(tx *gorm.DB) error {
		ids := make([]string, 0, len(list))
		for _, b := range list {
			ids = append(ids, b.Id)
			var count int64
			if err := tx.Table(models.TableVizBoard).Where("id = ?", b.Id).Count(&count).Error; err != nil {
				return err
			}
			if count > 0 {
				if err := tx.Table(models.TableVizBoard).Where("id = ?", b.Id).Updates(map[string]interface{}{
					"name":       b.Name,
					"icon":       b.Icon,
					"canvas":     b.Canvas,
					"is_default": b.IsDefault,
					"is_home":    b.IsHome,
					"update_at":  b.UpdateAt,
					"modified":   now,
				}).Error; err != nil {
					return err
				}
				continue
			}
			b.Timestamps = models.Timestamps{Created: now, Modified: now}
			if err := tx.Table(models.TableVizBoard).Create(&b).Error; err != nil {
				return err
			}
		}
		if len(ids) == 0 {
			return tx.Table(models.TableVizBoard).Where("1 = 1").Delete(&models.VizBoard{}).Error
		}
		return tx.Table(models.TableVizBoard).Where("id NOT IN ?", ids).Delete(&models.VizBoard{}).Error
	})
}

func (c *Client) VizBoards() ([]models.VizBoard, error) {
	return vizBoards(c)
}

func (c *Client) VizBoardsReplace(list []models.VizBoard) error {
	return vizBoardsReplace(c, list)
}
