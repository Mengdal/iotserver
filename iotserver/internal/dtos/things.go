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

// ThingsGatewayCreateRequest 新增 things 网关的参数。
// Secret、License 由平台生成/使用 things 默认值，不再由前端提供。
type ThingsGatewayCreateRequest struct {
	Sn     string `json:"sn" binding:"required"`
	Secret string `json:"secret" binding:"required"`
	Name   string `json:"name" binding:"required"`
	Model  string `json:"model" binding:"required"`
}

// ThingsConnectionStatus 测试连接结果。
type ThingsConnectionStatus struct {
	BaseUrl      string `json:"baseUrl"`
	Username     string `json:"username"`
	ProjectCount int64  `json:"projectCount"`
}

// ThingsReconcileResult 对账结果。
type ThingsReconcileResult struct {
	Tenants  int `json:"tenants"`
	Gateways int `json:"gateways"`
	Missing  int `json:"missing"`
	Failed   int `json:"failed"`
}
