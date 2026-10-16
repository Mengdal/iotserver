package models

import "sync"

// Tenant 租户。ROBX 作为平台管理员在一个实例内管理多个租户，
// 每个租户的数据通过 tenant_id 行级隔离。
type Tenant struct {
	Timestamps   `gorm:"embedded"`
	Id           string `gorm:"column:id;primaryKey;type:string;size:64;comment:主键"`
	Code         string `gorm:"column:code;type:string;size:64;comment:租户编码"`
	Name         string `gorm:"column:name;type:string;size:255;comment:租户名称"`
	Status       int    `gorm:"column:status;type:int;default:1;comment:状态 1正常 0停用"`
	ContactName  string `gorm:"column:contact_name;type:string;size:128;comment:联系人"`
	ContactPhone string `gorm:"column:contact_phone;type:string;size:64;comment:联系电话"`
	ContactEmail string `gorm:"column:contact_email;type:string;size:255;comment:联系邮箱"`
	// ExpireAt 到期时间（毫秒），0 表示永久
	ExpireAt int64 `gorm:"column:expire_at;comment:到期时间"`
	// 配额，0 表示不限
	MaxDevices  int `gorm:"column:max_devices;type:int;default:0;comment:设备配额"`
	MaxGateways int `gorm:"column:max_gateways;type:int;default:0;comment:网关配额"`
	MaxUsers    int `gorm:"column:max_users;type:int;default:0;comment:用户配额"`
	// RetentionDays 该租户的时序数据保留天数：
	//   >0 按该天数清理（覆盖系统全局配置）
	//   =0 用系统全局配置（sys_config 的 data.retentionDays）
	//   <0 永久保留，不清理
	RetentionDays int    `gorm:"column:retention_days;type:int;default:0;comment:时序数据保留天数(0=用全局配置,-1=永久)"`
	Remark        string `gorm:"column:remark;type:string;size:500;comment:备注"`
}

func (table *Tenant) TableName() string {
	return "tenant"
}

func (table *Tenant) Get() interface{} {
	return *table
}

// TableTenant 租户表名
const TableTenant = "tenant"

// TenantField 供需要行级租户隔离的模型内嵌：提供统一的 tenant_id 列。
// 隔离由 GORM 租户插件在查询/更新/删除时自动追加条件（见 tools/sqldb/tenant）。
type TenantField struct {
	TenantId string `json:"tenantId,omitempty" gorm:"column:tenant_id;type:string;size:64;index;comment:租户ID"`
}

// tableNamer 便于从模型实例推导表名
type tableNamer interface {
	TableName() string
}

// tenantScopedModels 需要「行级租户隔离」的表清单。
//
// 约定：这里的表**必须**有 tenant_id 列，且数据严格属于某个租户。
// 不含：
//   - 平台级共享表（sys_menu / unit / category_template / thing_model_template / doc / language_sdk / driver_classify）
//   - 「全局默认 + 租户覆盖」表（sys_config、solution），它们由应用层显式处理覆盖逻辑
//   - 物模型能力表（properties / events / actions）：始终通过 product_id 访问，由 product 的租户归属间接保证
//   - 驱动资源表（device_library / device_service / docker_config）：阶段一按平台级共享处理
//
// 新增带 tenant_id 的业务表时，务必同步登记到这里。
var tenantScopedModels = []tableNamer{
	&Product{},
	&Device{},
	&AlertRule{},
	&AlertList{},
	&AlertContent{},
	&RuleEngine{},
	&Scene{},
	&SceneLog{},
	&DataResource{},
	&MqttAuth{},
	&User{},
	&Dept{},
	&Role{},
	&AuditLog{},
	&Metrics{},
	&SystemMetrics{},
	&ServiceStats{},
	&MsgGather{},
	&Gateway{},
	&GatewayIngestLog{},
	&DeviceTag{},
	&VizBoard{},
}

var (
	tenantTablesOnce sync.Once
	tenantTables     map[string]struct{}
)

// TenantScopedTableNames 返回需要租户隔离的表名集合
func TenantScopedTableNames() map[string]struct{} {
	tenantTablesOnce.Do(func() {
		tenantTables = make(map[string]struct{}, len(tenantScopedModels))
		for _, m := range tenantScopedModels {
			tenantTables[m.TableName()] = struct{}{}
		}
	})
	return tenantTables
}

// IsTenantScopedTable 判断表是否需要租户隔离
func IsTenantScopedTable(table string) bool {
	if table == "" {
		return false
	}
	_, ok := TenantScopedTableNames()[table]
	return ok
}
