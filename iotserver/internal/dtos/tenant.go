package dtos

// TenantSearchQueryRequest 租户列表查询
type TenantSearchQueryRequest struct {
	BaseSearchConditionQuery
	Name string `schema:"name,omitempty" form:"name"`
	Code string `schema:"code,omitempty" form:"code"`
	// Status -1 表示不过滤
	Status int `schema:"status,omitempty" form:"status"`
}

// TenantAddRequest 新建租户
type TenantAddRequest struct {
	Code         string `json:"code" binding:"required"`
	Name         string `json:"name" binding:"required"`
	ContactName  string `json:"contactName"`
	ContactPhone string `json:"contactPhone"`
	ContactEmail string `json:"contactEmail"`
	ExpireAt     int64  `json:"expireAt"`
	MaxDevices   int    `json:"maxDevices"`
	MaxGateways  int    `json:"maxGateways"`
	MaxUsers     int    `json:"maxUsers"`
	// RetentionDays 时序数据保留天数：>0 按该天数、=0 用默认值（30 天）、<0 永久保留
	RetentionDays int    `json:"retentionDays"`
	Remark        string `json:"remark"`
}

// TenantUpdateRequest 修改租户
type TenantUpdateRequest struct {
	Id           string `json:"id" binding:"required"`
	Name         string `json:"name"`
	ContactName  string `json:"contactName"`
	ContactPhone string `json:"contactPhone"`
	ContactEmail string `json:"contactEmail"`
	ExpireAt     int64  `json:"expireAt"`
	MaxDevices   int    `json:"maxDevices"`
	MaxGateways  int    `json:"maxGateways"`
	MaxUsers     int    `json:"maxUsers"`
	// RetentionDays 时序数据保留天数：>0 按该天数、=0 用默认值（30 天）、<0 永久保留
	RetentionDays int    `json:"retentionDays"`
	Status        int    `json:"status"`
	Remark        string `json:"remark"`
}

// TenantResponse 租户详情
type TenantResponse struct {
	Id           string `json:"id"`
	Code         string `json:"code"`
	Name         string `json:"name"`
	Status       int    `json:"status"`
	ContactName  string `json:"contactName"`
	ContactPhone string `json:"contactPhone"`
	ContactEmail string `json:"contactEmail"`
	ExpireAt     int64  `json:"expireAt"`
	MaxDevices   int    `json:"maxDevices"`
	MaxGateways  int    `json:"maxGateways"`
	MaxUsers     int    `json:"maxUsers"`
	// RetentionDays 时序数据保留天数：>0 按该天数、=0 用默认值（30 天）、<0 永久保留
	RetentionDays int    `json:"retentionDays"`
	Remark        string `json:"remark"`
	Created       int64  `json:"created"`
	Modified      int64  `json:"modified"`
}
