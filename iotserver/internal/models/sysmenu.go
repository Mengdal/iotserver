package models

// 菜单类型
const (
	MenuTypeDir    = "M" // 目录
	MenuTypeMenu   = "C" // 菜单
	MenuTypeButton = "F" // 按钮
)

// Menu 菜单（同时承载前端菜单树与按钮权限）
type Menu struct {
	Timestamps `gorm:"embedded"`
	Id         string `gorm:"column:id;primaryKey;type:string;size:64;comment:主键"`
	ParentId   string `gorm:"column:parent_id;type:string;size:64;index;comment:父菜单ID"`
	Name       string `gorm:"column:name;type:string;size:255;comment:菜单名称"`
	// NameEn 英文菜单名称：英文界面优先显示；为空时回落到内置词条/中文名称。
	NameEn    string `gorm:"column:name_en;type:string;size:255;comment:英文菜单名称"`
	Path      string `gorm:"column:path;type:string;size:255;comment:路由地址"`
	Component string `gorm:"column:component;type:string;size:255;comment:组件路径"`
	Icon      string `gorm:"column:icon;type:string;size:128;comment:菜单图标"`
	MenuType  string `gorm:"column:menu_type;type:string;size:8;default:C;comment:菜单类型 M目录 C菜单 F按钮"`
	Perms     string `gorm:"column:perms;type:string;size:255;comment:权限标识"`
	Sort      int    `gorm:"column:sort;type:int;default:0;comment:显示顺序"`
	Visible   int    `gorm:"column:visible;type:int;default:1;comment:是否显示 1显示 0隐藏"`
	Status    int    `gorm:"column:status;type:int;default:1;comment:状态 1正常 0停用"`
	IsFrame   int    `gorm:"column:is_frame;type:int;default:1;comment:是否外链 1否 0是"`
	// PlatformOnly=1 表示平台级菜单（如「租户管理」），不下发给租户用户
	PlatformOnly int `gorm:"column:platform_only;type:int;default:0;comment:是否平台级菜单 1是 0否"`
	// TenantId 菜单归属：'' = 平台原始菜单（只有平台管理员能写）；非空 = 该租户的自建菜单。
	//
	// 注意：sys_menu **不**登记进 models.tenantScopedModels —— 登记后 GORM 插件会把
	// 全局行（tenant_id=''）和租户行一起过滤掉。读取统一由应用层显式拼接
	// tenant_id IN ('', 当前租户)（平台视图只看 ''），与 sys_config / solution 的
	// 「全局默认 + 租户覆盖」处理方式一致。
	TenantId string `gorm:"column:tenant_id;type:string;size:64;index;comment:租户ID(空=平台菜单)"`
	Remark   string `gorm:"column:remark;type:string;size:500;comment:备注"`
}

// TenantMenuPref 租户菜单展示层覆盖：隐藏 / 自定义名称 / 图标 / 排序。
//
// 只覆盖**展示属性**，绝不覆盖 path / perms / component —— 否则平台修路径、
// 修权限 bug 时租户会永远卡在旧配置上。
//
// 唯一键 (tenant_id, menu_id)（迁移里建唯一索引 uk_tenant_menu_pref）。
// 本表同样**不**登记进 tenantScopedModels：删除全局菜单时要跨租户级联清理覆盖记录，
// 平台视图下也必须按显式 tenant_id 过滤，登记后这两点都做不到。
type TenantMenuPref struct {
	Timestamps
	Id         string `gorm:"column:id;primaryKey;type:string;size:64;comment:主键"`
	TenantId   string `gorm:"column:tenant_id;type:string;size:64;index;comment:租户ID"`
	MenuId     string `gorm:"column:menu_id;type:string;size:64;index;comment:菜单ID(sys_menu.id)"`
	Hidden     int    `gorm:"column:hidden;type:int;default:0;comment:是否隐藏 1隐藏 0不隐藏(含子级)"`
	CustomName string `gorm:"column:custom_name;type:string;size:255;comment:自定义名称(空=跟随平台)"`
	CustomIcon string `gorm:"column:custom_icon;type:string;size:128;comment:自定义图标(空=跟随平台)"`
	Sort       int    `gorm:"column:sort;type:int;default:0;comment:自定义排序(0=跟随平台)"`
}

func (table *TenantMenuPref) Get() interface{} {
	return *table
}

func (table *TenantMenuPref) TableName() string {
	return "tenant_menu_pref"
}

func (table *Menu) Get() interface{} {
	return *table
}

func (table *Menu) TableName() string {
	return "sys_menu"
}
