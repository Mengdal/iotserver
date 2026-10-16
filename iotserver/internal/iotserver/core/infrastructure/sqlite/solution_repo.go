package sqlite

import (
	"time"

	"iotserver/internal/models"
)

// BatchUpsertSolution 按 key 幂等写入方案（平台内置模板 tenant_id 为空）。
// 返回写入条数。已存在则更新内容与版本，用于模板内容升级。
//
// 只匹配内置行（tenant_id=”）：租户可以克隆出同 key 的副本，
// 若这里按 key 盲匹配，重新 seed 可能改写租户副本（甚至把副本改成内置）。
func batchUpsertSolution(c *Client, list []models.Solution) (int64, error) {
	if len(list) == 0 {
		return 0, nil
	}
	now := time.Now().UnixMilli()
	var affected int64
	for _, t := range list {
		var exist models.Solution
		err := c.Pool.Table(models.TableSolution).Where("`key` = ? AND tenant_id = '' AND built_in = 1", t.Key).First(&exist).Error
		if err != nil {
			// 不存在 → 新建
			if t.Id == "" {
				t.Id = t.Key
			}
			t.Timestamps = models.Timestamps{Created: now, Modified: now}
			if createErr := c.Pool.Table(models.TableSolution).Create(&t).Error; createErr != nil {
				return affected, createErr
			}
			affected++
			continue
		}
		// 已存在 → 更新内容（保留 created 与内置标记）
		if err := c.Pool.Table(models.TableSolution).Where("`key` = ? AND tenant_id = '' AND built_in = 1", t.Key).Updates(map[string]interface{}{
			"tenant_id":        t.TenantId,
			"name":             t.Name,
			"industry":         t.Industry,
			"version":          t.Version,
			"built_in":         t.BuiltIn,
			"icon":             t.Icon,
			"description":      t.Description,
			"thing_model_json": t.ThingModelJson,
			"alarm_rules_json": t.AlarmRulesJson,
			"dashboard_json":   t.DashboardJson,
			"metrics_json":     t.MetricsJson,
			"modified":         now,
		}).Error; err != nil {
			return affected, err
		}
		affected++
	}
	return affected, nil
}

// dedupSolutionByKey 同一 key 只保留一条，租户副本优先于内置模板。
//
// SolutionSearch 会同时命中内置行与租户克隆的同 key 副本，
// 直接返回会让前端出现重复方案；保留租户副本才能反映租户自己的改动。
func dedupSolutionByKey(list []models.Solution) []models.Solution {
	indexOf := make(map[string]int, len(list))
	out := make([]models.Solution, 0, len(list))
	for _, s := range list {
		idx, ok := indexOf[s.Key]
		if !ok {
			indexOf[s.Key] = len(out)
			out = append(out, s)
			continue
		}
		// 已存在同 key：副本（租户副本/平台副本）覆盖内置行，内置行不覆盖副本
		rank := func(x models.Solution) int {
			if x.TenantId != "" {
				return 0 // 租户自己的副本
			}
			if x.BuiltIn == 0 {
				return 1 // 平台级覆盖副本
			}
			return 2 // 平台内置行
		}
		if rank(s) < rank(out[idx]) {
			out[idx] = s
		}
	}
	return out
}

func solutionSearch(c *Client, tenantId, industry string) ([]models.Solution, error) {
	var list []models.Solution
	query := c.Pool.Table(models.TableSolution)
	if tenantId == "" {
		query = query.Where("tenant_id = ''")
	} else {
		query = query.Where("tenant_id IN ?", []string{"", tenantId})
	}
	if industry != "" {
		query = query.Where("industry = ?", industry)
	}
	if err := query.Order("industry asc, name asc").Find(&list).Error; err != nil {
		return nil, err
	}
	return dedupSolutionByKey(list), nil
}

func solutionByKey(c *Client, tenantId, key string) (models.Solution, error) {
	var t models.Solution
	query := c.Pool.Table(models.TableSolution).Where("`key` = ?", key)
	if tenantId == "" {
		query = query.Where("tenant_id = ''")
	} else {
		query = query.Where("tenant_id IN ?", []string{"", tenantId})
	}
	// 同 key 时的优先级：租户副本 > 平台副本(内置覆盖) > 内置行
	if err := query.Order("built_in asc, tenant_id desc").First(&t).Error; err != nil {
		return t, err
	}
	return t, nil
}

// addSolution 新增方案记录（克隆租户副本用）
func addSolution(c *Client, s models.Solution) error {
	now := time.Now().UnixMilli()
	if s.Id == "" {
		s.Id = s.Key
	}
	s.Timestamps = models.Timestamps{Created: now, Modified: now}
	return c.Pool.Table(models.TableSolution).Create(&s).Error
}

// updateSolutionThingModel 只更新物模型 JSON，按主键定位，绝不触及内置行
func updateSolutionThingModel(c *Client, id, thingModelJson string) error {
	return c.Pool.Table(models.TableSolution).Where("id = ?", id).Updates(map[string]interface{}{
		"thing_model_json": thingModelJson,
		"modified":         time.Now().UnixMilli(),
	}).Error
}

func (c *Client) BatchUpsertSolution(list []models.Solution) (int64, error) {
	return batchUpsertSolution(c, list)
}

func (c *Client) SolutionSearch(tenantId, industry string) ([]models.Solution, error) {
	return solutionSearch(c, tenantId, industry)
}

func (c *Client) SolutionByKey(tenantId, key string) (models.Solution, error) {
	return solutionByKey(c, tenantId, key)
}

func (c *Client) AddSolution(s models.Solution) error {
	return addSolution(c, s)
}

func (c *Client) UpdateSolutionThingModel(id, thingModelJson string) error {
	return updateSolutionThingModel(c, id, thingModelJson)
}
