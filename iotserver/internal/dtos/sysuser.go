package dtos

/************** 请求 **************/

// SysUserSearchRequest 用户列表查询
type SysUserSearchRequest struct {
	Username string `schema:"username,omitempty" form:"username"`
	Nickname string `schema:"nickname,omitempty" form:"nickname"`
	Phone    string `schema:"phone,omitempty" form:"phone"`
	Status   string `schema:"status,omitempty" form:"status"`
	DeptId   string `schema:"deptId,omitempty" form:"deptId"`
	Page     int    `schema:"page,omitempty" form:"page" json:"page"`
	PageSize int    `schema:"pageSize,omitempty" form:"pageSize" json:"pageSize"`
	IsAll    bool   `schema:"isAll,omitempty" form:"isAll"`
}

// SysUserAddRequest 新增用户
type SysUserAddRequest struct {
	Username string   `json:"username" binding:"required"`
	Password string   `json:"password" binding:"required"`
	Nickname string   `json:"nickname"`
	Email    string   `json:"email"`
	Phone    string   `json:"phone"`
	DeptId   string   `json:"deptId"`
	Status   int      `json:"status"`
	Remark   string   `json:"remark"`
	RoleIds  []string `json:"roleIds"`
}

// SysUserUpdateRequest 编辑用户
type SysUserUpdateRequest struct {
	Id       string   `json:"id" binding:"required"`
	Nickname string   `json:"nickname"`
	Email    string   `json:"email"`
	Phone    string   `json:"phone"`
	DeptId   string   `json:"deptId"`
	Status   int      `json:"status"`
	Remark   string   `json:"remark"`
	RoleIds  []string `json:"roleIds"`
}

// SysUserIdsRequest 批量操作
type SysUserIdsRequest struct {
	Ids []string `json:"ids" binding:"required"`
}

// SysUserRegisterRequest OpenAPI 注册
type SysUserRegisterRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
	Nickname string `json:"nickname"`
	Email    string `json:"email"`
	Phone    string `json:"phone"`
}

/************** 响应 **************/

// SysUserResponse 用户信息
type SysUserResponse struct {
	Id        string   `json:"id"`
	Username  string   `json:"username"`
	Nickname  string   `json:"nickname"`
	Email     string   `json:"email"`
	Phone     string   `json:"phone"`
	DeptId    string   `json:"deptId"`
	DeptName  string   `json:"deptName"`
	Status    int      `json:"status"`
	Remark    string   `json:"remark"`
	RoleIds   []string `json:"roleIds"`
	RoleNames []string `json:"roleNames"`
	LastLogin int64    `json:"lastLogin"`
	Created   int64    `json:"created"`
	Modified  int64    `json:"modified"`
	Admin     bool     `json:"admin"`
}

// LoginUserResponse 登录用户信息（登录接口与个人中心共用）
type LoginUserResponse struct {
	Id          string            `json:"id"`
	Username    string            `json:"username"`
	Nickname    string            `json:"nickname"`
	Email       string            `json:"email"`
	Phone       string            `json:"phone"`
	Avatar      string            `json:"avatar"`
	DeptId      string            `json:"deptId"`
	DeptName    string            `json:"deptName"`
	Lang        string            `json:"lang"`
	Admin       bool              `json:"admin"`
	Roles       []string          `json:"roles"`
	RoleIds     []string          `json:"roleIds"`
	Permissions []string          `json:"permissions"`
	Menus       []SysMenuResponse `json:"menus"`
}

// SysUserProfileResponse 个人中心信息
type SysUserProfileResponse struct {
	User        SysUserResponse `json:"user"`
	Permissions []string        `json:"permissions"`
	Roles       []string        `json:"roles"`
	Admin       bool            `json:"admin"`
}
