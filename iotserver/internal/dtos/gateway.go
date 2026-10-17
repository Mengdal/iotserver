package dtos

/************** 网关（第三方工业网关接入）**************/

// GatewaySearchQueryRequest 网关列表查询
type GatewaySearchQueryRequest struct {
	BaseSearchConditionQuery
	Name   string `schema:"name,omitempty" form:"name"`
	Sn     string `schema:"sn,omitempty" form:"sn"`
	DeptId string `schema:"deptId,omitempty" form:"deptId"`
	// Status 0离线 1在线 2从未上线；nil（不传）或其它值都表示不过滤。
	// 注意：不能用 int 的零值表示"不过滤"，否则"未选择状态"会被当成"只看离线"，
	// 导致 status=2（从未上线）的网关在列表里永远看不到。
	Status *int `schema:"status,omitempty" form:"status"`
}

// GatewayAddRequest 新增网关
type GatewayAddRequest struct {
	Name string `json:"name" binding:"required"`
	Sn   string `json:"sn" binding:"required"`
	// Pk 外部协议的产品标识。参考系统（/edge 协议）的网关身份是 {pk}/{sn}，
	// 平台按它拼订阅主题；留空则用 sn 顶替（实测两者取值常相同）。自有协议不用它。
	Pk     string `json:"pk"`
	DeptId string `json:"deptId"`
	Vendor string `json:"vendor"`
	Model  string `json:"model"`
	Remark string `json:"remark"`
}

// GatewayUpdateRequest 修改网关（SN 不可改：它同时是 MQTT clientId）
type GatewayUpdateRequest struct {
	Id     string `json:"id" binding:"required"`
	Name   string `json:"name"`
	Pk     string `json:"pk"`
	DeptId string `json:"deptId"`
	Vendor string `json:"vendor"`
	Model  string `json:"model"`
	Remark string `json:"remark"`
}

// GatewayConfigRequest 下发网关采集/上报配置
type GatewayConfigRequest struct {
	Id     string `json:"id" binding:"required"`
	Config string `json:"config" binding:"required"`
}

// GatewayCommandRequest 下发网关命令（restart / ota / sync）
type GatewayCommandRequest struct {
	Id      string `json:"id" binding:"required"`
	Command string `json:"command" binding:"required"`
	// Payload 可选参数（JSON 字符串），如 OTA 的固件地址
	Payload string `json:"payload"`
}

// GatewayMqttInfo 网关接入平台所需的 MQTT 参数（仅新增/重置密钥时返回一次）
type GatewayMqttInfo struct {
	Broker   string `json:"broker"`
	ClientId string `json:"clientId"`
	Username string `json:"username"`
	Password string `json:"password"`
	// 上行/下行 topic 前缀约定
	TopicUp   string `json:"topicUp"`
	TopicDown string `json:"topicDown"`
}

// GatewayResponse 网关详情
type GatewayResponse struct {
	Id     string `json:"id"`
	Name   string `json:"name"`
	Sn     string `json:"sn"`
	Pk     string `json:"pk"`
	DeptId string `json:"deptId"`
	// DeptName 组织节点名称（列表展示「所属」用）
	DeptName        string `json:"deptName"`
	Vendor          string `json:"vendor"`
	Model           string `json:"model"`
	FirmwareVersion string `json:"firmwareVersion"`
	Config          string `json:"config"`
	ConfigVersion   int    `json:"configVersion"`
	Status          int    `json:"status"`
	LastOnlineAt    int64  `json:"lastOnlineAt"`
	LastReportAt    int64  `json:"lastReportAt"`
	Ip              string `json:"ip"`
	Remark          string `json:"remark"`
	DeviceCount     int64  `json:"deviceCount"`
	Created         int64  `json:"created"`
	Modified        int64  `json:"modified"`
	// Mqtt 接入信息：仅新增网关/重置密钥时返回，避免密钥被反复读取
	Mqtt *GatewayMqttInfo `json:"mqtt,omitempty"`
}
