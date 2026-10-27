package dtos

/************** things 平台集成 **************/

// ThingsProject things 项目。
type ThingsProject struct {
	Id          int64  `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	CreateTime  string `json:"createTime"`
}

// ThingsGateway things 网关（不含 secret/license）。
type ThingsGateway struct {
	Id        int64  `json:"id"`
	ProjectId int64  `json:"projectId"`
	Sn        string `json:"sn"`
	Pk        string `json:"pk"`
	Name      string `json:"name"`
	Model     string `json:"model"`
}

// ThingsGatewayCreateRequest 新增 things 网关时由前端/公司提供的认证参数。
// IoTServer 只透传，不计算、不校验。
type ThingsGatewayCreateRequest struct {
	Sn          string `json:"sn" binding:"required"`
	Secret      string `json:"secret" binding:"required"`
	Name        string `json:"name" binding:"required"`
	Model       string `json:"model" binding:"required"`
	LicenseType string `json:"licenseType"`
	License     string `json:"license"`
}

// ThingsConnectionStatus 测试连接结果。
type ThingsConnectionStatus struct {
	BaseUrl      string `json:"baseUrl"`
	Username     string `json:"username"`
	ProjectCount int64  `json:"projectCount"`
}

// TenantThingsBindRequest 将已有租户绑定到已有 things 项目，或创建新项目后绑定。
type TenantThingsBindRequest struct {
	// ProjectId 已有项目 ID；为 0 时使用 Name 创建新项目
	ProjectId int64 `json:"projectId"`
	// Name 项目名称（ProjectId=0 时必填）
	Name string `json:"name"`
	// Description 项目备注
	Description string `json:"description"`
}

// TenantThingsRenameRequest 重命名租户绑定的 things 项目。
type TenantThingsRenameRequest struct {
	Name string `json:"name" binding:"required"`
}

// GatewayThingsRenameRequest 在 things 中重命名网关。
type GatewayThingsRenameRequest struct {
	Id   string `json:"id" binding:"required"`
	Name string `json:"name" binding:"required"`
}

// ThingsReconcileResult 对账结果。
type ThingsReconcileResult struct {
	Tenants  int `json:"tenants"`
	Gateways int `json:"gateways"`
	Missing  int `json:"missing"`
	Failed   int `json:"failed"`
}
