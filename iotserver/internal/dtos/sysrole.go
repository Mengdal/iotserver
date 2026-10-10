package dtos

/************** 请求 **************/

// SysRoleSearchRequest 角色列表查询
type SysRoleSearchRequest struct {
	Name     string `schema:"name,omitempty" form:"name"`
	NameLike string `schema:"nameLike,omitempty" form:"nameLike"`
	RoleKey  string `schema:"roleKey,omitempty" form:"roleKey"`
	Status   string `schema:"status,omitempty" form:"status"`
	Page     int    `schema:"page,omitempty" form:"page" json:"page"`
	PageSize int    `schema:"pageSize,omitempty" form:"pageSize" json:"pageSize"`
	IsAll    bool   `schema:"isAll,omitempty" form:"isAll"`
}

// SysRoleAddRequest 新增角色
type SysRoleAddRequest struct {
	Name      string   `json:"name" binding:"required"`
	RoleKey   string   `json:"roleKey" binding:"required"`
	Sort      int      `json:"sort"`
	DataScope string   `json:"dataScope"`
	Status    int      `json:"status"`
	Remark    string   `json:"remark"`
	MenuIds   []string `json:"menuIds"`
}

// SysRoleUpdateRequest 编辑角色
type SysRoleUpdateRequest struct {
	Id        string   `json:"id" binding:"required"`
	Name      string   `json:"name" binding:"required"`
	RoleKey   string   `json:"roleKey" binding:"required"`
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
	Id        string   `json:"id"`
	Name      string   `json:"name"`
	RoleKey   string   `json:"roleKey"`
	Sort      int      `json:"sort"`
	DataScope string   `json:"dataScope"`
	Status    int      `json:"status"`
	Remark    string   `json:"remark"`
	MenuIds   []string `json:"menuIds"`
	Created   int64    `json:"created"`
	Modified  int64    `json:"modified"`
}
