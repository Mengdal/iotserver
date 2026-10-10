package dtos

/************** 登入日志 **************/

// SysLoginLogSearchRequest 登入日志查询
type SysLoginLogSearchRequest struct {
	Username  string `schema:"username,omitempty" form:"username"`
	Ip        string `schema:"ip,omitempty" form:"ip"`
	Status    string `schema:"status,omitempty" form:"status"`
	StartTime int64  `schema:"startTime,omitempty" form:"startTime"`
	EndTime   int64  `schema:"endTime,omitempty" form:"endTime"`
	Page      int    `schema:"page,omitempty" form:"page" json:"page"`
	PageSize  int    `schema:"pageSize,omitempty" form:"pageSize" json:"pageSize"`
	IsAll     bool   `schema:"isAll,omitempty" form:"isAll"`
}

// SysLoginLogIdsRequest 批量删除登入日志
type SysLoginLogIdsRequest struct {
	Ids []string `json:"ids" binding:"required"`
}

// SysLoginLogResponse 登入日志信息
type SysLoginLogResponse struct {
	Id        string `json:"id"`
	Username  string `json:"username"`
	Ip        string `json:"ip"`
	Browser   string `json:"browser"`
	Os        string `json:"os"`
	Status    int    `json:"status"`
	Message   string `json:"message"`
	LoginTime int64  `json:"loginTime"`
}

/************** 系统配置 **************/

// SysConfigSearchRequest 系统配置查询
type SysConfigSearchRequest struct {
	ConfigKey string `schema:"configKey,omitempty" form:"configKey"`
	NameLike  string `schema:"nameLike,omitempty" form:"nameLike"`
	GroupName string `schema:"groupName,omitempty" form:"groupName"`
	Page      int    `schema:"page,omitempty" form:"page" json:"page"`
	PageSize  int    `schema:"pageSize,omitempty" form:"pageSize" json:"pageSize"`
	IsAll     bool   `schema:"isAll,omitempty" form:"isAll"`
}

// SysConfigAddRequest 新增系统配置
type SysConfigAddRequest struct {
	ConfigKey   string `json:"configKey" binding:"required"`
	ConfigValue string `json:"configValue"`
	Name        string `json:"name" binding:"required"`
	GroupName   string `json:"groupName"`
	Type        string `json:"type"`
	Sort        int    `json:"sort"`
	Remark      string `json:"remark"`
}

// SysConfigUpdateRequest 编辑系统配置
type SysConfigUpdateRequest struct {
	Id          string `json:"id" binding:"required"`
	ConfigKey   string `json:"configKey" binding:"required"`
	ConfigValue string `json:"configValue"`
	Name        string `json:"name" binding:"required"`
	GroupName   string `json:"groupName"`
	Type        string `json:"type"`
	Sort        int    `json:"sort"`
	Remark      string `json:"remark"`
}

// SysConfigIdsRequest 批量删除系统配置
type SysConfigIdsRequest struct {
	Ids []string `json:"ids" binding:"required"`
}

// SysSetupRequest 系统配置批量保存（按 key 保存）
type SysSetupRequest struct {
	Items []SysSetupItem `json:"items" binding:"required"`
}

// SysSetupItem 单条配置
type SysSetupItem struct {
	ConfigKey   string `json:"configKey" binding:"required"`
	ConfigValue string `json:"configValue"`
}

// SysConfigResponse 系统配置信息
type SysConfigResponse struct {
	Id          string `json:"id"`
	ConfigKey   string `json:"configKey"`
	ConfigValue string `json:"configValue"`
	Name        string `json:"name"`
	GroupName   string `json:"groupName"`
	Type        string `json:"type"`
	Sort        int    `json:"sort"`
	Remark      string `json:"remark"`
	Created     int64  `json:"created"`
	Modified    int64  `json:"modified"`
}

/************** 站点品牌信息（登录页也需要，免鉴权） **************/

// SiteInfoResponse 站点品牌配置
type SiteInfoResponse struct {
	Name            string `json:"name"`
	Logo            string `json:"logo"`
	Favicon         string `json:"favicon"`
	LoginBackground string `json:"loginBackground"`
	Copyright       string `json:"copyright"`
	Description     string `json:"description"`
}
