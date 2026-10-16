package interfaces

import (
	"context"

	"iotserver/internal/dtos"
)

// TenantItf 租户管理：平台级能力
type TenantItf interface {
	TenantAdd(ctx context.Context, req dtos.TenantAddRequest) (dtos.TenantResponse, error)
	TenantUpdate(ctx context.Context, req dtos.TenantUpdateRequest) error
	TenantDelete(ctx context.Context, id string) error
	TenantById(ctx context.Context, id string) (dtos.TenantResponse, error)
	TenantSearch(ctx context.Context, req dtos.TenantSearchQueryRequest) ([]dtos.TenantResponse, uint32, error)
}
