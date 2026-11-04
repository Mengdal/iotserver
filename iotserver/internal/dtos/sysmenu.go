package dtos

/************** 请求 **************/

// SysMenuSearchRequest 菜单列表查询
type SysMenuSearchRequest struct {
	Name     string `schema:"name,omitempty" form:"name"`
	NameLike string `schema:"nameLike,omitempty" form:"nameLike"`
	Status   string `schema:"status,omitempty" form:"status"`
	MenuType string `schema:"menuType,omitempty" form:"menuType"`
	Page     int    `schema:"page,omitempty" form:"page" json:"page"`
	PageSize int    `schema:"pageSize,omitempty" form:"pageSize" json:"pageSize"`
	IsAll    bool   `schema:"isAll,omitempty" form:"isAll"`
}

// SysMenuAddRequest 新增菜单
type SysMenuAddRequest struct {
	ParentId  string `json:"parentId"`
	Name      string `json:"name" binding:"required"`
	NameEn    string `json:"nameEn"`
	Path      string `json:"path"`
	Component string `json:"component"`
	Icon      string `json:"icon"`
	MenuType  string `json:"menuType"`
	Perms     string `json:"perms"`
	Sort      int    `json:"sort"`
	Visible   int    `json:"visible"`
	Status    int    `json:"status"`
	IsFrame   int    `json:"isFrame"`
	Remark    string `json:"remark"`
}

// SysMenuUpdateRequest 编辑菜单
type SysMenuUpdateRequest struct {
	Id        string `json:"id" binding:"required"`
	ParentId  string `json:"parentId"`
	Name      string `json:"name" binding:"required"`
	NameEn    string `json:"nameEn"`
	Path      string `json:"path"`
	Component string `json:"component"`
	Icon      string `json:"icon"`
	MenuType  string `json:"menuType"`
	Perms     string `json:"perms"`
	Sort      int    `json:"sort"`
	Visible   int    `json:"visible"`
	Status    int    `json:"status"`
	IsFrame   int    `json:"isFrame"`
	Remark    string `json:"remark"`
}

// SysMenuIdsRequest 批量删除菜单
type SysMenuIdsRequest struct {
	Ids []string `json:"ids" binding:"required"`
}

// SysMenuPrefSaveRequest 租户菜单展示层覆盖（隐藏/改名/图标/排序）。
// 只写 tenant_menu_pref，不触碰菜单本体；平台视图（无租户）下不允许调用。
type SysMenuPrefSaveRequest struct {
	MenuId     string `json:"menuId" binding:"required"`
	Hidden     int    `json:"hidden"`
	CustomName string `json:"customName"`
	CustomIcon string `json:"customIcon"`
	Sort       int    `json:"sort"`
}

// SysMenuPrefResetRequest 平台重置某个租户的菜单定制（清空自建菜单 + 覆盖记录）
type SysMenuPrefResetRequest struct {
	TenantId string `json:"tenantId" binding:"required"`
}

// SysMenuPrefResponse 租户菜单展示层覆盖记录
type SysMenuPrefResponse struct {
	MenuId     string `json:"menuId"`
	Hidden     int    `json:"hidden"`
	CustomName string `json:"customName"`
	CustomIcon string `json:"customIcon"`
	Sort       int    `json:"sort"`
}

/************** 响应 **************/

// SysMenuResponse 菜单信息（children 用于构建树）
type SysMenuResponse struct {
	Id        string `json:"id"`
	ParentId  string `json:"parentId"`
	Name      string `json:"name"`
	NameEn    string `json:"nameEn"`
	Path      string `json:"path"`
	Component string `json:"component"`
	Icon      string `json:"icon"`
	MenuType  string `json:"menuType"`
	Perms     string `json:"perms"`
	Sort      int    `json:"sort"`
	Visible   int    `json:"visible"`
	Status    int    `json:"status"`
	IsFrame   int    `json:"isFrame"`
	// TenantId 菜单归属：'' = 平台菜单（只有平台管理员能写），非空 = 该租户的自建菜单
	TenantId string            `json:"tenantId"`
	Remark   string            `json:"remark"`
	Created  int64             `json:"created"`
	Modified int64             `json:"modified"`
	Children []SysMenuResponse `json:"children"`
}

// SysMenuRouteResponse 前端路由（精简字段）
type SysMenuRouteResponse struct {
	Id        string                 `json:"id"`
	ParentId  string                 `json:"parentId"`
	Name      string                 `json:"name"`
	Path      string                 `json:"path"`
	Component string                 `json:"component"`
	Icon      string                 `json:"icon"`
	Sort      int                    `json:"sort"`
	Visible   int                    `json:"visible"`
	Children  []SysMenuRouteResponse `json:"children"`
}
