package dtos

/************** 请求 **************/

// SysRoleSearchRequest 角色列表查询
type SysRoleSearchRequest struct {
	Name     string `schema:"name,omitempty" form:"name"`
	NameLike string `schema:"nameLike,omitempty" form:"nameLike"`
	Status   string `schema:"status,omitempty" form:"status"`
	Page     int    `schema:"page,omitempty" form:"page" json:"page"`
	PageSize int    `schema:"pageSize,omitempty" form:"pageSize" json:"pageSize"`
	IsAll    bool   `schema:"isAll,omitempty" form:"isAll"`

	// 以下字段由应用层按当前作用域填充，不接受客户端传参：
	// 平台视图 → tenant_id=''；租户视图 → 当前租户。
	ScopeTenantId string `json:"-" form:"-" schema:"-"`
	FilterTenant  bool   `json:"-" form:"-" schema:"-"`
	// IncludeGlobalBuiltin 租户视图额外包含全局内置角色（管理员/普通用户）。
	IncludeGlobalBuiltin bool `json:"-" form:"-" schema:"-"`
	// Assignable 只返回当前作用域可分配给用户的角色（用户表单下拉）。
	Assignable bool `json:"-" form:"-" schema:"-"`
}

// SysRoleAddRequest 新增角色
type SysRoleAddRequest struct {
	Name      string `json:"name" binding:"required"`
	Sort      int    `json:"sort"`
	DataScope string `json:"dataScope"`
	// Status 用指针区分「未传」和「显式传 0」：未传默认启用，显式 0 表示停用。
	Status  *int     `json:"status"`
	Remark  string   `json:"remark"`
	MenuIds []string `json:"menuIds"`
}

// SysRoleUpdateRequest 编辑角色
type SysRoleUpdateRequest struct {
	Id        string   `json:"id" binding:"required"`
	Name      string   `json:"name" binding:"required"`
	Sort      int      `json:"sort"`
	DataScope string   `json:"dataScope"`
	Status    int      `json:"status"`
	Remark    string   `json:"remark"`
	MenuIds   []string `json:"menuIds"`
}

// SysRoleIdsRequest 批量删除角色
type SysRoleIdsRequest struct {
	Ids []string `json:"ids" binding:"required"`
}

// SysRoleMenuRequest 角色绑定菜单
type SysRoleMenuRequest struct {
	RoleId  string   `json:"roleId" binding:"required"`
	MenuIds []string `json:"menuIds"`
}

/************** 响应 **************/

// SysRoleResponse 角色信息
type SysRoleResponse struct {
	Id         string   `json:"id"`
	Name       string   `json:"name"`
	IsAdmin    int      `json:"isAdmin"`
	BuiltinKey string   `json:"builtinKey"`
	Sort       int      `json:"sort"`
	DataScope  string   `json:"dataScope"`
	Status     int      `json:"status"`
	Remark     string   `json:"remark"`
	MenuIds    []string `json:"menuIds"`
	Created    int64    `json:"created"`
	Modified   int64    `json:"modified"`
}
