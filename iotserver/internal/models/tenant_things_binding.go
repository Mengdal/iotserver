package models

// 租户与 things 项目绑定状态。
const (
	TenantThingsStatusPending  = "pending"
	TenantThingsStatusBound    = "bound"
	TenantThingsStatusError    = "error"
	TenantThingsStatusOrphaned = "orphaned"
	TenantThingsStatusDeleting = "deleting"
)

// TenantThingsBinding 租户 ↔ things 项目 1:1 绑定。
//
// 这是平台级元数据，但带有 tenant_id 并登记进 tenantScopedModels，
// 避免普通租户上下文误读其他租户的绑定。平台任务使用 tenant.Platform 作用域。
type TenantThingsBinding struct {
	Timestamps  `gorm:"embedded"`
	TenantField `gorm:"embedded"`
	Id          string `gorm:"column:id;primaryKey;type:string;size:64;comment:主键"`
	// ThingsProjectId things edge_project.id
	ThingsProjectId int64 `gorm:"column:things_project_id;uniqueIndex;comment:things 项目ID"`
	// ThingsProjectName 冗余项目名，便于列表展示和对账
	ThingsProjectName string `gorm:"column:things_project_name;type:string;size:255;comment:things 项目名称"`
	// Status 见 TenantThingsStatus*
	Status     string `gorm:"column:status;type:string;size:32;default:'pending';comment:绑定状态"`
	LastSyncAt int64  `gorm:"column:last_sync_at;comment:最后同步时间"`
	LastError  string `gorm:"column:last_error;type:string;size:1000;comment:最后错误"`
}

func (table *TenantThingsBinding) TableName() string {
	return "tenant_things_binding"
}

func (table *TenantThingsBinding) Get() interface{} {
	return *table
}

// TableTenantThingsBinding 租户-things 绑定表名
const TableTenantThingsBinding = "tenant_things_binding"
