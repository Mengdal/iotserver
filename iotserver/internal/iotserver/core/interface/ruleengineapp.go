package interfaces

import (
	"context"
	"io"

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
	RuleEngineBatchStart(ctx context.Context, ids []string) dtos.RuleEngineBatchResult
	RuleEngineBatchStop(ctx context.Context, ids []string) dtos.RuleEngineBatchResult
	RuleEngineValidate(ctx context.Context, sql string) (dtos.RuleEngineValidateResponse, error)
	RuleEngineDebugStart(ctx context.Context, req dtos.RuleEngineDebugRequest) (string, error)
	RuleEngineDebugStream(ctx context.Context, id string) (io.ReadCloser, error)
	RuleEngineDebugStop(ctx context.Context, id string) error
	RuleEngineDebugDeliver(ctx context.Context, id, payload string) []dtos.RuleEngineDebugActionResult
	RuleEngineExport(ctx context.Context, ids []string) (dtos.RuleEngineExportFile, error)
	RuleEngineImport(ctx context.Context, req dtos.RuleEngineImportRequest) dtos.RuleEngineImportResult
}
