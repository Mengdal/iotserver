package interfaces

import (
	"iotserver/internal/dtos"
	"iotserver/internal/models"
)

// RuleEngineDB 高级能力-规则引擎仓储
type RuleEngineDB interface {
	AddRuleEngine(r models.RuleEngine) (models.RuleEngine, error)
	RuleEngineById(id string) (models.RuleEngine, error)
	RuleEngineSearch(offset, limit int, req dtos.RuleEngineSearchQueryRequest) ([]models.RuleEngine, uint32, error)
	UpdateRuleEngine(r models.RuleEngine) error
	DeleteRuleEngine(id string) error
	// UpdateRuleEngineStatus 更新运行状态（running/stopped）
	UpdateRuleEngineStatus(id, status string) error
}
