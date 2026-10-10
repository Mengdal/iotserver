package interfaces

import (
	"context"

	"iotserver/internal/dtos"
	"iotserver/internal/pkg/constants"
)

// ResourceItf 高级能力 - 资源管理
type ResourceItf interface {
	DataResourceSearch(ctx context.Context, req dtos.DataResourceSearchRequest) ([]dtos.DataResourceResponse, uint32, error)
	TypeResource(ctx context.Context) []constants.DataResourceType
	DataResourceAdd(ctx context.Context, req dtos.DataResourceAddRequest) error
	DataResourceUpdate(ctx context.Context, req dtos.DataResourceUpdateRequest) error
	DataResourceDelete(ctx context.Context, id string) error
	DataResourceVerify(ctx context.Context, id string) (dtos.DataResourceVerifyResponse, error)
}
