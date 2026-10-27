package interfaces

import (
	"context"

	"iotserver/internal/dtos"
	"iotserver/internal/models"
)

// ThingsItf things 平台集成：项目/网关生命周期、租户绑定与本地镜像对账。
//
// 该接口刻意不提供 secret/license 的计算或校验能力：认证参数由调用方原样透传，
// 是否有效由 things 判定。
type ThingsItf interface {
	// Enabled 当前是否启用 things 集成。
	Enabled(ctx context.Context) (bool, error)
	// TestConnection 登录 things 并拉取一次项目列表。
	TestConnection(ctx context.Context) (dtos.ThingsConnectionStatus, error)
	// ListProjects 拉取 things 项目。
	ListProjects(ctx context.Context, keyword string) ([]dtos.ThingsProject, error)
	// CreateProject 创建 things 项目。
	CreateProject(ctx context.Context, name, description string) (dtos.ThingsProject, error)
	// RenameProject 修改 things 项目名称。
	RenameProject(ctx context.Context, projectID int64, name string) error
	// DeleteProject 删除 things 项目（使用配置的服务账号密码）。
	DeleteProject(ctx context.Context, projectID int64) error

	// GetTenantBinding 查询租户绑定。found=false 表示尚未绑定。
	GetTenantBinding(ctx context.Context, tenantID string) (models.TenantThingsBinding, bool, error)
	// BindTenantProject 将租户绑定到已有 things 项目。
	BindTenantProject(ctx context.Context, tenantID string, projectID int64, projectName string) error
	// UnbindTenant 删除租户绑定（不删除 things 项目）。
	UnbindTenant(ctx context.Context, tenantID string) error
	// UpdateBindingStatus 更新绑定状态/错误（用于失败重试与对账）。
	UpdateBindingStatus(ctx context.Context, tenantID, status, lastError string) error

	// CreateGateway 在 things 中创建网关。Secret/License 原样透传。
	CreateGateway(ctx context.Context, projectID int64, req dtos.ThingsGatewayCreateRequest) (dtos.ThingsGateway, error)
	// RenameGateway 在 things 中修改网关名称。
	RenameGateway(ctx context.Context, projectID, gatewayID int64, name string) error
	// DeleteGateway 在 things 中删除网关（使用配置的服务账号密码）。
	DeleteGateway(ctx context.Context, projectID, gatewayID int64) error

	// ReconcileTenant 对单个租户执行一次本地镜像对账。
	ReconcileTenant(ctx context.Context, tenantID string) (dtos.ThingsReconcileResult, error)
	// ReconcileAll 对所有已绑定租户执行对账。
	ReconcileAll(ctx context.Context) (dtos.ThingsReconcileResult, error)
	// Start 启动后台定期对账（配置关闭时不做任何事）。
	Start(ctx context.Context)
}
