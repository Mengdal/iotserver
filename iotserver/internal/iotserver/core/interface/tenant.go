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
	// TenantBindThingsProject 绑定已有 things 项目；ProjectId=0 时用 Name 创建新项目。
	TenantBindThingsProject(ctx context.Context, id string, req dtos.TenantThingsBindRequest) error
	// TenantRenameThingsProject 修改绑定的 things 项目名称。
	TenantRenameThingsProject(ctx context.Context, id string, name string) error
}
