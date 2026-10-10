package interfaces

import (
	"context"

	"iotserver/internal/dtos"
)

// SceneItf 高级能力 - 场景联动
type SceneItf interface {
	SceneSearch(ctx context.Context, req dtos.SceneSearchQueryRequest) ([]dtos.SceneSearchQueryResponse, uint32, error)
	SceneAdd(ctx context.Context, req dtos.SceneAddRequest) error
	SceneUpdate(ctx context.Context, req dtos.SceneUpdateRequest) error
	SceneDelete(ctx context.Context, id string) error
	SceneStart(ctx context.Context, id string) error
	SceneStop(ctx context.Context, id string) error
	SceneLog(ctx context.Context, sceneId string, req dtos.SceneLogSearchQueryRequest) ([]dtos.SceneLogResponse, uint32, error)
	// ExecuteSceneByRule eKuiper 回调：命中规则后执行场景动作
	ExecuteSceneByRule(ctx context.Context, payload map[string]interface{}) error
}
