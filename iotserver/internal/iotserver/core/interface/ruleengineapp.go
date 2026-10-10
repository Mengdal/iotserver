package interfaces

import (
	"context"

	"iotserver/internal/dtos"
)

// RuleEngineItf 高级能力 - 规则引擎
type RuleEngineItf interface {
	RuleEngineSearch(ctx context.Context, req dtos.RuleEngineSearchQueryRequest) ([]dtos.RuleEngineSearchQueryResponse, uint32, error)
	RuleEngineById(ctx context.Context, id string) (dtos.RuleEngineSearchQueryResponse, error)
	RuleEngineStatus(ctx context.Context, id string) (map[string]interface{}, error)
	RuleEngineAdd(ctx context.Context, req dtos.RuleEngineAddRequest) error
	RuleEngineUpdate(ctx context.Context, req dtos.RuleEngineUpdateRequest) error
	RuleEngineDelete(ctx context.Context, id string) error
	RuleEngineStart(ctx context.Context, id string) error
	RuleEngineStop(ctx context.Context, id string) error
}
