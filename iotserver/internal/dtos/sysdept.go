package dtos

/************** 请求 **************/

// SysDeptSearchRequest 部门列表查询
type SysDeptSearchRequest struct {
	Name     string `schema:"name,omitempty" form:"name"`
	NameLike string `schema:"nameLike,omitempty" form:"nameLike"`
	Status   string `schema:"status,omitempty" form:"status"`
	Page     int    `schema:"page,omitempty" form:"page" json:"page"`
	PageSize int    `schema:"pageSize,omitempty" form:"pageSize" json:"pageSize"`
	IsAll    bool   `schema:"isAll,omitempty" form:"isAll"`
}

// SysDeptAddRequest 新增部门
type SysDeptAddRequest struct {
	ParentId string `json:"parentId"`
	Name     string `json:"name" binding:"required"`
	// DeptType 组织节点类型：1公司 2工厂 3车间 4产线（0 表示未指定，按公司处理）
	DeptType int    `json:"deptType"`
	Sort     int    `json:"sort"`
	Leader   string `json:"leader"`
	Phone    string `json:"phone"`
	Email    string `json:"email"`
	Status   int    `json:"status"`
}

// SysDeptUpdateRequest 编辑部门
type SysDeptUpdateRequest struct {
	Id       string `json:"id" binding:"required"`
	ParentId string `json:"parentId"`
	Name     string `json:"name" binding:"required"`
	// DeptType 组织节点类型：1公司 2工厂 3车间 4产线（0 表示不修改）
	DeptType int    `json:"deptType"`
	Sort     int    `json:"sort"`
	Leader   string `json:"leader"`
	Phone    string `json:"phone"`
	Email    string `json:"email"`
	Status   int    `json:"status"`
}

// SysDeptIdsRequest 批量删除部门
type SysDeptIdsRequest struct {
	Ids []string `json:"ids" binding:"required"`
}

/************** 响应 **************/

// SysDeptResponse 部门信息
type SysDeptResponse struct {
	Id        string            `json:"id"`
	ParentId  string            `json:"parentId"`
	Ancestors string            `json:"ancestors"`
	Name      string            `json:"name"`
	DeptType  int               `json:"deptType"`
	Sort      int               `json:"sort"`
	Leader    string            `json:"leader"`
	Phone     string            `json:"phone"`
	Email     string            `json:"email"`
	Status    int               `json:"status"`
	Created   int64             `json:"created"`
	Modified  int64             `json:"modified"`
	Children  []SysDeptResponse `json:"children"`
}
