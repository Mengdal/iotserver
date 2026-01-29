package dtos

import "time"

// UpdateDepartmentRequest 更新部门请求结构
type UpdateDepartmentRequest struct {
	ID        int64   `json:"id" example:"1"`
	Name      string  `json:"name" example:"生产部"`
	Leader    string  `json:"leader" example:"张三"`
	Phone     string  `json:"phone" example:"13800000000"`
	Email     string  `json:"email" example:"dept@test.com"`
	Status    string  `json:"status" example:"1"`
	Remark    string  `json:"remark" example:"生产部门"`
	ParentID  int64   `json:"parentId" example:"0"`
	Sort      int     `json:"sort" example:"1"`
	DeviceIds []int64 `json:"deviceIds" example:"[1,2,3]"`

	Factory     string `json:"factory" example:"电站类型：基站叠光、离网电站"`
	Description string `json:"desc" example:"站点名称"`
	GIS         string `json:"gis" example:"站点经纬度"`
	Address     string `json:"address" example:"区域"`
	Active      int64  `json:"active"`
	Capacity    string `json:"capacity" example:"装机容量"`
}

// AssignDevicesRequest 分配设备请求结构
type AssignDevicesRequest struct {
	DepartmentID int64   `json:"departmentId" example:"1"`
	DeviceIDs    []int64 `json:"deviceIds" example:"[1,2,3]"`
}

// RemoveDevicesRequest 移除设备请求结构
type RemoveDevicesRequest struct {
	DeviceIDs []int64 `json:"deviceIds" example:"[1,2,3]"`
}

// CreateDepartmentRequest 部门创建请求参数
type CreateDepartmentRequest struct {
	Name      string  `json:"name" example:"生产部"`
	LevelType string  `json:"level_type" example:"机构类型"`
	Leader    string  `json:"leader" example:"张三"`
	Phone     string  `json:"phone" example:"13800000000"`
	Email     string  `json:"email" example:"dept@test.com"`
	Status    string  `json:"status" example:"1"`
	Remark    string  `json:"remark" example:"生产部门"`
	ParentID  int64   `json:"parentId" example:"0"`
	Sort      int     `json:"sort" example:"1"`
	DeviceIDs []int64 `json:"deviceIds" example:"[1,2,3]"`

	Factory     string `json:"factory" example:"电站类型：基站叠光、离网电站"`
	Description string `json:"desc" example:"站点名称"`
	GIS         string `json:"gis" example:"站点经纬度"`
	Address     string `json:"address" example:"区域"`
	Active      int64  `json:"active" example:"开通时间"`
	Capacity    string `json:"capacity" example:"装机容量"`
}

type TenantDetailDTO struct {
	// Department 字段
	Id        int64  `json:"id"`
	Name      string `json:"name"`
	LevelType string `json:"level_type"`
	ParentId  int64  `json:"parent_id,omitempty"`
	TenantId  int64  `json:"tenant_id"`
	Leader    string `json:"leader"`
	Phone     string `json:"phone"`
	Email     string `json:"email"`
	Status    string `json:"status"`
	Sort      int    `json:"sort"`
	Remark    string `json:"remark"`
	Created   int64  `json:"created"`
	Modified  int64  `json:"modified"`
	// Project 补充字段
	Devices interface{} `json:"devices"`
	// Tenant 补充字段
	Enable         bool       `json:"enable"`
	Address        string     `json:"address"`
	Area           string     `json:"area"`
	DeviceNum      int        `json:"deviceNum"`
	Images         string     `json:"images"`
	Logo           string     `json:"logo"`
	PersonNum      string     `json:"personNum"`
	Ranges         string     `json:"ranges"`
	ActiveTime     time.Time  `json:"activeTime"`
	Icon           string     `json:"icon"`
	ExpirationTime *time.Time `json:"expirationTime"`
	// 机构补充字段
	Factory     string `json:"factory"`  // 工厂/电站类型
	Active      int64  `json:"active"`   // 开通时间
	Description string `json:"desc"`     // 描述/站点名称
	GIS         string `json:"gis"`      // GIS信息/站点经纬度
	Capacity    string `json:"capacity"` // 容量/装机容量
	AreaId      int64  `json:"area_id"`  // 区域ID
}
