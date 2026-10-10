package interfaces

import (
	"iotserver/internal/dtos"
	"iotserver/internal/models"
)

// DataResourceDB 高级能力-资源管理仓储
type DataResourceDB interface {
	AddDataResource(r models.DataResource) (models.DataResource, error)
	DataResourceById(id string) (models.DataResource, error)
	DataResourceByIds(ids []string) ([]models.DataResource, error)
	DataResourceSearch(offset, limit int, req dtos.DataResourceSearchRequest) ([]models.DataResource, uint32, error)
	UpdateDataResource(r models.DataResource) error
	DeleteDataResource(id string) error
	// UpdateDataResourceHealth 写回最近一次验证结果
	UpdateDataResourceHealth(id string, health bool) error
}

// AdvancedDB 高级能力相关仓储（资源管理 / 规则引擎 / 场景联动）
type AdvancedDB interface {
	DataResourceDB
	RuleEngineDB
	SceneDB
}
