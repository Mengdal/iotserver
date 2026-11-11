package dtos

// SystemComponentDTO 内置系统组件（Mosquitto / eKuiper / IotEdgeDB 等）的展示信息。
type SystemComponentDTO struct {
	Id          string `json:"id"`
	Name        string `json:"name"`
	Version     string `json:"version"`
	Description string `json:"description"`
	Runtime     string `json:"runtime"`
	Essential   bool   `json:"essential"`
	// Endpoint 组件实际监听地址（来自平台配置，仅用于展示与探活）
	Endpoint string `json:"endpoint"`
	// Status running / stopped / unknown
	Status  string `json:"status"`
	Updated int64  `json:"updated"`
}
